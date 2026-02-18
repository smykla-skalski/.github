package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/go-github/v80/github"

	"github.com/smykla-skalski/.github/internal/configtypes"
	"github.com/smykla-skalski/.github/pkg/config"
	"github.com/smykla-skalski/.github/pkg/logger"
	"github.com/smykla-skalski/.github/pkg/merge"
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
	MergedFiles      map[string]string // path -> strategy
}

// SyncFiles synchronizes files from a central repo to a target repository.
//
//nolint:funlen // TODO: refactor to reduce function length
func SyncFiles(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	sourceRepo string,
	filesConfig string,
	syncConfig *configtypes.SyncConfig,
	branchPrefix string,
	prLabels []string,
	dryRun bool,
) (*FilesSyncResult, error) {
	result := NewFilesSyncResult(repo, dryRun)

	// Check if sync is skipped
	if syncConfig.Sync.Skip || syncConfig.Sync.Files.Skip {
		log.Info("file sync skipped by config")

		// Check for existing PR to close
		if err := closeExistingPR(ctx, log, client, org, repo, branchPrefix,
			"File synchronization is disabled for this repository"); err != nil {
			log.Warn("failed to close existing PR", "error", err)
		}

		result.CompleteSkipped("sync disabled by config")

		return result, nil
	}

	// Parse files config
	fileMappings, err := parseFilesConfig(filesConfig)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "parsing files config"))

		return result, err
	}

	log.Debug("parsed files config", "count", len(fileMappings))

	// Get repository info
	defaultBranch, baseSHA, err := getRepoBaseInfo(ctx, log, client, org, repo)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "getting repository base info"))

		return result, err
	}

	// Process files
	stats := &FileSyncStats{
		MergedFiles: make(map[string]string),
	}

	var changes []FileChange

	for _, mapping := range fileMappings {
		fileChanges := processFileMapping(
			ctx, log, client, org, repo, sourceRepo, defaultBranch, mapping, syncConfig, stats,
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

	// Populate result from stats
	result.CreatedFiles = stats.CreatedFiles
	result.UpdatedFiles = stats.UpdatedFiles
	result.DeletedFiles = stats.DeletedFiles
	result.HasDeletionsWarn = len(stats.DeletedFiles) > 0

	// If no changes, close any existing PR
	if len(changes) == 0 {
		log.Info("no changes needed")

		if closeErr := closeExistingPR(ctx, log, client, org, repo, branchPrefix,
			"All files are now in sync. Closing this PR."); closeErr != nil {
			log.Warn("failed to close existing PR", "error", closeErr)
		}

		result.Complete(StatusSuccess)

		return result, nil
	}

	if dryRun {
		log.Info("dry-run mode: skipping PR creation")
		logFileChanges(log, stats)
		result.Complete(StatusSuccess)

		return result, nil
	}

	// Create/update PR
	prNumber, prURL, err := createOrUpdatePRWithResult(
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
	)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "creating or updating PR"))

		return result, err
	}

	result.PRNumber = prNumber
	result.PRURL = prURL

	log.Info("file sync completed successfully")
	result.Complete(StatusSuccess)

	return result, nil
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
	defaultBranch string,
	mapping FileMapping,
	syncConfig *configtypes.SyncConfig,
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

	// Apply template replacements
	sourceContent = renderFileTemplate(sourceContent, defaultBranch)

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

	// Check for merge configuration
	mergeConfig := config.GetMergeConfig(syncConfig, mapping.Dest)

	var mergeStrategy configtypes.MergeStrategy

	if mergeConfig != nil {
		log.Info(
			"found merge config for file",
			"file", mapping.Dest,
			"strategy", mergeConfig.Strategy,
		)

		// Apply merge
		mergedContent, mergeErr := applyMerge(
			log, sourceContent, mapping.Dest, mergeConfig,
		)
		if mergeErr != nil {
			log.Warn("failed to apply merge, falling back to replacement",
				"file", mapping.Dest, "error", mergeErr)
		} else {
			// Use merged content
			sourceContent = mergedContent
			mergeStrategy = mergeConfig.Strategy
		}
	}

	if targetExists {
		return processExistingFile(
			ctx,
			log,
			client,
			org,
			repo,
			mapping,
			sourceContent,
			targetContent,
			mergeStrategy,
			stats,
			changes,
		)
	}

	return processNewFile(log, mapping, sourceContent, mergeStrategy, stats, changes)
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
	mergeStrategy configtypes.MergeStrategy,
	stats *FileSyncStats,
	changes []FileChange,
) []FileChange {
	// File exists - check if update needed
	if string(sourceContent) == string(targetContent) {
		log.Debug("file already up to date", "file", mapping.Dest)

		stats.Skipped++

		return changes
	}

	// Special case: renovate.json - check for manual modifications (only if not merged)
	if mapping.Dest == "renovate.json" && mergeStrategy == "" {
		if shouldSkipRenovateJSON(ctx, log, client, org, repo, mapping.Dest, stats) {
			return changes
		}
	}

	log.Debug("file needs update", "file", mapping.Dest)

	stats.Updated++
	stats.UpdatedFiles = append(stats.UpdatedFiles, mapping.Dest)

	if mergeStrategy != "" {
		stats.MergedFiles[mapping.Dest] = string(mergeStrategy)
	}

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
	mergeStrategy configtypes.MergeStrategy,
	stats *FileSyncStats,
	changes []FileChange,
) []FileChange {
	log.Debug("will create file", "file", mapping.Dest)

	stats.Created++
	stats.CreatedFiles = append(stats.CreatedFiles, mapping.Dest)

	if mergeStrategy != "" {
		stats.MergedFiles[mapping.Dest] = string(mergeStrategy)
	}

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

// filterOutMerged filters out files that appear in the merged map.
// Used to prevent duplicate listings in PR body sections.
func filterOutMerged(files []string, merged map[string]string) []string {
	result := make([]string, 0, len(files))

	for _, f := range files {
		if _, isMerged := merged[f]; !isMerged {
			result = append(result, f)
		}
	}

	return result
}

// applyMerge applies merge configuration to file content.
// It uses the org template (sourceContent) as base and applies configured overrides.
func applyMerge(
	log *logger.Logger,
	sourceContent []byte,
	path string,
	mergeConfig *configtypes.FileMergeConfig,
) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(path))

	// Handle markdown files
	if ext == ".md" || ext == ".markdown" {
		return applyMarkdownMerge(log, sourceContent, path, mergeConfig)
	}

	// Reject markdown strategy for non-markdown files
	if mergeConfig.Strategy == configtypes.MergeStrategyMarkdown {
		return nil, errors.Wrapf(
			merge.ErrMergeUnknownStrategy,
			"strategy %q is only valid for markdown files, not %s",
			mergeConfig.Strategy, ext,
		)
	}

	return applyStructuredMerge(log, sourceContent, path, ext, mergeConfig)
}

