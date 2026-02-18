package github

import (
	"testing"

	"github.com/google/go-github/v80/github"

	"github.com/smykla-skalski/.github/internal/configtypes"
)

func TestBuildRulesetFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   configtypes.RulesetConfig
		existing *github.RepositoryRuleset
		validate func(t *testing.T, ruleset *github.RepositoryRuleset)
	}{
		{
			name: "creates basic ruleset with target and enforcement",
			config: configtypes.RulesetConfig{
				Name:        "test-ruleset",
				Target:      "branch",
				Enforcement: "active",
			},
			existing: nil,
			validate: func(t *testing.T, ruleset *github.RepositoryRuleset) {
				if ruleset.Name != "test-ruleset" {
					t.Errorf("expected name 'test-ruleset', got %q", ruleset.Name)
				}

				if ruleset.Target == nil || *ruleset.Target != "branch" {
					t.Errorf("expected target 'branch', got %v", ruleset.Target)
				}

				if string(ruleset.Enforcement) != "active" {
					t.Errorf("expected enforcement 'active', got %q", ruleset.Enforcement)
				}
			},
		},
		{
			name: "converts ref name conditions",
			config: configtypes.RulesetConfig{
				Name:        "main-protection",
				Target:      "branch",
				Enforcement: "active",
				Conditions: &configtypes.RulesetConditionsConfig{
					RefName: &configtypes.RefNameCondition{
						Include: []string{"refs/heads/main"},
						Exclude: []string{"refs/heads/test"},
					},
				},
			},
			existing: nil,
			validate: func(t *testing.T, ruleset *github.RepositoryRuleset) {
				if ruleset.Conditions == nil {
					t.Fatal("expected conditions to be set")
				}

				if ruleset.Conditions.RefName == nil {
					t.Fatal("expected ref_name condition to be set")
				}

				if len(ruleset.Conditions.RefName.Include) != 1 ||
					ruleset.Conditions.RefName.Include[0] != "refs/heads/main" {
					t.Errorf("unexpected include patterns: %v", ruleset.Conditions.RefName.Include)
				}

				if len(ruleset.Conditions.RefName.Exclude) != 1 ||
					ruleset.Conditions.RefName.Exclude[0] != "refs/heads/test" {
					t.Errorf("unexpected exclude patterns: %v", ruleset.Conditions.RefName.Exclude)
				}
			},
		},
		{
			name: "converts bypass actors",
			config: configtypes.RulesetConfig{
				Name:        "with-bypass",
				Target:      "branch",
				Enforcement: "active",
				BypassActors: []configtypes.BypassActorConfig{
					{
						ActorID:    5,
						ActorType:  "OrganizationAdmin",
						BypassMode: "always",
					},
					{
						ActorID:    1197525,
						ActorType:  "Integration",
						BypassMode: "pull_request",
					},
				},
			},
			existing: nil,
			validate: func(t *testing.T, ruleset *github.RepositoryRuleset) {
				if len(ruleset.BypassActors) != 2 {
					t.Fatalf("expected 2 bypass actors, got %d", len(ruleset.BypassActors))
				}

				actor1 := ruleset.BypassActors[0]
				if actor1.ActorID == nil || *actor1.ActorID != 5 {
					t.Errorf("expected actor_id 5, got %v", actor1.ActorID)
				}

				if actor1.ActorType == nil || *actor1.ActorType != "OrganizationAdmin" {
					t.Errorf("expected actor_type 'OrganizationAdmin', got %v", actor1.ActorType)
				}

				if actor1.BypassMode == nil || *actor1.BypassMode != "always" {
					t.Errorf("expected bypass_mode 'always', got %v", actor1.BypassMode)
				}

				actor2 := ruleset.BypassActors[1]
				if actor2.ActorID == nil || *actor2.ActorID != 1197525 {
					t.Errorf("expected actor_id 1197525, got %v", actor2.ActorID)
				}
			},
		},
		{
			name: "converts boolean rules",
			config: configtypes.RulesetConfig{
				Name:        "with-bool-rules",
				Target:      "branch",
				Enforcement: "active",
				Rules: &configtypes.RulesetRulesConfig{
					Deletion:              new(true),
					NonFastForward:        new(true),
					RequiredLinearHistory: new(true),
					RequiredSignatures:    new(true),
				},
			},
			existing: nil,
			validate: func(t *testing.T, ruleset *github.RepositoryRuleset) {
				if ruleset.Rules == nil {
					t.Fatal("expected rules to be set")
				}

				if ruleset.Rules.Deletion == nil {
					t.Error("expected deletion rule to be set")
				}

				if ruleset.Rules.NonFastForward == nil {
					t.Error("expected non_fast_forward rule to be set")
				}

				if ruleset.Rules.RequiredLinearHistory == nil {
					t.Error("expected required_linear_history rule to be set")
				}

				if ruleset.Rules.RequiredSignatures == nil {
					t.Error("expected required_signatures rule to be set")
				}
			},
		},
		{
			name: "converts pull request rules",
			config: configtypes.RulesetConfig{
				Name:        "with-pr-rules",
				Target:      "branch",
				Enforcement: "active",
				Rules: &configtypes.RulesetRulesConfig{
					PullRequest: &configtypes.PullRequestRuleConfig{
						RequiredApprovingReviewCount: new(2),
						DismissStaleReviewsOnPush:    new(true),
						RequireCodeOwnerReview:       new(true),
						RequireLastPushApproval:      new(true),
						AllowedMergeMethods:          []string{"squash"},
					},
				},
			},
			existing: nil,
			validate: func(t *testing.T, ruleset *github.RepositoryRuleset) {
				if ruleset.Rules == nil || ruleset.Rules.PullRequest == nil {
					t.Fatal("expected pull request rule to be set")
				}

				pr := ruleset.Rules.PullRequest
				if pr.RequiredApprovingReviewCount != 2 {
					t.Errorf("expected 2 required reviews, got %d", pr.RequiredApprovingReviewCount)
				}

				if !pr.DismissStaleReviewsOnPush {
					t.Error("expected dismiss_stale_reviews_on_push to be true")
				}

				if !pr.RequireCodeOwnerReview {
					t.Error("expected require_code_owner_review to be true")
				}

				if !pr.RequireLastPushApproval {
					t.Error("expected require_last_push_approval to be true")
				}

				if len(pr.AllowedMergeMethods) != 1 ||
					pr.AllowedMergeMethods[0] != "squash" {
					t.Errorf("unexpected allowed merge methods: %v", pr.AllowedMergeMethods)
				}
			},
		},
		{
			name: "converts status checks rule",
			config: configtypes.RulesetConfig{
				Name:        "with-status-checks",
				Target:      "branch",
				Enforcement: "active",
				Rules: &configtypes.RulesetRulesConfig{
					RequiredStatusChecks: &configtypes.StatusChecksRuleConfig{
						StrictRequiredStatusChecksPolicy: new(true),
						RequiredStatusChecks: []configtypes.StatusCheckConfig{
							{Context: "ci/lint"},
							{Context: "ci/test"},
						},
					},
				},
			},
			existing: nil,
			validate: func(t *testing.T, ruleset *github.RepositoryRuleset) {
				if ruleset.Rules == nil || ruleset.Rules.RequiredStatusChecks == nil {
					t.Fatal("expected required status checks rule to be set")
				}

				checks := ruleset.Rules.RequiredStatusChecks
				if !checks.StrictRequiredStatusChecksPolicy {
					t.Error("expected strict policy to be true")
				}

				if len(checks.RequiredStatusChecks) != 2 {
					t.Fatalf("expected 2 status checks, got %d", len(checks.RequiredStatusChecks))
				}

				if checks.RequiredStatusChecks[0].Context != "ci/lint" {
					t.Errorf("expected first check 'ci/lint', got %q",
						checks.RequiredStatusChecks[0].Context)
				}
			},
		},
		{
			name: "converts code scanning rule",
			config: configtypes.RulesetConfig{
				Name:        "with-code-scanning",
				Target:      "branch",
				Enforcement: "active",
				Rules: &configtypes.RulesetRulesConfig{
					CodeScanning: &configtypes.CodeScanningRuleConfig{
						CodeScanningTools: []configtypes.CodeScanningToolConfig{
							{
								Tool:                    "CodeQL",
								AlertsThreshold:         "errors",
								SecurityAlertsThreshold: "high_or_higher",
							},
						},
					},
				},
			},
			existing: nil,
			validate: func(t *testing.T, ruleset *github.RepositoryRuleset) {
				if ruleset.Rules == nil || ruleset.Rules.CodeScanning == nil {
					t.Fatal("expected code scanning rule to be set")
				}

				tools := ruleset.Rules.CodeScanning.CodeScanningTools
				if len(tools) != 1 {
					t.Fatalf("expected 1 code scanning tool, got %d", len(tools))
				}

				if tools[0].Tool != "CodeQL" {
					t.Errorf("expected tool 'CodeQL', got %q", tools[0].Tool)
				}

				if string(tools[0].AlertsThreshold) != "errors" {
					t.Errorf("expected alerts threshold 'errors', got %q", tools[0].AlertsThreshold)
				}

				if string(tools[0].SecurityAlertsThreshold) != "high_or_higher" {
					t.Errorf("expected security alerts threshold 'high_or_higher', got %q",
						tools[0].SecurityAlertsThreshold)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleset := buildRulesetFromConfig(tt.config, tt.existing)
			tt.validate(t, ruleset)
		})
	}
}

