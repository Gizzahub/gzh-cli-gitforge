# gz-git `run` JSON output schema

Consumer contract for task-manager (and any other caller) driving the
gz-git run lifecycle. Every verb answers `--json` with **one response
document on stdout**; refusals additionally print `Error: <reason>` on
stderr next to their non-zero exit. The design-level reference is
[docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md](design/RUN_LIFECYCLE_PARITY_CONTRACT.md);
when this document and the fixture suite (`tests/parity/`) disagree, the
fixtures win.

---

## 1. Verbs and invocation

CLI verbs are two words under the `run` group. Protocol tokens
(`allowedActions` values, receipt `operation` names) are CE-hyphenated and
are **never** argv: to act on `run-finish`, invoke `gz-git run finish`.

| Invocation | Protocol token | Purpose |
|------------|----------------|---------|
| `gz-git run doctor [--json]` | `run-doctor` | Runtime dependency check |
| `gz-git run start <task> --type <t> [--json]` | `run-start` | Create the owner-bound task worktree; `<t>` ∈ `feat\|fix\|refactor\|docs\|test\|chore\|perf` |
| `gz-git run status [task] [--json]` | `run-status` | Derived state for one task (or the repository answer), lock-free |
| `gz-git run list [--json]` | `run-list` | All repository-shared executions, lock-free |
| `gz-git run finish [task] [--json]` | `run-finish` | Verify, integrate, and reclaim a clean pushed task; no arg = the single ACTIVE run |
| `gz-git run recover <task> [--json]` | `run-recover` | Close a proven exit-3 cleanup failure without deleting residuals |
| `gz-git run abort <task> [--reason R] [--json]` | `run-abort` | Terminal ABORTED closure; removes nothing |
| `gz-git run discard <task> --reason R [--take-over-from actor/host] [--json]` | `run-discard` | Remove the exact clean unintegrated worktree/branch |
| `gz-git run import-ce [--dry-run] [--json]` | — | One-shot carryover of in-flight CE records (ADR-0055); separate report, §7 |

## 2. Exit codes

| Code | Meaning |
|------|---------|
| 0 | Answer delivered (including refusals that are no-ops, e.g. re-`status` of a DONE task, idempotent re-discard) |
| 1 | Refusal, precondition failure, unknown task, BLOCKED runtime, parse errors |
| 1–2 (propagated) | `finish` only: the in-process provider `check`/`run` exit codes pass through verbatim when integration itself fails |
| 3 | `finish` only: **integration succeeded but recovery cleanup failed**. The work is integrated and pushed; residuals remain and `run-recover` is the next action |

## 3. Response document

All verbs except `run list` and `run import-ce` return one `TaskResponse`.
Fields are serialized in this shape (`schemaVersion` is currently `1`;
empty-array fields serialize as `null`, not `[]`):

```json
{
  "schemaVersion": 1,
  "status": "ACTIVE",
  "activeCount": 0,
  "task": "reclaimed-task",
  "allowedActions": ["run-status"],
  "reason": "task execution started",
  "nextAction": "work in the task worktree; use run-status to evaluate finish readiness",
  "finishReady": false,
  "warnings": [],
  "lock": {},
  "execution": {},
  "executions": [],
  "states": [],
  "receipt": {},
  "diagnostics": [],
  "provider": {}
}
```

| Field | Type | Present | Meaning |
|-------|------|---------|---------|
| `schemaVersion` | int | always | Output contract version |
| `status` | string | always | The verb's answer token (§4) |
| `activeCount` | int | always | Repository-wide count of ACTIVE executions |
| `task` | string | always | Normalized task id the verb answered for ("" when the verb is not task-scoped) |
| `allowedActions` | string[] \| null | always | Protocol tokens the caller may act on next (§5); `null` when nothing applies |
| `reason` | string | always | Human-readable answer/refusal text — safe to surface verbatim |
| `nextAction` | string | always | Recommended next step in prose |
| `finishReady` | bool | always | `true` only when `run finish` would proceed for this task right now |
| `warnings` | string[] | when non-empty | Non-fatal observations (e.g. identity resolution warnings) |
| `lock` | object | when a lock is observed | `TaskLockInfo` (§6.4) |
| `execution` | object | when one execution is the subject | `TaskExecution` (§6.1) |
| `executions` | object[] | `run list` only | History array **without** status (§6.1 objects) |
| `states` | object[] | `run list` only | Derived states, same order as `executions` (§6.2) |
| `receipt` | object | when a receipt was read or written | `TaskReceipt` (§6.3) |
| `diagnostics` | object[] | when external commands ran | `CommandDiagnostics` (§6.5) |
| `provider` | object | `run doctor` only | `IntegrationProviderDiagnostics` (§6.6) |

## 4. Status tokens

| Token | Meaning | Terminal? |
|-------|---------|-----------|
| `ACTIVE` | A live execution exists (also the healthy `run doctor` answer) | no |
| `READY` | No live execution and nothing blocking — the idle answer | no |
| `BLOCKED` | Refused or the runtime cannot act (lock, precondition, owner, config) | no |
| `UNKNOWN` | No record for the requested task | no |
| `DONE` | Integrated and reclaimed, proven by receipt | yes |
| `ABORTED` | Closed without integrating (abort/discard), proven by receipt | yes |

