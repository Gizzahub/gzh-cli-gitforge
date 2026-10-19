---
id: ISSUE-66
title: "install-mdformat prints a misleading uv PATH warning"
type: bug
priority: P3
severity: low
discovered-in: "session review of the bin/tools mdformat pin (d0c1197b)"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

`make install-mdformat` installs mdformat into `bin/tools/` on purpose. The
gate calls the tool by its absolute path and never puts that directory on
`PATH`. uv still warns that `bin/tools/` is not on your PATH. It also
suggests `export PATH=...` or `uv tool update-shell`. Following that advice
would change the user's shell for no benefit.

With uv 0.12.0, adding `-q` to `uv tool install` drops the warning and the
"Installed 1 executable" line. An unsatisfiable pin still prints uv's
resolution error and exits 1. This was checked on 2026-10-19 with a
throwaway `UV_TOOL_DIR`.

## Reproduction

1. Remove `bin/tools/mdformat`.
1. Run `make install-mdformat`.
1. The output ends with the PATH warning from uv.

## Expected vs Actual

- Expected: the install prints only the target's own progress line, and
  install failures stay visible.
- Actual: uv's PATH warning and `update-shell` advice appear on every fresh
  install.

## Resolution

Resolved on 2026-10-19. `install-mdformat` now calls
`uv tool install --quiet`. A fresh install into an empty `bin/tools` printed
only the target's own two lines. An unsatisfiable pin still printed uv's
resolution error and the target exited 1. `--quiet` hides every uv warning,
not only the PATH one; errors still print. The same change pinned the
transitive dependencies (ISSUE-63) and added the Windows suffix (ISSUE-64).
