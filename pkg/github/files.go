package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/go-github/v80/github"

	"github.com/smykla-labs/.github/pkg/config"
	"github.com/smykla-labs/.github/pkg/logger"
)

const (
	httpStatusNotFound    = 404
	commitsPerPageForFile = 20
)

// FileMapping represents a source to destination file mapping.
type FileMapping struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

// FileChange represents a file change to be applied.
type FileChange struct {
	Path    string
	Content []byte
	Action  string // "create", "update", "delete"
	BlobSHA string // For blobs created
}

// FileSyncStats tracks file sync statistics.
type FileSyncStats struct {
	Created          int
	Updated          int
	Deleted          int
	Skipped          int
	Excluded         int
	ModifiedExcluded int
	CreatedFiles     []string
	UpdatedFiles     []string
	DeletedFiles     []string
}

// SyncFiles synchronizes files from a central repo to a target repository.
func SyncFiles(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	sourceRepo string,
	filesConfig string,
	syncConfig *config.SyncConfig,
	branchPrefix string,
	prLabels []string,
	dryRun bool,
) error {
	// Check if sync is skipped
	if syncConfig.Sync.Skip || syncConfig.Sync.Files.Skip {
		log.Info("file sync skipped by config")

		// Check for existing PR to close
		if err := closeExistingPR(ctx, log, client, org, repo, branchPrefix,
			"File synchronization is disabled for this repository"); err != nil {
			log.Warn("failed to close existing PR", "error", err)
		}

		return nil
	}

	// Parse files config
	fileMappings, err := parseFilesConfig(filesConfig)
	if err != nil {
		return errors.Wrap(err, "parsing files config")
	}

	log.Debug("parsed files config", "count", len(fileMappings))

	// Get repository info
	defaultBranch, baseSHA, err := getRepoBaseInfo(ctx, log, client, org, repo)
	if err != nil {
		return errors.Wrap(err, "getting repository base info")
	}

	// Process files
	stats := &FileSyncStats{}

	var changes []FileChange

	for _, mapping := range fileMappings {
		fileChanges := processFileMapping(
			ctx, log, client, org, repo, sourceRepo, mapping, syncConfig, stats,
		)
		changes = append(changes, fileChanges...)
	}

	// Log stats
	log.Info("file sync summary",
		"created", stats.Created,
		"updated", stats.Updated,
		"deleted", stats.Deleted,
		"skipped", stats.Skipped,
		"excluded", stats.Excluded,
		"modified_excluded", stats.ModifiedExcluded,
	)

	// If no changes, close any existing PR
	if len(changes) == 0 {
		log.Info("no changes needed")

		if err := closeExistingPR(ctx, log, client, org, repo, branchPrefix,
			"All files are now in sync. Closing this PR."); err != nil {
			log.Warn("failed to close existing PR", "error", err)
		}

		return nil
	}

	if dryRun {
		log.Info("dry-run mode: skipping PR creation")
		logFileChanges(log, stats)

		return nil
	}

	// Create/update PR
	if err := createOrUpdatePR(
		ctx,
		log,
		client,
		org,
		repo,
		sourceRepo,
		defaultBranch,
		baseSHA,
		branchPrefix,
		prLabels,
		changes,
		stats,
	); err != nil {
		return errors.Wrap(err, "creating or updating PR")
	}

	log.Info("file sync completed successfully")

	return nil
}

// getRepoBaseInfo retrieves the default branch and base SHA for a repository.
func getRepoBaseInfo(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
) (string, string, error) {
	// Get default branch
	repoInfo, _, err := client.Repositories.Get(ctx, org, repo)
	if err != nil {
		return "", "", errors.Wrap(err, "getting repository info")
	}

	defaultBranch := repoInfo.GetDefaultBranch()
	log.Debug("default branch", "branch", defaultBranch)

	// Get base SHA
	ref, _, err := client.Git.GetRef(ctx, org, repo, "heads/"+defaultBranch)
	if err != nil {
		return "", "", errors.Wrap(err, "getting default branch ref")
	}

	baseSHA := ref.GetObject().GetSHA()
	log.Debug("base SHA", "sha", baseSHA[:7])

	return defaultBranch, baseSHA, nil
}

