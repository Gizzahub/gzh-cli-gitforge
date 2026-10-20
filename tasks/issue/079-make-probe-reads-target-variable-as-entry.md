---
id: ISSUE-79
title: "The make probe reads a target-specific variable line as the entry"
type: bug
priority: P3
severity: low
discovered-in: "ISSUE-75 re-review"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`makeTargetMatchedByPatternRule` (`pkg/integrate/check_make_pattern.go`)
takes the first dump line equal to `check:` or starting with `check: ` as
the target's entry. GNU make 4.4.1 prints a target-specific variable such
as `check: FOO = 1` before the entry, so the probe stops on that line,
finds no comment below it, and returns a declared verdict. A file named
`check` with no rule then passes the gate having run nothing, where make
3.81 reports `SKIP make check: undefined`. origin/master behaves the
same, so this predates ISSUE-75.

## Reproduction

1. Write a Makefile with `check: FOO=1` and `lint:` followed by a `@:`
   recipe line, and create an empty file named `check` beside it.
1. Run the probe under GNU make 4.4.1: it reports Defined, and
   `make check` prints `Nothing to be done for 'check'`.
1. Under GNU make 3.81 the same Makefile reports undefined.

By the same reading, a target-specific variable beside a catch-all `%:`
is expected to let an undeclared `check` pass silently on 4.4.1; that
combination was not run.

## Expected vs Actual

- Expected: a file named `check` that only a target-specific variable
  names reports `SKIP make check: undefined` on make 3.81 and 4.4.1.
- Actual: make 4.4.1 reads the variable line as the entry and the gate
  reports PASS having run nothing.

## Scope

- Take as the entry only a header followed by the entry's `#` comment
  block, so a target-specific variable line is skipped.
- Cover a file named `check` with only a target-specific variable, and
  the same beside a catch-all, on make 3.81 and 4.4.1.
