package github

import (
	"context"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
	"github.com/google/go-github/v83/github"

	"github.com/smykla-skalski/.github/pkg/logger"
)

// Client wraps the GitHub API client with additional functionality.
type Client struct {
	*github.Client
	log *logger.Logger
}

// NewClient creates a new GitHub API client with rate limiting.
func NewClient(ctx context.Context, log *logger.Logger, token string) (*Client, error) {
	if token == "" {
		return nil, errors.WithStack(ErrGitHubTokenNotFound)
	}

	rateLimiter := github_ratelimit.NewClient(nil)

	client := github.NewClient(rateLimiter).WithAuthToken(token)

	if err := validateToken(ctx, client); err != nil {
		return nil, errors.Wrap(err, "validating token")
	}

	log.Debug("GitHub client initialized successfully")

	return &Client{
		Client: client,
		log:    log,
	}, nil
}

func validateToken(ctx context.Context, client *github.Client) error {
	_, resp, err := client.RateLimit.Get(ctx)
	if err != nil {
		return errors.Wrap(ErrValidatingToken, err.Error())
	}

	if resp.StatusCode != http.StatusOK {
		return errors.Wrapf(ErrValidatingToken, "unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
