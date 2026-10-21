# CE `task run-*` Lifecycle Parity Contract

Reference for porting the CE run lifecycle into gz-git, per ce-agent-kit
[ADR-0055](https://github.com/Gizzahub/ce-agent-kit/blob/master/decisions/adr/0055-task-manager-owns-tasks-gz-git-owns-lifecycle.md)
("task-manager owns tasks, gz-git owns the repository lifecycle, no compatibility
window") and the `run-lifecycle` row of the CE migration ledger
(`ce-agent-kit/docs/00-product/11-task-migration-ledger.md`).

The current behavioral reference is **CE source `ce-agent-kit`
`c6d5bf842d6adae7bd93327ee0fa0d13d49ca584`**, installed as CE 0.10.14 with
`vcs.modified=false`. In addition to CE's release/build stamps, the suite verifies
the full revision and clean state from Go executable metadata. Fixtures must
not claim a reference revision different from the executable they capture.

Earlier references, including the `f540f972` remote-tracking source change
that the port does not follow yet (ISSUE-42), are recorded in
[RUN_LIFECYCLE_PARITY_ADOPTIONS.md](RUN_LIFECYCLE_PARITY_ADOPTIONS.md). The current capture also records purpose fields, explicit resume
refusals, owner-scoped bare finish, and observational queries (CE ADR-0073).
The reference suite is not proof that every verb has been ported. Target-mode
drift remains tracked under ISSUE-42; settlement/close verbs and complete
purpose/resume/receipt parity are separate port work. No normalization hides
these differences.
The behavioral reference is
the fixture suite in `tests/parity/` — golden files there are
machine-captured from the CE binary, never hand-written. This document
describes the same surface at the source level; when the two disagree, the
fixtures win.

Reference toolchain used by the fixtures:

| Component        | Version                                                  | Role                             |
| ---------------- | -------------------------------------------------------- | -------------------------------- |
| `ce`             | 0.10.14 (`c6d5bf84`, clean Go build metadata)            | system under test (reference)    |
| `wt` (Worktrunk) | 0.80.0 (CE accepts the 0.80 minor line since `b09a605e`) | worktree create/remove/inventory |
| `gz-git`         | test-built current checkout (`GOWORK=off`)               | integration provider (reclaim)   |
| `git`            | system git                                               | refs, worktree plumbing          |

Historical provider-only diagnostic snapshots in `doctor-active`, `discard-no-fetch`,
and `finish-explicit` were recaptured after the official source installation of
`96d5ecd569c3aaf8ad5cfd0b2dcc922fe63c4859`. The CE reference revision and
lifecycle state/ref/cleanup assertions remain unchanged. Human diagnostic
`Duration=` values are normalized; additional readiness diagnostics and the
canonical executable path are retained. Other scenarios retained their prior
capture at that checkpoint because their normalized observations were unchanged.

On 2026-10-14 all twelve scenarios were freshly machine-captured with clean CE
`9877764d` and the designated `make install` provider `1503edf`. The current
master harness retains its narrow provider-version normalization and approved
diagnostic help updates. Capture and fresh verification each passed twelve
scenarios with no skips; both executable hashes were unchanged across these runs.
The CE checkout subsequently contains only a documentation commit `3e4340e5`
above the selected executable revision; no CE code or installed settings were
changed for this capture. Evidence is retained in the source primary checkout's
`tmp/ce-reference-stable-adoption/`. This reference adoption does not complete
the native port or ADR-0117 acceptance.

The final TASK-327 candidate captures the installed clean CE `7c22bfb4`
reference without adding `processHost` normalization. Its original normalizer
retains physical host values in the two historical FINISHING receipts. Those
fields are machine-sensitive: captures on a different physical host can differ.
They are not diagnostic-only fields; CE uses physical host equality before
interpreting a claim PID and conservatively retains foreign-host claims.
The existing finish scenarios record completed histories, not a live two-host
claim regression. This adoption does not claim that additional coverage.
The frozen `d9e9677` candidate and its earlier evidence remain unchanged.
New capture, verification and final-candidate logs are recorded independently.

ISSUE-303's provider-capture follow-up uses the checkout build in CE reference
mode as well as target mode. Each CE sandbox owns a `bin/gz-git` symlink to
that build and prepends only that directory to the child process's PATH.
The toolchain version probe reads the same build directly. No installed binary,
parent process PATH, runtime configuration or physical-host value is changed.
The sandbox path uses the existing `<SANDBOX>` normalization; no new semantic
normalization is introduced. A hostile ambient `gz-git` regression verifies
both CE's actual provider resolution and the direct toolchain probe.

In the earlier 2026-10-16 installed-provider capture, all twelve scenarios were freshly machine-captured from the
designated clean CE 0.10.8 installation `05438e10` and provider `1503edf`.
Capture and fresh verification each passed twelve scenarios with no skips;
both executable SHA256 values were unchanged before and after. Evidence is in
the source primary checkout's `tmp/verification-profiles-20261016/`.
The original normalizer is unchanged, including physical `processHost` fields.
Besides reference stamps, the finish captures include the new remote-source
push proof described below. The two FINISHING receipts also change physical
`processHost` from `CEui-MacBookPro.local` to `CE-Mac-Studio.local`, reflecting
the capture host; this provenance remains visible without normalization.
This reference adoption does not attest a native
port, baseline Make execution, or consumer acceptance.

After rebasing onto the checkout-provider change `43f94e8`, the prior capture
uses the merged checkout build with `GOWORK=off` and the same clean CE `05438e10`.
The first full record run rejected the old `917e236a` golden pins before writing
the twelve replacement captures; its exit 1 is retained, not accepted as PASS.
With those machine-generated inputs updated, full record and separate full
non-record verification passed all twelve scenarios with zero skips in 21.763s
and 21.737s. The latter includes the hostile-ambient provider regression.
The CE executable hash remained unchanged. This is checkout-provider evidence,
not a capture of the installed provider. Logs are
`parity-capture-rebased.log`, `parity-capture-rebased-current.log`, and
`parity-verify-rebased.log` in the same primary evidence directory.

### CE legacy-cleanup provider reference refresh

The legacy-cleanup provider follow-up selects clean CE `9fe1ce45` / `0.10.9`
(`233-g9fe1ce45`) while preserving the checkout-provider binding introduced by
ISSUE-303. Its capture and verification use the merged checkout provider, not
an installed provider. The existing normalizer and hostile-ambient regression
remain unchanged from the adopted source contract.

The selected reference copy has SHA-256
`0f4268204b2f08cd2bcd41a4664bd848b906f81e71dab388a95e391b1093cf2c`.
The designated CLI-only installation of the same clean revision has SHA-256
`beebe3c4a47d701599e25c8eaf2ce7633bdcf02e13d4d4e098ebb01b74861aae`.
Earlier pre-rebase installed-provider captures remain historical evidence only;
they do not establish the merged checkout-provider behavior. After preserving
the adopted checkout-provider implementation, targeted capture freshly recorded
all twelve scenarios in 14.089s. Full non-record verification passed in 13.371s,
including the hostile-ambient provider regression and capture pin checks. Both
runs used the same clean installed CE revision and unchanged executable hash.
Evidence is retained in the source primary's
`tmp/issue331-ce-provider-proof-20261016/`.

Governing CE ADRs: 0006 (superseded original contract), 0015 (lifecycle
bypass prohibition + root safety), 0017 (abort verb, append-only closure),
0026 (provider adapter boundary only), 0028 (finish owns DONE evidence),
0049 (guarded discard), 0055 (ownership transfer to gz-git).

______________________________________________________________________

## 1. Verb surface

The tables below describe the inherited port surface. Current CE also has
purpose/resume and explicit settlement/closure contracts beyond these eight
verbs; their remaining port differences are tracked in ISSUE-42. Current
machine captures and the owner/observation rules below are authoritative for
the updated scenarios, rather than a claim that the entire CE CLI is ported.

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
| `ce task run-finish`  | `gz-git run finish`    | `--json`                                 | native no-arg reports readiness; explicit task performs completion          |
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
The provider's `gz-git version …` build string is represented as
`gz-git version <GZ_GIT_VERSION>` in fixture metadata and diagnostics. This
removes workstation-specific provider identity only; provider presence,
capabilities, command/help diagnostics, exit codes, and receipts remain exact
parity inputs.

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
| `receipts.jsonl`  | append-only JSONL, one `TaskReceipt` per line; malformed line is a hard read error                                           | start/finish/abort/discard/recover                  |
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
Worktrunk inventory + git, without implicit closure (below). Response carries both
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

CE's post-operation source-push proof under network policy `allow` queries
`git ls-remote --exit-code --heads origin refs/heads/<source>`, validates the
exact returned ref/OID, and requires that commit to exist locally with
`git cat-file -e <oid>^{commit}`. It checks whether a local source branch exists
and, when present, requires `git rev-list --count <oid>..refs/heads/<source>`
to be zero. The local source branch may be absent or behind the proven remote
tip. Under `no-fetch`, the proof uses the local remote-tracking ref instead;
the allow-path remote query is not evidence of the no-fetch branch. Captured
finish success proves the allow-path sequence, not every refusal branch.
Proof errors distinguish missing/transport failures, malformed or wrong-ref
responses, unavailable local commits, local-ref probe errors, and a local branch
ahead of the proven remote. Finish exposes these as
`post-operation evidence failed: <stage error>`.

**No-arg path.** Native bare finish is a readiness query. Exactly one ACTIVE
delivery owned by the current actor and host may return the aggregate
READY/exit 0/task empty/activeCount 1 document. It does not invoke integration,
write a finish receipt, or remove the execution's worktree or local/remote refs.
Zero or several owned matches return task-empty BLOCKED with exit 1; several
matches name sorted task IDs and explicit finish commands. Foreign, terminal,
and experiment records cannot become implicit completion targets. A non-ready
aggregate returns its refusal with exit 1. Explicit task finish still performs
integration and reclaim.

CE deliberately differs: bare finish selects the sole owned ACTIVE delivery
and performs normal completion, returning DONE and reclaiming its resources
when ready. The user selected restoration of the native READY contract in
`gzh-cli-devbox:decisions/003-current-parity-successor.md` (TASK-322).
The twelve reference scenarios retain ambiguity and reclamation safety tests.
`finish-noarg-single-active` captures a committed, pushed, finish-ready run:
CE DONE is verified with reclaimed resources, while native READY is verified
against the unchanged pre-finish Git and execution/receipt snapshot. This case
and `reclaimed-run-reconciliation` use explicit semantic validation instead of
CE byte comparison. The reclaimed case verifies failed external cleanup,
retained execution identity and history, exact absent resources, and matching
BLOCKED observations. Other target scenarios retain their byte comparisons;
passing these two cases does not claim full native twelve-scenario equality.

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

Closes a proven exit-3 partial cleanup or a bound native completion without
removing anything. For the partial-cleanup path, the
latest receipt must match `operation: finish, status: BLOCKED`, reason
`integration succeeded but task recovery cleanup failed`, with heads, push
flag, and an `integrate run` diagnostic exiting 3 — else
`run-recover requires the recorded exit-3 provider cleanup failure, a bound native completion, or an interrupted run-finish`, exit 1.
Owner-only. Probes (never mutates): the same policy-dependent source-push proof
as finish runs first for partial cleanup (errors are wrapped as `resolve current source head: ...`),
then the proven source head must contain the recorded task head; residual
worktree/branch/remote detection — **zero residuals is a failure**
(`provider cleanup left no residual path or ref to recover`). Success:
`operation: recover, status: DONE` receipt, exit 0, reason
`integration confirmed after partial provider cleanup; residual <list> left untouched`. A recover DONE receipt only validates as terminal when at
least one residual remains for that partial-cleanup path.

If source-push proof fails, CE returns BLOCKED with
`partial cleanup recovery is not proven: resolve current source head: <stage error>`.
The response contains the failed probe diagnostics and a transient BLOCKED
recovery receipt, which is not appended to history. The twelve-scenario
`recover-requires-cleanup-failure` capture rejects an ineligible prior receipt
before this proof, so it does not cover the source-proof failure response.

A separate native-completion path accepts one exact, successful, context-bound
`integrate run` diagnostic from the latest owner-matching BLOCKED finish receipt.
It binds the absolute provider executable, SHA-256, argv, execution/generation,
owner, task SHA, target ref and complete JSON push/cleanup outcome. It rechecks
that the recorded worktree (including dangling symlinks) and local task branch
are absent and that the local source contains the task head. It does not rerun
the provider, access the network, or mutate refs. Failure is BLOCKED with
`native completion recovery is not proven: <cause>`; success records a DONE
recover receipt with reason `native provider completion confirmed from durable receipt`
and all push/cleanup flags true. This is the zero-residual exception to the
partial-cleanup rule above; terminal validation retains the bound proof.
The twelve reference scenarios still cover only recovery refusal, not this new
success path. Its positive, forged-proof, residual-resource, dangling-link and
non-ancestor cases are verified in the owning CE taskruntime unit suite.

### Observation and explicit closure

CE ADR-0073 separates read queries from execution closure. If the worktree or
inventory disappears, status/list retain the execution history as BLOCKED,
with `activeCount: 0`. They append no terminal receipt, take no mutation lock,
and infer neither DONE nor ABORTED from resource absence. The fixture asserts
the retained record and unchanged start receipt in addition to byte comparison.

The port also keeps read derivation observational. CE's explicit `run-settle`
and `run-close` surfaces remain unported; the existing owner-only `run abort`
is still an explicit closure action. This change does not claim full target
parity or treat an unavailable worktree as proof of successful integration.

______________________________________________________________________

## 7. Fixture suite

The current clean `c6d5bf84` reference (`412-gc6d5bf84`) replaces `f88f4331`
together with the readiness contract removal. CE's non-test changes in that
range (`access*.go`, `task_cleanup.go`, `task_lint.go`) are off the captured
paths. Normalized for CE identity, ten fixtures are byte-identical and two
(discard-no-fetch, doctor-active) differ only in captured `integrate --help`
text. Exits, status, ownership, refs, push and cleanup are unchanged.
Evidence is in `reports/2026-10-21-ce-parity-reference-c6d5bf84.md`.

Adopting a reference requires a machine capture of all twelve scenarios, fresh
non-record verification, and independent comparison of the complete diff.
Earlier references and their scope are recorded in
[RUN_LIFECYCLE_PARITY_ADOPTIONS.md](RUN_LIFECYCLE_PARITY_ADOPTIONS.md); a new
adoption replaces the paragraph above and moves the old one there.

`tests/parity/` (Go, stdlib only) is built only with `-tags=parity`. It runs the
installed `ce`, so it is not part of `make check`, `make quick` or `go test ./...`:
another CE install must not fail an unrelated gitforge change. Modes:

```bash
make test-parity                                                  # verify: fresh CE runs must match goldens
PARITY_RECORD=1 go test -tags=parity ./tests/parity/... -run '^TestCEParity$' # capture all 12 CE scenarios
PARITY_TARGET=gz-git go test -tags=parity ./tests/parity/...      # target: same fixtures against the port
```

After changing the CE pin, capture the twelve scenarios first, then run the full
non-record verification. The full suite checks existing golden pins before the
scenario writer and also runs the hostile-ambient provider regression; the
targeted capture is generation, not a replacement for that verification.

Target mode (`PARITY_TARGET=gz-git`, or `GZ_GIT_BIN=<path>`) runs the same
fixtures against the port binary instead of CE. Both sides are
canonicalized into one shape before diffing — argv, provider prose,
in-process-provider diagnostics, build-identity folds; the canonicalization
header in `portmode_test.go` is the exhaustive declared divergence surface.
Target mode never records: goldens describe CE. The target build runs with
`GOWORK=off` so a devbox workspace cannot leak another tree's sources into
the binary being judged.

CE reference mode also builds that checkout once and exposes it as the
fixture-local provider described above. `GZ_GIT_BIN` retains its existing
target-mode meaning; it is not a CE provider override. Missing builds or CE
capabilities fail the fixture rather than falling back to an installed provider.

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
re-recording when `make test-parity` is run — that is the pin doing its job.
Adopting a new CE reference is a deliberate step, not a side effect of
another repository deploying CE. A behavioral change is accepted
only after the scenario's explicit contract is updated and the CE capture
passes; fixture bytes are never hand-edited to hide a divergence.
