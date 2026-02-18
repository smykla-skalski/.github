package github

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/go-github/v80/github"
	"go.yaml.in/yaml/v4"

	"github.com/smykla-skalski/.github/internal/configtypes"
	"github.com/smykla-skalski/.github/pkg/logger"
)

// SettingsFile represents the structure of a settings YAML file.
type SettingsFile struct {
	Settings SettingsDefinition `yaml:"settings"`
}

// SettingsDefinition contains all repository settings to sync.
type SettingsDefinition struct {
	Repository       configtypes.RepositorySettingsConfig     `yaml:"repository"`
	Features         configtypes.FeaturesConfig               `yaml:"features"`
	Security         configtypes.SecurityConfig               `yaml:"security"`
	BranchProtection []configtypes.BranchProtectionRuleConfig `yaml:"branch_protection"`
	Rulesets         []configtypes.RulesetConfig              `yaml:"rulesets"`
}

// SyncSettings synchronizes repository settings from a YAML file to a target repository.
func SyncSettings(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	settingsFile string,
	syncConfig *configtypes.SyncConfig,
	dryRun bool,
) (*SettingsSyncResult, error) {
	result := NewSettingsSyncResult(repo, dryRun)

	// Check if sync is skipped
	if syncConfig.Sync.Skip || syncConfig.Sync.Settings.Skip {
		log.Info("settings sync skipped by config")
		result.CompleteSkipped("sync disabled by config")

		return result, nil
	}

	// Parse settings file and fetch current repository
	desiredSettings, currentRepo, err := loadSettings(ctx, log, client, org, repo, settingsFile)
	if err != nil {
		result.CompleteWithError(err)

		return result, err
	}

	// Apply merge configurations if present
	desiredSettings, err = ApplySettingsMerge(log, desiredSettings, syncConfig)
	if err != nil {
		result.CompleteWithError(errors.Wrap(err, "applying settings merge"))

		return result, err
	}

	// Compute all changes
	repoChanges, hasBranchProtectionChanges := computeAllSettingsChanges(
		desiredSettings,
		currentRepo,
		syncConfig.Sync.Settings.Exclude,
	)

	log.Info("computed settings diff",
		"has_repo_changes", repoChanges != nil,
		"has_branch_protection", hasBranchProtectionChanges,
	)

	// Count changes
	changesCount := 0
	if repoChanges != nil {
		changesCount++
	}

	if hasBranchProtectionChanges {
		changesCount++
	}

	result.ChangesApplied = changesCount

	// Handle dry-run mode or apply changes
	if dryRun {
		err = handleDryRun(log, repoChanges, hasBranchProtectionChanges, desiredSettings)
	} else {
		err = applyAllSettingsChanges(
			ctx,
			log,
			client,
			org,
			repo,
			repoChanges,
			hasBranchProtectionChanges,
			desiredSettings.BranchProtection,
		)
	}

	if err != nil {
		result.CompleteWithError(err)

		return result, err
	}

	// Sync rulesets
	if err := SyncRulesets(
		ctx,
		log,
		client,
		org,
		repo,
		desiredSettings.Rulesets,
		syncConfig.Sync.Settings.Exclude,
		dryRun,
	); err != nil {
		result.CompleteWithError(errors.Wrap(err, "syncing rulesets"))

		return result, err
	}

	result.Complete(StatusSuccess)

	return result, nil
}

// loadSettings loads desired settings from file and fetches current repository state.
func loadSettings(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	settingsFile string,
) (*SettingsDefinition, *github.Repository, error) {
	desiredSettings, err := parseSettingsFile(settingsFile)
	if err != nil {
		return nil, nil, errors.Wrap(err, "parsing settings file")
	}

	log.Debug("parsed settings file", "file", settingsFile)

	currentRepo, err := fetchRepository(ctx, client, org, repo)
	if err != nil {
		return nil, nil, errors.Wrap(err, "fetching repository")
	}

	log.Debug("fetched current repository settings")

	return desiredSettings, currentRepo, nil
}

