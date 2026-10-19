# Readiness resource plan (plan-v1)

A readiness contract can let its target-owned runner classify each check run
as light or high, instead of a static repository-wide `resourceHighLoad`:

```yaml
branch:
  readiness:
    version: 1
    runner: .gz-git/readiness/check
    resourcePlan: plan-v1
```

`resourcePlan` is part of the contract digest. A source-only change fails
closed like any other contract change and is adopted through
[readiness update](integrate-readiness-update.md) or first bootstrap. Omitting
the field keeps the existing canonical contract, digest and runner arguments.
The only accepted value is `plan-v1`; empty, null, non-string, duplicate or
unknown values fail closed.

## Sequence

1. The contract's preparation profile, if any, runs first under its own
   `prepareHighLoad` rules.
2. gz-git runs the runner once without the host slot, with the usual V1
   arguments except `--result-format plan-v1`. The plan invocation has a fixed
   2-minute budget and the same environment and worktrees as the execution.
3. The runner prints exactly one object:

   ```json
   {"version":1,"resource":"light","plan_digest":"<64 lowercase hex>","summary":"..."}
   ```

   `resource` is `light` or `high`. Unknown or missing fields, any other value,
   a nonzero exit, oversized output or a timeout make the check `unavailable`
   before the execution starts.
4. The slot is taken when the plan is `high` **or** either committed side
   declares `branch.resourceHighLoad: true`. A plan never removes a static
   declaration's slot.
5. gz-git runs the execution with the V1 arguments plus
   `--resource-class <granted> --plan-digest <digest>`, where `granted` is
   `high` if the slot is held and `light` otherwise.

## Runner responsibilities

gz-git does not interpret repository paths or check names. The runner owns the
selection and must keep the plan and the execution bound to the same input:

- Derive the plan only from the arguments and the prepared worktrees, and
  compute `plan_digest` over the selected check set (for example SHA-256 of a
  canonical selection list including source and target SHAs).
- On execution, recompute the selection. If its digest differs from
  `--plan-digest`, or it needs `high` while `--resource-class` is `light`,
  report `unavailable` instead of running. A plan that escalates to a full run
  must therefore have been reported as `high` so the slot is held first.
- Treat a missing binding as a non-plan invocation; gz-git always passes it
  when `resourcePlan: plan-v1` is declared.

Only lower `branch.resourceHighLoad` in a separate commit after the planned
runner has been verified on the target; the static flag remains the
fail-safe while both are true.

## Reporting

The human check report adds `resource plan — Requested=… Granted=…
PlanDigest=… Duration=…` and the provenance line carries `ResourceClass`.
`integrate run --json` adds `plan_duration_ms`, `preparation_duration_ms`,
`integration_duration_ms` (always present) and `resource_class` (contract mode
only), next to `resource_wait_duration_ms` and `validation_duration_ms`. Plan
time is neither validation nor resource wait. Preparation time excludes its
slot wait. Integration time spans revalidation, the target push and local
target fast-forward; reclaim and the target-lock wait are outside it.
