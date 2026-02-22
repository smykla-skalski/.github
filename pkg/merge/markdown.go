package merge

import (
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/smykla-skalski/.github/internal/configtypes"
)

const (
	// maxHeadingLevel is the maximum ATX heading level (######).
	maxHeadingLevel = 6
	// minCodeFenceLen is the minimum length for a code fence opener/closer.
	minCodeFenceLen = 3
	// maxConsecutiveBlanks limits consecutive blank lines in output.
	maxConsecutiveBlanks = 2
	// insertCapPadding is extra capacity for blank line separators when inserting.
	insertCapPadding = 2
)

// mdSection represents a parsed heading in a markdown document.
type mdSection struct {
	level     int    // heading level (1-6)
	title     string // normalized heading text
	lineIndex int    // zero-based line index of the heading
}

// MergeMarkdown applies a sequence of section operations to a markdown document.
// Each section is applied sequentially, with the document re-parsed between operations
// to ensure correct line indices after modifications.
func MergeMarkdown(base []byte, sections []configtypes.MarkdownSection) ([]byte, error) {
	if err := ValidateMarkdownSections(sections); err != nil {
		return nil, err
	}

	content := string(base)

	for i, section := range sections {
		lines := splitLines(content)

		result, err := applySection(lines, section)
		if err != nil {
			return nil, errors.Wrapf(err, "applying section operation %d (%s)", i, section.Action)
		}

		content = joinLines(trimTrailingBlanks(result))
	}

	return []byte(content), nil
}

// ValidateMarkdownSections validates that all section operations have required fields.
func ValidateMarkdownSections(sections []configtypes.MarkdownSection) error {
	for i, s := range sections {
		if err := validateSingleSection(s, i); err != nil {
			return err
		}
	}

	return nil
}

// validateSingleSection validates a single markdown section operation.
func validateSingleSection(s configtypes.MarkdownSection, idx int) error {
	switch s.Action {
	case configtypes.MarkdownActionAfter,
		configtypes.MarkdownActionBefore,
		configtypes.MarkdownActionReplace:
		if s.Heading == "" {
			return errors.Wrapf(ErrMarkdownInvalidAction,
				"section %d: action %q requires heading", idx, s.Action)
		}

		if s.Content == "" {
			return errors.Wrapf(ErrMarkdownInvalidAction,
				"section %d: action %q requires content", idx, s.Action)
		}

	case configtypes.MarkdownActionDelete:
		if s.Heading == "" {
			return errors.Wrapf(ErrMarkdownInvalidAction,
				"section %d: action %q requires heading", idx, s.Action)
		}

	case configtypes.MarkdownActionAppend, configtypes.MarkdownActionPrepend:
		if s.Content == "" {
			return errors.Wrapf(ErrMarkdownInvalidAction,
				"section %d: action %q requires content", idx, s.Action)
		}

	default:
		return errors.Wrapf(ErrMarkdownInvalidAction,
			"section %d: unknown action %q", idx, s.Action)
	}

	return nil
}

// applySection applies a single section operation to the document lines.
func applySection(lines []string, section configtypes.MarkdownSection) ([]string, error) {
	switch section.Action {
	case configtypes.MarkdownActionAfter:
		return applyAfter(lines, section)
	case configtypes.MarkdownActionBefore:
		return applyBefore(lines, section)
	case configtypes.MarkdownActionReplace:
		return applyReplace(lines, section)
	case configtypes.MarkdownActionDelete:
		return applyDelete(lines, section)
	case configtypes.MarkdownActionAppend:
		return applyAppend(lines, section), nil
	case configtypes.MarkdownActionPrepend:
		return applyPrepend(lines, section), nil
	default:
		return nil, errors.Wrapf(ErrMarkdownInvalidAction, "unknown action %q", section.Action)
	}
}

// applyAfter inserts content after the matched section (including its subsections).
func applyAfter(lines []string, section configtypes.MarkdownSection) ([]string, error) {
	sections := parseMarkdownSections(lines)

	idx := findSectionByHeading(sections, section.Heading)
	if idx < 0 {
		return nil, errors.Wrapf(ErrMarkdownSectionNotFound, "heading %q", section.Heading)
	}

	endLine := sectionEnd(sections, idx, len(lines))
	contentLines := splitLines(strings.TrimRight(section.Content, "\n"))

	return insertWithSpacing(lines, endLine, contentLines), nil
}

// applyBefore inserts content before the matched section's heading line.
func applyBefore(lines []string, section configtypes.MarkdownSection) ([]string, error) {
	sections := parseMarkdownSections(lines)

	idx := findSectionByHeading(sections, section.Heading)
	if idx < 0 {
		return nil, errors.Wrapf(ErrMarkdownSectionNotFound, "heading %q", section.Heading)
	}

	insertAt := sections[idx].lineIndex
	contentLines := splitLines(strings.TrimRight(section.Content, "\n"))

	return insertWithSpacing(lines, insertAt, contentLines), nil
}

// applyReplace replaces the entire matched section with new content.
func applyReplace(lines []string, section configtypes.MarkdownSection) ([]string, error) {
	sections := parseMarkdownSections(lines)

	idx := findSectionByHeading(sections, section.Heading)
	if idx < 0 {
		return nil, errors.Wrapf(ErrMarkdownSectionNotFound, "heading %q", section.Heading)
	}

	startLine := sections[idx].lineIndex
	endLine := sectionEnd(sections, idx, len(lines))
	contentLines := splitLines(strings.TrimRight(section.Content, "\n"))

	result := make([]string, 0, len(lines)-endLine+startLine+len(contentLines))
	result = append(result, lines[:startLine]...)
	result = ensureTrailingBlank(result)
	result = append(result, contentLines...)
	result = append(result, "") // blank line after
	result = append(result, lines[endLine:]...)

	return trimExcessBlanks(result), nil
}

