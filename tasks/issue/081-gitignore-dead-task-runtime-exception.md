---
id: ISSUE-81
title: ".gitignore keeps a dead exception for .ce/task-runtime.yaml"
type: bug
priority: P3
severity: low
discovered-in: "gzh-cli-gitforge workspace defect registration"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`.gitignore` has two blocks for `.ce/` that contradict each other. On
origin/master (d0a34735):

- Lines 98-104 come from fddf29df (2026-10-01), which kept the older `.ce/`
  at line 101 (ad8b9684) and wrapped it. The comment says
  `.ce/task-runtime.yaml` is a repository contract and is tracked. The block
  re-includes the file with `!/.ce/`, `/.ce/*` and `!/.ce/task-runtime.yaml`.
- Lines 105-107 come from 929d0d92 (2026-10-08, "stop tracking
  machine-local task runtime"). That commit untracked the file because it held
  machine-local absolute paths, then added a second bare `.ce/`.

The later `.ce/` overrides the exception, so the first block's comment and
re-include rules are dead.

## Reproduction

```
$ git ls-files .ce
$ git check-ignore -v .ce/task-runtime.yaml
.gitignore:107:.ce/	.ce/task-runtime.yaml
```

The only declaration is an untracked local copy (schema-version 2, absolute
`worktree-roots` for mbp and mst under `~/worktrees/gzh-cli/gzh-cli-gitforge`).
`ce task run-start` on this host placed the worktree under
`~/myworktree/gzh-cli/gzh-cli-gitforge/` instead.

## Expected vs Actual

- Expected: `.gitignore` states one policy for the runtime declaration, and
  its comment matches what git does.
- Actual: the comment says the file is tracked, but git ignores it. A reader
  who trusts the comment, or deletes lines 105-107 as a duplicate, changes the
  policy without meaning to.

## Follow-up scope

Decide whether the declaration is tracked, then make `.gitignore` say only
that.

- Keep it untracked: delete lines 98-104 and keep the bare `.ce/` with a
  comment pointing to 929d0d92's reason.
- Track it again: a rootless schema 5 declaration takes worktree placement
  from the global `~/.config/ce/task-runtime-defaults.yaml` ("Rootless schema
  5 repositories need only their catalog id-to-family map"), which removes the
  absolute-path reason 929d0d92 gave. That needs a declaration with no
  machine-local paths and a family mapping for this repository on every host:
  mst's `repository-layouts` does not list it, and mbp has no
  `repository-layouts`, only `default-family: misc`. Neither exists today.
  Then delete lines 105-107 and update the comment.

This is a design choice for the repository owner. Registration only; no
remediation is included here.
