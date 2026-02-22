package merge

import "github.com/cockroachdb/errors"

var (
	// ErrMergeParseError indicates a failure to parse file for merge
	ErrMergeParseError = errors.New("failed to parse file for merge")
	// ErrMergeUnsupportedFileType indicates merge only supports JSON, YAML, and Markdown files
	ErrMergeUnsupportedFileType = errors.New("merge only supports JSON, YAML, and Markdown files")
	// ErrMergeUnknownStrategy indicates an unknown merge strategy was specified
	ErrMergeUnknownStrategy = errors.New("unknown merge strategy")
	// ErrMarkdownSectionNotFound indicates the specified heading was not found in the document
	ErrMarkdownSectionNotFound = errors.New("markdown section heading not found")
	// ErrMarkdownInvalidAction indicates an invalid action or missing required fields
	ErrMarkdownInvalidAction = errors.New("invalid markdown section action")
	// ErrMarkdownPatchNotFound indicates the find text was not present in the section
	ErrMarkdownPatchNotFound = errors.New("markdown patch text not found in section")
)
