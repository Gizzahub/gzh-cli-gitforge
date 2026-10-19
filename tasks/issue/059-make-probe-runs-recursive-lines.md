---
id: ISSUE-59
title: "Pattern-rule probe executes recursive make lines"
type: bug
priority: P2
severity: medium
discovered-in: "release-gate-ci review: make -n executes recipe lines naming $(MAKE)"
discovered-at: 2026-10-19
created: 2026-10-19
---

## Summary

`makeTargetMatchedByPatternRule` (`pkg/integrate/check_make.go:470`)
treats `make -pn <target>` as a read-only database read, but GNU make
still executes recipe lines that name `$(MAKE)`/`${MAKE}` or start with
`+` under `-n`. Any target repository whose gate graph contains such a
line has those lines run by every integrate probe, and the exit status is
ignored (`check_make.go:477`), so the side effects stay silent. In this
repository `check` → `verify-full` → `$(MAKE) quality-check` ran pieces of
`test-install-atomic` on every probe from f02041dc until the release-gate
fix routed that recipe through a variable.

## Reproduction

1. In a scratch repository, declare `check:` with a recipe line
   `$(MAKE) side-effect` where `side-effect:` touches a file.
1. Run any `gz-git integrate check` that reaches the pattern-rule probe.
1. The file is created although the probe is documented as read-only.

## Expected vs Actual

- Expected: the probe reads the rule database without executing any
  recipe of the target repository.
- Actual: recursive and `+` recipe lines run with the probe's environment.

## Scope

- Read the database without executing any recipe while keeping the
  implicit-search signal the function relies on. `-q` and `-t` are not a
  fix: GNU make runs `$(MAKE)` and `+` lines under them too, and the
  "Implicit rule search" annotation only appears for targets make
  considered. Prove the choice with a recursive-line fixture.

## Resolution

Resolved on 2026-10-20.

- The probe (moved to `pkg/integrate/check_make_pattern.go`) keeps
  `make -pn <target>` and adds `SHELL=<path of true>` on the command
  line. make hands every recipe line, including `$(MAKE)` and `+` lines,
  and every `$(shell ...)` call to `SHELL`, and `true` runs none of
  them. The database dump and its "Implicit rule search" verdict are
  unchanged, so the existing pattern-rule detection keeps its signal.
- A command-line variable outranks the Makefile's own `SHELL`
  assignment, target-specific ones included; only `override SHELL`
  defeats it. When `true` is not on `PATH` the probe reports the
  database as unreadable and the caller keeps its previous behavior.
- Checked by hand on GNU Make 3.81 (macOS) and 4.4.1 (golang:1.27
  image): with the override no marker file appears and the verdicts
  are `not been done` for a declared target and `has been done` for a
  catch-all one; without it the `+` and `$(shell)` markers appear.
- Verified by `TestMakePatternProbeRunsNoRecipe` (a declared target
  with `$(MAKE)` and `+` lines, a `+` catch-all, and a `$(shell)`
  assignment; asserts both verdicts and no marker). Replacing the
  override with a no-op flag makes it fail on the `shell`, `plus` and
  `catchall` markers. `TestMakeTargetMatchedByPatternRuleIsUndeclared`
  moved to `check_make_pattern_test.go` unchanged.
