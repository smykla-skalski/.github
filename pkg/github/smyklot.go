package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/go-github/v80/github"
	"gopkg.in/yaml.v3"

	"github.com/smykla-labs/.github/internal/configtypes"
	"github.com/smykla-labs/.github/pkg/logger"
)

const (
	smyklotBranchPrefix = "chore/sync-smyklot"
	smyklotPRLabel      = "ci/skip-all"
)

// SmyklotSyncStats tracks smyklot sync statistics.
type SmyklotSyncStats struct {
	Updated          int
	Skipped          int
	UpdatedFiles     []string
	Installed        int
	InstalledFiles   []string
	Replaced         int
	ReplacedFiles    []string
	VersionOnly      int
	VersionOnlyFiles []string
}

// SyncSmyklot synchronizes smyklot workflows and version references.
//
//nolint:gocognit,nestif,funlen // complexity from workflow template sync logic
func SyncSmyklot(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	version string,
	tag string,
	sha string,
	syncConfig *configtypes.SyncConfig,
	templatesDir string,
	smyklotFilePath string,
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

	// Fetch org-level smyklot config
	orgConfig, err := fetchSmyklotOrgConfig(ctx, client, org, smyklotFilePath)
	if err != nil {
		return errors.Wrap(err, "fetching org smyklot config")
	}

	// Get repository info
	defaultBranch, baseSHA, err := getRepoBaseInfo(ctx, log, client, org, repo)
	if err != nil {
		return errors.Wrap(err, "getting repository base info")
	}

	// List existing workflow files
	workflowFiles, err := listWorkflowFiles(ctx, log, client, org, repo)
	if err != nil {
		return errors.Wrap(err, "listing workflow files")
	}

	// Create a map of existing workflows for quick lookup
	existingWorkflows := make(map[string]bool)

	for _, path := range workflowFiles {
		filename := filepath.Base(path)
		existingWorkflows[filename] = true
	}

	// Process workflow templates
	stats := &SmyklotSyncStats{}

	var changes []FileChange

	workflowNames := []string{"pr-commands", "poll-reactions"}

	for _, workflowName := range workflowNames {
		if !shouldSyncWorkflow(orgConfig, syncConfig, workflowName) {
			log.Debug("workflow sync disabled by config", "workflow", workflowName)

			continue
		}

		// Read and render template
		templateContent, err := readWorkflowTemplate(templatesDir, workflowName)
		if err != nil {
			log.Warn("failed to read workflow template", "workflow", workflowName, "error", err)

			continue
		}

		expectedContent := renderWorkflowTemplate(templateContent, tag, sha)
		workflowPath := ".github/workflows/" + workflowName + ".yml"

		// Check if workflow already exists
		if existingWorkflows[workflowName+".yml"] {
			// Fetch existing workflow
			fileContent, _, _, fetchErr := client.Repositories.GetContents(
				ctx, org, repo, workflowPath, nil,
			)
			if fetchErr != nil {
				log.Warn(
					"failed to fetch existing workflow",
					"path",
					workflowPath,
					"error",
					fetchErr,
				)

				continue
			}

			existingContent, decodeErr := fileContent.GetContent()
			if decodeErr != nil {
				log.Warn(
					"failed to decode existing workflow",
					"path",
					workflowPath,
					"error",
					decodeErr,
				)

				continue
			}

			// Compare content
			if existingContent != string(expectedContent) {
				log.Debug("workflow differs from template, will replace",
					"workflow", workflowName)

				stats.Replaced++
				stats.ReplacedFiles = append(stats.ReplacedFiles, workflowPath)

				changes = append(changes, FileChange{
					Path:    workflowPath,
					Content: expectedContent,
					Action:  "update",
				})
			} else {
				log.Debug("workflow matches template", "workflow", workflowName)

				stats.Skipped++
			}
		} else {
			// Workflow doesn't exist, install it
			log.Debug("workflow not found, will install", "workflow", workflowName)

			stats.Installed++
			stats.InstalledFiles = append(stats.InstalledFiles, workflowPath)

			changes = append(changes, FileChange{
				Path:    workflowPath,
				Content: expectedContent,
				Action:  "create",
			})
		}
	}

	// Version-only sync for other workflows if enabled and no workflow structure changes
	if orgConfig.SyncVersion && len(changes) == 0 && len(workflowFiles) > 0 {
		log.Debug("no workflow structure changes, checking for version-only updates")

		for _, workflowPath := range workflowFiles {
			// Skip the managed workflow files
			filename := filepath.Base(workflowPath)
			if filename == "pr-commands.yml" || filename == "poll-reactions.yml" {
				continue
			}

			change, processed := processWorkflowFile(
				ctx, log, client, org, repo, workflowPath, version, tag, stats,
			)
			if processed {
				changes = append(changes, change)
			}
		}
	}

	// Log stats
	log.Info("smyklot sync summary",
		"installed", stats.Installed,
		"replaced", stats.Replaced,
		"version_only", stats.VersionOnly,
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
func getSkipReason(syncConfig *configtypes.SyncConfig) string {
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

	stats.VersionOnly++
	stats.VersionOnlyFiles = append(stats.VersionOnlyFiles, workflowPath)

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

	// Workflows installed section
	if len(stats.InstalledFiles) > 0 {
		body.WriteString("\n## Workflows Installed\n\n")

		for _, file := range stats.InstalledFiles {
			body.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
	}

	// Workflows replaced section
	if len(stats.ReplacedFiles) > 0 {
		body.WriteString("\n## Workflows Replaced\n\n")

		for _, file := range stats.ReplacedFiles {
			body.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
	}

	// Version-only updates section
	if len(stats.VersionOnlyFiles) > 0 {
		body.WriteString("\n## Version Updates\n\n")

		for _, file := range stats.VersionOnlyFiles {
			body.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
	}

	// Legacy files updated section (for backward compatibility)
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
	if len(stats.InstalledFiles) > 0 {
		log.Info("workflows to install:")

		for _, file := range stats.InstalledFiles {
			log.Info("  + " + file)
		}
	}

	if len(stats.ReplacedFiles) > 0 {
		log.Info("workflows to replace:")

		for _, file := range stats.ReplacedFiles {
			log.Info("  ~ " + file)
		}
	}

	if len(stats.VersionOnlyFiles) > 0 {
		log.Info("workflows with version-only updates:")

		for _, file := range stats.VersionOnlyFiles {
			log.Info("  v " + file)
		}
	}

	if len(stats.UpdatedFiles) > 0 {
		log.Info("files to update:")

		for _, file := range stats.UpdatedFiles {
			log.Info("  ~ " + file)
		}
	}

	if stats.Installed+stats.Replaced+stats.VersionOnly+stats.Updated == 0 {
		log.Info("no smyklot changes needed")
	}
}

// readWorkflowTemplate reads a workflow template from the templates directory.
func readWorkflowTemplate(templatesDir string, name string) ([]byte, error) {
	templatePath := filepath.Join(templatesDir, name+".yml")

	//nolint:gosec // templatesDir is from CLI flag, name is from controlled list
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading template %s", templatePath)
	}

	return content, nil
}

// renderWorkflowTemplate replaces {{TAG}} and {{SHA}} placeholders in a template.
func renderWorkflowTemplate(content []byte, tag string, sha string) []byte {
	rendered := string(content)
	rendered = strings.ReplaceAll(rendered, "{{TAG}}", tag)
	rendered = strings.ReplaceAll(rendered, "{{SHA}}", sha)

	return []byte(rendered)
}

// fetchSmyklotOrgConfig fetches the org-level smyklot config from .github repo.
func fetchSmyklotOrgConfig(
	ctx context.Context,
	client *Client,
	org string,
	smyklotFilePath string,
) (*configtypes.SmyklotFile, error) {
	// If smyklotFilePath is provided (local file), read from filesystem
	if smyklotFilePath != "" {
		//nolint:gosec // smyklotFilePath is from CLI flag
		content, err := os.ReadFile(smyklotFilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading smyklot config from %s", smyklotFilePath)
		}

		var config configtypes.SmyklotFile
		if err := yaml.Unmarshal(content, &config); err != nil {
			return nil, errors.Wrap(err, "parsing smyklot config")
		}

		return &config, nil
	}

	// Otherwise fetch from .github repository
	fileContent, _, _, err := client.Repositories.GetContents(
		ctx,
		org,
		".github",
		".github/smyklot.yml",
		nil,
	)
	if err != nil {
		if isNotFoundError(err) {
			// Return default config if file doesn't exist
			return &configtypes.SmyklotFile{
				SyncVersion: true,
				Workflows: configtypes.SmyklotWorkflowsConfig{
					PrCommands:    boolPtr(true),
					PollReactions: boolPtr(true),
				},
			}, nil
		}

		return nil, errors.Wrap(err, "fetching smyklot org config")
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, errors.Wrap(err, "decoding smyklot org config")
	}

	var config configtypes.SmyklotFile
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, errors.Wrap(err, "parsing smyklot org config")
	}

	return &config, nil
}

// shouldSyncWorkflow determines if a workflow should be synced based on org and repo config.
func shouldSyncWorkflow(
	orgConfig *configtypes.SmyklotFile,
	repoConfig *configtypes.SyncConfig,
	workflowName string,
) bool {
	// Get the workflow setting from org config
	var orgEnabled *bool

	switch workflowName {
	case "pr-commands":
		orgEnabled = orgConfig.Workflows.PrCommands
	case "poll-reactions":
		orgEnabled = orgConfig.Workflows.PollReactions
	default:
		return false
	}

	// Get repo override if it exists
	var repoEnabled *bool

	switch workflowName {
	case "pr-commands":
		repoEnabled = repoConfig.Sync.Smyklot.Workflows.PrCommands
	case "poll-reactions":
		repoEnabled = repoConfig.Sync.Smyklot.Workflows.PollReactions
	}

	// Repo config takes precedence
	if repoEnabled != nil {
		return *repoEnabled
	}

	// Fall back to org config
	if orgEnabled != nil {
		return *orgEnabled
	}

	// Default to true
	return true
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}
