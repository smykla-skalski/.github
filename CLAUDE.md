# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

Organization-wide defaults and synchronization for smykla-labs repositories. This is a special `.github` repository that:

1. **Community Health Files** - Provides default templates for all smykla-labs repos (CODE_OF_CONDUCT, CONTRIBUTING, SECURITY, issue/PR templates)
2. **Label Sync** - Automated synchronization of GitHub labels across all repositories using custom composite action
3. **File Sync** - Automated synchronization of specified files across repositories using custom composite action
4. **Reusable Workflows** - Shared CI/CD workflows for Go projects (lint, test, build, release)

## Repository Structure

```text
.
├── .github/
│   ├── labels.yml              # Label definitions synced to all repos
│   ├── workflows/
│   │   ├── sync-labels.yml     # Label sync workflow
│   │   ├── sync-files.yml      # File sync workflow
│   │   ├── lib-lint.yml        # Reusable lint workflow
│   │   ├── lib-test.yml        # Reusable test workflow
│   │   ├── lib-build.yml       # Reusable build workflow
│   │   └── lib-release.yml     # Reusable release workflow
│   ├── actions/                # Reusable composite actions
│   │   ├── generate-token/     # GitHub App token generation
│   │   ├── get-org-repos/      # Fetch org repositories
│   │   ├── get-sync-config/    # Fetch per-repo sync configuration
│   │   ├── sync-labels-to-repo/ # Label sync with config support
│   │   └── sync-files-to-repo/ # File sync with config support
│   ├── ISSUE_TEMPLATE/         # Default issue templates
│   └── PULL_REQUEST_TEMPLATE.md
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
└── SECURITY.md
```

## Key Concepts

### Community Health Files (Native GitHub)

Files in the root automatically apply to all smykla-labs repositories that don't have their own versions. This is a native GitHub feature for `.github` repositories.

### Synchronization System

The synchronization system uses custom composite actions with unified per-repo configuration:

**Label Sync:**

- **Action**: Custom composite action (`.github/actions/sync-labels-to-repo`)
- **Workflow**: `.github/workflows/sync-labels.yml`
- **Config**: `.github/labels.yml` (central) + per-repo `.github/sync-config.yml`
- **Method**: Direct API updates via GitHub CLI
- **Features**: Label removal, exclusions, skip flags

**File Sync:**

- **Action**: Custom composite action (`.github/actions/sync-files-to-repo`)
- **Workflow**: `.github/workflows/sync-files.yml`
- **Source**: All files in `templates/` directory (auto-discovered)
- **Config**: Per-repo `.github/sync-config.yml` (optional)
- **Method**: Creates PRs with file changes via GitHub API
- **Features**: File exclusions, skip flags, PR management, smart renovate.json handling

**Unified Config:**

- Repos can create `.github/sync-config.yml` to customize behavior
- Control both label and file sync from single config file
- Supports skip flags, exclusions, and removal options
- See `examples/sync-config.yml` for full schema

**Flow:**

- Each workflow triggers independently on relevant file changes
- Both workflows run for all repos in parallel
- Label sync updates directly; file sync creates PRs with `org-sync` label
- File sync uses `chore/org-sync` branch for changes

### Label Synchronization

- **Source**: `.github/labels.yml` contains all label definitions
- **Format**: YAML with `name`, `color` (with #), and optional `description`
- **Target**: All repositories in smykla-labs organization (except `.github` itself)
- **Method**: Custom composite action using GitHub CLI
- **Features**: Efficient map-based diff, parallel processing, dry-run support

### File Synchronization

- **Source**: `templates/` directory (all files auto-discovered)
- **Config**: Per-repo `.github/sync-config.yml` (optional)
- **Target**: All repositories in smykla-labs organization (except `.github` itself)
- **Method**: Custom composite action using GitHub API
- **Features**: PR creation, single commit, exclusions, skip flags, smart renovate.json handling

**Smart renovate.json Handling:**
- Detects manual modifications to `renovate.json` by checking commit history
- If manual commits found (not from sync workflow), excludes file from sync
- Shows alert in PR with instructions to add to `.github/sync-config.yml`
- This is the ONLY file with this special behavior

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
      go-version: "1.23.x"
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

### Manual Sync

Trigger workflows manually via GitHub Actions:
1. Go to Actions tab in this repository
2. Select "Sync Labels" or "Sync Files"
3. Click "Run workflow"
4. Optionally enable "Dry run" to preview changes

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

## Files to Edit

### Labels

- **Definition**: `.github/labels.yml`
- **Workflow**: `.github/workflows/sync-labels.yml`

### Files

- **Source**: `templates/` directory (auto-discovered)
- **Workflow**: `.github/workflows/sync-files.yml`

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

### Custom Composite Actions for Sync

Both label and file sync use custom composite actions instead of third-party actions:

**Label Sync:**

- **Location**: `.github/actions/sync-labels-to-repo`
- **Why**: Most label sync actions don't support multi-repo targeting with per-repo config
- **Implementation**: Bash script using GitHub CLI and jq for efficient map-based diff
- **Benefits**: Full control, no external dependencies, easy to maintain, label removal support

**File Sync:**

- **Location**: `.github/actions/sync-files-to-repo`
- **Why**: Need per-repo exclusions and unified config support (not available in BetaHuhn)
- **Implementation**: Bash script using GitHub API for PR creation and file management
- **Benefits**: Full control, per-file exclusions, unified config, no external dependencies

### Unified Config Design

- Single `.github/sync-config.yml` controls both label and file sync
- Repos opt-in to features (skip flags, exclusions, removal)
- Label/file removal defaults to false (safer)
- Config fetched per-repo at sync time

### Reusable Workflows Over File Sync

For CI/CD workflows, use reusable workflows instead of file sync:

- **Why**: Industry best practice 2025 (instant updates, no PRs, version pinning)
- **Anti-pattern**: Syncing workflow files requires PRs in every repo, version management nightmare
- **Approach**: Create shared workflows in `.github/workflows/lib-*.yml`, repos call via `uses:`

## Important Notes

- This repository does NOT use standard `make` targets or testing frameworks
- All automation is via GitHub Actions workflows
- Changes to `labels.yml` or `templates/**` trigger automatic syncs
- Workflows exclude `.github` repository from sync targets
- Label sync happens directly via API (no PRs)
- File sync creates PRs for review
- Dry run mode available to preview changes without making them
- Community actions pinned to commit SHAs for security
- Both sync actions are custom, simple, and maintainable (~100-200 lines each)
- Unified config system supports per-repo customization
- Reusable workflows provide instant updates without PRs
- See `examples/sync-config.yml` for complete configuration schema
- See `docs/MIGRATION.md` for reusable workflow migration guide