// computeAllSettingsChanges computes all repository and branch protection changes.
func computeAllSettingsChanges(
	desired *SettingsDefinition,
	current *github.Repository,
	exclude []string,
) (*github.Repository, bool) {
	repoUpdate := computeRepositorySettingsDiff(&desired.Repository, current, exclude)
	featuresUpdate := computeFeaturesDiff(&desired.Features, current, exclude)
	securityUpdate := computeSecurityDiff(&desired.Security, current, exclude)

	repoChanges := mergeRepositoryUpdates(repoUpdate, featuresUpdate, securityUpdate)

	hasBranchProtectionChanges := len(desired.BranchProtection) > 0 &&
		!isSettingExcluded("branch_protection", exclude)

	return repoChanges, hasBranchProtectionChanges
}

// handleDryRun logs planned changes without applying them.
//
//nolint:unparam // error return kept for consistency with applyAllSettingsChanges signature
func handleDryRun(
	log *logger.Logger,
	repoChanges *github.Repository,
	hasBranchProtectionChanges bool,
	desired *SettingsDefinition,
) error {
	log.Info("dry-run mode: skipping settings changes")

	if repoChanges != nil {
		logRepositoryChanges(log, repoChanges)
	}

	if hasBranchProtectionChanges {
		log.Info("branch protection rules would be synced", "count", len(desired.BranchProtection))
	}

	return nil
}

// applyAllSettingsChanges applies repository and branch protection changes.
func applyAllSettingsChanges(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	repoChanges *github.Repository,
	hasBranchProtectionChanges bool,
	branchProtectionRules []configtypes.BranchProtectionRuleConfig,
) error {
	if repoChanges != nil {
		if err := applyRepositoryChanges(ctx, client, org, repo, repoChanges); err != nil {
			return errors.Wrap(err, "applying repository settings")
		}

		log.Info("repository settings updated successfully")
	}

	if hasBranchProtectionChanges {
		if err := syncBranchProtection(
			ctx,
			log,
			client,
			org,
			repo,
			branchProtectionRules,
		); err != nil {
			return errors.Wrap(err, "syncing branch protection")
		}

		log.Info("branch protection synced successfully")
	}

	if repoChanges == nil && !hasBranchProtectionChanges {
		log.Info("no settings changes needed")
	}

	log.Info("settings sync completed successfully")

	return nil
}

// parseSettingsFile reads and parses a YAML settings file.
func parseSettingsFile(path string) (*SettingsDefinition, error) {
	//nolint:gosec // File path is provided by user via CLI flag
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "reading settings file")
	}

	var settingsFile SettingsFile
	if err := yaml.Unmarshal(data, &settingsFile); err != nil {
		return nil, errors.Wrap(err, "unmarshaling settings YAML")
	}

	return &settingsFile.Settings, nil
}

// fetchRepository retrieves repository information.
func fetchRepository(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
) (*github.Repository, error) {
	repository, _, err := client.Repositories.Get(ctx, org, repo)
	if err != nil {
		return nil, errors.Wrap(err, "getting repository")
	}

	return repository, nil
}

// isSettingExcluded checks if a setting path is in the exclusion list.
func isSettingExcluded(path string, exclude []string) bool {
	for _, excluded := range exclude {
		if excluded == path || strings.HasPrefix(path, excluded+".") {
			return true
		}
	}

	return false
}

