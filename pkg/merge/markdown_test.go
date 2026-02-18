package merge_test

import (
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/smykla-skalski/.github/internal/configtypes"
	"github.com/smykla-skalski/.github/pkg/merge"
)

func TestMergeMarkdown_After(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     string
		sections []configtypes.MarkdownSection
		want     string
	}{
		{
			name: "insert after section",
			base: "# Title\n\nIntro text.\n\n## Prerequisites\n\nInstall Go.\n\n## Getting Started\n\nStart here.",
			sections: []configtypes.MarkdownSection{
				{
					Action:  configtypes.MarkdownActionAfter,
					Heading: "Prerequisites",
					Content: "### Custom Section\n\nCustom content here.",
				},
			},
			want: "# Title\n\nIntro text.\n\n## Prerequisites\n\nInstall Go.\n\n### Custom Section\n\nCustom content here.\n\n## Getting Started\n\nStart here.",
		},
		{
			name: "insert after section with subsections",
			base: "# Title\n\n## Section A\n\nContent A.\n\n### Subsection A1\n\nSub content.\n\n## Section B\n\nContent B.",
			sections: []configtypes.MarkdownSection{
				{
					Action:  configtypes.MarkdownActionAfter,
					Heading: "Section A",
					Content: "## Inserted Section\n\nInserted content.",
				},
			},
			want: "# Title\n\n## Section A\n\nContent A.\n\n### Subsection A1\n\nSub content.\n\n## Inserted Section\n\nInserted content.\n\n## Section B\n\nContent B.",
		},
		{
			name: "insert after last section",
			base: "# Title\n\n## Last Section\n\nLast content.",
			sections: []configtypes.MarkdownSection{
				{
					Action:  configtypes.MarkdownActionAfter,
					Heading: "Last Section",
					Content: "## Appended\n\nNew content.",
				},
			},
			want: "# Title\n\n## Last Section\n\nLast content.\n\n## Appended\n\nNew content.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge.MergeMarkdown([]byte(tt.base), tt.sections)
			if err != nil {
				t.Fatalf("MergeMarkdown() error = %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("MergeMarkdown() =\n%s\nwant:\n%s", string(got), tt.want)
			}
		})
	}
}

func TestMergeMarkdown_Before(t *testing.T) {
	t.Parallel()

	base := "# Title\n\n## Section A\n\nContent A.\n\n## Section B\n\nContent B."
	sections := []configtypes.MarkdownSection{
		{
			Action:  configtypes.MarkdownActionBefore,
			Heading: "Section B",
			Content: "## Inserted Ahead\n\nPreceding content.",
		},
	}

	got, err := merge.MergeMarkdown([]byte(base), sections)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	want := "# Title\n\n## Section A\n\nContent A.\n\n## Inserted Ahead\n\nPreceding content.\n\n## Section B\n\nContent B."
	if string(got) != want {
		t.Errorf("MergeMarkdown() =\n%s\nwant:\n%s", string(got), want)
	}
}

func TestMergeMarkdown_Replace(t *testing.T) {
	t.Parallel()

	base := "# Title\n\n## Old Section\n\nOld content.\n\n### Old Sub\n\nOld sub content.\n\n## Next Section\n\nNext content."
	sections := []configtypes.MarkdownSection{
		{
			Action:  configtypes.MarkdownActionReplace,
			Heading: "Old Section",
			Content: "## New Section\n\nNew content.",
		},
	}

	got, err := merge.MergeMarkdown([]byte(base), sections)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	want := "# Title\n\n## New Section\n\nNew content.\n\n## Next Section\n\nNext content."
	if string(got) != want {
		t.Errorf("MergeMarkdown() =\n%s\nwant:\n%s", string(got), want)
	}
}

func TestMergeMarkdown_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     string
		sections []configtypes.MarkdownSection
		want     string
	}{
		{
			name: "delete middle section",
			base: "# Title\n\n## Retained\n\nStays here.\n\n## Remove Me\n\nGone soon.\n\n## Also Retained\n\nAlso stays.",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionDelete, Heading: "Remove Me"},
			},
			want: "# Title\n\n## Retained\n\nStays here.\n\n## Also Retained\n\nAlso stays.",
		},
		{
			name: "delete last section",
			base: "# Title\n\n## Stays\n\nPersistent content.\n\n## Remove This\n\nEphemeral content.",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionDelete, Heading: "Remove This"},
			},
			want: "# Title\n\n## Stays\n\nPersistent content.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge.MergeMarkdown([]byte(tt.base), tt.sections)
			if err != nil {
				t.Fatalf("MergeMarkdown() error = %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("MergeMarkdown() =\n%s\nwant:\n%s", string(got), tt.want)
			}
		})
	}
}

