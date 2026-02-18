package merge

import (
	"testing"
)

func TestParseHeading(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		line      string
		wantLevel int
		wantTitle string
	}{
		{name: "h1", line: "# Title", wantLevel: 1, wantTitle: "Title"},
		{name: "h2", line: "## Section", wantLevel: 2, wantTitle: "Section"},
		{name: "h3", line: "### Subsection", wantLevel: 3, wantTitle: "Subsection"},
		{name: "h6", line: "###### Deep", wantLevel: 6, wantTitle: "Deep"},
		{name: "h7 invalid", line: "####### Too deep", wantLevel: 0, wantTitle: ""},
		{name: "trailing hash", line: "## Heading ##", wantLevel: 2, wantTitle: "Heading"},
		{
			name: "trailing hash with spaces", line: "## Heading  ##  ",
			wantLevel: 2, wantTitle: "Heading",
		},
		{name: "no space after hash", line: "##NoSpace", wantLevel: 0, wantTitle: ""},
		{name: "empty heading", line: "## ", wantLevel: 2, wantTitle: ""},
		{name: "just hashes", line: "##", wantLevel: 2, wantTitle: ""},
		{name: "not a heading", line: "regular text", wantLevel: 0, wantTitle: ""},
		{name: "empty string", line: "", wantLevel: 0, wantTitle: ""},
		{name: "leading whitespace", line: "  ## Heading", wantLevel: 2, wantTitle: "Heading"},
		{name: "tab after hash", line: "#\tTitle", wantLevel: 1, wantTitle: "Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			level, title := parseHeading(tt.line)
			if level != tt.wantLevel {
				t.Errorf("parseHeading(%q) level = %d, want %d", tt.line, level, tt.wantLevel)
			}

			if title != tt.wantTitle {
				t.Errorf("parseHeading(%q) title = %q, want %q", tt.line, title, tt.wantTitle)
			}
		})
	}
}

func TestParseMarkdownSections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  []mdSection
	}{
		{
			name: "basic headings",
			lines: []string{
				"# Title",
				"",
				"## Section A",
				"",
				"### Subsection",
				"",
				"## Section B",
			},
			want: []mdSection{
				{level: 1, title: "Title", lineIndex: 0},
				{level: 2, title: "Section A", lineIndex: 2},
				{level: 3, title: "Subsection", lineIndex: 4},
				{level: 2, title: "Section B", lineIndex: 6},
			},
		},
		{
			name:  "code fence skipped",
			lines: []string{"# Title", "```", "# Not heading", "```", "## Real"},
			want: []mdSection{
				{level: 1, title: "Title", lineIndex: 0},
				{level: 2, title: "Real", lineIndex: 4},
			},
		},
		{
			name:  "tilde code fence skipped",
			lines: []string{"# Title", "~~~", "## Not heading", "~~~", "## Real"},
			want: []mdSection{
				{level: 1, title: "Title", lineIndex: 0},
				{level: 2, title: "Real", lineIndex: 4},
			},
		},
		{
			name:  "no headings",
			lines: []string{"Just plain text", "More text"},
			want:  nil,
		},
		{
			name:  "empty document",
			lines: []string{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseMarkdownSections(tt.lines)
			if len(got) != len(tt.want) {
				t.Fatalf(
					"parseMarkdownSections() returned %d sections, want %d",
					len(got),
					len(tt.want),
				)
			}

			for i, want := range tt.want {
				if got[i].level != want.level {
					t.Errorf("section[%d].level = %d, want %d", i, got[i].level, want.level)
				}

				if got[i].title != want.title {
					t.Errorf("section[%d].title = %q, want %q", i, got[i].title, want.title)
				}

				if got[i].lineIndex != want.lineIndex {
					t.Errorf(
						"section[%d].lineIndex = %d, want %d",
						i,
						got[i].lineIndex,
						want.lineIndex,
					)
				}
			}
		})
	}
}

func TestFindSectionByHeading(t *testing.T) {
	t.Parallel()

	sections := []mdSection{
		{level: 1, title: "Title", lineIndex: 0},
		{level: 2, title: "Getting Started", lineIndex: 2},
		{level: 3, title: "Prerequisites", lineIndex: 5},
		{level: 2, title: "Contributing", lineIndex: 10},
	}

	tests := []struct {
		name    string
		heading string
		want    int
	}{
		{name: "exact match", heading: "Getting Started", want: 1},
		{name: "case insensitive", heading: "getting started", want: 1},
		{name: "uppercase", heading: "PREREQUISITES", want: 2},
		{name: "not found", heading: "Nonexistent", want: -1},
		{name: "with whitespace", heading: "  Contributing  ", want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := findSectionByHeading(sections, tt.heading)
			if got != tt.want {
				t.Errorf("findSectionByHeading(%q) = %d, want %d", tt.heading, got, tt.want)
			}
		})
	}
}

func TestSectionEnd(t *testing.T) {
	t.Parallel()

	sections := []mdSection{
		{level: 1, title: "Title", lineIndex: 0},
		{level: 2, title: "Section A", lineIndex: 3},
		{level: 3, title: "Sub A1", lineIndex: 6},
		{level: 3, title: "Sub A2", lineIndex: 9},
		{level: 2, title: "Section B", lineIndex: 12},
	}
	totalLines := 15

	tests := []struct {
		name string
		idx  int
		want int
	}{
		{name: "h1 ends at EOF", idx: 0, want: 15},
		{name: "h2 ends at next h2", idx: 1, want: 12},
		{name: "h3 ends at next h3", idx: 2, want: 9},
		{name: "last h3 ends at next h2", idx: 3, want: 12},
		{name: "last h2 ends at EOF", idx: 4, want: 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := sectionEnd(sections, tt.idx, totalLines)
			if got != tt.want {
				t.Errorf("sectionEnd(%d) = %d, want %d", tt.idx, got, tt.want)
			}
		})
	}
}

func TestIsCodeFence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "backtick fence", line: "```", want: true},
		{name: "backtick fence with lang", line: "```go", want: true},
		{name: "tilde fence", line: "~~~", want: true},
		{name: "tilde fence with lang", line: "~~~bash", want: true},
		{name: "indented backtick", line: "   ```", want: true},
		{name: "not a fence", line: "hello", want: false},
		{name: "only two backticks", line: "``", want: false},
		{name: "inline code", line: "`code`", want: false},
		{name: "empty line", line: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isCodeFence(tt.line)
			if got != tt.want {
				t.Errorf("isCodeFence(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}
