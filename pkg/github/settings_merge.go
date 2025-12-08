package github

import (
	"github.com/cockroachdb/errors"
	"go.yaml.in/yaml/v4"

	"github.com/smykla-labs/.github/internal/configtypes"
	"github.com/smykla-labs/.github/pkg/logger"
	"github.com/smykla-labs/.github/pkg/merge"
)

// ErrSettingsMerge indicates a failure during settings merge operation.
var ErrSettingsMerge = errors.New("failed to merge settings")

// mergeStructWithOverrides converts a struct to map, applies merge, and unmarshals back to struct.
// Uses YAML marshal/unmarshal for flexibility with nil pointer handling.
func mergeStructWithOverrides(
	orgStruct any,
	overrides map[string]any,
	strategy configtypes.MergeStrategy,
	resultPtr any,
) error {
	// Marshal struct to YAML
	yamlBytes, err := yaml.Marshal(orgStruct)
	if err != nil {
		return errors.Wrap(ErrSettingsMerge, "marshaling struct to YAML")
	}

	// Parse YAML to map
	orgMap, err := merge.ParseYAML(yamlBytes)
	if err != nil {
		return errors.Wrap(err, "parsing struct YAML to map")
	}

	// Apply merge
	mergedMap, err := merge.MergeYAML(orgMap, overrides, strategy)
	if err != nil {
		return errors.Wrap(err, "applying merge")
	}

	// Marshal merged map back to YAML
	mergedYAML, err := merge.MarshalYAML(mergedMap)
	if err != nil {
		return errors.Wrap(err, "marshaling merged map to YAML")
	}

	// Unmarshal to result struct
	if err := yaml.Unmarshal(mergedYAML, resultPtr); err != nil {
		return errors.Wrap(ErrSettingsMerge, "unmarshaling merged YAML to struct")
	}

	return nil
}

// mergeRepositorySettings merges repository settings with overrides.
func mergeRepositorySettings(
	org *configtypes.RepositorySettingsConfig,
	overrides map[string]any,
	strategy configtypes.MergeStrategy,
) (*configtypes.RepositorySettingsConfig, error) {
	result := &configtypes.RepositorySettingsConfig{}

	if err := mergeStructWithOverrides(org, overrides, strategy, result); err != nil {
		return nil, errors.Wrap(err, "merging repository settings")
	}

	return result, nil
}

// mergeFeaturesSettings merges features settings with overrides.
func mergeFeaturesSettings(
	org *configtypes.FeaturesConfig,
	overrides map[string]any,
	strategy configtypes.MergeStrategy,
) (*configtypes.FeaturesConfig, error) {
	result := &configtypes.FeaturesConfig{}

	if err := mergeStructWithOverrides(org, overrides, strategy, result); err != nil {
		return nil, errors.Wrap(err, "merging features settings")
	}

	return result, nil
}

// mergeSecuritySettings merges security settings with overrides.
func mergeSecuritySettings(
	org *configtypes.SecurityConfig,
	overrides map[string]any,
	strategy configtypes.MergeStrategy,
) (*configtypes.SecurityConfig, error) {
	result := &configtypes.SecurityConfig{}

	if err := mergeStructWithOverrides(org, overrides, strategy, result); err != nil {
		return nil, errors.Wrap(err, "merging security settings")
	}

	return result, nil
}

// mergeBranchProtectionRule merges a branch protection rule with overrides.
func mergeBranchProtectionRule(
	org *configtypes.BranchProtectionRuleConfig,
	overrides map[string]any,
	strategy configtypes.MergeStrategy,
) (*configtypes.BranchProtectionRuleConfig, error) {
	result := &configtypes.BranchProtectionRuleConfig{}

	if err := mergeStructWithOverrides(org, overrides, strategy, result); err != nil {
		return nil, errors.Wrap(err, "merging branch protection rule")
	}

	return result, nil
}

// mergeRulesetConfig merges a ruleset config with overrides.
func mergeRulesetConfig(
	org *configtypes.RulesetConfig,
	overrides map[string]any,
	strategy configtypes.MergeStrategy,
) (*configtypes.RulesetConfig, error) {
	result := &configtypes.RulesetConfig{}

	if err := mergeStructWithOverrides(org, overrides, strategy, result); err != nil {
		return nil, errors.Wrap(err, "merging ruleset config")
	}

	return result, nil
}