func TestGetRequiredReviewCountForRuleset(t *testing.T) {
	tests := []struct {
		name          string
		prConfig      *configtypes.PullRequestRuleConfig
		existingRules *github.RepositoryRulesetRules
		expectedCount int
		testNoUpgrade bool
	}{
		{
			name: "uses desired count when no existing ruleset",
			prConfig: &configtypes.PullRequestRuleConfig{
				RequiredApprovingReviewCount: new(1),
			},
			existingRules: nil,
			expectedCount: 1,
		},
		{
			name: "uses desired count when existing has no PR rule",
			prConfig: &configtypes.PullRequestRuleConfig{
				RequiredApprovingReviewCount: new(2),
			},
			existingRules: &github.RepositoryRulesetRules{},
			expectedCount: 2,
		},
		{
			name: "uses desired count when higher than existing",
			prConfig: &configtypes.PullRequestRuleConfig{
				RequiredApprovingReviewCount: new(3),
			},
			existingRules: &github.RepositoryRulesetRules{
				PullRequest: &github.PullRequestRuleParameters{
					RequiredApprovingReviewCount: 2,
				},
			},
			expectedCount: 3,
		},
		{
			name: "keeps existing count when higher than desired (no-downgrade)",
			prConfig: &configtypes.PullRequestRuleConfig{
				RequiredApprovingReviewCount: new(1),
			},
			existingRules: &github.RepositoryRulesetRules{
				PullRequest: &github.PullRequestRuleParameters{
					RequiredApprovingReviewCount: 3,
				},
			},
			expectedCount: 3,
			testNoUpgrade: true,
		},
		{
			name: "uses zero when config specifies zero",
			prConfig: &configtypes.PullRequestRuleConfig{
				RequiredApprovingReviewCount: new(0),
			},
			existingRules: nil,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create existing ruleset if we have existing rules
			var existing *github.RepositoryRuleset
			if tt.existingRules != nil {
				existing = &github.RepositoryRuleset{
					Rules: tt.existingRules,
				}
			}

			count := getRequiredReviewCountForRuleset(tt.prConfig, existing)

			if count != tt.expectedCount {
				t.Errorf("expected count %d, got %d", tt.expectedCount, count)
			}

			// Additional validation for no-downgrade test
			if tt.testNoUpgrade && count < existing.Rules.PullRequest.RequiredApprovingReviewCount {
				t.Error("no-downgrade protection failed: count was downgraded")
			}
		})
	}
}

