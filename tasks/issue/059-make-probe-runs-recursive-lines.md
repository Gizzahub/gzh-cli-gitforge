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
- A generated include with a catch-all (`include gen.mk` or
  `-include gen.mk`, a `gen.mk:` rule that writes `check:`, and `%:`)
  exits 0 under the guard: the remake goes to `true`, gen.mk never
  exists, and the catch-all satisfies `check`. Re-review found that this
  reported `SKIP make check: undefined` where master ran the check. The
  same holds when gen.mk exists but is older than its prerequisites: the
  real run regenerates it. The probe runs `make --debug=b,m -pn`, and
  any "Must remake target" line between "Updating makefiles" and
  "Updating goal targets" means make remade a makefile under the guard,
  so a pattern-rule verdict is `ok=false`. An up-to-date include, or an
  existing one whose rule has no prerequisites, adds no line there,
  because the real run does not remake it either, and keeps its verdict.
- The probe file goes first in `MAKEFILES` and the caller's own value
  follows, so a target declared there stays declared. make splits
  `MAKEFILES` on whitespace and expands `$` and wildcards in it, so a
  probe or `true` path containing any of ` \t\n$*?[` is not run
  (`ok=false`), and a dump whose `MAKEFILE_LIST` lacks the probe path is
  `ok=false` too. No other character tried (`\ # % : ( ' " ~ = ; | & { ,`)
  changes the path on 3.81 or 4.4.1, so that second check is a backstop
  with no reachable test input; `probeLoaded` itself is unit-tested.
- Known limits: a target-specific `SHELL` in the Makefile outranks the
  pattern-specific value, so that target's recursive and `+` lines
  still run under `-n`, as before this fix. When `true` is not on
  `PATH` or the temporary file cannot be written, the probe reports the
  database as unreadable and the caller keeps its previous behavior,
  which reports a target a catch-all satisfies as PASS. A catch-all with
  file-less prerequisites (`%: force`) remakes every makefile, so its
  verdict is `ok=false` and the real run decides. The same holds where
  the catch-all or an include rule's prerequisites remake a makefile the
  real run leaves unchanged: an optional include of a missing file with
  no rule of its own (`-include .env`, `-include deps.d`, which the
  catch-all "remakes"), an include rule whose only missing prerequisite
  is order-only (`gen.mk: | gdir`), and `.PHONY: gen.mk` on 3.81. Those
  shapes get no verdict, and the caller keeps its previous PASS.
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
- Re-review follow-up, passing on GNU Make 3.81 and 4.4.1:
  `TestMakePatternProbeParsesLikeTheRealRun` adds the `include` and
  `-include` generated cases (`ok=false`, gen.mk never written) and an
  existing include (`ok=true`); `TestMakePatternProbeKeepsCallerMakefiles`
  declares `check` in the caller's `MAKEFILES`;
  `TestMakePatternProbeTempFile` asserts the probe file is removed and a
  `TMPDIR` with a space is `ok=false`. Mutations, each reverted: ignoring
  the remade missing file fails both generated cases, dropping the
  caller's `MAKEFILES` fails its test, dropping the whitespace check
  fails the space case, skipping the removal fails the temp-file test,
  and counting the goal itself as a remade file fails
  `TestMakePatternProbeRunsNoRecipe`.
- Second re-review follow-up, passing on GNU Make 3.81 and 4.4.1: the
  dump's "File does not exist." signal above missed an out-of-date
  include and is replaced by the `--debug=m` trace.
  `TestMakePatternProbeParsesLikeTheRealRun` adds an out-of-date
  generated include (`ok=false`, gen.mk unchanged) and an up-to-date one
  (`ok=true`); `TestMakePatternProbeTempFile` adds `TMPDIR` with `$` and
  `*` and asserts the catch-all's `+touch` line never runs;
  `TestMakePatternProbeLoaded` pins the `MAKEFILE_LIST` match on both
  dump formats. Mutations, each reverted and each caught: ignoring the
  trace, dropping `m` from `--debug`, matching the probe path as a
  substring, dropping `$` or the wildcards from the pre-check, and
  reading the trace past "Updating goal targets". Dropping the
  `probeLoaded` call survives, as the pre-check covers every case.
- Third re-review (PASS with warnings, 24 shapes on both versions,
  including caller-`MAKEFILES`, nested, pattern-rule, double-colon and
  subdirectory generated includes) found that a parse-time
  `$(info Updating goal targets)` ended the trace before it began. The
  terminator now counts only inside the trace; the table adds that
  case, and accepting the terminator outside the trace fails it.