// ApplySettingsMerge applies merge configurations to settings.
// It iterates through merge configs and applies overrides to matching sections.
// Returns original settings and nil error if no merge configs exist (graceful no-op).
func ApplySettingsMerge(
	log *logger.Logger,
	orgSettings *SettingsDefinition,
	syncConfig *configtypes.SyncConfig,
) (*SettingsDefinition, error) {
	if syncConfig == nil || len(syncConfig.Sync.Settings.Merge) == 0 {
		return orgSettings, nil
	}

	// Create a copy to avoid mutating the original.
	// Repository, Features, Security are value types (no pointer fields) - safe for shallow copy.
	// BranchProtection and Rulesets slices are deep copied below.
	bpLen := len(orgSettings.BranchProtection)
	rsLen := len(orgSettings.Rulesets)

	result := &SettingsDefinition{
		Repository:       orgSettings.Repository,
		Features:         orgSettings.Features,
		Security:         orgSettings.Security,
		BranchProtection: make([]configtypes.BranchProtectionRuleConfig, bpLen),
		Rulesets:         make([]configtypes.RulesetConfig, rsLen),
	}

	copy(result.BranchProtection, orgSettings.BranchProtection)
	copy(result.Rulesets, orgSettings.Rulesets)

	// Apply each merge configuration
	for _, mergeConfig := range syncConfig.Sync.Settings.Merge {
		if err := applySingleMerge(result, &mergeConfig); err != nil {
			// Log warning and continue - graceful degradation
			log.Warn("failed to apply settings merge",
				"section", mergeConfig.Section,
				"error", err,
			)

			continue
		}
	}

	return result, nil
}

// applySingleMerge applies a single merge configuration to the settings.
func applySingleMerge(
	settings *SettingsDefinition,
	mergeConfig *configtypes.SettingsMergeConfig,
) error {
	section := mergeConfig.Section
	strategy := mergeConfig.Strategy
	overrides := mergeConfig.Overrides

	// Default strategy to deep-merge if not specified
	if strategy == "" {
		strategy = configtypes.MergeStrategyDeep
	}

	switch section {
	case "repository":
		merged, err := mergeRepositorySettings(&settings.Repository, overrides, strategy)
		if err != nil {
			return err
		}

		settings.Repository = *merged

	case "features":
		merged, err := mergeFeaturesSettings(&settings.Features, overrides, strategy)
		if err != nil {
			return err
		}

		settings.Features = *merged

	case "security":
		merged, err := mergeSecuritySettings(&settings.Security, overrides, strategy)
		if err != nil {
			return err
		}

		settings.Security = *merged

	default:
		// Try branch protection pattern match
		if err := tryMergeBranchProtection(settings, section, overrides, strategy); err == nil {
			return nil
		}

		// Try ruleset name match
		if err := tryMergeRuleset(settings, section, overrides, strategy); err == nil {
			return nil
		}

		// Unknown section - skip silently (graceful degradation)
		return errors.Wrapf(ErrSettingsMerge, "unknown section: %s", section)
	}

	return nil
}

// tryMergeBranchProtection attempts to merge a branch protection rule by pattern.
func tryMergeBranchProtection(
	settings *SettingsDefinition,
	pattern string,
	overrides map[string]any,
	strategy configtypes.MergeStrategy,
) error {
	for i := range settings.BranchProtection {
		if settings.BranchProtection[i].Pattern == pattern {
			merged, err := mergeBranchProtectionRule(
				&settings.BranchProtection[i],
				overrides,
				strategy,
			)
			if err != nil {
				return err
			}

			settings.BranchProtection[i] = *merged

			return nil
		}
	}

	return errors.Wrapf(ErrSettingsMerge, "branch protection pattern not found: %s", pattern)
}

// tryMergeRuleset attempts to merge a ruleset by name.
func tryMergeRuleset(
	settings *SettingsDefinition,
	name string,
	overrides map[string]any,
	strategy configtypes.MergeStrategy,
) error {
	for i := range settings.Rulesets {
		if settings.Rulesets[i].Name == name {
			merged, err := mergeRulesetConfig(&settings.Rulesets[i], overrides, strategy)
			if err != nil {
				return err
			}

			settings.Rulesets[i] = *merged

			return nil
		}
	}

	return errors.Wrapf(ErrSettingsMerge, "ruleset not found: %s", name)
}
