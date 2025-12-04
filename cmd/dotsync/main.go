package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/smykla-labs/.github/pkg/config"
	"github.com/smykla-labs/.github/pkg/github"
	"github.com/smykla-labs/.github/pkg/logger"
)

var version = "dev"

// CLI defines the command-line interface structure.
type CLI struct {
	LogLevel  string     `help:"Log level (trace|debug|info|warn|error)" default:"info" enum:"trace,debug,info,warn,error"`
	UseGHAuth bool       `help:"Use 'gh auth token' for authentication"`
	DryRun    bool       `help:"Preview changes without applying them"`
	Org       string     `help:"GitHub organization" default:"smykla-labs"`
	Version   VersionCmd `cmd:"" help:"Show version information"`
	Labels    LabelsCmd  `cmd:"" help:"Label synchronization commands"`
	Files     FilesCmd   `cmd:"" help:"File synchronization commands"`
	Smyklot   SmyklotCmd `cmd:"" help:"Smyklot version synchronization commands"`
	Repos     ReposCmd   `cmd:"" help:"Repository listing commands"`
}

// VersionCmd shows version information.
type VersionCmd struct{}

// Run executes the version command.
func (*VersionCmd) Run(_ context.Context) error {
	fmt.Printf("orgsync version %s\n", version)

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
	Sync FilesSyncCmd `cmd:"" help:"Sync files to a repository"`
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

// splitLabels splits comma-separated labels into a slice.
func splitLabels(labels string) []string {
	parts := strings.Split(labels, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
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

// ReposCmd contains repository listing subcommands.
type ReposCmd struct {
	List ReposListCmd `cmd:"" help:"List organization repositories"`
}

// ReposListCmd lists organization repositories.
type ReposListCmd struct{}

// Run executes the repos list command.
func (*ReposListCmd) Run(ctx context.Context, cli *CLI) error {
	log := logger.FromContext(ctx)

	token, err := github.GetToken(ctx, log, cli.UseGHAuth)
	if err != nil {
		return err
	}

	client, err := github.NewClient(ctx, log, token)
	if err != nil {
		return err
	}

	log.Debug("listing repositories", "org", cli.Org)

	repos, _, err := client.Repositories.ListByOrg(ctx, cli.Org, nil)
	if err != nil {
		return err
	}

	type repoInfo struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		Private       bool   `json:"private"`
		Archived      bool   `json:"archived"`
		Disabled      bool   `json:"disabled"`
		DefaultBranch string `json:"default_branch"`
	}

	repoList := make([]repoInfo, 0, len(repos))
	for _, repo := range repos {
		repoList = append(repoList, repoInfo{
			Name:          repo.GetName(),
			FullName:      repo.GetFullName(),
			Private:       repo.GetPrivate(),
			Archived:      repo.GetArchived(),
			Disabled:      repo.GetDisabled(),
			DefaultBranch: repo.GetDefaultBranch(),
		})
	}

	output, err := json.MarshalIndent(repoList, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(output))

	return nil
}

func main() {
	var cli CLI

	appCtx := context.Background()

	kongCtx := kong.Parse(&cli,
		kong.Name("orgsync"),
		kong.Description("Organization sync tool for labels, files, and smyklot versions"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": version,
		},
		kong.BindTo(appCtx, (*context.Context)(nil)),
	)

	log := logger.New(cli.LogLevel)

	appCtx = logger.WithContext(appCtx, log)
	kongCtx.BindTo(appCtx, (*context.Context)(nil))

	err := kongCtx.Run(&cli)
	kongCtx.FatalIfErrorf(err)
}
