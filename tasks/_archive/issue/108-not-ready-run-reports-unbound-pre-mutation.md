---
id: ISSUE-108
title: "A not-ready run without an execution context reports an unbound pre-mutation outcome"
type: bug
priority: P3
severity: low
discovered-in: "refresh-under-target-lock implementation (ADR-0003)"
discovered-at: 2026-10-20
created: 2026-10-23
---

## Summary

`Run` (`pkg/integrate/run.go`) records a pre-mutation outcome whenever the
readiness check rejects the run (`err != nil && check != nil && !check.Ready`).
A run without a native execution context has no capture, so
`recordCurrentPreMutationOutcome` (`pkg/integrate/run.go`) passes a nil
`opts.preMutation` to `recordPreMutationOutcome`
(`pkg/integrate/pre_mutation_outcome.go`), which
returns `errPreMutationOutcomeUnbound`. The refusal then ends with
"; record pre-mutation outcome: pre-mutation outcome is not bound to the
current execution". It reads as a second failure, though nothing was meant
to be recorded.

Still present on master at 280e1c3d (2026-10-23), `pkg/integrate/run.go`
lines 146–154.

## Reproduction

1. Make a task branch whose base is behind the integration target.
1. Run `gz-git integrate run` without an execution context (the legacy
   path CE uses for `ce task run-finish`).
1. The stale-base refusal message carries the unbound pre-mutation suffix.

## Expected vs Actual

- Expected: a run that never captured pre-mutation evidence refuses with
  the readiness cause only.
- Actual: the refusal adds an unbound pre-mutation recording error.

## Resolution

Resolved on 2026-10-24 in the commit that archives this card. `run` records
a readiness-rejected outcome only when `opts.preMutation` exists, which is
exactly when an execution context was given, so a no-context refusal carries
only the readiness cause. `recordPreMutationOutcome` is unchanged and still
returns `errPreMutationOutcomeUnbound` for a capture whose candidate never
bound. Tests cover the no-context not-ready run and both unbound cases.

## Scope

- Skip the recording when no capture exists, and keep the error for a
  capture that exists but no longer matches the execution.
- Add a no-context not-ready case to the run tests.

## Provenance

First written as ISSUE-77 on the unlanded branch
`dev/claude/mst/feat/refresh-under-target-lock` (commit e12081b8). That run
was ABORTED on 2026-10-21 and is not to be landed (see
[ISSUE-93](093-integrate-target-lock-removal.md)), and ISSUE-77 is now another
card on master, so the finding is re-registered here.
