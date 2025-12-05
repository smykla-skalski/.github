// Package config provides configuration types and parsing.
package config

import (
	"encoding/json"

	"github.com/cockroachdb/errors"
	"gopkg.in/yaml.v3"
)

// ParseSyncConfig parses sync configuration from YAML or JSON.
func ParseSyncConfig(data []byte) (*SyncConfig, error) {
	var config SyncConfig

	if err := yaml.Unmarshal(data, &config); err != nil {
		if jsonErr := json.Unmarshal(data, &config); jsonErr != nil {
			return nil, errors.Wrap(err, "parsing sync config as YAML or JSON")
		}
	}

	return &config, nil
}

// ParseSyncConfigJSON parses sync configuration from JSON string.
func ParseSyncConfigJSON(jsonStr string) (*SyncConfig, error) {
	if jsonStr == "" {
		return &SyncConfig{}, nil
	}

	var config SyncConfig
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, errors.Wrap(err, "parsing sync config JSON")
	}

	return &config, nil
}
