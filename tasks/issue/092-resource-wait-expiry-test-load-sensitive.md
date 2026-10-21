---
id: ISSUE-92
title: "TestIntegrateResourceWaitExpiryExitsTwo fails under package-wide test load"
type: bug
priority: P3
severity: low
discovered-in: "CI run 39524694645 (Quality gate) on master 0e883d00"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

`TestIntegrateResourceWaitExpiryExitsTwo` is load-sensitive. The test uses a
50ms lock wait. Under package-wide test load, the FIFO admission step can
consume the whole 50ms budget before the host-lock holder is read, so the
expiry error lacks the holder description and the test fails.

This is a suspected cause from the failure shape; no instrumented reproduction
exists yet.

## Evidence

- `cmd/gz-git/cmd/integrate_resource_wait_unix_test.go:83`: `missing %q in report` (check subtest), and `:110`: `diagnostics: ...` (run JSON subtest).
  Both assert that the holder description
  `/fixture-holder/integration branch --target not ready` appears.
- `pkg/integrate/admission.go`: `acquireAdmission` and `admissionWaitExpired`
  spend the same wait budget on FIFO admission before the OS lock holder is
  described.
- CI run 39524694645 failed on master `0e883d00` (Quality gate) and passed on
  rerun.
- Locally `go test -run TestIntegrateResourceWaitExpiryExitsTwo -count=20`
  passes 20/20 under go1.27.3 and go1.26.10, so the failure needs load.
- The gz-git integrate diagnostic recorded `diagnostics 2 -> 3` on the first
  run-finish attempt.

## Reproduction

1. Run the whole `cmd/gz-git/cmd` package, for example
   `go test ./cmd/gz-git/cmd/... -count=1`, on a loaded machine or hosted CI.
1. Intermittently the test fails at `:83` or `:110`; the single-test loop
   above does not reproduce it.

## Expected vs Actual

- Expected: the expiry report always names the holder, whatever the load.
- Actual: with the budget exhausted in admission, the report omits the holder
  and the test fails intermittently.

## Proposal

Decide first whether the product contract is "the expiry report always names
the holder". If yes, make the production path describe the holder even when
admission consumed the budget. If no, make the test deterministic by
injecting a clock or a wait budget the test controls instead of a real 50ms.
Verify under `go test ./cmd/gz-git/cmd/... -count=N` with `-cpu` or parallel
load, not the single-test loop.
