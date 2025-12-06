package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"github.com/smykla-labs/.github/internal/configtypes"
	"github.com/smykla-labs/.github/pkg/config"
	"github.com/smykla-labs/.github/pkg/github"
	"github.com/smykla-labs/.github/pkg/logger"
)

var version = "dev"

// FileMapping represents a source to destination file mapping.
type FileMapping struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

// RepoInfo represents repository information for JSON output.
type RepoInfo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	Archived      bool   `json:"archived"`
	Disabled      bool   `json:"disabled"`
	DefaultBranch string `json:"default_branch"`
}

// splitLabels splits comma-separated labels into a slice.
func splitLabels(labels string) []string {
	parts := strings.Split(labels, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// getStringFlagWithEnvFallback retrieves a string flag value with environment variable fallback.
// Priority: 1) explicit flag value, 2) INPUT_* env var, 3) GitHub standard env var.
func getStringFlagWithEnvFallback(cmd *cobra.Command, flagName, githubEnvFallback string) string {
	// Check explicit flag value
	val, _ := cmd.Flags().GetString(flagName)
	if val != "" {
		return val
	}

	// Check INPUT_* env var (from GitHub Actions input)
	inputEnv := "INPUT_" + strings.ToUpper(strings.ReplaceAll(flagName, "-", "_"))

	val = os.Getenv(inputEnv)
	if val != "" {
		return val
	}

	// Fall back to GitHub standard env var
	if githubEnvFallback != "" {
		return os.Getenv(githubEnvFallback)
	}

	return ""
}

// getPersistentStringFlagWithEnvFallback retrieves a persistent flag value with environment variable fallback.
// Priority: 1) explicit flag value, 2) INPUT_* env var, 3) GitHub standard env var.
//
//nolint:unparam // flagName parameter kept generic for potential future use with other persistent flags
func getPersistentStringFlagWithEnvFallback(cmd *cobra.Command, flagName, githubEnvFallback string) string {
	// Check explicit flag value
	val, _ := cmd.Root().PersistentFlags().GetString(flagName)
	if val != "" {
		return val
	}

	// Check INPUT_* env var (from GitHub Actions input)
	inputEnv := "INPUT_" + strings.ToUpper(strings.ReplaceAll(flagName, "-", "_"))

	val = os.Getenv(inputEnv)
	if val != "" {
		return val
	}

	// Fall back to GitHub standard env var
	if githubEnvFallback != "" {
		return os.Getenv(githubEnvFallback)
	}

	return ""
}

// getRepoFromEnv extracts repository name from GITHUB_REPOSITORY env var (format: owner/repo).
func getRepoFromEnv() string {
	fullRepo := os.Getenv("GITHUB_REPOSITORY")
	parts := strings.Split(fullRepo, "/")

	const expectedParts = 2
	if len(parts) == expectedParts {
		return parts[1]
	}

	return ""
}

// getPersistentBoolFlagWithEnvFallback retrieves a persistent bool flag value with environment variable fallback.
// Priority: 1) explicit flag value (if changed), 2) INPUT_* env var.
//
//nolint:unparam // flagName parameter kept generic for potential future use with other persistent flags
func getPersistentBoolFlagWithEnvFallback(cmd *cobra.Command, flagName string) bool {
	// Check if flag was explicitly set
	if cmd.Root().PersistentFlags().Changed(flagName) {
		val, _ := cmd.Root().PersistentFlags().GetBool(flagName)
		return val
	}

	// Check INPUT_* env var
	inputEnv := "INPUT_" + strings.ToUpper(strings.ReplaceAll(flagName, "-", "_"))
	envVal := os.Getenv(inputEnv)

	return envVal == "true"
}

// Cobra root command and initialization

var rootCmd = &cobra.Command{
	Use:   "dotsync",
	Short: "Organization sync tool for labels, files, settings, and smyklot versions",
	Long: `dotsync is a CLI tool for synchronizing organization-wide configurations
across GitHub repositories. It supports syncing labels, files, repository
settings, and smyklot version references.`,
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// Get log level from flag
		logLevel, err := cmd.Flags().GetString("log-level")
		if err != nil {
			return errors.Wrap(err, "failed to get log-level flag")
		}

		// Initialize logger
		log := logger.New(logLevel)

		// Inject logger into context
		ctx := logger.WithContext(cmd.Context(), log)
		cmd.SetContext(ctx)

		return nil
	},
}

// Cobra command definitions

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  "Display the current version of dotsync",
	RunE: func(_ *cobra.Command, _ []string) error {
		fmt.Printf("dotsync version %s\n", version)
		return nil
	},
}

var labelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "Label synchronization commands",
	Long:  "Commands for synchronizing GitHub labels across repositories",
}

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "File synchronization commands",
	Long:  "Commands for synchronizing files across repositories",
}

var smyklotCmd = &cobra.Command{
	Use:   "smyklot",
	Short: "Smyklot version synchronization commands",
	Long:  "Commands for synchronizing smyklot version references across repositories",
}

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Repository settings synchronization commands",
	Long:  "Commands for synchronizing repository settings across repositories",
}

var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Repository listing commands",
	Long:  "Commands for listing and querying organization repositories",
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration schema commands",
	Long:  "Commands for managing sync configuration schemas",
}

// Helper types and functions for sync commands

type syncParams struct {
	org        string
	repo       string
	configFile string
	configJSON string
	dryRun     bool
}

type syncFunc func(
	ctx context.Context,
	log *slog.Logger,
	client *github.Client,
	org string,
	repo string,
	configFile string,
	syncConfig *configtypes.SyncConfig,
	dryRun bool,
) error

func getSyncParams(cmd *cobra.Command, configFlag string) (syncParams, error) {
	// Use env fallback for org (GITHUB_REPOSITORY_OWNER) and repo (GITHUB_REPOSITORY)
	org := getPersistentStringFlagWithEnvFallback(cmd, "org", "GITHUB_REPOSITORY_OWNER")
	dryRun := getPersistentBoolFlagWithEnvFallback(cmd, "dry-run")

	// Repo has special handling: flag → INPUT_REPO → extract from GITHUB_REPOSITORY
	repo := getStringFlagWithEnvFallback(cmd, "repo", "")
	if repo == "" {
		repo = getRepoFromEnv()
	}

	configFile := getStringFlagWithEnvFallback(cmd, configFlag, "")
	configJSON := getStringFlagWithEnvFallback(cmd, "config", "")

	// Validate required fields
	if org == "" {
		return syncParams{}, errors.New("org is required (set via --org flag, INPUT_ORG, or GITHUB_REPOSITORY_OWNER)")
	}

	if repo == "" {
		return syncParams{}, errors.New("repo is required (set via --repo flag, INPUT_REPO, or GITHUB_REPOSITORY)")
	}

	if configFile == "" {
		return syncParams{}, errors.Newf("%s is required (set via --%s flag or INPUT_%s)",
			configFlag, configFlag, strings.ToUpper(strings.ReplaceAll(configFlag, "-", "_")))
	}

	return syncParams{
		org:        org,
		repo:       repo,
		configFile: configFile,
		configJSON: configJSON,
		dryRun:     dryRun,
	}, nil
}

func setupGitHubClient(
	ctx context.Context,
	log *slog.Logger,
	cmd *cobra.Command,
) (*github.Client, error) {
	useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")

	token, err := github.GetToken(ctx, log, useGHAuth)
	if err != nil {
		return nil, err
	}

	return github.NewClient(ctx, log, token)
}

func fetchSyncConfig(
	ctx context.Context,
	log *slog.Logger,
	client *github.Client,
	org string,
	repo string,
	configJSON string,
) (*configtypes.SyncConfig, error) {
	// If config JSON provided, use it
	if configJSON != "" {
		return config.ParseSyncConfigJSON(configJSON)
	}

	// Otherwise, fetch from target repo
	log.Debug("fetching sync config from target repo", "org", org, "repo", repo)

	syncConfigBytes, err := github.FetchSyncConfig(ctx, client, org, repo)
	if err != nil {
		// If file doesn't exist, return empty config (all syncs enabled by default)
		if errors.Is(err, github.ErrFileNotFound) {
			log.Debug("no sync-config.yml found in target repo, using defaults")

			return &configtypes.SyncConfig{}, nil
		}

		return nil, errors.Wrap(err, "fetching sync config from target repo")
	}

	// Parse config (handles both YAML and JSON)
	return config.ParseSyncConfig(syncConfigBytes)
}

