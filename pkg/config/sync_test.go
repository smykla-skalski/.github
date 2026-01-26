package config

import (
	"testing"

	"github.com/smykla-skalski/.github/internal/configtypes"
)

func TestParseSyncConfigJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(*testing.T, *configtypes.SyncConfig)
	}{
		{
			name:    "empty string returns default config",
			input:   "",
			wantErr: false,
			validate: func(t *testing.T, cfg *configtypes.SyncConfig) {
				if cfg == nil {
					t.Fatal("expected non-nil config")
				}
			},
		},
		{
			name:    "valid JSON with all fields",
			input:   `{"sync":{"skip":true,"labels":{"skip":false,"exclude":["test"],"allow_removal":true}}}`,
			wantErr: false,
			validate: func(t *testing.T, cfg *configtypes.SyncConfig) {
				if !cfg.Sync.Skip {
					t.Error("expected sync.skip to be true")
				}

				if cfg.Sync.Labels.Skip {
					t.Error("expected sync.labels.skip to be false")
				}

				if !cfg.Sync.Labels.AllowRemoval {
					t.Error("expected sync.labels.allow_removal to be true")
				}

				if len(cfg.Sync.Labels.Exclude) != 1 || cfg.Sync.Labels.Exclude[0] != "test" {
					t.Errorf("expected exclude to contain 'test', got %v", cfg.Sync.Labels.Exclude)
				}
			},
		},
		{
			name:    "invalid JSON returns error",
			input:   `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseSyncConfigJSON(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSyncConfigJSON() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestParseSyncConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantErr  bool
		validate func(*testing.T, *configtypes.SyncConfig)
	}{
		{
			name: "valid YAML",
			input: []byte(`
sync:
  skip: true
  labels:
    exclude:
      - test1
      - test2
    allow_removal: true
`),
			wantErr: false,
			validate: func(t *testing.T, cfg *configtypes.SyncConfig) {
				if !cfg.Sync.Skip {
					t.Error("expected sync.skip to be true")
				}

				if len(cfg.Sync.Labels.Exclude) != 2 {
					t.Errorf("expected 2 excluded labels, got %d", len(cfg.Sync.Labels.Exclude))
				}
			},
		},
		{
			name:    "valid JSON",
			input:   []byte(`{"sync":{"labels":{"exclude":["test"]}}}`),
			wantErr: false,
			validate: func(t *testing.T, cfg *configtypes.SyncConfig) {
				if len(cfg.Sync.Labels.Exclude) != 1 {
					t.Errorf("expected 1 excluded label, got %d", len(cfg.Sync.Labels.Exclude))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseSyncConfig(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSyncConfig() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}
