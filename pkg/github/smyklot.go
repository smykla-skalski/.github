package github

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/go-github/v79/github"

	"github.com/smykla-labs/.github/pkg/config"
	"github.com/smykla-labs/.github/pkg/logger"
)

const (
	smyklotBranchPrefix = "chore/sync-smyklot"
	smyklotPRLabel      = "ci/skip-all"
)

// SmyklotSyncStats tracks smyklot sync statistics.
type SmyklotSyncStats struct {
	Updated      int
	Skipped      int
	UpdatedFiles []string
}

// SyncSmyklot synchronizes smyklot version references in workflow files.
func SyncSmyklot(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	version string,
	tag string,
	syncConfig *config.SyncConfig,
	dryRun bool,
) error {
	// Check if sync is skipped
	if syncConfig.Sync.Skip || syncConfig.Sync.Smyklot.Skip {
		log.Info("smyklot sync skipped by config")

		// Check for existing PR to close
		skipReason := getSkipReason(syncConfig)
		if err := closeSmyklotPR(ctx, log, client, org, repo, skipReason); err != nil {
			log.Warn("failed to close existing PR", "error", err)
		}

		return nil
	}

	// Get repository info
	defaultBranch, baseSHA, err := getRepoBaseInfo(ctx, log, client, org, repo)
	if err != nil {
		return errors.Wrap(err, "getting repository base info")
	}

	// List workflow files
	workflowFiles, err := listWorkflowFiles(ctx, log, client, org, repo)
	if err != nil {
		return errors.Wrap(err, "listing workflow files")
	}

	if len(workflowFiles) == 0 {
		log.Info("no workflow files found")

		return nil
	}

	log.Debug("found workflow files", "count", len(workflowFiles))

	// Process workflow files
	stats := &SmyklotSyncStats{}

	var changes []FileChange

	for _, workflowPath := range workflowFiles {
		change, processed := processWorkflowFile(
			ctx, log, client, org, repo, workflowPath, version, tag, stats,
		)
		if processed {
			changes = append(changes, change)
		}
	}

	// Log stats
	log.Info("smyklot sync summary",
		"updated", stats.Updated,
		"skipped", stats.Skipped,
	)

	// If no changes, close any existing PR
	if len(changes) == 0 {
		log.Info("no changes needed")

		if err := closeSmyklotPR(ctx, log, client, org, repo,
			"smyklot is already up to date. Closing this PR."); err != nil {
			log.Warn("failed to close existing PR", "error", err)
		}

		return nil
	}

	if dryRun {
		log.Info("dry-run mode: skipping PR creation")
		logSmyklotChanges(log, stats)

		return nil
	}

	// Create/update PR
	if err := createSmyklotPR(
		ctx,
		log,
		client,
		org,
		repo,
		defaultBranch,
		baseSHA,
		tag,
		changes,
		stats,
	); err != nil {
		return errors.Wrap(err, "creating or updating PR")
	}

	log.Info("smyklot sync completed successfully")

	return nil
}

// getSkipReason returns the reason for skipping smyklot sync.
func getSkipReason(syncConfig *config.SyncConfig) string {
	if syncConfig.Sync.Skip {
		return "smyklot version synchronization is disabled for this repository (sync.skip=true)"
	}

	return "smyklot version synchronization is disabled for this repository (sync.smyklot.skip=true)"
}

// listWorkflowFiles lists all workflow files in .github/workflows directory.
func listWorkflowFiles(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
) ([]string, error) {
	log.Debug("listing workflow files")

	_, dirContent, _, err := client.Repositories.GetContents(
		ctx, org, repo, ".github/workflows", nil,
	)
	if err != nil {
		if isNotFoundError(err) {
			return []string{}, nil
		}

		return nil, errors.Wrap(err, "listing workflows directory")
	}

	var workflowFiles []string

	for _, content := range dirContent {
		if content.GetType() != "file" {
			continue
		}

		name := content.GetName()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			workflowFiles = append(workflowFiles, content.GetPath())
		}
	}

	return workflowFiles, nil
}

