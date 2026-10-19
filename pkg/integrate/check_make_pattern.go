// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"os"
	"os/exec"
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
// entry, which states outright whether make had to search implicit rules to
// satisfy it: "Implicit rule search has been done." means a pattern rule
// supplied the recipe, "has not been done." means the Makefile declared one.
//
// The neighboring "Implicit/static pattern stem:" line is not a usable signal.
// It is printed with an empty stem for some declared targets and omitted
// entirely for others, so its presence says nothing.
//
// -n alone does not make the probe read-only. GNU make still runs recipe
// lines that name $(MAKE) or start with +, and $(shell ...) while parsing, so
// a gate graph with a recursive line would run part of itself on every probe.
// SHELL is overridden on the command line with true: make hands every recipe
// line and $(shell) call to it, and true runs nothing. A command-line variable
// outranks the Makefile's own SHELL assignment unless that one uses override.
// The database dump, and its implicit-search verdict, are unchanged.
//
// ok is false when the database could not be read at all — make or true
// missing, no makefile, a parse error. The caller then keeps its previous
// behavior rather than treating an unreadable database as proof of absence.
func makeTargetMatchedByPatternRule(ctx context.Context, dir, target string) (matched, ok bool) {
	noop, err := exec.LookPath("true")
	if err != nil {
		return false, false
	}
	cmd := exec.CommandContext(ctx, "make", "-pn", "SHELL="+noop, target) // #nosec G204 -- validated against allowedMakeTargets by the caller
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "MAKELEVEL=0", "MAKEFLAGS=", "LC_ALL=C")
	cmd.Env = hostSlotsHeldForChild(ctx, cmd.Env)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	// The exit status is deliberately ignored. -p prints the database even when
	// the target cannot be made, and that dump is the entire point of the call.
	_ = cmd.Run() //nolint:errcheck // intentional: -p dumps the rule database regardless of exit status

	lines := strings.Split(buf.String(), "\n")
	header := target + ":"
	for i, line := range lines {
		// Entries begin at column zero; recipe lines are tab-indented, so a
		// recipe that echoes "check:" cannot be mistaken for an entry.
		if line != header && !strings.HasPrefix(line, header+" ") {
			continue
		}
		for _, entry := range lines[i+1:] {
			if !strings.HasPrefix(entry, "#") {
				break
			}
			// "has not been done." must not match; check it first.
			if strings.HasPrefix(entry, "#  Implicit rule search has not been done.") {
				return false, true
			}
			if strings.HasPrefix(entry, "#  Implicit rule search has been done.") {
				return true, true
			}
		}
		// The entry exists but never states the verdict. Keep the caller's
		// previous behavior rather than guessing from an unfamiliar dump.
		return false, true
	}
	return false, false
}
