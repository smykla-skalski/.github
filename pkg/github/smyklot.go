package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/go-github/v83/github"
	"go.yaml.in/yaml/v4"

	"github.com/smykla-skalski/.github/internal/configtypes"
	"github.com/smykla-skalski/.github/pkg/logger"
)

const (
	smyklotBranchPrefix = "chore/sync-smyklot"
	smyklotPRLabel      = "ci/skip-all"

	// Workflow template names (current)
	WorkflowPrCommands = "smyklot-pr-commands"
	WorkflowPoll       = "smyklot-poll"

	// Legacy workflow names (for migration)
	legacyPrCommands    = "pr-commands"
	legacyPollReactions = "poll-reactions"

	// Header comment that identifies smyklot-managed workflows
	smyklotManagedHeader = "# This file is managed by smykla-skalski/.github org sync."
)

// legacyWorkflowNames maps current workflow names to their legacy equivalents.
var legacyWorkflowNames = map[string]string{
	WorkflowPrCommands: legacyPrCommands,
	WorkflowPoll:       legacyPollReactions,
}

// SmyklotSyncStats tracks smyklot sync statistics.
type SmyklotSyncStats struct {
	Skipped          int
	Installed        int
	InstalledFiles   []string
	Replaced         int
	ReplacedFiles    []string
	VersionOnly      int
	VersionOnlyFiles []string
}

// SyncSmyklot synchronizes smyklot workflows and version references.
//
//nolint:funlen // TODO: refactor to reduce function complexity
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
) (*SmyklotSyncResult, error) {
	result := NewSmyklotSyncResult(repo, dryRun)

	// Check if sync is skipped
	if syncConfig.Sync.Skip || syncConfig.Sync.Smyklot.Skip {
		err := handleSkippedSyncWithResult(ctx, log, client, org, repo, syncConfig, result)

		return result, err
	}

	// Fetch org-level smyklot config
	orgConfig, err := fetchSmyklotOrgConfig(ctx, client, org, smyklotFilePath)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "fetching org smyklot config"))

		return result, err
	}

	// Get repository info
	defaultBranch, baseSHA, err := getRepoBaseInfo(ctx, log, client, org, repo)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "getting repository base info"))

		return result, err
	}

	// List existing workflow files
	workflowFiles, err := listWorkflowFiles(ctx, log, client, org, repo)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "listing workflow files"))

		return result, err
	}

	// Build map of existing workflows (name without extension -> full path)
	existingWorkflows := buildExistingWorkflowsMap(workflowFiles)

	// Process workflow templates
	stats := &SmyklotSyncStats{}

	changes, err := syncManagedWorkflows(
		ctx, log, client, org, repo, tag, sha,
		orgConfig, syncConfig, templatesDir, existingWorkflows, stats,
	)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "syncing managed workflows"))

		return result, err
	}

	// Version-only sync for other workflows if enabled
	versionChanges, err := syncVersionOnlyWorkflows(
		ctx, log, client, org, repo, version, tag,
		orgConfig, syncConfig, workflowFiles, stats,
	)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "syncing version-only workflows"))

		return result, err
	}

	changes = append(changes, versionChanges...)

	// Log stats
	log.Info("smyklot sync summary",
		"installed", stats.Installed,
		"replaced", stats.Replaced,
		"version_only", stats.VersionOnly,
		"skipped", stats.Skipped,
	)

	// Populate result from stats
	result.InstalledFiles = stats.InstalledFiles
	result.ReplacedFiles = stats.ReplacedFiles
	result.VersionOnlyFiles = stats.VersionOnlyFiles

	// If no changes, close any existing PR
	if len(changes) == 0 {
		log.Info("no changes needed")

		if closeErr := closeSmyklotPR(ctx, log, client, org, repo,
			"smyklot is already up to date. Closing this PR."); closeErr != nil {
			log.Warn("failed to close existing PR", "error", closeErr)
		}

		result.Complete(StatusSuccess)

		return result, nil
	}

	if dryRun {
		log.Info("dry-run mode: skipping PR creation")
		logSmyklotChanges(log, stats)
		result.Complete(StatusSuccess)

		return result, nil
	}

	// Create/update PR
	prNumber, prURL, err := createSmyklotPRWithResult(
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
	)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "creating or updating PR"))

		return result, err
	}

	result.PRNumber = prNumber
	result.PRURL = prURL

	log.Info("smyklot sync completed successfully")
	result.Complete(StatusSuccess)

	return result, nil
}

