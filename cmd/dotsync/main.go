package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	"github.com/smykla-labs/.github/pkg/config"
	"github.com/smykla-labs/.github/pkg/github"
	"github.com/smykla-labs/.github/pkg/logger"
	"github.com/smykla-labs/.github/pkg/schema"
)

var version = "dev"

// CLI defines the command-line interface structure.
type CLI struct {
	LogLevel     string      `help:"Log level (trace|debug|info|warn|error)" default:"info" enum:"trace,debug,info,warn,error"`
	UseGHAuth    bool        `help:"Use 'gh auth token' for authentication"`
	DryRun       bool        `help:"Preview changes without applying them"`
	GitHubOutput bool        `name:"github-output" help:"Write outputs to GITHUB_OUTPUT for GitHub Actions"`
	Org          string      `help:"GitHub organization" default:"smykla-labs"`
	Version      VersionCmd  `cmd:"" help:"Show version information"`
	Labels       LabelsCmd   `cmd:"" help:"Label synchronization commands"`
	Files        FilesCmd    `cmd:"" help:"File synchronization commands"`
	Smyklot      SmyklotCmd  `cmd:"" help:"Smyklot version synchronization commands"`
	Settings     SettingsCmd `cmd:"" help:"Repository settings synchronization commands"`
	Repos        ReposCmd    `cmd:"" help:"Repository listing commands"`
	Config       ConfigCmd   `cmd:"" help:"Configuration schema commands"`
}

// VersionCmd shows version information.
type VersionCmd struct{}

// Run executes the version command.
func (*VersionCmd) Run(_ context.Context) error {
	fmt.Printf("dotsync version %s\n", version)

	return nil
}

// LabelsCmd contains label sync subcommands.
type LabelsCmd struct {
	Sync LabelsSyncCmd `cmd:"" help:"Sync labels to a repository"`
}

// LabelsSyncCmd syncs labels to a repository.
type LabelsSyncCmd struct {
	Repo       string `help:"Target repository (e.g., 'myrepo')" required:""`
	LabelsFile string `help:"Path to labels YAML file" required:""`
	Config     string `help:"JSON sync config (optional)"`
}

// Run executes the label sync command.
//
//nolint:dupl // Similar structure to other sync commands but with different operations
func (c *LabelsSyncCmd) Run(ctx context.Context, cli *CLI) error {
	log := logger.FromContext(ctx)

	log.Info("starting label sync",
		"org", cli.Org,
		"repo", c.Repo,
		"labels_file", c.LabelsFile,
		"dry_run", cli.DryRun,
	)

	// Get GitHub token
	token, err := github.GetToken(ctx, log, cli.UseGHAuth)
	if err != nil {
		return err
	}

	// Create GitHub client
	client, err := github.NewClient(ctx, log, token)
	if err != nil {
		return err
	}

	// Parse sync config
	syncConfig, err := config.ParseSyncConfigJSON(c.Config)
	if err != nil {
		return err
	}

	// Sync labels
	if err := github.SyncLabels(
		ctx,
		log,
		client,
		cli.Org,
		c.Repo,
		c.LabelsFile,
		syncConfig,
		cli.DryRun,
	); err != nil {
		return err
	}

	log.Info("label sync completed successfully")

	return nil
}

// FilesCmd contains file sync subcommands.
type FilesCmd struct {
	Sync     FilesSyncCmd     `cmd:"" help:"Sync files to a repository"`
	Discover FilesDiscoverCmd `cmd:"" help:"Discover files in templates directory"`
}

// FilesSyncCmd syncs files to a repository.
type FilesSyncCmd struct {
	Repo         string `help:"Target repository (e.g., 'myrepo')" required:""`
	FilesConfig  string `help:"JSON files config" required:""`
	Config       string `help:"JSON sync config (optional)"`
	BranchPrefix string `help:"Branch name prefix" default:"chore/org-sync"`
	PRLabels     string `help:"Comma-separated PR labels" default:"ci/skip-all"`
}