// computeRepositorySettingsDiff computes repository settings that need updating.
func computeRepositorySettingsDiff(
	desired *configtypes.RepositorySettingsConfig,
	current *github.Repository,
	exclude []string,
) *github.Repository {
	update := &github.Repository{}
	hasChanges := false

	// Allow squash merge
	if desired.AllowSquashMerge != nil &&
		!isSettingExcluded("repository.allow_squash_merge", exclude) &&
		current.GetAllowSquashMerge() != *desired.AllowSquashMerge {
		update.AllowSquashMerge = desired.AllowSquashMerge
		hasChanges = true
	}

	// Allow merge commit
	if desired.AllowMergeCommit != nil &&
		!isSettingExcluded("repository.allow_merge_commit", exclude) &&
		current.GetAllowMergeCommit() != *desired.AllowMergeCommit {
		update.AllowMergeCommit = desired.AllowMergeCommit
		hasChanges = true
	}

	// Allow rebase merge
	if desired.AllowRebaseMerge != nil &&
		!isSettingExcluded("repository.allow_rebase_merge", exclude) &&
		current.GetAllowRebaseMerge() != *desired.AllowRebaseMerge {
		update.AllowRebaseMerge = desired.AllowRebaseMerge
		hasChanges = true
	}

	// Allow auto-merge
	if desired.AllowAutoMerge != nil &&
		!isSettingExcluded("repository.allow_auto_merge", exclude) &&
		current.GetAllowAutoMerge() != *desired.AllowAutoMerge {
		update.AllowAutoMerge = desired.AllowAutoMerge
		hasChanges = true
	}

	// Delete branch on merge
	if desired.DeleteBranchOnMerge != nil &&
		!isSettingExcluded("repository.delete_branch_on_merge", exclude) &&
		current.GetDeleteBranchOnMerge() != *desired.DeleteBranchOnMerge {
		update.DeleteBranchOnMerge = desired.DeleteBranchOnMerge
		hasChanges = true
	}

	if !hasChanges {
		return nil
	}

	return update
}

// computeFeaturesDiff computes feature settings that need updating.
func computeFeaturesDiff(
	desired *configtypes.FeaturesConfig,
	current *github.Repository,
	exclude []string,
) *github.Repository {
	update := &github.Repository{}
	hasChanges := false

	// Has issues
	if desired.HasIssues != nil &&
		!isSettingExcluded("features.has_issues", exclude) &&
		current.GetHasIssues() != *desired.HasIssues {
		update.HasIssues = desired.HasIssues
		hasChanges = true
	}

	// Has wiki
	if desired.HasWiki != nil &&
		!isSettingExcluded("features.has_wiki", exclude) &&
		current.GetHasWiki() != *desired.HasWiki {
		update.HasWiki = desired.HasWiki
		hasChanges = true
	}

	// Has projects
	if desired.HasProjects != nil &&
		!isSettingExcluded("features.has_projects", exclude) &&
		current.GetHasProjects() != *desired.HasProjects {
		update.HasProjects = desired.HasProjects
		hasChanges = true
	}

	// Has discussions
	if desired.HasDiscussions != nil &&
		!isSettingExcluded("features.has_discussions", exclude) &&
		current.GetHasDiscussions() != *desired.HasDiscussions {
		update.HasDiscussions = desired.HasDiscussions
		hasChanges = true
	}

	if !hasChanges {
		return nil
	}

	return update
}

// computeSecurityDiff computes security settings that need updating.
func computeSecurityDiff(
	desired *configtypes.SecurityConfig,
	current *github.Repository,
	exclude []string,
) *github.Repository {
	update := &github.Repository{}
	hasChanges := false

	// Only proceed if we have security settings to sync
	if desired.SecretScanning == nil &&
		desired.SecretScanningPushProtection == nil &&
		desired.DependabotSecurityUpdates == nil {
		return nil
	}

	// Initialize SecurityAndAnalysis if needed
	currentSecurity := current.GetSecurityAndAnalysis()
	securityUpdate := &github.SecurityAndAnalysis{}

	// Secret scanning
	if desired.SecretScanning != nil && !isSettingExcluded("security.secret_scanning", exclude) {
		desiredStatus := *desired.SecretScanning
		currentStatus := getSecurityFeatureStatus(currentSecurity.GetSecretScanning())

		if currentStatus != desiredStatus {
			securityUpdate.SecretScanning = &github.SecretScanning{
				Status: github.Ptr(desiredStatus),
			}
			hasChanges = true
		}
	}

	// Secret scanning push protection
	if desired.SecretScanningPushProtection != nil &&
		!isSettingExcluded("security.secret_scanning_push_protection", exclude) {
		desiredStatus := *desired.SecretScanningPushProtection
		currentStatus := getSecurityFeatureStatus(currentSecurity.GetSecretScanningPushProtection())

		if currentStatus != desiredStatus {
			securityUpdate.SecretScanningPushProtection = &github.SecretScanningPushProtection{
				Status: github.Ptr(desiredStatus),
			}
			hasChanges = true
		}
	}

	// Dependabot security updates
	if desired.DependabotSecurityUpdates != nil &&
		!isSettingExcluded("security.dependabot_security_updates", exclude) {
		desiredStatus := *desired.DependabotSecurityUpdates
		currentStatus := getSecurityFeatureStatus(currentSecurity.GetDependabotSecurityUpdates())

		if currentStatus != desiredStatus {
			securityUpdate.DependabotSecurityUpdates = &github.DependabotSecurityUpdates{
				Status: github.Ptr(desiredStatus),
			}
			hasChanges = true
		}
	}

	if !hasChanges {
		return nil
	}

	update.SecurityAndAnalysis = securityUpdate

	return update
}

