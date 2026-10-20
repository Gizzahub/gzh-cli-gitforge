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

## Measurements

2026-10-20, master `769e2cf8`, macOS, 18 CPUs, no other gate running:
`GOWORK=off go test -mod=readonly -race -count=1 -json -timeout 40m ./...`
took about 9 minutes of wall-clock time (08:41 to 08:50).

| Package             | Elapsed (s) |
| ------------------- | ----------- |
| `pkg/integrate`     | 534         |
| `pkg/repository`    | 47          |
| `pkg/gitlab`        | 42          |
| `cmd/gz-git/cmd`    | 36          |
| `pkg/branch`        | 20          |
| every other package | 16 or less  |

- `pkg/integrate` alone is the critical path: the other packages finish
  in its shadow. Its slowest top-level tests take 10 to 22 s each
  (`TestRun_ReclaimRefusesNestedRepoThatIsNotProvablyDisposable` 22 s,
  `TestNDReadinessNativePreparation` 13 s, the cleanup lock and retry
  tests about 13 s).
- Idle, the race step takes about a quarter of the 37 minutes observed on
  2026-10-19, so contention from concurrent gates likely explains most of
  that run. Per-step timing of `verify-full` under load is still
  unrecorded.
- Two failures in that run came from the primary checkout, not from the
  code, and a gate worktree does not see them: Go files left in the
  ignored `tmp/`, and `tests/parity` reporting CE toolchain drift
  (installed build `61d1c0dd`, goldens recorded against `42363c6e`).
- Candidate cuts, in order of evidence: run `pkg/integrate` tests in
  parallel or split the package, then cap concurrent full gates. Both
  keep coverage and `-race`.