// getSkipReason returns the reason for skipping smyklot sync.
func getSkipReason(syncConfig *configtypes.SyncConfig) string {
	if syncConfig.Sync.Skip {
		return "smyklot version synchronization is disabled for this repository (sync.skip=true)"
	}

	return "smyklot version synchronization is disabled for this repository (sync.smyklot.skip=true)"
}

// handleSkippedSyncWithResult handles the case when smyklot sync is skipped and updates result.
//
//nolint:unparam // Error return kept for consistency with sync function pattern
func handleSkippedSyncWithResult(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	syncConfig *configtypes.SyncConfig,
	result *SmyklotSyncResult,
) error {
	log.Info("smyklot sync skipped by config")

	skipReason := getSkipReason(syncConfig)
	if err := closeSmyklotPR(ctx, log, client, org, repo, skipReason); err != nil {
		log.Warn("failed to close existing PR", "error", err)
	}

	result.CompleteSkipped(skipReason)

	return nil
}

// buildExistingWorkflowsMap creates a map of workflow names to their full paths.
func buildExistingWorkflowsMap(workflowFiles []string) map[string]string {
	existingWorkflows := make(map[string]string)

	for _, path := range workflowFiles {
		filename := filepath.Base(path)
		nameWithoutExt := strings.TrimSuffix(strings.TrimSuffix(filename, ".yml"), ".yaml")
		existingWorkflows[nameWithoutExt] = path
	}

	return existingWorkflows
}

// syncManagedWorkflows syncs the managed workflow templates (pr-commands, poll-reactions).
func syncManagedWorkflows(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	tag string,
	sha string,
	orgConfig *configtypes.SmyklotFile,
	syncConfig *configtypes.SyncConfig,
	templatesDir string,
	existingWorkflows map[string]string,
	stats *SmyklotSyncStats,
) ([]FileChange, error) {
	var changes []FileChange

	workflowNames := []string{WorkflowPrCommands, WorkflowPoll}

	for _, workflowName := range workflowNames {
		if !shouldSyncWorkflow(orgConfig, syncConfig, workflowName) {
			log.Debug("workflow sync disabled by config", "workflow", workflowName)

			continue
		}

		workflowChanges, err := syncSingleManagedWorkflow(
			ctx, log, client, org, repo, workflowName, tag, sha,
			templatesDir, existingWorkflows, stats,
		)
		if err != nil {
			return nil, errors.Wrapf(err, "syncing workflow %s", workflowName)
		}

		changes = append(changes, workflowChanges...)
	}

	return changes, nil
}

// syncSingleManagedWorkflow syncs a single managed workflow template.
func syncSingleManagedWorkflow(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	workflowName string,
	tag string,
	sha string,
	templatesDir string,
	existingWorkflows map[string]string,
	stats *SmyklotSyncStats,
) ([]FileChange, error) {
	templateContent, err := readWorkflowTemplate(templatesDir, workflowName)
	if err != nil {
		return nil, errors.Wrapf(err, "reading workflow template %s", workflowName)
	}

	expectedContent := renderWorkflowTemplate(templateContent, tag, sha)
	targetPath := ".github/workflows/" + workflowName + ".yml"
	existingPath, exists := existingWorkflows[workflowName]

	// Check for legacy workflow name if current doesn't exist
	if !exists {
		legacyChanges, legacyErr := handleLegacyWorkflow(
			ctx, log, client, org, repo, workflowName, targetPath,
			expectedContent, existingWorkflows, stats,
		)
		if legacyErr != nil {
			return nil, legacyErr
		}

		if legacyChanges != nil {
			return legacyChanges, nil
		}

		return handleNewWorkflow(log, workflowName, targetPath, expectedContent, stats), nil
	}

	return handleExistingWorkflow(
		ctx, log, client, org, repo, workflowName,
		existingPath, targetPath, expectedContent, stats,
	)
}

// handleNewWorkflow handles installing a new workflow that doesn't exist.
func handleNewWorkflow(
	log *logger.Logger,
	workflowName string,
	targetPath string,
	expectedContent []byte,
	stats *SmyklotSyncStats,
) []FileChange {
	log.Debug("workflow not found, will install", "workflow", workflowName)

	stats.Installed++
	stats.InstalledFiles = append(stats.InstalledFiles, targetPath)

	return []FileChange{{
		Path:    targetPath,
		Content: expectedContent,
		Action:  "create",
	}}
}

