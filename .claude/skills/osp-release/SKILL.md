---
name: osp-release
description: Look up OpenShift Pipelines (OSP) release data with the osp-release
  CLI. Use for questions about the latest or unreleased OSP version, what shipped
  in a given minor such as 1.21, which version of a component like Pipelines as
  Code, Tekton Chains, Triggers, Results, tkn, or opc went into a release, how a
  shipped component compares to its latest upstream tag, and whether an OSP
  version is still supported, in maintenance, or past end of life.
---

# osp-release

`osp-release` is a read-only CLI over OpenShift Pipelines release data from
Confluence, Jira, the Red Hat product life cycle API, and upstream GitHub
releases. It makes no changes to anything.

## Establish capabilities first

```bash
command -v osp-release || echo missing
osp-release version --json
osp-release describe
```

`describe` is the authoritative schema: every command path, its positional
arguments, its flags, the exact `--field` names it accepts, which environment
variables it needs, and the full exit code and error kind tables. It prints
JSON even on a terminal.

Narrow it when you only need one command:

```bash
osp-release describe commands release show
osp-release describe fields release show
```

Rules:

- Treat `describe` as the source of truth. Do not parse `--help`.
- Do not invent a flag or a field name. If `describe` does not list it, it does
  not exist; say so and, if the flag is documented elsewhere, tell the user the
  binary needs upgrading.
- If `osp-release` is not on `PATH`, stop and report that. Do not query
  Confluence, Jira, or GitHub by hand to work around it.

## Reading data

Always request JSON, and select fields rather than post-processing:

```bash
osp-release release latest --json
osp-release release show 1.21 --json
osp-release release list --json --field minor --field version --field eol_date
osp-release support show 1.21 --field eol_date --quiet
```

- `--json` is explicit and safe. Output is already JSON when stdout is not a
  terminal, but say what you mean.
- `--field` repeats to select multiple fields. Names come from
  `describe fields`.
- `--quiet` with exactly one `--field` prints a bare value, ideal for capturing
  into a shell variable.
- Never parse table output. Tables are for humans, the column layout is not a
  contract, and on a terminal they carry ANSI colour.
- `release` output includes component versions by default; add
  `--no-components` when you only want the release metadata.
- Release listings hide unreleased versions unless you pass `--unreleased`.
- `support list` hides end-of-life versions unless you pass `--all`.
  `support show` resolves any version, including end-of-life ones.

## Credentials

`release` and `component` commands need Jira credentials in the environment:

```bash
OSP_JIRA_EMAIL   # Jira account email
OSP_JIRA_TOKEN   # Jira API token
```

- Never read, print, echo, or log these values.
- Both accept a `pass::entry` reference, which the CLI resolves itself. Never
  run `pass` yourself to expand one.
- `support` and `upstream` need no Jira credentials. Prefer them when they
  answer the question: a support-status or EOL question never needs Jira.
- `upstream` uses ordinary GitHub CLI auth.

## Caching

Jira, Confluence, and life cycle data are cached for 7 days. Answers may be up
to a week old.

Pass `--refresh` only when freshness genuinely matters, such as checking
whether a release just shipped, and tell the user you bypassed the cache. It
makes live network calls, so do not use it in a loop.

## Handling failures

Match on the exit code and on the `error` field of the JSON payload, never on
message text. `osp-release describe` prints the full table; the ones that need
a decision from you:

| Exit | Meaning | What to do |
|---|---|---|
| 2 | usage | You passed a bad flag, field, or argument. Re-read `describe` and correct it. |
| 3 | not found | The release, component, or command does not exist. Report that plainly; do not retry. |
| 4 | auth | Credentials are missing or rejected. Tell the user to set `OSP_JIRA_EMAIL` and `OSP_JIRA_TOKEN`. Do not retry. |
| 5 | dependency | An external tool such as `pass` failed. Report it. |
| 6 | upstream | Jira, Confluence, or GitHub failed. Retrying once is reasonable. |

When JSON output is requested, the error payload goes to stdout as
`{"error": "<kind>", "message": "..."}` plus any context fields, and the human
message goes to stderr. Capture both.

## Recipes

Latest released version:

```bash
osp-release release latest --field version --quiet
```

Everything that shipped in one minor, components included:

```bash
osp-release release show 1.21 --json
```

One component across every release, plus its latest upstream tag:

```bash
osp-release component show pac --json
```

Is a version still supported, and when does it go end of life:

```bash
osp-release support show 1.21 --json
```

Which versions are still supported at all:

```bash
osp-release support list --json
```

Shipped versus latest upstream for one component:

```bash
osp-release release latest --field pac --quiet
osp-release upstream show pac --field version --quiet
```

Component names are listed by `osp-release component list --json` and appear as
the `values` of the positional argument in
`osp-release describe commands upstream show`.

## Safety

- This tool is read-only. There is no mutating command, so there is nothing to
  dry-run and nothing to confirm.
- The component catalog lives in `internal/release/components.yaml` and is
  embedded at build time. Do not edit it unless the user asks. If you do, the
  binary must be rebuilt with `make build` before the change takes effect.
