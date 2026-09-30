# Repository sync catalog

These files are the source for Smyklot's Git-backed shared-file sync. Smyklot
reads `catalog.json` at a Git commit, resolves the profiles selected for each
repository, and proposes the resulting files by pull request.

| Profile | Files |
| --- | --- |
| `base` | Community files, Renovate, and Markdown lint configuration |
| `typescript` | oxlint configuration |
| `go` | golangci-lint and common Go mise tools/tasks |
| `opencode-plugin` | CI, npm publishing, and common mise tools/tasks |

Select `base`, `typescript`, and `opencode-plugin` for an OpenCode plugin.
The shared mise fragment is `mise/conf.d/00-shared.toml`. A repo's own
`mise.toml` supplies `typecheck`, `test`, `check`, and package-specific tasks.
Set `OPENCODE_PLUGIN_E2E=true` only
when the repository defines `test:e2e`.

Select `base` and `go` for a new Go repository. Existing Go repositories have
different lint rules and build tasks; migrate them individually. The `go` and
`opencode-plugin` profiles both own `mise/conf.d/00-shared.toml`: keep both available to a
workspace, but select only one for each repository.

The npm publish workflow defaults to a dry run. After the repository's
`publish.yml` and `npm` environment are registered as an npm trusted
publisher, set `NPM_PUBLISH_ENABLED=true`. A repository migrating from a
different workflow filename or environment must update npm's trusted
publisher before enabling publication.

The `base` profile holds the eight files previously entered in Smyklot's panel.
Most use this repository's root files as their source. Renovate uses
`sync/files/base/renovate.json` because this repository's own `renovate.json`
has an extra rule for its workflow templates.

When connecting this catalog, remove the eight matching inline templates in
the same settings change. Smyklot rejects a path owned by both sources. Keep
repository-specific file adjustments: they are keyed by destination path.
Review the first sync plan before applying file pull requests.
