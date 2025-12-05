// Package github provides GitHub API client and operations.
package github

import "github.com/cockroachdb/errors"

var (
	ErrGitHubTokenNotFound = errors.New(
		"GitHub token not found: use --use-gh-auth flag or set GITHUB_TOKEN",
	)
	ErrGHAuthFailed     = errors.New("failed to get token from 'gh auth token'")
	ErrGHNotInstalled   = errors.New("'gh' command not found in PATH")
	ErrGHAuthEmptyToken = errors.New("'gh auth token' returned empty output")
	ErrValidatingToken  = errors.New("failed to validate GitHub token")
	ErrSyncConfigParse  = errors.New("failed to parse sync config")
	ErrLabelSync        = errors.New("failed to sync labels")
	ErrFileSync         = errors.New("failed to sync files")
	ErrSmyklotSync      = errors.New("failed to sync smyklot version")
	ErrSettingsSync     = errors.New("failed to sync repository settings")
)