// processFileMapping processes a single file mapping and returns any changes.
func processFileMapping(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	sourceRepo string,
	mapping FileMapping,
	syncConfig *config.SyncConfig,
	stats *FileSyncStats,
) []FileChange {
	log.Debug("processing file", "dest", mapping.Dest)

	// Check if file is excluded
	if isExcluded(mapping.Dest, syncConfig.Sync.Files.Exclude) {
		log.Debug("file excluded by config", "file", mapping.Dest)

		stats.Excluded++

		return nil
	}

	// Fetch source file
	sourceContent, err := fetchFileContent(ctx, client, org, sourceRepo, mapping.Source)
	if err != nil {
		log.Warn("source file not found", "path", mapping.Source, "error", err)

		return nil
	}

	var changes []FileChange

	// Special case: renovate.json - check for non-standard locations to delete
	if mapping.Dest == "renovate.json" {
		deleteChanges := checkNonStandardRenovateConfigs(ctx, log, client, org, repo, stats)
		changes = append(changes, deleteChanges...)
	}

	// Fetch target file
	targetContent, targetExists, err := fetchTargetFile(ctx, client, org, repo, mapping.Dest)
	if err != nil {
		log.Warn("failed to fetch target file", "path", mapping.Dest, "error", err)

		return changes
	}

	if targetExists {
		return processExistingFile(
			ctx, log, client, org, repo, mapping, sourceContent, targetContent, stats, changes,
		)
	}

	return processNewFile(log, mapping, sourceContent, stats, changes)
}

// processExistingFile handles updates to existing files.
func processExistingFile(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	mapping FileMapping,
	sourceContent []byte,
	targetContent []byte,
	stats *FileSyncStats,
	changes []FileChange,
) []FileChange {
	// File exists - check if update needed
	if string(sourceContent) == string(targetContent) {
		log.Debug("file already up to date", "file", mapping.Dest)

		stats.Skipped++

		return changes
	}

	// Special case: renovate.json - check for manual modifications
	if mapping.Dest == "renovate.json" {
		if shouldSkipRenovateJSON(ctx, log, client, org, repo, mapping.Dest, stats) {
			return changes
		}
	}

	log.Debug("file needs update", "file", mapping.Dest)

	stats.Updated++
	stats.UpdatedFiles = append(stats.UpdatedFiles, mapping.Dest)

	return append(changes, FileChange{
		Path:    mapping.Dest,
		Content: sourceContent,
		Action:  "update",
	})
}

// processNewFile handles creation of new files.
func processNewFile(
	log *logger.Logger,
	mapping FileMapping,
	sourceContent []byte,
	stats *FileSyncStats,
	changes []FileChange,
) []FileChange {
	log.Debug("will create file", "file", mapping.Dest)

	stats.Created++
	stats.CreatedFiles = append(stats.CreatedFiles, mapping.Dest)

	return append(changes, FileChange{
		Path:    mapping.Dest,
		Content: sourceContent,
		Action:  "create",
	})
}

// shouldSkipRenovateJSON checks if renovate.json should be skipped due to manual modifications.
func shouldSkipRenovateJSON(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	path string,
	stats *FileSyncStats,
) bool {
	hasManualChanges, err := hasManualModifications(ctx, client, org, repo, path)
	if err != nil {
		log.Warn("failed to check manual modifications", "file", path, "error", err)

		return false
	}

	if hasManualChanges {
		log.Info("file has manual modifications, excluding from sync", "file", path)

		stats.ModifiedExcluded++

		return true
	}

	return false
}

// parseFilesConfig parses the files configuration JSON.
func parseFilesConfig(filesConfig string) ([]FileMapping, error) {
	if filesConfig == "" {
		return nil, errors.New("files config is empty")
	}

	var mappings []FileMapping
	if err := json.Unmarshal([]byte(filesConfig), &mappings); err != nil {
		return nil, errors.Wrap(err, "unmarshaling files config")
	}

	return mappings, nil
}

// isExcluded checks if a file is in the exclusion list.
func isExcluded(path string, exclude []string) bool {
	return slices.Contains(exclude, path)
}

// fetchFileContent fetches file content from a repository.
func fetchFileContent(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
	path string,
) ([]byte, error) {
	fileContent, _, _, err := client.Repositories.GetContents(ctx, org, repo, path, nil)
	if err != nil {
		return nil, errors.Wrap(err, "fetching file content")
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, errors.Wrap(err, "decoding file content")
	}

	return []byte(content), nil
}

