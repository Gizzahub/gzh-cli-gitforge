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
fix routed that recipe through a variable; the recursive `quality-check`
sub-make itself still ran on every probe. Recipes that remake an included
makefile run under `-n` as well.

## Reproduction

1. In a scratch repository, declare `check:` with a recipe line
   `touch marker && $(MAKE) side`. The side effect must sit on the
   recursive line itself: the sub-make inherits `-n` and only prints.
1. Run any `gz-git integrate check` that reaches the pattern-rule probe.
1. `marker` is created although the probe is documented as read-only.

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
  `make -pn <target>` and loads, through `MAKEFILES`, a temporary file
  holding the pattern-specific `%: SHELL := <path of true>`. Every
  target's recipe lines, `$(MAKE)` and `+` lines and makefile remakes
  included, go to `true`, which runs none of them.
- `$(shell ...)` keeps the Makefile's own `SHELL`, so a Makefile that
  declares targets under a `$(shell)` condition parses as it does for
  the real `make <target>` that follows (which evaluates the same calls).
  A first version overrode `SHELL` on the command line; review showed
  that it empties `$(shell)` and flips such verdicts, dropping a
  declared `check` from the report or, with a catch-all, restoring a
  false `PASS make check`.
- A pattern-rule verdict from a make that exited non-zero (an include it
  could not remake, a missing target without a catch-all) is no verdict:
  the probe returns `ok=false` and the real run decides through "No rule
  to make target".
- Known limits: a target-specific `SHELL` in the Makefile outranks the
  pattern-specific value, so that target's recursive and `+` lines
  still run under `-n`, as before this fix. When `true` is not on
  `PATH` or the temporary file cannot be written, the probe reports the
  database as unreadable and the caller keeps its previous behavior,
  which reports a target a catch-all satisfies as PASS.
- Checked by hand on GNU Make 3.81 (macOS) and 4.4.1 (golang:1.27
  image): no recipe marker appears; the `$(shell)`-conditional `check`
  reads as declared with and without a catch-all; a catch-all repository
  whose parse hinges on `$(shell)` reads as pattern-matched; the include
  case exits 2.
- Verified by `TestMakePatternProbeRunsNoRecipe` (a global
  `SHELL := /bin/sh`, a `touch && $(MAKE)` line, a `+` line, a `+`
  catch-all; asserts both verdicts, no recipe marker, and that
  `$(shell)` ran) and `TestMakePatternProbeParsesLikeTheRealRun` (the
  three `$(shell)` and exit-status shapes). Mutations: the command-line
  override fails both tests, dropping the `MAKEFILES` guard fails the
  recipe markers and the include case, and ignoring the exit status
  fails the include case. `TestMakeTargetMatchedByPatternRuleIsUndeclared`
  moved to `check_make_pattern_test.go` unchanged.
