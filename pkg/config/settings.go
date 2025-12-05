//nolint:golines // Config structs have jsonschema tags that exceed line length limits
package config

// Configuration for controlling organization-wide synchronization of labels, files, smyklot
// versions, and repository settings across all repositories
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type SyncConfig struct {
	// Top-level sync configuration controlling label, file, and smyklot version synchronization
	// behavior
	Sync SyncSettings `json:"sync" yaml:"sync"`
}

// Centralized control for all synchronization operations including labels, files, smyklot
// versions, and repository settings
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type SyncSettings struct {
	// Skip ALL syncs for this repository. Equivalent to setting labels.skip, files.skip,
	// smyklot.skip, and settings.skip to true
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

// Manages which GitHub labels are synced from central configuration, with options to exclude
// specific labels or remove labels not in the central config
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type LabelsConfig struct {
	// Skip label synchronization only. File sync still runs unless sync.skip or sync.files.skip
	// is true
	Skip bool `json:"skip" jsonschema:"default=false" yaml:"skip"`
	// Label names to exclude from synchronization. These labels will NOT be created/updated in
	// this repository. Existing labels with these names are preserved but not managed
	Exclude []string `json:"exclude" jsonschema:"examples=ci/skip-tests|ci/force-full,examples=release/major|release/minor|release/patch,minLength=1,uniqueItems=true" yaml:"exclude"`
	// When true, labels in this repo that are NOT in the central config will be DELETED. Use
	// with caution - this removes custom labels
	AllowRemoval bool `json:"allow_removal" jsonschema:"default=false" yaml:"allow_removal"`
}

// Controls which organization template files (CODE_OF_CONDUCT.md, CONTRIBUTING.md, etc.) are
// synced to this repository, with options to exclude specific files or remove unmanaged files
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type FilesConfig struct {
	// Skip file synchronization only. Label sync still runs unless sync.skip or
	// sync.labels.skip is true
	Skip bool `json:"skip" jsonschema:"default=false" yaml:"skip"`
	// File paths (relative to repo root) to exclude from sync. These files will NOT be
	// created/updated in this repository. Existing files at these paths are preserved but not
	// managed
	Exclude []string `json:"exclude" jsonschema:"examples=CONTRIBUTING.md|CODE_OF_CONDUCT.md,examples=.github/PULL_REQUEST_TEMPLATE.md|SECURITY.md,minLength=1,pattern=^[^/].*$,uniqueItems=true" yaml:"exclude"`
	// DANGEROUS: When true, files in this repo that are NOT in the central sync config will be
	// DELETED. This can cause data loss. Strongly recommend keeping this false
	AllowRemoval bool `json:"allow_removal" jsonschema:"default=false" yaml:"allow_removal"`
}

// Controls automatic updates of smyklot version references in workflow files when new versions
// are released
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type SmyklotConfig struct {
	// Skip smyklot version synchronization only. Label and file sync still run unless their
	// respective skip flags are set. Use this for repos that don't use smyklot or manage their
	// own versions
	Skip bool `json:"skip" jsonschema:"default=false" yaml:"skip"`
}

// Controls synchronization of GitHub repository settings like merge strategies, branch
// protection, security features, and access controls
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type SettingsConfig struct {
	// Skip repository settings synchronization. Other sync operations still run unless their
	// respective skip flags are set
	Skip bool `json:"skip" jsonschema:"default=false" yaml:"skip"`
	// Specific settings sections or fields to exclude from sync
	Exclude []string `json:"exclude" jsonschema:"examples=branch_protection,examples=security.secret_scanning,minLength=1,uniqueItems=true" yaml:"exclude"`
}

// Configures merge strategies and branch cleanup behavior for pull requests
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type RepositorySettingsConfig struct {
	// Allow squash merge for pull requests
	AllowSquashMerge *bool `json:"allow_squash_merge" yaml:"allow_squash_merge"`
	// Allow merge commits for pull requests
	AllowMergeCommit *bool `json:"allow_merge_commit" yaml:"allow_merge_commit"`
	// Allow rebase merge for pull requests
	AllowRebaseMerge *bool `json:"allow_rebase_merge" yaml:"allow_rebase_merge"`
	// Allow auto-merge for pull requests
	AllowAutoMerge *bool `json:"allow_auto_merge" yaml:"allow_auto_merge"`
	// Automatically delete head branch after pull request is merged
	DeleteBranchOnMerge *bool `json:"delete_branch_on_merge" yaml:"delete_branch_on_merge"`
}

// Controls which GitHub features are enabled for the repository (Issues, Wiki, Projects,
// Discussions)
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
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

// Configures GitHub Advanced Security features including secret scanning and Dependabot
// security updates
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type SecurityConfig struct {
	// Enable secret scanning (requires GitHub Advanced Security)
	SecretScanning *string `json:"secret_scanning" jsonschema:"enum=enabled,enum=disabled" yaml:"secret_scanning"`
	// Enable secret scanning push protection (requires GitHub Advanced Security and secret
	// scanning enabled)
	SecretScanningPushProtection *string `json:"secret_scanning_push_protection" jsonschema:"enum=enabled,enum=disabled" yaml:"secret_scanning_push_protection"`
	// Enable Dependabot security updates
	DependabotSecurityUpdates *string `json:"dependabot_security_updates" jsonschema:"enum=enabled,enum=disabled" yaml:"dependabot_security_updates"`
}

// Configures protection rules for branches including required status checks, required reviews,
// and restrictions on who can push
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type BranchProtectionRuleConfig struct {
	// Branch name pattern
	Pattern string `json:"pattern" jsonschema:"examples=main,examples=release/*,minLength=1,required" yaml:"pattern"`
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

// Specifies CI/CD checks that must pass before merging, with option to require branches be
// up-to-date
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type RequiredStatusChecks struct {
	// Require branches to be up to date before merging
	Strict *bool `json:"strict" yaml:"strict"`
	// Required status check contexts. Empty array inherits repo's existing checks (hybrid
	// approach)
	Contexts []string `json:"contexts" yaml:"contexts"`
}

// Configures pull request review requirements including approval count, code owner reviews,
// and who can bypass requirements
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
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

// BypassPullRequestAllowances defines who can bypass pull request requirements
type BypassPullRequestAllowances struct {
	// GitHub usernames that can bypass pull request requirements
	Users []string `json:"users" yaml:"users"`
	// GitHub team slugs that can bypass pull request requirements
	Teams []string `json:"teams" yaml:"teams"`
	// GitHub App slugs that can bypass pull request requirements
	Apps []string `json:"apps" yaml:"apps"`
}

// BranchRestrictionsConfig defines who can push to protected branches
type BranchRestrictionsConfig struct {
	// GitHub usernames allowed to push
	Users []string `json:"users" yaml:"users"`
	// GitHub team slugs allowed to push
	Teams []string `json:"teams" yaml:"teams"`
	// GitHub App slugs allowed to push
	Apps []string `json:"apps" yaml:"apps"`
}

// Configures GitHub Rulesets (modern replacement for branch protection) with more granular
// targeting, additional rule types, better bypass management, and enforcement modes
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type RulesetConfig struct {
	// Ruleset name (unique per repository)
	Name string `json:"name" jsonschema:"minLength=1,required" yaml:"name"`
	// Target type (branch, tag, or push)
	Target string `json:"target" jsonschema:"enum=branch,enum=tag,enum=push,required" yaml:"target"`
	// Enforcement level (active, disabled, or evaluate)
	Enforcement string `json:"enforcement" jsonschema:"enum=active,enum=disabled,enum=evaluate,required" yaml:"enforcement"`
	// Conditions for when ruleset applies
	Conditions *RulesetConditionsConfig `json:"conditions" yaml:"conditions"`
	// Actors who can bypass this ruleset
	BypassActors []BypassActorConfig `json:"bypass_actors" yaml:"bypass_actors"`
	// Rules to enforce
	Rules *RulesetRulesConfig `json:"rules" yaml:"rules"`
}

// Defines conditions for when a ruleset applies, primarily using ref name patterns
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type RulesetConditionsConfig struct {
	// Ref name patterns (branch/tag names)
	RefName *RefNameCondition `json:"ref_name" yaml:"ref_name"`
}

// Specifies include/exclude patterns for ref names (branches or tags)
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type RefNameCondition struct {
	// Ref patterns to include (e.g., "refs/heads/main", "refs/heads/release/*")
	Include []string `json:"include" jsonschema:"minLength=1,pattern=^refs/,uniqueItems=true" yaml:"include"`
	// Ref patterns to exclude
	Exclude []string `json:"exclude" jsonschema:"minLength=1,pattern=^refs/,uniqueItems=true" yaml:"exclude"`
}

// Defines an actor (user, team, app, or role) who can bypass ruleset requirements
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type BypassActorConfig struct {
	// Actor ID (user ID, team ID, app ID, or role ID)
	ActorID int64 `json:"actor_id" jsonschema:"required" yaml:"actor_id"`
	// Actor type (Integration, OrganizationAdmin, RepositoryRole, or Team)
	ActorType string `json:"actor_type" jsonschema:"enum=Integration,enum=OrganizationAdmin,enum=RepositoryRole,enum=Team,required" yaml:"actor_type"`
	// Bypass mode (always or pull_request)
	BypassMode string `json:"bypass_mode" jsonschema:"enum=always,enum=pull_request,required" yaml:"bypass_mode"`
}

// Contains all rule types that can be enforced by a ruleset
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type RulesetRulesConfig struct {
	// Pull request rules (reviews, dismissal, code owners)
	PullRequest *PullRequestRuleConfig `json:"pull_request" yaml:"pull_request"`
	// Required status checks that must pass
	RequiredStatusChecks *StatusChecksRuleConfig `json:"required_status_checks" yaml:"required_status_checks"`
	// Prevent branch/tag deletion
	Deletion *bool `json:"deletion" yaml:"deletion"`
	// Prevent non-fast-forward pushes
	NonFastForward *bool `json:"non_fast_forward" yaml:"non_fast_forward"`
	// Require linear commit history (no merge commits)
	RequiredLinearHistory *bool `json:"required_linear_history" yaml:"required_linear_history"`
	// Require signed commits
	RequiredSignatures *bool `json:"required_signatures" yaml:"required_signatures"`
	// Code scanning requirements
	CodeScanning *CodeScanningRuleConfig `json:"code_scanning" yaml:"code_scanning"`
	// Prevent branch/tag creation
	Creation *bool `json:"creation" yaml:"creation"`
	// Restrict updates to refs
	Update *bool `json:"update" yaml:"update"`
}

// Configures pull request requirements including reviews, code owners, and merge strategies
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type PullRequestRuleConfig struct {
	// Dismiss stale reviews when new commits are pushed
	DismissStaleReviewsOnPush *bool `json:"dismiss_stale_reviews_on_push" yaml:"dismiss_stale_reviews_on_push"`
	// Require review from code owners
	RequireCodeOwnerReview *bool `json:"require_code_owner_review" yaml:"require_code_owner_review"`
	// Require approval of the most recent reviewable push
	RequireLastPushApproval *bool `json:"require_last_push_approval" yaml:"require_last_push_approval"`
	// Number of required approving reviews (0-6)
	RequiredApprovingReviewCount *int `json:"required_approving_review_count" jsonschema:"maximum=6,minimum=0" yaml:"required_approving_review_count"`
	// Require all conversations to be resolved before merging
	RequiredReviewThreadResolution *bool `json:"required_review_thread_resolution" yaml:"required_review_thread_resolution"`
	// Allowed merge methods (squash, merge, rebase)
	AllowedMergeMethods []string `json:"allowed_merge_methods" jsonschema:"enum=squash,enum=merge,enum=rebase,uniqueItems=true" yaml:"allowed_merge_methods"`
}

// Specifies CI/CD status checks that must pass, with option to require strict updates
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type StatusChecksRuleConfig struct {
	// Require branches to be up to date before merging (strict mode)
	StrictRequiredStatusChecksPolicy *bool `json:"strict_required_status_checks_policy" yaml:"strict_required_status_checks_policy"`
	// Required status checks. Empty array = skip setting checks (inherit repo's existing)
	RequiredStatusChecks []StatusCheckConfig `json:"required_status_checks" yaml:"required_status_checks"`
}

// Defines a single required status check with context name and optional integration ID
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type StatusCheckConfig struct {
	// Status check context name
	Context string `json:"context" jsonschema:"minLength=1,required" yaml:"context"`
	// Integration ID for GitHub App status checks (optional)
	IntegrationID *int64 `json:"integration_id" yaml:"integration_id"`
}

// Configures code scanning tool requirements and alert thresholds
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type CodeScanningRuleConfig struct {
	// Code scanning tools and their thresholds
	CodeScanningTools []CodeScanningToolConfig `json:"code_scanning_tools" yaml:"code_scanning_tools"`
}

// Defines a code scanning tool with alert thresholds
//
//nolint:staticcheck // ST1021: Descriptive comment preferred over struct name prefix
type CodeScanningToolConfig struct {
	// Tool name (e.g., "CodeQL")
	Tool string `json:"tool" jsonschema:"minLength=1,required" yaml:"tool"`
	// Alert threshold level (none, errors, errors_and_warnings, all)
	AlertsThreshold string `json:"alerts_threshold" jsonschema:"enum=none,enum=errors,enum=errors_and_warnings,enum=all,required" yaml:"alerts_threshold"`
	// Security alert threshold level (none, critical, high_or_higher, medium_or_higher, all)
	SecurityAlertsThreshold string `json:"security_alerts_threshold" jsonschema:"enum=none,enum=critical,enum=high_or_higher,enum=medium_or_higher,enum=all,required" yaml:"security_alerts_threshold"`
}
