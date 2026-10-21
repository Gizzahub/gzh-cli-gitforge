---
id: ISSUE-89
title: "Releasing 0.12.0 breaks integrate in repositories that still declare removed make keys"
type: bug
priority: P1
severity: high
discovered-in: "legacy make gate removal (ee40ef50) release preparation"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

Master (0ec69cfb) removed the legacy make gate. A `.gz-git.yaml` that still
declares `branch.makeTimeout`, `branch.makeOutcomeReport`,
`branch.prepareProfile` or `integration.prepareProfile` now fails to load
with `<key> was removed: ...; delete it from <path>`. `VERSION` is still
0.11.1. A bump releases through CI (`release.yml`) and the Homebrew cask, so
every device that installs it refuses `gz-git integrate` in the repositories
below until their owners delete the keys.

## Reproduction

1. Install a gz-git built from master 0ec69cfb or later.
1. In `~/mydevbox/cwrapper-devbox`, run `gz-git integrate check`.

## Expected vs Actual

- Expected: the release lands after every owner has deleted the removed keys,
  so `integrate` keeps working everywhere.
- Actual: the load fails with
  `branch.makeTimeout was removed: gz-git no longer runs make; delete it from <path>`
  in each repository listed below.

## Evidence

Scan of primary checkouts on mbp, 2026-10-21 (task worktrees inherit the same
lines on their branches):

| Repository | Removed keys |
| --- | --- |
| `~/mydevbox/cwrapper-devbox` | `branch.makeTimeout: 90m` |
| `~/mydevbox/familybook-devbox` (workspace `familybook-engine-fiber`) | `integration.prepareProfile: familybook-ent-v1` |
| `~/mydevbox/flow-taskchain-devbox` | `branch.prepareProfile: flow-taskchain-local-subprojects-v1` |
| `~/mydevbox/scripton-nd-stack-devbox` | `branch.makeTimeout: 45m` |
| `~/mydevbox/sigdock-idp-devbox/sigdock-idp-rs` | `branch.makeTimeout`, `branch.prepareProfile: cargo-workspace-v1` |
| `~/mydevbox/task-manager-devbox/taskchain-task-manager` | `branch.makeTimeout: "45m"` |
| `~/mywork/iac/infra-ops` | `branch.makeTimeout: 52m`, `branch.prepareProfile: infra-ops-python-uv-v1` |

`resourceHighLoad` and `prepareHighLoad` still load. Without a readiness
contract they no longer guard any code run.

## Scope

- Each owner removes the keys in its own repository. Where the repository
  relied on make for verification, it declares a `branch.readiness` contract
  or runs its checks before integrating. infra-ops and sigdock-idp-rs move
  their kept profiles to `branch.readiness.prepareProfile` only together with
  a contract.
- Bump `VERSION` to 0.12.0 (minor: removal of public behavior) only after the
  owners have cleaned up, or with an explicit decision to let those
  repositories fail and fix forward.
- Re-scan before the bump. This card does not change any other repository.