func createSyncCommand(
	use string,
	short string,
	long string,
	configFlag string,
	configFileLogKey string,
	syncType string,
	syncFn syncFunc,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := logger.FromContext(ctx)

			params, err := getSyncParams(cmd, configFlag)
			if err != nil {
				return err
			}

			log.Info("starting "+syncType+" sync",
				"org", params.org,
				"repo", params.repo,
				configFileLogKey, params.configFile,
				"dry_run", params.dryRun,
			)

			client, err := setupGitHubClient(ctx, log, cmd)
			if err != nil {
				return err
			}

			syncConfig, err := fetchSyncConfig(ctx, log, client, params.org, params.repo, params.configJSON)
			if err != nil {
				return err
			}

			if err := syncFn(
				ctx,
				log,
				client,
				params.org,
				params.repo,
				params.configFile,
				syncConfig,
				params.dryRun,
			); err != nil {
				return err
			}

			log.Info(syncType + " sync completed successfully")

			return nil
		},
	}
}

// Subcommand definitions

var labelsSyncCmd = createSyncCommand(
	"sync",
	"Sync labels to a repository",
	"Synchronize GitHub labels from a YAML file to a target repository",
	"labels-file",
	"labels_file",
	"label",
	github.SyncLabels,
)

var filesSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync files to a repository",
	Long:  "Synchronize files from templates to a target repository via pull request",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		// Get flags with env fallback
		org := getPersistentStringFlagWithEnvFallback(cmd, "org", "GITHUB_REPOSITORY_OWNER")
		dryRun := getPersistentBoolFlagWithEnvFallback(cmd, "dry-run")
		useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")

		// Repo has special handling: flag → INPUT_REPO → extract from GITHUB_REPOSITORY
		repo := getStringFlagWithEnvFallback(cmd, "repo", "")
		if repo == "" {
			repo = getRepoFromEnv()
		}

		filesConfig := getStringFlagWithEnvFallback(cmd, "files-config", "")
		configJSON := getStringFlagWithEnvFallback(cmd, "config", "")

		// These only check INPUT_* env vars (no GitHub standard fallback)
		branchPrefix := getStringFlagWithEnvFallback(cmd, "branch-prefix", "")
		prLabelsStr := getStringFlagWithEnvFallback(cmd, "pr-labels", "")

		// Validate required fields
		if org == "" {
			return errors.New("org is required (set via --org flag, INPUT_ORG, or GITHUB_REPOSITORY_OWNER)")
		}

		if repo == "" {
			return errors.New("repo is required (set via --repo flag, INPUT_REPO, or GITHUB_REPOSITORY)")
		}

		if filesConfig == "" {
			return errors.New("files-config is required (set via --files-config flag or INPUT_FILES_CONFIG)")
		}

		log.Info("starting file sync",
			"org", org,
			"repo", repo,
			"branch_prefix", branchPrefix,
			"dry_run", dryRun,
		)

		// Get GitHub token
		token, err := github.GetToken(ctx, log, useGHAuth)
		if err != nil {
			return err
		}

		// Create GitHub client
		client, err := github.NewClient(ctx, log, token)
		if err != nil {
			return err
		}

		// Fetch sync config (auto-fetch from target repo if not provided)
		syncConfig, err := fetchSyncConfig(ctx, log, client, org, repo, configJSON)
		if err != nil {
			return err
		}

		// Parse PR labels
		var prLabels []string
		if prLabelsStr != "" {
			prLabels = splitLabels(prLabelsStr)
		}

		// Sync files
		if err := github.SyncFiles(
			ctx,
			log,
			client,
			org,
			repo,
			".github", // Source repo is always .github
			filesConfig,
			syncConfig,
			branchPrefix,
			prLabels,
			dryRun,
		); err != nil {
			return err
		}

		log.Info("file sync completed successfully")

		return nil
	},
}

var filesDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover files in templates directory",
	Long:  "Scan templates directory and output JSON mapping of source to destination paths",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		// Get flags with env fallback
		githubOutput, _ := cmd.Root().PersistentFlags().GetBool("github-output")
		templatesDir := getStringFlagWithEnvFallback(cmd, "templates-dir", "")

		log.Debug("discovering files", "templates_dir", templatesDir)

		mappings := make([]FileMapping, 0)

		if err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(templatesDir, path)
			if err != nil {
				return err
			}

			mappings = append(mappings, FileMapping{
				Source: path,
				Dest:   relPath,
			})

			log.Debug("discovered file", "source", path, "dest", relPath)

			return nil
		}); err != nil {
			return err
		}

		output, err := json.Marshal(mappings)
		if err != nil {
			return err
		}

		fmt.Println(string(output))

		// Write to GITHUB_OUTPUT if enabled
		if err := github.WriteGitHubOutput(githubOutput, "config", string(output)); err != nil {
			log.Warn("failed to write to GITHUB_OUTPUT", "error", err)
		}

		log.Debug("file discovery completed", "count", len(mappings))

		return nil
	},
}

var smyklotSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync smyklot version to a repository",
	Long:  "Update smyklot version references in workflow files via pull request",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		// Get flags with env fallback
		org := getPersistentStringFlagWithEnvFallback(cmd, "org", "GITHUB_REPOSITORY_OWNER")
		dryRun := getPersistentBoolFlagWithEnvFallback(cmd, "dry-run")
		useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")

		// Repo has special handling: flag → INPUT_REPO → extract from GITHUB_REPOSITORY
		repo := getStringFlagWithEnvFallback(cmd, "repo", "")
		if repo == "" {
			repo = getRepoFromEnv()
		}

		smyklotVersion := getStringFlagWithEnvFallback(cmd, "version", "")
		tag := getStringFlagWithEnvFallback(cmd, "tag", "")
		configJSON := getStringFlagWithEnvFallback(cmd, "config", "")

		// Validate required fields
		if org == "" {
			return errors.New("org is required (set via --org flag, INPUT_ORG, or GITHUB_REPOSITORY_OWNER)")
		}

		if repo == "" {
			return errors.New("repo is required (set via --repo flag, INPUT_REPO, or GITHUB_REPOSITORY)")
		}

		if smyklotVersion == "" {
			return errors.New("version is required (set via --version flag or INPUT_VERSION)")
		}

		if tag == "" {
			return errors.New("tag is required (set via --tag flag or INPUT_TAG)")
		}

		log.Info("starting smyklot sync",
			"org", org,
			"repo", repo,
			"version", smyklotVersion,
			"tag", tag,
			"dry_run", dryRun,
		)

		// Get GitHub token
		token, err := github.GetToken(ctx, log, useGHAuth)
		if err != nil {
			return err
		}

		// Create GitHub client
		client, err := github.NewClient(ctx, log, token)
		if err != nil {
			return err
		}

		// Fetch sync config (auto-fetch from target repo if not provided)
		syncConfig, err := fetchSyncConfig(ctx, log, client, org, repo, configJSON)
		if err != nil {
			return err
		}

		// Sync smyklot version
		if err := github.SyncSmyklot(
			ctx,
			log,
			client,
			org,
			repo,
			smyklotVersion,
			tag,
			syncConfig,
			dryRun,
		); err != nil {
			return err
		}

		log.Info("smyklot sync completed successfully")

		return nil
	},
}

var settingsSyncCmd = createSyncCommand(
	"sync",
	"Sync settings to a repository",
	"Synchronize repository settings from a YAML file to a target repository",
	"settings-file",
	"settings_file",
	"settings",
	github.SyncSettings,
)

var reposListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organization repositories",
	Long:  "List all repositories in the organization with optional output format",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		// Get flags with env fallback
		org := getPersistentStringFlagWithEnvFallback(cmd, "org", "GITHUB_REPOSITORY_OWNER")
		useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")
		githubOutput, _ := cmd.Root().PersistentFlags().GetBool("github-output")
		format := getStringFlagWithEnvFallback(cmd, "format", "")

		if org == "" {
			return errors.New("org is required (set via --org flag, INPUT_ORG, or GITHUB_REPOSITORY_OWNER)")
		}

		token, err := github.GetToken(ctx, log, useGHAuth)
		if err != nil {
			return err
		}

		client, err := github.NewClient(ctx, log, token)
		if err != nil {
			return err
		}

		log.Debug("listing repositories", "org", org, "format", format)

		repos, _, err := client.Repositories.ListByOrg(ctx, org, nil)
		if err != nil {
			return err
		}

		var output []byte

		switch format {
		case "names":
			// Build list of repository names, excluding .github
			repoNames := make([]string, 0, len(repos))
			for _, repo := range repos {
				name := repo.GetName()
				if name == ".github" {
					log.Debug("excluding .github repository from list")
					continue
				}

				repoNames = append(repoNames, name)
			}

			// Output as compact JSON array
			output, err = json.Marshal(repoNames)
			if err != nil {
				return err
			}

		default: // "json"
			repoList := make([]RepoInfo, 0, len(repos))
			for _, repo := range repos {
				repoList = append(repoList, RepoInfo{
					Name:          repo.GetName(),
					FullName:      repo.GetFullName(),
					Private:       repo.GetPrivate(),
					Archived:      repo.GetArchived(),
					Disabled:      repo.GetDisabled(),
					DefaultBranch: repo.GetDefaultBranch(),
				})
			}

			output, err = json.MarshalIndent(repoList, "", "  ")
			if err != nil {
				return err
			}
		}

		fmt.Println(string(output))

		// Write to GITHUB_OUTPUT if enabled
		if err := github.WriteGitHubOutput(githubOutput, "repos", string(output)); err != nil {
			log.Warn("failed to write to GITHUB_OUTPUT", "error", err)
		}

		return nil
	},
}

var configVerifyFileCmd = &cobra.Command{
	Use:   "verify-file",
	Short: "Verify externally generated schema and commit if needed",
	Long:  "Compare pre-generated schema file with committed schema, commit if different. Use this when schema is generated externally (e.g., from PR branch code).",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		// Get flags with env fallback
		dryRun := getPersistentBoolFlagWithEnvFallback(cmd, "dry-run")
		useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")

		// Repo and branch have GitHub standard env fallbacks
		repo := getStringFlagWithEnvFallback(cmd, "repo", "")
		if repo == "" {
			repo = getRepoFromEnv()
		}

		branch := getStringFlagWithEnvFallback(cmd, "branch", "GITHUB_REF_NAME")
		schemaFile := getStringFlagWithEnvFallback(cmd, "schema-file", "")
		generatedSchemaFile := getStringFlagWithEnvFallback(cmd, "generated-schema", "")

		// Validate required fields
		if repo == "" {
			return errors.New("repo is required (set via --repo flag, INPUT_REPO, or GITHUB_REPOSITORY)")
		}

		if branch == "" {
			return errors.New("branch is required (set via --branch flag, INPUT_BRANCH, or GITHUB_REF_NAME)")
		}

		if generatedSchemaFile == "" {
			return errors.New("generated-schema is required (path to externally generated schema file)")
		}

		log.Info("verifying schema from external file",
			"repo", repo,
			"branch", branch,
			"schema_file", schemaFile,
			"generated_schema", generatedSchemaFile,
		)

		// Read the externally generated schema
		//nolint:gosec // generatedSchemaFile is controlled input from CLI flags
		generatedSchema, err := os.ReadFile(generatedSchemaFile)
		if err != nil {
			return errors.Wrap(err, "reading generated schema file")
		}

		// Get GitHub token
		token, err := github.GetToken(ctx, log, useGHAuth)
		if err != nil {
			return err
		}

		// Create GitHub client
		client, err := github.NewClient(ctx, log, token)
		if err != nil {
			return err
		}

		// Verify and commit schema if needed
		changed, err := github.VerifyAndCommitSchemaFromContent(
			ctx,
			log,
			client,
			repo,
			branch,
			schemaFile,
			generatedSchema,
			dryRun,
		)
		if err != nil {
			return err
		}

		if changed {
			log.Info("schema was out of sync and has been committed")

			return errors.New("schema was regenerated - check will pass on re-run")
		}

		log.Info("schema is up to date")

		return nil
	},
}

