// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
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
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: master\n  taskPattern: feature/*\n")
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

func setIntegrateRunContextGlobals(t *testing.T) func() {
	t.Helper()
	origExecutionID := integrateRunExecutionID
	origGeneration := integrateRunGeneration
	origOwnerActor := integrateRunOwnerActor
	origOwnerHost := integrateRunOwnerHost
	origOwnerKind := integrateRunOwnerKind
	origExpectSource := integrateRunExpectSource
	origJSON := integrateRunJSON
	origQuiet := quiet
	integrateRunExecutionID = ""
	integrateRunGeneration = ""
	integrateRunOwnerActor = ""
	integrateRunOwnerHost = ""
	integrateRunOwnerKind = ""
	integrateRunExpectSource = ""
	integrateRunJSON = false
	quiet = false
	return func() {
		integrateRunExecutionID = origExecutionID
		integrateRunGeneration = origGeneration
		integrateRunOwnerActor = origOwnerActor
		integrateRunOwnerHost = origOwnerHost
		integrateRunOwnerKind = origOwnerKind
		integrateRunExpectSource = origExpectSource
		integrateRunJSON = origJSON
		quiet = origQuiet
	}
}
