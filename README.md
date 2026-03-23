# osp-release

`osp-release` is a small Go CLI for looking up OpenShift Pipelines release data from Confluence, Jira, and upstream GitHub releases.

<img width="3335" height="1071" alt="image" src="https://github.com/user-attachments/assets/6e7f0a7e-732d-4ccc-9368-b906857109f0" />

## Install

```bash
go install github.com/openshift-pipelines/osp-release/cmd/osp-release@latest
```

Homebrew:

```bash
brew tap openshift-pipelines/osp-release https://github.com/openshift-pipelines/osp-release
brew install --cask osp-release
```

Linux packages are published on GitHub Releases as `.deb`, `.rpm`, and `.apk` artifacts.

## Auth

Release and component commands need Jira credentials.

Use env vars:

```bash
export OSP_JIRA_EMAIL='you@example.com'
export OSP_JIRA_TOKEN='token'
```

Or use `pass::` references:

```bash
export OSP_JIRA_EMAIL='pass::jira/email'
export OSP_JIRA_TOKEN='pass::jira/token'
```

Flags override env vars:

```bash
osp-release release latest --jira-email 'pass::jira/email' --jira-token 'pass::jira/token'
```

`upstream` commands use normal GitHub CLI auth conventions through the official `go-gh` library.

The component catalog lives in `internal/release/components.yaml` and is embedded into the binary at build time. Edit that file if you need to adjust component names, display labels, or repo mappings.

## Commands

```bash
osp-release release latest
osp-release release show 1.21
osp-release release list --json
osp-release release all --unreleased
osp-release component list
osp-release component show pac
osp-release upstream list
osp-release upstream show pac --field version --quiet
```

## Output

- Default on TTY: table
- Default when piped or redirected: JSON
- Override with `--output table|text|json`
- Release commands include component versions by default
- Release listings hide unreleased versions unless `--unreleased` is passed
- Use `--field` to select stable fields
- Use `--no-components` to hide component versions in release output
- Use `--quiet` with a single `--field` for bare values
- Use `--no-headers` to hide table headers

Examples:

```bash
osp-release release latest --field version
osp-release release show 1.21
osp-release release latest --no-components
osp-release release list --unreleased
osp-release release list --field minor --field version --output text
osp-release upstream list --json
```

## Development

```bash
make build
make test
make lint
make release-snapshot
```

## Release

CI runs tests, lint, and `goreleaser check`. Tags publish release archives, Homebrew casks, and Linux packages with GoReleaser.

The Homebrew cask is published directly to `homebrew/Casks/` in this repository using the workflow's `GITHUB_TOKEN`.
