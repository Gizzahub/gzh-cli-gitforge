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

- Decide between merge-candidate verification (gate the rebased result
  and land it if the target has not moved since), re-verifying only the
  impact set between the old and new target, or serialising landings
  through the existing host slot and queue. Owner decision.
- Name the stale target in the Run-phase refusal.
