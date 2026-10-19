---
id: ISSUE-67
title: "Resource plan fail-closed paths lack end-to-end tests"
type: bug
priority: P3
severity: low
discovered-in: "post-merge review of cf5c6acb"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

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
