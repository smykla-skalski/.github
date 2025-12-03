# Reusable Workflows

Shared CI/CD workflows for Go projects. Call via `workflow_call` trigger. Always pin to commit SHA: `git rev-parse v1.0.0`

## lib-lint.yml

Multi-linter: golangci-lint, yamllint, shellcheck, markdownlint.

**Inputs:** `go-version` (default: `1.23.x`), `enable-golangci-lint` (default: `true`), `enable-yamllint`, `enable-shellcheck`, `enable-markdownlint`, `golangci-lint-args`

**Outputs:** `lint-passed` (boolean)

```yaml
uses: smykla-labs/.github/.github/workflows/lib-lint.yml@abc1234 # v1.0.0
with:
  go-version: "1.23.x"
  enable-golangci-lint: true
```

## lib-test.yml

Go test runner with coverage.

**Inputs:** `go-version`, `test-flags` (default: `"-v -race"`), `coverage-threshold` (default: `0`), `test-timeout` (default: `"10m"`)

**Outputs:** `coverage-percent`

```yaml
uses: smykla-labs/.github/.github/workflows/lib-test.yml@abc1234 # v1.0.0
with:
  coverage-threshold: 80
```

## lib-build.yml

Cross-platform Go binary builder.

**Inputs:** `go-version`, `build-targets` (JSON array, default: `'["linux/amd64"]'`), `artifact-name`, `binary-name`, `ldflags` (default: `"-s -w"`)

**Outputs:** `artifact-name`

```yaml
uses: smykla-labs/.github/.github/workflows/lib-build.yml@abc1234 # v1.0.0
with:
  build-targets: '["linux/amd64", "darwin/amd64", "windows/amd64"]'
```

## lib-release.yml

Semantic versioning and GitHub releases. Auto-detects version from commits: `feat!:` → major, `feat:` → minor, else → patch.

**Inputs:** `version-type` (default: `"auto"`), `prerelease`, `draft`, `generate-notes` (default: `true`)

**Outputs:** `version`, `release-url`

```yaml
uses: smykla-labs/.github/.github/workflows/lib-release.yml@abc1234 # v1.0.0
with:
  version-type: "auto"
```

## Usage Pattern

```yaml
name: CI
on: [push, pull_request]

jobs:
  lint:
    uses: smykla-labs/.github/.github/workflows/lib-lint.yml@abc1234 # v1.0.0
    with:
      enable-golangci-lint: true

  test:
    uses: smykla-labs/.github/.github/workflows/lib-test.yml@abc1234 # v1.0.0
    with:
      coverage-threshold: 80

  build:
    needs: [lint, test]
    uses: smykla-labs/.github/.github/workflows/lib-build.yml@abc1234 # v1.0.0
```
