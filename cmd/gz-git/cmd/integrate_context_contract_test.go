// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

func TestIntegrateRunContextFlagsAreAllOrNothing(t *testing.T) {
	restore := setIntegrateRunContextGlobals(t)
	defer restore()

	integrateRunExecutionID = "run-42"
	_, supplied, err := executionContextFromFlags()
	if err == nil {
		t.Fatal("executionContextFromFlags() error = nil, want partial context rejection")
	}
	if supplied {
		t.Fatal("executionContextFromFlags() supplied = true, want false for rejected context")
	}
	if !strings.Contains(err.Error(), "supplied together or not at all") {
		t.Fatalf("executionContextFromFlags() error = %v, want all-or-nothing message", err)
	}
}

func TestIntegrateRunContextSourceMismatchPrintsProtocolAndExitFour(t *testing.T) {
	restore := setIntegrateRunContextGlobals(t)
	defer restore()

	fx := testutil.TempWorktreeWithBareOrigin(t)
	target := gitOutputForIntegrateRun(t, fx.Clone, "branch", "--show-current")
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: "+target+"\n  taskPattern: feature/*\n")
	writeFile(t, fx.Worktree, "Makefile", "check:\n\t@true\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "declare integration")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")

	integrateRunExecutionID = "run-42"
	integrateRunGeneration = "generation-7"
	integrateRunOwnerActor = "codex"
	integrateRunOwnerHost = "mbp"
	integrateRunOwnerKind = "agent"
	integrateRunExpectSource = strings.Repeat("0", 40)
	integrateRunJSON = true
	quiet = false
	var stdout bytes.Buffer
	integrateRunCmd.SetOut(&stdout)
	t.Cleanup(func() { integrateRunCmd.SetOut(nil) })
	t.Chdir(fx.Worktree)

	err := runIntegrateRun(integrateRunCmd, []string{"feature/worktree"})
	if got := cliutil.ExitCodeForError(err); got != cliutil.ExitSourceMismatch {
		t.Fatalf("exit code = %d, want %d; err=%v", got, cliutil.ExitSourceMismatch, err)
	}
	if err == nil || !strings.Contains(err.Error(), "source is not the expected commit") {
		t.Fatalf("runIntegrateRun() error = %v, want source mismatch", err)
	}
	var protocol integrate.RunProtocol
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &protocol); unmarshalErr != nil {
		t.Fatalf("run protocol JSON = %q: %v", stdout.String(), unmarshalErr)
	}
	if protocol.SchemaVersion != 1 || protocol.Command != "integrate-run" || protocol.ExitCode != cliutil.ExitSourceMismatch {
		t.Fatalf("protocol = %+v, want schema=1 command=integrate-run exit=%d", protocol, cliutil.ExitSourceMismatch)
	}
	if !strings.Contains(protocol.Error, "source is not the expected commit") {
		t.Fatalf("protocol error = %q, want source mismatch", protocol.Error)
	}
}

func TestIntegrateRunContextTargetMismatchPrintsProtocolAndExitFour(t *testing.T) {
	restore := setIntegrateRunContextGlobals(t)
	defer restore()

	fx := testutil.TempWorktreeWithBareOrigin(t)
	target := gitOutputForIntegrateRun(t, fx.Clone, "branch", "--show-current")
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: "+target+"\n  taskPattern: feature/*\n")
	writeFile(t, fx.Worktree, "Makefile", "check:\n\t@true\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "declare integration")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")

	integrateRunExecutionID = "run-43"
	integrateRunGeneration = "generation-7"
	integrateRunOwnerActor = "codex"
	integrateRunOwnerHost = "mbp"
	integrateRunOwnerKind = "agent"
	integrateRunExpectSource = gitOutputForIntegrateRun(t, fx.Worktree, "rev-parse", "HEAD")
	integrateRunExpectTarget = strings.Repeat("0", 40)
	integrateRunJSON = true
	quiet = false
	var stdout bytes.Buffer
	integrateRunCmd.SetOut(&stdout)
	t.Cleanup(func() { integrateRunCmd.SetOut(nil) })
	t.Chdir(fx.Worktree)

	err := runIntegrateRun(integrateRunCmd, []string{"feature/worktree"})
	if got := cliutil.ExitCodeForError(err); got != cliutil.ExitSourceMismatch {
		t.Fatalf("exit code = %d, want %d; err=%v", got, cliutil.ExitSourceMismatch, err)
	}
	if !errors.Is(err, integrate.ErrTargetMismatch) {
		t.Fatalf("runIntegrateRun() error = %v, want target mismatch", err)
	}
	var protocol integrate.RunProtocol
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &protocol); unmarshalErr != nil {
		t.Fatalf("run protocol JSON = %q: %v", stdout.String(), unmarshalErr)
	}
	if protocol.SchemaVersion != 1 || protocol.Command != "integrate-run" || protocol.ExitCode != cliutil.ExitSourceMismatch {
		t.Fatalf("protocol = %+v, want schema=1 command=integrate-run exit=%d", protocol, cliutil.ExitSourceMismatch)
	}
}

func TestIntegrateRunJSONKeepsFailureDiagnosticsOffProtocolStream(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("readiness contract runners are unsupported on Windows")
	}
	restore := setIntegrateRunContextGlobals(t)
	defer restore()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	target := gitOutputForIntegrateRun(t, fx.Clone, "branch", "--show-current")
	// The target-owned readiness runner fails only when the source tree (its
	// --source-dir argument) carries the broken marker, so the contract is
	// identical on both sides and the failure is the source's own.
	if err := os.MkdirAll(filepath.Join(fx.Clone, ".gz-git", "readiness"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: "+target+"\n  taskPattern: feature/*\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n")
	runner := filepath.Join(fx.Clone, ".gz-git", "readiness", "check")
	writeFile(t, fx.Clone, ".gz-git/readiness/check", "#!/bin/sh\nif [ -f \"$2/broken\" ]; then echo readiness-failure-marker >&2; exit 1; fi\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"ok\"}'\n")
	if err := os.Chmod(runner, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "declare passing readiness contract")
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD")
	runGit(t, fx.Worktree, "merge", "--ff-only", target)
	writeFile(t, fx.Worktree, "broken", "readiness must fail\n")
	runGit(t, fx.Worktree, "add", "broken")
	runGit(t, fx.Worktree, "commit", "-m", "introduce failing readiness")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	before := gitOutputForIntegrateRun(t, fx.Clone, "rev-parse", target)
	integrateRunJSON, quiet = true, false
	integrateRunExecutionID, integrateRunGeneration = "diagnostic-run", "generation-1"
	integrateRunOwnerActor, integrateRunOwnerHost, integrateRunOwnerKind = "codex", "mst", "agent"
	integrateRunExpectSource = gitOutputForIntegrateRun(t, fx.Worktree, "rev-parse", "HEAD")
	var stdout, stderr bytes.Buffer
	integrateRunCmd.SetOut(&stdout)
	integrateRunCmd.SetErr(&stderr)
	t.Cleanup(func() { integrateRunCmd.SetOut(nil); integrateRunCmd.SetErr(nil) })
	t.Chdir(fx.Worktree)
	err := runIntegrateRun(integrateRunCmd, []string{"feature/worktree"})
	if cliutil.ExitCodeForError(err) != 1 {
		t.Fatalf("exit = %v; stdout=%s stderr=%s", err, stdout.String(), stderr.String())
	}
	var protocol integrate.RunProtocol
	if err := json.Unmarshal(stdout.Bytes(), &protocol); err != nil {
		t.Fatalf("protocol stream contains non-JSON diagnostics: %s: %v", stdout.String(), err)
	}
	if protocol.SourcePushed || protocol.Integrated || protocol.PreMutationOutcome == nil {
		t.Fatalf("refusal lost its negative binding: %+v", protocol)
	}
	if !strings.Contains(stderr.String(), "FAIL  readiness contract") || !strings.Contains(stderr.String(), "diagnostic: ") || !strings.Contains(stderr.String(), "NOT READY") {
		t.Fatalf("failed gate cause missing from stderr: %s", stderr.String())
	}
	for _, line := range strings.Split(stderr.String(), "\n") {
		if _, path, found := strings.Cut(line, "diagnostic: "); found {
			log, readErr := os.ReadFile(strings.TrimSpace(path))
			if readErr != nil || !bytes.Contains(log, []byte("readiness-failure-marker")) {
				t.Fatalf("reported diagnostic did not preserve the actual failed command: %v: %s", readErr, log)
			}
		}
	}
	if got := gitOutputForIntegrateRun(t, fx.Clone, "rev-parse", target); got != before {
		t.Fatalf("failed gate changed target: %s -> %s", before, got)
	}
}

func setIntegrateRunContextGlobals(t *testing.T) func() {
	t.Helper()
	origExecutionID := integrateRunExecutionID
	origGeneration := integrateRunGeneration
	origOwnerActor := integrateRunOwnerActor
	origOwnerHost := integrateRunOwnerHost
	origOwnerKind := integrateRunOwnerKind
	origExpectSource := integrateRunExpectSource
	origExpectTarget := integrateRunExpectTarget
	origJSON := integrateRunJSON
	origQuiet := quiet
	integrateRunExecutionID = ""
	integrateRunGeneration = ""
	integrateRunOwnerActor = ""
	integrateRunOwnerHost = ""
	integrateRunOwnerKind = ""
	integrateRunExpectSource = ""
	integrateRunExpectTarget = ""
	integrateRunJSON = false
	quiet = false
	return func() {
		integrateRunExecutionID = origExecutionID
		integrateRunGeneration = origGeneration
		integrateRunOwnerActor = origOwnerActor
		integrateRunOwnerHost = origOwnerHost
		integrateRunOwnerKind = origOwnerKind
		integrateRunExpectSource = origExpectSource
		integrateRunExpectTarget = origExpectTarget
		integrateRunJSON = origJSON
		quiet = origQuiet
	}
}
