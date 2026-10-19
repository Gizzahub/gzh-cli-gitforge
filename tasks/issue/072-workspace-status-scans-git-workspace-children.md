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
