# osp-release Agent Notes

- Do not commit, tag, or rewrite git history without explicit user approval.
- This is a new project. Do not preserve command compatibility with the old Python script.
- Use the Python script only as a reference for release data behavior.
- Prefer idiomatic Go and stdlib-first design.
- Keep command output stable and automation-friendly.
- Every user-facing command should support JSON and field selection unless there is a strong reason not to.
- Default to table output on TTY and JSON otherwise.
- Credentials are env-first; only resolve `pass::` when explicitly requested by env or flags.
- Keep docs concise and direct.
- Keep `osp-release describe` accurate: new commands need an entry in
  `commandMetadata` in `internal/cli/describe.go`, and new error kinds need a
  constant and a `Kinds()` entry in `internal/app/kinds.go`.
- The agent skill for this CLI lives in `.github/skills/osp-release/SKILL.md`
  (symlinked to `.claude/skills/osp-release`). Update it when the command
  surface changes.
- If Python tooling is needed, use `python3` and `uv`.
- Run targeted tests before broad test runs.
