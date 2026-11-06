---
id: ISSUE-113
title: "Complete the guarded task-sync prepare H1 wire contract"
type: bug
priority: P1
effort: L
exec-tier: strong
severity: medium
discovered-in: "Complete the guarded task-sync prepare H1 wire contract"
discovered-at: 2026-11-06
created: 2026-11-06
---

## Summary

The H1 `task-sync-prepare` provider boundary is only a guarded checkpoint,
not a usable command contract. Checkpoint `bd656c55dff84836f630b2e24f4120c757189ff6`
adds a hidden command and provisional decoding helpers, but leaves the
`task-sync-prepare-v1` capability disabled; the command exits before it reads
input. No caller can therefore rely on it to prepare a task, and the current
draft must not be published or integrated as an H1 implementation.

This repository owns the provider-side wire, strict parsing, time-bounded
request handling, and deterministic result contract. A future owner must
first agree the complete v1 wire with the caller's owner, then implement and
verify the agreed contract here. This issue intentionally records the
follow-up only; it does not authorize enabling the capability.

## Reproduction

1. Build the checkpoint branch at
   `bd656c55dff84836f630b2e24f4120c757189ff6`.
1. Run `gz-git task-sync-prepare` with any stdin payload.
1. Observe exit 2 with `task sync prepare wire is not accepted`; stdin is not
   decoded and no preparation result is produced.

## Expected vs Actual

- Expected: after the v1 contract is accepted, the command accepts exactly one
  bounded request, validates the complete envelope before processing, observes
  the caller-provided response deadline, and emits one deterministic,
  schema-valid result. It preserves a delivery failure or uncertain outcome
  when the peer cannot receive the complete result.
- Actual: the capability remains disabled. The checkpoint has already added
  timeout and strict-parsing support to the established apply wire; this issue
  does not reopen that apply-wire extension. The separate prepare-v1 command
  and local preparation adapter have no accepted end-to-end contract or
  interoperability evidence.

## Scope

- Specify the v1 request and result schemas, including framing, required and
  forbidden fields, duplicate and case-variant key handling, size/depth limits,
  error taxonomy, and compatibility/version negotiation.
- Define the response-timeout authority and cancellation behavior. Bound all
  reads and writes so an absent, slow, or malformed peer cannot leave a
  process blocked. A failed or partial physical write must never be represented
  as successful delivery; retain an explicit result-delivery failure or
  uncertain outcome for recovery.
- Keep prepare local-only: it may validate and calculate a result and may
  create and preserve its owner-bound private prepared-candidate ref at
  `refs/gz-git/tasksync/<handle>` with the agreed scratch lifecycle. It must
  not alter the original task, source, target, or remote refs, apply a
  candidate, publish it, or make an integration decision.
- Add black-box command and coprocess tests for valid requests; unknown,
  duplicate, trailing, oversized, and deeply nested JSON; malformed frames;
  deadline expiry; cancellation; output-write failure; and deterministic
  serialization.
- Add interoperability fixtures from the accepted caller contract and retain
  compatibility tests for every supported schema version.

## Completion criteria

- The caller and this repository have an accepted, versioned v1 contract with
  fixtures that both sides consume.
- The hidden capability is enabled only after the command satisfies the
  local-only and bounded-I/O invariants above.
- `VERIFY_BASE=origin/master make verify-commit` passes for the implementation,
  followed by the repository's required full integration gate on the exact
  candidate head.
- An independent strong review confirms that malformed peer input cannot cause
  an apply/publish action, an original-ref mutation, unbounded wait, ambiguous
  decode, or partial/output-loss delivery reported as success.

## Non-goals

- Enabling an unagreed provisional wire.
- Treating the checkpoint's tests as H1 acceptance evidence.
- Implementing apply, publish, original task/source/target/remote ref mutation,
  or integration behavior in this command. The owner-bound private
  prepared-candidate ref and its scratch lifecycle are in scope.
