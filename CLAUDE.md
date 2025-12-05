# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

Organization-wide defaults and synchronization for smykla-labs repositories. This is a special `.github` repository that:

1. **Community Health Files** - Provides default templates for all smykla-labs repos (CODE_OF_CONDUCT, CONTRIBUTING, SECURITY, issue/PR templates)
2. **Label Sync** - Automated synchronization of GitHub labels across all repositories using custom composite action
3. **File Sync** - Automated synchronization of specified files across repositories using custom composite action
4. **Settings Sync** - Automated synchronization of repository settings (branch protection, security, merge strategies, features) across all repositories
5. **Smyklot Sync** - Automated synchronization of smyklot version references in workflow files
6. **Reusable Workflows** - Shared CI/CD workflows for Go projects (lint, test, build, release)

## Repository Structure

```text
.
├── .github/
│   ├── labels.yml              # Label definitions synced to all repos
│   ├── settings.yml            # Repository settings synced to all repos
│   ├── workflows/
│   │   ├── sync-labels.yml     # Label sync workflow
│   │   ├── sync-files.yml      # File sync workflow
│   │   ├── sync-settings.yml   # Settings sync workflow
│   │   ├── sync-smyklot.yml    # Smyklot version sync workflow
│   │   ├── release-dotsync.yml # Release workflow for dotsync CLI
│   │   ├── lib-lint.yml        # Reusable lint workflow
│   │   ├── lib-test.yml        # Reusable test workflow
│   │   ├── lib-build.yml       # Reusable build workflow
│   │   └── lib-release.yml     # Reusable release workflow
│   ├── actions/                # Reusable composite actions
│   │   ├── generate-token/     # GitHub App token generation
│   │   ├── get-org-repos/      # Fetch org repositories
│   │   ├── dotsync-labels/     # Container-based label sync action
│   │   ├── dotsync-files/      # Container-based file sync action
│   │   ├── dotsync-settings/   # Container-based settings sync action
│   │   └── dotsync-smyklot/    # Container-based smyklot sync action
│   ├── ISSUE_TEMPLATE/         # Default issue templates
│   └── PULL_REQUEST_TEMPLATE.md
├── cmd/
│   └── dotsync/
│       └── main.go             # Go CLI entry point (Kong framework)
├── pkg/
│   ├── github/
│   │   ├── client.go           # GitHub client with rate limiting
│   │   ├── token.go            # Auth cascade implementation
│   │   ├── errors.go           # Sentinel errors (cockroachdb/errors)
│   │   ├── labels.go           # Label sync operations
│   │   ├── files.go            # File sync operations
│   │   ├── settings.go         # Settings sync operations
│   │   └── smyklot.go          # Smyklot sync operations
│   ├── config/
│   │   └── sync.go             # Sync config types and parsing
│   └── logger/
│       └── logger.go           # Logging setup (slog)
├── examples/
│   └── sync-config.yml         # Unified sync config example
├── docs/
│   └── MIGRATION.md            # Reusable workflows migration guide
├── templates/                  # Source files for file sync (auto-discovered)
│   ├── CODE_OF_CONDUCT.md
│   ├── CONTRIBUTING.md
│   ├── SECURITY.md
│   ├── LICENSE
│   ├── renovate.json
│   └── .github/
│       ├── ISSUE_TEMPLATE/
│       └── PULL_REQUEST_TEMPLATE.md
├── CODE_OF_CONDUCT.md          # Org-wide defaults (native GitHub)
├── CONTRIBUTING.md
├── SECURITY.md
├── go.mod                      # Go module definition
├── Taskfile.yaml               # Build/lint/test automation
├── Dockerfile                  # Container image build
├── .goreleaser.yml             # Release automation with container builds
├── .golangci.yml               # Go linter configuration
└── .mise.toml                  # Tool version management
```

## Key Concepts

### Community Health Files (Native GitHub)

Files in the root automatically apply to all smykla-labs repositories that don't have their own versions. This is a native GitHub feature for `.github` repositories.

### Synchronization System

The synchronization system uses a Go CLI (`dotsync`) packaged as a container, wrapped in composite actions for easy workflow integration:

**Architecture:**

- **CLI**: `dotsync` - Go-based CLI tool using Kong framework
- **Container**: Published to `ghcr.io/smykla-labs/dotsync:latest`
- **Actions**: Container-based composite actions wrap the CLI commands
- **Implementation**: Type-safe Go code with proper error handling (cockroachdb/errors)
- **Distribution**: Multi-arch container builds via GoReleaser

