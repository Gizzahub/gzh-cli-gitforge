---
id: ISSUE-90
title: "`workspace status` places a nested config workspace's relative repositories under the root config"
type: bug
priority: P3
severity: low
discovered-in: "ISSUE-86 independent review"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

`reposynccli.FileSpecLoader.loadConfigWorkspace`
(`pkg/reposynccli/config_loader.go`) returns a `type: config` workspace's
`repositories:` entries as the nested config lists them. A relative path
such as `lib` stays `lib`; it is not rebased on the nested config's
directory. `workspace status` and `quicksync --check` load through
`configDirSpecLoader` (`pkg/workspacecli/status_loader.go`), which
resolves every relative path against the top-level config directory, so
the nested `lib` is checked at `<devbox>/lib`. `workspace sync` places it
at `<devbox>/<child>/lib` (ISSUE-86).

Before ISSUE-86 the status side resolved it against the working directory,
which was also wrong, so this is not a regression.

## Reproduction

Derived from the code during the ISSUE-86 review; not run end to end. A
devbox with `workspaces: {child: {type: config, path: child}}` and
`child/.gz-git.yaml` listing `repositories: [{name: lib, url: <url>}]`.
After `gz-git workspace sync`, `lib` is at `child/lib`. `gz-git workspace
status` reports `lib` missing, because it looks at `<devbox>/lib`.

## Fix note

Rebase the nested entries on `wsPath` inside `loadConfigWorkspace`, where
the nested config's directory is known; the wrapper cannot tell nested
entries from top-level ones. That changes the reposynccli `sync` family's
status too, so check what its own sync does with nested config
workspaces first.

## Expected vs Actual

- Expected: status checks a nested config's relative repository where
  sync placed it, under the nested config's directory.
- Actual: status checks it under the top-level config's directory.