Only `DONE` and `ABORTED` are terminal. An ACTIVE execution is judged from
`states[].status`, never from the presence of an `executions[]` entry: a
run reclaimed outside the runtime keeps its record but reports a terminal
state, and `activeCount` drops.

## 5. `allowedActions` protocol tokens

One of: `run-start`, `run-status`, `run-list`, `run-finish`, `run-abort`,
`run-discard`, `run-recover`. Hyphenated by contract; the CLI form is
`gz-git run <token after the hyphen>` (§1). Tokens are suggestions backed
by the derived state, not guarantees — each verb re-verifies everything.

## 6. Nested object schemas

### 6.1 `TaskExecution` (execution, executions[])

| Field | Type | Meaning |
|-------|------|---------|
| `task` | string | Task id |
| `type` | string | Execution type: `feat` `fix` `refactor` `docs` `test` `chore` `perf` |
| `source` | string | Source branch the task branched from |
| `owner` | object | `{actor, host, kind}` — `kind` is `agent` or `person` |
| `branch` | string | Task branch `dev/<actor>/<host>/<type>/<task>` |
| `worktree` | string | Absolute worktree path |
| `startedAt` / `updatedAt` | string | RFC3339 UTC timestamps |

### 6.2 `DerivedTaskState` (states[])

`{execution, status, allowedActions, reason, nextAction, finishReady,
diagnostics?}` — a per-record answer with the same semantics as the
top-level fields (§3, §4), plus the record's `execution` (§6.1).

### 6.3 `TaskReceipt` (receipt)

Append-only evidence, one per lifecycle event. `operation` ∈ `start`,
`finish`, `abort`, `discard`, `recover`; `status` is a §4 token.

| Field group | Fields |
|-------------|--------|
| Identity | `task`, `operation`, `status`, `owner`, `performedBy?`, `branch`, `worktree`, `reason?`, `createdAt` |
| Finish evidence | `sourceHead?`, `sourceHeadBefore?`, `baseHead?`, `taskHead?`, `sourcePushed` |
| Reclaim evidence | `worktreeRemoved`, `localBranchRemoved`, `remoteBranchRemoved` |
| Build stamps | `tool_version?`, `tool_revision?` (§8) |
| Evidence trail | `diagnostics?` (§6.5) |

### 6.4 `TaskLockInfo` (lock)

`{path, pid?, createdAt?, ageSeconds?, pidRunning?}`. A lock is never
removed automatically; a BLOCKED answer citing the lock names its holder.

### 6.5 `CommandDiagnostics` (diagnostics[])

`{command, executable?, args?, cwd?, exitCode, stdout?, stderr?, error?,
notStarted?}` — what ran, where, and what it returned, so every refusal is
auditable without re-running it.

### 6.6 `IntegrationProviderDiagnostics` (provider)

`{name, present, version?, executable?, capabilities}` — doctor's view of
the integration provider. In gz-git the provider is this binary
(integration is in-process), so `executable` names the running gz-git
itself, and `capabilities` is `{integrate-check: true, integrate-run: true,
integrate-no-fetch: true}` — the no-fetch capability is declared only when
both check and run support it.

## 7. `run import-ce` report

```json
{
  "schemaVersion": 1,
  "imported": false,
  "dryRun": true,
  "executions": 2,
  "receipts": 5,
  "sourceDir": "<git-common-dir>/ce/task-runtime/v1",
  "targetDir": "<git-common-dir>/gz-git/task-runtime/v1"
}
```

| Field | Meaning |
|-------|---------|
| `imported` | `true` only when bytes were written |
| `dryRun` | Echoes `--dry-run`; a dry run reports counts and writes nothing |
| `executions` / `receipts` | Record counts found at the source (0 when that file is absent) |
| `sourceDir` / `targetDir` | Resolved CE state and gz-git state directories |

On refusal (existing gz-git state, nothing to import, unreadable source)
the command exits non-zero, and with `--json` the report is still printed
first so the caller can see what was (not) imported and why.

## 8. `tool_version` and `tool_revision`

Receipt build stamps of **the tool that wrote the receipt** — not of the
record format. They are the two snake_case fields in the envelope (a CE
layout parity artifact the fixtures pin); every other key is camelCase.
task-manager must treat them as opaque strings for audit trails, never as
a format version: `schemaVersion` (§3) is the contract version.

## 9. Stability rules

- `schemaVersion: 1` fields are added only additively; a breaking change
  bumps the version.
- Status/allowedActions/operation token sets (§4, §5, §6.3) are closed —
  new tokens appear only with a schema review.
- `reason` and `nextAction` are prose: match on tokens and exit codes,
  never on their text.
- The fixture goldens (`tests/parity/testdata/golden/`) are the executable
  form of this schema; `PARITY_TARGET=gz-git go test ./tests/parity/...`
  asserts the shipped binary against them.
