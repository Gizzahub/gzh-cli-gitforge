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
[ISSUE-93](../_archive/issue/093-integrate-target-lock-removal.md) removes the target lock
and rewrites several of those tests.

Whether `pkg/integrate` is still the critical path of the race step has
not been re-measured since the deletions. Re-measure after ISSUE-93 lands
and decide from that measurement whether this card still needs work.

## Re-measurement (2026-10-22)

The make gate has never run `-race`: `git log -S'-race' -- .make Makefile`
finds only the commit that added the build files, and `-race` is only in
the `cover`, `cover-html` and `test-race` targets, which the gate does not
call. The race figures in ISSUE-62's Measurements came from a manual
`go test -race` command. `verify-full` runs `test-unit-quality`, which
passes `-parallel=1` (`VERIFY_GO_TEST_PARALLEL`, `.make/test.mk`), so
`t.Parallel()` does not shorten the gate today.

Measured on an export of master `86f02984` (macOS, 18 CPUs) with the
`test-unit-quality` flags plus `-count=1` and `-json`:

| `-parallel` | Wall-clock | `pkg/integrate` | Next package          |
| ----------- | ---------- | --------------- | --------------------- |
| 1 (gate)    | 263 s      | 243 s           | `pkg/repository` 48 s |
| 4           | 219 s      | 202 s           | `pkg/repository` 49 s |

- `pkg/integrate` is still the critical path: five times the next package
  in the gate run, four times with `-parallel=4`.
- It has 229 top-level tests; 155 call `t.Parallel()` (one of them skips)
  and 74 do not. The 74 sequential tests add up to 153 s in the gate run
  and 168 s with `-parallel=4`, so no `-parallel` value takes the package
  much below 150 s. The slowest sequential tests are cleanup tests:
  `TestLegacyCleanupCERuntimeGuard` 12 s,
  `TestCleanupOnlyRetryLeaseGeneration` 11 s and
  `TestCleanupRereadGuardAfterFirstRead` 9 s (gate run).
- The export has no `.git`, so seven top-level tests in `pkg/repository`,
  `benchmarks` and `tests/cli` that need the repository root to be a Git
  checkout failed in both runs. They do not touch `pkg/integrate`.

The card still needs work. Raising the gate's `-parallel` value is a
separate, smaller choice: it is worth about 40 s, and the comment next to
`VERIFY_GO_TEST_PARALLEL` keeps it at 1 on purpose.

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
