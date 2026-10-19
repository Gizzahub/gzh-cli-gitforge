---
id: ISSUE-62
title: "Full gate duration makes every stale-base retry cost an hour"
type: bug
priority: P2
severity: medium
discovered-in: "release-gate-ci run-finish: verify-full retry after rebase"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

`verify-full` is budgeted at 60 minutes (run-finish allows 75) and runs
`go test -mod=readonly -race -timeout 40m ./...` over the whole module.
Any change touching a shared input such as `.make/build.mk` or
`pkg/integrate` widens readiness to this gate (`verify-commit` exit 3), so
a one-line make or test fix pays the full cost, and each ISSUE-61 retry
pays it again. On 2026-10-19 the `release-gate-ci` retry spent over 37
minutes in the race test step alone. No per-step timing is recorded, so
the dominant packages are unknown.

## Reproduction

1. Change `.make/build.mk` on a task branch.
1. `VERIFY_BASE=origin/master make verify-commit` exits 3 (widened).
1. `ce task run-finish` runs `verify-full`; the race step dominates.

## Expected vs Actual

- Expected: the full gate's step and package durations are measured, and
  the slowest steps are cut (parallelism, splitting `-race` from the
  plain run, narrowing what widens to the full gate) so the gate fits
  well inside its budget.
- Actual: the gate takes most of its budget with no timing breakdown.

## Scope

- Record per-step and per-package durations first; choose cuts from that
  evidence. Do not lower coverage or drop `-race` without an owner
  decision.
