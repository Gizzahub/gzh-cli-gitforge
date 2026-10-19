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
2. Make `ResolveCallerOwner` return a fixed owner when nothing is
   configured; no test fails.

## Expected vs Actual

- Expected: each guard failure case asserts a guard-specific error, and
  a hermetic test (identity env cleared, `GIT_CONFIG_GLOBAL` empty)
  asserts cleanup refuses an unconfigured caller and creates no
  evidence root.
- Actual: both refusals are only implied.