// processWorkflowFile processes a single workflow file and returns changes.
func processWorkflowFile(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	workflowPath string,
	version string,
	tag string,
	stats *SmyklotSyncStats,
) (FileChange, bool) {
	log.Debug("processing workflow file", "path", workflowPath)

	// Fetch file content
	fileContent, _, _, err := client.Repositories.GetContents(ctx, org, repo, workflowPath, nil)
	if err != nil {
		log.Warn("failed to fetch workflow file", "path", workflowPath, "error", err)

		return FileChange{}, false
	}

	content, err := fileContent.GetContent()
	if err != nil {
		log.Warn("failed to decode workflow file", "path", workflowPath, "error", err)

		return FileChange{}, false
	}

	// Apply version replacements
	updatedContent, changed := applyVersionReplacements(content, version, tag)
	if !changed {
		log.Debug("no smyklot references or already up to date", "file", workflowPath)

		stats.Skipped++

		return FileChange{}, false
	}

	log.Debug("found outdated smyklot references", "file", workflowPath)

	stats.Updated++
	stats.UpdatedFiles = append(stats.UpdatedFiles, workflowPath)

	return FileChange{
		Path:    workflowPath,
		Content: []byte(updatedContent),
		Action:  "update",
	}, true
}

// applyVersionReplacements applies smyklot version replacements to content.
func applyVersionReplacements(content string, version string, tag string) (string, bool) {
	original := content

	// Pattern 1: GitHub Action reference (uses: smykla-labs/smyklot@v1.2.3)
	actionPattern := regexp.MustCompile(`(uses:\s*smykla-labs/smyklot@)v\d+\.\d+\.\d+`)
	content = actionPattern.ReplaceAllString(content, "${1}"+tag)

	// Pattern 2: Docker image reference (ghcr.io/smykla-labs/smyklot:1.2.3)
	dockerPattern := regexp.MustCompile(`(ghcr\.io/smykla-labs/smyklot:)\d+\.\d+\.\d+`)
	content = dockerPattern.ReplaceAllString(content, "${1}"+version)

	return content, content != original
}

// closeSmyklotPR closes an existing smyklot PR if it exists.
func closeSmyklotPR(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	comment string,
) error {
	branchName := smyklotBranchPrefix

	// List open PRs for the branch
	prs, _, err := client.PullRequests.List(ctx, org, repo, &github.PullRequestListOptions{
		State: "open",
		Head:  org + ":" + branchName,
	})
	if err != nil {
		return errors.Wrap(err, "listing PRs")
	}

	if len(prs) == 0 {
		return nil
	}

	prNumber := prs[0].GetNumber()
	log.Info("closing existing PR", "pr", prNumber)

	// Close the PR
	pr := &github.PullRequest{
		State: github.Ptr("closed"),
	}

	_, _, err = client.PullRequests.Edit(ctx, org, repo, prNumber, pr)
	if err != nil {
		return errors.Wrap(err, "closing PR")
	}

	// Add comment
	prComment := &github.IssueComment{
		Body: github.Ptr(comment),
	}

	_, _, err = client.Issues.CreateComment(ctx, org, repo, prNumber, prComment)
	if err != nil {
		log.Warn("failed to add PR comment", "error", err)
	}

	return nil
}

// createSmyklotPR creates or updates a PR with smyklot version updates.
func createSmyklotPR(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	defaultBranch string,
	baseSHA string,
	tag string,
	changes []FileChange,
	stats *SmyklotSyncStats,
) error {
	branchName := smyklotBranchPrefix
	log.Info("creating/updating PR", "branch", branchName)

	// Ensure branch exists
	if err := ensureBranchExists(ctx, log, client, org, repo, branchName, baseSHA); err != nil {
		return errors.Wrap(err, "ensuring branch exists")
	}

	// Create Git commit
	if err := createGitCommit(ctx, log, client, org, repo, branchName, baseSHA, changes); err != nil {
		return errors.Wrap(err, "creating Git commit")
	}

	// Create or update pull request
	prNumber, err := upsertSmyklotPullRequest(
		ctx, log, client, org, repo, defaultBranch, branchName, tag, stats,
	)
	if err != nil {
		return errors.Wrap(err, "upserting pull request")
	}

	// Add labels and enable auto-merge
	return finalizeSmyklotPR(ctx, log, client, org, repo, prNumber)
}

