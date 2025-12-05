package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"github.com/smykla-labs/.github/pkg/config"
	"github.com/smykla-labs/.github/pkg/github"
	"github.com/smykla-labs/.github/pkg/logger"
	"github.com/smykla-labs/.github/pkg/schema"
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

// Subcommand definitions

var labelsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync labels to a repository",
	Long:  "Synchronize GitHub labels from a YAML file to a target repository",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		// Get flags
		org, _ := cmd.Root().PersistentFlags().GetString("org")
		dryRun, _ := cmd.Root().PersistentFlags().GetBool("dry-run")
		useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")
		repo, _ := cmd.Flags().GetString("repo")
		labelsFile, _ := cmd.Flags().GetString("labels-file")
		configJSON, _ := cmd.Flags().GetString("config")

		log.Info("starting label sync",
			"org", org,
			"repo", repo,
			"labels_file", labelsFile,
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

		// Parse sync config
		syncConfig, err := config.ParseSyncConfigJSON(configJSON)
		if err != nil {
			return err
		}

		// Sync labels
		if err := github.SyncLabels(
			ctx,
			log,
			client,
			org,
			repo,
			labelsFile,
			syncConfig,
			dryRun,
		); err != nil {
			return err
		}

		log.Info("label sync completed successfully")

		return nil
	},
}

var filesSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync files to a repository",
	Long:  "Synchronize files from templates to a target repository via pull request",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		// Get flags
		org, _ := cmd.Root().PersistentFlags().GetString("org")
		dryRun, _ := cmd.Root().PersistentFlags().GetBool("dry-run")
		useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")
		repo, _ := cmd.Flags().GetString("repo")
		filesConfig, _ := cmd.Flags().GetString("files-config")
		configJSON, _ := cmd.Flags().GetString("config")
		branchPrefix, _ := cmd.Flags().GetString("branch-prefix")
		prLabelsStr, _ := cmd.Flags().GetString("pr-labels")

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

		// Parse sync config
		syncConfig, err := config.ParseSyncConfigJSON(configJSON)
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

		// Get flags
		githubOutput, _ := cmd.Root().PersistentFlags().GetBool("github-output")
		templatesDir, _ := cmd.Flags().GetString("templates-dir")

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

		// Get flags
		org, _ := cmd.Root().PersistentFlags().GetString("org")
		dryRun, _ := cmd.Root().PersistentFlags().GetBool("dry-run")
		useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")
		repo, _ := cmd.Flags().GetString("repo")
		smyklotVersion, _ := cmd.Flags().GetString("version")
		tag, _ := cmd.Flags().GetString("tag")
		configJSON, _ := cmd.Flags().GetString("config")

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

		// Parse sync config
		syncConfig, err := config.ParseSyncConfigJSON(configJSON)
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

var settingsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync settings to a repository",
	Long:  "Synchronize repository settings from a YAML file to a target repository",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		// Get flags
		org, _ := cmd.Root().PersistentFlags().GetString("org")
		dryRun, _ := cmd.Root().PersistentFlags().GetBool("dry-run")
		useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")
		repo, _ := cmd.Flags().GetString("repo")
		settingsFile, _ := cmd.Flags().GetString("settings-file")
		configJSON, _ := cmd.Flags().GetString("config")

		log.Info("starting settings sync",
			"org", org,
			"repo", repo,
			"settings_file", settingsFile,
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

		// Parse sync config
		syncConfig, err := config.ParseSyncConfigJSON(configJSON)
		if err != nil {
			return err
		}

		// Sync settings
		if err := github.SyncSettings(
			ctx,
			log,
			client,
			org,
			repo,
			settingsFile,
			syncConfig,
			dryRun,
		); err != nil {
			return err
		}

		log.Info("settings sync completed successfully")

		return nil
	},
}

var reposListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organization repositories",
	Long:  "List all repositories in the organization with optional output format",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		// Get flags
		org, _ := cmd.Root().PersistentFlags().GetString("org")
		useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")
		githubOutput, _ := cmd.Root().PersistentFlags().GetBool("github-output")
		format, _ := cmd.Flags().GetString("format")

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

var configSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Generate JSON Schema for sync configuration",
	Long:  "Generate and output JSON Schema for the sync configuration file format",
	RunE: func(_ *cobra.Command, _ []string) error {
		output, err := schema.GenerateSchema("github.com/smykla-labs/.github", "./pkg/config")
		if err != nil {
			return err
		}

		fmt.Println(string(output))

		return nil
	},
}

var configVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify schema is in sync and commit if needed",
	Long:  "Check if configuration schema file is up to date and commit updates if needed",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		log := logger.FromContext(ctx)

		// Get flags
		dryRun, _ := cmd.Root().PersistentFlags().GetBool("dry-run")
		useGHAuth, _ := cmd.Root().PersistentFlags().GetBool("use-gh-auth")
		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		schemaFile, _ := cmd.Flags().GetString("schema-file")

		log.Info("verifying schema sync status",
			"repo", repo,
			"branch", branch,
			"schema_file", schemaFile,
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

		// Verify and commit schema if needed
		changed, err := github.VerifyAndCommitSchema(
			ctx,
			log,
			client,
			repo,
			branch,
			schemaFile,
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
	rootCmd.PersistentFlags().String("org", "smykla-labs", "GitHub organization")

	// Configure label sync command flags
	labelsSyncCmd.Flags().String("repo", "", "Target repository (e.g., 'myrepo')")
	labelsSyncCmd.Flags().String("labels-file", "", "Path to labels YAML file")
	labelsSyncCmd.Flags().String("config", "", "JSON sync config (optional)")
	_ = labelsSyncCmd.MarkFlagRequired("repo")
	_ = labelsSyncCmd.MarkFlagRequired("labels-file")

	// Configure file sync command flags
	filesSyncCmd.Flags().String("repo", "", "Target repository (e.g., 'myrepo')")
	filesSyncCmd.Flags().String("files-config", "", "JSON files config")
	filesSyncCmd.Flags().String("config", "", "JSON sync config (optional)")
	filesSyncCmd.Flags().String("branch-prefix", "chore/org-sync", "Branch name prefix")
	filesSyncCmd.Flags().String("pr-labels", "ci/skip-all", "Comma-separated PR labels")
	_ = filesSyncCmd.MarkFlagRequired("repo")
	_ = filesSyncCmd.MarkFlagRequired("files-config")

	// Configure files discover command flags
	filesDiscoverCmd.Flags().String("templates-dir", "templates", "Path to templates directory")

	// Configure smyklot sync command flags
	smyklotSyncCmd.Flags().String("repo", "", "Target repository (e.g., 'myrepo')")
	smyklotSyncCmd.Flags().String("version", "", "Smyklot version (e.g., '1.9.2')")
	smyklotSyncCmd.Flags().String("tag", "", "Smyklot tag (e.g., 'v1.9.2')")
	smyklotSyncCmd.Flags().String("config", "", "JSON sync config (optional)")
	_ = smyklotSyncCmd.MarkFlagRequired("repo")
	_ = smyklotSyncCmd.MarkFlagRequired("version")
	_ = smyklotSyncCmd.MarkFlagRequired("tag")

	// Configure settings sync command flags
	settingsSyncCmd.Flags().String("repo", "", "Target repository (e.g., 'myrepo')")
	settingsSyncCmd.Flags().String("settings-file", "", "Path to settings YAML file")
	settingsSyncCmd.Flags().String("config", "", "JSON sync config (optional)")
	_ = settingsSyncCmd.MarkFlagRequired("repo")
	_ = settingsSyncCmd.MarkFlagRequired("settings-file")

	// Configure repos list command flags
	reposListCmd.Flags().String("format", "json", "Output format (json|names)")

	// Configure config verify command flags
	configVerifyCmd.Flags().String("repo", "", "Repository (owner/name)")
	configVerifyCmd.Flags().String("branch", "", "Branch name")
	configVerifyCmd.Flags().String("schema-file", "schemas/sync-config.schema.json", "Path to schema file")
	_ = configVerifyCmd.MarkFlagRequired("repo")
	_ = configVerifyCmd.MarkFlagRequired("branch")

	// Build command tree
	labelsCmd.AddCommand(labelsSyncCmd)
	filesCmd.AddCommand(filesSyncCmd, filesDiscoverCmd)
	smyklotCmd.AddCommand(smyklotSyncCmd)
	settingsCmd.AddCommand(settingsSyncCmd)
	reposCmd.AddCommand(reposListCmd)
	configCmd.AddCommand(configSchemaCmd, configVerifyCmd)

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