// getSecurityFeatureStatus extracts status from security feature interfaces.
func getSecurityFeatureStatus(
	feature interface {
		GetStatus() string
	},
) string {
	if feature == nil {
		return ""
	}

	return feature.GetStatus()
}

// mergeRepositoryUpdates merges multiple repository update structs.
func mergeRepositoryUpdates(updates ...*github.Repository) *github.Repository {
	merged := &github.Repository{}
	hasChanges := false

	for _, update := range updates {
		if update == nil {
			continue
		}

		hasChanges = true

		mergeRepositoryFields(merged, update)
		mergeFeatureFields(merged, update)
		mergeSecurityFields(merged, update)
	}

	if !hasChanges {
		return nil
	}

	return merged
}

// mergeRepositoryFields merges repository settings fields.
func mergeRepositoryFields(merged, update *github.Repository) {
	if update.AllowSquashMerge != nil {
		merged.AllowSquashMerge = update.AllowSquashMerge
	}

	if update.AllowMergeCommit != nil {
		merged.AllowMergeCommit = update.AllowMergeCommit
	}

	if update.AllowRebaseMerge != nil {
		merged.AllowRebaseMerge = update.AllowRebaseMerge
	}

	if update.AllowAutoMerge != nil {
		merged.AllowAutoMerge = update.AllowAutoMerge
	}

	if update.DeleteBranchOnMerge != nil {
		merged.DeleteBranchOnMerge = update.DeleteBranchOnMerge
	}
}

// mergeFeatureFields merges feature settings fields.
func mergeFeatureFields(merged, update *github.Repository) {
	if update.HasIssues != nil {
		merged.HasIssues = update.HasIssues
	}

	if update.HasWiki != nil {
		merged.HasWiki = update.HasWiki
	}

	if update.HasProjects != nil {
		merged.HasProjects = update.HasProjects
	}

	if update.HasDiscussions != nil {
		merged.HasDiscussions = update.HasDiscussions
	}
}

// mergeSecurityFields merges security settings fields.
func mergeSecurityFields(merged, update *github.Repository) {
	if update.SecurityAndAnalysis != nil {
		merged.SecurityAndAnalysis = update.SecurityAndAnalysis
	}
}

// applyRepositoryChanges applies repository settings changes.
func applyRepositoryChanges(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
	update *github.Repository,
) error {
	_, _, err := client.Repositories.Edit(ctx, org, repo, update)
	if err != nil {
		return errors.Wrap(err, "updating repository")
	}

	return nil
}

