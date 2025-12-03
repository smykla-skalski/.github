# smykla-labs/.github

Organization-wide defaults and synchronization for smykla-labs repositories.

## What This Repository Does

1. **Community Health Files** - Default templates for all repos (CODE_OF_CONDUCT, CONTRIBUTING, SECURITY, issue/PR templates)
2. **Label Sync** - Automated synchronization of GitHub labels across all repositories
3. **File Sync** - Automated synchronization of specified files across repositories
4. **Reusable Workflows** - Shared CI/CD workflows for Go projects (lint, test, build, release)

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

**Features:**

- Direct API updates (no PRs)
- Efficient map-based diff algorithm
- Per-repo exclusions and skip flags
- Optional label removal (delete non-central labels)

**Per-repo configuration**: Create `.github/sync-config.yml` in any repo to customize:

```yaml
sync:
  labels:
    skip: false                       # Skip label sync
    exclude: ["ci/skip-tests"]       # Don't sync these labels
    allow_removal: true              # Delete non-central labels
```

### File Synchronization

The `sync-files.yml` workflow synchronizes files from `templates/` to all repositories.

**Features:**

- Creates PRs with file changes
- Per-file commits
- Per-repo exclusions and skip flags
- Custom file sync action (no external dependencies)

**Per-repo configuration**: Create `.github/sync-config.yml` to customize:

```yaml
sync:
  files:
    skip: false                       # Skip file sync
    exclude: ["CONTRIBUTING.md"]     # Don't sync these files
    allow_removal: false             # Don't delete non-central files
```

### Reusable Workflows

Shared CI/CD workflows for Go projects. These provide standardized, version-controlled workflows that can be called from any repository.

**Available Workflows:**

- **lib-lint.yml** - Multi-linter (golangci-lint, yamllint, shellcheck, markdownlint)
- **lib-test.yml** - Go test runner with coverage reporting
- **lib-build.yml** - Cross-platform Go binary builder
- **lib-release.yml** - Semantic versioning and GitHub releases

**Usage Example:**

```yaml
name: CI
on: [push, pull_request]

jobs:
  lint:
    uses: smykla-labs/.github/.github/workflows/lib-lint.yml@abc1234 # v1.0.0
    with:
      go-version: "1.23.x"
      enable-golangci-lint: true

  test:
    uses: smykla-labs/.github/.github/workflows/lib-test.yml@abc1234 # v1.0.0
    with:
      go-version: "1.23.x"
      coverage-threshold: 80
```

**Benefits:**

- Instant updates when pinning to tags/SHAs
- No PRs needed across repos
- Consistent CI/CD across organization
- Easy version management

See [docs/MIGRATION.md](docs/MIGRATION.md) for migration guide.

## Configuration

### sync-config.yml Reference

Each repository can have a `.github/sync-config.yml` file to customize sync behavior:

```yaml
sync:
  skip: false               # Skip ALL syncs for this repo

  labels:
    skip: false             # Skip label sync only
    exclude: []             # Label names to exclude from sync
    allow_removal: false    # Delete labels not in central config

  files:
    skip: false             # Skip file sync only
    exclude: []             # File paths to exclude from sync
    allow_removal: false    # Delete files not in central config (DANGEROUS)
```

**Key Fields:**

- `sync.skip` - Completely disable all syncs for this repo
- `exclude` - List of labels/files to NOT sync (they're preserved but not managed)
- `allow_removal` - Delete items in repo that aren't in central config (defaults to `false` for safety)

See [examples/sync-config.yml](examples/sync-config.yml) for full schema documentation with examples.

### Adding New Labels

1. Edit `.github/labels.yml` in this repository
2. Commit and push to `main`
3. The workflow will automatically sync to all repositories

### Adding New Sync Files

1. Add the file to `templates/` directory (preserving the desired path structure)
2. Commit and push to `main` - syncs automatically to all repos

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

## Troubleshooting

### Sync not running for a repo

**Check:**

- Does the repo have `.github/sync-config.yml` with `skip: true`?
- Is the repo excluded by the `get-org-repos` action?
- Check workflow logs in Actions tab for errors

### Labels not updating

**Possible causes:**

- Label excluded in repo's `.github/sync-config.yml` (check `labels.exclude`)
- Repo has `labels.skip: true` in sync config
- API rate limits (check workflow logs)

**Debug:**

- Run workflow with "Dry run" enabled to see what would change
- Check workflow summary for repo-specific results

### File sync PR not created

**Possible causes:**

- No changes detected (files already up-to-date)
- File excluded in repo's `.github/sync-config.yml` (check `files.exclude`)
- Repo has `files.skip: true` in sync config
- Existing PR already open (check for `chore/org-sync` branch)

**Debug:**

- Run workflow with "Dry run" to see planned changes
- Check if PR already exists: `gh pr list --label org-sync`

### Reusable workflow not found

**Error**: `error: workflows/lib-lint.yml not found`

**Fix**: Ensure the path includes `.github/` prefix:

```yaml
uses: smykla-labs/.github/.github/workflows/lib-lint.yml@abc1234
```

### Version pinning

**Best practice**: Pin to commit SHA with semver comment:

```bash
# Get SHA for a tag
git rev-parse v1.0.0

# Use in workflow
uses: smykla-labs/.github/.github/workflows/lib-lint.yml@abc1234 # v1.0.0
```

**Why**: Commit SHAs are immutable; tags can be moved (security risk)
