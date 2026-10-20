// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

func TestIntegrateRunHelp(t *testing.T) {
	cmd := findCommand(t, rootCmd, "integrate", "run")
	for _, name := range []string{"target", "direct-to-default", "release", "controller-config"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("integrate run missing --%s", name)
		}
	}
	// The legacy make gate and its skip escape hatch are gone: gz-git runs no
	// repository code, so there is no skipped check left to allow.
	if cmd.Flags().Lookup("allow-skipped-checks") != nil {
		t.Errorf("integrate run still exposes the removed --allow-skipped-checks")
	}
	if !strings.Contains(cmd.Long, "Engine: gz-git-integrate (Go)") {
		t.Errorf("integrate run help missing engine identifier:\n%s", cmd.Long)
	}
}

func TestIntegrateRunBareTargetCheckoutExitsOneWithoutMovingRemote(t *testing.T) {
	restore := setIntegrateRunGlobals(t)
	defer restore()

	fx := testutil.TempWorktreeWithBareOrigin(t)
	target := gitOutputForIntegrateRun(t, fx.Clone, "branch", "--show-current")
	writeFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: "+target+"\n")
	before := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/"+target)
	t.Chdir(fx.Clone)
	quiet = true
	err := runIntegrateRun(integrateRunCmd, nil)
	if got := cliutil.ExitCodeForError(err); got != 1 {
		t.Fatalf("bare target checkout exit = %d, want 1; err=%v", got, err)
	}
	if !errors.Is(err, integrate.ErrImplicitSourceIsTarget) {
		t.Fatalf("error = %v, want ErrImplicitSourceIsTarget", err)
	}
	after := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/"+target)
	if after != before {
		t.Fatalf("remote master moved: before=%s after=%s", before, after)
	}
}

func TestIntegrateRunReclaimIncompleteExitsThree(t *testing.T) {
	restore := setIntegrateRunGlobals(t)
	defer restore()

	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "branch", "develop")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	writeFile(t, fx.Worktree, "task.txt", "task\n")
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: dev/*\n")
	writeFile(t, fx.Worktree, "Makefile", "check:\n\t@true\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "task")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")

	extra := filepath.Join(t.TempDir(), "extra")
	runGit(t, fx.Clone, "worktree", "add", "--force", extra, "dev/actor/feat/task")

	t.Chdir(fx.Worktree)
	quiet = true
	err := runIntegrateRun(integrateRunCmd, []string{"dev/actor/feat/task"})
	if got := cliutil.ExitCodeForError(err); got != cliutil.ExitReclaimIncomplete {
		t.Fatalf("partial reclaim exit = %d, want %d (%v)", got, cliutil.ExitReclaimIncomplete, err)
	}
}

func TestIntegrateRunNotRepoExitsTwo(t *testing.T) {
	restore := setIntegrateRunGlobals(t)
	defer restore()
	t.Chdir(t.TempDir())
	quiet = true
	err := runIntegrateRun(integrateRunCmd, nil)
	if got := cliutil.ExitCodeForError(err); got != 2 {
		t.Fatalf("not-a-repo exit = %d, want 2; err=%v", got, err)
	}
}

func TestIntegrateRunExplicitEmptyExpectedTargetExitsTwoBeforeEngine(t *testing.T) {
	restore := setIntegrateRunGlobals(t)
	defer restore()
	if err := integrateRunCmd.ParseFlags([]string{"--expect-target="}); err != nil {
		t.Fatalf("ParseFlags(--expect-target=) = %v", err)
	}
	t.Chdir(t.TempDir())
	err := runIntegrateRun(integrateRunCmd, nil)
	if got := cliutil.ExitCodeForError(err); got != 2 {
		t.Fatalf("exit code = %d, want 2; err=%v", got, err)
	}
	if err == nil || !strings.Contains(err.Error(), "--expect-target must not be empty") {
		t.Fatalf("error = %v, want explicit empty-target usage rejection", err)
	}
}

func TestIntegrateRunPushSucceededLocalTargetFailedExitsThree(t *testing.T) {
	restore := setIntegrateRunGlobals(t)
	defer restore()

	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "checkout", "-b", "develop")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
	before := gitOutputForIntegrateRun(t, fx.Clone, "rev-parse", "HEAD")
	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	writeFile(t, fx.Worktree, "task.txt", "committed task\n")
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: dev/*\n")
	writeFile(t, fx.Worktree, "Makefile", "check:\n\t@true\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "task")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	approved := gitOutputForIntegrateRun(t, fx.Worktree, "rev-parse", "HEAD")
	// A user's untracked file makes the local target fast-forward refuse
	// after the remote push. Preserve both that file and the pushed result.
	writeFile(t, fx.Clone, "task.txt", "user work\n")
	t.Chdir(fx.Worktree)
	quiet = true
	err := runIntegrateRun(integrateRunCmd, []string{"dev/actor/feat/task"})
	if got := cliutil.ExitCodeForError(err); got != cliutil.ExitReclaimIncomplete {
		t.Fatalf("successful push with local target failure exit = %d, want 3 (%v)", got, err)
	}
	if got := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != approved {
		t.Fatalf("remote target = %s, want pushed commit %s", got, approved)
	}
	if got := gitOutputForIntegrateRun(t, fx.Clone, "rev-parse", "HEAD"); got != before {
		t.Fatalf("local target moved despite refusal: %s, want %s", got, before)
	}
	if data, readErr := os.ReadFile(filepath.Join(fx.Clone, "task.txt")); readErr != nil || string(data) != "user work\n" {
		t.Fatalf("user work changed: %q (%v)", data, readErr)
	}
	if got := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/dev/actor/feat/task"); got != approved {
		t.Fatalf("task was reclaimed after local target failure: %s", got)
	}
}

func setIntegrateRunGlobals(t *testing.T) func() {
	t.Helper()
	origTarget := integrateRunTarget
	origDirect := integrateRunDirectToDefault
	origRelease := integrateRunRelease
	origController := integrateRunControllerConfig
	origNoFetch := integrateRunNoFetch
	origExpectSource := integrateRunExpectSource
	origExpectTarget := integrateRunExpectTarget
	origExpectTargetChanged := integrateRunCmd.Flags().Lookup("expect-target").Changed
	origQuiet := quiet
	integrateRunTarget = ""
	integrateRunDirectToDefault = false
	integrateRunRelease = false
	integrateRunControllerConfig = ""
	integrateRunNoFetch = false
	integrateRunExpectSource = ""
	integrateRunExpectTarget = ""
	integrateRunCmd.Flags().Lookup("expect-target").Changed = false
	quiet = false
	return func() {
		integrateRunTarget = origTarget
		integrateRunDirectToDefault = origDirect
		integrateRunRelease = origRelease
		integrateRunControllerConfig = origController
		integrateRunNoFetch = origNoFetch
		integrateRunExpectSource = origExpectSource
		integrateRunExpectTarget = origExpectTarget
		integrateRunCmd.Flags().Lookup("expect-target").Changed = origExpectTargetChanged
		quiet = origQuiet
	}
}

func gitOutputForIntegrateRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}
