---
id: ISSUE-68
title: "Resource plan fail-closed paths lack end-to-end tests"
type: bug
priority: P3
severity: low
discovered-in: "post-merge review of cf5c6acb"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

Registered as ISSUE-67 in 49fe5547, which collided with the CHANGELOG
card landed concurrently in 7b630a23; renumbered to ISSUE-68.

`pkg/integrate/readiness_plan_test.go` covers the resource plan end to end
only for an invalid digest, plus parser cases. The code fails closed on the
other paths (reviewed), but no test pins that behavior, so a regression
would pass the suite.

## Reproduction

1. Make any path below succeed instead of failing closed (for example,
   ignore a nonzero plan exit).
1. Run `GOWORK=off go test ./pkg/integrate/...`: it still passes.

## Untested paths

- nonzero exit from the plan invocation
- plan output over the size limit
- plan timeout: `readinessPlanTimeout` is a constant, so a test cannot
  shorten it
- duplicate keys in the plan JSON
- duplicate `resourcePlan` in the JSON contract form (YAML form is tested)
- a `high` plan combined with the static `resourceHighLoad` flag
- ND build time no longer counted in `ValidationDuration`

## Expected vs Actual

- Expected: each path has a table case asserting `unavailable` before the
  execution starts (or the stated duration split).
- Actual: the paths are verified only by code reading.

## Scope

- Make the plan timeout injectable the same way the execution timeout is,
  then add the table cases. No behavior change.

## Resolution

Resolved on 2026-10-20.

- Injection seam: the plan budget is now a parameter like the
  execution budget. `planReadinessResources` takes `timeout`, and
  `selectReadinessResource`, `runContractPair` and
  `runContractWithTimeoutResource` pass `planTimeout` down. Every
  production caller passes `readinessPlanTimeout` (2 minutes), so
  behavior is unchanged.
- `TestReadinessResourcePlanFailuresFailClosed`
  (`pkg/integrate/readiness_plan_failclosed_test.go`) runs real
  worktrees and a real runner per case. Each case asserts `unavailable`,
  its reason, a plan record and no execution record:
  - nonzero exit: `resource plan: runner failed: plan broke`
  - output over 1 MiB: `resource plan: runner emitted invalid plan JSON`
  - timeout at an injected 1s budget: `resource plan: runner timed out`,
    with a `contract_plan` timeout failure at that budget
  - duplicate `resource` keys (high, then light):
    `resource plan: runner result has duplicate or invalid fields`
- The oversize case fails closed through the parser's 1 MiB bound, not
  through `runner output exceeded limit`. `limitedBuffer` embeds
  `bytes.Buffer`, so the exec copy uses its promoted `ReadFrom` and never
  calls the limiting `Write`. A 1 MiB + 1 byte runner output reaches the
  parser in full. This defect is ISSUE-70 and is not fixed here. The
  oversized output is a valid light plan padded with whitespace, so
  dropping every size bound would start the execution and fail the test.
- `TestReadinessResourcePlanDuplicateJSONContractFailsClosed`: a
  `.gz-git.json` contract with `resourcePlan` repeated fails the
  readiness contract item with
  `.gz-git.json: branch.readiness must be an object`, and neither the
  plan nor the execution runs.
- `TestReadinessResourcePlanSelectsSlot` adds a row for a `high` plan
  combined with static `resourceHighLoad`. The execution holds the slot
  exactly once (`integrate`) and is bound to `high`.
- `TestNDBuildTimeIsPreparationNotValidation` runs a planned ND contract
  through `Check` with six 100ms builds. It asserts at least 600ms of
  preparation, measured plan and validation durations, and validation
  not exceeding the execution span.
- Mutation evidence, each reverted before commit:
  - ignoring a nonzero plan exit fails the nonzero exit case (execution ran)
  - dropping timeout classification for the plan stage fails the timeout
    case (wrong reason)
  - ignoring the injected plan budget fails the timeout case (execution
    ran after 30s)
  - accepting duplicate result keys fails the duplicate key case
    (execution ran)
  - parsing the JSON contract without the duplicate check fails the JSON
    contract test
  - XOR in place of OR for the slot decision fails the new slot row
  - counting ND build time as validation again fails the ND test
    (validation 770ms against a 5ms execution span)