// applyDelete removes the entire matched section.
func applyDelete(lines []string, section configtypes.MarkdownSection) ([]string, error) {
	sections := parseMarkdownSections(lines)

	idx := findSectionByHeading(sections, section.Heading)
	if idx < 0 {
		return nil, errors.Wrapf(ErrMarkdownSectionNotFound, "heading %q", section.Heading)
	}

	startLine := sections[idx].lineIndex
	endLine := sectionEnd(sections, idx, len(lines))

	result := make([]string, 0, len(lines)-(endLine-startLine))
	result = append(result, lines[:startLine]...)
	result = append(result, lines[endLine:]...)

	return trimExcessBlanks(result), nil
}

// applyAppend appends content at the end of the document.
func applyAppend(lines []string, section configtypes.MarkdownSection) []string {
	contentLines := splitLines(strings.TrimRight(section.Content, "\n"))
	result := make([]string, 0, len(lines)+len(contentLines)+1)
	result = append(result, lines...)
	result = ensureTrailingBlank(result)
	result = append(result, contentLines...)

	return result
}

// applyPrepend inserts content at the start of the document.
func applyPrepend(lines []string, section configtypes.MarkdownSection) []string {
	contentLines := splitLines(strings.TrimRight(section.Content, "\n"))
	result := make([]string, 0, len(contentLines)+len(lines)+1)
	result = append(result, contentLines...)
	result = append(result, "") // blank line after
	result = append(result, lines...)

	return trimExcessBlanks(result)
}

// parseMarkdownSections parses all ATX headings from the document, respecting code fences.
func parseMarkdownSections(lines []string) []mdSection {
	var sections []mdSection

	inCodeFence := false

	for i, line := range lines {
		if isCodeFence(line) {
			inCodeFence = !inCodeFence

			continue
		}

		if inCodeFence {
			continue
		}

		level, title := parseHeading(line)
		if level > 0 {
			sections = append(sections, mdSection{
				level:     level,
				title:     title,
				lineIndex: i,
			})
		}
	}

	return sections
}

// parseHeading parses an ATX-style heading line and returns the level and title.
// Returns (0, "") if the line is not a heading.
func parseHeading(line string) (int, string) {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) == 0 || trimmed[0] != '#' {
		return 0, ""
	}

	// Count leading # marks
	level := 0

	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}

	// Must be 1-6 and followed by space or end of string
	if level > maxHeadingLevel {
		return 0, ""
	}

	rest := trimmed[level:]
	if len(rest) > 0 && rest[0] != ' ' && rest[0] != '\t' {
		return 0, ""
	}

	// Strip trailing # marks and whitespace
	title := strings.TrimSpace(rest)
	title = strings.TrimRight(title, "#")
	title = strings.TrimSpace(title)

	return level, title
}

// findSectionByHeading finds the first section matching the given heading text.
// Matching is case-insensitive with trimmed whitespace.
func findSectionByHeading(sections []mdSection, heading string) int {
	target := strings.TrimSpace(strings.ToLower(heading))

	for i, s := range sections {
		if strings.ToLower(s.title) == target {
			return i
		}
	}

	return -1
}

// sectionEnd computes the end boundary (exclusive line index) of a section.
// A section at level N ends just before the next heading at level <= N, or at totalLines.
func sectionEnd(sections []mdSection, idx int, totalLines int) int {
	level := sections[idx].level

	for i := idx + 1; i < len(sections); i++ {
		if sections[i].level <= level {
			return sections[i].lineIndex
		}
	}

	return totalLines
}

// isCodeFence returns true if the line is a code fence opener/closer (``` or ~~~).
func isCodeFence(line string) bool {
	trimmed := strings.TrimSpace(line)

	if len(trimmed) < minCodeFenceLen {
		return false
	}

	if strings.HasPrefix(trimmed, "```") {
		return true
	}

	return strings.HasPrefix(trimmed, "~~~")
}

// splitLines splits content into lines. An empty string returns a single empty line.
func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}

	return strings.Split(content, "\n")
}

// joinLines joins lines back into a single string with a trailing newline.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n") + "\n"
}

// insertWithSpacing inserts contentLines at the given position with blank line separation.
func insertWithSpacing(lines []string, insertAt int, contentLines []string) []string {
	result := make([]string, 0, len(lines)+len(contentLines)+insertCapPadding)
	result = append(result, lines[:insertAt]...)
	result = ensureTrailingBlank(result)
	result = append(result, contentLines...)
	result = append(result, "") // blank line after

	if insertAt < len(lines) {
		result = append(result, lines[insertAt:]...)
	}

	return trimExcessBlanks(result)
}

// ensureTrailingBlank ensures the slice ends with a blank line for spacing.
func ensureTrailingBlank(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	if strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}

	return lines
}

// trimTrailingBlanks removes blank lines from the end of the slice.
func trimTrailingBlanks(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// trimExcessBlanks removes runs of more than 2 consecutive blank lines.
func trimExcessBlanks(lines []string) []string {
	result := make([]string, 0, len(lines))
	consecutiveBlanks := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			consecutiveBlanks++

			if consecutiveBlanks > maxConsecutiveBlanks {
				continue
			}
		} else {
			consecutiveBlanks = 0
		}

		result = append(result, line)
	}

	// Trim trailing blank lines to at most one
	for len(result) > 1 && strings.TrimSpace(result[len(result)-1]) == "" &&
		strings.TrimSpace(result[len(result)-2]) == "" {
		result = result[:len(result)-1]
	}

	return result
}
