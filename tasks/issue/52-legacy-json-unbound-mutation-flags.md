# ISSUE-052: Legacy JSON reports unbound mutation flags as false

- status: open
- priority: P2
- category: lifecycle/evidence
- created_at: 2026-10-16
- owner: `gzh-cli-gitforge/pkg/integrate/protocol.go`
- implementation-grade: strong
- review-grade: strong (independent)

## Reproduction and evidence

The designated clean provider `9227bf3` (0.10.1) integrated infra-ops task
`b7ad3178524ff6ac13e3345d3a5e2477162f9689` into `master` with exit 0.
The invocation supplied `--expect-source`, `--expect-target`, and `--json`,
without an execution context. Its JSON reports `integrated: true` and all
three `reclaim_done` entries, but `source_pushed`, `worktree_removed`,
`local_branch_removed`, and `remote_branch_removed` are all false.

Independent Git/remote/filesystem checks prove the remote master moved from
`19f690b5` to `b7ad3178` and the task worktree, local branch, remote branch,
and native preparation root were removed. Raw output remains unchanged.
Evidence belongs to `infra-ops:tmp/verification-profiles-20261016/`:
`native-adoption.json`, `native-adoption-inputs.json`,
`native-adoption-verification.json`, and `native-adoption-dva-receipt.json`.

`BuildRunProtocol` copies these booleans only when `RunReport.Receipt` is
non-nil. Successful no-context legacy runs have no bound durable receipt,
so the serialized Go defaults conflate absent proof with a false outcome.
This is distinct from ISSUE-050's interrupted legacy refusal reconciliation.

## Follow-up boundary

Define compatible semantics for unbound receipt fields and add a successful
no-context regression that checks them against integration/reclaim results.
Do not treat these four booleans alone as side-effect proof for such runs.
The actual effects of this invocation were independently verified, so this
finding does not invalidate its acceptance. Registration does not authorize
a public API/schema change or runtime redesign in the Python/UV delivery.
