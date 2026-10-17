# Readiness contract update

`gz-git integrate readiness update` is the narrow transaction for changing an
existing target-owned Readiness V1 contract. It is
not a target rollout mechanism: both the target and source must already have a
valid V1 contract.

```sh
gz-git integrate readiness update run \
  --branch feature/readiness-change --target origin/master
```

## Global approval policy

The global gz-git configuration alone selects the approval mode:

```yaml
integration:
  readinessApproval: manual
```

`manual` is the default when the setting or file is absent. `automatic` opts into
agent-controlled contract updates. Invalid values or malformed configuration
fail visibly. Profiles, repository manifests and command flags cannot override
this setting. The file uses the existing `config.NewPaths` platform location:
`~/Library/Application Support/gz-git/config.yaml` on macOS, or
`${XDG_CONFIG_HOME:-$HOME/.config}/gz-git/config.yaml` on Linux. Existing `.yml`
and `.json` global files are also supported. Preserve other global settings.
`gz-git integrate readiness update policy` prints only the effective mode and
does not fetch, execute a runner, or mutate Git state.

In manual mode, `run` displays the repository, exact source/target commits and
complete contract diff, then asks for one `yes` from an interactive terminal.
Decline or EOF stops the update. No plan file or digest needs copying.
`review` explicitly selects this human path even when automatic mode is enabled.
The reviewed snapshot is recomputed after approval. Reading for more than 15
minutes does not invalidate an unchanged snapshot: a fresh internal transaction
is issued only for the identical reviewed state. Drift requires a new review.

In automatic mode, invoke the same command with an exact source and request:

```sh
gz-git integrate readiness update run \
  --branch feature/readiness-change --target origin/master \
  --expect-source "$SOURCE_SHA" --statement '테스트완료'
```

`SOURCE_SHA` must be the full commit intended for integration. `auto` explicitly
selects the automatic path, but still refuses unless the global policy is
`automatic`. Agent hosts may expose a pinned `branch-readiness-auto` launcher;
their own policy must authorize it. There is no chat-output watcher.

The Korean statement means “tests completed” and is only a request to execute
checks. The tool loads the **proposed runner** from an isolated source checkout
and runs it with the detached target checkout as its working directory and
`--source-dir` pointing to the source, preserving the V1 protocol against
frozen source/target commits. Successful execution and a valid
`json-v1` result with `status: ready` are required. Nonzero exit, malformed
output, `not_ready`, `unavailable`, changed commits or revoked global policy
stop the update. The candidate is trusted to define its own checks in this
opt-in mode; execution proves that runner's result, not test completeness or
the honesty of its code. Manual review remains available for broken-runner
repairs without executing either runner.

Actual automatic measurements are recorded under the Git common directory at
`gz-git-readiness-updates/<operation-id>.json`, including the frozen plan,
runner identity, result and duration. These are local observations, not
reusable authorization or proof that a push succeeded. The successful remote
operation ref below records application separately. Neither approval mode
reclaims the task branch or worktree; use the adopted task lifecycle to settle
remaining resources.

## Compatibility: saved plans

The original plan/apply interface remains available:

```sh
gz-git integrate readiness update plan \
  --branch feature/readiness-change \
  --target origin/master \
  --issuer alice \
  --output /tmp/readiness-update.json

# Run only from an interactive terminal, after an independent human review.
gz-git integrate readiness update apply \
  --plan /tmp/readiness-update.json \
  --confirm <CONFIRM_DIGEST>
```

## Plan is evidence, not authorization

`plan` makes no remote ref changes (although it may refresh local fetch state).
It requires `--issuer`, a canonical remote-tracking `--target`, and a lifetime
of at most 15 minutes. The emitted plan is strict JSON whose exact fields are
the reviewed evidence; the CLI separately prints their canonical confirmation
digest. Unknown, missing, or duplicate fields are rejected when the plan is
read back.

The source must be exactly one fast-forward commit ahead of the target. That
commit may change only the selected existing manifest (`.gz-git.yaml`,
`.gz-git.yml`, or `.gz-git.json`) and paths under `.gz-git/readiness/`; it may
not migrate the selected manifest path. The final manifest and readiness files
must be regular blobs. A change only to optional `branch.readiness.timeout`
(including adding or removing it) is also a contract change: the existing old/new
manifest and contract digests bind the execution budget without additional plan
fields. The duration must be positive and no greater than `2h`; omission uses
15 minutes. A runner rename and adding, changing, or deleting
readiness helpers are valid contract changes, including deletion of the old
runner, provided the resulting V1 contract is valid.

On Unix, runner timeout or caller cancellation first sends `SIGTERM` to the
runner's captured process group. A two-second cleanup grace allows a cooperative
gate to cancel bindings in separate process groups; a surviving outer group is
then sent `SIGKILL`. The supervisor completes this sequence synchronously before
returning. The timeout receipt records the initial `SIGTERM` request, not proof
of signal delivery or complete descendant cleanup. An uncooperative process that
escapes the outer group remains outside this guarantee; inherited output pipes
are independently bounded by the existing two-second wait delay. The cleanup
grace does not change the execution budget or turn a timeout into a ready verdict.

The plan binds the repository and push endpoint, source and target refs/object
IDs, destination ref, operation-marker ref, and both old and new manifest,
runner, readiness-tree, tree-digest, and contract-digest evidence. It does not
run the old runner: this transaction is specifically able to repair a broken
or replaced runner.

## Apply is a human-only, one-use transaction

`apply` accepts only a saved plan plus a `--confirm` value exactly equal to its
canonical digest. The supported CLI requires an interactive TTY and has no
`--yes` or environment bypass. A TTY and digest do not authenticate the
operator: the digest only acknowledges review, and downstream policy and hook
enforcement are required to keep automated agents outside this boundary.

Before pushing, `apply` recomputes the complete snapshot and fails closed on
expiry, plan tampering, source/target drift, endpoint changes, or a changed old
contract. It then performs one atomic push with exact leases for the target and
for an absent, unique operation marker. The successful push advances the target
and writes the operation ref together. While that marker is preserved, it
blocks reuse of the plan even if the target is later rolled back. If the remote
does not support atomic push or its custom operation ref, the command fails
closed and changes neither ref.

Operation refs below `refs/gz-git/readiness-update/operations/` are intended to
be permanent audit and replay-protection markers. Preserve them; deleting one
removes both its audit record and the replay protection it provides.

The downstream agent policy and PreToolUse hook must continue to deny raw
`integrate readiness update apply`. Automatic approval is a separate,
globally opted-in entrypoint, never an implicit bypass of saved-plan review.
