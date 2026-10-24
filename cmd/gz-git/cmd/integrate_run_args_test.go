// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

var (
	fullSHA1   = strings.Repeat("a", 40)
	fullSHA256 = strings.Repeat("b", 64)
)

// resetIntegrateRunFlags returns every run flag to its default, Changed
// included, so a test can parse a fresh argument list; the returned func
// does the same on cleanup.
func resetIntegrateRunFlags(t *testing.T) {
	t.Helper()
	reset := func() {
		integrateRunCmd.Flags().VisitAll(func(f *pflag.Flag) {
			if err := f.Value.Set(f.DefValue); err != nil {
				t.Fatalf("reset --%s: %v", f.Name, err)
			}
			f.Changed = false
		})
	}
	reset()
	origQuiet := quiet
	t.Cleanup(func() {
		reset()
		quiet = origQuiet
	})
}

func parseIntegrateRunArgs(t *testing.T, flags ...string) (integrateRunArgs, error) {
	t.Helper()
	resetIntegrateRunFlags(t)
	if err := integrateRunCmd.ParseFlags(flags); err != nil {
		t.Fatalf("ParseFlags(%q) = %v", flags, err)
	}
	return integrateRunArgsFromFlags(integrateRunCmd)
}

func TestIntegrateRunNewArgsMapOntoLegacyFields(t *testing.T) {
	newForm, err := parseIntegrateRunArgs(t,
		"--source", fullSHA1, "--target", "master@"+fullSHA256,
		"--record", "run-42@g7", "--owner", "claude/mbp/agent")
	if err != nil {
		t.Fatalf("new arguments: %v", err)
	}
	legacy, err := parseIntegrateRunArgs(t,
		"--expect-source", fullSHA1, "--target", "master", "--expect-target", fullSHA256,
		"--execution-id", "run-42", "--generation", "g7",
		"--owner-actor", "claude", "--owner-host", "mbp", "--owner-kind", "agent")
	if err != nil {
		t.Fatalf("legacy arguments: %v", err)
	}
	if newForm != legacy {
		t.Fatalf("new arguments = %+v, want the legacy resolution %+v", newForm, legacy)
	}
}

// The engine lowercases expected SHAs, so an uppercase pin is a pin too; the
// engine, not the parser, normalizes it.
func TestIntegrateRunPinnedTargetAcceptsUppercaseSHA(t *testing.T) {
	upper := strings.ToUpper(fullSHA1)
	got, err := parseIntegrateRunArgs(t, "--target", "master@"+upper)
	if err != nil {
		t.Fatalf("uppercase pin: %v", err)
	}
	if got.Target != "master" || got.ExpectTarget != upper {
		t.Fatalf("uppercase pin = %+v, want target master pinned at %s", got, upper)
	}
	if _, err := parseIntegrateRunArgs(t, "--target", "master@ABC1234"); err == nil {
		t.Fatal("uppercase short pin: error = nil, want truncated-SHA refusal")
	}
}

func TestIntegrateRunTargetStaysAPlainRefWithoutAFullSHA(t *testing.T) {
	for _, target := range []string{"origin/main", "release@2026", "fix@cafe", "user@host/topic", "main@"} {
		got, err := parseIntegrateRunArgs(t, "--target", target)
		if err != nil {
			t.Fatalf("--target %q: %v", target, err)
		}
		if got.Target != target || got.ExpectTarget != "" {
			t.Fatalf("--target %q = %+v, want a plain ref with no pinned commit", target, got)
		}
	}
}

func TestIntegrateRunNewArgsRefuseMalformedValues(t *testing.T) {
	cases := map[string][]string{
		"short SHA after @":        {"--target", "master@abc1234"},
		"empty ref before @":       {"--target", "@" + fullSHA1},
		"empty source":             {"--source="},
		"record without @":         {"--record", "run-42"},
		"record without id":        {"--record", "@g7"},
		"record without gen":       {"--record", "run-42@"},
		"owner with two parts":     {"--owner", "claude/mbp"},
		"owner with an empty part": {"--owner", "claude//agent"},
		"owner with four parts":    {"--owner", "claude/mbp/agent/x"},
	}
	for name, flags := range cases {
		if _, err := parseIntegrateRunArgs(t, flags...); err == nil {
			t.Errorf("%s (%q): error = nil, want refusal", name, flags)
		}
	}
}