// applyMarkdownMerge applies markdown section merge to file content.
func applyMarkdownMerge(
	log *logger.Logger,
	sourceContent []byte,
	path string,
	mergeConfig *configtypes.FileMergeConfig,
) ([]byte, error) {
	// For markdown files, only "markdown" or empty strategy is valid
	if mergeConfig.Strategy != "" && mergeConfig.Strategy != configtypes.MergeStrategyMarkdown {
		return nil, errors.Wrapf(
			merge.ErrMergeUnknownStrategy,
			"markdown files only support %q strategy, got %q",
			configtypes.MergeStrategyMarkdown, mergeConfig.Strategy,
		)
	}

	result, err := merge.MergeMarkdown(sourceContent, mergeConfig.Sections)
	if err != nil {
		return nil, errors.Wrapf(err, "merging markdown file %s", path)
	}

	log.Debug("applied markdown merge", "file", path, "sections", len(mergeConfig.Sections))

	return result, nil
}

// applyStructuredMerge applies JSON/YAML merge to file content.
func applyStructuredMerge(
	log *logger.Logger,
	sourceContent []byte,
	path string,
	ext string,
	mergeConfig *configtypes.FileMergeConfig,
) ([]byte, error) {
	var (
		parseFunc   func([]byte) (map[string]any, error)
		marshalFunc func(map[string]any) ([]byte, error)
		mergeFunc   func(map[string]any, map[string]any, configtypes.MergeStrategy, *merge.MergeOptions) (map[string]any, error)
		isJSON      bool
	)

	switch ext {
	case ".json":
		parseFunc = merge.ParseJSON
		marshalFunc = merge.MarshalJSON
		mergeFunc = merge.MergeJSON
		isJSON = true
	case ".yml", ".yaml":
		parseFunc = merge.ParseYAML
		marshalFunc = merge.MarshalYAML
		mergeFunc = merge.MergeYAML
		isJSON = false
	default:
		return nil, errors.Wrapf(
			merge.ErrMergeUnsupportedFileType,
			"file: %s, extension: %s",
			path,
			ext,
		)
	}

	// Parse source (org template) - always use as base to inherit org updates
	sourceMap, err := parseFunc(sourceContent)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing source file %s", path)
	}

	// Construct merge options from config
	var opts *merge.MergeOptions
	if len(mergeConfig.ArrayStrategies) > 0 || mergeConfig.DeduplicateArrays {
		opts = &merge.MergeOptions{
			ArrayStrategies:   mergeConfig.ArrayStrategies,
			DeduplicateArrays: mergeConfig.DeduplicateArrays,
		}

		log.Debug(
			"applying array merge strategies",
			"file", path,
			"strategies", mergeConfig.ArrayStrategies,
			"deduplicate", mergeConfig.DeduplicateArrays,
		)
	}

	// Apply merge: org template (base) + configured overrides
	// This ensures repos always get org template updates while preserving their custom overrides
	var result map[string]any

	result, err = mergeFunc(sourceMap, mergeConfig.Overrides, mergeConfig.Strategy, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "merging file %s", path)
	}

	// Marshal back to bytes
	var mergedContent []byte

	mergedContent, err = marshalFunc(result)
	if err != nil {
		return nil, errors.Wrapf(err, "marshaling merged file %s", path)
	}

	// For JSON, add newline at end for better git diffs
	if isJSON {
		mergedContent = append(mergedContent, '\n')
	}

	log.Debug("applied merge", "file", path, "strategy", mergeConfig.Strategy)

	return mergedContent, nil
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