// handleLegacyWorkflow checks for and migrates legacy-named workflows.
// Returns (nil, nil) if no legacy workflow found or if it's not smyklot-managed.
func handleLegacyWorkflow(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	workflowName string,
	targetPath string,
	expectedContent []byte,
	existingWorkflows map[string]string,
	stats *SmyklotSyncStats,
) ([]FileChange, error) {
	legacyName, hasLegacy := legacyWorkflowNames[workflowName]
	if !hasLegacy {
		return nil, nil
	}

	legacyPath, legacyExists := existingWorkflows[legacyName]
	if !legacyExists {
		return nil, nil
	}

	log.Debug("found legacy workflow", "legacy", legacyName, "current", workflowName)

	// Fetch legacy workflow content to verify it's smyklot-managed
	fileContent, _, _, err := client.Repositories.GetContents(ctx, org, repo, legacyPath, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching legacy workflow content from %s", legacyPath)
	}

	legacyContent, err := fileContent.GetContent()
	if err != nil {
		return nil, errors.Wrapf(err, "decoding legacy workflow content from %s", legacyPath)
	}

	if !strings.Contains(legacyContent, smyklotManagedHeader) {
		log.Info("legacy workflow exists but is not smyklot-managed, skipping migration",
			"path", legacyPath)

		return nil, nil
	}

	log.Info("migrating legacy workflow to new name",
		"from", legacyPath, "to", targetPath)

	stats.Replaced++
	stats.ReplacedFiles = append(stats.ReplacedFiles, targetPath)

	return []FileChange{
		{
			Path:   legacyPath,
			Action: "delete",
		},
		{
			Path:    targetPath,
			Content: expectedContent,
			Action:  "create",
		},
	}, nil
}

// handleExistingWorkflow handles updating an existing workflow.
func handleExistingWorkflow(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	workflowName string,
	existingPath string,
	targetPath string,
	expectedContent []byte,
	stats *SmyklotSyncStats,
) ([]FileChange, error) {
	fileContent, _, _, fetchErr := client.Repositories.GetContents(
		ctx, org, repo, existingPath, nil,
	)
	if fetchErr != nil {
		return nil, errors.Wrapf(fetchErr, "fetching existing workflow from %s", existingPath)
	}

	existingContent, decodeErr := fileContent.GetContent()
	if decodeErr != nil {
		return nil, errors.Wrapf(decodeErr, "decoding existing workflow from %s", existingPath)
	}

	needsExtensionFix := strings.HasSuffix(existingPath, ".yaml")
	contentMatches := existingContent == string(expectedContent)

	switch {
	case needsExtensionFix:
		return handleExtensionNormalization(
			log, workflowName, existingPath, targetPath, expectedContent, stats,
		), nil

	case !contentMatches:
		return handleContentUpdate(log, workflowName, targetPath, expectedContent, stats), nil

	default:
		log.Debug("workflow matches template", "workflow", workflowName)

		stats.Skipped++

		return nil, nil
	}
}

// handleExtensionNormalization handles renaming .yaml to .yml.
func handleExtensionNormalization(
	log *logger.Logger,
	workflowName string,
	existingPath string,
	targetPath string,
	expectedContent []byte,
	stats *SmyklotSyncStats,
) []FileChange {
	log.Debug("normalizing workflow extension from .yaml to .yml", "workflow", workflowName)

	stats.Replaced++
	stats.ReplacedFiles = append(stats.ReplacedFiles, targetPath)

	return []FileChange{
		{Path: existingPath, Action: "delete"},
		{Path: targetPath, Content: expectedContent, Action: "create"},
	}
}

// handleContentUpdate handles updating workflow content.
func handleContentUpdate(
	log *logger.Logger,
	workflowName string,
	targetPath string,
	expectedContent []byte,
	stats *SmyklotSyncStats,
) []FileChange {
	log.Debug("workflow differs from template, will replace", "workflow", workflowName)

	stats.Replaced++
	stats.ReplacedFiles = append(stats.ReplacedFiles, targetPath)

	return []FileChange{{
		Path:    targetPath,
		Content: expectedContent,
		Action:  "update",
	}}
}

