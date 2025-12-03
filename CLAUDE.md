# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

Organization-wide defaults and synchronization for smykla-labs repositories. This is a special `.github` repository that:

1. **Community Health Files** - Provides default templates for all smykla-labs repos (CODE_OF_CONDUCT, CONTRIBUTING, SECURITY, issue/PR templates)
2. **Label Sync** - Automated synchronization of GitHub labels across all repositories
3. **File Sync** - Automated synchronization of specified files across repositories

## Repository Structure

```text
.
├── .github/
│   ├── labels.yml              # Label definitions synced to all repos
│   ├── workflows/
│   │   ├── sync-labels.yml     # Label sync automation
│   │   └── sync-files.yml      # File sync automation
│   ├── ISSUE_TEMPLATE/         # Default issue templates
│   └── PULL_REQUEST_TEMPLATE.md
├── templates/                  # Source files for file sync
│   ├── CODE_OF_CONDUCT.md
│   ├── CONTRIBUTING.md
│   ├── SECURITY.md
│   └── .github/
│       ├── ISSUE_TEMPLATE/
│       └── PULL_REQUEST_TEMPLATE.md
├── examples/
│   └── sync-config.yml         # Example config for repos
├── CODE_OF_CONDUCT.md          # Org-wide defaults (native GitHub)
├── CONTRIBUTING.md
└── SECURITY.md
```

## Key Concepts

### Community Health Files (Native GitHub)

Files in the root automatically apply to all smykla-labs repositories that don't have their own versions. This is a native GitHub feature for `.github` repositories.

### Label Synchronization

- **Source**: `.github/labels.yml` contains all label definitions
- **Target**: All repositories in smykla-labs organization (except `.github` itself)
- **Trigger**: Automatically on push to `main` when `labels.yml` changes, or manually via workflow dispatch
- **Per-repo config**: Repos can create `.github/sync-config.yml` to disable specific labels or skip sync

### File Synchronization

- **Source**: `templates/` directory
- **Target**: All repositories in smykla-labs organization (except `.github` itself)
- **Trigger**: Automatically on push to `main` when `templates/**` changes, or manually via workflow dispatch
- **Per-repo config**: Repos can create `.github/sync-config.yml` to disable specific files or skip sync
- **Method**: Creates PR with changes (never force-pushes)

### Authentication

Both workflows use the **smyklot** GitHub App for authentication:
- `vars.SMYKLOT_APP_ID` - GitHub App ID (org-level variable)
- `secrets.SMYKLOT_PRIVATE_KEY` - GitHub App private key (org-level secret)

## Common Tasks

### Adding New Labels

1. Edit `.github/labels.yml`
2. Follow format:
   ```yaml
   - name: "label-name"          # Max 50 chars
     color: "hex-color"          # 6-char hex without #
     description: "description"  # Max 100 chars (optional)
   ```
3. Commit and push to `main` - syncs automatically to all repos

### Adding New Sync Files

1. Add file to `templates/` directory
2. Update `FILES` array in `.github/workflows/sync-files.yml`:
   ```bash
   declare -A FILES=(
     ["templates/path/file.md"]="path/file.md"
   )
   ```
3. Commit and push to `main` - syncs automatically to all repos

### Per-Repo Configuration

Repositories can customize sync behavior with `.github/sync-config.yml`:

```yaml
labels:
  skip: false                    # Skip label sync entirely
  disabled:                      # Labels to exclude
    - "smyklot:pending-ci:rebase"
  allow_additional: true         # Keep labels not in central config

files:
  skip: false                    # Skip file sync entirely
  disabled:                      # Files to exclude
    - "CONTRIBUTING.md"
    - ".github/ISSUE_TEMPLATE/bug_report.yml"
```

### Manual Sync

Trigger workflows manually via GitHub Actions:
1. Go to Actions tab in this repository
2. Select "Sync Labels" or "Sync Files"
3. Click "Run workflow"
4. Optionally enable "Dry run" to preview changes

### Workflow Behavior

#### Label Sync

- Processes all repos in parallel (fail-fast: false)
- Creates/updates labels based on `.github/labels.yml`
- Respects per-repo `disabled` list
- If `allow_additional: false`, removes labels not in central config
- Fetches per-repo config from `.github/sync-config.yml` (if exists)

#### File Sync

- Processes all repos in parallel (fail-fast: false)
- Compares content hash to determine if sync needed
- Creates branch: `chore/file-sync-YYYYMMDDHHMMSS`
- Creates PR with changes (with `file-sync` label)
- Commit messages: `chore(sync): add/update {file}`
- Respects per-repo `disabled` list
- Fetches per-repo config from `.github/sync-config.yml` (if exists)

## Files to Edit

### Labels

- **Definition**: `.github/labels.yml`
- **Workflow**: `.github/workflows/sync-labels.yml`

### Files

- **Source**: `templates/` directory
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
- `smyklot:*` - Automation labels (pending-ci with merge strategies)
- `triage/*` - Issue status (duplicate, wontfix, invalid, needs-info)
- `priority/*` - Priority levels (low, medium, high, critical)
- Community: `good first issue`, `help wanted`

## Important Notes

- This repository does NOT use standard `make` targets or testing frameworks
- All automation is via GitHub Actions workflows
- Changes to `labels.yml` or `templates/**` trigger automatic syncs
- Workflows exclude `.github` repository from sync targets
- File sync creates PRs (never direct commits)
- Label sync modifies labels directly (no PR)
- Dry run mode available for both workflows to preview changes