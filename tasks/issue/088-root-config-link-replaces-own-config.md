---
id: ISSUE-88
title: "a root workspace at the devbox with a configLink replaces the devbox's own config"
type: bug
priority: P2
severity: medium
discovered-in: "ISSUE-85 second independent review"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

`ensureChildConfigsWithDryRun` (`pkg/workspacecli/sync_command.go`) runs
for the root config's own workspaces before planning. It resolves each
workspace path and, when `configLink` is set, replaces
`<path>/.gz-git.yaml` with a symlink. It has no self-sync guard, so a
workspace whose path is the devbox itself replaces the devbox's own
config. With `sync.recursive: true` this happens before the planner's
self-sync guard runs; without it, no planner picks the workspace up and
no guard runs at all.

## Reproduction

Reproduced at function level by the reviewer on 6edfd2d6, following the
CLI's loading order: `FileSpecLoader.Load`, `config.LoadConfigRecursive`,
then `ensureChildConfigsWithDryRun` with dry run off. The CLI itself was
not run. A root `.gz-git.yaml` with
`workspaces: {self: {path: ., configLink: shared.yaml}}`, or with `path`
set to a symlink to the root. The workspace defaults to type git with no
URL, so it is not skipped. The devbox's `.gz-git.yaml` became a symlink
to `shared.yaml` for both `path: .` and a path that is a symlink to the
root.

## Expected vs Actual

- Expected: a workspace that is the config directory is skipped with the
  self-sync warning before any config write.
- Actual: the devbox's own config is replaced by a symlink.

## Resolution

Resolved on 2026-10-21.

- `ensureChildConfigsWithDryRun` now skips a workspace whose resolved path
  is the config directory before the configLink or bootstrap branch runs,
  with
  `Skipping workspace '<name>': path '<path>' points to config directory (self-sync not allowed)`.
  It uses `pointsAtConfigDir`, which compares file identity, so both
  `path: .` and a path that is a symlink to the config directory are
  caught. The loader has already made `path` absolute at this point, so a
  string check for `.` would miss both.
- Scope: the config directory itself. An ancestor of the root config
  directory is ISSUE-87.
- Test (`pkg/workspacecli/child_config_self_guard_test.go`):
  `TestEnsureChildConfigsSkipsWorkspaceAtConfigDir` loads the config with
  `config.LoadConfigRecursive` as the CLI does, then runs
  `ensureChildConfigsWithDryRun` with dry run off. Cases: `path: .` and a
  symlink to the root. It asserts the devbox config is still the original
  regular file and the warning is printed. `TestEnsureChildConfigsSkipsBootstrapAtConfigDir`
  covers the bootstrap branch: a root loaded from `root.yaml` with a
  workspace at `path: .` and no configLink gets no `.gz-git.yaml`, and
  dry run prints the skip instead of the bootstrap preview.
- Mutation: with the guard disabled (`if false && ...`), both cases fail
  with "devbox config was replaced by a symlink", and the bootstrap test
  fails with "bootstrap config written into the devbox".
- Found in review, out of scope: a `type: config` workspace at its own
  config directory crashes the loader (ISSUE-89).
