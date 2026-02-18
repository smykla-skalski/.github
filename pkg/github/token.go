package github

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cockroachdb/errors"
	"golang.org/x/term"

	"github.com/smykla-skalski/.github/pkg/logger"
)

// GetToken retrieves a GitHub token using the configured cascade:
// 1. If useGHAuth=true, try 'gh auth token' first
// 2. Check GITHUB_TOKEN env var
// 3. Check GH_TOKEN env var
// 4. If interactive terminal and gh available, prompt user
func GetToken(ctx context.Context, log *logger.Logger, useGHAuth bool) (string, error) {
	if useGHAuth {
		if token, err := tryGHAuth(ctx, log); err == nil && token != "" {
			log.Debug("using token from 'gh auth token' (explicit flag)")

			return token, nil
		}
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		log.Debug("using token from GITHUB_TOKEN env var")

		return token, nil
	}

	if token := os.Getenv("GH_TOKEN"); token != "" {
		log.Debug("using token from GH_TOKEN env var")

		return token, nil
	}

	if !isInteractive() || !isGHAvailable() {
		return "", errors.WithStack(ErrGitHubTokenNotFound)
	}

	log.Debug("no token found, checking if user wants to use gh auth")

	if !promptYesNo("No GitHub token found. Use 'gh auth token'?") {
		return "", errors.WithStack(ErrGitHubTokenNotFound)
	}

	token, err := tryGHAuth(ctx, log)
	if err != nil {
		return "", errors.Wrap(err, "getting token from gh auth")
	}

	if token == "" {
		return "", errors.WithStack(ErrGHAuthEmptyToken)
	}

	return token, nil
}

func tryGHAuth(ctx context.Context, log *logger.Logger) (string, error) {
	log.Debug("attempting to get token from 'gh auth token'")

	cmd := exec.CommandContext(ctx, "gh", "auth", "token")

	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(ErrGHAuthFailed, err.Error())
	}

	token := strings.TrimSpace(string(output))

	if token == "" {
		return "", errors.WithStack(ErrGHAuthEmptyToken)
	}

	return token, nil
}

func isGHAvailable() bool {
	_, err := exec.LookPath("gh")

	return err == nil
}

func isInteractive() bool {
	//nolint:gosec // G115: Fd() returns uintptr; safe narrowing on all supported platforms
	return term.IsTerminal(int(os.Stdin.Fd())) &&
		term.IsTerminal(int(os.Stdout.Fd())) //nolint:gosec // G115: same as above
}

func promptYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s [y/N] ", question)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))

	return response == "y" || response == "yes"
}
