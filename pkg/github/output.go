package github

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
)

// WriteGitHubOutput writes an output variable to GITHUB_OUTPUT file if enabled
// and running in GitHub Actions environment. This allows Docker container actions
// to set outputs that can be consumed by subsequent workflow steps.
//
// If enabled is false, this function does nothing and returns nil.
// If GITHUB_OUTPUT environment variable is not set (not running in GitHub
// Actions), this function does nothing and returns nil.
//
// For single-line values: key=value
// For multi-line values: heredoc syntax with random delimiter
func WriteGitHubOutput(enabled bool, key, value string) error {
	if !enabled {
		return nil
	}

	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		return nil
	}

	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY, 0600) //nolint:gosec,mnd,gofumpt,golines // GITHUB_OUTPUT from GH Actions
	if err != nil {
		return errors.Wrap(err, "opening GITHUB_OUTPUT file")
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.CombineErrors(err, errors.Wrap(closeErr, "closing GITHUB_OUTPUT file"))
		}
	}()

	if strings.Contains(value, "\n") {
		delimiter := randomDelimiter()
		_, err = fmt.Fprintf(f, "%s<<%s\n%s\n%s\n", key, delimiter, value, delimiter)
	} else {
		_, err = fmt.Fprintf(f, "%s=%s\n", key, value)
	}

	if err != nil {
		return errors.Wrap(err, "writing to GITHUB_OUTPUT")
	}

	return err
}

const delimiterBytes = 16

func randomDelimiter() string {
	b := make([]byte, delimiterBytes)

	_, _ = rand.Read(b)

	return "EOF_" + hex.EncodeToString(b)
}
