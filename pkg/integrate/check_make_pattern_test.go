// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMakeTargetMatchedByPatternRuleIsUndeclared pins the gitforge/gzh-cli
// Makefile shape: a catch-all pattern rule guarding "make run <arg>", with no
// check target declared. Before the fix, make exited 0 having done nothing and
// the probe was marked Defined, so the integration gate reported
// "PASS make check" for a gate that did not exist.
func TestMakeTargetMatchedByPatternRuleIsUndeclared(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "catchall-ran")
	writeRepoFile(t, dir, "Makefile",
		"lint:\n\t@:\n\n"+
			"# Prevent make from interpreting arguments as targets\n"+
			"%:\n\t@touch "+marker+"\n")

	check := runMakeTarget(context.Background(), dir, "check", 0)
	if check.Defined {
		t.Fatalf("check is satisfied only by the catch-all pattern rule; Defined must be false (err=%v)\n%s", check.Err, check.Output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("undeclared target must not be run at all; catch-all recipe executed, stat err = %v", err)
	}

	// A declared target in the same Makefile stays declared: the detector must
	// key on the pattern-rule stem, not on the presence of a catch-all.
	lint := runMakeTarget(context.Background(), dir, "lint", 0)
	if !lint.Defined {
		t.Fatalf("declared lint target must stay Defined (err=%v)\n%s", lint.Err, lint.Output)
	}

	// Without the catch-all, the pre-existing output-string detection must keep
	// working — the new probe must not mask the "No rule to make target" path.
	plain := t.TempDir()
	writeRepoFile(t, plain, "Makefile", "lint:\n\t@:\n")
	if bare := runMakeTarget(context.Background(), plain, "check", 0); bare.Defined {
		t.Fatalf("missing check target must not be Defined\n%s", bare.Output)
	}
}

// TestMakePatternProbeRunsNoRecipe pins ISSUE-59: make -n still runs recipe
// lines that name $(MAKE) or start with +. The probe must read the database
// without running any of them, for a declared target and for one a catch-all
// satisfies, and keep its verdict for both, whatever SHELL the Makefile sets
// globally. $(shell) runs as it would for the real make that follows.
func TestMakePatternProbeRunsNoRecipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the probe needs make and true on PATH")
	}
	dir := t.TempDir()
	marker := func(name string) string { return filepath.Join(dir, name) }
	writeRepoFile(t, dir, "Makefile",
		"SHELL := /bin/sh\n"+
			"PARSED := $(shell touch "+marker("shell-ran")+")\n"+
			"lint:\n\ttouch "+marker("make-ran")+" && $(MAKE) side\n\t+touch "+marker("plus-ran")+"\n\n"+
			"side:\n\ttouch "+marker("side-ran")+"\n\n"+
			"%:\n\t+touch "+marker("catchall-ran")+"\n")

	for _, tc := range []struct {
		target  string
		matched bool
	}{
		{target: "lint", matched: false},
		{target: "check", matched: true},
	} {
		matched, ok := makeTargetMatchedByPatternRule(context.Background(), dir, tc.target)
		if !ok || matched != tc.matched {
			t.Fatalf("%s: matched=%v ok=%v, want matched=%v ok=true", tc.target, matched, ok, tc.matched)
		}
	}
	for _, name := range []string{"make-ran", "plus-ran", "side-ran", "catchall-ran"} {
		if _, err := os.Stat(marker(name)); !os.IsNotExist(err) {
			t.Errorf("probe ran a recipe: %s exists (stat err = %v)", name, err)
		}
	}
	if _, err := os.Stat(marker("shell-ran")); err != nil {
		t.Errorf("$(shell) must keep the Makefile's SHELL while parsing: %v", err)
	}
}

// TestMakePatternProbeParsesLikeTheRealRun pins the verdicts that depend on
// $(shell) and on make's exit status. A target declared only when a $(shell)
// condition holds stays declared; a catch-all repository whose parse hinges on
// $(shell) still reads as pattern-matched; and a pattern match from a failed
// make (an include it cannot remake) is no verdict at all.
func TestMakePatternProbeParsesLikeTheRealRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the probe needs make and true on PATH")
	}
	for _, tc := range []struct {
		name     string
		makefile string
		matched  bool
		ok       bool
	}{
		{
			name:     "check declared under a shell condition, with a catch-all",
			makefile: "ifneq ($(shell echo yes),)\ncheck:\n\t@:\nendif\nlint:\n\t@:\n%:\n\t@:\n",
			matched:  false, ok: true,
		},
		{
			name:     "shell-guarded error, catch-all satisfies check",
			makefile: "ifeq ($(shell echo yes),)\n$(error unreachable)\nendif\nlint:\n\t@:\n%:\n\t@:\n",
			matched:  true, ok: true,
		},
		{
			name:     "include whose remake recipe does not run",
			makefile: "include gen.mk\ngen.mk:\n\techo 'check:' > gen.mk\nlint:\n\t@:\n",
			matched:  true, ok: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRepoFile(t, dir, "Makefile", tc.makefile)
			matched, ok := makeTargetMatchedByPatternRule(context.Background(), dir, "check")
			if matched != tc.matched || ok != tc.ok {
				t.Fatalf("matched=%v ok=%v, want matched=%v ok=%v", matched, ok, tc.matched, tc.ok)
			}
		})
	}
}
