package github

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/google/go-github/v80/github"

	"github.com/smykla-skalski/.github/internal/configtypes"
	"github.com/smykla-skalski/.github/pkg/logger"
)

// SyncRulesets synchronizes repository rulesets from configuration to target repository.
func SyncRulesets(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	rulesets []configtypes.RulesetConfig,
	exclude []string,
	dryRun bool,
) error {
	// Check if rulesets sync is excluded
	if isSettingExcluded("rulesets", exclude) {
		log.Debug("rulesets sync excluded by config")

		return nil
	}

	// No rulesets to sync
	if len(rulesets) == 0 {
		log.Debug("no rulesets configured")

		return nil
	}

	// Fetch existing rulesets
	existingRulesets, err := fetchExistingRulesets(ctx, client, org, repo)
	if err != nil {
		return errors.Wrap(err, "fetching existing rulesets")
	}

	log.Debug("fetched existing rulesets", "count", len(existingRulesets))

	// Build map of existing rulesets by name for quick lookup
	existingByName := make(map[string]*github.RepositoryRuleset)
	for _, ruleset := range existingRulesets {
		existingByName[ruleset.Name] = ruleset
	}

	// Sync each ruleset
	for _, rulesetConfig := range rulesets {
		existing := existingByName[rulesetConfig.Name]

		if dryRun {
			logRulesetOperation(log, rulesetConfig.Name, existing)

			continue
		}

		if err := createOrUpdateRuleset(ctx, log, client, org, repo, rulesetConfig, existing); err != nil {
			return errors.Wrapf(err, "syncing ruleset %q", rulesetConfig.Name)
		}

		if existing == nil {
			log.Info("created ruleset", "name", rulesetConfig.Name)
		} else {
			log.Info("updated ruleset", "name", rulesetConfig.Name)
		}
	}

	if dryRun {
		log.Info("dry-run mode: skipping ruleset changes", "count", len(rulesets))
	} else {
		log.Info("rulesets synced successfully", "count", len(rulesets))
	}

	return nil
}

// fetchExistingRulesets retrieves all rulesets from a repository.
func fetchExistingRulesets(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
) ([]*github.RepositoryRuleset, error) {
	opts := &github.RepositoryListRulesetsOptions{
		IncludesParents: github.Ptr(false),
	}

	rulesets, _, err := client.Repositories.GetAllRulesets(ctx, org, repo, opts)
	if err != nil {
		return nil, errors.Wrap(err, "listing rulesets")
	}

	return rulesets, nil
}

// createOrUpdateRuleset creates a new ruleset or updates an existing one.
func createOrUpdateRuleset(
	ctx context.Context,
	log *logger.Logger,
	client *Client,
	org string,
	repo string,
	rulesetConfig configtypes.RulesetConfig,
	existing *github.RepositoryRuleset,
) error {
	// Build ruleset from config
	ruleset := buildRulesetFromConfig(rulesetConfig, existing)

	if existing == nil {
		// Create new ruleset
		_, _, err := client.Repositories.CreateRuleset(ctx, org, repo, *ruleset)
		if err != nil {
			return errors.Wrap(err, "creating ruleset")
		}

		log.Debug("created new ruleset", "name", rulesetConfig.Name)
	} else {
		// Update existing ruleset
		_, _, err := client.Repositories.UpdateRuleset(ctx, org, repo, existing.GetID(), *ruleset)
		if err != nil {
			return errors.Wrap(err, "updating ruleset")
		}

		log.Debug("updated existing ruleset",
			"name", rulesetConfig.Name,
			"id", existing.GetID(),
		)
	}

	return nil
}

// buildRulesetFromConfig converts config ruleset to go-github RepositoryRuleset.
func buildRulesetFromConfig(
	rulesetConfig configtypes.RulesetConfig,
	existing *github.RepositoryRuleset,
) *github.RepositoryRuleset {
	target := github.RulesetTarget(rulesetConfig.Target)
	enforcement := github.RulesetEnforcement(rulesetConfig.Enforcement)

	ruleset := &github.RepositoryRuleset{
		Name:        rulesetConfig.Name,
		Target:      &target,
		Enforcement: enforcement,
	}

	// Convert conditions
	if rulesetConfig.Conditions != nil {
		ruleset.Conditions = buildConditions(rulesetConfig.Conditions)
	}

	// Convert bypass actors
	if len(rulesetConfig.BypassActors) > 0 {
		ruleset.BypassActors = buildBypassActors(rulesetConfig.BypassActors)
	}

	// Convert rules
	if rulesetConfig.Rules != nil {
		ruleset.Rules = buildRules(rulesetConfig.Rules, existing)
	}

	return ruleset
}

