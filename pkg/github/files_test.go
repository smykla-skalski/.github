package github

import (
	"bytes"
	"testing"

	"github.com/smykla-skalski/.github/internal/configtypes"
)

func TestRenderFileTemplate(t *testing.T) {
	tests := []struct {
		name          string
		content       []byte
		defaultBranch string
		want          []byte
	}{
		{
			name:          "replaces single placeholder",
			content:       []byte(`branches: ["{{DEFAULT_BRANCH}}"]`),
			defaultBranch: "main",
			want:          []byte(`branches: ["main"]`),
		},
		{
			name:          "replaces multiple placeholders",
			content:       []byte(`branch: {{DEFAULT_BRANCH}}, target: {{DEFAULT_BRANCH}}`),
			defaultBranch: "develop",
			want:          []byte(`branch: develop, target: develop`),
		},
		{
			name:          "returns unchanged when no placeholder",
			content:       []byte(`no placeholders here`),
			defaultBranch: "main",
			want:          []byte(`no placeholders here`),
		},
		{
			name:          "handles empty content",
			content:       []byte{},
			defaultBranch: "main",
			want:          []byte{},
		},
		{
			name:          "case sensitive - lowercase not replaced",
			content:       []byte(`{{default_branch}}`),
			defaultBranch: "main",
			want:          []byte(`{{default_branch}}`),
		},
		{
			name:          "empty default branch returns unchanged",
			content:       []byte(`branch: {{DEFAULT_BRANCH}}`),
			defaultBranch: "",
			want:          []byte(`branch: {{DEFAULT_BRANCH}}`),
		},
		{
			name:          "malformed - missing closing braces",
			content:       []byte(`{{DEFAULT_BRANCH`),
			defaultBranch: "main",
			want:          []byte(`{{DEFAULT_BRANCH`),
		},
		{
			name:          "malformed - partial match",
			content:       []byte(`{{DEFAULT_BRANC}}`),
			defaultBranch: "main",
			want:          []byte(`{{DEFAULT_BRANC}}`),
		},
		{
			name:          "malformed - extra braces",
			content:       []byte(`{{{DEFAULT_BRANCH}}}`),
			defaultBranch: "main",
			want:          []byte(`{main}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderFileTemplate(tt.content, tt.defaultBranch)

			if !bytes.Equal(got, tt.want) {
				t.Errorf("renderFileTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEffectiveMergeStrategy(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		mergeConfig *configtypes.FileMergeConfig
		want        configtypes.MergeStrategy
	}{
		{
			name: "nil config has no merge strategy",
			path: "renovate.json",
			want: "",
		},
		{
			name:        "default JSON strategy is deep merge",
			path:        "renovate.json",
			mergeConfig: &configtypes.FileMergeConfig{},
			want:        configtypes.MergeStrategyDeep,
		},
		{
			name:        "default YAML strategy is deep merge",
			path:        ".github/settings.yml",
			mergeConfig: &configtypes.FileMergeConfig{},
			want:        configtypes.MergeStrategyDeep,
		},
		{
			name:        "default Markdown strategy is markdown",
			path:        "CONTRIBUTING.md",
			mergeConfig: &configtypes.FileMergeConfig{},
			want:        configtypes.MergeStrategyMarkdown,
		},
		{
			name: "explicit strategy wins",
			path: "renovate.json",
			mergeConfig: &configtypes.FileMergeConfig{
				Strategy: configtypes.MergeStrategyShallow,
			},
			want: configtypes.MergeStrategyShallow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveMergeStrategy(tt.path, tt.mergeConfig)
			if got != tt.want {
				t.Errorf("effectiveMergeStrategy() = %q, want %q", got, tt.want)
			}
		})
	}
}
