// Package config provides sync configuration parsing.
// Type definitions are in internal/configtypes for minimal import footprint.
package config

import (
	"encoding/json"

	"github.com/cockroachdb/errors"
	"go.yaml.in/yaml/v4"

	"github.com/smykla-skalski/.github/internal/configtypes"
)

// ParseSyncConfig parses sync configuration from YAML or JSON.
func ParseSyncConfig(data []byte) (*configtypes.SyncConfig, error) {
	var cfg configtypes.SyncConfig

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
			return nil, errors.Wrap(err, "parsing sync config as YAML or JSON")
		}
	}

	return &cfg, nil
}

// ParseSyncConfigJSON parses sync configuration from JSON string.
func ParseSyncConfigJSON(jsonStr string) (*configtypes.SyncConfig, error) {
	if jsonStr == "" {
		return &configtypes.SyncConfig{}, nil
	}

	var cfg configtypes.SyncConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return nil, errors.Wrap(err, "parsing sync config JSON")
	}

	return &cfg, nil
}

// GetMergeConfig returns the merge configuration for a specific file path, if configured.
// Returns nil if no merge config exists for the path or if the config is nil.
// Safe to call with nil config - Sync and Files are value types, Merge slice iteration is nil-safe.
func GetMergeConfig(c *configtypes.SyncConfig, path string) *configtypes.FileMergeConfig {
	if c == nil {
		return nil
	}

	for i := range c.Sync.Files.Merge {
		if c.Sync.Files.Merge[i].Path == path {
			return &c.Sync.Files.Merge[i]
		}
	}

	return nil
}
