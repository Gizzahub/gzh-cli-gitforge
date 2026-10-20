---
id: ISSUE-82
title: "Interrupting integrate run orphans the readiness runner, its gate, and the readiness worktrees"
type: bug
priority: P2
severity: medium
discovered-in: "singlepageweb-devbox dva-rollout-final-record delivery"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`gz-git integrate run` registers no SIGINT/SIGTERM handler, yet it starts the
readiness runner in its own process group (`configureReadinessProcess`,
`pkg/integrate/readiness_process_unix.go:22-24`). When the operator stops a
measurement by signalling gz-git, as the CE contract instructs
(ce-agent-kit `docs/10-ce-cli/25-task-runtime-commands.md`: "측정을 꼭 멈춰야
하면 gz-git 프로세스에 직접 신호를 보낸다"), gz-git exits at once:

- the runner's group never receives `killReadinessProcess`, so the runner and
  the `ce task gate` it spawned keep running with no parent and keep the host
  busy with product builds and e2e;
- neither deferred cleanup in `runContractPair` runs. Skipping
  `mergeCleanupError` → `worktreeRemoveForce` (`pkg/integrate/readiness.go:249`,
  `:253`, function at `:312-321`) leaves the
  `gz-git-readiness-*/{source,target}` registrations in the target
  repository. Skipping `os.RemoveAll(root)` (`:225-233`) leaves the
  directories.

The runner already has a cancellation path: `cmd.Cancel` calls
`killReadinessProcess` (`readiness.go:375-378`). Nothing cancels its context
on a signal. gz-git has a helper for that, `withInterruptCancel`
(`cmd/gz-git/cmd/bulk_common.go:503`), which `clean`, `cleanup branch`,
`commit`, `diff`, `handoff start`, and `handoff end` use. Clone and watch
register their own handlers (`clone.go:99`, `bulk_common.go:458` for
`RunBulkWatch`, `watch.go:140`). `integrate run` uses none of them.

## Reproduction

Observed, not synthetic.

gz-git 0.10.5, singlepageweb-devbox, 2026-10-20 (times UTC+9):

1. `ce task run-finish dva-rollout-final-record --json` (pid 63233) ran
   `gz-git integrate run --target origin/master` (66056) → readiness
   `check.mjs` (8651, ppid 66056) → `ce task gate --dir …/gz-git-readiness-4237320632/source`
   (70916, ppid 8651), which was running a DateCove `make check && make e2e`
   binding.
1. SIGINT to 63233 did nothing visible: CE holds it by design and reported
   "ce received SIGINT during the integration run, did not relay it, and
   waited for gz-git to exit".
1. SIGINT to 66056: gz-git exited, and CE recorded `BLOCKED` with
   "post-operation evidence failed: verify task head is contained in source: exit 1".
1. Five seconds later 8651 and 70916 were still alive, and
   `git worktree list --porcelain` still listed
   `…/T/gz-git-readiness-4237320632/source` (HEAD 6ff29b5d) and `…/target`
   (HEAD b676c67e).
1. Manual recovery: `kill -TERM 70916 8651`, then
   `git worktree remove --force` on both paths and `rmdir` on the root.

## Expected vs Actual

- Expected: on SIGINT/SIGTERM, integrate run stops the readiness group through
  the existing `killReadinessProcess` path (TERM, grace, KILL), removes the
  readiness workspace it created, records the interruption in the receipt, and
  only then exits non-zero. A cleanup failure is reported with the retained path,
  as the existing `preserveRoot` branch already does.
- Actual: the runner group and its gate survive as orphans, and the readiness
  worktrees remain registered until removed by hand.

## Related

- [ISSUE-47](47-interrupted-baseline-worktree-recovery.md): the same class for
  the `check_make` baseline worktree. That issue asks for a recovery path for an
  already-orphaned worktree. This issue asks that interruption not orphan
  anything in the first place.
- ISSUE-74 (`074-readiness-cleanup-leaves-registration.md`, registered on a
  pending branch; not yet on master): readiness cleanup fails
  after a READY verdict without any signal. Same workspace, different trigger.
- singlepageweb-devbox `troubleshooting/2026-10-20/02-run-finish-interrupt-orphans.md`
  (consumer record of the manual recovery).

## Follow-up scope

Give `integrate run` a signal-aware context (for example
`withInterruptCancel`) so that readiness and make probes reach their existing
kill and cleanup paths. Add a Unix test that signals
a running integrate with a long readiness runner, then asserts that the group
and the readiness root are gone. Keep the push and reclaim phases'
interruption semantics explicit, since CE relies on gz-git finishing them.

Registration only; no remediation is included here.

## Resolution

Resolved on 2026-10-20.

- `integrate run` and `integrate check` wrap their command context in
  `withInterruptCancel`. SIGINT or SIGTERM cancels readiness and make
  probes, so the existing paths run: `cmd.Cancel` →
  `killReadinessProcess` (TERM, grace, KILL) stops the runner group,
  `mergeCleanupError` unregisters both readiness worktrees on a detached
  context, and `os.RemoveAll` (or the `preserveRoot` report) handles the
  root. `integrate check` exits 2 with `integrate check interrupted`
  instead of reporting the killed gate as `not ready`.
- `integrate.Run` has an explicit intent boundary just before the receipt
  (or, without an execution context, the push). Before it, a canceled
  context returns `ErrInterrupted` ("interrupted before the target push"),
  exit 2, and records no pre-mutation outcome: a killed gate is neither
  ready nor rejected, and the outcome's v1 phases stay unchanged. From the
  boundary on, `Run` continues on `context.WithoutCancel`, so the receipt,
  the push, the local fast-forward and reclaim finish. `withInterruptCancel`
  handles only the first signal; a second one terminates gz-git at once.
  The `integrate run` help text states both halves. Only cancellation is
  an interrupt: an expired caller deadline keeps its lock-wait or
  readiness classification and its pre-mutation outcome
  (`TestRunRecordsBoundOutcomeWhenTargetLockWaitExpires`).
- The baseline make probe (`baselineAgainstTarget`) removed its worktree
  with the caller's context, so a cancellation left the
  `gz-git-integrate-base-*` registration behind. It now uses
  `removePreparedWorktree`, which detaches the context and verifies the
  registration is gone.
- Tests (Unix):
  - `TestIntegrateRunInterruptStopsReadinessGroupAndRemovesWorkspace`
    sends SIGINT to the test process while a blocking runner and its
    background child run, then asserts exit 2 with `ErrInterrupted`, both
    processes gone, the readiness root removed and unregistered, and the
    target unmoved.
  - `TestIntegrateRunInterruptDuringPushFinishesIntegration` holds the
    target push in an origin `pre-receive` hook, sends SIGINT, releases
    the hook, and asserts a successful run, the moved target and
    `RECLAIMED`. Removing the `WithoutCancel` line makes it fail with a
    killed `git push`.
  - `TestCheckCancelDuringBaselineUnregistersBaselineWorktree` cancels
    Check during the baseline probe; it failed before the
    `check_make.go` change.
- Not covered here: the cleanup error of the baseline worktree is still
  discarded, as before. ISSUE-84 keeps the startup reclaim of
  `gz-git-integrate-*` directories and the test-helper `os.Exit` scope.
