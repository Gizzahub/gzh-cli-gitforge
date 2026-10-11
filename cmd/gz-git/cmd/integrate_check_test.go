// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

func TestIntegrateCheckHelp(t *testing.T) {
	cmd := findCommand(t, rootCmd, "integrate", "check")
	for _, name := range []string{"target", "direct-to-default", "release", "allow-skipped-checks", "controller-config", "lock-wait"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("integrate check missing --%s", name)
		}
	}
	if !strings.Contains(cmd.Long, "Engine: gz-git-integrate (Go)") {
		t.Errorf("integrate check help missing engine identifier:\n%s", cmd.Long)
	}
}

func TestIntegrateCheckBareTargetCheckoutExitsOne(t *testing.T) {
	restore := setIntegrateCheckGlobals(t)
	defer restore()

	fx := testutil.TempWorktreeWithBareOrigin(t)
	target := gitOutputForIntegrateRun(t, fx.Clone, "branch", "--show-current")
	writeFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: "+target+"\n")
	t.Chdir(fx.Clone)
	quiet = true
	err := runIntegrateCheck(integrateCheckCmd, nil)
	if got := cliutil.ExitCodeForError(err); got != 1 {
		t.Fatalf("bare target checkout exit = %d, want 1; err=%v", got, err)
	}
	if !errors.Is(err, integrate.ErrImplicitSourceIsTarget) {
		t.Fatalf("error = %v, want ErrImplicitSourceIsTarget", err)
	}
}

func TestIntegrateCheckUndeclaredResolvesOriginHead(t *testing.T) {
	restore := setIntegrateCheckGlobals(t)
	defer restore()

	fx := testutil.TempWorktreeWithBareOrigin(t)
	t.Chdir(fx.Worktree)
	quiet = true
	err := runIntegrateCheck(integrateCheckCmd, []string{"feature/worktree"})
	if err != nil && strings.Contains(err.Error(), "--target") {
		t.Fatalf("undeclared repo with origin/HEAD must not require --target: %v", err)
	}
}

func TestIntegrateCheckNotRepoExitsTwo(t *testing.T) {
	restore := setIntegrateCheckGlobals(t)
	defer restore()
	t.Chdir(t.TempDir())
	quiet = true
	err := runIntegrateCheck(integrateCheckCmd, nil)
	if got := cliutil.ExitCodeForError(err); got != 2 {
		t.Fatalf("not-a-repo exit = %d, want 2; err=%v", got, err)
	}
}

func setIntegrateCheckGlobals(t *testing.T) func() {
	t.Helper()
	origTarget := integrateCheckTarget
	origDirect := integrateCheckDirectToDefault
	origRelease := integrateCheckRelease
	origSkip := integrateCheckAllowSkipped
	origController := integrateCheckControllerConfig
	origNoFetch := integrateCheckNoFetch
	origLockWait := integrateCheckLockWait
	origLockWaitChanged := integrateCheckCmd.Flags().Changed("lock-wait")
	origQuiet := quiet
	integrateCheckTarget = ""
	integrateCheckDirectToDefault = false
	integrateCheckRelease = false
	integrateCheckAllowSkipped = false
	integrateCheckControllerConfig = ""
	integrateCheckNoFetch = false
	integrateCheckLockWait = 0
	_ = integrateCheckCmd.Flags().Set("lock-wait", "0")
	integrateCheckCmd.Flags().Lookup("lock-wait").Changed = false
	quiet = false
	return func() {
		integrateCheckTarget = origTarget
		integrateCheckDirectToDefault = origDirect
		integrateCheckRelease = origRelease
		integrateCheckAllowSkipped = origSkip
		integrateCheckControllerConfig = origController
		integrateCheckNoFetch = origNoFetch
		integrateCheckLockWait = origLockWait
		_ = integrateCheckCmd.Flags().Set("lock-wait", origLockWait.String())
		integrateCheckCmd.Flags().Lookup("lock-wait").Changed = origLockWaitChanged
		quiet = origQuiet
	}
}