**Label Sync:**

- **Action**: Container-based composite action (`.github/actions/dotsync-labels`)
- **Workflow**: `.github/workflows/sync-labels.yml`
- **CLI Command**: `dotsync labels sync`
- **Config**: `.github/labels.yml` (central) + per-repo `.github/sync-config.yml`
- **Method**: Direct API updates via go-github SDK
- **Features**: Label removal, exclusions, skip flags

**File Sync:**

- **Action**: Container-based composite action (`.github/actions/dotsync-files`)
- **Workflow**: `.github/workflows/sync-files.yml`
- **CLI Command**: `dotsync files sync`
- **Source**: All files in `templates/` directory (auto-discovered)
- **Config**: Per-repo `.github/sync-config.yml` (optional)
- **Method**: Creates PRs with file changes via go-github SDK
- **Features**: File exclusions, skip flags, PR management, smart renovate.json handling

**Settings Sync:**

- **Action**: Container-based composite action (`.github/actions/dotsync-settings`)
- **Workflow**: `.github/workflows/sync-settings.yml`
- **CLI Command**: `dotsync settings sync`
- **Config**: `.github/settings.yml` (central) + per-repo `.github/sync-config.yml`
- **Method**: Direct API updates via go-github SDK
- **Features**: Branch protection, security settings, merge strategies, repository features, no-downgrade protection, skip flags, exclusions

**Smyklot Sync:**

- **Action**: Container-based composite action (`.github/actions/dotsync-smyklot`)
- **Workflow**: `.github/workflows/sync-smyklot.yml`
- **CLI Command**: `dotsync smyklot sync`
- **Trigger**: Repository dispatch from smyklot releases or manual dispatch
- **Config**: Per-repo `.github/sync-config.yml` (optional)
- **Method**: Creates PRs with version updates via go-github SDK
- **Features**: Auto-merge, skip flags, updates `uses:` and `ghcr.io/` references

**Unified Config:**

- Repos can create `.github/sync-config.yml` to customize behavior
- Control label, file, settings, and smyklot sync from single config file
- Supports skip flags, exclusions, and removal options
- See `examples/sync-config.yml` for full schema

**Flow:**

- Each workflow triggers independently on relevant events
- All workflows run for all repos in parallel
- Label and settings sync update directly; file and smyklot sync create PRs
- File sync uses `chore/org-sync` branch; smyklot sync uses `chore/sync-smyklot` branch

### Label Synchronization

