---
id: ISSUE-107
title: "No governed reusable gate worktree, so every candidate gate cold-compiles its workspace"
type: bug
priority: P2
severity: medium
discovered-in: "sigdock-idp-rs workspace gate timing, 2026-10-23"
discovered-at: 2026-10-23
created: 2026-10-23
---

## Summary

A consumer repository's full gate runs in the candidate's task worktree.
That worktree is new for every task, so its `target/` starts empty and the
first gate compiles the whole workspace. sigdock-idp-rs measured the same
gate's workspace stage at 924s in a fresh worktree and 632s in a worktree
already gated twice (2026-10-23). A gate worktree kept at one fixed path and
moved to each candidate SHA would keep that difference, but no governed lifecycle
creates one: `gz-git worktree add` has no detached mode, and a raw
`git worktree add --detach` outside the lifecycle is forbidden by the
shared policy and by sigdock-idp-devbox
`docs/40-development/47-verification-and-batch-integration.md`.

## Reproduction

1. Create a task worktree for a sigdock-idp-rs candidate with
   `ce task run-start` and run its full gate (`make check`) there.
1. Run the same gate again in that worktree, or in a worktree that has
   already been gated, and compare the stage durations in each
   `summary.txt`.
1. Look for a governed way to keep one gate worktree at a fixed path and
   move it to the next candidate SHA: `gz-git worktree add` has no detached
   mode, and the CE task runtime only creates task worktrees.

## Evidence

Values are `command_duration_s` (workspace `cargo test`) and
`feature_compile_duration_s` from each gate's `summary.txt`. The commits
differ, and other sessions' builds may have shared the CPU, so the rows are
indicative, not a controlled benchmark.

| Gate (rs HEAD) | Worktree state | Workspace stage | Server feature compile |
|---|---|---|---|
| 08:03 `2975dbcd9` | third gate in the same worktree | 632s | 552s |
| 09:41 `54622cbe4` | fresh | 924s | 693s |
| 10:43 `39e3f5028` | fresh, shared `CARGO_TARGET_DIR` holding prebuilt third-party deps, workspace members cleaned | 1045s | 724s |

The third row rules out sharing one `CARGO_TARGET_DIR` across worktrees.
cargo names workspace-member artifacts relative to the workspace root, so
two worktrees share fingerprints and freshness is decided by mtime alone.
A worktree edited before another worktree's build then runs that
worktree's test binaries and passes (reproduced on a toy workspace). Making
it safe requires cleaning every workspace member on a worktree switch,
which leaves only third-party dependencies reused, and that saved nothing.
The cost lives in the workspace's own crates, which only stay warm at a
fixed path.

Evidence summaries: sigdock-idp-rs pool
`.omo/evidence/workspace-gate-20261023-{080356,094134,104333}/summary.txt`
(gitignored, local to the reporting device).

## Expected vs Actual

- Expected: a lifecycle-owned gate worktree per repository (and device)
  that stays at one path, is checked out detached to the candidate SHA
  under the integration resource lock, records which SHA it gated, and is
  reclaimable by the same lifecycle.
- Actual: the only governed worktrees are task worktrees. Each gate starts
  cold, and keeping a warm one means a raw worktree the policy forbids.

## Scope

- Owner decision first: whether the gate worktree belongs to `gz-git
  worktree`, to `gz-git integrate`, or to the CE task runtime.
- It must not let a gate verdict attach to a SHA other than the one checked
  out; the gate already records `nested_head` and dirty files.
- Non-goal: sharing build output across different worktree paths (see
  Evidence).
