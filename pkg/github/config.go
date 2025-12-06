package github

import (
	"context"
	"encoding/base64"

	"github.com/cockroachdb/errors"
)

const syncConfigPath = ".github/sync-config.yml"

// ErrFileNotFound is returned when a file doesn't exist in the repository.
var ErrFileNotFound = errors.New("file not found")

// FetchSyncConfig fetches .github/sync-config.yml from the target repository.
// Returns ErrFileNotFound if the file doesn't exist.
func FetchSyncConfig(
	ctx context.Context,
	client *Client,
	org string,
	repo string,
) ([]byte, error) {
	fileContent, _, _, err := client.Repositories.GetContents(
		ctx,
		org,
		repo,
		syncConfigPath,
		nil,
	)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrFileNotFound
		}

		return nil, errors.Wrapf(err, "fetching %s from %s/%s", syncConfigPath, org, repo)
	}

	if fileContent == nil {
		return nil, errors.Newf("%s is a directory, expected file", syncConfigPath)
	}

	// Decode base64 content
	content, err := base64.StdEncoding.DecodeString(*fileContent.Content)
	if err != nil {
		return nil, errors.Wrap(err, "decoding file content")
	}

	return content, nil
}
