# ISSUE: `workspace status` scans a `type: git` workspace's children instead of the repository itself

- Status: open; reproduced on gz-git 0.10.5 (source ed395f5d); code unchanged at master a312815d
- Owner: `pkg/reposynccli`

## Finding

A workspace config whose `workspaces` entries each name one repository
(`type: git`, `path` = the checkout itself, `url`) is accepted by
`gz-git workspace validate` and planned correctly by `workspace sync --dry-run`
(`pkg/workspacecli/sync_command.go:1177`, "Planning git workspace ..."). The
same config fails `workspace status`:

```
Using config: /Users/archmagece/mydevbox/foundation-devbox/.gz-git.yaml
failed to load config: no git repositories found in workspaces
```

exit 1. foundation-devbox (five member checkouts under `~/mywork/...`) and
task-manager-devbox reproduce it.

The loader's default branch (`pkg/reposynccli/config_loader.go:590`) calls
`scanGitRepos(wsPath)`, which (`:778`) lists only the *subdirectories* of
`wsPath` that contain `.git`. A `wsPath` that is itself a repository yields no
entries, so every such workspace is dropped and the empty plan raises the error
at `:611`. The per-workspace `scanGitRepos` error is also ignored with
`continue` ("Workspace might not exist yet"), so a missing or unreadable path is
indistinguishable from an empty one.

## Follow-up scope

Make the status loader agree with sync's interpretation of `type: git`
workspaces: a `path` that is a Git repository (with its declared `url`) is one
repo; keep the subdirectory scan for parent-directory workspaces only if that
mode is still intended, and say which in the config docs. Report an unreadable
or missing workspace path instead of silently skipping it. Cover both shapes in
`flat_loader_parity_test.go`.

Registration only; no remediation is included here.