// createOrUpdatePRWithResult creates or updates a PR and returns PR number and URL.
func createOrUpdatePRWithResult(
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
) (int, string, error) {
	branchName := getBranchName(repo, branchPrefix)
	log.Info("creating/updating PR", "branch", branchName)

	// Ensure branch exists
	if err := ensureBranchExists(ctx, log, client, org, repo, branchName, baseSHA); err != nil {
		return 0, "", errors.Wrap(err, "ensuring branch exists")
	}

	// Create Git commit
	if err := createGitCommit(
		ctx,
		log,
		client,
		org,
		repo,
		branchName,
		baseSHA,
		changes,
	); err != nil {
		return 0, "", errors.Wrap(err, "creating Git commit")
	}

	// Create or update pull request
	prNumber, prURL, err := upsertPullRequestWithURL(
		ctx, log, client, org, repo, sourceRepo, defaultBranch, branchName, stats,
	)
	if err != nil {
		return 0, "", errors.Wrap(err, "upserting pull request")
	}

	// Add labels and enable auto-merge
	if err := finalizePR(ctx, log, client, org, repo, prNumber, prLabels, stats); err != nil {
		return prNumber, prURL, err
	}

	return prNumber, prURL, nil
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

// upsertPullRequestWithURL creates or updates a pull request and returns number and URL.
func upsertPullRequestWithURL(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	sourceRepo string,
	defaultBranch string,
	branchName string,
	stats *FileSyncStats,
) (int, string, error) {
	prTitle := "chore(sync): sync organization files"
	prBody := buildPRBody(org, sourceRepo, stats)

	// Check for existing PR
	prs, _, err := client.PullRequests.List(ctx, org, repo, &github.PullRequestListOptions{
		State: "open",
		Head:  org + ":" + branchName,
	})
	if err != nil {
		return 0, "", errors.Wrap(err, "listing PRs")
	}

	if len(prs) > 0 {
		// Update existing PR
		prNumber := prs[0].GetNumber()
		prURL := prs[0].GetHTMLURL()

		log.Info("updating existing PR", "pr", prNumber)

		pr := &github.PullRequest{
			Title: github.Ptr(prTitle),
			Body:  github.Ptr(prBody),
		}

		_, _, editErr := client.PullRequests.Edit(ctx, org, repo, prNumber, pr)
		if editErr != nil {
			return 0, "", errors.Wrap(editErr, "updating PR")
		}

		return prNumber, prURL, nil
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
		return 0, "", errors.Wrap(err, "creating PR")
	}

	prNumber := createdPR.GetNumber()
	prURL := createdPR.GetHTMLURL()
	log.Info("created PR", "pr", prNumber, "url", prURL)

	return prNumber, prURL, nil
}

// finalizePR adds labels and enables auto-merge for a PR.
//
//nolint:unparam // Error return kept for consistency with similar functions
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

	// Files merged section (show first since these are customized)
	if len(stats.MergedFiles) > 0 {
		body.WriteString("\n## Files Merged with Configured Overrides\n\n")

		// Sort files for deterministic output
		mergedPaths := make([]string, 0, len(stats.MergedFiles))
		for file := range stats.MergedFiles {
			mergedPaths = append(mergedPaths, file)
		}

		slices.Sort(mergedPaths)

		for _, file := range mergedPaths {
			strategy := stats.MergedFiles[file]
			body.WriteString(fmt.Sprintf("- `%s` (%s strategy)\n", file, strategy))
		}
	}

	// Files created section (exclude merged files)
	createdNonMerged := filterOutMerged(stats.CreatedFiles, stats.MergedFiles)
	if len(createdNonMerged) > 0 {
		body.WriteString("\n## Files Created\n\n")

		for _, file := range createdNonMerged {
			body.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
	}

	// Files updated section (exclude merged files)
	updatedNonMerged := filterOutMerged(stats.UpdatedFiles, stats.MergedFiles)
	if len(updatedNonMerged) > 0 {
		body.WriteString("\n## Files Updated\n\n")

		for _, file := range updatedNonMerged {
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
	logFilesWithPrefix(log, "files to create:", "+", stats.CreatedFiles)
	logFilesWithPrefix(log, "files to update:", "~", stats.UpdatedFiles)
	logFilesWithPrefix(log, "files to delete:", "-", stats.DeletedFiles)

	if stats.Created+stats.Updated+stats.Deleted == 0 {
		log.Info("no file changes needed")
	}
}

// logFilesWithPrefix logs a list of files with a header and prefix symbol.
func logFilesWithPrefix(log *logger.Logger, header string, prefix string, files []string) {
	if len(files) == 0 {
		return
	}

	log.Info(header)

	for _, file := range files {
		log.Info("  " + prefix + " " + file)
	}
}

// renderFileTemplate replaces template placeholders in file content.
//
// Supported placeholders:
//   - {{DEFAULT_BRANCH}} - replaced with the target repository's default branch
//
// Replacement is case-sensitive and exact-match only. If a placeholder is not
// found, the content is returned unchanged. Multiple occurrences of the same
// placeholder are all replaced. If defaultBranch is empty, content is returned
// unchanged to avoid creating invalid references.
func renderFileTemplate(content []byte, defaultBranch string) []byte {
	if defaultBranch == "" {
		return content
	}

	return bytes.ReplaceAll(content, []byte("{{DEFAULT_BRANCH}}"), []byte(defaultBranch))
}
