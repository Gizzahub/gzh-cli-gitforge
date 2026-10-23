---
id: ISSUE-70
title: "Readiness runner output limit is never enforced"
type: bug
priority: P2
severity: medium
discovered-in: "ISSUE-68 implementation and review"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`limitedBuffer` in `pkg/integrate/readiness.go` embeds `bytes.Buffer` and
caps only its own `Write`. `os/exec` copies a child's output through the
`ReadFrom` method promoted from the embedded buffer, so the limiting
`Write` is never called. The 1 MiB `readinessMaxIO` cap on runner stdout
and stderr does nothing, in the plan stage and in the execution stage.
A runner that writes gigabytes is held in gz-git's memory in full, and
`runner output exceeded limit` can never be reported. Plan and result
parsing still reject anything over 1 MiB, so verdicts stay fail-closed;
only memory use is unbounded.

## Reproduction

1. Declare a readiness runner that writes 3 MiB to stdout and to stderr.
1. Run the readiness stage.
1. Both streams are captured in full and no limit error is returned.
   Calling `limitedBuffer.ReadFrom` with 10 bytes and a limit of 4 also
   keeps all 10 bytes and leaves `exceeded` false.

## Expected vs Actual

- Expected: capture stops at `readinessMaxIO` bytes per stream and the
  stage fails with `runner output exceeded limit`.
- Actual: capture is unbounded, and the parser's size check is the only
  bound.

## Scope

- Stop the promoted `ReadFrom` from bypassing the limit, for example
  with a named buffer field instead of embedding, or a `ReadFrom` that
  goes through `Write`. Add a test that drives real child output past the
  limit in both stages. Update the over-limit case in
  `readiness_plan_failclosed_test.go`, which pins the parser's reason
  today.

## Resolution

Resolved on 2026-10-20.

- `limitedBuffer` holds its `bytes.Buffer` as a named field and exposes
  only `Write`, `Bytes` and `String`, so no `ReadFrom` is promoted and
  `io.Copy` in `os/exec` goes through the limiting `Write`. `Write` still
  never fails: an overflowing runner keeps draining the pipe and ends by
  itself or at its timeout, instead of blocking on a full pipe.
- `TestExecuteReadiness_NonzeroAndOversizeFailUnavailable` moved to
  `readiness_output_limit_test.go` and now asserts the reason. Its
  oversize runners print a valid ready result and then 3 MiB of padding
  on stdout or stderr, so only the capture limit can reject them; the
  stderr case was accepted as ready before the fix. It also asserts the
  captured stderr stays within `readinessMaxIO`.
- `TestLimitedBufferCopyHonorsLimit` drives the buffer through `io.Copy`.
- The plan-stage over-limit case in `readiness_plan_failclosed_test.go`
  now expects `resource plan: runner output exceeded limit`.
- Mutation evidence, reverted before commit: restoring the embedded
  buffer fails all three tests.
