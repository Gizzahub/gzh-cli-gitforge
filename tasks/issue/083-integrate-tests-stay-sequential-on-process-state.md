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

## Reproduction

`grep -c 't.Parallel()' pkg/integrate/*_test.go` against the count of
top-level tests, and the ISSUE-62 commit message for the exclusion list.

## Expected vs Actual

- Expected: `pkg/integrate` is no longer the single critical path of the
  race step, while keeping coverage and `-race`.
- Actual: 229 tests still run one at a time on Linux and macOS.

## Scope

Choose one or both, as separate changes:

- inject the environment, lock directory and timing settings into the
  integrate entry points instead of reading process state, so the
  affected tests can run in parallel; or
- split the package's tests across processes (package split or test
  sharding in `verify-full`), which needs no production change but
  changes the readiness contract.

The second option touches `verify-full`, which is readiness-contract
material; decide it before implementing.
