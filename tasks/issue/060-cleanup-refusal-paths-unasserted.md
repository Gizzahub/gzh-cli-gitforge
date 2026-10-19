---
id: ISSUE-60
title: "Cleanup guard and identity refusal paths are unasserted"
type: bug
priority: P3
severity: low
discovered-in: "release-gate-ci review: test hermeticity fixes"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

Two refusal paths in `pkg/integrate` cleanup have no test that proves
the refusal comes from the intended check. The CE runtime guard failure
cases in `legacy_runtime_guard_test.go` (malformed, wrong_guard,
stale_pid, hash_changed) assert only `err != nil`; on CI runners they
had been failing on "primary worktree is dirty" before reaching the
guard, and nothing would notice a recurrence. The comment at
`cleanup_retry_test.go:22-24` claims cleanup refuses an unconfigured
caller, but the body covers only wrong actor and wrong host; the one
subtest that reached the no-identity path did so by accident and now
declares an identity.

## Reproduction

1. Make the legacy guard checker accept every attestation; the four
   failure cases still pass whenever any earlier check fails.
1. Make `ResolveCallerOwner` return a fixed owner when nothing is
   configured; no test fails.

## Expected vs Actual

- Expected: each guard failure case asserts a guard-specific error, and
  a hermetic test (identity env cleared, `GIT_CONFIG_GLOBAL` empty)
  asserts cleanup refuses an unconfigured caller and creates no
  evidence root.
- Actual: both refusals are only implied.

## Resolution

Resolved on 2026-10-20.

- `TestLegacyCleanupCERuntimeGuard` now asserts each failure case's own
  refusal: `parse CE runtime attestation` (malformed),
  `runtime attestation after remove step 0: parse CE runtime attestation`
  (fail_after_step), `binding was refused or changed` (wrong_guard),
  `process identity changed` (stale_pid) and `CE verifier digest changed`
  (hash_changed). Making the guard checker fail with "primary worktree
  is dirty" fails all five; before, the four pre-step cases passed.
  Naming the step makes removing the post-step attestation fail
  fail_after_step, which otherwise moved to step 1 unnoticed.
- New `TestCleanupOnlyRetryRefusesUnconfiguredCaller` clears
  `GZ_GIT_TASK_ACTOR`, `GIT_WORK_ACTOR` and `GIT_WORK_HOST` and points
  `GIT_CONFIG_GLOBAL` at an empty file. Without a prior run, cleanup
  refuses and no evidence root appears; after a crashed run, it refuses
  and the residuals stay intact. Returning a fixed owner for an empty
  identity fails both subtests. The comment on
  `TestCleanupOnlyRetryOwnerGuard` now points to it.
- The test found a defect: `ResolveCallerOwner` treated an unset global
  `ce.workActor` (`git config --get` exit 1) as a Git failure, so an
  unconfigured caller got `read global Git ce.workActor: git command failed ... (exit code 1)` and never the "caller identity is not
  configured: set ..." guidance. Exit 1 with empty stderr now reads as
  unset. Git also exits 1 for an unreadable global config, with a
  warning on stderr; that case and every other non-zero exit fail with
  the exit code and stderr (or the exec error when Git did not run).
  `TestResolveCallerOwnerReportsUnreadableGlobalConfig` pins it and fails
  when stderr is ignored. The same ambiguity in `internal/runtask`
  identity resolution, and its different multi-value rule, are ISSUE-71.
