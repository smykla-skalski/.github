// Package merge provides strategies for merging organization and repository file contents.
package merge

import (
	"encoding/json"
	"maps"

	"github.com/cockroachdb/errors"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"gopkg.in/yaml.v3"

	"github.com/smykla-labs/.github/pkg/config"
	"github.com/smykla-labs/.github/pkg/github"
)

// DeepMerge recursively merges two maps using RFC 7396 JSON Merge Patch semantics.
//   - Nested objects are merged recursively
//   - Arrays are replaced, not merged
//   - Null values in override explicitly remove keys from base
//   - Type mismatches are handled per RFC 7396 (override wins)
func DeepMerge(base, override map[string]any) (map[string]any, error) {
	// Handle nil cases
	if base == nil && override == nil {
		return make(map[string]any), nil
	}

	if base == nil {
		base = make(map[string]any)
	}

	if override == nil {
		result := make(map[string]any, len(base))
		maps.Copy(result, base)

		return result, nil
	}

	// Convert maps to JSON
	baseJSON, err := json.Marshal(base)
	if err != nil {
		return nil, errors.Wrap(github.ErrMergeParseError, "marshaling base map to JSON")
	}

	overrideJSON, err := json.Marshal(override)
	if err != nil {
		return nil, errors.Wrap(
			github.ErrMergeParseError,
			"marshaling override map to JSON",
		)
	}

	// Apply RFC 7396 merge patch
	mergedJSON, err := jsonpatch.MergePatch(baseJSON, overrideJSON)
	if err != nil {
		return nil, errors.Wrap(github.ErrMergeParseError, "applying merge patch")
	}

	// Convert back to map
	var result map[string]any
	if err := json.Unmarshal(mergedJSON, &result); err != nil {
		return nil, errors.Wrap(
			github.ErrMergeParseError,
			"unmarshaling merged JSON to map",
		)
	}

	return result, nil
}

// ShallowMerge merges two maps at the top level only.
//   - Only top-level keys are merged
//   - Nested objects are replaced if overridden, not merged recursively
//   - Null values in override explicitly remove keys from base
func ShallowMerge(base, override map[string]any) (map[string]any, error) {
	if base == nil {
		base = make(map[string]any)
	}

	if override == nil {
		// Return a copy of base
		result := make(map[string]any, len(base))
		maps.Copy(result, base)

		return result, nil
	}

	// Create result with base values
	result := make(map[string]any, len(base))
	maps.Copy(result, base)

	// Apply override values at top level only
	for key, overrideVal := range override {
		if overrideVal == nil {
			// Explicit nil means delete the key
			delete(result, key)

			continue
		}

		// Replace with override value (no recursion)
		result[key] = overrideVal
	}

	return result, nil
}

// MergeJSON merges two JSON objects using the specified strategy.
func MergeJSON(
	base, override map[string]any,
	strategy config.MergeStrategy,
) (map[string]any, error) {
	switch strategy {
	case config.MergeStrategyDeep, config.MergeStrategyOverlay:
		return DeepMerge(base, override)
	case config.MergeStrategyShallow:
		return ShallowMerge(base, override)
	default:
		return nil, errors.Wrapf(
			github.ErrMergeUnknownStrategy,
			"strategy: %q",
			strategy,
		)
	}
}

// MergeYAML merges two YAML objects using the specified strategy.
// YAML is converted to JSON internally, merged, then converted back.
func MergeYAML(
	base, override map[string]any,
	strategy config.MergeStrategy,
) (map[string]any, error) {
	// YAML and JSON have compatible data models, so we can use the same merge logic
	return MergeJSON(base, override, strategy)
}

// ParseJSON parses JSON bytes into a map.
func ParseJSON(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, errors.Wrap(github.ErrMergeParseError, "parsing JSON")
	}

	return result, nil
}

// ParseYAML parses YAML bytes into a map.
func ParseYAML(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, errors.Wrap(github.ErrMergeParseError, "parsing YAML")
	}

	return result, nil
}

// MarshalJSON converts a map to JSON bytes.
func MarshalJSON(data map[string]any) ([]byte, error) {
	result, err := json.Marshal(data)
	if err != nil {
		return nil, errors.Wrap(github.ErrMergeParseError, "marshaling to JSON")
	}

	return result, nil
}

// MarshalYAML converts a map to YAML bytes.
func MarshalYAML(data map[string]any) ([]byte, error) {
	result, err := yaml.Marshal(data)
	if err != nil {
		return nil, errors.Wrap(github.ErrMergeParseError, "marshaling to YAML")
	}

	return result, nil
}
