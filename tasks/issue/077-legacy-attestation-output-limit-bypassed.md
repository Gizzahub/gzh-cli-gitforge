---
id: ISSUE-77
title: "Legacy runtime attestation output limit is never enforced"
type: bug
priority: P3
severity: low
discovered-in: "ISSUE-70 independent review"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`boundedLegacyBuffer` in `pkg/integrate/legacy_runtime_guard.go` embeds
`bytes.Buffer` and is the verifier's `cmd.Stdout` and `cmd.Stderr`. It has
the ISSUE-70 defect: `os/exec` copies through the promoted `ReadFrom`, so
the capping `Write` and its mutex are skipped. The 64 KiB
`legacyAttestationMaxOutput` cap does nothing, and
`CE runtime attestation exceeds output limit` is never reported.
`strictSingleJSON` has no size bound either, so a valid attestation with
any amount of whitespace padding or stderr is accepted. The binding
checks on its content (plan digest, attempt, guard, PIDs) still apply,
so the refusal decision is unchanged; memory use is unbounded.

## Reproduction

1. `io.Copy` 200 KiB into a `boundedLegacyBuffer` with the production
   limit.
1. The buffer holds all 204800 bytes and records no overflow.

## Expected vs Actual

- Expected: capture stops at `legacyAttestationMaxOutput` and the
  attestation fails with the output limit error.
- Actual: capture is unbounded and nothing rejects the size.

## Scope

- Stop the promoted `ReadFrom` from bypassing the cap, as ISSUE-70 did
  for `limitedBuffer`.
- Its `Write` returns `io.ErrShortWrite` on overflow, which ends the copy
  early and makes `cmd.Run` fail. Decide whether the overflow should be
  reported as the limit error rather than as a failed attestation, and
  assert the reason with a verifier that writes past the cap.
