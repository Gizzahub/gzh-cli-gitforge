---
id: ISSUE-61
title: "A concurrent landing discards a finished full gate"
type: bug
priority: P1
severity: medium
discovered-in: "release-gate-ci run-finish: stale-base refusal after another session landed"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

Integration only accepts a branch whose base is the current target tip,
and it checks that twice: before readiness (`finishIntegrate`,
`internal/runtask/service_lifecycle.go:315`, names the stale base) and
after readiness (`revalidateCheckedRefsForExpectedTarget`,
`pkg/integrate/run.go:333`, "target branch changed during readiness;
re-run check"). Each refusal sends the session back to rebase and run the
whole readiness gate again, which in this repository is `verify-full`
(40-75 minutes). With several sessions landing on the same target, every
landing during that window invalidates every other session's finished
gate, so the retry count grows with concurrency and a session can retry
indefinitely. The post-readiness refusal also reaches CE as the generic
"integration did not complete", because the stale-base explanation is
computed only for the Check phase (`service_lifecycle.go:315`, `index == 0`).
ISSUE-55 covers a narrower window (target moving before slot
acquisition); this card covers the cost of any target move.

## Reproduction

1. Session A: `ce task run-finish` on a branch whose readiness widens to
   `verify-full`.
1. Session B lands a commit on the target while A's gate runs (or before
   A starts).
1. A is refused, rebases, and re-runs the full gate; on 2026-10-19
   `release-gate-ci` was refused after cf5c6acb and 9d88a2b6 landed and
   its retry ran the full gate again (over 37 minutes in `go test -race`).

## Expected vs Actual

- Expected: a target move that does not touch the branch's inputs costs a
  rebase and a gate over the changed scope, not a full re-run, and a
  queue or slot keeps one session from being refused repeatedly; the
  post-readiness refusal names the stale target like the pre-readiness
  one.
- Actual: every target move costs a full re-run, retries are unbounded
  under concurrency, and the post-readiness refusal is unnamed.

## Scope

- Name the stale target in the Run-phase refusal (gitforge-owned).
- Out of scope here: choosing between merge-candidate verification (gate
  the rebased result and land it if the target has not moved since),
  re-verifying only the impact set between the old and new target, or
  serialising landings through the existing host slot and queue. Any of
  them changes integration policy under ce-devenv ADR-0002, so the
  decision belongs to ce-devenv ISSUE-041
  (`tasks/issue/041-batch-integration-needs-decision.md`); gitforge
  implements it after that decision.

## Findings

- 2026-10-20: ISSUE-75's `run-finish` was refused five times in one day,
  four of them for a stale base after other sessions landed and one for a
  real format defect; each stale refusal cost a rebase and a full gate
  rerun. Landings of ISSUE-71, ISSUE-72 and ISSUE-75 then went to master
  by an operator push as a stopgap until landing gets faster; that is not
  the intended workflow.
- `revalidateCheckedRefsForExpectedTarget` (`pkg/integrate/run.go`) now
  names the target ref, its current commit and the commit readiness
  checked, instead of the unnamed wording quoted in the Summary. The same
  text reaches both places a moved target is reported: the Check pass's
  post-gate freshness item (`pkg/integrate/check.go`, the usual case,
  since `Run` re-runs the check after the gate) and the Run-pass refusal
  in the short window after that re-check. When the remote-tracking ref
  is gone and the local target was read instead, the message names the
  local ref. Test:
  `TestLegacyTargetMovementDoesNotBecomeExpectedTargetMismatch` compares
  the whole message, so swapped commits fail it. Independent review: PASS
  WITH WARNINGS; the ref fallback, the exact assertion and this wording
  are its follow-up.
- Still open: CE's `run-finish` reason classifies a stale base only for
  the Check pass, so CE still reports the generic "integration did not
  complete" after readiness (ce-agent-kit; gitforge's `internal/runtask`
  mirrors that reference and follows it), and the policy choice under
  ce-devenv ISSUE-041.
