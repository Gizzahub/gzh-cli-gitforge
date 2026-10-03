# CE `task run-*` Lifecycle Parity Contract

Reference for porting the CE run lifecycle into gz-git, per ce-agent-kit
[ADR-0055](https://github.com/Gizzahub/ce-agent-kit/blob/master/decisions/adr/0055-task-manager-owns-tasks-gz-git-owns-lifecycle.md)
("task-manager owns tasks, gz-git owns the repository lifecycle, no compatibility
window") and the `run-lifecycle` row of the CE migration ledger
(`ce-agent-kit/docs/00-product/11-task-migration-ledger.md`).

Everything in this document is pinned to **CE source `ce-agent-kit` master
`f540f972`** (installed binary `ce 0.8.4`, build `498-gf540f972`, with
Worktrunk 0.80.0; fixtures were first authored against `bb970b24` /
`400-gbb970b24`, re-recorded across `f927ae5d` / `401-gf927ae5d`, re-recorded
again across the move to `771c54cf` / `404-g771c54cf` (stamp-only), again
across the move to `950650ef` / `475-g950650ef` (stamp plus captured wording:
the `nextAction` guidance grew a run-finish clause and the captured
`gz-git integrate` `--help` output gained `Effect: mutating (...)` lines), and
again across the move to `f540f972` / `498-gf540f972`. That last move changed
captured behaviour, not only stamps: CE `b09a605e` accepts the Worktrunk 0.80
line, and CE `f540f972` reads the run source from the remote-tracking ref — it
verifies `refs/remotes/origin/<integration>` (and `refs/heads/<integration>`)
with `git show-ref --verify`, creates the task worktree from that ref's SHA
instead of the local branch name, and passes `--target origin/<integration>`
to `gz-git integrate check/run` instead of `<integration>` /
`origin/<integration>..<integration>`. The sandbox therefore fetches its
origin once, as a real clone would. Schema, statuses and allowed actions were
unchanged, verified by golden diff. The gz-git port does not follow either
change yet; that is a port decision tracked outside the fixtures, not a
known-divergent scenario).
The behavioral reference is
the fixture suite in `tests/parity/` — golden files there are
machine-captured from the CE binary, never hand-written. This document
describes the same surface at the source level; when the two disagree, the
fixtures win.

Reference toolchain used by the fixtures:

| Component        | Version                                                    | Role                             |
| ---------------- | ---------------------------------------------------------- | -------------------------------- |
| `ce`             | 0.8.4 (`f540f972`)                                         | system under test (reference)    |
| `wt` (Worktrunk) | 0.80.0 (CE accepts the 0.80 minor line since `b09a605e`)   | worktree create/remove/inventory |
| `gz-git`         | 0.8.x with `integrate check/run` + `--no-fetch` capability | integration provider (reclaim)   |
| `git`            | system git                                                 | refs, worktree plumbing          |

Governing CE ADRs: 0006 (superseded original contract), 0015 (lifecycle
bypass prohibition + root safety), 0017 (abort verb, append-only closure),
0026 (provider adapter boundary only), 0028 (finish owns DONE evidence),
0049 (guarded discard), 0055 (ownership transfer to gz-git).

______________________________________________________________________

## 1. Verb surface

Hand-rolled dispatcher (no cobra): `cmd/ce/handlers_task.go` registers eight
verbs; parse errors print `Error: <msg>` to stderr and exit 1. `--help`/`-h`
short-circuits to the usage line and exits 0.

| Verb          | Usage                                                                                     | Positional args                                | Flags                                                          |
| ------------- | ----------------------------------------------------------------------------------------- | ---------------------------------------------- | -------------------------------------------------------------- |
| `run-doctor`  | `ce task run-doctor [--json]`                                                             | none                                           | `--json`                                                       |
| `run-start`   | `ce task run-start <task> --type <feat\|fix\|refactor\|docs\|test\|chore\|perf> [--json]` | exactly 1 (`run-start accepts one task`)       | `--type` (required value), `--json`                            |
| `run-status`  | `ce task run-status [task] [--json]`                                                      | 0 or 1                                         | `--json`                                                       |
| `run-list`    | `ce task run-list [--json]`                                                               | 0 (positional arg → `unexpected argument %s`)  | `--json`                                                       |
| `run-finish`  | `ce task run-finish [task] [--json]`                                                      | 0 or 1 (`run-finish accepts at most one task`) | `--json`                                                       |
| `run-recover` | `ce task run-recover <task> [--json]`                                                     | exactly 1                                      | `--json`                                                       |
| `run-abort`   | `ce task run-abort <task> [--reason R] [--json]`                                          | exactly 1                                      | `--reason` (optional value), `--json`                          |
| `run-discard` | `ce task run-discard <task> --reason R [--take-over-from actor/host] [--json]`            | exactly 1                                      | `--reason` (required), `--take-over-from` (optional), `--json` |

Flag-value parse errors: `--type requires a value`, `--reason requires a value`, `--take-over-from requires a value` (a value starting `--` is
rejected); unknown flags: `unknown flag %s (valid: ...)`.

### gz-git verb mapping (ADR-0055 port)

The port keeps CE's verb vocabulary but moves it into gz-git's cobra CLI:
the `task` namespace becomes the `run` command group, and the hyphenated
`run-<verb>` dispatch becomes two words. Protocol-level tokens in response
documents (`allowedActions`) keep the CE-hyphenated spelling — they name
capabilities, not CLI words.

| CE verb               | gz-git verb            | Flags (identical semantics)              | Notes                                                                       |
| --------------------- | ---------------------- | ---------------------------------------- | --------------------------------------------------------------------------- |
| `ce task run-doctor`  | `gz-git run doctor`    | `--json`                                 |                                                                             |
| `ce task run-start`   | `gz-git run start`     | `--type`, `--json`                       |                                                                             |
| `ce task run-status`  | `gz-git run status`    | `--json`                                 |                                                                             |
| `ce task run-list`    | `gz-git run list`      | `--json`                                 |                                                                             |
| `ce task run-finish`  | `gz-git run finish`    | `--json`                                 | known-divergent: no-arg finish with one ACTIVE run exits 0 (ISSUE-069)      |
| `ce task run-recover` | `gz-git run recover`   | `--json`                                 |                                                                             |
| `ce task run-abort`   | `gz-git run abort`     | `--reason`, `--json`                     |                                                                             |
| `ce task run-discard` | `gz-git run discard`   | `--reason`, `--take-over-from`, `--json` |                                                                             |
| —                     | `gz-git run import-ce` | `--dry-run`, `--json`                    | new: ADR-0055 one-shot carryover of in-flight CE records; no CE counterpart |

Structural differences, all declared port divergences rather than behavior
changes:

- **Dispatcher**: cobra replaces CE's hand-rolled parser. `--help`/`-h`
  renders cobra help and exits 0; flag parse errors come from cobra on
  stderr with exit 1. Required-flag enforcement (`--type`, `--reason`)
  lives in the command bodies and reproduces CE's wording.
- **Exit-code carrier**: identical — the response document always goes to
  stdout first; non-zero exits additionally print `Error: <reason>` on
  stderr.
- **Target config/state paths**: `.gz-git-task.yaml` and
  `<git common dir>/gz-git/task-runtime/v1/` replace CE's
  `.ce/task-runtime.yaml` and `<git common dir>/ce/task-runtime/v1/`
  (§3 layout and write discipline unchanged; file names identical).
- **Integration provider**: finish integrates in-process — gz-git *is*
  the provider CE shelled out to — so no external provider executable is
  probed for a path, and provider version/capability discovery is local.
- **CE state**: never read at lifecycle time. The only reader of
  `<git common dir>/ce/task-runtime/v1/` is `run import-ce`.

The fixture harness (`tests/parity/portmode_test.go`) maps
`["task", "run-<verb>"]` to `["run", "<verb>"]` at execution time and
canonicalizes both sides into one diffable shape; the canonicalization
header in that file is the exhaustive list of declared divergences.

### Exit codes

Carrier: stdout always carries the report first; on non-zero exit stderr
additionally gets `Error: <reason>` (`RuntimeExitError` implements
`ExitCode()`; `runOrFail` maps it to `os.Exit`).

| Verb                      | exit 0                                 | exit 1                                                   | exit 2                                                                        | exit 3                                                                                                                                              |
| ------------------------- | -------------------------------------- | -------------------------------------------------------- | ----------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `run-doctor`              | ACTIVE                                 | BLOCKED                                                  | —                                                                             | —                                                                                                                                                   |
| `run-start`               | ACTIVE                                 | BLOCKED (all failures, incl. parse→1)                    | —                                                                             | —                                                                                                                                                   |
| `run-status` / `run-list` | READY/ACTIVE answer                    | BLOCKED or UNKNOWN (e.g. `task execution was not found`) | —                                                                             | —                                                                                                                                                   |
| `run-finish`              | DONE (incl. already-DONE)              | refusals, owner mismatch, preconditions                  | provider `check`/`run` exit 1–2 propagated verbatim; out-of-range clamps to 2 | provider `run` exit 3 (`integration succeeded but task recovery cleanup failed`); also finish-side evidence/receipt-validation/persistence failures |
| `run-recover`             | DONE (incl. already-DONE)              | everything else                                          | —                                                                             | —                                                                                                                                                   |
| `run-abort`               | ABORTED (incl. already-terminal no-op) | failures, unknown task                                   | —                                                                             | —                                                                                                                                                   |
| `run-discard`             | ABORTED (incl. idempotent re-discard)  | all refusals                                             | —                                                                             | —                                                                                                                                                   |

______________________________________________________________________

## 2. JSON response envelope

Every verb's `--json` output is a single `TaskResponse` envelope
(`json.MarshalIndent`, two-space indent, trailing newline). `schemaVersion`
is always `1`.

```
schemaVersion int                  always 1
status        string               ACTIVE | READY | BLOCKED | UNKNOWN | DONE | ABORTED
activeCount   int                  count of states[].status == ACTIVE
task          string               normalized task name ("" on list answers)
allowedActions []string|null       admitted verbs for this answer (null → JSON null)
reason        string               human-readable cause
nextAction    string               suggested follow-up command
finishReady   bool
warnings      []string             omitempty (identity warnings)
lock          TaskLockInfo         omitempty (doctor/status under a lock)
execution     TaskExecution        omitempty (single-record answers)
executions    []TaskExecution      omitempty (list answers: raw history)
states        []DerivedTaskState   omitempty (list answers: derived, sorted by task)
receipt       TaskReceipt          omitempty (mutations + finish/status answers)
diagnostics   []CommandDiagnostics omitempty (external command evidence)
provider      IntegrationProviderDiagnostics omitempty (doctor only)
```

Nested shapes (exact json tags):

```
TaskOwner            {actor, host, kind}                 kind: "agent" | "human"
TaskExecution        {task, type, source, owner, branch, worktree, startedAt, updatedAt}
                     type: feat|fix|refactor|docs|test|chore|perf; times RFC3339 UTC
DerivedTaskState     {execution, status, allowedActions, reason, nextAction, finishReady, diagnostics?}
TaskLockInfo         {path, pid?, createdAt?, ageSeconds?, pidRunning?}
CommandDiagnostics   {command?, executable?, args?, cwd?, exitCode, stdout?, stderr?, error?, notStarted?}
IntegrationProviderDiagnostics {name, present, version?, executable?, capabilities{name:bool}}
TaskReceipt          {task, operation, status, owner, performedBy?, branch, worktree,
                      reason?, createdAt, sourceHead?, sourceHeadBefore?, baseHead?,
                      taskHead?, sourcePushed, worktreeRemoved, localBranchRemoved,
                      remoteBranchRemoved, tool_version?, tool_revision?, diagnostics?}
```

Receipt `operation` values: `start`, `finish`, `abort`, `discard`,
`recover`. Receipt `status` values: `ACTIVE`, `BLOCKED`, `DONE`, `ABORTED`
(`READY` never appears in receipts). Note the snake_case outliers
`tool_version`/`tool_revision` inside an otherwise camelCase record.
`performedBy` is discard-only: the actor who actually executed a discard.

Human (non-`--json`) output: first line `<STATUS>[ <task>]: <reason>`, then
`branch:`/`worktree:`/`owner:` lines when a single execution is present,
then the run-list state listing (`executions: none`, per-record lines,
`settled: DONE=n ABORTED=n`), then `warning: …` lines, then
`next: <nextAction>`.

`READY` is a **list-level token only** ("the runtime answered with these
states"), never a per-record status.

______________________________________________________________________

## 3. Storage layout

All runtime state lives under the **git common dir**
(`git rev-parse --git-common-dir`) + `ce/task-runtime/v1/`:

| Path              | Format                                                                                                                       | Writer                                              |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------- |
| `executions.json` | pretty-printed JSON array of `TaskExecution`; atomic replace (temp+fsync+rename+dirsync); missing file reads as `[]`         | start (rewrite), never append-only                  |
| `receipts.jsonl`  | append-only JSONL, one `TaskReceipt` per line; malformed line is a hard read error                                           | start/finish/abort/discard/recover/reconcile        |
| `mutation.lock`   | `O_EXCL`-created file, content `pid=<pid> created=<RFC3339>`; removed by the holder's unlock closure; **never auto-removed** | every mutating verb                                 |
| `board-recovery/` | preserved card copies                                                                                                        | `ce task reconcile` (board verb, out of scope here) |

Locking: mutating verbs (start/finish/abort/discard/recover) hold the lock;
doctor/status/list take none but *observe* it. Under a lock, derived state
is BLOCKED (`repository task runtime mutation is in progress or stale`);
doctor reports `stale or active mutation lock: <path>` with `next: inspect the lock owner; do not delete it automatically`.

Other state under `<common-dir>/ce/` (gate evidence, heartbeat, audit, card
ID reservations) belongs to non-run verbs and is not part of this contract.

______________________________________________________________________

## 4. Identity

Resolution order (first hit wins), all values normalized (lowercase, trim,
`[^a-z0-9]+` → `-`, trim `-`):

| Field | Order                                                                                                                                                                                                                                                                            |
| ----- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| actor | 1. env `CE_TASK_ACTOR` → kind `agent`; 2. env `GIT_WORK_ACTOR` → kind `human`; 3. `defaults.actor`(+`defaults.actor-kind`) in `${XDG_CONFIG_HOME:-$HOME/.config}/ce/identity.yaml`; 4. global git config `ce.workActor` → kind `human`; 5. error `task actor is not configured…` |
| host  | 1. env `GIT_WORK_HOST`; 2. conflict error if identity-file `defaults.host` ≠ global git `ce.workHost` (`device identity source conflict: …`); 3. `defaults.host`; 4. git `ce.workHost`; 5. error                                                                                 |

The run **owner** is the `TaskOwner{actor, host, kind}` stored with the
execution. Owner equality compares actor+host (kind ignored when the stored
kind is empty). Only the recorded owner may finish/abort/recover/discard.

The local identity file is parsed with strict unknown-field rejection;
`schema-version` must be `1`. Fixtures isolate it by pointing
`XDG_CONFIG_HOME` (and `HOME`) at sandbox dirs and setting
`CE_TASK_ACTOR`/`GIT_WORK_HOST` explicitly.

______________________________________________________________________

## 5. Naming and worktree rules

- Branch: `dev/<actor>/<host>/<type>/<task>` — the third segment is the
  execution `type` from `--type` (`feat|fix|…`), **not** the owner kind.
  Owner `kind` (`agent`/`human`) travels in the record only.
- Worktree path: `<worktree-roots.<host>>/<flattened>` where flattened =
  branch without the `dev/` prefix, `/` → `__`:
  `<actor>__<host>__<type>__<task>`. The host segment selects the root, so
  the directory keeps actor+host+type+task and stays collision-free.
- Task slugs and type: positional `<task>` is normalized; `--type` must be
  one of `feat|fix|refactor|docs|test|chore|perf`.
- Source branch: first entry of `.gz-git.yaml` `branch.integrationBranch`;
  fallback `git symbolic-ref --short refs/remotes/origin/HEAD`; else error
  `source branch is not declared and origin/HEAD is unresolved`.

Declaration file `.ce/task-runtime.yaml` (gitignored by design):

```yaml
schema-version: 1                # or 2 (2 requires integration-network-policy)
repository-id: <kebab-segment>   # worktree root leaf must equal this
worktree-roots:
  <host>: /absolute/path         # machine-local, host-keyed
integration-provider: gz-git     # only accepted value
integration-network-policy: allow | no-fetch   # schema 2 only
```

Root safety (`run-doctor` BLOCKED set, ADR-0015): root must be declared for
the owner's host, absolute, not a filesystem root, not the user home, must
end with the repository-id leaf, and must be outside and not beside the
repository. Legacy singular `worktree-root` is a hard error.

External command boundaries:

- run-start → `wt --config-set "worktree-path = \"<path>\"" switch --create --base <source> --no-cd --format=json dev/<actor>/<host>/<type>/<task>`
  (cwd = repo root). The wt JSON answer's `action` is `created` or
  `existing` depending on whether the worktree/branch already existed;
  CE's own `reason` distinguishes `task execution started` vs
  `existing execution returned`, and only the created path carries the wt
  `diagnostics` block (fixture-evidenced).
- inventory (doctor/status/list/finish) → `wt --config-set "list.json-schema = 2" list --format=json`; a non-`{"schema":2,"items":…}`
  answer is BLOCKED (`worktrunk list must return a schema 2 items envelope`).
- run-finish → `gz-git integrate check` then `gz-git integrate run`
  (with `--no-fetch` appended under `integration-network-policy: no-fetch`,
  and only if the provider advertises the `integrate-no-fetch` capability).
- run-discard removal → `wt remove --no-hooks --foreground --format=json -y -D <recorded-worktree-path>`; never `-f`, never a path other than the
  recorded one.

______________________________________________________________________

## 6. Verb semantics

### run-doctor

Read-only. Checks in order, first failure wins (BLOCKED, exit 1): config
validation → identity/host resolution → worktree-root safety → provider
probe (`gz-git --version` must match `^gz-git version [0-9]…` from an
absolute path; `integrate --help` must declare `check` and `run`;
per-op `--help` must declare `--no-fetch`, recorded as capability but
non-blocking) → lock presence → receipt parseability → Worktrunk version
on the accepted minor line (`0.80.x` since CE `b09a605e`) → schema-2 inventory. ACTIVE
reason: `task runtime dependencies are ready`; next:
`ce task run-start <task> --type <type>`; allowed
`[run-start, run-status, run-list]`. Staged-gate/hook-payload checks are a
different verb (`ce task doctor`) and out of scope.

### run-start

Doctor first (its BLOCKED answer is returned verbatim). Requires the task
to be free of a live record; a terminal record is replaced in place. Then:
identity → source branch → create worktree via wt → write `executions.json`
record → append `operation: start, status: ACTIVE` receipt.

- created: `task execution started`, allowed `[run-status]`.
- existing (same owner, live record): `existing execution returned`,
  no wt call, no new receipt.
- other owner's live record: BLOCKED `task is owned by <actor/host>`,
  next `ask the owner to finish the task`.
- invalid DONE receipt on the old record: BLOCKED
  `finish receipt does not prove this execution completed`.

### run-status / run-list

Read-only, no lock. Derive per-record state from record + receipts +
Worktrunk inventory + git, then reconcile (below). Response carries both
`executions[]` (raw history, no status field on purpose) and `states[]`
(derived). `activeCount` counts `states[].status == ACTIVE` — **never**
`len(executions)` (ISSUE-069). List-level status: READY, or BLOCKED with
`one or more task executions are blocked` if any state is BLOCKED. Unknown
task: UNKNOWN `task execution was not found`, exit 1.

Derived `allowedActions`: ACTIVE `[run-status, run-abort]`; BLOCKED
`[run-status, run-abort]` (+ `run-recover` when a partial-cleanup receipt
is present); terminal DONE `[run-status, run-finish, run-abort, run-discard]`; ABORTED `[run-status]` (+ `run-discard` when the discard
intent can be retried).

### run-finish

Single closure path for successful integration (ADR-0028). Explicit task:
lock → derived status must be ACTIVE and `finishReady` → owner check
(`only <actor/host> may finish this task`) → refuse to run inside the
target worktree (`run-finish refuses to remove its current worktree`) → git
preconditions: uncommitted changes — untracked files included, status stays
ACTIVE (`task worktree has uncommitted changes`, next
`commit or remove task-owned changes`; note this string differs from
run-discard's `task worktree is dirty`) — no upstream
(`task branch has no upstream`), unpushed (`task branch must be fully pushed`) → capture heads → provider `integrate check`, then `integrate run`
→ verify the provider evidence (source heads, push success, worktree/local/
remote removal) → append `operation: finish, status: DONE` receipt → DONE,
exit 0, reason `integration and recovery completed`. Incomplete/incorrect
evidence keeps the run BLOCKED (never promoted to DONE). A stale base
(source tip moved past the branch base) exits 1 with the rebase-directed
reason.

**No-arg path — KNOWN DIVERGENCE (ISSUE-069).** With no task argument, the
internal status call returns the *list* answer, and the nil-execution guard
returns exit 1. Fixture-pinned CE behavior with exactly one active run:
the JSON envelope is the full list answer — `status READY`,
`activeCount 1`, `executions[]` and `states[]` both present,
`allowedActions [run-list, run-status]`, reason
`task execution states returned`, next `select a task` — yet the process
exits **1** (stderr `Error: task execution states returned`).

> Port contract (this is the divergence the port must fix): with exactly
> one active run, no-arg `run-finish` answers **READY with exit 0**. The
> fixture `finish-noarg-single-active` pins the CE-actual behavior as
> `referenceBehavior` and the port requirement as `portContract`; the port
> must flip it, not copy it.

### run-abort

Explicit task only (no single-run inference — closure is irreversible,
ADR-0017). Owner-only. Append-only: one `operation: abort, status: ABORTED`
receipt; `executions.json` untouched; worktree/branches untouched (a
remaining worktree is reported as a warning). Already-terminal record:
success no-op, exit 0 (`task execution is already ABORTED: …`). Unknown
task: exit 1.

### run-discard

Guarded non-integration disposal (ADR-0049). Owner-only; a different actor
on the **same host** may take over only with `--take-over-from <actor/host>` spelling the recorded owner exactly — the receipt keeps the
original `owner` and records `performedBy`. `--reason` is mandatory.
Refusals (all BLOCKED, exit 1, exact strings): DONE receipt
(`run-discard refuses an execution with a DONE receipt`), recorded
partial-cleanup receipt, `no-fetch` policy
(`run-discard requires integration-network-policy allow because no-fetch cannot prove remote branch absence`), take-over mismatch
(`only <actor/host> may discard this task; cross-actor takeover requires exact --take-over-from <actor/host> on host <host>`), worktree outside the
configured root, conflicting live execution on the same branch/path, caller
inside the target worktree (`run-discard refuses to remove its current worktree`), Worktrunk inventory mismatch, dirty worktree
(`task worktree is dirty`), branch mismatch, remote branch present
(`task branch still exists on origin`). Sequence: append BLOCKED intent
receipt → `wt remove … -y -D` (no `-f`) → re-verify path/local-branch/
remote-branch absence and inventory → append terminal
`operation: discard, status: ABORTED` receipt with all three removal flags
and `performedBy` → ABORTED, exit 0. Partial removal leaves the BLOCKED
intent and never reports success; retry re-verifies remaining state.
Idempotent re-discard of a closed execution: exit 0
(`task execution is already discarded: <reason>`).

### run-recover

Closes a *proven* exit-3 partial cleanup without removing anything. The
latest receipt must match `operation: finish, status: BLOCKED`, reason
`integration succeeded but task recovery cleanup failed`, with heads, push
flag, and an `integrate run` diagnostic exiting 3 — else
`run-recover requires the recorded exit-3 provider cleanup failure`, exit 1.
Owner-only. Probes (never mutates): source head contains the recorded task
head; source is pushed (`current source branch is not pushed`); residual
worktree/branch/remote detection — **zero residuals is a failure**
(`provider cleanup left no residual path or ref to recover`). Success:
`operation: recover, status: DONE` receipt, exit 0, reason
`integration confirmed after partial provider cleanup; residual <list> left untouched`. A recover DONE receipt only validates as terminal when at
least one residual remains.

### Reconciliation (runs inside every read)

When a record's worktree is gone (`os.Stat` fails), the read path attempts
reconciliation under the mutation lock: if landing evidence proves the task
landed in source (surviving local/remote tip contained in source, or all
refs gone and — under `allow` policy — `ls-remote` confirms remote
absence), an `operation: abort, status: ABORTED` receipt with
`worktreeRemoved: true` and the evidence reason
(`execution <task> closed by CE reconciliation: …`) is appended and the
state becomes ABORTED. Without evidence the state stays BLOCKED
(`Worktrunk inventory does not match execution metadata` /
`execution metadata points to an unavailable worktree`) — never silently
ABORTED. This is why an active run is judged from `states[].status`, not
from the number of executions: an externally reclaimed run keeps its
`executions[]` entry but reports a terminal state and `activeCount` drops.

______________________________________________________________________

## 7. Fixture suite

`tests/parity/` (Go, stdlib only). Modes:

```bash
go test ./tests/parity/...                       # verify: fresh CE runs must match goldens
PARITY_RECORD=1 go test ./tests/parity/...       # record: re-capture goldens from CE
PARITY_TARGET=gz-git go test ./tests/parity/...  # target: same fixtures against the port
```

Target mode (`PARITY_TARGET=gz-git`, or `GZ_GIT_BIN=<path>`) runs the same
fixtures against the port binary instead of CE. Both sides are
canonicalized into one shape before diffing — argv, provider prose,
in-process-provider diagnostics, build-identity folds; the canonicalization
header in `portmode_test.go` is the exhaustive declared divergence surface.
Target mode never records: goldens describe CE. The target build runs with
`GOWORK=off` so a devbox workspace cannot leak another tree's sources into
the binary being judged.

Each scenario builds a throwaway sandbox (git repo on `master` + bare
origin cloned from it + `.ce/task-runtime.yaml` + `.gz-git.yaml`), runs a
fixed sequence of `ce` invocations with a fully pinned environment
(no ambient `CE_TASK_ACTOR`/`GIT_WORK_ACTOR`/`GIT_WORK_HOST`/identity-file
leakage), captures per-step stdout/stderr/exit plus git and record state,
normalizes volatile fields (sandbox paths incl. `~/`-abbreviated forms,
40-hex SHAs by first appearance, timestamps, pids), and diffs the result
against `testdata/golden/<scenario>.json`.

Golden files are machine-captured reference behavior of the pinned CE
toolchain. Toolchain upgrades (ce/wt/gz-git) intentionally require
re-recording — that is the pin doing its job. Known-divergent scenarios
carry `knownDivergent: true` plus a `portContract` block in the golden
itself, and the suite asserts the divergence semantically
(`assertContract`): if a later CE build fixes ISSUE-069 or breaks the
reconciliation contract, the marker goes stale and the suite fails with
instructions to re-evaluate — the fix requirement is metadata, never an
edit to captured bytes.
