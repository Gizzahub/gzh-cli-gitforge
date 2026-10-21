---
id: ISSUE-83
title: "Most pkg/integrate tests stay sequential because production code reads process-wide state"
type: bug
priority: P3
severity: low
discovered-in: "ISSUE-62 parallelisation"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

ISSUE-62 made 274 of the 504 top-level tests in `pkg/integrate` run in
parallel (271 of 500 on Linux and macOS, which skip four Windows-only
tests), which cut the package's `-race` time by about a quarter. The
other 229 stay sequential, and with them the package remains the
critical path of `verify-full`. They stay sequential for reasons the
tests cannot fix on their own:

- production code reads the environment directly (`XDG_STATE_HOME`,
  `PATH`, Git and CE variables), so the tests set it with `t.Setenv`,
  which Go forbids in a parallel test;
- tests swap package globals (`hostLockPollInterval`,
  `hostLockNoticeInterval`, `defaultHostLockWait`, `infraOpsDependencies`);
- the host-lock, measurement-lock, admission and resource-fence tests
  share the package-wide state directory that `TestMain` sets;
- tests that write a script and then execute it can fail with
  `text file busy` on Linux when a parallel test forks while the file is
  still open for writing. That cause is separate from the environment:
  committing the executables to the fixture repository, so Git checks
  them out, would remove it without the injection below.

## Update (2026-10-21)

ee40ef50 and b8b4cccc deleted the legacy make gate, readiness runners,
host slots, admission and the measurement lock, and their tests with them.
On master `fb5062fa`, `pkg/integrate` has 226 top-level tests; 149 run in
parallel and 77 stay sequential. The package globals listed above no
longer exist in production code. Of the 77, 25 call `t.Setenv` directly;
most of the rest are cleanup and reclaim tests (`cleanup_retry_test.go`
23, `reclaim_evidence_test.go` 13, `legacy_cleanup_security_test.go` 9)
that share the `TestMain` state directory or the target lock.
[ISSUE-93](093-integrate-target-lock-removal.md) removes the target lock
and rewrites several of those tests.

Whether `pkg/integrate` is still the critical path of the race step has
not been re-measured since the deletions. Re-measure after ISSUE-93 lands
and decide from that measurement whether this card still needs work.

## Reproduction

`grep -c 't.Parallel()' pkg/integrate/*_test.go` against the count of
top-level tests, and the ISSUE-62 commit message for the exclusion list.

## Expected vs Actual

- Expected: `pkg/integrate` is no longer the single critical path of the
  race step, while keeping coverage and `-race`.
- Actual: 77 tests still run one at a time (master `fb5062fa`); the
  effect on the race step is unmeasured since the gate removal.

## Scope

Choose one or both, as separate changes, after the re-measurement:

- inject the environment and state directory into the integrate entry
  points instead of reading process state, so the affected tests can
  run in parallel; or
- split the package's tests across processes (package split or test
  sharding in `verify-full`), which needs no production change.

`verify-full` is now a repository make target that CE `run-finish` runs;
the readiness contract it used to belong to was removed in b8b4cccc.
