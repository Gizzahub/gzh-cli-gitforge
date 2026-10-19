---
id: ISSUE-71
title: "Run and cleanup resolve the caller identity from global Git differently"
type: bug
priority: P3
severity: low
discovered-in: "review of the ISSUE-60 change"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

Two functions read the same `ce.workActor` and `ce.workHost` keys from
global Git and disagree on two inputs.

- Several values: `pkg/integrate/receipt.go` `ResolveCallerOwner` uses
  `git config --get`, so the last value wins. `internal/runtask/identity.go`
  `gitIdentity` uses `--get-all` and refuses more than one value. A
  caller with two values passes cleanup's owner check but is refused by
  the run lifecycle.
- An unreadable global config: Git exits 1 and warns on stderr.
  `gitIdentity` treats every exit 1 as unset, so the caller is told to
  configure an identity that is already configured. ISSUE-60 fixed this
  in `ResolveCallerOwner` only.

## Reproduction

1. Point `GIT_CONFIG_GLOBAL` at a file with two `ce.workActor` values,
   with `GIT_WORK_ACTOR` and `GZ_GIT_TASK_ACTOR` unset.
1. `ResolveCallerOwner` returns the second value; `gitIdentity` refuses
   with "must have exactly one value".
1. Point `GIT_CONFIG_GLOBAL` at a directory instead. `ResolveCallerOwner`
   reports Git's read warning; `gitIdentity` returns an empty value and
   the caller is told to configure it.

## Expected vs Actual

- Expected: one rule for both paths. Refuse ambiguous values, and report
  Git's warning when the config cannot be read.
- Actual: the two paths resolve differently, and `runtask` hides the
  read failure.

## Scope

- Make `ResolveCallerOwner` and `gitIdentity` share one resolution rule,
  or make one call the other. Add tests for a key with two values and
  for an unreadable `GIT_CONFIG_GLOBAL`, in both packages.
