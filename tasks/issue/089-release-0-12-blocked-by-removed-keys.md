---
id: ISSUE-89
title: "gz-git 0.12.0 refuses integrate in repositories that still declare removed gate keys"
type: bug
priority: P1
severity: high
discovered-in: "legacy make gate (ee40ef50) and readiness contract removal"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

gz-git no longer runs repository code. The legacy make gate and the
`contract-v1` readiness contract are removed. A `.gz-git.yaml` that still
declares one of these keys fails to load with
`<key> was removed: ...; delete it from <path>`:

- `branch.makeTimeout`, `branch.makeOutcomeReport`
- `branch.prepareProfile`, `integration.prepareProfile`
- `branch.readiness` (any content)
- `branch.prepareHighLoad`, `branch.resourceHighLoad`
- `integration.readinessApproval`
- `branch.lockWait` (the `--lock-wait` flag is removed with it; it only
  bounded the gate host slot)

This is intended: 0.12.0 releases normally and each owner fixes the error in
its own repository. Verification that the readiness runner performed moves
to the task runtime (CE `ce task gate` and the repository's declared checks,
run by `ce task run-finish` before it calls gz-git). That CE change lands in
ce-agent-kit before, or in the same cycle as, the 0.12.0 release. This card
is the impact list, not a release precondition.

## Reproduction

1. Install a gz-git built from the readiness contract removal or later.
1. In `~/mydevbox/gzh-cli-devbox`, run `gz-git integrate check`.

## Expected vs Actual

- Expected: an owner who has not yet deleted a removed key sees one clear
  error naming the key and the file, and `integrate` works after deleting it.
- Actual (before 0.12.0): the keys load, and the readiness runner executes
  repository code under gz-git.

## Evidence

Scan of the `.gz-git.yaml` files on mbp, 2026-10-21. These are only direct
children of `branch:` and `integration:`. No file declares
`integration.readinessApproval`. Another 85 hits are in task worktrees, which
inherit these lines from their branches.

| Repository (`~` = home)                                      | Removed keys                                                                                       |
| ------------------------------------------------------------ | -------------------------------------------------------------------------------------------------- |
| `~/mydevbox/cwrapper-devbox`                                 | `branch.makeTimeout`                                                                               |
| `~/mydevbox/flow-taskchain-devbox`                           | `branch.prepareProfile`                                                                            |
| `~/mydevbox/scripton-nd-stack-devbox`                        | `branch.makeTimeout`, `branch.prepareHighLoad`, `branch.readiness`                                 |
| `~/mydevbox/sigdock-idp-devbox/sigdock-idp-rs`               | `branch.makeTimeout`, `branch.prepareHighLoad`, `branch.prepareProfile`, `branch.resourceHighLoad` |
| `~/mydevbox/task-manager-devbox/taskchain-task-manager`      | `branch.makeTimeout`, `branch.resourceHighLoad`                                                    |
| `~/mywork/iac/infra-ops`                                     | `branch.makeTimeout`, `branch.prepareHighLoad`, `branch.prepareProfile`, `branch.resourceHighLoad` |
| `~/mydevbox/familybook-devbox` (+ `familybook-engine-fiber`) | `branch.readiness`                                                                                 |
| `~/mywork/ce/ce-agent-kit`, `~/mywork/scripton/dva`          | `branch.readiness`                                                                                 |
| `~/mydevbox/gzh-cli-devbox` (this repository's DevBox)       | `branch.readiness`                                                                                 |

These repositories declare only `branch.readiness`:

- **DevBox roots:** airouter, flow-agent-mesh, flow-observechain, flow-station,
  funbricks-elemhant, funbricks-notifire, funbricks-postkit, gizza-plane,
  gizzahub, gorisa, hek, lottomaster, mansero, matdosa, netow, policy-gate,
  primeno1, reviewrary, sadawiki, scripton-db-orchestrator, scripton-gitrump,
  scripton-signalhub, task-manager.
- **Subprojects:** notifire-frontend-react, postkit-engine-fiber,
  postkit-int-embed, postkit-ui-react, gizzahub-web-svelte,
  db-orchestrator-ui, gitrump-ce, gitrump-cli, gitrump-ee, sigdock-idp-ts,
  sigdock-client, sigdock-extension, sigdock-server-rs, sigdock-shared,
  sigdock-ts, sigdock-vscode.

### Global config on mbp

The 2026-10-21 scan covered only `.gz-git.yaml` files. The global gz-git
config on mbp (`~/Library/Application Support/gz-git/config.yaml`) declares
`integration.readinessApproval: automatic`; ISSUE-94 records it as coming
from devenv readinessApproval seeding. Installed gz-git 0.11.1 still accepts
it; a 0.12.0 build refuses to load it, and `gz-git branch name` then falls
back to the default naming without saying why (ISSUE-97).

The seed is ce-devenv `config/gz-git/config.base.yaml` (devenv commits
239e0093..74f8a8d9, 2026-10-21), which writes the key into a device's global
config when none exists. Remove it there before tagging 0.12.0, and delete
the key from seeded devices only after 0.12.0 is installed: an older gz-git
asks for readiness approval again without it.

## Scope

- Each owner deletes the removed keys in its own repository. If a readiness
  runner did real verification, the owner keeps the checks that runner
  invoked (for example `ce task gate`) as the repository's declared checks,
  which CE runs before integrating.
- Re-scan when 0.12.0 is published and close this card when the scan is
  empty. This card does not change any other repository.
- The re-scan includes each machine's global config, and the owner that
  seeds the key (ce-devenv, per ISSUE-94) is asked to remove it.
