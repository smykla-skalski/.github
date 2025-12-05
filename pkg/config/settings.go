//nolint:golines // Config structs have jsonschema tags that exceed line length limits
package config

// SyncConfig is the root configuration structure.
type SyncConfig struct {
	// Top-level sync configuration controlling label, file, and smyklot version synchronization behavior
	Sync SyncSettings `json:"sync" yaml:"sync"`
}

// SyncSettings contains all sync-related settings.
type SyncSettings struct {
	// Skip ALL syncs for this repository. Equivalent to setting labels.skip, files.skip, smyklot.skip, and settings.skip to true
	Skip bool `json:"skip" jsonschema:"default=false" yaml:"skip"`
	// Label synchronization configuration
	Labels LabelsConfig `json:"labels" yaml:"labels"`
	// File synchronization configuration
	Files FilesConfig `json:"files" yaml:"files"`
	// Smyklot version synchronization configuration
	Smyklot SmyklotConfig `json:"smyklot" yaml:"smyklot"`
	// Repository settings synchronization configuration
	Settings SettingsConfig `json:"settings" yaml:"settings"`
}

// LabelsConfig controls label synchronization behavior.
type LabelsConfig struct {
	// Skip label synchronization only. File sync still runs unless sync.skip or sync.files.skip is true
	Skip bool `json:"skip" jsonschema:"default=false" yaml:"skip"`
	// Label names to exclude from synchronization. These labels will NOT be created/updated in this repository. Existing labels with these names are preserved but not managed
	Exclude []string `json:"exclude" jsonschema:"minLength=1,uniqueItems=true" yaml:"exclude"`
	// When true, labels in this repo that are NOT in the central config will be DELETED. Use with caution - this removes custom labels
	AllowRemoval bool `json:"allow_removal" jsonschema:"default=false" yaml:"allow_removal"`
}

// FilesConfig controls file synchronization behavior.
type FilesConfig struct {
	// Skip file synchronization only. Label sync still runs unless sync.skip or sync.labels.skip is true
	Skip bool `json:"skip" jsonschema:"default=false" yaml:"skip"`
	// File paths (relative to repo root) to exclude from sync. These files will NOT be created/updated in this repository. Existing files at these paths are preserved but not managed
	Exclude []string `json:"exclude" jsonschema:"minLength=1,pattern=^[^/].*$,uniqueItems=true" yaml:"exclude"`
	// DANGEROUS: When true, files in this repo that are NOT in the central sync config will be DELETED. This can cause data loss. Strongly recommend keeping this false
	AllowRemoval bool `json:"allow_removal" jsonschema:"default=false" yaml:"allow_removal"`
}

// SmyklotConfig controls smyklot version synchronization behavior.
type SmyklotConfig struct {
	// Skip smyklot version synchronization only. Label and file sync still run unless their respective skip flags are set. Use this for repos that don't use smyklot or manage their own versions
	Skip bool `json:"skip" jsonschema:"default=false" yaml:"skip"`
}

// SettingsConfig controls repository settings synchronization behavior.
type SettingsConfig struct {
	// Skip repository settings synchronization. Other sync operations still run unless their respective skip flags are set
	Skip bool `json:"skip" jsonschema:"default=false" yaml:"skip"`
	// Specific settings sections or fields to exclude from sync. Format: 'section' or 'section.field'. Examples: 'branch_protection', 'security.secret_scanning'
	Exclude []string `json:"exclude" jsonschema:"minLength=1,uniqueItems=true" yaml:"exclude"`
}

// RepositorySettingsConfig defines repository-level settings.
type RepositorySettingsConfig struct {
	// Allow squash merge for pull requests
	AllowSquashMerge *bool `json:"allow_squash_merge" yaml:"allow_squash_merge"`
	// Allow merge commits for pull requests
	AllowMergeCommit *bool `json:"allow_merge_commit" yaml:"allow_merge_commit"`
	// Allow rebase merge for pull requests
	AllowRebaseMerge *bool `json:"allow_rebase_merge" yaml:"allow_rebase_merge"`
	// Allow auto-merge for pull requests
	AllowAutoMerge *bool `json:"allow_auto_merge" yaml:"allow_auto_merge"`
	// Automatically delete head branch after pull requests
	DeleteBranchOnMerge *bool `json:"delete_branch_on_merge" yaml:"delete_branch_on_merge"`
}