// logRepositoryChanges logs planned repository changes in dry-run mode.
func logRepositoryChanges(log *logger.Logger, update *github.Repository) {
	log.Info("repository settings to update:")

	if update.AllowSquashMerge != nil {
		log.Info("  ~ allow_squash_merge", "value", *update.AllowSquashMerge)
	}

	if update.AllowMergeCommit != nil {
		log.Info("  ~ allow_merge_commit", "value", *update.AllowMergeCommit)
	}

	if update.AllowRebaseMerge != nil {
		log.Info("  ~ allow_rebase_merge", "value", *update.AllowRebaseMerge)
	}

	if update.AllowAutoMerge != nil {
		log.Info("  ~ allow_auto_merge", "value", *update.AllowAutoMerge)
	}

	if update.DeleteBranchOnMerge != nil {
		log.Info("  ~ delete_branch_on_merge", "value", *update.DeleteBranchOnMerge)
	}

	if update.HasIssues != nil {
		log.Info("  ~ has_issues", "value", *update.HasIssues)
	}

	if update.HasWiki != nil {
		log.Info("  ~ has_wiki", "value", *update.HasWiki)
	}

	if update.HasProjects != nil {
		log.Info("  ~ has_projects", "value", *update.HasProjects)
	}

	if update.HasDiscussions != nil {
		log.Info("  ~ has_discussions", "value", *update.HasDiscussions)
	}

	if update.SecurityAndAnalysis != nil {
		security := update.SecurityAndAnalysis

		if security.SecretScanning != nil {
			log.Info("  ~ secret_scanning", "status", security.SecretScanning.GetStatus())
		}

		if security.SecretScanningPushProtection != nil {
			log.Info("  ~ secret_scanning_push_protection",
				"status", security.SecretScanningPushProtection.GetStatus())
		}

		if security.DependabotSecurityUpdates != nil {
			log.Info("  ~ dependabot_security_updates",
				"status", security.DependabotSecurityUpdates.GetStatus())
		}
	}
}

// syncBranchProtection synchronizes branch protection rules.
func syncBranchProtection(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	rules []configtypes.BranchProtectionRuleConfig,
) error {
	// Get all branches in the repository
	branches, err := fetchBranches(ctx, client, org, repo)
	if err != nil {
		return errors.Wrap(err, "fetching branches")
	}

	log.Debug("fetched branches", "count", len(branches))

	// Apply each protection rule
	for _, rule := range rules {
		// Find matching branches
		matchingBranches := findMatchingBranches(branches, rule.Pattern)

		log.Debug("found matching branches",
			"pattern", rule.Pattern,
			"count", len(matchingBranches),
		)

		// Apply protection to each matching branch
		for _, branch := range matchingBranches {
			if err := applyBranchProtection(
				ctx,
				log,
				client,
				org,
				repo,
				branch,
				&rule,
			); err != nil {
				return errors.Wrapf(err, "applying protection to branch %q", branch)
			}

			log.Debug("applied branch protection", "branch", branch, "pattern", rule.Pattern)
		}
	}

	return nil
}

