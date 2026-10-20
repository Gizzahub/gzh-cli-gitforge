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
config before the planner's self-sync guard runs.

## Reproduction

Derived from the code by the reviewer, following the CLI's loading order
(flat load, `LoadConfigRecursive`, then `ensureChildConfigs`); not run end
to end. A root `.gz-git.yaml` with
`workspaces: {self: {path: ., configLink: shared.yaml}}`, or with `path`
set to a symlink to the root. The workspace defaults to type git with no
URL, so it is not skipped. A real `gz-git workspace sync` replaces the
devbox's `.gz-git.yaml` with a symlink to `shared.yaml`.

## Expected vs Actual

- Expected: a workspace that is the config directory is skipped with the
  self-sync warning before any config write.
- Actual: the devbox's own config is replaced by a symlink.
