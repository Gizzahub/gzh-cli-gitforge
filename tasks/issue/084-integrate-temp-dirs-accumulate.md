---
id: ISSUE-84
title: "gz-git-integrate-* temp directories accumulate in the system temp dir"
type: bug
priority: P2
severity: medium
discovered-in: "gzh-cli-gitforge inactive worktree cleanup"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

The macOS user temp dir (`$TMPDIR`) on the mbp host had 223 leftover
`gz-git-integrate-*` directories, about 19 GB in total, built up over two days.
They were left by integrate runs and by package tests, and nothing reclaims
them later.

| Prefix                       | Count                | Size                                            | Created by                                                 |
| ---------------------------- | -------------------- | ----------------------------------------------- | ---------------------------------------------------------- |
| `gz-git-integrate-state-*`   | 218 (mostly empty)   | small                                           | `TestMain`, `pkg/integrate/measurement_lock_test.go:49`    |
| `gz-git-integrate-prepare-*` | 4 (2026-10-18)       | 17 GB, 1.9 GB, 24 MB, 24 MB                     | `pkg/integrate/prepare.go:137`, `:205`                     |
| `gz-git-integrate-base-*`    | 1 (2026-10-20 13:33) | 190 MB, `wt` still registered as a git worktree | `baselineAgainstTarget`, `pkg/integrate/check_make.go:720` |

No gz-git process was running when this inventory was taken, so none of these
directories belonged to a live run.

## Causes

Cause 1 was reproduced on 2026-10-20 (see Reproduction below). Cause 2 comes
from reading the code and has not been reproduced.

1. **State dirs from test helpers.** `TestMain` creates the state dir and
   removes it only after `m.Run()` returns. Helper tests that the suite re-runs
   as child processes end with `os.Exit` from inside the test, so in the child
   `m.Run()` never returns. Examples: `TestReadinessNestedGroupHelper`
   (`readiness_nested_unix_test.go:63-74`) and the waitdelay helper
   (`readiness_waitdelay_unix_test.go:64-71`). Every such child therefore
   leaves one state dir behind. A suite that is killed, or a `go test -timeout`
   panic, leaks the parent's state dir the same way.
1. **Prepare and base dirs on abnormal exit.** Normal returns clean up through
   `defer os.RemoveAll(root)` and `worktreeRemoveForce`. A signal or kill skips
   those defers, which is the same mechanism as
   [ISSUE-82](082-integrate-run-interrupt-orphans-readiness.md). The leaked
   base `wt` also stays registered in the target repository's
   `git worktree list`.

## Reproduction (cause 1)

Each case ran with `TMPDIR` set to its own empty directory, so no concurrent
run could add entries:
`TMPDIR=<empty dir> GOWORK=off go test -count=1 -run '<pattern>' ./pkg/integrate/`.
Every run passed.

| `-run` pattern                                         | `gz-git-integrate-state-*` left behind |
| ------------------------------------------------------ | -------------------------------------- |
| `^$` (control: no tests, `TestMain` only)              | 0                                      |
| `^TestExecuteReadinessCancelLetsGateCleanNestedGroup$` | 1                                      |
| `^TestExecuteReadinessWaitDelayBoundsEscapedPipe$`     | 1                                      |

The parent's own state dir is removed. The one left behind belongs to the
child test binary that exits through `os.Exit`.

## Expected vs Actual

- Expected: test runs and integrate runs, whether they finish normally or are
  interrupted, leave no `gz-git-integrate-*` directories or worktree
  registrations behind; or the next run reclaims them.
- Actual: directories pile up without limit. A single prepare dir can reach
  17 GB.

## Manual recovery used (2026-10-20)

After confirming no gz-git process was running, only directories older than
60 minutes were removed: `rmdir` for empty state dirs, `rm -rf` for the rest,
and `git worktree remove --force` followed by `git worktree prune` for the base
worktree.

## Follow-up scope

- Make test helpers return without `os.Exit`, or give each child its own state
  dir that it removes itself.
- On an integrate signal, clean up through `withInterruptCancel`, together with
  ISSUE-82.
- Consider reclaiming stale `gz-git-integrate-*` directories at startup.

## Related

- [ISSUE-82](082-integrate-run-interrupt-orphans-readiness.md): readiness
  orphans left by an integrate interrupt.