// Run executes the file sync command.
func (c *FilesSyncCmd) Run(ctx context.Context, cli *CLI) error {
	log := logger.FromContext(ctx)

	log.Info("starting file sync",
		"org", cli.Org,
		"repo", c.Repo,
		"branch_prefix", c.BranchPrefix,
		"dry_run", cli.DryRun,
	)

	// Get GitHub token
	token, err := github.GetToken(ctx, log, cli.UseGHAuth)
	if err != nil {
		return err
	}

	// Create GitHub client
	client, err := github.NewClient(ctx, log, token)
	if err != nil {
		return err
	}

	// Parse sync config
	syncConfig, err := config.ParseSyncConfigJSON(c.Config)
	if err != nil {
		return err
	}

	// Parse PR labels
	var prLabels []string
	if c.PRLabels != "" {
		prLabels = splitLabels(c.PRLabels)
	}

	// Sync files
	if err := github.SyncFiles(
		ctx,
		log,
		client,
		cli.Org,
		c.Repo,
		".github", // Source repo is always .github
		c.FilesConfig,
		syncConfig,
		c.BranchPrefix,
		prLabels,
		cli.DryRun,
	); err != nil {
		return err
	}

	log.Info("file sync completed successfully")

	return nil
}

// FilesDiscoverCmd discovers files in templates directory.
type FilesDiscoverCmd struct {
	TemplatesDir string `help:"Path to templates directory" default:"templates"`
}

// FileMapping represents a source to destination file mapping.
type FileMapping struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

