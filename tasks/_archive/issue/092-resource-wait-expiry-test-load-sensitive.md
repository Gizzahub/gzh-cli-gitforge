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

The CI failure log names the admission step directly (see Evidence); no
instrumented reproduction exists yet.

## Evidence

- `cmd/gz-git/cmd/integrate_resource_wait_unix_test.go:83`: `missing %q in report` (check subtest), and `:110`: `diagnostics: ...` (run JSON subtest).
  Both assert that the holder description
  `/fixture-holder/integration branch --target not ready` appears.
- `pkg/integrate/admission.go`: `acquireAdmission` and `admissionWaitExpired`
  spend the same wait budget on FIFO admission before the OS lock holder is
  described.
- CI run 39524694645 failed on master `0e883d00` (Quality gate) at `:110` and
  passed on rerun. Its log reads
  `acquire FIFO resource admission ...; host measurement lock wait expired after 50ms; admission ticket canceled`.
- The failure needs host load, not the whole package:
  `go test -run TestIntegrateResourceWaitExpiryExitsTwo -count=20` passed 20/20
  on an idle host, but on darwin with go1.27.3 at load average 7-10 it failed
  at `:110` with the same admission-expiry message in about 2 of 80
  iterations.

## Reproduction

1. On a loaded host (local reproductions were at load average 7-10) or hosted
   CI, run either the single-test loop above or the whole package,
   `go test ./cmd/gz-git/cmd/... -count=1`.
1. Intermittently the test fails at `:83` or `:110`; on an idle host neither
   form reproduces it.

## Expected vs Actual

- Expected: the expiry report always names the holder, whatever the load.
- Actual: with the budget exhausted in admission, the report omits the holder
  and the test fails intermittently.

## Proposal

Decide first whether the product contract is "the expiry report always names
the holder". If yes, make the production path describe the holder even when
admission consumed the budget. If no, make the test deterministic by
injecting a clock or a wait budget the test controls instead of a real 50ms.
Verify under sustained host load (for example the single-test loop with a
high `-count` while a parallel build or `stress` runs), since an idle host
passes either way.

## Resolution

Obsolete on 2026-10-21. b8b4cccc ("remove the readiness contract and gate
declarations") deleted the code under test and the test itself:
`pkg/integrate/admission.go`, the host measurement lock, the `--lock-wait`
budget and `cmd/gz-git/cmd/integrate_resource_wait_unix_test.go`. No
remaining test spends a real wait budget on FIFO admission.
