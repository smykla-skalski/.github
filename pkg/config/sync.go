// Package config provides configuration types and parsing.
package config

import (
	"encoding/json"

	"github.com/cockroachdb/errors"
	"gopkg.in/yaml.v3"
)

// SyncConfig is the root configuration structure.
type SyncConfig struct {
	Sync SyncSettings `json:"sync" yaml:"sync"`
}

// SyncSettings contains all sync-related settings.
type SyncSettings struct {
	Skip    bool          `json:"skip"    yaml:"skip"`
	Labels  LabelsConfig  `json:"labels"  yaml:"labels"`
	Files   FilesConfig   `json:"files"   yaml:"files"`
	Smyklot SmyklotConfig `json:"smyklot" yaml:"smyklot"`
}

// LabelsConfig controls label synchronization behavior.
type LabelsConfig struct {
	Skip         bool     `json:"skip"          yaml:"skip"`
	Exclude      []string `json:"exclude"       yaml:"exclude"`
	AllowRemoval bool     `json:"allow_removal" yaml:"allow_removal"`
}

// FilesConfig controls file synchronization behavior.
type FilesConfig struct {
	Skip         bool     `json:"skip"          yaml:"skip"`
	Exclude      []string `json:"exclude"       yaml:"exclude"`
	AllowRemoval bool     `json:"allow_removal" yaml:"allow_removal"`
}

// SmyklotConfig controls smyklot version synchronization behavior.
type SmyklotConfig struct {
	Skip bool `json:"skip" yaml:"skip"`
}

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