// FeaturesConfig defines repository feature settings.
type FeaturesConfig struct {
	// Enable GitHub Issues
	HasIssues *bool `json:"has_issues" yaml:"has_issues"`
	// Enable GitHub Wiki
	HasWiki *bool `json:"has_wiki" yaml:"has_wiki"`
	// Enable GitHub Projects
	HasProjects *bool `json:"has_projects" yaml:"has_projects"`
	// Enable GitHub Discussions
	HasDiscussions *bool `json:"has_discussions" yaml:"has_discussions"`
}

// SecurityConfig defines security and analysis settings.
type SecurityConfig struct {
	// Enable secret scanning (requires GitHub Advanced Security)
	SecretScanning *string `json:"secret_scanning" jsonschema:"enum=enabled,enum=disabled" yaml:"secret_scanning"`
	// Enable secret scanning push protection (requires GitHub Advanced Security and secret scanning enabled)
	SecretScanningPushProtection *string `json:"secret_scanning_push_protection" jsonschema:"enum=enabled,enum=disabled" yaml:"secret_scanning_push_protection"`
	// Enable Dependabot security updates
	DependabotSecurityUpdates *string `json:"dependabot_security_updates" jsonschema:"enum=enabled,enum=disabled" yaml:"dependabot_security_updates"`
}

// BranchProtectionRuleConfig defines branch protection rules.
type BranchProtectionRuleConfig struct {
	// Branch name pattern (e.g. 'main' or 'release/*')
	Pattern string `json:"pattern" jsonschema:"minLength=1,required" yaml:"pattern"`
	// Required status checks configuration
	RequiredStatusChecks *RequiredStatusChecks `json:"required_status_checks" yaml:"required_status_checks"`
	// Required pull request reviews configuration
	RequiredReviews *RequiredReviews `json:"required_reviews" yaml:"required_reviews"`
	// Enforce branch protection rules for administrators
	EnforceAdmins *bool `json:"enforce_admins" yaml:"enforce_admins"`
	// Require linear commit history
	RequireLinearHistory *bool `json:"require_linear_history" yaml:"require_linear_history"`
	// Allow force pushes to this branch
	AllowForcePushes *bool `json:"allow_force_pushes" yaml:"allow_force_pushes"`
	// Allow branch deletion
	AllowDeletions *bool `json:"allow_deletions" yaml:"allow_deletions"`
	// Require all conversations on code to be resolved before pull request can be merged
	RequiredConversationResolution *bool `json:"required_conversation_resolution" yaml:"required_conversation_resolution"`
	// Restrict who can push to this branch. Leave unset to allow all users with push access
	Restrictions *BranchRestrictionsConfig `json:"restrictions" yaml:"restrictions"`
}

// RequiredStatusChecks defines required status check settings.
type RequiredStatusChecks struct {
	// Require branches to be up to date before merging
	Strict *bool `json:"strict" yaml:"strict"`
	// Required status check contexts. Empty array inherits repo's existing checks (hybrid approach)
	Contexts []string `json:"contexts" yaml:"contexts"`
}

// RequiredReviews defines required pull request review settings.
type RequiredReviews struct {
	// Number of required approving reviews
	RequiredApprovingReviewCount *int `json:"count" jsonschema:"maximum=6,minimum=0" yaml:"count"`
	// Dismiss stale pull request approvals when new commits are pushed
	DismissStaleReviews *bool `json:"dismiss_stale" yaml:"dismiss_stale"`
	// Require review from code owners
	RequireCodeOwnerReviews *bool `json:"require_code_owner_reviews" yaml:"require_code_owner_reviews"`
	// Require approval of the most recent reviewable push
	RequireLastPushApproval *bool `json:"require_last_push_approval" yaml:"require_last_push_approval"`
	// Allow specified users, teams, or apps to bypass pull request requirements
	BypassPullRequestAllowances *BypassPullRequestAllowances `json:"bypass_pull_request_allowances" yaml:"bypass_pull_request_allowances"`
}

// BypassPullRequestAllowances defines who can bypass pull request requirements.
type BypassPullRequestAllowances struct {
	// GitHub usernames that can bypass pull request requirements
	Users []string `json:"users" yaml:"users"`
	// GitHub team slugs that can bypass pull request requirements
	Teams []string `json:"teams" yaml:"teams"`
	// GitHub App slugs that can bypass pull request requirements
	Apps []string `json:"apps" yaml:"apps"`
}

// BranchRestrictionsConfig defines who can push to protected branches.
type BranchRestrictionsConfig struct {
	// GitHub usernames allowed to push
	Users []string `json:"users" yaml:"users"`
	// GitHub team slugs allowed to push
	Teams []string `json:"teams" yaml:"teams"`
	// GitHub App slugs allowed to push
	Apps []string `json:"apps" yaml:"apps"`
}
