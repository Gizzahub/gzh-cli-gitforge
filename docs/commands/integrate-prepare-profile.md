# Removed gate declarations

gz-git does not build, execute, or lint repository code. The gate mode of
`gz-git integrate check` and `run` is always `none`: the git checks alone
decide readiness. Two gates that ran repository code are removed without a
fallback:

- the legacy make gate, with `--allow-skipped-checks` and its preparation
  profiles;
- the readiness contract (`contract-v1`): `.gz-git/readiness/` runners,
  `integrate bootstrap`, `integrate readiness review|update`, the
  `integrate-readiness-v1` capability, the preparation profiles
  `infra-ops-python-uv-v1` and `nd-stack-checked-tests-v1`, `plan-v1`
  resource plans, the host measurement slot with its FIFO admission and
  `HOST_SLOTS_HELD`, and the runner diagnostic files.

The `--lock-wait` flag is removed with `branch.lockWait`: its bound applied
only to the host slot. Git-mutation locks wait until the lock is free or the
command is interrupted.

Verification belongs to the task runtime (CE `ce task gate` and the
repository's own checks), which must pass before gz-git is called.

## Removed keys

A config that still carries one of these keys fails to load with an error
that names the key and the file to delete it from:

| Key                             | Removed with                     |
| ------------------------------- | -------------------------------- |
| `branch.makeTimeout`            | legacy make gate                 |
| `branch.makeOutcomeReport`      | legacy make gate                 |
| `branch.prepareProfile`         | legacy make gate                 |
| `integration.prepareProfile`    | legacy make gate (controller)    |
| `branch.readiness`              | readiness contract (any content) |
| `branch.prepareHighLoad`        | readiness preparation slot       |
| `branch.resourceHighLoad`       | readiness host measurement slot  |
| `integration.readinessApproval` | readiness update approvals       |
| `branch.lockWait`               | readiness host measurement slot  |

```text
branch.readiness was removed: gz-git no longer builds, runs, or lints repository code; verification belongs to the task runtime (CE `ce task gate` and the repository's own checks), which must pass before gz-git is called; delete it from /repo/.gz-git.yaml
```

Every loader rejects them: the repository-root declaration, workspace and
controller configs, `workspace` and `sync` configs, `clone --config`, the
task runtime, and the global config. The key is rejected wherever a `branch`
or `integration` mapping holds it, including under `workspaces` and
`profiles`. Delete the key from the named file.

## What stays

- The git checks: freshness, merge-tree, other branches, a clean working
  tree, a pushed source, and post-gate revalidation.
- The fail-fast local fences (task fence, execution run lock); gz-git never
  waits for another integration. See
  [integration resource scope](integrate-resources.md).
- The `readiness contract` row and the `readiness provenance` line of the
  check report, which keep their `none` output; run JSON keeps schema v1,
  with `resource_wait_duration_ms`, `validation_duration_ms`,
  `plan_duration_ms` and `preparation_duration_ms` always `0`.
- Receipts written by an older gz-git still decode; their preparation fields
  are read-only history.