// buildConditions converts config conditions to go-github RepositoryRulesetConditions.
func buildConditions(
	conditionsConfig *configtypes.RulesetConditionsConfig,
) *github.RepositoryRulesetConditions {
	conditions := &github.RepositoryRulesetConditions{}

	if conditionsConfig.RefName != nil {
		// Always initialize both arrays - GitHub API requires non-null arrays
		refNameCondition := &github.RepositoryRulesetRefConditionParameters{
			Include: conditionsConfig.RefName.Include,
			Exclude: conditionsConfig.RefName.Exclude,
		}

		// Ensure empty arrays instead of nil (GitHub API rejects null values)
		if refNameCondition.Include == nil {
			refNameCondition.Include = []string{}
		}

		if refNameCondition.Exclude == nil {
			refNameCondition.Exclude = []string{}
		}

		conditions.RefName = refNameCondition
	}

	return conditions
}

// buildBypassActors converts config bypass actors to go-github BypassActor.
func buildBypassActors(actorsConfig []configtypes.BypassActorConfig) []*github.BypassActor {
	actors := make([]*github.BypassActor, 0, len(actorsConfig))

	for _, actorConfig := range actorsConfig {
		actorType := github.BypassActorType(actorConfig.ActorType)
		bypassMode := github.BypassMode(actorConfig.BypassMode)

		actor := &github.BypassActor{
			ActorID:    github.Ptr(actorConfig.ActorID),
			ActorType:  &actorType,
			BypassMode: &bypassMode,
		}
		actors = append(actors, actor)
	}

	return actors
}

// buildRules converts config rules to go-github RepositoryRulesetRules.
func buildRules(
	rulesConfig *configtypes.RulesetRulesConfig,
	existing *github.RepositoryRuleset,
) *github.RepositoryRulesetRules {
	rules := &github.RepositoryRulesetRules{}

	// Pull request rule
	if rulesConfig.PullRequest != nil {
		rules.PullRequest = buildPullRequestRule(rulesConfig.PullRequest, existing)
	}

	// Required status checks
	if rulesConfig.RequiredStatusChecks != nil {
		rules.RequiredStatusChecks = buildStatusChecksRule(
			rulesConfig.RequiredStatusChecks,
			existing,
		)
	}

	// Simple boolean rules using EmptyRuleParameters
	if rulesConfig.Deletion != nil && *rulesConfig.Deletion {
		rules.Deletion = &github.EmptyRuleParameters{}
	}

	if rulesConfig.NonFastForward != nil && *rulesConfig.NonFastForward {
		rules.NonFastForward = &github.EmptyRuleParameters{}
	}

	if rulesConfig.RequiredLinearHistory != nil && *rulesConfig.RequiredLinearHistory {
		rules.RequiredLinearHistory = &github.EmptyRuleParameters{}
	}

	if rulesConfig.RequiredSignatures != nil && *rulesConfig.RequiredSignatures {
		rules.RequiredSignatures = &github.EmptyRuleParameters{}
	}

	if rulesConfig.Creation != nil && *rulesConfig.Creation {
		rules.Creation = &github.EmptyRuleParameters{}
	}

	if rulesConfig.Update != nil && *rulesConfig.Update {
		rules.Update = &github.UpdateRuleParameters{
			UpdateAllowsFetchAndMerge: true,
		}
	}

	// Code scanning rule
	if rulesConfig.CodeScanning != nil {
		rules.CodeScanning = buildCodeScanningRule(rulesConfig.CodeScanning)
	}

	return rules
}

// buildPullRequestRule converts config PR rule to go-github PullRequestRuleParameters.
func buildPullRequestRule(
	prConfig *configtypes.PullRequestRuleConfig,
	existing *github.RepositoryRuleset,
) *github.PullRequestRuleParameters {
	prRule := &github.PullRequestRuleParameters{}

	if prConfig.DismissStaleReviewsOnPush != nil {
		prRule.DismissStaleReviewsOnPush = *prConfig.DismissStaleReviewsOnPush
	}

	if prConfig.RequireCodeOwnerReview != nil {
		prRule.RequireCodeOwnerReview = *prConfig.RequireCodeOwnerReview
	}

	if prConfig.RequireLastPushApproval != nil {
		prRule.RequireLastPushApproval = *prConfig.RequireLastPushApproval
	}

	if prConfig.RequiredReviewThreadResolution != nil {
		prRule.RequiredReviewThreadResolution = *prConfig.RequiredReviewThreadResolution
	}

	// Apply no-downgrade logic for review count
	if prConfig.RequiredApprovingReviewCount != nil {
		reviewCount := getRequiredReviewCountForRuleset(prConfig, existing)
		prRule.RequiredApprovingReviewCount = reviewCount
	}

	// Allowed merge methods
	if len(prConfig.AllowedMergeMethods) > 0 {
		for _, method := range prConfig.AllowedMergeMethods {
			prRule.AllowedMergeMethods = append(
				prRule.AllowedMergeMethods,
				github.PullRequestMergeMethod(method),
			)
		}
	}

	return prRule
}