// fetchBranches retrieves all branch names from a repository.
func fetchBranches(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
) ([]string, error) {
	const branchesPerPage = 100

	opts := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: branchesPerPage},
	}

	var allBranches []string

	for {
		branches, resp, err := client.Repositories.ListBranches(ctx, org, repo, opts)
		if err != nil {
			return nil, errors.Wrap(err, "listing branches")
		}

		for _, branch := range branches {
			allBranches = append(allBranches, branch.GetName())
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allBranches, nil
}

// findMatchingBranches finds branches matching a pattern.
func findMatchingBranches(branches []string, pattern string) []string {
	var matches []string

	for _, branch := range branches {
		if matchBranchPattern(branch, pattern) {
			matches = append(matches, branch)
		}
	}

	return matches
}

// matchBranchPattern checks if a branch name matches a pattern.
func matchBranchPattern(branch, pattern string) bool {
	// Exact match
	if branch == pattern {
		return true
	}

	// Glob pattern support (simple wildcard matching)
	matched, err := filepath.Match(pattern, branch)
	if err != nil {
		return false
	}

	return matched
}

// applyBranchProtection applies protection rules to a specific branch.
func applyBranchProtection(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	branch string,
	rule *configtypes.BranchProtectionRuleConfig,
) error {
	// Fetch current protection settings
	currentProtection, err := fetchBranchProtection(ctx, client, org, repo, branch)
	if err != nil {
		// If branch protection doesn't exist, that's okay - we'll create it
		log.Debug("no existing protection found for branch", "branch", branch)
	}

	// Build protection request
	protectionReq := buildProtectionRequest(rule, currentProtection)

	// Apply protection
	_, _, err = client.Repositories.UpdateBranchProtection(ctx, org, repo, branch, protectionReq)
	if err != nil {
		return errors.Wrapf(err, "updating branch protection for %q", branch)
	}

	return nil
}

// fetchBranchProtection retrieves current branch protection settings.
func fetchBranchProtection(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
	branch string,
) (*github.Protection, error) {
	protection, _, err := client.Repositories.GetBranchProtection(ctx, org, repo, branch)
	if err != nil {
		return nil, errors.Wrap(err, "getting branch protection")
	}

	return protection, nil
}

// buildProtectionRequest builds a protection request from config and current state.
func buildProtectionRequest(
	rule *configtypes.BranchProtectionRuleConfig,
	current *github.Protection,
) *github.ProtectionRequest {
	req := &github.ProtectionRequest{}

	// Required status checks
	if rule.RequiredStatusChecks != nil {
		contexts := rule.RequiredStatusChecks.Contexts

		// Hybrid approach: if contexts is empty, inherit existing checks
		if len(contexts) == 0 && current != nil && current.RequiredStatusChecks != nil {
			contexts = current.RequiredStatusChecks.GetContexts()
		}

		req.RequiredStatusChecks = &github.RequiredStatusChecks{
			Strict:   getBoolValue(rule.RequiredStatusChecks.Strict),
			Contexts: &contexts,
		}
	}

	// Required pull request reviews
	if rule.RequiredReviews != nil {
		reviewCount := getRequiredReviewCount(rule, current)
		reviews := &github.PullRequestReviewsEnforcementRequest{
			DismissStaleReviews: getBoolValue(
				rule.RequiredReviews.DismissStaleReviews,
			),
			RequireCodeOwnerReviews: getBoolValue(
				rule.RequiredReviews.RequireCodeOwnerReviews,
			),
			RequiredApprovingReviewCount: reviewCount,
			RequireLastPushApproval:      rule.RequiredReviews.RequireLastPushApproval,
		}

		// Bypass allowances
		if rule.RequiredReviews.BypassPullRequestAllowances != nil {
			bypass := rule.RequiredReviews.BypassPullRequestAllowances
			reviews.BypassPullRequestAllowancesRequest = &github.BypassPullRequestAllowancesRequest{
				Users: bypass.Users,
				Teams: bypass.Teams,
				Apps:  bypass.Apps,
			}
		}

		req.RequiredPullRequestReviews = reviews
	}

	// Other settings
	req.EnforceAdmins = getBoolValue(rule.EnforceAdmins)
	req.RequireLinearHistory = rule.RequireLinearHistory
	req.AllowForcePushes = rule.AllowForcePushes
	req.AllowDeletions = rule.AllowDeletions
	req.RequiredConversationResolution = rule.RequiredConversationResolution

	// Restrictions
	if rule.Restrictions != nil {
		req.Restrictions = &github.BranchRestrictionsRequest{
			Users: rule.Restrictions.Users,
			Teams: rule.Restrictions.Teams,
			Apps:  rule.Restrictions.Apps,
		}
	}

	return req
}

// getRequiredReviewCount implements no-downgrade logic for review counts.
func getRequiredReviewCount(
	rule *configtypes.BranchProtectionRuleConfig,
	current *github.Protection,
) int {
	if rule.RequiredReviews == nil || rule.RequiredReviews.RequiredApprovingReviewCount == nil {
		return 0
	}

	desiredCount := *rule.RequiredReviews.RequiredApprovingReviewCount

	// If no current protection, use desired count
	if current == nil || current.RequiredPullRequestReviews == nil {
		return desiredCount
	}

	currentCount := current.RequiredPullRequestReviews.RequiredApprovingReviewCount

	// No-downgrade: use the higher of current or desired
	if currentCount > desiredCount {
		return currentCount
	}

	return desiredCount
}

// getBoolValue returns the bool value if not nil, otherwise returns false.
func getBoolValue(val *bool) bool {
	if val == nil {
		return false
	}

	return *val
}
