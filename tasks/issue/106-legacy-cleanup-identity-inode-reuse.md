---
id: ISSUE-106
title: "Legacy cleanup ABA test intermittently accepts a re-created worktree on Linux"
type: bug
priority: P2
severity: medium
discovered-in: "0.13.0 release CI"
discovered-at: 2026-10-23
created: 2026-10-23
---

## Summary

`TestLegacyCleanupSecurityRejectsGitDirABASubstitutionBeforeDeletion`
(`pkg/integrate/legacy_cleanup_security_test.go`) intermittently fails on
Linux: cleanup accepts a task worktree that was removed and re-created at the
same path after the plan was written. The guard compares the planned
`LegacyWorktreeIdentity` with a fresh capture by `reflect.DeepEqual`
(`pkg/integrate/legacy_cleanup_apply.go`); the struct holds the git directory
path plus the `(device, inode)` of the worktree directory, its git directory
and its `.git` file (`legacyPhysicalIdentity`,
`pkg/integrate/legacy_cleanup_safety.go`).

## Reproduction

1. The test removes the task worktree with `--force`, re-adds it for the same
   branch, then applies the plan.
1. Observed on GitHub Actions `ubuntu` runners: it fails with
   `gitdir substitution unexpectedly authorized cleanup` in 2 of the 14 master
   CI runs up to 2026-10-23 (runs 39732156326 and 39881876319). It passed in a
   local Linux container run and passes on macOS.

## Expected vs Actual

- Expected: any substitution of the planned worktree is rejected with
  `physical identity changed` before deletion.
- Actual: sometimes the substitution is accepted.

## Notes

Unconfirmed cause: the file system hands the re-created resources back the
inode numbers just freed, so every compared `(device, inode)` matches.
`(device, inode)` is not a stable identity across delete and re-create. Confirm
by logging both identities on a failing run before choosing a fix, such as a
nonce written into the planned resource and checked before deletion. The
re-created worktree also matches the plan on path, branch, OID and clean
state, which bounds the impact.
