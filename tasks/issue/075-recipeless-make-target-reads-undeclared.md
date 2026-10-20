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

## Resolution

Resolved on 2026-10-20.

- The probe reads a searched entry as a pattern match only when an
  `Implicit/static pattern stem:` line follows `Implicit rule search has been done.`. GNU Make 3.81 and 4.4.1 both record that line only for a
  rule that matched: the `%:` catch-all with a recipe, `%: ;`, `%::` and
  `%: force` leave a stem, and `.DEFAULT` leaves an empty one. A declared
  recipe-less target (`check: lint`, a bare `check:`, `check:: lint`) and
  an undeclared target with no rule leave none, so they return
  `matched=false ok=true` and the real run tells them apart through
  `No rule to make target`.
- `TestMakePatternProbeParsesLikeTheRealRun` gains the recipe-less cases,
  `.DEFAULT` and `%: ;`. The included-makefile case whose remake recipe
  does not run now reads as no match (`false, true`): no rule matched
  `check`, and the real run still generates `gen.mk` and finds it there.
- `TestRecipelessMakeTargetIsDeclared` runs `check: lint` end to end:
  `Defined` is true and the prerequisite's recipe runs.
- Known limit, unchanged from before: a catch-all that supplies the recipe
  of a declared recipe-less target still reads as a pattern match, since
  the stem is all the dump records. That case is pinned as such.
- Mutation evidence, reverted before commit: treating `has been done.` as
  a match again, dropping the `searched` guard on the stem line, dropping
  the run qualification of a stem verdict, and returning no verdict for a
  searched entry without a stem each fail the probe tests.
