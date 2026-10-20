# Integration preparation profiles

gz-git does not build, execute, or lint repository code itself. Preparation
exists only as part of a target-owned readiness contract, which selects one
closed profile in `branch.readiness.prepareProfile`:

```yaml
branch:
  integrationBranch: develop
  readiness:
    version: 1
    runner: .gz-git/readiness/check
    prepareProfile: infra-ops-python-uv-v1
```

The field accepts only built-in profile IDs. It does not accept commands,
paths, environment variables, or network endpoints. It is part of the
contract digest, so the target and source contracts must agree on it, and
adopting or changing it goes through `gz-git integrate readiness update` or
the first-bootstrap procedure. The supported profiles are:

| Profile                     | Prepares                                                                                  |
| --------------------------- | ----------------------------------------------------------------------------------------- |
| `infra-ops-python-uv-v1`    | the source snapshot of the supported infra-ops layout (below)                             |
| `nd-stack-checked-tests-v1` | the checked ND binding; see [ND checked-binding preparation](integrate-nd-preparation.md) |

## Removed declarations

The legacy make gate and its preparation path are gone. A declaration that
still carries one of these keys fails to load with an error that names the key
and the file to delete it from, for example
`branch.prepareProfile was removed: gz-git no longer runs make; a readiness contract selects preparation with branch.readiness.prepareProfile; delete it from /repo/.gz-git.yaml`:

- repository-root `branch.prepareProfile` (with its profiles
  `flow-taskchain-local-subprojects-v1`, `cargo-workspace-v1`,
  `pnpm-frozen-lockfile-v1` and `familybook-ent-v1`);
- controller `integration.prepareProfile`;
- `branch.makeTimeout` and `branch.makeOutcomeReport`.

There is no fallback: delete the key from that repository's `.gz-git.yaml`.
A repository that needs code-level verification declares a readiness
contract; without one, the git checks alone decide readiness.

## Resource class

For a confirmed heavy profile, set repository-root `branch.prepareHighLoad: true`.
This strict boolean is independent of `branch.resourceHighLoad`, which classifies
the readiness runner. Either committed source or target opting in uses the host
slot for each actual profile executor and its integrity checks. Worktree
materialization and cleanup remain outside. Absent or false leaves preparation
light. When `branch.prepareHighLoad: true` is set, its root config file must be
at most 64 KiB. See [integration resource scope](integrate-resources.md) for
inheritance and waits.

## Ref invariant

Every profile is held to one ref invariant: preparation must not move the source ref, the target (bound as the exact ref it resolves to, so a remote-tracking target does not also bind a same-named local branch), or the prepared worktree's HEAD, which must stay detached at the same commit. Refs are compared by name and commit. The failure reads "bound ref moved during preparation (possibly by another session)" because a concurrent session can move a bound ref too. Other branches created or deleted meanwhile (by another session or a fetch --prune) are not a preparation change. Accepted trade-off: a ref outside the bound set that preparation itself creates, moves or deletes is not detected.

## `infra-ops-python-uv-v1`

`infra-ops-python-uv-v1` is a fixed, opt-in executor for the supported
infra-ops layout. It prepares the source snapshot only, because the V1 runner
receives one environment and validates `--source-dir`. The source commit must
contain `scripts/worktree-env-inputs.json`, a schema-version-1 JSON document
with the two ordered dependency names `sbkube` and `cloud-script-transformer`
and their lowercase 40-character commit IDs. The manifest does not name
remotes, paths, commands, registries, or environment variables. Those are fixed
by this executor: it checks out both dependencies from their canonical remotes,
runs `uv sync --frozen`, and runs frozen pnpm installation for
`library/ga4-admin` and `library/m365ctl`.

Child clones and the virtual environment live under the task-owned prepare root
next to the source worktree, while `codes/*` inside the worktree are links to
those child clones. This avoids nested Git repositories that would prevent Git
from removing the prepared worktree. The runner receives only that worktree's
`UV_PROJECT_ENVIRONMENT`, `VIRTUAL_ENV`, and `UV_NO_SYNC=1`; inherited UV
variables are replaced only for this profile. The manifest blob and child
commit IDs appear in preparation evidence and durable receipt provenance,
alongside the source's Python and pnpm lockfile blob IDs. Missing tools,
invalid project shape, unsafe paths, clone/fetch failures, lockfile failures,
ref movement, or the 30-minute preparation ceiling fail preparation; there is
no offline or ambient-venv fallback.
Git dependency operations ignore global/system configuration, URL rewrites and
checkout hooks while retaining SSH transport authentication. Existing output
directories and symlink escapes are refused; preparation checks the created
venv and Node markers before declaring readiness. This remains process
isolation, not a sandbox for trusted repository code or dependencies.
The optional `codes/.envrc` boundary marker is preserved only when it is a
tracked regular source file. It is not an owned dependency resource. Untracked
markers, symlinks, directories and any other pre-existing `codes` entries are
refused. The tracked-tree guard still rejects changes to this marker during
preparation; cleanup never treats it as a generated resource.