// upsertSmyklotPullRequest creates or updates a smyklot sync pull request.
func upsertSmyklotPullRequest(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	defaultBranch string,
	branchName string,
	tag string,
	stats *SmyklotSyncStats,
) (int, error) {
	prTitle := "chore(deps): update smyklot to " + tag
	prBody := buildSmyklotPRBody(tag, stats)

	// Check for existing PR
	prs, _, err := client.PullRequests.List(ctx, org, repo, &github.PullRequestListOptions{
		State: "open",
		Head:  org + ":" + branchName,
	})
	if err != nil {
		return 0, errors.Wrap(err, "listing PRs")
	}

	if len(prs) > 0 {
		// Update existing PR
		prNumber := prs[0].GetNumber()
		log.Info("updating existing PR", "pr", prNumber)

		pr := &github.PullRequest{
			Title: github.Ptr(prTitle),
			Body:  github.Ptr(prBody),
		}

		_, _, editErr := client.PullRequests.Edit(ctx, org, repo, prNumber, pr)
		if editErr != nil {
			return 0, errors.Wrap(editErr, "updating PR")
		}

		return prNumber, nil
	}

	// Create new PR
	log.Info("creating new PR")

	pr := &github.NewPullRequest{
		Title: github.Ptr(prTitle),
		Head:  github.Ptr(branchName),
		Base:  github.Ptr(defaultBranch),
		Body:  github.Ptr(prBody),
	}

	createdPR, _, err := client.PullRequests.Create(ctx, org, repo, pr)
	if err != nil {
		return 0, errors.Wrap(err, "creating PR")
	}

	prNumber := createdPR.GetNumber()
	log.Info("created PR", "pr", prNumber, "url", createdPR.GetHTMLURL())

	return prNumber, nil
}

// finalizeSmyklotPR adds labels and enables auto-merge for a smyklot PR.
func finalizeSmyklotPR(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	prNumber int,
) error {
	// Add label
	_, _, err := client.Issues.AddLabelsToIssue(ctx, org, repo, prNumber, []string{smyklotPRLabel})
	if err != nil {
		log.Warn("failed to add label to PR", "error", err)
	}

	// Enable auto-merge
	if err := enableAutoMerge(ctx, log, client, org, repo, prNumber); err != nil {
		log.Warn("failed to enable auto-merge", "error", err)
	}

	return nil
}

// buildSmyklotPRBody builds the PR body text for smyklot sync.
func buildSmyklotPRBody(tag string, stats *SmyklotSyncStats) string {
	var body strings.Builder

	body.WriteString(fmt.Sprintf(
		"Updates [`smykla-labs/smyklot`](https://github.com/smykla-labs/smyklot) "+
			"to version [`%s`](https://github.com/smykla-labs/smyklot/releases/tag/%s).\n",
		tag, tag,
	))

	// Files updated section
	if len(stats.UpdatedFiles) > 0 {
		body.WriteString("\n## Files Updated\n\n")

		for _, file := range stats.UpdatedFiles {
			body.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
	}

	body.WriteString("\n---\n\n")
	body.WriteString("*This PR was automatically created by the smyklot sync workflow*\n")

	return body.String()
}

// logSmyklotChanges logs the planned smyklot changes in dry-run mode.
func logSmyklotChanges(log *logger.Logger, stats *SmyklotSyncStats) {
	if len(stats.UpdatedFiles) > 0 {
		log.Info("files to update:")

		for _, file := range stats.UpdatedFiles {
			log.Info("  ~ " + file)
		}
	}

	if stats.Updated == 0 {
		log.Info("no smyklot changes needed")
	}
}