// Run executes the files discover command.
func (c *FilesDiscoverCmd) Run(ctx context.Context, cli *CLI) error {
	log := logger.FromContext(ctx)

	log.Debug("discovering files", "templates_dir", c.TemplatesDir)

	mappings := make([]FileMapping, 0)

	if err := filepath.Walk(c.TemplatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(c.TemplatesDir, path)
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
	if err := github.WriteGitHubOutput(cli.GitHubOutput, "config", string(output)); err != nil {
		log.Warn("failed to write to GITHUB_OUTPUT", "error", err)
	}

	log.Debug("file discovery completed", "count", len(mappings))

	return nil
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

// SmyklotCmd contains smyklot sync subcommands.
type SmyklotCmd struct {
	Sync SmyklotSyncCmd `cmd:"" help:"Sync smyklot version to a repository"`
}

// SmyklotSyncCmd syncs smyklot version to a repository.
type SmyklotSyncCmd struct {
	Repo    string `help:"Target repository (e.g., 'myrepo')" required:""`
	Version string `help:"Smyklot version (e.g., '1.9.2')" required:""`
	Tag     string `help:"Smyklot tag (e.g., 'v1.9.2')" required:""`
	Config  string `help:"JSON sync config (optional)"`
}

// Run executes the smyklot sync command.
func (c *SmyklotSyncCmd) Run(ctx context.Context, cli *CLI) error {
	log := logger.FromContext(ctx)

	log.Info("starting smyklot sync",
		"org", cli.Org,
		"repo", c.Repo,
		"version", c.Version,
		"tag", c.Tag,
		"dry_run", cli.DryRun,
	)

	// Get GitHub token
	token, err := github.GetToken(ctx, log, cli.UseGHAuth)
	if err != nil {
		return err
	}

	// Create GitHub client
	client, err := github.NewClient(ctx, log, token)
	if err != nil {
		return err
	}

	// Parse sync config
	syncConfig, err := config.ParseSyncConfigJSON(c.Config)
	if err != nil {
		return err
	}

	// Sync smyklot version
	if err := github.SyncSmyklot(
		ctx,
		log,
		client,
		cli.Org,
		c.Repo,
		c.Version,
		c.Tag,
		syncConfig,
		cli.DryRun,
	); err != nil {
		return err
	}

	log.Info("smyklot sync completed successfully")

	return nil
}

// SettingsCmd contains settings sync subcommands.
type SettingsCmd struct {
	Sync SettingsSyncCmd `cmd:"" help:"Sync settings to a repository"`
}

// SettingsSyncCmd syncs repository settings to a repository.
type SettingsSyncCmd struct {
	Repo         string `help:"Target repository (e.g., 'myrepo')" required:""`
	SettingsFile string `help:"Path to settings YAML file" required:""`
	Config       string `help:"JSON sync config (optional)"`
}

// Run executes the settings sync command.
//
//nolint:dupl // Similar structure to other sync commands but with different operations
func (c *SettingsSyncCmd) Run(ctx context.Context, cli *CLI) error {
	log := logger.FromContext(ctx)

	log.Info("starting settings sync",
		"org", cli.Org,
		"repo", c.Repo,
		"settings_file", c.SettingsFile,
		"dry_run", cli.DryRun,
	)

	// Get GitHub token
	token, err := github.GetToken(ctx, log, cli.UseGHAuth)
	if err != nil {
		return err
	}

	// Create GitHub client
	client, err := github.NewClient(ctx, log, token)
	if err != nil {
		return err
	}

	// Parse sync config
	syncConfig, err := config.ParseSyncConfigJSON(c.Config)
	if err != nil {
		return err
	}

	// Sync settings
	if err := github.SyncSettings(
		ctx,
		log,
		client,
		cli.Org,
		c.Repo,
		c.SettingsFile,
		syncConfig,
		cli.DryRun,
	); err != nil {
		return err
	}

	log.Info("settings sync completed successfully")

	return nil
}

// ReposCmd contains repository listing subcommands.
type ReposCmd struct {
	List ReposListCmd `cmd:"" help:"List organization repositories"`
}

// ReposListCmd lists organization repositories.
type ReposListCmd struct {
	Format string `help:"Output format (json|names)" default:"json" enum:"json,names"`
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

// Run executes the repos list command.
func (c *ReposListCmd) Run(ctx context.Context, cli *CLI) error {
	log := logger.FromContext(ctx)

	token, err := github.GetToken(ctx, log, cli.UseGHAuth)
	if err != nil {
		return err
	}

	client, err := github.NewClient(ctx, log, token)
	if err != nil {
		return err
	}

	log.Debug("listing repositories", "org", cli.Org, "format", c.Format)

	repos, _, err := client.Repositories.ListByOrg(ctx, cli.Org, nil)
	if err != nil {
		return err
	}

	var output []byte

	switch c.Format {
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
	if err := github.WriteGitHubOutput(cli.GitHubOutput, "repos", string(output)); err != nil {
		log.Warn("failed to write to GITHUB_OUTPUT", "error", err)
	}

	return nil
}

// ConfigCmd contains config schema subcommands.
type ConfigCmd struct {
	Schema ConfigSchemaCmd `cmd:"" help:"Generate JSON Schema for sync configuration"`
	Verify ConfigVerifyCmd `cmd:"" help:"Verify schema is in sync and commit if needed"`
}

// ConfigSchemaCmd generates JSON Schema for sync configuration.
type ConfigSchemaCmd struct{}

// Run executes the config schema command.
func (*ConfigSchemaCmd) Run(_ context.Context) error {
	output, err := schema.GenerateSchema("github.com/smykla-labs/.github", "./pkg/config")
	if err != nil {
		return err
	}

	fmt.Println(string(output))

	return nil
}

// ConfigVerifyCmd verifies schema is in sync and commits if needed.
type ConfigVerifyCmd struct {
	Repo       string `help:"Repository (owner/name)" required:""`
	Branch     string `help:"Branch name" required:""`
	SchemaFile string `help:"Path to schema file" default:"schemas/sync-config.schema.json"`
}

// Run executes the config verify command.
func (c *ConfigVerifyCmd) Run(ctx context.Context, cli *CLI) error {
	log := logger.FromContext(ctx)

	log.Info("verifying schema sync status",
		"repo", c.Repo,
		"branch", c.Branch,
		"schema_file", c.SchemaFile,
	)

	// Get GitHub token
	token, err := github.GetToken(ctx, log, cli.UseGHAuth)
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
		c.Repo,
		c.Branch,
		c.SchemaFile,
		cli.DryRun,
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

func init() {
	// Add persistent flags (global flags available to all commands)
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (trace|debug|info|warn|error)")
	rootCmd.PersistentFlags().Bool("use-gh-auth", false, "Use 'gh auth token' for authentication")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Preview changes without applying them")
	rootCmd.PersistentFlags().Bool("github-output", false, "Write outputs to GITHUB_OUTPUT for GitHub Actions")
	rootCmd.PersistentFlags().String("org", "smykla-labs", "GitHub organization")
}

func main() {
	var cli CLI

	appCtx := context.Background()

	kongCtx := kong.Parse(&cli,
		kong.Name("dotsync"),
		kong.Description("Organization sync tool for labels, files, and smyklot versions"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.Vars{"version": version},
		kong.BindTo(appCtx, (*context.Context)(nil)),
	)

	kongCtx.BindTo(logger.WithContext(appCtx, logger.New(cli.LogLevel)), (*context.Context)(nil))

	kongCtx.FatalIfErrorf(kongCtx.Run(&cli))
}
