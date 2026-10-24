---
id: ISSUE-112
title: "runtask parity port follow-ups: bare finish counts every ACTIVE run, provider children are not process-isolated, no missing remote-ref scenario"
type: bug
priority: P3
severity: low
discovered-in: "ISSUE-42 CE fd175fd2 parity port reviews"
discovered-at: 2026-10-24
created: 2026-10-24
---

## Summary

[ISSUE-42](../_archive/issue/42-runtask-port-follow-ce-f540f972.md) ported
the CE fd175fd2 run lifecycle to `internal/runtask`. Its independent reviews
left four items outside that card's scope. The declared divergences (bound
native completion, sibling leases) are in
`docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md` and are not repeated here.

### (a) Bare finish readiness counts every ACTIVE run

`service_lifecycle.go` answers bare `gz-git run finish` with exit 0 only when
the aggregate is READY and `ActiveCount == 1`. `ActiveCount` counts every
ACTIVE state, of any owner or purpose. With one ready delivery task and any
experiment execution, bare finish exits 1 although the selected task is
ready. It also exits 0 for a single delivery task that is not `FinishReady`.
The behaviour predates ISSUE-42; experiment and review purposes make it
reachable.

### (b) Integration children are not process-isolated

CE runs its provider in its own process group, so a terminal Ctrl-C reaches
neither the provider nor its children. The port integrates in-process and
holds SIGINT/SIGTERM only for its own process. The `git push` and gate
commands that `pkg/integrate` starts stay in the terminal's process group:
Ctrl-C during the gate is recorded as a failing check, and Ctrl-C during the
push leaves it unclear whether the push landed. The contract declares this
as a divergence; this card owns the decision to fix it.

### (c) No scenario for a missing remote-tracking ref

Run start and finish now base on `refs/remotes/origin/<integration>`. CE
refuses when that ref is missing; no parity scenario captures the refusal,
so the port's wording and exit code there are unverified.

### (d) Abort trusts any ABORTED receipt as terminal

Port Abort treats every ABORTED receipt as terminal. CE's
`validTerminalAbortReceipt` also checks that the receipt belongs to this
execution, and CE's status path blocks an ABORTED receipt that fails the
check. The port writes these receipts itself and has no settle or close
verbs, so this matters little today. The behaviour predates ISSUE-42.

## Reproduction

- (a) Start one delivery run and make it finish-ready, start an experiment
  run with `gz-git run start <task> --type feat --purpose experiment`, then
  run bare `gz-git run finish`. It exits 1.
- (b) Press Ctrl-C while `gz-git run finish <task>` runs the gate. The
  outcome is recorded as a failing check.
- (c) Delete `refs/remotes/origin/<integration>` in a parity fixture and run
  `run-start`. No golden captures the result.
- (d) Found by reading the code (CE `service_abort.go` against the port's
  `service_abort.go`). No test asserts it.

## Expected vs Actual

- Expected (a): bare finish answers from the selected task's `FinishReady`.
- Actual (a): it answers from the aggregate and the count of ACTIVE runs.
- Expected (b): an interrupt during run finish cannot kill the push or the
  gate, as in CE; or the divergence stays declared with a decision recorded.
- Actual (b): the children share the terminal's process group.
- Expected (c): a parity scenario pins the refusal.
- Actual (c): none exists.
- Expected (d): only this execution's ABORTED receipt is terminal, as in CE.
- Actual (d): any ABORTED receipt is.

## Scope

- In scope: the four items above, each decided or fixed separately.
- Out of scope: CE's context-provider cleanup recognition
  (`isContextPartialCleanupDiagnostic`); the port records no
  context-provider diagnostics.
