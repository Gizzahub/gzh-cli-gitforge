---
id: ISSUE-107
title: "No governed reusable gate worktree, so a consumer cannot keep its gate build warm at a fixed path"
type: bug
priority: P3
severity: low
discovered-in: "sigdock-idp-rs workspace gate timing, 2026-10-23"
discovered-at: 2026-10-23
created: 2026-10-23
---

## Summary

A consumer repository's full gate runs in the candidate's task worktree.
That worktree is new for every task, so its `target/` starts empty. A gate
worktree kept at one fixed path and moved to each candidate SHA would keep
the workspace's own crates compiled, but no governed command creates one:
`gz-git worktree add <branch> [path]` has no detached mode, and the CE task
runtime only creates task worktrees. sigdock-idp-devbox
`docs/40-development/47-verification-and-batch-integration.md` (§배치 통합
절차, steps 2–3) does not authorize raw worktree creation (step 2) and says
to record a missing capability rather than bypass it with ad-hoc Git
commands (step 3).

The benefit is not established. sigdock-idp-rs timings from 2026-10-23
(Evidence) range from 632s to 894s for gates in an already-gated worktree,
793s to 924s for first gates with their own `target/`, and 1045s to 1065s
for first gates with a shared `CARGO_TARGET_DIR`. That is too uncontrolled
to size the gain.

## Reproduction

1. In sigdock-idp-rs, which has no CE task-runtime declaration and uses its
   existing gz-git workflow, create a task worktree with
   `gz-git worktree add -b dev/<actor>/<host>/<type>/<slug> <worktree-root>/<actor>__<host>__<type>__<slug>`
   and run the full gate (`make check`) there.
1. Run the gate again in the same worktree and compare the stage durations
   in each `summary.txt`.
1. Look for a governed way to keep one gate worktree at a fixed path and
   move it detached to the next candidate SHA. `gz-git worktree add --help`
   lists only `-b/--create` and `--force`.

## Evidence

Values are `command_duration_s` (workspace `cargo test`) and
`feature_compile_duration_s` from each gate's `summary.txt` under the
sigdock-idp-rs pool `.omo/evidence/workspace-gate-20261023-<run>/`
(gitignored, local to the reporting device).

| Run | rs HEAD | Worktree state | Workspace stage | Server feature compile |
|---|---|---|---|---|
| 070938 | `4cdec01b9` | first gate in worktree A, `gate_exit=1` | 793s | 1025s |
| 074207 | `ac0cc6a34` | second gate in A | 633s | 603s |
| 080356 | `2975dbcd9` | third gate in A | 632s | 552s |
| 103303 | `d0381e9d4` | fourth gate in A; its feature stage ran alongside all of 104333's workspace stage | 894s | 895s |
| 094134 | `54622cbe4` | first gate in a new worktree | 924s | 693s |
| 101516 | `8d19bf366` | new worktree, shared `CARGO_TARGET_DIR` starting empty, stopped during the feature stage after 103303 started | 1065s | — |
| 104333 | `39e3f5028` | new worktree, shared `CARGO_TARGET_DIR` with workspace members cleaned; whole workspace stage overlapped 103303 (about 17 min) | 1045s | 724s |

The rows are not comparable like-for-like:
- The runs differ in commit, gate script revision (080356 ran 14 portable lanes; 094134 and 104333 ran none), `Cargo.lock`, and cargo binary path (mise shim vs `~/.cargo/bin`).
- Other sessions' builds shared the CPU, including the overlap marked above.

A shared `CARGO_TARGET_DIR` across worktrees is not an alternative. cargo
names workspace-member artifacts relative to the workspace root, so two
worktrees share fingerprints and freshness is decided by mtime alone; a
worktree edited before another worktree's build then runs that worktree's
test binaries and passes (reproduced on a local toy workspace; no artifact
kept). Making it safe requires cleaning every workspace member on a switch,
which leaves only third-party dependencies reused. The one such run above
(104333) was not faster than the new-worktree run, but it overlapped
another gate for its whole workspace stage, so it does not measure the
gain either.

## Expected vs Actual

- Expected: a lifecycle-owned gate worktree per repository (and device)
  that stays at one path, is checked out detached to the candidate SHA
  under the integration resource lock, records which SHA it gated, and is
  reclaimable by the same lifecycle.
- Actual: the only governed worktrees are task worktrees. Each gate starts
  in a new path, and keeping a warm one needs a raw worktree outside any
  lifecycle.

## Scope

- Before building it: a controlled measurement (same commit, script and
  toolchain, no concurrent gate) of a first gate vs a repeat gate at one
  path, to decide whether the gain justifies the capability.
- Owner decision: whether the gate worktree belongs to `gz-git worktree`,
  to `gz-git integrate`, or to the CE task runtime. `gz-git integrate
  prepare` already checks out detached worktrees, but in temporary
  directories that are not reused.
- It must not let a gate verdict attach to a SHA other than the one checked
  out; the sigdock gate already records `nested_head` and dirty files.
- Non-goal: sharing build output across different worktree paths (see
  Evidence).