func TestMergeMarkdown_Append(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     string
		sections []configtypes.MarkdownSection
		want     string
	}{
		{
			name: "append to document",
			base: "# Title\n\nContent.",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionAppend, Content: "## Closing\n\nEnd matter."},
			},
			want: "# Title\n\nContent.\n\n## Closing\n\nEnd matter.",
		},
		{
			name: "append to empty document",
			base: "",
			sections: []configtypes.MarkdownSection{
				{
					Action:  configtypes.MarkdownActionAppend,
					Content: "## Fresh Section\n\nStarting content.",
				},
			},
			want: "## Fresh Section\n\nStarting content.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge.MergeMarkdown([]byte(tt.base), tt.sections)
			if err != nil {
				t.Fatalf("MergeMarkdown() error = %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("MergeMarkdown() =\n%s\nwant:\n%s", string(got), tt.want)
			}
		})
	}
}

func TestMergeMarkdown_Prepend(t *testing.T) {
	t.Parallel()

	base := "# Title\n\nContent."
	sections := []configtypes.MarkdownSection{
		{
			Action:  configtypes.MarkdownActionPrepend,
			Content: "> **Note**: This is a custom notice.",
		},
	}

	got, err := merge.MergeMarkdown([]byte(base), sections)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	want := "> **Note**: This is a custom notice.\n\n# Title\n\nContent."
	if string(got) != want {
		t.Errorf("MergeMarkdown() =\n%s\nwant:\n%s", string(got), want)
	}
}

func TestMergeMarkdown_SectionNotFound(t *testing.T) {
	t.Parallel()

	_, err := merge.MergeMarkdown(
		[]byte("# Title\n\n## Existing"),
		[]configtypes.MarkdownSection{
			{
				Action:  configtypes.MarkdownActionAfter,
				Heading: "Nonexistent",
				Content: "content",
			},
		},
	)
	if err == nil {
		t.Fatal("expected error for missing section")
	}

	if !errors.Is(err, merge.ErrMarkdownSectionNotFound) {
		t.Errorf("expected ErrMarkdownSectionNotFound, got %v", err)
	}
}

func TestMergeMarkdown_CodeBlockNotParsed(t *testing.T) {
	t.Parallel()

	base := "# Title\n\n```\n# Not a heading\n## Also not\n```\n\n## Real Section\n\nContent."
	sections := []configtypes.MarkdownSection{
		{Action: configtypes.MarkdownActionAfter, Heading: "Real Section", Content: "Added."},
	}

	got, err := merge.MergeMarkdown([]byte(base), sections)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	result := string(got)
	if !strings.Contains(result, "Added.") {
		t.Error("should contain inserted content")
	}

	if !strings.Contains(result, "# Not a heading") {
		t.Error("should preserve code block content")
	}
}

func TestMergeMarkdown_DuplicateHeadingsFirstMatch(t *testing.T) {
	t.Parallel()

	base := "# Title\n\n## Dup\n\nFirst.\n\n## Dup\n\nSecond.\n\n## Final\n\nClosing content."
	sections := []configtypes.MarkdownSection{
		{Action: configtypes.MarkdownActionAfter, Heading: "Dup", Content: "After first."},
	}

	got, err := merge.MergeMarkdown([]byte(base), sections)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	result := string(got)
	lines := strings.Split(result, "\n")
	afterIdx := -1
	secondDupIdx := -1

	for i, line := range lines {
		if line == "After first." && afterIdx == -1 {
			afterIdx = i
		}

		if line == "## Dup" && i > 2 && secondDupIdx == -1 {
			secondDupIdx = i
		}
	}

	if afterIdx < 0 {
		t.Fatal("should find inserted content")
	}

	if secondDupIdx <= afterIdx {
		t.Error("inserted content should be before second Dup")
	}
}

func TestMergeMarkdown_TrailingHashMarks(t *testing.T) {
	t.Parallel()

	base := "# Title\n\n## Heading ##\n\nContent."
	sections := []configtypes.MarkdownSection{
		{Action: configtypes.MarkdownActionAfter, Heading: "Heading", Content: "Added."},
	}

	got, err := merge.MergeMarkdown([]byte(base), sections)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	if !strings.Contains(string(got), "Added.") {
		t.Error("should match heading with trailing hash marks")
	}
}

func TestMergeMarkdown_CaseInsensitive(t *testing.T) {
	t.Parallel()

	base := "# Title\n\n## Getting Started\n\nContent."
	sections := []configtypes.MarkdownSection{
		{Action: configtypes.MarkdownActionAfter, Heading: "getting started", Content: "Added."},
	}

	got, err := merge.MergeMarkdown([]byte(base), sections)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	if !strings.Contains(string(got), "Added.") {
		t.Error("should match heading case-insensitively")
	}
}