// syncVersionOnlyWorkflows syncs version references in non-managed workflows.
func syncVersionOnlyWorkflows(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	version string,
	tag string,
	orgConfig *configtypes.SmyklotFile,
	syncConfig *configtypes.SyncConfig,
	workflowFiles []string,
	stats *SmyklotSyncStats,
) ([]FileChange, error) {
	// Check repo-level version skip first
	if syncConfig.Sync.Smyklot.Version.Skip {
		log.Debug("version-only sync skipped by repo config (sync.smyklot.version.skip=true)")

		return nil, nil
	}

	// Check org-level sync_version setting
	if orgConfig.SyncVersion == nil || !*orgConfig.SyncVersion || len(workflowFiles) == 0 {
		return nil, nil
	}

	log.Debug("checking for version-only updates")

	var changes []FileChange

	for _, workflowPath := range workflowFiles {
		filename := filepath.Base(workflowPath)
		if filename == WorkflowPrCommands+".yml" || filename == WorkflowPoll+".yml" {
			continue
		}

		change, processed, err := processWorkflowFile(
			ctx, log, client, org, repo, workflowPath, version, tag, stats,
		)
		if err != nil {
			return nil, errors.Wrapf(err, "processing workflow file %s", workflowPath)
		}

		if processed {
			changes = append(changes, change)
		}
	}

	return changes, nil
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
// Returns (FileChange{}, false, nil) if no changes needed, (change, true, nil) if changes needed,
// or (FileChange{}, false, err) if an error occurred.
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
) (FileChange, bool, error) {
	log.Debug("processing workflow file", "path", workflowPath)

	// Fetch file content
	fileContent, _, _, err := client.Repositories.GetContents(ctx, org, repo, workflowPath, nil)
	if err != nil {
		return FileChange{}, false, errors.Wrapf(err, "fetching workflow file %s", workflowPath)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return FileChange{}, false, errors.Wrapf(err, "decoding workflow file %s", workflowPath)
	}

	// Apply version replacements
	updatedContent, changed := applyVersionReplacements(content, version, tag)
	if !changed {
		log.Debug("no smyklot references or already up to date", "file", workflowPath)

		stats.Skipped++

		return FileChange{}, false, nil
	}

	log.Debug("found outdated smyklot references", "file", workflowPath)

	stats.VersionOnly++
	stats.VersionOnlyFiles = append(stats.VersionOnlyFiles, workflowPath)

	return FileChange{
		Path:    workflowPath,
		Content: []byte(updatedContent),
		Action:  "update",
	}, true, nil
}

// applyVersionReplacements applies smyklot version replacements to content.
func applyVersionReplacements(content string, version string, tag string) (string, bool) {
	original := content

	// Pattern 1: GitHub Action reference (uses: smykla-skalski/smyklot@v1.2.3)
	// Anchored to prevent bypass attacks (CWE-020): only matches when followed by
	// whitespace, newline, or end of string - prevents embedding in URLs or with suffixes
	actionPattern := regexp.MustCompile(`(uses:\s*smykla-skalski/smyklot@)v\d+\.\d+\.\d+(\s|$)`)
	content = actionPattern.ReplaceAllString(content, "${1}"+tag+"$2")

	// Pattern 2: Docker image reference (ghcr.io/smykla-skalski/smyklot:1.2.3)
	// Anchored to prevent bypass attacks (CWE-020): only matches when followed by
	// whitespace, newline, or end of string - prevents embedding in URLs or with suffixes
	dockerPattern := regexp.MustCompile(`(ghcr\.io/smykla-skalski/smyklot:)\d+\.\d+\.\d+(\s|$)`)
	content = dockerPattern.ReplaceAllString(content, "${1}"+version+"$2")

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
		State: new("closed"),
	}

	_, _, err = client.PullRequests.Edit(ctx, org, repo, prNumber, pr)
	if err != nil {
		return errors.Wrap(err, "closing PR")
	}

	// Add comment
	prComment := &github.IssueComment{
		Body: new(comment),
	}

	_, _, err = client.Issues.CreateComment(ctx, org, repo, prNumber, prComment)
	if err != nil {
		log.Warn("failed to add PR comment", "error", err)
	}

	return nil
}

// createSmyklotPRWithResult creates or updates a PR and returns PR number and URL.
func createSmyklotPRWithResult(
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
) (int, string, error) {
	branchName := smyklotBranchPrefix
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
	prNumber, prURL, err := upsertSmyklotPullRequestWithURL(
		ctx, log, client, org, repo, defaultBranch, branchName, tag, stats,
	)
	if err != nil {
		return 0, "", errors.Wrap(err, "upserting pull request")
	}

	// Add labels and enable auto-merge
	if err := finalizeSmyklotPR(ctx, log, client, org, repo, prNumber); err != nil {
		return prNumber, prURL, err
	}

	return prNumber, prURL, nil
}