// getRequiredReviewCountForRuleset implements no-downgrade logic for ruleset review counts.
func getRequiredReviewCountForRuleset(
	prConfig *configtypes.PullRequestRuleConfig,
	existing *github.RepositoryRuleset,
) int {
	if prConfig.RequiredApprovingReviewCount == nil {
		return 0
	}

	desiredCount := *prConfig.RequiredApprovingReviewCount

	// If no existing ruleset, use desired count
	if existing == nil || existing.Rules == nil || existing.Rules.PullRequest == nil {
		return desiredCount
	}

	currentCount := existing.Rules.PullRequest.RequiredApprovingReviewCount

	// No-downgrade: use the higher of current or desired
	if currentCount > desiredCount {
		return currentCount
	}

	return desiredCount
}

// buildStatusChecksRule converts config status checks to go-github RequiredStatusChecksRuleParameters.
// Returns nil if no status checks would be configured (GitHub API requires at least 1 check).
func buildStatusChecksRule(
	statusConfig *configtypes.StatusChecksRuleConfig,
	existing *github.RepositoryRuleset,
) *github.RequiredStatusChecksRuleParameters {
	rule := &github.RequiredStatusChecksRuleParameters{}

	if statusConfig.StrictRequiredStatusChecksPolicy != nil {
		rule.StrictRequiredStatusChecksPolicy = *statusConfig.StrictRequiredStatusChecksPolicy
	}

	// Handle status checks with hybrid approach:
	// Empty array = inherit repo's existing checks (or skip rule entirely if none exist)
	if len(statusConfig.RequiredStatusChecks) > 0 {
		var checks []*github.RuleStatusCheck

		for _, checkConfig := range statusConfig.RequiredStatusChecks {
			check := &github.RuleStatusCheck{
				Context: checkConfig.Context,
			}

			if checkConfig.IntegrationID != nil {
				check.IntegrationID = checkConfig.IntegrationID
			}

			checks = append(checks, check)
		}

		rule.RequiredStatusChecks = checks
	} else if existing != nil &&
		existing.Rules != nil &&
		existing.Rules.RequiredStatusChecks != nil &&
		len(existing.Rules.RequiredStatusChecks.RequiredStatusChecks) > 0 {
		// Preserve existing checks if config has empty array
		rule.RequiredStatusChecks = existing.Rules.RequiredStatusChecks.RequiredStatusChecks
	}

	// GitHub API requires at least 1 status check - skip rule entirely if empty
	if len(rule.RequiredStatusChecks) == 0 {
		return nil
	}

	return rule
}

// buildCodeScanningRule converts config code scanning to go-github CodeScanningRuleParameters.
func buildCodeScanningRule(
	csConfig *configtypes.CodeScanningRuleConfig,
) *github.CodeScanningRuleParameters {
	rule := &github.CodeScanningRuleParameters{}

	if len(csConfig.CodeScanningTools) > 0 {
		var tools []*github.RuleCodeScanningTool

		for _, toolConfig := range csConfig.CodeScanningTools {
			alertsThreshold := github.CodeScanningAlertsThreshold(toolConfig.AlertsThreshold)
			securityThreshold := github.CodeScanningSecurityAlertsThreshold(
				toolConfig.SecurityAlertsThreshold,
			)

			tool := &github.RuleCodeScanningTool{
				Tool:                    toolConfig.Tool,
				AlertsThreshold:         alertsThreshold,
				SecurityAlertsThreshold: securityThreshold,
			}
			tools = append(tools, tool)
		}

		rule.CodeScanningTools = tools
	}

	return rule
}

// logRulesetOperation logs the planned operation in dry-run mode.
func logRulesetOperation(log *logger.Logger, name string, existing *github.RepositoryRuleset) {
	if existing == nil {
		log.Info("would create ruleset", "name", name)
	} else {
		log.Info("would update ruleset", "name", name, "id", existing.GetID())
	}
}
