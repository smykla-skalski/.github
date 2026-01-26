package github

import (
	"testing"
)

func TestApplyVersionReplacements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		version     string
		tag         string
		wantContent string
		wantChanged bool
		description string
	}{
		{
			name:        "replace GitHub Action version",
			content:     "uses: smykla-skalski/smyklot@v1.0.0",
			version:     "2.0.0",
			tag:         "v2.0.0",
			wantContent: "uses: smykla-skalski/smyklot@v2.0.0",
			wantChanged: true,
			description: "Should replace GitHub Action version tag",
		},
		{
			name:        "replace Docker image version",
			content:     "image: ghcr.io/smykla-skalski/smyklot:1.0.0",
			version:     "2.0.0",
			tag:         "v2.0.0",
			wantContent: "image: ghcr.io/smykla-skalski/smyklot:2.0.0",
			wantChanged: true,
			description: "Should replace Docker image version",
		},
		{
			name: "replace both versions in workflow",
			content: `name: Test
jobs:
  test:
    steps:
      - uses: smykla-skalski/smyklot@v1.0.0
      - run: docker pull ghcr.io/smykla-skalski/smyklot:1.0.0`,
			version: "2.0.0",
			tag:     "v2.0.0",
			wantContent: `name: Test
jobs:
  test:
    steps:
      - uses: smykla-skalski/smyklot@v2.0.0
      - run: docker pull ghcr.io/smykla-skalski/smyklot:2.0.0`,
			wantChanged: true,
			description: "Should replace both patterns in complete workflow",
		},
		{
			name:        "no change when versions already match",
			content:     "uses: smykla-skalski/smyklot@v2.0.0",
			version:     "2.0.0",
			tag:         "v2.0.0",
			wantContent: "uses: smykla-skalski/smyklot@v2.0.0",
			wantChanged: false,
			description: "Should not change when version already current",
		},
		{
			name:        "no change when no smyklot references",
			content:     "uses: other/action@v1.0.0",
			version:     "2.0.0",
			tag:         "v2.0.0",
			wantContent: "uses: other/action@v1.0.0",
			wantChanged: false,
			description: "Should not change non-smyklot references",
		},
		{
			name:        "security: reject malicious GitHub Action URL in query param",
			content:     "url: https://evil.com?ref=smykla-skalski/smyklot@v1.0.0",
			version:     "2.0.0",
			tag:         "v2.0.0",
			wantContent: "url: https://evil.com?ref=smykla-skalski/smyklot@v1.0.0",
			wantChanged: false,
			description: "Should NOT match pattern embedded in URL query params",
		},
		{
			name:        "security: reject malicious Docker image in URL path",
			content:     "url: https://evil.com/ghcr.io/smykla-skalski/smyklot:1.0.0/malware",
			version:     "2.0.0",
			tag:         "v2.0.0",
			wantContent: "url: https://evil.com/ghcr.io/smykla-skalski/smyklot:1.0.0/malware",
			wantChanged: false,
			description: "Should NOT match pattern embedded in URL path",
		},
		{
			name:        "security: reject version suffix bypass",
			content:     "uses: smykla-skalski/smyklot@v1.0.0-malicious",
			version:     "2.0.0",
			tag:         "v2.0.0",
			wantContent: "uses: smykla-skalski/smyklot@v1.0.0-malicious",
			wantChanged: false,
			description: "Should NOT match version with suffix (word boundary protection)",
		},
		{
			name:        "security: reject Docker version suffix bypass",
			content:     "image: ghcr.io/smykla-skalski/smyklot:1.0.0-alpine",
			version:     "2.0.0",
			tag:         "v2.0.0",
			wantContent: "image: ghcr.io/smykla-skalski/smyklot:1.0.0-alpine",
			wantChanged: false,
			description: "Should NOT match Docker tag with suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotContent, gotChanged := applyVersionReplacements(tt.content, tt.version, tt.tag)

			if gotContent != tt.wantContent {
				t.Errorf("applyVersionReplacements() content = %q, want %q\nDescription: %s",
					gotContent, tt.wantContent, tt.description)
			}

			if gotChanged != tt.wantChanged {
				t.Errorf("applyVersionReplacements() changed = %v, want %v\nDescription: %s",
					gotChanged, tt.wantChanged, tt.description)
			}
		})
	}
}