func TestIntegrateRunNewArgsAndTheirLegacyFlagsAreMutuallyExclusive(t *testing.T) {
	cases := map[string][]string{
		"source":  {"--source", fullSHA1, "--expect-source", fullSHA1},
		"target":  {"--target", "master@" + fullSHA1, "--expect-target", fullSHA1},
		"record":  {"--record", "run-42@g7", "--generation", "g7"},
		"owner":   {"--owner", "claude/mbp/agent", "--owner-kind", "agent"},
		"record2": {"--record", "run-42@g7", "--execution-id", "run-42"},
	}
	for name, flags := range cases {
		_, err := parseIntegrateRunArgs(t, flags...)
		if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("%s (%q): error = %v, want mutual exclusion", name, flags, err)
		}
	}
}

func TestIntegrateRunNewArgsKeepTheContextAllOrNothing(t *testing.T) {
	args, err := parseIntegrateRunArgs(t, "--record", "run-42@g7", "--source", fullSHA1)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, _, err := executionContextFromArgs(args); err == nil ||
		!strings.Contains(err.Error(), "supplied together or not at all") {
		t.Fatalf("record without owner: error = %v, want all-or-nothing refusal", err)
	}
}

// integrateRunNewArgsFixture is a pushed task branch over an integration
// target, both with remote tips, for end-to-end runs with the new arguments.
func integrateRunNewArgsFixture(t *testing.T) (fx *testutil.WorktreeOrigin, target string) {
	t.Helper()
	fx = testutil.TempWorktreeWithBareOrigin(t)
	target = gitOutputForIntegrateRun(t, fx.Clone, "branch", "--show-current")
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: "+target+"\n  taskPattern: feature/*\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "declare integration")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	return fx, target
}

func runIntegrateRunJSON(t *testing.T, dir string, args []string, flags ...string) (integrate.RunProtocol, error) {
	t.Helper()
	resetIntegrateRunFlags(t)
	if err := integrateRunCmd.ParseFlags(append(flags, "--json")); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	var stdout bytes.Buffer
	integrateRunCmd.SetOut(&stdout)
	t.Cleanup(func() { integrateRunCmd.SetOut(nil) })
	t.Chdir(dir)
	err := runIntegrateRun(integrateRunCmd, args)
	var protocol integrate.RunProtocol
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &protocol); unmarshalErr != nil {
		t.Fatalf("run protocol JSON = %q: %v (run err=%v)", stdout.String(), unmarshalErr, err)
	}
	return protocol, err
}

func TestIntegrateRunLandsWithTheNewArguments(t *testing.T) {
	fx, target := integrateRunNewArgsFixture(t)
	source := gitOutputForIntegrateRun(t, fx.Worktree, "rev-parse", "HEAD")
	targetSHA := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/"+target)

	protocol, err := runIntegrateRunJSON(t, fx.Worktree, []string{"feature/worktree"},
		"--source", source, "--target", target+"@"+targetSHA,
		"--record", "run-51@g3", "--owner", "claude/mbp/agent")
	if got := cliutil.ExitCodeForError(err); got != 0 {
		t.Fatalf("exit code = %d, want 0; err=%v", got, err)
	}
	if got := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/"+target); got != source {
		t.Fatalf("remote target = %s, want the pinned source %s", got, source)
	}
	if protocol.ExecutionID != "run-51" || protocol.Generation != "g3" ||
		protocol.Owner == nil || *protocol.Owner != (integrate.OwnerIdentity{Actor: "claude", Host: "mbp", Kind: "agent"}) {
		t.Fatalf("protocol identity = id %q gen %q owner %+v, want run-51/g3/claude-mbp-agent",
			protocol.ExecutionID, protocol.Generation, protocol.Owner)
	}
}

func TestIntegrateRunPinnedTargetThatMovedExitsFourWithoutPushing(t *testing.T) {
	fx, target := integrateRunNewArgsFixture(t)
	source := gitOutputForIntegrateRun(t, fx.Worktree, "rev-parse", "HEAD")
	before := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/"+target)

	_, err := runIntegrateRunJSON(t, fx.Worktree, []string{"feature/worktree"},
		"--source", source, "--target", target+"@"+strings.Repeat("0", 40),
		"--record", "run-52@g1", "--owner", "claude/mbp/agent")
	if got := cliutil.ExitCodeForError(err); got != cliutil.ExitSourceMismatch {
		t.Fatalf("exit code = %d, want %d; err=%v", got, cliutil.ExitSourceMismatch, err)
	}
	if !errors.Is(err, integrate.ErrTargetMismatch) {
		t.Fatalf("error = %v, want target mismatch", err)
	}
	if got := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/"+target); got != before {
		t.Fatalf("remote target moved: %s, want %s", got, before)
	}
}
