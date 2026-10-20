---
id: ISSUE-75
title: "A declared make target without a recipe reads as undeclared"
type: bug
priority: P2
severity: medium
discovered-in: "ISSUE-59 re-review"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`makeTargetMatchedByPatternRule` (`pkg/integrate/check_make_pattern.go`)
treats "Implicit rule search has been done." as proof that a pattern rule
supplied the target. GNU make also searches implicit rules for a declared
target that has no recipe and is not `.PHONY`, such as an aggregate
`check: lint test`. The probe then reports the target as undeclared, and
the integration gate prints `SKIP make check: undefined` for a gate that
exists. The behavior predates ISSUE-59: the guard does not change make's
implicit search.

## Reproduction

1. Write a Makefile with `check: lint` and `lint:` followed by a `@:`
   recipe, with no `.PHONY` line.
1. Call `runMakeTarget(ctx, dir, "check", 0)`: `Defined` is false and the
   probe returns `matched=true ok=true` (GNU Make 3.81 and 4.4.1).
1. Add `.PHONY: check lint`: `Defined` is true.

## Expected vs Actual

- Expected: a target the Makefile declares reads as declared, with or
  without a recipe or `.PHONY`.
- Actual: a recipe-less, non-phony declared target reads as matched by a
  pattern rule and is skipped as undefined.

## Scope

- Tell a declared recipe-less target from a pattern match in the dump,
  for example by the entry's origin line or by whether a pattern rule
  supplied a recipe, on both make versions. Add the recipe-less case to
  `TestMakePatternProbeParsesLikeTheRealRun`.
