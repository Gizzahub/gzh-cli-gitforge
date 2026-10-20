// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

// TestRecipelessMakeTargetIsDeclared pins ISSUE-75 end to end: an aggregate
// check with no recipe and no .PHONY line is a declared gate, so it runs and
// its prerequisite's recipe runs with it.
func TestRecipelessMakeTargetIsDeclared(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "lint-ran")
	writeRepoFile(t, dir, "Makefile", "check: lint\nlint:\n\t@touch "+marker+"\n")

	check := runMakeTarget(context.Background(), dir, "check", 0)
	if !check.Defined || check.Err != nil {
		t.Fatalf("recipe-less check must be Defined and pass (err=%v)\n%s", check.Err, check.Output)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("check's prerequisite did not run: %v", err)
	}
}

// TestRulelessPathNamedLikeTargetIsUndeclared keeps a file or directory named
// check from passing a gate nothing declares: make prints "Nothing to be
// done" and exits 0 for it, never "No rule to make target". A declared
// recipe-less check beside the same file stays declared.
func TestRulelessPathNamedLikeTargetIsUndeclared(t *testing.T) {
	for _, tc := range []struct {
		name     string
		makefile string
		dir      bool
		defined  bool
	}{
		{name: "file", makefile: "lint:\n\t@:\n"},
		{name: "directory", makefile: "lint:\n\t@:\n", dir: true},
		{name: "file beside a declared recipe-less check", makefile: "check:\nlint:\n\t@:\n", defined: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRepoFile(t, dir, "Makefile", tc.makefile)
			if tc.dir {
				if err := os.Mkdir(filepath.Join(dir, "check"), 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				writeRepoFile(t, dir, "check", "")
			}
			if check := runMakeTarget(context.Background(), dir, "check", 0); check.Defined != tc.defined {
				t.Fatalf("Defined = %v, want %v (err=%v)\n%s", check.Defined, tc.defined, check.Err, check.Output)
			}
		})
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
		genOlder *bool // gen.mk exists, and is older than gen.in when true
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
		// ISSUE-75: make searches implicit rules for a declared target that
		// has no recipe and is not .PHONY. Such a search leaves no stem and
		// no "# Not a target:" line, so these stay declared.
		{
			name:     "recipe-less aggregate check",
			makefile: "check: lint\nlint:\n\t@:\n",
			matched:  false, ok: true,
		},
		{
			name:     "recipe-less check without prerequisites",
			makefile: "check:\nlint:\n\t@:\n",
			matched:  false, ok: true,
		},
		{
			name:     "recipe-less aggregate check beside a .DEFAULT rule",
			makefile: "check: lint\nlint:\n\t@:\n.DEFAULT:\n\t@:\n",
			matched:  false, ok: true,
		},
		// Every catch-all form still leaves a stem: .DEFAULT an empty one.
		{
			name:     ".DEFAULT satisfies check",
			makefile: "lint:\n\t@:\n.DEFAULT:\n\t@:\n",
			matched:  true, ok: true,
		},
		{
			name:     "empty-recipe catch-all satisfies check",
			makefile: "lint:\n\t@:\n%: ;\n",
			matched:  true, ok: true,
		},
		// Known limit: a catch-all that supplies the recipe of a declared
		// recipe-less target reads as a pattern match, because the stem is
		// all the dump records.
		{
			name:     "catch-all supplies the recipe of a recipe-less check",
			makefile: "check: lint\nlint:\n\t@:\n%:\n\t@:\n",
			matched:  true, ok: true,
		},
		// make exits 0 here: the guard sends the remake to true and the
		// catch-all satisfies check. The real run generates gen.mk first and
		// finds check declared there.
		{
			name:     "generated include with a catch-all",
			makefile: "include gen.mk\ngen.mk:\n\techo 'check:' > gen.mk\nlint:\n\t@:\n%:\n\t@:\n",
			matched:  true, ok: false,
		},
		{
			name:     "optional generated include with a catch-all",
			makefile: "-include gen.mk\ngen.mk:\n\techo 'check:' > gen.mk\nlint:\n\t@:\n%:\n\t@:\n",
			matched:  true, ok: false,
		},
		{
			name:     "existing include with a catch-all",
			makefile: "include lint.mk\nlint.mk:\n\t@:\n%:\n\t@:\n",
			matched:  true, ok: true,
		},
		// gen.mk exists but is older than gen.in: the real run regenerates it
		// and finds check declared there.
		{
			name:     "out-of-date generated include with a catch-all",
			makefile: "-include gen.mk\ngen.mk: gen.in\n\techo 'check:' > gen.mk\nlint:\n\t@:\n%:\n\t@:\n",
			genOlder: ptrTo(true),
			matched:  true, ok: false,
		},
		{
			name:     "out-of-date generated include after parse-time output that mimics the trace",
			makefile: "$(info Updating goal targets)\n-include gen.mk\ngen.mk: gen.in\n\techo 'check:' > gen.mk\nlint:\n\t@:\n%:\n\t@:\n",
			genOlder: ptrTo(true),
			matched:  true, ok: false,
		},
		{
			name:     "up-to-date generated include with a catch-all",
			makefile: "-include gen.mk\ngen.mk: gen.in\n\techo 'check:' > gen.mk\nlint:\n\t@:\n%:\n\t@:\n",
			genOlder: ptrTo(false),
			matched:  true, ok: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRepoFile(t, dir, "Makefile", tc.makefile)
			writeRepoFile(t, dir, "lint.mk", "lint:\n\t@:\n")
			const genBefore = "lint2:\n"
			if tc.genOlder != nil {
				writeRepoFile(t, dir, "gen.mk", genBefore)
				writeRepoFile(t, dir, "gen.in", "")
				older, newer := time.Now().Add(-time.Hour), time.Now()
				if !*tc.genOlder {
					older, newer = newer, older
				}
				for name, at := range map[string]time.Time{"gen.mk": older, "gen.in": newer} {
					if err := os.Chtimes(filepath.Join(dir, name), at, at); err != nil {
						t.Fatal(err)
					}
				}
			}
			matched, ok := makeTargetMatchedByPatternRule(context.Background(), dir, "check")
			if matched != tc.matched || ok != tc.ok {
				t.Fatalf("matched=%v ok=%v, want matched=%v ok=%v", matched, ok, tc.matched, tc.ok)
			}
			got, err := os.ReadFile(filepath.Join(dir, "gen.mk"))
			if tc.genOlder == nil && !os.IsNotExist(err) || tc.genOlder != nil && string(got) != genBefore {
				t.Fatalf("probe ran the include's remake recipe: gen.mk = %q, err = %v", got, err)
			}
		})
	}
}

