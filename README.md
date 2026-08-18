# smykla-skalski/.github

Organization-wide defaults for smykla-skalski repositories: the community health
files GitHub falls back to, and the reusable CI workflows other repositories call.

## Community health files

GitHub applies these to every repository in the organization that does not carry
its own copy. Nothing has to run for that to happen; it is a feature of a
repository named `.github`.

- `CODE_OF_CONDUCT.md`
- `CONTRIBUTING.md`
- `LICENSE`
- `SECURITY.md`
- `.github/ISSUE_TEMPLATE/*`
- `.github/PULL_REQUEST_TEMPLATE.md`

A repository that wants its own writes its own, and that copy wins.

## Reusable workflows

Shared CI for Go projects, called from any repository with `workflow_call`.

| Workflow | What it runs |
| --- | --- |
| `lib-lint.yml` | golangci-lint, yamllint, shellcheck, markdownlint |
| `lib-test.yml` | the Go test suite, with coverage reporting |
| `lib-build.yml` | cross-platform Go binaries |
| `lib-release.yml` | semantic versioning and GitHub releases |

Pin by commit SHA, with the tag in a comment beside it, so an update is a change
somebody reviews rather than one that arrives on its own:

```yaml
jobs:
  lint:
    uses: smykla-skalski/.github/.github/workflows/lib-lint.yml@abc1234 # v1.0.0
    with:
      go-version: "1.25.x"
      enable-golangci-lint: true
```

`git rev-parse v1.0.0` in this repository gives the SHA for a tag.

See [.github/workflows/lib-README.md](.github/workflows/lib-README.md) for every
input each one takes.

## Synchronization

Labels, repository settings, rulesets and shared files are synchronized by
[Smyklot](https://github.com/smykla-skalski/smyklot), which runs as a service and
is configured in its panel rather than in this repository.

The `dotsync` CLI that used to do it lived here and has been removed. What it
held moved into Smyklot: the label set, `settings.yml`, the `main-branch-protection`
ruleset, and the templates that were under `templates/`. Two differences are worth
knowing, because they are not oversights:

- A repository no longer customises the org's templates through its own
  `.github/sync-config.yml`. Those adjustments are kept against the repository in
  the panel, so a rename cannot orphan one and a repository cannot grant itself
  something the organization did not offer. Any `sync-config.yml` still in a
  repository is read by nothing.
- A file change arrives as a pull request, and a plan is shown and approved before
  anything is written. dotsync computed and applied in one pass and reported the
  plan as though it were the outcome.

`.github/scripts/migrate-smyklot-pending-ci-labels.sh` stays: it renames the
hyphenated `smyklot:pending-ci` labels to the colon-separated names Smyklot uses,
on one repository at a time.
