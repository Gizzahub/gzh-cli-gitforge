---
id: ISSUE-93
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

Update 2026-10-21: b8b4cccc removed the readiness gate and `--lock-wait`, but
not the lock. `integrate check` holds it (`pkg/integrate/check.go:118`
`holdTargetLock`) and `integrate run` holds it from fetch to reclaim
(`pkg/integrate/run.go:138` `withTargetLockSlot`), and the wait is now
unbounded: it ends only when the holder releases it or the command is
interrupted. The gate that made the wait long now runs in CE `run-finish`
before `integrate run`, so the wait is shorter per holder but still blocks
on any concurrent run of the same target.

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

- The run `refresh-under-target-lock`
  (`dev/claude/mst/feat/refresh-under-target-lock`, commit 14834a5e "refresh a
  stale task branch under the target lock") extends the lock this issue
  removes. It was ABORTED on 2026-10-21 with its branch and worktree kept; do
  not land it.
- [ISSUE-61](061-concurrent-landing-reverify.md) hands its remaining gitforge
  item here: when the exact-lease push is rejected without `--expect-target`,
  the refusal must name the stale target like the revalidation does.
- ISSUE-55 (resource slot release window) is obsolete since b8b4cccc.
  ISSUE-057 and ISSUE-069 (target-lock notices in the provenance stream)
  assume the lock exists. Revisit them once it is removed.

## Resolution

Resolved on 2026-10-22 by `feat(integrate): remove the target lock; detect and report, never wait` and `fix(integrate): name the Windows lock holder and retry flock on EINTR`.

- `integrate run` takes no target lock. Concurrent runs on one target are not ordered: the exact-lease push lets one
  land, and every other run refuses at once, naming the target, the commit it checked, the current commit and the
  rebase to do. The refusal without `--expect-target` names the stale target too (ISSUE-61's gitforge item).
- The remaining locks only protect gz-git's own on-disk state (the reclaim of one task's worktree and branches, and the
  receipt write, which also covers the intent registration marker). A held lock refuses with
  `integration resource is busy`, names the holder and exits 2; nothing waits. Unix retries `flock` on EINTR only.
  Windows locks a byte past the holder record so another process can read who holds it.
- The waiting tests are gone. New tests cover two concurrent runs (the second fails fast with an actionable reason) and
  `validateOwnedCheckPlan` refusing a changed source, target, declaration file, integration branch or controller.
- The design doc and the integrate, prepare-profile and troubleshooting docs state the reporting-only contract.
- Not verified: no native Windows run of the lock change (`GOOS=windows go vet` only).
- Follow-up: the parity goldens `doctor-active.json` and `discard-no-fetch.json` still embed the old integrate help
  ("waits for it until the lock is free"). They are recorded through the installed ce and gz-git, so they can be
  re-recorded only after a release with this change is installed; `make test-parity` is opt-in and outside `make check`.
- ISSUE-057 and ISSUE-069 assumed the lock; ISSUE-069 is closed, and CE consumers of the retired `target_lock_wait`
  phase and `LockNotice` fields should be checked when ce-agent-kit adopts this release.
