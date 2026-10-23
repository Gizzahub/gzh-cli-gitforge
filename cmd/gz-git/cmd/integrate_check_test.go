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

func TestIntegrateCheckHelp(t *testing.T) {
	cmd := findCommand(t, rootCmd, "integrate", "check")
	for _, name := range []string{"target", "direct-to-default", "release", "controller-config"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("integrate check missing --%s", name)
		}
	}
	// The legacy make gate and its skip escape hatch are gone: gz-git runs no
	// repository code, so there is no skipped check left to allow.
	if cmd.Flags().Lookup("allow-skipped-checks") != nil {
		t.Errorf("integrate check still exposes the removed --allow-skipped-checks")
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

func TestIntegrateCheckExplicitEmptyExpectedTargetExitsTwoBeforeEngine(t *testing.T) {
	restore := setIntegrateCheckGlobals(t)
	defer restore()
	if err := integrateCheckCmd.ParseFlags([]string{"--expect-target="}); err != nil {
		t.Fatalf("ParseFlags(--expect-target=) = %v", err)
	}
	t.Chdir(t.TempDir())
	err := runIntegrateCheck(integrateCheckCmd, nil)
	if got := cliutil.ExitCodeForError(err); got != 2 {
		t.Fatalf("exit code = %d, want 2; err=%v", got, err)
	}
	if err == nil || !strings.Contains(err.Error(), "--expect-target must not be empty") {
		t.Fatalf("error = %v, want explicit empty-target usage rejection", err)
	}
}

// TestIntegrateCheckReleaseReadsSourceDeclaration is the ISSUE-109 command:
// `integrate check <remote>/develop --target <remote>/main --release
// --expect-source <sha>` from a primary checkout whose working-tree
// .gz-git.yaml still carries a removed key that develop already deleted. The
// release reads the declaration at the source commit and names that commit.
func TestIntegrateCheckReleaseReadsSourceDeclaration(t *testing.T) {
	restore := setIntegrateCheckGlobals(t)
	defer restore()

	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: [dev/*]\n  prepareProfile: pnpm-frozen-lockfile-v1\n")
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "declare policy")
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/main")
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", "develop", fx.Remote+"/main")
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: [dev/*]\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "drop removed key")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Clone, "fetch", fx.Remote)
	source := gitOutputForIntegrateRun(t, fx.Clone, "rev-parse", fx.Remote+"/develop")

	integrateCheckTarget = fx.Remote + "/main"
	integrateCheckRelease = true
	integrateCheckExpectSource = source
	var stdout bytes.Buffer
	integrateCheckCmd.SetOut(&stdout)
	t.Cleanup(func() { integrateCheckCmd.SetOut(nil) })
	t.Chdir(fx.Clone)

	err := runIntegrateCheck(integrateCheckCmd, []string{fx.Remote + "/develop"})
	if got := cliutil.ExitCodeForError(err); got != 0 {
		t.Fatalf("release check exit = %d, want 0; err=%v\n%s", got, err, stdout.String())
	}
	if want := "at release source commit " + source[:12]; !strings.Contains(stdout.String(), want) {
		t.Fatalf("output does not name %q:\n%s", want, stdout.String())
	}
}

func setIntegrateCheckGlobals(t *testing.T) func() {
	t.Helper()
	origTarget := integrateCheckTarget
	origDirect := integrateCheckDirectToDefault
	origRelease := integrateCheckRelease
	origController := integrateCheckControllerConfig
	origNoFetch := integrateCheckNoFetch
	origExpectSource := integrateCheckExpectSource
	origExpectTarget := integrateCheckExpectTarget
	origExpectTargetChanged := integrateCheckCmd.Flags().Lookup("expect-target").Changed
	origQuiet := quiet
	integrateCheckTarget = ""
	integrateCheckDirectToDefault = false
	integrateCheckRelease = false
	integrateCheckControllerConfig = ""
	integrateCheckNoFetch = false
	integrateCheckExpectSource = ""
	integrateCheckExpectTarget = ""
	integrateCheckCmd.Flags().Lookup("expect-target").Changed = false
	quiet = false
	return func() {
		integrateCheckTarget = origTarget
		integrateCheckDirectToDefault = origDirect
		integrateCheckRelease = origRelease
		integrateCheckControllerConfig = origController
		integrateCheckNoFetch = origNoFetch
		integrateCheckExpectSource = origExpectSource
		integrateCheckExpectTarget = origExpectTarget
		integrateCheckCmd.Flags().Lookup("expect-target").Changed = origExpectTargetChanged
		quiet = origQuiet
	}
}