func TestBuildStatusChecksRulePreservesExisting(t *testing.T) {
	// Test that empty status checks in config preserves existing checks
	config := &configtypes.StatusChecksRuleConfig{
		StrictRequiredStatusChecksPolicy: new(true),
		RequiredStatusChecks:             []configtypes.StatusCheckConfig{}, // Empty
	}

	existingCheck := &github.RuleStatusCheck{
		Context: "existing-check",
	}

	existing := &github.RepositoryRuleset{
		Rules: &github.RepositoryRulesetRules{
			RequiredStatusChecks: &github.RequiredStatusChecksRuleParameters{
				RequiredStatusChecks: []*github.RuleStatusCheck{existingCheck},
			},
		},
	}

	rule := buildStatusChecksRule(config, existing)

	if len(rule.RequiredStatusChecks) != 1 {
		t.Fatalf("expected 1 status check (preserved), got %d", len(rule.RequiredStatusChecks))
	}

	if rule.RequiredStatusChecks[0].Context != "existing-check" {
		t.Errorf("expected preserved check 'existing-check', got %q",
			rule.RequiredStatusChecks[0].Context)
	}
}

func TestBuildStatusChecksRuleOverridesExisting(t *testing.T) {
	// Test that explicit status checks in config override existing checks
	config := &configtypes.StatusChecksRuleConfig{
		StrictRequiredStatusChecksPolicy: new(true),
		RequiredStatusChecks: []configtypes.StatusCheckConfig{
			{Context: "new-check-1"},
			{Context: "new-check-2"},
		},
	}

	existingCheck := &github.RuleStatusCheck{
		Context: "old-check",
	}

	existing := &github.RepositoryRuleset{
		Rules: &github.RepositoryRulesetRules{
			RequiredStatusChecks: &github.RequiredStatusChecksRuleParameters{
				RequiredStatusChecks: []*github.RuleStatusCheck{existingCheck},
			},
		},
	}

	rule := buildStatusChecksRule(config, existing)

	if len(rule.RequiredStatusChecks) != 2 {
		t.Fatalf("expected 2 status checks (overridden), got %d", len(rule.RequiredStatusChecks))
	}

	if rule.RequiredStatusChecks[0].Context != "new-check-1" {
		t.Errorf("expected 'new-check-1', got %q", rule.RequiredStatusChecks[0].Context)
	}

	if rule.RequiredStatusChecks[1].Context != "new-check-2" {
		t.Errorf("expected 'new-check-2', got %q", rule.RequiredStatusChecks[1].Context)
	}
}