func init() {
	// Add persistent flags (global flags available to all commands)
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (trace|debug|info|warn|error)")
	rootCmd.PersistentFlags().Bool("use-gh-auth", false, "Use 'gh auth token' for authentication")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Preview changes without applying them")
	rootCmd.PersistentFlags().Bool("github-output", false, "Write outputs to GITHUB_OUTPUT for GitHub Actions")
	rootCmd.PersistentFlags().String("org", "", "GitHub organization")

	// Configure label sync command flags
	labelsSyncCmd.Flags().String("repo", "", "Target repository (e.g., 'myrepo')")
	labelsSyncCmd.Flags().String("labels-file", "", "Path to labels YAML file")
	labelsSyncCmd.Flags().String("config", "", "JSON sync config (optional)")

	// Configure file sync command flags
	filesSyncCmd.Flags().String("repo", "", "Target repository (e.g., 'myrepo')")
	filesSyncCmd.Flags().String("files-config", "", "JSON files config")
	filesSyncCmd.Flags().String("config", "", "JSON sync config (optional)")
	filesSyncCmd.Flags().String("branch-prefix", "chore/org-sync", "Branch name prefix")
	filesSyncCmd.Flags().String("pr-labels", "ci/skip-all", "Comma-separated PR labels")

	// Configure files discover command flags
	filesDiscoverCmd.Flags().String("templates-dir", "templates", "Path to templates directory")

	// Configure smyklot sync command flags
	smyklotSyncCmd.Flags().String("repo", "", "Target repository (e.g., 'myrepo')")
	smyklotSyncCmd.Flags().String("version", "", "Smyklot version (e.g., '1.9.2')")
	smyklotSyncCmd.Flags().String("tag", "", "Smyklot tag (e.g., 'v1.9.2')")
	smyklotSyncCmd.Flags().String("config", "", "JSON sync config (optional)")

	// Configure settings sync command flags
	settingsSyncCmd.Flags().String("repo", "", "Target repository (e.g., 'myrepo')")
	settingsSyncCmd.Flags().String("settings-file", "", "Path to settings YAML file")
	settingsSyncCmd.Flags().String("config", "", "JSON sync config (optional)")

	// Configure repos list command flags
	reposListCmd.Flags().String("format", "json", "Output format (json|names)")

	// Configure config verify-file command flags
	configVerifyFileCmd.Flags().String("repo", "", "Repository (owner/name)")
	configVerifyFileCmd.Flags().String("branch", "", "Branch name")
	configVerifyFileCmd.Flags().String("schema-file", "schemas/sync-config.schema.json", "Path to committed schema file")
	configVerifyFileCmd.Flags().String("generated-schema", "", "Path to externally generated schema file")

	// Build command tree
	labelsCmd.AddCommand(labelsSyncCmd)
	filesCmd.AddCommand(filesSyncCmd, filesDiscoverCmd)
	smyklotCmd.AddCommand(smyklotSyncCmd)
	settingsCmd.AddCommand(settingsSyncCmd)
	reposCmd.AddCommand(reposListCmd)
	configCmd.AddCommand(configVerifyFileCmd)

	// Add commands to root
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(labelsCmd)
	rootCmd.AddCommand(filesCmd)
	rootCmd.AddCommand(smyklotCmd)
	rootCmd.AddCommand(settingsCmd)
	rootCmd.AddCommand(reposCmd)
	rootCmd.AddCommand(configCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
