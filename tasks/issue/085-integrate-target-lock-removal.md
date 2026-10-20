---
id: ISSUE-85
title: "integrate holds a target lock across the readiness gate; remove it"
type: design-defect
priority: P1
severity: high
discovered-in: "singlepageweb-devbox review run-finish"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`gz-git integrate run` takes an exclusive flock per target ref
(`pkg/integrate/targetlock.go` `acquireTargetLock`, file
`<git-common-dir>/gz-git-integrate-targets/<sha256(targetRef)>.lock`) and keeps
it from before the fetch through the readiness gate, the exact-lease push and
reclaim (`pkg/integrate/run.go:104-107`). A second integration of the same
target waits for the whole run in front of it, gate included, and prints
`integrate: waiting for target ownership common-dir=… destination=…` while it
does.

The lock was introduced in d3a9a0e8 (2026-10-12) and is specified in
`docs/design/integration-resource-contract.md`.

## Owner decision (2026-10-20)

The repository owner rejected integration locking for gz-git:

> rebase, merge 충돌은 미리 알려주거나 발생했을 때 알려줄 수 있지
> 그런데 뭘 락을 걸고 이런건 하지마.
> 그건 다른데서 판단해서 처리할일이다.
> 그리고 애초에 워크트리 합치는데 무슨락처럼 30분씩 대기할일이 없지 말이 안된다.

So gz-git's responsibility is limited to detecting and reporting rebase/merge
conflicts, either up front or when they occur. Serializing or queueing
integrations belongs to the caller/orchestrator, not to gz-git.

## Evidence

- singlepageweb-devbox (`/Users/archmagece/mydevbox/singlepageweb-devbox`,
  `destination=refs/heads/master`), 2026-10-20: a review run-finish repeated
  the target-ownership notice for about 30 minutes. Two other `run-finish`
  integrations of the same target held the lock while their `ce task gate`
  runs took about 31 minutes.
- The exact-lease push already rejects a target that moved after the gate, so
  the lock does not add a correctness guarantee for the push. It only turns a
  fast stale-target refusal into a long wait.

## Scope

- Remove the target-ownership lock from `integrate run` (fetch → gate → push
  → reclaim). Concurrent runs must not wait on each other. When the target
  moves, the run must refuse with the conflict/stale-target reason
  (rebase needed, or the conflicting paths) instead of waiting.
- Decide case by case what replaces the other `acquireTargetLock` callers:
  - `cleanup.go:114`
  - `legacy_cleanup_apply.go:107`
  - `task_resource_fence.go:42` (`task-resource:<branch>`)

  Keep only what protects gz-git's own on-disk state from corruption. Nothing
  may wait on another integration's gate.
- Update `docs/design/integration-resource-contract.md` and the integrate docs
  so they state the reporting-only contract.
- Remove the tests that pin waiting behavior, and add a test where two
  concurrent runs on one target never block: the second fails fast with an
  actionable reason.

## Related

- The ACTIVE run `refresh-under-target-lock`
  (`dev/claude/mst/feat/refresh-under-target-lock`, commit 14834a5e "refresh a
  stale task branch under the target lock") extends the lock this issue
  removes. Reconcile it with this decision before it lands.
- ISSUE-55 (resource slot release window), ISSUE-057 and ISSUE-069 (target-lock
  notices in the provenance stream) assume the lock exists. Revisit them once
  it is removed.
