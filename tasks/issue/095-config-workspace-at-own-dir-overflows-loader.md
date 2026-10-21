---
id: ISSUE-95
title: "a config workspace at its own config directory crashes the loader with a stack overflow"
type: bug
priority: P3
severity: medium
discovered-in: "ISSUE-88 independent review"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

`loadConfigRecursiveWithVisited` (`pkg/config/recursive.go`) loads each
`type: config` workspace with `LoadConfigRecursive`, which starts a new
`visited` map at depth 0. The cycle check and `MaxConfigDepth` therefore
never see the outer call, and a config workspace that points back at its
own config directory, or any cycle between config workspaces, recurses
until the process dies. No file is written; the CLI crashes instead of
reporting the cycle.

## Reproduction

Run on 2026-10-21 against be42e369 with a temporary test, since removed:
a root `.gz-git.yaml` of
`workspaces: {self: {type: config, path: .}}` loaded with
`config.LoadConfigRecursive(root, ".gz-git.yaml")` ends in
`runtime: goroutine stack exceeds 1000000000-byte limit` and
`fatal error: stack overflow`.

## Expected vs Actual

- Expected: the loader returns the existing
  `circular config reference detected` error, or the depth error.
- Actual: the process crashes with a stack overflow.
