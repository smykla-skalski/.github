# smykla-labs/.github

Organization-wide defaults and synchronization for smykla-labs repositories.

## What This Repository Does

1. **Community Health Files** - Default templates for all repos (CODE_OF_CONDUCT, CONTRIBUTING, SECURITY, issue/PR templates)
2. **Label Sync** - Automated synchronization of GitHub labels across all repositories
3. **File Sync** - Automated synchronization of specified files across repositories

## How It Works

### Community Health Files (Native GitHub Feature)

Files in this repository automatically apply to all smykla-labs repositories that don't have their own versions:

- `CODE_OF_CONDUCT.md`
- `CONTRIBUTING.md`
- `LICENSE`
- `SECURITY.md`
- `.github/ISSUE_TEMPLATE/*`
- `.github/PULL_REQUEST_TEMPLATE.md`

### Label Synchronization

The `sync-labels.yml` workflow synchronizes labels from `.github/labels.yml` to all repositories in the organization.

**Per-repo configuration**: Create `.github/sync-config.yml` in any repo to customize:

```yaml
labels:
  # Disable specific labels for this repo
  disabled:
    - "smyklot:pending-ci:rebase"
  # Add repo-specific labels (won't be removed by sync)
  allow_additional: true
```

### File Synchronization

The `sync-files.yml` workflow synchronizes files from `templates/` to all repositories.

**Per-repo configuration**: Create `.github/sync-config.yml` to customize:

```yaml
files:
  # Disable specific files from syncing
  disabled:
    - "CONTRIBUTING.md"
    - ".github/ISSUE_TEMPLATE/bug_report.yml"
```

## Configuration

### sync-config.yml Reference

Each repository can have a `.github/sync-config.yml` file:

```yaml
# Label sync configuration
labels:
  # Skip this repo entirely for label sync
  skip: false

  # List of label names to NOT sync to this repo
  disabled:
    - "smyklot:pending-ci"
    - "smyklot:pending-ci:squash"

  # Allow repo to keep labels not in the central config
  allow_additional: true

# File sync configuration
files:
  # Skip this repo entirely for file sync
  skip: false

  # List of files to NOT sync to this repo
  disabled:
    - "CONTRIBUTING.md"
    - ".github/PULL_REQUEST_TEMPLATE.md"
```

### Adding New Labels

1. Edit `.github/labels.yml` in this repository
2. Commit and push to `main`
3. The workflow will automatically sync to all repositories

### Adding New Sync Files

1. Add the file to `templates/` directory
2. Update the `FILES` array in `.github/workflows/sync-files.yml`
3. Commit and push to `main`

## Setup

### Authentication

Workflows use the **smyklot** GitHub App for authentication. The app must be installed on the organization with access to all repositories.

Required org-level configuration:
- `vars.SMYKLOT_APP_ID` - GitHub App ID
- `secrets.SMYKLOT_PRIVATE_KEY` - GitHub App private key

## Manual Sync

You can manually trigger syncs via GitHub Actions:

1. Go to Actions tab
2. Select "Sync Labels" or "Sync Files"
3. Click "Run workflow"
4. Optionally enable "Dry run" to preview changes
