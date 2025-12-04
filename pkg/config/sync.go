// Package config provides configuration types and parsing.
package config

import (
	"encoding/json"

	"github.com/cockroachdb/errors"
	"gopkg.in/yaml.v3"
)

// SyncConfig is the root configuration structure.
type SyncConfig struct {
	Sync SyncSettings `json:"sync" jsonschema:"description=Top-level sync configuration controlling label file and smyklot version synchronization behavior" yaml:"sync"`
}

// SyncSettings contains all sync-related settings.
type SyncSettings struct {
	Skip    bool          `json:"skip"    jsonschema:"default=false,description=Skip ALL syncs for this repository. Equivalent to setting labels.skip files.skip and smyklot.skip to true" yaml:"skip"`
	Labels  LabelsConfig  `json:"labels"  jsonschema:"description=Label synchronization configuration"                                                                                     yaml:"labels"`
	Files   FilesConfig   `json:"files"   jsonschema:"description=File synchronization configuration"                                                                                      yaml:"files"`
	Smyklot SmyklotConfig `json:"smyklot" jsonschema:"description=Smyklot version synchronization configuration"                                                                           yaml:"smyklot"`
}

// LabelsConfig controls label synchronization behavior.
type LabelsConfig struct {
	Skip         bool     `json:"skip"          jsonschema:"default=false,description=Skip label synchronization only. File sync still runs unless sync.skip or sync.files.skip is true"                                                                                                                                                                               yaml:"skip"`
	Exclude      []string `json:"exclude"       jsonschema:"description=Label names to exclude from synchronization. These labels will NOT be created/updated in this repository. Existing labels with these names are preserved but not managed,uniqueItems=true,minLength=1,examples=ci/skip-tests|ci/force-full,examples=release/major|release/minor|release/patch" yaml:"exclude"`
	AllowRemoval bool     `json:"allow_removal" jsonschema:"default=false,description=When true labels in this repo that are NOT in the central config will be DELETED. Use with caution - this removes custom labels"                                                                                                                                                 yaml:"allow_removal"`
}

// FilesConfig controls file synchronization behavior.
type FilesConfig struct {
	Skip         bool     `json:"skip"          jsonschema:"default=false,description=Skip file synchronization only. Label sync still runs unless sync.skip or sync.labels.skip is true"                                                                                                                                                                                                                 yaml:"skip"`
	Exclude      []string `json:"exclude"       jsonschema:"description=File paths (relative to repo root) to exclude from sync. These files will NOT be created/updated in this repository. Existing files at these paths are preserved but not managed,uniqueItems=true,minLength=1,pattern=^[^/].*$,examples=CONTRIBUTING.md|CODE_OF_CONDUCT.md,examples=.github/PULL_REQUEST_TEMPLATE.md|SECURITY.md" yaml:"exclude"`
	AllowRemoval bool     `json:"allow_removal" jsonschema:"default=false,description=DANGEROUS: When true files in this repo that are NOT in the central sync config will be DELETED. This can cause data loss. Strongly recommend keeping this false"                                                                                                                                                   yaml:"allow_removal"`
}

// SmyklotConfig controls smyklot version synchronization behavior.
type SmyklotConfig struct {
	Skip bool `json:"skip" jsonschema:"default=false,description=Skip smyklot version synchronization only. Label and file sync still run unless their respective skip flags are set. Use this for repos that don't use smyklot or manage their own versions" yaml:"skip"`
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