// upsertSmyklotPullRequestWithURL creates or updates a PR and returns number and URL.
func upsertSmyklotPullRequestWithURL(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	defaultBranch string,
	branchName string,
	tag string,
	stats *SmyklotSyncStats,
) (int, string, error) {
	prTitle := buildSmyklotPRTitle(tag, stats)
	prBody := buildSmyklotPRBody(tag, stats)

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
			Title: &prTitle,
			Body:  &prBody,
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
		Title: &prTitle,
		Head:  new(branchName),
		Base:  new(defaultBranch),
		Body:  &prBody,
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

// finalizeSmyklotPR adds labels and enables auto-merge for a smyklot PR.
//
//nolint:unparam // Error return kept for consistency with similar functions
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

// buildSmyklotPRTitle builds the PR title based on what changes are included.
func buildSmyklotPRTitle(tag string, stats *SmyklotSyncStats) string {
	hasWorkflowChanges := stats.Installed > 0 || stats.Replaced > 0
	hasVersionChanges := stats.VersionOnly > 0

	switch {
	case hasVersionChanges && hasWorkflowChanges:
		// Both version and workflow changes
		return "chore(smyklot): update to " + tag + " and sync workflows"

	case hasVersionChanges:
		// Version-only changes
		return "chore(deps): update smyklot to " + tag

	case hasWorkflowChanges:
		// Workflow-only changes (no version bump in other files)
		return "chore(sync): sync smyklot workflows"

	default:
		// Fallback (shouldn't happen if we have changes)
		return "chore(sync): sync smyklot"
	}
}

// buildSmyklotPRBody builds the PR body text for smyklot sync.
func buildSmyklotPRBody(tag string, stats *SmyklotSyncStats) string {
	var body strings.Builder

	hasWorkflowChanges := stats.Installed > 0 || stats.Replaced > 0
	hasVersionChanges := stats.VersionOnly > 0

	switch {
	case hasVersionChanges:
		// Version changes present - mention the update
		fmt.Fprintf(&body,
			"Updates [`smykla-skalski/smyklot`](https://github.com/smykla-skalski/smyklot) "+
				"to version [`%s`](https://github.com/smykla-skalski/smyklot/releases/tag/%s).\n",
			tag, tag,
		)

	case hasWorkflowChanges:
		// Workflow-only changes
		fmt.Fprintf(&body,
			"Syncs smyklot workflow files from "+
				"[`smykla-skalski/smyklot@%s`](https://github.com/smykla-skalski/smyklot/releases/tag/%s).\n",
			tag, tag,
		)

	default:
		body.WriteString("Syncs smyklot configuration.\n")
	}

	// Workflows installed section
	if len(stats.InstalledFiles) > 0 {
		body.WriteString("\n## Workflows Installed\n\n")

		for _, file := range stats.InstalledFiles {
			fmt.Fprintf(&body, "- `%s`\n", file)
		}
	}

	// Workflows replaced section
	if len(stats.ReplacedFiles) > 0 {
		body.WriteString("\n## Workflows Replaced\n\n")

		for _, file := range stats.ReplacedFiles {
			fmt.Fprintf(&body, "- `%s`\n", file)
		}
	}

	// Version-only updates section
	if len(stats.VersionOnlyFiles) > 0 {
		body.WriteString("\n## Version Updates\n\n")

		for _, file := range stats.VersionOnlyFiles {
			fmt.Fprintf(&body, "- `%s`\n", file)
		}
	}

	body.WriteString("\n---\n\n")
	body.WriteString("*This PR was automatically created by the smyklot sync workflow*\n")

	return body.String()
}

// logSmyklotChanges logs the planned smyklot changes in dry-run mode.
func logSmyklotChanges(log *logger.Logger, stats *SmyklotSyncStats) {
	logFilesWithPrefix(log, "workflows to install:", "+", stats.InstalledFiles)
	logFilesWithPrefix(log, "workflows to replace:", "~", stats.ReplacedFiles)
	logFilesWithPrefix(log, "workflows with version-only updates:", "v", stats.VersionOnlyFiles)

	if stats.Installed+stats.Replaced+stats.VersionOnly == 0 {
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

		config.SetDefaults()

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
				SyncVersion: new(true),
				Workflows: configtypes.SmyklotWorkflowsConfig{
					PrCommands: new(true),
					Poll:       new(true),
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

	config.SetDefaults()

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
	case WorkflowPrCommands:
		orgEnabled = orgConfig.Workflows.PrCommands
	case WorkflowPoll:
		orgEnabled = orgConfig.Workflows.Poll
	default:
		return false
	}

	// Get repo override if it exists
	var repoEnabled *bool

	switch workflowName {
	case WorkflowPrCommands:
		repoEnabled = repoConfig.Sync.Smyklot.Workflows.PrCommands
	case WorkflowPoll:
		repoEnabled = repoConfig.Sync.Smyklot.Workflows.Poll
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
