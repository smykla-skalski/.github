# CLAUDE.md

## Repository Purpose

Organization-wide defaults and synchronization for smykla-skalski repositories. This special `.github` repository provides:

1. **Community Health Files** - Default templates (CODE_OF_CONDUCT, CONTRIBUTING, SECURITY, issue/PR templates)
2. **Label/File/Settings/Smyklot Sync** - Automated synchronization across all repos via `dotsync` CLI
3. **Reusable Workflows** - Shared CI/CD workflows (`lib-lint.yml`, `lib-test.yml`, `lib-build.yml`, `lib-release.yml`)

## Structure

```text
.github/
├── labels.yml              # Label definitions synced to all repos
├── settings.yml            # Repository settings synced to all repos
├── workflows/sync-*.yml    # Sync workflows (labels, files, settings, smyklot)
├── workflows/lib-*.yml     # Reusable workflows for Go projects
└── actions/dotsync/        # Unified container-based sync action
cmd/
├── dotsync/main.go         # Main sync CLI (Cobra, released as container)
└── schemagen/main.go       # Schema generator (go run, not released)
internal/configtypes/       # Config types (zero imports for fast schemagen compile)
pkg/
├── github/                 # GitHub API operations (labels, files, settings, smyklot)
├── config/sync.go          # Sync config parsing (returns configtypes)
├── schema/generator.go     # JSON Schema generation
└── logger/                 # slog wrapper
templates/                  # Source files for file sync (auto-discovered)
```

## Synchronization System

**Architecture**: Go CLI (`dotsync`) → container (`ghcr.io/smykla-skalski/dotsync`) → composite action

**Unified Action** (`action.yml` in repository root):

```yaml
- uses: ./
  with:
    command: labels|files|settings|smyklot|repos|config
    subcommand: sync|discover|list|verify
    token: ${{ steps.token.outputs.token }}
    repo: ${{ matrix.repo.name }}
```

### Sync Types

| Type     | Trigger               | Method          | Branch               |
|----------|-----------------------|-----------------|----------------------|
| Labels   | `labels.yml` change   | Direct API      | -                    |
| Files    | `templates/**` change | PR              | `chore/org-sync`     |
| Settings | `settings.yml` change | Direct API      | -                    |
| Smyklot  | `repository_dispatch` | PR + auto-merge | `chore/sync-smyklot` |

### Per-Repo Config

Repos customize via `.github/sync-config.yml`:

```yaml
sync:
  skip: false           # Skip ALL syncs
  labels:
    skip: false
    exclude: ["label-name"]
    allow_removal: false
  files:
    skip: false
    exclude: ["path/to/file"]
    merge:
      - path: "renovate.json"
        strategy: "deep-merge"
        arrayStrategies:           # Per-path array merge control
          "$.packageRules": "append"    # append | prepend | replace
          "$.extends": "prepend"
        deduplicateArrays: true    # Remove duplicate items
        overrides:
          packageRules: [{...}]
      - path: "CONTRIBUTING.md"
        strategy: "markdown"
        sections:
          - action: "after"
            heading: "Prerequisites"
            content: |
              ### Project Setup
              Custom setup instructions here.
  settings:
    skip: false
    exclude: ["branch_protection", "security.secret_scanning"]
  smyklot:
    skip: false
```

### Key Behaviors

- **No-downgrade**: Settings sync never reduces security (repo with 2 reviews + config says 1 = keeps 2)
- **GHAS-aware**: Skips security settings gracefully if Advanced Security unavailable
- **Hybrid status checks**: Empty `contexts: []` inherits existing; explicit overrides
- **Smart renovate.json**: Detects manual modifications, excludes from sync, shows alert
- **Array merge strategies**: Control per-path array merging (append/prepend/replace) with optional deduplication. Default: arrays replaced per RFC 7396. JSONPath exact match only (no wildcards).
- **Markdown section merge**: Heading-based section operations (after/before/replace/delete/append/prepend) for `.md` files. Case-insensitive heading match, code-fence aware, first match wins. Sections applied sequentially with re-parse between operations.
- **Matrix pattern**: `dotsync repos list --format json` returns `[{name, full_name, ...}]`

## Common Tasks

**Add labels**: Edit `.github/labels.yml`, push to main
**Add sync files**: Add to `templates/`, push to main (auto-discovered)
**Update settings**: Edit `.github/settings.yml`, push to main (use dry-run first)
**Manual sync**: Actions tab → Select workflow → Run workflow (dry-run available)

## Label Categories

- `kind/*` - Issue type (bug, enhancement, documentation, question, security)
- `area/*` - Component (ci, docs, api, testing, deps)
- `ci/*` - CI control (skip-tests, skip-lint, skip-build, force-full)
- `release/*` - Release triggers (major, minor, patch)
- `triage/*` - Status (duplicate, wontfix, invalid, needs-info)
- `priority/*` - Priority (low, medium, high, critical)
- Automation: `org-sync`, `smyklot:*`
- Community: `good first issue`, `help wanted`

## Authentication

All workflows use **smyklot** GitHub App:

- `vars.SMYKLOT_APP_ID` + `secrets.SMYKLOT_PRIVATE_KEY`

## Go Code

**CLI** (`cmd/dotsync/`): Main sync tool, add new commands here via Cobra subcommands

**Schemagen** (`cmd/schemagen/`): JSON Schema generator, run via `go run ./cmd/schemagen`

- Uses `internal/configtypes` (zero external imports) for fast compilation (~1.5s)
- NOT released as binary - always run from source to use current branch types

**Config types** (`internal/configtypes/`): All sync config structs, zero imports

- `pkg/config/sync.go` imports configtypes and adds parsing functions
- `pkg/github/*.go` imports configtypes for type references

**GitHub operations** (`pkg/github/`): Labels, files, settings, smyklot sync implementations

### Linter Rules (STRICT - will fail CI)

All rules in `.golangci.yml` are strictly enforced. Code WILL NOT pass CI if violated.

**Errors** (depguard + forbidigo - NO EXCEPTIONS):

- ONLY `github.com/cockroachdb/errors` - anything else fails lint
- `errors` stdlib → DENIED
- `github.com/pkg/errors` → DENIED
- `fmt.Errorf` → FORBIDDEN, use `errors.New`, `errors.Wrap`, `errors.Wrapf`

**Function limits** (funlen + cyclop):

- Max 100 lines, 50 statements per function
- Cyclomatic complexity max: 30

**Imports** (gci - enforced order):

1. Standard library
2. External packages
3. Local (`github.com/smykla-skalski/.github`)

**nolint directives** (nolintlint - REQUIRED format):

- Must specify linter: `//nolint:lintername`
- Must have explanation: `//nolint:lintername // reason`
- Bare `//nolint` will fail

**Whitespace** (wsl_v5):

- Blank line before returns, after control flow blocks
- Run `task fmt` to auto-fix

## Development

```bash
task lint        # golangci-lint
task test        # Run tests
task build       # Build binary
task fmt         # Format code
```

## Critical Notes

- Workflows exclude `.github` and `smyklot` repos from sync targets
- Community actions pinned to commit SHAs for security
- Reusable workflows (`lib-*.yml`) preferred over file sync for CI/CD (instant updates, version pinning)
- See `examples/sync-config.yml` for complete schema
- See `docs/MIGRATION.md` for reusable workflow migration