func ptrTo[T any](v T) *T { return &v }

// TestMakePatternProbeLoaded pins that the probe file counts as loaded only
// when MAKEFILE_LIST names its exact path, as make 3.81 ("MAKEFILE_LIST :=  a")
// and 4.x print it, and not an expansion of it.
func TestMakePatternProbeLoaded(t *testing.T) {
	for _, tc := range []struct {
		dump string
		want bool
	}{
		{dump: "MAKEFILE_LIST :=  /t/probe.mk Makefile", want: true},
		{dump: "x\nMAKEFILE_LIST := /t/probe.mk Makefile gen.mk", want: true},
		{dump: "MAKEFILE_LIST := /t/pro.mk Makefile", want: false},
		{dump: "MAKEFILE_LIST := /t/probe.mk.old Makefile", want: false},
		{dump: "Makefile", want: false},
	} {
		if got := probeLoaded(strings.Split(tc.dump, "\n"), "/t/probe.mk"); got != tc.want {
			t.Errorf("probeLoaded(%q) = %v, want %v", tc.dump, got, tc.want)
		}
	}
}

// TestMakePatternProbeKeepsCallerMakefiles pins that the caller's MAKEFILES
// is read after the probe's guard, as the real run reads it: a check declared
// there is declared, not a catch-all match.
func TestMakePatternProbeKeepsCallerMakefiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the probe needs make and true on PATH")
	}
	dir := t.TempDir()
	writeRepoFile(t, dir, "Makefile", "lint:\n\t@:\n%:\n\t@:\n")
	writeRepoFile(t, dir, "extra.mk", "check:\n\t@:\n")
	t.Setenv("MAKEFILES", filepath.Join(dir, "extra.mk"))
	if matched, ok := makeTargetMatchedByPatternRule(context.Background(), dir, "check"); matched || !ok {
		t.Fatalf("matched=%v ok=%v, want the check from the caller's MAKEFILES declared", matched, ok)
	}
}

// TestMakePatternProbeTempFile pins the probe file's lifetime and path: it is
// removed after the probe, and a path MAKEFILES would split on whitespace or
// expand is neither run, so the catch-all's + line never runs unguarded, nor
// given a verdict.
func TestMakePatternProbeTempFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the probe needs make and true on PATH")
	}
	dir := t.TempDir()
	writeRepoFile(t, dir, "Makefile", "lint:\n\t@:\n%:\n\t+touch RAN\n")
	for _, tc := range []struct {
		name    string
		tmp     string
		matched bool
		ok      bool
	}{
		{name: "plain temp dir", tmp: "plain", matched: true, ok: true},
		{name: "temp dir with a space", tmp: "with space", matched: false, ok: false},
		{name: "temp dir with a dollar", tmp: "td$x", matched: false, ok: false},
		{name: "temp dir with a wildcard", tmp: "td*", matched: false, ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := filepath.Join(t.TempDir(), tc.tmp)
			if err := os.Mkdir(tmp, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TMPDIR", tmp)
			if matched, ok := makeTargetMatchedByPatternRule(context.Background(), dir, "check"); matched != tc.matched || ok != tc.ok {
				t.Fatalf("matched=%v ok=%v, want matched=%v ok=%v", matched, ok, tc.matched, tc.ok)
			}
			left, err := os.ReadDir(tmp)
			if err != nil || len(left) != 0 {
				t.Fatalf("probe file left behind: %v, %v", left, err)
			}
			if _, err := os.Stat(filepath.Join(dir, "RAN")); !os.IsNotExist(err) {
				t.Fatalf("probe ran the catch-all's + line: stat err = %v", err)
			}
		})
	}
}