func TestMergeMarkdown_MultipleOperations(t *testing.T) {
	t.Parallel()

	base := "# Title\n\n## A\n\nContent A.\n\n## B\n\nContent B.\n\n## C\n\nContent C."
	sections := []configtypes.MarkdownSection{
		{Action: configtypes.MarkdownActionDelete, Heading: "B"},
		{
			Action:  configtypes.MarkdownActionAfter,
			Heading: "A",
			Content: "## New B\n\nNew B content.",
		},
	}

	got, err := merge.MergeMarkdown([]byte(base), sections)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	result := string(got)
	if strings.Contains(result, "Content B.") {
		t.Error("should have deleted section B content")
	}

	if !strings.Contains(result, "New B content.") {
		t.Error("should contain new B content")
	}

	if !strings.Contains(result, "Content A.") {
		t.Error("should preserve section A")
	}

	if !strings.Contains(result, "Content C.") {
		t.Error("should preserve section C")
	}
}

func TestValidateMarkdownSections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sections []configtypes.MarkdownSection
		wantErr  bool
	}{
		{
			name: "missing heading for after",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionAfter, Content: "content"},
			},
			wantErr: true,
		},
		{
			name: "missing content for replace",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionReplace, Heading: "Section"},
			},
			wantErr: true,
		},
		{
			name: "missing content for append",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionAppend},
			},
			wantErr: true,
		},
		{
			name: "invalid action",
			sections: []configtypes.MarkdownSection{
				{Action: "invalid", Heading: "Section", Content: "content"},
			},
			wantErr: true,
		},
		{
			name: "delete only requires heading",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionDelete, Heading: "Section"},
			},
			wantErr: false,
		},
		{
			name: "missing heading for before",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionBefore, Content: "content"},
			},
			wantErr: true,
		},
		{
			name: "missing heading for delete",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionDelete},
			},
			wantErr: true,
		},
		{
			name: "valid after",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionAfter, Heading: "Section", Content: "content"},
			},
			wantErr: false,
		},
		{
			name: "valid prepend",
			sections: []configtypes.MarkdownSection{
				{Action: configtypes.MarkdownActionPrepend, Content: "content"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := merge.ValidateMarkdownSections(tt.sections)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMarkdownSections() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && !errors.Is(err, merge.ErrMarkdownInvalidAction) {
				t.Errorf("expected ErrMarkdownInvalidAction, got %v", err)
			}
		})
	}
}

func TestMergeMarkdown_RealWorld_ContributingMD(t *testing.T) {
	t.Parallel()

	base := `# Contributing to Organization Projects

Thank you for your interest in contributing!

## Prerequisites

Before you begin, ensure you have:
- Go 1.21+
- Docker (optional)

## Getting Started

1. Fork the repository
2. Clone your fork
3. Create a branch

## Code Style

Follow the project's linting rules.

## Pull Requests

Submit PRs against the main branch.
`

	sections := []configtypes.MarkdownSection{
		{
			Action:  configtypes.MarkdownActionAfter,
			Heading: "Prerequisites",
			Content: "### SAI Monorepo Structure\n\nFor the SAI project specifically:\n- Each plugin is in its own top-level directory\n- Test individual plugins: `claude --plugin-dir {plugin-name}/`",
		},
	}

	got, err := merge.MergeMarkdown([]byte(base), sections)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	result := string(got)
	if !strings.Contains(result, "### SAI Monorepo Structure") {
		t.Error("should contain SAI section heading")
	}

	if !strings.Contains(result, "Each plugin is in its own top-level directory") {
		t.Error("should contain SAI section content")
	}

	prerequisitesIdx := strings.Index(result, "## Prerequisites")
	saiIdx := strings.Index(result, "### SAI Monorepo Structure")
	gettingStartedIdx := strings.Index(result, "## Getting Started")

	if saiIdx <= prerequisitesIdx {
		t.Error("SAI section should be after Prerequisites")
	}

	if saiIdx >= gettingStartedIdx {
		t.Error("SAI section should be before Getting Started")
	}
}

func TestMergeMarkdown_TildeCodeFence(t *testing.T) {
	t.Parallel()

	base := "# Title\n\n~~~\n# Not a heading\n~~~\n\n## Real\n\nContent."
	sections := []configtypes.MarkdownSection{
		{Action: configtypes.MarkdownActionAfter, Heading: "Real", Content: "Added."},
	}

	got, err := merge.MergeMarkdown([]byte(base), sections)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	result := string(got)
	if !strings.Contains(result, "Added.") {
		t.Error("should contain inserted content")
	}

	if !strings.Contains(result, "# Not a heading") {
		t.Error("should preserve code block content")
	}
}

func TestMergeMarkdown_EmptySections(t *testing.T) {
	t.Parallel()

	base := "# Title\n\nContent."

	got, err := merge.MergeMarkdown([]byte(base), nil)
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	if string(got) != base {
		t.Errorf("MergeMarkdown() with nil sections should return base unchanged")
	}

	got, err = merge.MergeMarkdown([]byte(base), []configtypes.MarkdownSection{})
	if err != nil {
		t.Fatalf("MergeMarkdown() error = %v", err)
	}

	if string(got) != base {
		t.Errorf("MergeMarkdown() with empty sections should return base unchanged")
	}
}
