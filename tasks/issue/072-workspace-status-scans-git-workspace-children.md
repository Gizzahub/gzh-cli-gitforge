---
id: ISSUE-72
title: "workspace status scans a type: git workspace's children instead of the repository itself"
type: bug
priority: P3
severity: medium
discovered-in: "foundation-devbox gz-git workspace adoption"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

A workspace config whose `workspaces` entries each name one repository
(`type: git`, `path` = the checkout itself, `url`) is planned correctly by
`workspace sync`: `planGitWorkspaces` creates one update action aimed at the
workspace path (`pkg/workspacecli/sync_command.go:1177`, :1241-1255). The same
config fails `workspace status`.

`status` loads the config through `reposynccli.FileSpecLoader`
(`pkg/workspacecli/status_command.go:43-45`). Its default case calls
`scanGitRepos(wsPath)` (`pkg/reposynccli/config_loader.go:589-590`), which
collects only direct subdirectories that contain `.git` (:777-822). A path that
is itself a repository yields nothing, every workspace is dropped, and the
empty plan returns `no git repositories found in workspaces` (:611).

The loader also skips every per-workspace error with `continue`: the forge
(:575-580), config (:583-587) and git (:591-594) branches alike. A missing or
unreadable path is therefore indistinguishable from an empty one.

## Reproduction

On gz-git 0.10.5 (source ed395f5d; the files are unchanged at master
a312815d), with foundation-devbox's `.gz-git.yaml` (five member checkouts
under `~/mywork/...`):

```
$ gz-git workspace status
Using config: /Users/archmagece/mydevbox/foundation-devbox/.gz-git.yaml
failed to load config: no git repositories found in workspaces
```

Exit 1. task-manager-devbox's config has the same shape and fails the same
way. `gz-git workspace sync --dry-run` on the same config plans all five
repositories.

## Expected vs Actual

- Expected: `workspace status` reports the five repositories that
  `workspace sync` plans from the same config.
- Actual: it exits 1 with `no git repositories found in workspaces`.

## Follow-up scope

Make the status loader agree with sync's interpretation of `type: git`
workspaces: a `path` that is a Git repository, with its declared `url`, is one
repository. Keep the subdirectory scan for parent-directory workspaces only if
that mode is still intended, and say which in the config docs. Report an
unreadable or missing workspace path instead of skipping it. Cover both shapes
in `pkg/reposynccli/config_loader_test.go` (existing workspace-scan tests at
:167, :191), or with one config loaded through both `FileSpecLoader` and
`planGitWorkspaces`.

Registration only; no remediation is included here.

## Resolution

- `loadGitWorkspace` (`pkg/reposynccli/config_loader_git_workspace.go`)
  reads a `type: git` workspace as sync plans it: with a declared `url`, a
  path holding `.git` (a directory, or a file in a linked worktree) is one
  repository named after the workspace. `workspaceEntry` now carries `url`.
- A declared path that does not exist stays in the plan when the workspace
  has a `url`, so status reports it unreachable. When a path cannot be
  read, the loader prints `Warning: workspace '<name>' skipped: <cause>`;
  the `config` branch does the same instead of skipping silently.
- The parent-directory scan stays for a workspace without a `url`, a path
  that exists but is not a repository, and the config directory itself,
  which sync refuses to plan: a devbox whose `path: .` workspace lists its
  member clones keeps that listing even though the devbox is a checkout.
  The config guide says which shape each command reads.
- `scanGitRepos` and `getGitRemoteURLAt` moved unchanged into the new file,
  so `config_loader.go` shrinks rather than grows.
- Tests: `TestGitWorkspacePathIsOneRepository` (clone and linked worktree),
  `TestGitWorkspaceMissingPath` and
  `TestGitWorkspaceKeepsParentDirectoryScan`.
- Mutation evidence, reverted before commit: the master scan, no
  config-directory guard, a missing path always refused, and no
  parent-directory scan each fail the tests.
- Independent review (PASS WITH WARNINGS) follow-up:
  - The first cut also read a `url`-less path holding `.git` as one
    repository, named from its origin, which changed status for configs
    that point a `url`-less workspace at a devbox to list its members. Sync
    plans no `url`-less workspace (`config.GetGitWorkspaces`), and this
    card's scope names the declared `url`, so only a workspace with a `url`
    now becomes one repository; the rest keep the master scan.
  - The config-directory guard compared cleaned strings, so a symlink to the
    devbox or a relative `-c` with an absolute `path` slipped past it;
    `sameDir` compares the directories with `os.SameFile`.
  - A stat failure no longer wraps a path error that already names the
    path.
  - The config guide no longer says sync reads every `type: git` path as
    one repository, and names the omitted-`path` difference.
  - New tests: `TestGitWorkspaceUnreadablePathIsReported` and
    `TestGitWorkspaceWithoutURLKeepsScan`; the parent-scan test adds a
    symlinked config directory. Seven mutations (the four above, plus
    collapsing without a url, the string guard, and swallowing a read
    error) each fail the tests.
- End to end, on a config naming the gzh-cli-gitforge and gzh-cli-core
  checkouts as `type: git` workspaces, the prior binary fails with
  `no git repositories found in workspaces` and the fixed one reports
  `2 ok` with `--skip-fetch`.
