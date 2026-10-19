---
id: ISSUE-69
title: "Hold releases with the run provenance line until CE skips it as a failure reason"
type: bug
priority: P1
severity: high
discovered-in: "review of the ISSUE-57/58 change"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

ISSUE-57 makes `integrate check|run|cleanup` and
`integrate readiness update review|auto|run` print
`integrate: run device=<label> gz-git=<version>` as their first stderr
line, plus one hint line when the device is undeclared or invalid.

ce-agent-kit `internal/usecase/taskruntime/finish_reason.go` picks the
recorded failure reason in this order: a stdout `FAIL ` line, a stderr
`FAIL ` line, then the first non-empty stderr line. A failed run with no
`FAIL ` line (a refusal, a lock timeout, a stale base) would therefore
record the provenance line as its reason instead of the actual cause.

## Reproduction

1. Build gz-git from a commit that contains the ISSUE-57 change.
1. Run `ce task run-finish` on a task whose integrate run fails without a
   `FAIL ` line, for example on a stale base.
1. The recorded finish reason reads `integrate: run device=...` rather
   than the refusal.

## Expected vs Actual

- Expected: the finish reason names the failure.
- Actual: it names the device and build.

## Scope

- The fix belongs to ce-agent-kit: skip the provenance and hint lines
  when choosing the reason (or prefer the last non-empty stderr line).
  It is registered and made in a ce-agent-kit session; record its
  issue ID here once registered.
- In this repository: do not bump `VERSION` (which publishes a release,
  ISSUE-56) while master contains the ISSUE-57 change and the CE fix is
  not installed. Devices that build gz-git from master with
  `make install` are affected as soon as they rebuild, release or not.
- Close this card when the CE fix is installed on the devices that run
  `ce task run-finish`, citing the CE issue and commit.
