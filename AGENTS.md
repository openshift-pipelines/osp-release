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
- If Python tooling is needed, use `python3` and `uv`.
- Run targeted tests before broad test runs.
