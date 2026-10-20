---
id: ISSUE-86
title: "a relative `repositories:` path is placed relative to the working directory"
type: bug
priority: P2
severity: medium
discovered-in: "ISSUE-85 independent review"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

A flat config's `repositories:` entry keeps its `path` as written, and the
entry's name when `path` is omitted (`pkg/workspacecli/config_loader.go`,
`TargetPath: path`). Nothing rebases it on the directory of the config that
lists it, and the executor clones to and updates `Repo.TargetPath` as given
(`pkg/reposync/executor_git.go`). A relative path is therefore resolved
against the process working directory. Workspace paths, by contrast, are
resolved against the config directory (`resolveWorkspacePath`).

## Reproduction

Derived from the code; not run end to end. A root devbox with
`workspaces: {child: {path: child, sync: {recursive: true}}}`, and
`child/.gz-git.yaml` with `repositories: [{name: lib, url: <url>}]`. Run
`gz-git workspace sync` from the root: the child's planned action has
`TargetPath: "lib"`, so `lib` is cloned into the root, not into `child/`.
The same holds for a root config synced from another directory, and for
the `--recursive` sync after execution (`runChildSync`,
`pkg/workspacecli/sync_recursive.go`), which builds the child's flat
actions the same way.

## Fix note

The per-repository guard (`keepActionsOffEnclosingConfigDirs`,
`pkg/workspacecli/self_sync_guard.go`) resolves a relative target against
the working directory on purpose, as the executor does. Rebase the paths
once in the loader, not in the executor alone, or the guard and the
executor will disagree about where a path points.

## Expected vs Actual

- Expected: a relative `repositories:` path is resolved against the
  directory of the config that lists it, as workspace paths are.
- Actual: it is resolved against the process working directory.

## Resolution

Resolved on 2026-10-21.

- `createActionsFromFlatConfig` (`pkg/workspacecli/sync_command.go`) now
  resolves each repository's `TargetPath` with `resolveWorkspacePath`
  against the directory of the config that lists it, as workspace paths
  are resolved. Absolute and `~/` paths keep their meaning. It is called
  for the root config, for a child config while planning, and for the
  `--recursive` child sync (`runChildSync`), so all three are covered.
- Place of the fix: the fix note asks for the loader. The rebase is done
  where the flat plan becomes actions instead, because every consumer that
  plans or executes reads those actions afterwards: the devbox filter in
  the same function, `keepActionsOffEnclosingConfigDirs`, the executor and
  `recordReadOnlyWorkspaceAccess`. They therefore agree on one absolute
  target. `recordReadOnlyWorkspaceAccess` already joined a relative path
  with the config directory, so before this change it recorded a
  different directory than the executor used. The loader itself is
  unchanged: `config_loader_test.go` and `flat_loader_parity_test.go` pin
  its relative `TargetPath`, and `validate` reads it without placing
  anything.
- Behaviour change: planned flat repositories now show an absolute path in
  plan and dry-run output, and a flat entry and a forge entry for the same
  directory are deduplicated, since both keys are now absolute.
- Out of scope: `status` and `quicksync` load through
  `reposynccli.FileSpecLoader`, a separate loader in another package; they
  were not changed.
- Tests (`pkg/workspacecli/flat_repository_path_test.go`):
  `TestFlatConfigResolvesRelativePathsAgainstConfigDir` checks that `lib`
  and `deps/nested` are placed under the config directory and an absolute
  path is kept. `TestConfigPlannerResolvesChildRepositoriesAgainstChildDir`
  plans a child whose `lib` has no path and whose `up` is at `..`: `lib`
  is placed in the child, and `up`, the devbox seen from the child, is
  dropped by the enclosing-config guard.
- Mutation: with the resolution line removed, both tests fail; before the
  fix `up` resolved against the test's working directory and was planned.
