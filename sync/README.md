# Repository sync catalog

These files are the source for Smyklot's Git-backed shared-file sync. Smyklot
reads `catalog.json` at a Git commit, resolves the profiles selected for each
repository, and proposes the resulting files by pull request.

| Profile | Files |
| --- | --- |
| `base` | Markdown lint configuration |
| `typescript` | oxlint configuration |
| `go` | golangci-lint and common Go mise tools/tasks |
| `opencode-plugin` | CI, npm publishing, and common mise tools/tasks |

Select `base`, `typescript`, and `opencode-plugin` for an OpenCode plugin.
The plugin's `mise.toml` override supplies its own `typecheck`, `test`,
`check`, and package-specific tasks. Set `OPENCODE_PLUGIN_E2E=true` only
when the repository defines `test:e2e`.

Select `base` and `go` for a new Go repository. Existing Go repositories have
different lint rules and build tasks; migrate them individually. The `go` and
`opencode-plugin` profiles both own `mise.toml`: keep both available to a
workspace, but select only one for each repository.

The npm publish workflow defaults to a dry run. After the repository's
`publish.yml` and `npm` environment are registered as an npm trusted
publisher, set `NPM_PUBLISH_ENABLED=true`. A repository migrating from a
different workflow filename or environment must update npm's trusted
publisher before enabling publication.

The catalog becomes active only after Smyklot supports Git-backed profiles.
Until then, the panel's existing file templates remain authoritative.
