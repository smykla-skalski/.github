package github

import (
	"context"
	"os"
	"testing"

	"github.com/smykla-skalski/.github/pkg/logger"
)

func TestGetToken(t *testing.T) {
	ctx := context.Background()
	log := logger.New("error")

	tests := []struct {
		name       string
		useGHAuth  bool
		setupEnv   func()
		cleanupEnv func()
		wantErr    bool
	}{
		{
			name:      "returns token from GITHUB_TOKEN env var",
			useGHAuth: false,
			setupEnv: func() {
				os.Setenv("GITHUB_TOKEN", "test-token-123")
			},
			cleanupEnv: func() {
				os.Unsetenv("GITHUB_TOKEN")
			},
			wantErr: false,
		},
		{
			name:      "returns token from GH_TOKEN env var",
			useGHAuth: false,
			setupEnv: func() {
				os.Setenv("GH_TOKEN", "test-token-456")
			},
			cleanupEnv: func() {
				os.Unsetenv("GH_TOKEN")
			},
			wantErr: false,
		},
		{
			name:      "prefers GITHUB_TOKEN over GH_TOKEN",
			useGHAuth: false,
			setupEnv: func() {
				os.Setenv("GITHUB_TOKEN", "github-token")
				os.Setenv("GH_TOKEN", "gh-token")
			},
			cleanupEnv: func() {
				os.Unsetenv("GITHUB_TOKEN")
				os.Unsetenv("GH_TOKEN")
			},
			wantErr: false,
		},
		{
			name:      "returns error when no token available",
			useGHAuth: false,
			setupEnv: func() {
				os.Unsetenv("GITHUB_TOKEN")
				os.Unsetenv("GH_TOKEN")
			},
			cleanupEnv: func() {},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()

			defer tt.cleanupEnv()

			token, err := GetToken(ctx, log, tt.useGHAuth)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetToken() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && token == "" {
				t.Error("GetToken() returned empty token when error was not expected")
			}
		})
	}
}

func TestIsGHAvailable(t *testing.T) {
	available := isGHAvailable()

	t.Logf("gh command available: %v", available)
}

func TestIsInteractive(t *testing.T) {
	interactive := isInteractive()

	t.Logf("running in interactive mode: %v", interactive)
}
