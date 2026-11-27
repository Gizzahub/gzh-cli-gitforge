// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

// Not parallel: it shares the command's package-level flag state.
func TestIntegrateRunHotfixFlag(t *testing.T) {
	// A task runtime probes the capability through --help before using it.
	flag := integrateRunCmd.Flags().Lookup("hotfix")
	if flag == nil || !strings.Contains(integrateRunCmd.Long, "--hotfix") {
		t.Fatalf("integrate run must declare --hotfix in its flags and help: flag=%v", flag)
	}

	restore := setIntegrateRunContextGlobals(t, "--hotfix")
	defer restore()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	var stdout bytes.Buffer
	integrateRunCmd.SetOut(&stdout)
	t.Cleanup(func() { integrateRunCmd.SetOut(nil) })
	t.Chdir(fx.Worktree)

	// Without an execution context only a run that received --hotfix is
	// refused this way, so the refusal proves the flag reached the engine.
	err := runIntegrateRun(integrateRunCmd, []string{"feature/worktree"})
	if !errors.Is(err, integrate.ErrHotfixRefused) {
		t.Fatalf("runIntegrateRun() error = %v, want a hotfix refusal", err)
	}
	if got := cliutil.ExitCodeForError(err); got != integrate.RunExitToolError {
		t.Fatalf("exit code = %d, want %d", got, integrate.RunExitToolError)
	}
}