// fetchTargetFile fetches a file from the target repository.
func fetchTargetFile(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
	path string,
) ([]byte, bool, error) {
	fileContent, _, _, err := client.Repositories.GetContents(ctx, org, repo, path, nil)
	if err != nil {
		if isNotFoundError(err) {
			return nil, false, nil
		}

		return nil, false, errors.Wrap(err, "fetching target file")
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, true, errors.Wrap(err, "decoding target file content")
	}

	return []byte(content), true, nil
}

// isNotFoundError checks if an error is a 404 Not Found error.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) {
		return ghErr.Response.StatusCode == httpStatusNotFound
	}

	return false
}

// checkNonStandardRenovateConfigs checks for non-standard renovate config files.
func checkNonStandardRenovateConfigs(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	stats *FileSyncStats,
) []FileChange {
	nonStandardPaths := []string{
		".github/renovate.json",
		".github/renovate.json5",
		"renovate.json5",
		".renovaterc",
		".renovaterc.json",
		".renovaterc.json5",
	}

	var changes []FileChange

	for _, path := range nonStandardPaths {
		_, exists, err := fetchTargetFile(ctx, client, org, repo, path)
		if err != nil {
			log.Warn("failed to check non-standard renovate config", "path", path, "error", err)

			continue
		}

		if exists {
			log.Info("scheduling deletion of non-standard renovate config", "path", path)

			stats.Deleted++
			stats.DeletedFiles = append(stats.DeletedFiles, path)

			changes = append(changes, FileChange{
				Path:   path,
				Action: "delete",
			})
		}
	}

	return changes
}

// hasManualModifications checks if a file has manual modifications.
func hasManualModifications(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
	path string,
) (bool, error) {
	// Get commit history for the file
	opts := &github.CommitsListOptions{
		Path: path,
		ListOptions: github.ListOptions{
			PerPage: commitsPerPageForFile,
		},
	}

	commits, _, err := client.Repositories.ListCommits(ctx, org, repo, opts)
	if err != nil {
		return false, errors.Wrap(err, "listing commits")
	}

	// Check if any commits are not from sync workflow
	for _, commit := range commits {
		message := commit.GetCommit().GetMessage()
		if !strings.HasPrefix(message, "chore(sync):") {
			return true, nil
		}
	}

	return false, nil
}