- **Source**: `.github/labels.yml` contains all label definitions
- **Format**: YAML with `name`, `color` (with #), and optional `description`
- **Target**: All repositories in smykla-labs organization (except `.github` itself)
- **Method**: Go-based CLI using go-github SDK with rate limiting
- **Features**: Efficient map-based diff, parallel processing, dry-run support, type-safe operations

### File Synchronization

- **Source**: `templates/` directory (all files auto-discovered)
- **Config**: Per-repo `.github/sync-config.yml` (optional)
- **Target**: All repositories in smykla-labs organization (except `.github` itself)
- **Method**: Go-based CLI using go-github SDK with Git tree/blob API
- **Features**: PR creation, single commit, exclusions, skip flags, smart renovate.json handling, type-safe operations

**Smart renovate.json Handling:**

- Detects manual modifications to `renovate.json` by checking commit history
- If manual commits found (not from sync workflow), excludes file from sync
- Shows alert in PR with instructions to add to `.github/sync-config.yml`
- This is the ONLY file with this special behavior

### Settings Synchronization

- **Source**: `.github/settings.yml` contains organization-wide repository settings
- **Config**: Per-repo `.github/sync-config.yml` (optional)
- **Target**: All repositories in smykla-labs organization (except `.github` itself)
- **Method**: Go-based CLI using go-github SDK with direct API updates
- **Features**: No-downgrade protection, GHAS awareness, hybrid status checks, diff-based updates, dry-run support, type-safe operations

**Categories:** Repository settings (merge strategies, auto-merge, branch deletion), features (Issues, Wiki, Projects, Discussions), security (secret scanning, push protection, Dependabot—requires GHAS), branch protection (reviews, status checks, linear history, force push protection)

**Key Behaviors:**

- **No-downgrade**: Never reduces security/quality below repo's current settings (e.g., repo with 2 reviews + config says 1 = keeps 2). Central config sets baseline minimums.
- **GHAS-aware**: If Advanced Security unavailable, logs warning and skips security settings (graceful degradation)
- **Hybrid status checks**: Empty `contexts: []` inherits repo's existing checks; explicit contexts override
- **Exclusions**: Dot notation in sync-config.yml (e.g., `"branch_protection"`, `"security.secret_scanning"`, `"features.has_wiki"`)
- **Never syncs**: Repository visibility (public/private), GitHub Actions permissions (deferred to future)

### Smyklot Synchronization

- **Trigger**: Automatically triggered by smyklot releases via `repository_dispatch`
- **Purpose**: Updates smyklot version references in workflow files across all repos
- **Target**: All repositories in smykla-labs organization (except `.github` and `smyklot` itself)
- **Method**: Go-based CLI using go-github SDK with regex-based replacement
- **Features**: PR creation with auto-merge, skip flags, intelligent change detection, type-safe operations

**Version References Updated:**

1. GitHub Action references: `uses: smykla-labs/smyklot@v1.2.3`
2. Docker image references: `ghcr.io/smykla-labs/smyklot:1.2.3`

**Workflow Triggers:**

- **Primary**: `repository_dispatch` event from smyklot release workflow
  - Event type: `smyklot-release`
  - Payload: `{ "version": "X.Y.Z", "tag": "vX.Y.Z" }`
- **Secondary**: Manual `workflow_dispatch` with version inputs

**Branch & PR Management:**

- Branch: `chore/sync-smyklot` (distinct from file sync)
- Commit: Single commit per repo with all workflow file updates
- PR title: `chore(deps): update smyklot to vX.Y.Z`
- Label: `ci/skip-all` to avoid triggering CI
- Auto-merge: Enabled with squash strategy after PR creation

**Skip Configuration:**

Repos can opt out via `.github/sync-config.yml`:

```yaml
sync:
  smyklot:
    skip: true  # Skip smyklot version sync for this repo
```

**Smart PR Management:**

- Closes existing PR if smyklot is already up to date
- Updates existing PR if new version released before merge
- Closes existing PR if sync is disabled via config

### Reusable Workflows

Shared CI/CD workflows for Go projects that can be called from any repository:

**Available Workflows:**

- **lib-lint.yml** - Multi-linter workflow (golangci-lint, yamllint, shellcheck, markdownlint)
- **lib-test.yml** - Go test runner with coverage reporting
- **lib-build.yml** - Cross-platform Go binary builder
- **lib-release.yml** - Semantic versioning and GitHub release creation

**Usage Example:**

```yaml
jobs:
  lint:
    uses: smykla-labs/.github/.github/workflows/lib-lint.yml@<commit-sha> # v1.0.0
    with:
      go-version: "1.25.x"
      enable-golangci-lint: true
```

**Version Pinning:**

- Always pin to commit SHA with semver comment
- Get SHA: `git rev-parse v1.0.0`
- Security best practice for reusable workflows

See `.github/workflows/lib-*.yml` files for full documentation of inputs/outputs.

### Authentication

All workflows use the **smyklot** GitHub App for authentication:

- `vars.SMYKLOT_APP_ID` - GitHub App ID (org-level variable)
- `secrets.SMYKLOT_PRIVATE_KEY` - GitHub App private key (org-level secret)

## Common Tasks

### Adding New Labels

1. Edit `.github/labels.yml`
2. Follow format:

   ```yaml
   - name: "label-name"          # Max 50 chars
     color: "#hex-color"         # 6-char hex with #
     description: "description"  # Max 100 chars (optional)
   ```

3. Commit and push to `main` - syncs automatically to all repos

### Adding New Sync Files

1. Add file to `templates/` directory (preserving the desired path structure)
2. Commit and push to `main` - file is auto-discovered and syncs automatically to all repos

**Note:** No configuration needed! All files in `templates/` are automatically synced to target repos.

### Updating Settings

1. Edit `.github/settings.yml`
2. Follow the structure:
   - `settings.repository` - Merge strategies, branch deletion, auto-merge
   - `settings.features` - Issues, Wiki, Projects, Discussions
   - `settings.security` - Secret scanning, push protection, Dependabot
   - `settings.branch_protection` - Array of protection rules with patterns
3. Commit and push to `main` - syncs automatically to all repos

**Note:** Use dry-run mode first to preview changes before applying.

### Manual Sync

Trigger workflows manually via GitHub Actions:

1. Go to Actions tab in this repository
2. Select "Sync Labels", "Sync Files", "Sync Settings", or "Sync smyklot Version"
3. Click "Run workflow"
4. For Labels/Files/Settings: Optionally enable "Dry run" to preview changes
5. For smyklot: Provide version (e.g., `1.9.2`) and tag (e.g., `v1.9.2`)

### Workflow Behavior

#### Label Sync Workflow

- **Trigger**: Push to `main` when `labels.yml` changes, or manual dispatch
- **Flow**:
  1. Get list of all org repositories
  2. For each repo: Generate token and sync labels
- **Matrix**: Processes all repos in parallel (fail-fast: false)
- **Action**: Custom composite action handles sync logic efficiently
- **Dry run**: Available via workflow dispatch

#### File Sync Workflow

- **Trigger**: Push to `main` when `templates/**` changes, or manual dispatch
- **Flow**:
  1. Auto-discover all files in `templates/` directory
  2. Get list of all org repositories
  3. For each repo: Fetch sync config, generate token, sync files
- **Matrix**: Processes all repos in parallel (fail-fast: false)
- **Action**: Custom composite action handles PR creation and file sync
- **Branch**: Uses `chore/org-sync` prefix
- **Commits**: Single commit with all changes using `chore(sync):` prefix
- **PR**: Labeled with `ci/skip-all`, title: "chore(sync): sync organization files"
- **Dry run**: Available via workflow dispatch
- **Special**: Detects manual `renovate.json` modifications and excludes from sync

#### Settings Sync Workflow

- **Trigger**: Push to `main` when `settings.yml` changes, or manual dispatch
- **Flow**:
  1. Get list of all org repositories
  2. For each repo: Fetch sync config, generate token, sync settings
- **Matrix**: Processes all repos in parallel (fail-fast: false)
- **Action**: Custom composite action handles settings sync via API
- **Method**: Direct API updates (no PRs)—settings are operational config
- **Categories**: Repository settings, features, security (GHAS-aware), branch protection
- **Features**: No-downgrade protection, hybrid status checks, diff-based updates
- **Dry run**: Available via workflow dispatch

#### Smyklot Sync Workflow

- **Trigger**: Repository dispatch from smyklot releases, or manual dispatch
- **Flow**:
  1. Get list of all org repositories
  2. For each repo: Fetch sync config, generate token, scan workflow files
  3. Update smyklot version references (both `uses:` and `ghcr.io/`)
  4. Create PR if changes detected
- **Matrix**: Processes all repos in parallel (fail-fast: false)
- **Action**: Custom composite action handles version replacement and PR creation
- **Branch**: Uses `chore/sync-smyklot` prefix
- **Commits**: Single commit with all workflow file updates
- **PR**: Labeled with `ci/skip-all`, title: "chore(deps): update smyklot to vX.Y.Z"
- **Auto-merge**: Enabled with squash strategy
- **Dry run**: Available via workflow dispatch

## Files to Edit

### Labels

- **Definition**: `.github/labels.yml`
- **Workflow**: `.github/workflows/sync-labels.yml`

### Files

- **Source**: `templates/` directory (auto-discovered)
- **Workflow**: `.github/workflows/sync-files.yml`

### Settings

- **Definition**: `.github/settings.yml`
- **Workflow**: `.github/workflows/sync-settings.yml`

### Smyklot Versions

- **Trigger**: Automatic via smyklot release workflow
- **Workflow**: `.github/workflows/sync-smyklot.yml`
- **Manual**: Run workflow dispatch with version/tag inputs

### Community Health Files

- **Default templates**: Root directory (CODE_OF_CONDUCT.md, CONTRIBUTING.md, SECURITY.md)
- **Issue templates**: `.github/ISSUE_TEMPLATE/`
- **PR template**: `.github/PULL_REQUEST_TEMPLATE.md`

## Label Categories

Labels are organized by prefix:

- `kind/*` - Issue/PR type (bug, enhancement, documentation, question, security)
- `area/*` - Affected component (ci, docs, api, testing, deps)
- `ci/*` - CI behavior control (skip-tests, skip-lint, skip-build, force-full)
- `release/*` - Release triggers (major, minor, patch, or auto-detect from commits)
- Automation: `org-sync` (sync PRs), `smyklot:*` (pending-ci with merge strategies)
- `triage/*` - Issue status (duplicate, wontfix, invalid, needs-info)
- `priority/*` - Priority levels (low, medium, high, critical)
- Community: `good first issue`, `help wanted`

## Architecture Decisions

### Go CLI with Container Packaging

The sync system is implemented as a Go CLI (`dotsync`) packaged in containers, replacing previous shell-based composite actions:

**Why Go:**

- **Type safety**: Strongly-typed config parsing and API interactions
- **Error handling**: cockroachdb/errors provides stack traces and context propagation
- **Maintainability**: ~2000 lines of idiomatic Go vs ~1600 lines of fragile shell scripts
- **Testing**: Go's testing framework vs manual bash testing
- **Performance**: Compiled binary with efficient rate limiting via go-github-ratelimit
- **Reliability**: No JSON/YAML parsing with jq/yq, proper data structures instead

**Why Containers:**

- **Distribution**: Single artifact (container image) deployed via GHCR
- **Versioning**: Semantic versioning with multi-arch builds via GoReleaser
- **Consistency**: Identical execution environment across all workflows
- **Simplicity**: Native Docker action support (`runs.using: docker`)
- **Updates**: Update CLI version by changing container tag in one place

**Implementation Stack:**

- **CLI Framework**: Kong (struct-based, cleaner than Cobra)
- **GitHub API**: google/go-github v79+ with rate limiting wrapper
- **Error Handling**: cockroachdb/errors for stack traces and wrapping
- **Logging**: Standard library slog with configurable levels
- **Build/Release**: GoReleaser with multi-arch container builds to GHCR

**Composite Action Wrappers:**

- Container-based composite actions (`.github/actions/dotsync-*`) wrap CLI commands
- Provide clean `with:` interface matching old shell-based actions
- All use `docker://ghcr.io/smykla-labs/dotsync:latest` for direct container execution
- Simpler than shell-based actions: just map inputs to CLI flags

### Unified Config Design

- Single `.github/sync-config.yml` controls label, file, and smyklot sync
- Repos opt-in to features (skip flags, exclusions, removal)
- Label/file removal defaults to false (safer)
- Config fetched per-repo at sync time by dotsync CLI
- Smyklot sync respects both `sync.skip` and `sync.smyklot.skip` flags
- Type-safe config parsing with proper validation and error messages

### Reusable Workflows Over File Sync

For CI/CD workflows, use reusable workflows instead of file sync:

- **Why**: Industry best practice 2025 (instant updates, no PRs, version pinning)
- **Anti-pattern**: Syncing workflow files requires PRs in every repo, version management nightmare
- **Approach**: Create shared workflows in `.github/workflows/lib-*.yml`, repos call via `uses:`

## Important Notes

- This repository contains both GitHub Actions workflows AND a Go CLI (`dotsync`)
- Build automation uses `task` (see `Taskfile.yaml` for targets: lint, test, build, fmt)
- Tool versions managed via mise (`.mise.toml`)
- Go code follows strict linting (golangci-lint with wrapcheck for error handling)
- All automation is via GitHub Actions workflows calling dotsync container
- Changes to `labels.yml`, `settings.yml`, or `templates/**` trigger automatic syncs
- Smyklot releases trigger automatic version sync across all repos
- Workflows exclude `.github` and `smyklot` repositories from their respective sync targets
- Label and settings sync happen directly via API (no PRs)
- File and smyklot sync create PRs for review (smyklot with auto-merge)
- Settings sync includes no-downgrade protection (never reduces security requirements)
- Settings sync is GHAS-aware (gracefully handles missing Advanced Security)
- Dry run mode available to preview changes without making them
- Community actions pinned to commit SHAs for security
- dotsync CLI is type-safe, tested, and maintainable (~2000 lines of idiomatic Go)
- Container images published to GHCR with multi-arch support (amd64, arm64)
- Unified config system supports per-repo customization
- Reusable workflows provide instant updates without PRs
- See `examples/sync-config.yml` for complete configuration schema
- See `docs/MIGRATION.md` for reusable workflow migration guide
- **Matrix pattern**: All sync workflows use `dotsync-repos-list` action with `format: json` (default)
  - Format `json`: Returns array of objects `[{name, full_name, ...}]` - use `${{ matrix.repo.name }}`
  - Format `names`: Returns array of strings `["repo1", "repo2"]` - use `${{ matrix.repo }}`
  - Current workflows use `json` format for richer repository metadata access
