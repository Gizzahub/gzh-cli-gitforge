---
id: ISSUE-90
title: "Removed make keys load silently through loaders the integrate guard does not cover"
type: bug
priority: P3
severity: low
discovered-in: "review of the legacy make gate removal (ee40ef50)"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

`pkg/config/removed_keys.go` `rejectRemovedConfigKeys` runs only in the
integrate-facing loaders: `manager.go:25`, `controller.go:38`,
`recursive.go:72` and `task_pattern.go:149`. These loaders decode the same
files and ignore removed keys without an error:

- `pkg/config/workspace_access.go:104`: push access
- `pkg/config/workspace_clones.go:56`: declared clones
- `pkg/config/flat_repositories.go:59`: flat repository list
- `pkg/reposynccli/config_loader.go`: workspace sync
- `internal/runtask/service.go:336`: `sourceBranch` reads `.gz-git.yaml`

## Reproduction

1. Add `makeTimeout: 1m` under `branch:` in a workspace `.gz-git.yaml`.
1. Run `gz-git workspace sync`; it succeeds without a warning.
1. Run `gz-git integrate check`; it fails with the removed-key error.

## Expected vs Actual

- Expected: one documented rule says which commands reject removed keys.
- Actual: the commands that read the file disagree, and no rule is
  written down.

## Decision needed

If every loader rejects the keys, commands unrelated to integration (sync,
push access, run-task) also fail on a stale key. That makes the error loud,
but one leftover key then blocks a whole workspace sync. If they stay
silent, the error appears only on `integrate`, where the key used to matter.
Choose one of these before changing code. The low-cost option is to keep the
guard at integrate only and document that boundary in `removed_keys.go`.

## Resolution

Resolved on 2026-10-21 by the readiness contract removal: every loader
rejects removed keys. The decision followed the principle that gz-git runs
no repository code and keeps no compatibility path, so a stale key fails
loudly wherever the file is read. `pkg/config/removed_keys.go` is the single
check: `ReadConfigFile` for loaders that open the file, `RejectRemovedKeys`
for loaders that read the bytes themselves. Push access, declared clones,
flat repositories, workspace sync, `clone --config`, the task runtime and
the global config now go through it.
`TestEveryConfigLoaderUsesTheRemovedKeyGuard` fails when a new loader reads
and decodes configuration without it. A leftover
key now blocks `workspace sync` too; the error names the key and the file to
delete it from. The impact list is ISSUE-89.
