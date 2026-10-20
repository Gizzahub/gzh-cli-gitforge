// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// makeTargetMatchedByPatternRule reports whether make would satisfy target
// through a pattern rule rather than a rule the Makefile declares for it.
//
// Exit status cannot answer this, and isUndefinedMakeTarget below inherits that
// blindness: it reads the output for "No rule to make target", which make never
// prints when a pattern rule matched. A repository whose Makefile ends with the
// common argument guard
//
//	%:
//		@:
//
// — there so "make run foo" does not treat foo as a target — silently satisfies
// every target it does not declare. `make check` exits 0 having run nothing and
// printed nothing, the probe is marked Defined, and the gate reports
// "PASS make check". Two repositories in this family carry that catch-all and
// declare no check target, so their integration gate has been lint-only for its
// whole life without ever saying so.
//
// The rule database does answer it. `make -pn <target>` prints the target's
// entry, which states whether make searched implicit rules for it and, when
// one matched, what it matched. "Implicit rule search has not been done."
// means the Makefile declared a recipe. "has been done." alone does not mean
// a pattern rule supplied one: make also searches for a declared target with
// no recipe that is not .PHONY, such as an aggregate `check: lint test`, and
// for one nothing declares. Only a rule that matched records the
// "Implicit/static pattern stem:" line below it, with an empty stem for
// .DEFAULT, so a searched entry is a pattern match only with that line.
//
// Read on its own, the stem line says nothing: declared targets print it
// with an empty stem when they have a recipe and omit it when they do not.
// A catch-all that supplies the recipe of a declared recipe-less target
// still reads as a pattern match, since the stem is all the dump records.
//
// -n alone does not make the probe read-only. GNU make still runs recipe
// lines that name $(MAKE) or start with +, and the recipes that remake an
// included makefile, so a gate graph with a recursive line would run part of
// itself on every probe. The probe therefore loads, through MAKEFILES, a
// pattern-specific `%: SHELL := <path of true>`: every target's recipe lines
// go to true, which runs none of them. $(shell ...) keeps the Makefile's own
// SHELL, because a Makefile that chooses its targets with it must parse
// exactly as the real `make <target>` that follows, which evaluates the same
// calls anyway. A command-line SHELL would empty those calls and flip the
// verdict, dropping a declared check from the report.
//
// A pattern-specific value outranks the Makefile's global SHELL but not a
// target-specific one, so a target that sets its own SHELL still runs its
// recursive and + lines under -n, as before this guard. The probe goes first
// in MAKEFILES and the caller's own entries follow, so the parse matches the
// real run. make splits MAKEFILES on whitespace and expands $ and wildcards
// in it, so a probe path containing any of them is not run, and a dump whose
// MAKEFILE_LIST lacks the probe path gives no verdict: either way there was
// no guard.
//
// When a pattern rule matched, the dump may still not describe the real run,
// so ok is false and the real run decides through "No rule to make target":
//   - make failed (an included makefile it could not remake, a missing target
//     without a catch-all), so the dump may come from a partial parse;
//   - make remade a makefile before reading the goal, as its --debug=m trace
//     states. The guard sent that recipe to true, so an included makefile
//     that is missing or older than its prerequisites was not generated; the
//     real run generates it first and may find the target declared there. A
//     catch-all with a prerequisite, such as `%: force`, remakes every
//     makefile this way and so never gets a verdict.
//
// ok is also false when the database could not be read at all — make or
// true missing, the probe file not writable, no makefile, a parse error. The
// caller then keeps its previous behavior, which reports a target a catch-all
// satisfies as PASS, rather than treating an unreadable database as proof of
// absence.
func makeTargetMatchedByPatternRule(ctx context.Context, dir, target string) (matched, ok bool) {
	noop, err := exec.LookPath("true")
	if err != nil {
		return false, false
	}
	probeFile, err := os.CreateTemp("", "gz-git-make-probe-*.mk")
	if err != nil {
		return false, false
	}
	defer func() { _ = os.Remove(probeFile.Name()) }()
	_, err = probeFile.WriteString("%: SHELL := " + noop + "\n")
	if closeErr := probeFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil || strings.ContainsAny(probeFile.Name()+noop, " \t\n$*?[") {
		return false, false
	}
	makefiles := strings.TrimSpace(probeFile.Name() + " " + os.Getenv("MAKEFILES"))
	cmd := exec.CommandContext(ctx, "make", "--debug=b,m", "-pn", target) // #nosec G204 -- validated against allowedMakeTargets by the caller
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "MAKELEVEL=0", "MAKEFLAGS=", "MAKEFILES="+makefiles, "LC_ALL=C")
	cmd.Env = hostSlotsHeldForChild(ctx, cmd.Env)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	// -p prints the database even when the target cannot be made, and that
	// dump is the entire point of the call; the exit status only qualifies a
	// pattern-rule verdict below.
	runErr := cmd.Run()

	lines := strings.Split(buf.String(), "\n")
	if !probeLoaded(lines, probeFile.Name()) {
		return false, false
	}
	header := target + ":"
	for i, line := range lines {
		// Entries begin at column zero; recipe lines are tab-indented, so a
		// recipe that echoes "check:" cannot be mistaken for an entry.
		if line != header && !strings.HasPrefix(line, header+" ") {
			continue
		}
		searched := false
		for _, entry := range lines[i+1:] {
			if !strings.HasPrefix(entry, "#") {
				break
			}
			// "has not been done." must not match; check it first.
			if strings.HasPrefix(entry, "#  Implicit rule search has not been done.") {
				return false, true
			}
			if strings.HasPrefix(entry, "#  Implicit rule search has been done.") {
				searched = true
				continue
			}
			if searched && strings.HasPrefix(entry, "#  Implicit/static pattern stem:") {
				return true, runErr == nil && !remadeMakefile(lines)
			}
		}
		// A search that left no stem found no rule: the target is declared
		// without a recipe, or not declared at all, and the real run tells the
		// two apart through "No rule to make target". An entry that never
		// states the verdict keeps the caller's previous behavior the same way
		// rather than guessing from an unfamiliar dump.
		return false, true
	}
	return false, false
}

// probeLoaded reports whether the dump's MAKEFILE_LIST names the probe file,
// that is, whether make read the guard rather than an expansion of its path.
func probeLoaded(lines []string, probe string) bool {
	for _, line := range lines {
		if list, found := strings.CutPrefix(line, "MAKEFILE_LIST :="); found {
			return slices.Contains(strings.Fields(list), probe)
		}
	}
	return false
}

// remadeMakefile reports whether the --debug=m trace, which make prints
// between "Updating makefiles" and "Updating goal targets", has make remake a
// makefile or one of its prerequisites. Make 3.81 and 4.x both print it, and
// an up-to-date makefile adds no line there.
func remadeMakefile(lines []string) bool {
	updating := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "Updating makefiles"):
			updating = true
		// Parse-time output such as $(info) comes first on the same stream,
		// so only a terminator inside the trace ends it.
		case updating && strings.HasPrefix(line, "Updating goal targets"):
			return false
		case updating && strings.HasPrefix(strings.TrimLeft(line, " "), "Must remake target "):
			return true
		}
	}
	return false
}