// closeExistingPR closes an existing PR if it exists.
func closeExistingPR(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	branchPrefix string,
	comment string,
) error {
	branchName := getBranchName(repo, branchPrefix)

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

	if _, _, errEdit := client.PullRequests.Edit(ctx, org, repo, prNumber, pr); errEdit != nil {
		return errors.Wrap(errEdit, "closing PR")
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

// getBranchName generates the branch name from repo name and prefix.
func getBranchName(repo string, branchPrefix string) string {
	// Strip leading dot from repo name
	repoSanitized := strings.TrimPrefix(repo, ".")

	return fmt.Sprintf("%s/%s", branchPrefix, repoSanitized)
}

// createOrUpdatePR creates or updates a PR with the file changes.
func createOrUpdatePR(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	sourceRepo string,
	defaultBranch string,
	baseSHA string,
	branchPrefix string,
	prLabels []string,
	changes []FileChange,
	stats *FileSyncStats,
) error {
	branchName := getBranchName(repo, branchPrefix)
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
	prNumber, err := upsertPullRequest(
		ctx, log, client, org, repo, sourceRepo, defaultBranch, branchName, stats,
	)
	if err != nil {
		return errors.Wrap(err, "upserting pull request")
	}

	// Add labels and enable auto-merge
	return finalizePR(ctx, log, client, org, repo, prNumber, prLabels, stats)
}

// ensureBranchExists creates the branch if it doesn't exist.
func ensureBranchExists(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	branchName string,
	baseSHA string,
) error {
	// Check if branch exists
	_, existingRef, err := client.Git.GetRef(ctx, org, repo, "heads/"+branchName)
	branchExists := err == nil && existingRef != nil

	// If branch exists, check for merged PR and delete if merged
	if branchExists {
		if mergeErr := handleMergedPR(ctx, log, client, org, repo, branchName); mergeErr != nil {
			log.Warn("failed to handle merged PR", "error", mergeErr)
		}

		// Check again if branch still exists after handling merged PR
		_, existingRef, err = client.Git.GetRef(ctx, org, repo, "heads/"+branchName)
		branchExists = err == nil && existingRef != nil
	}

	// Create branch if it doesn't exist
	if !branchExists {
		log.Debug("creating branch", "branch", branchName)

		ref := github.CreateRef{
			Ref: "refs/heads/" + branchName,
			SHA: baseSHA,
		}

		_, _, createErr := client.Git.CreateRef(ctx, org, repo, ref)
		if createErr != nil {
			return errors.Wrap(createErr, "creating branch")
		}
	}

	return nil
}

// createGitCommit creates blobs, tree, and commit for the changes.
func createGitCommit(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	branchName string,
	baseSHA string,
	changes []FileChange,
) error {
	// Create blobs for all files
	log.Debug("creating blobs", "count", len(changes))

	for i := range changes {
		if changes[i].Action == "delete" {
			continue
		}

		blob := github.Blob{
			Content:  github.Ptr(base64.StdEncoding.EncodeToString(changes[i].Content)),
			Encoding: github.Ptr("base64"),
		}

		createdBlob, _, blobErr := client.Git.CreateBlob(ctx, org, repo, blob)
		if blobErr != nil {
			return errors.Wrapf(blobErr, "creating blob for %s", changes[i].Path)
		}

		changes[i].BlobSHA = createdBlob.GetSHA()
	}

	// Get base tree
	baseCommit, _, err := client.Git.GetCommit(ctx, org, repo, baseSHA)
	if err != nil {
		return errors.Wrap(err, "getting base commit")
	}

	baseTreeSHA := baseCommit.GetTree().GetSHA()

	// Build tree entries
	treeEntries := buildTreeEntries(changes)

	// Create tree
	log.Debug("creating tree")

	tree, _, err := client.Git.CreateTree(ctx, org, repo, baseTreeSHA, treeEntries)
	if err != nil {
		return errors.Wrap(err, "creating tree")
	}

	// Create commit
	log.Debug("creating commit")

	commitMessage := "chore(sync): sync organization files"
	commit := github.Commit{
		Message: github.Ptr(commitMessage),
		Tree:    tree,
		Parents: []*github.Commit{
			{SHA: github.Ptr(baseSHA)},
		},
	}

	newCommit, _, err := client.Git.CreateCommit(ctx, org, repo, commit, nil)
	if err != nil {
		return errors.Wrap(err, "creating commit")
	}

	// Update branch ref
	log.Debug("updating branch ref", "sha", newCommit.GetSHA()[:7])

	updateRef := github.UpdateRef{
		SHA:   newCommit.GetSHA(),
		Force: github.Ptr(true),
	}

	_, _, err = client.Git.UpdateRef(ctx, org, repo, "heads/"+branchName, updateRef)
	if err != nil {
		return errors.Wrap(err, "updating branch ref")
	}

	return nil
}

// buildTreeEntries builds tree entries from file changes.
func buildTreeEntries(changes []FileChange) []*github.TreeEntry {
	treeEntries := make([]*github.TreeEntry, 0, len(changes))

	for _, change := range changes {
		if change.Action == "delete" {
			treeEntries = append(treeEntries, &github.TreeEntry{
				Path: github.Ptr(change.Path),
				Mode: github.Ptr("100644"),
				Type: github.Ptr("blob"),
				SHA:  nil,
			})
		} else {
			treeEntries = append(treeEntries, &github.TreeEntry{
				Path: github.Ptr(change.Path),
				Mode: github.Ptr("100644"),
				Type: github.Ptr("blob"),
				SHA:  github.Ptr(change.BlobSHA),
			})
		}
	}

	return treeEntries
}

// upsertPullRequest creates or updates a pull request.
func upsertPullRequest(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	sourceRepo string,
	defaultBranch string,
	branchName string,
	stats *FileSyncStats,
) (int, error) {
	prTitle := "chore(sync): sync organization files"
	prBody := buildPRBody(org, sourceRepo, stats)

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

// finalizePR adds labels and enables auto-merge for a PR.
func finalizePR(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	prNumber int,
	prLabels []string,
	stats *FileSyncStats,
) error {
	// Add labels (include review/destructive if deletions present)
	labels := prLabels
	if stats.Deleted > 0 {
		labels = append(labels, "review/destructive")
	}

	if len(labels) > 0 {
		_, _, err := client.Issues.AddLabelsToIssue(ctx, org, repo, prNumber, labels)
		if err != nil {
			log.Warn("failed to add labels to PR", "error", err)
		}
	}

	// Enable auto-merge if no deletions
	if stats.Deleted == 0 {
		if err := enableAutoMerge(ctx, log, client, org, repo, prNumber); err != nil {
			log.Warn("failed to enable auto-merge", "error", err)
		}
	} else {
		log.Info("skipping auto-merge due to deletions")
	}

	return nil
}

// handleMergedPR handles a branch with a merged PR.
func handleMergedPR(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	branchName string,
) error {
	// Check for merged PRs
	prs, _, err := client.PullRequests.List(ctx, org, repo, &github.PullRequestListOptions{
		State: "closed",
		Head:  org + ":" + branchName,
	})
	if err != nil {
		return errors.Wrap(err, "listing closed PRs")
	}

	for _, pr := range prs {
		if pr.GetMerged() {
			log.Info("deleting branch with merged PR", "pr", pr.GetNumber())

			_, err := client.Git.DeleteRef(ctx, org, repo, "heads/"+branchName)
			if err != nil {
				return errors.Wrap(err, "deleting branch")
			}

			return nil
		}
	}

	return nil
}

// buildPRBody builds the PR body text.
func buildPRBody(org string, sourceRepo string, stats *FileSyncStats) string {
	var body strings.Builder

	body.WriteString(fmt.Sprintf(
		"Syncs organization files from [`%s/%s`](https://github.com/%s/%s).\n",
		org, sourceRepo, org, sourceRepo,
	))

	// Files created section
	if len(stats.CreatedFiles) > 0 {
		body.WriteString("\n## Files Created\n\n")

		for _, file := range stats.CreatedFiles {
			body.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
	}

	// Files updated section
	if len(stats.UpdatedFiles) > 0 {
		body.WriteString("\n## Files Updated\n\n")

		for _, file := range stats.UpdatedFiles {
			body.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
	}

	// Deletion alert
	if len(stats.DeletedFiles) > 0 {
		body.WriteString("\n> [!CAUTION]\n")
		body.WriteString("> **Files are being deleted in this sync.**\n")
		body.WriteString(">\n")
		body.WriteString("> The following non-standard Renovate config files are being removed\n")
		body.WriteString("> in favor of the organization standard `renovate.json` in root:\n")
		body.WriteString(">\n")

		for _, file := range stats.DeletedFiles {
			body.WriteString(fmt.Sprintf("> - `%s`\n", file))
		}

		body.WriteString(">\n")
		body.WriteString("> **This PR requires manual review before merging.**\n")
	}

	body.WriteString("\n---\n\n")
	body.WriteString("*This PR was automatically created by the org file sync workflow*\n")

	return body.String()
}

// enableAutoMerge enables auto-merge for a PR.
func enableAutoMerge(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	prNumber int,
) error {
	log.Debug("enabling auto-merge", "pr", prNumber)

	// Get PR node ID
	pr, _, err := client.PullRequests.Get(ctx, org, repo, prNumber)
	if err != nil {
		return errors.Wrap(err, "getting PR")
	}

	nodeID := pr.GetNodeID()

	// GraphQL mutation to enable auto-merge
	mutation := `
mutation($prId: ID!) {
  enablePullRequestAutoMerge(input: {
    pullRequestId: $prId,
    mergeMethod: SQUASH
  }) {
    pullRequest {
      autoMergeRequest {
        enabledAt
      }
    }
  }
}`

	variables := map[string]any{
		"prId": nodeID,
	}

	input := struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}{
		Query:     mutation,
		Variables: variables,
	}

	req, err := client.NewRequest("POST", "graphql", input)
	if err != nil {
		return errors.Wrap(err, "creating GraphQL request")
	}

	var result any

	_, err = client.Do(ctx, req, &result)
	if err != nil {
		return errors.Wrap(err, "executing GraphQL mutation")
	}

	log.Debug("auto-merge enabled")

	return nil
}

// logFileChanges logs the planned file changes in dry-run mode.
func logFileChanges(log *logger.Logger, stats *FileSyncStats) {
	if len(stats.CreatedFiles) > 0 {
		log.Info("files to create:")

		for _, file := range stats.CreatedFiles {
			log.Info("  + " + file)
		}
	}

	if len(stats.UpdatedFiles) > 0 {
		log.Info("files to update:")

		for _, file := range stats.UpdatedFiles {
			log.Info("  ~ " + file)
		}
	}

	if len(stats.DeletedFiles) > 0 {
		log.Info("files to delete:")

		for _, file := range stats.DeletedFiles {
			log.Info("  - " + file)
		}
	}

	if stats.Created+stats.Updated+stats.Deleted == 0 {
		log.Info("no file changes needed")
	}
}
