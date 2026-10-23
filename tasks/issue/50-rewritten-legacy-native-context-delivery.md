---
id: ISSUE-50
title: "Rewritten task heads cannot reconcile the original legacy native refusal"
type: bug
priority: P2
severity: medium
discovered-in: "CE sibling-evidence-delivery finish"
discovered-at: 2026-10-14
created: 2026-10-14
---

## Summary

CE cannot reconcile a legacy native refusal once the task branch has been
rewritten past the captured head, so the retained `sibling-evidence-delivery`
run can be neither finished nor reclaimed through a supported path.

- category: lifecycle/recovery
- owner: ce-agent-kit — `internal/usecase/taskruntime/legacy_context_reconcile.go`

## Evidence

The retained CE delivery `sibling-evidence-delivery` has a BLOCKED finish receipt
capturing task head `2f9d6f059e01667dd84c4505eb907be48b0c4047`. Its sole native
run used the clean b323 artifact SHA-256
`36bd85e8b21d6b0aaecb2f33d55f98bbae462b0bf4d7472142c6a9bd23a77e20` and returned
exit 1 with the exact old JSON `error: not ready`, empty execution identity and
explicit false integration/push/cleanup fields.

The task branch ref had already been rewritten from the captured `2f9d6f0`
to `9100919` before this recovery work, and was rebased again during this work.
The latest preserved, pushed documentation head is
`7be186f604f44c53b344b816b9c68c8a0b952b54`, under
`dev/codex/mst/docs/sibling-evidence-delivery`. The retained worktree is
`/Users/archmagece/myworktree/gzh-cli/gzh-cli-gitforge/codex__mst__docs__sibling-evidence-delivery`.
Current CE status still refuses finish because the original native outcome is
uncertain. The original receipt is unchanged and the earlier commit objects
remain addressable. This task did not reset the branch back to the captured SHA.

The explicit version-bound reconciliation contract requires the current clean,
pushed worktree HEAD to equal the captured original task SHA. This retained run
does not meet that condition. This is an unsupported recovery case, not proof
of a new push failure, completed cleanup, or a regression in the new contract.

## Reproduction

On the device holding the retained worktree (mst), run
`ce task run-status sibling-evidence-delivery` from the gitforge primary
checkout. Derived from the receipts above; not re-run from this device.

## Expected vs Actual

- Expected: CE offers a supported way to reconcile the uncertain original
  native outcome, or to clean up the retained run explicitly, after the task
  branch was rewritten.
- Actual: status keeps refusing finish because the original outcome is
  uncertain, and reconciliation requires the current HEAD to equal the
  captured SHA, which a rewritten branch can no longer meet.

## Scope and follow-up

Documentation delivery is tracked by the separate official
`sibling-evidence-resumed-delivery` run. That does not settle or reclaim the
original execution. Keep its worktree, local/remote branch and receipt until the
CE owner provides a supported evidence reconciliation or explicit retained-run
cleanup path. Do not reset the branch to manufacture the required HEAD, edit
runtime state, weaken the identity check, or use raw Git cleanup.

This registration does not authorize extending the migration allowlist or the
CE recovery contract. ISSUE-47's interrupted detached baseline worktree is a
different resource and recovery case.

Raw status evidence: machine-local
`/Users/archmagece/mycache/tmp/gzh-cli-quality.5wDebO/gzh-docs-current-status.json`.
