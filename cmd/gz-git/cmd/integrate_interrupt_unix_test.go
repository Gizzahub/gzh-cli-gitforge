//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

// ISSUE-82: SIGINT during readiness must stop the runner's process group
// through killReadinessProcess and remove the readiness workspace before
// integrate run exits, instead of orphaning both.
func TestIntegrateRunInterruptStopsReadinessGroupAndRemovesWorkspace(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "runner")
	// The runner records its pid, its background child and its own path, then
	// blocks until its group is signaled. Both share the runner's group.
	fx := interruptRunFixture(t, "#!/bin/sh\nsleep 60 &\nchild=$!\nprintf '%s %s %s\\n' \"$$\" \"$child\" \"$0\" > '"+marker+".tmp'\nmv '"+marker+".tmp' '"+marker+"'\nwait \"$child\"\n")
	before := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/develop")
	run := startInterruptibleRun(t)
	runnerPID, childPID, runnerPath := waitForInterruptRunner(t, marker, run)
	t.Cleanup(func() { _ = syscall.Kill(-runnerPID, syscall.SIGKILL) })
	// check → readiness → .gz-git → target → the readiness root.
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(runnerPath))))
	if !strings.HasPrefix(filepath.Base(root), "gz-git-readiness-") {
		t.Fatalf("runner path %s is not inside a readiness root", runnerPath)
	}
	signalSelf(t)
	err := run.wait(t, 30*time.Second)
	if cliutil.ExitCodeForError(err) != 2 || !errors.Is(err, integrate.ErrInterrupted) {
		t.Fatalf("exit=%d err=%v stdout=%s stderr=%s", cliutil.ExitCodeForError(err), err, run.out.String(), run.stderr.String())
	}
	for _, pid := range []int{runnerPID, childPID} {
		waitForProcessGone(t, pid)
	}
	if _, statErr := os.Stat(root); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("readiness root %s survived the interrupt: %v", root, statErr)
	}
	if list := gitOutputForIntegrateRun(t, fx.Worktree, "worktree", "list", "--porcelain"); strings.Contains(list, filepath.Base(root)) {
		t.Fatalf("readiness worktrees still registered:\n%s", list)
	}
	if got := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != before {
		t.Fatalf("target moved after interrupt: %s -> %s", before, got)
	}
}

// Past the intent boundary a signal no longer applies: the push in flight,
// the local fast-forward and reclaim all finish, because CE relies on gz-git
// completing them.
func TestIntegrateRunInterruptDuringPushFinishesIntegration(t *testing.T) {
	fx := interruptRunFixture(t, "#!/bin/sh\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"target gate passed\"}'\n")
	source := gitOutputForIntegrateRun(t, fx.Worktree, "rev-parse", "HEAD")
	// The hook holds the target push open until the test has signaled.
	marker := filepath.Join(t.TempDir(), "push")
	release := marker + ".release"
	hooks := filepath.Join(fx.Origin, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\ncat >/dev/null\n: > '" + marker + "'\nn=0\nwhile [ ! -e '" + release + "' ] && [ $n -lt 600 ]; do sleep 0.1; n=$((n+1)); done\n"
	if err := os.WriteFile(filepath.Join(hooks, "pre-receive"), []byte(hook), 0o755); err != nil { // #nosec G306 -- test hook must be executable
		t.Fatal(err)
	}
	run := startInterruptibleRun(t)
	deadline := time.Now().Add(60 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case <-run.finished:
			t.Fatalf("integrate run returned before the push: %v stderr=%s", run.err, run.stderr.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("target push never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	signalSelf(t)
	// Give the handler time to cancel the command context before the push
	// is allowed to complete.
	time.Sleep(300 * time.Millisecond)
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run.wait(t, 60*time.Second); err != nil {
		t.Fatalf("interrupted push did not finish: %v stdout=%s stderr=%s", err, run.out.String(), run.stderr.String())
	}
	if got := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != source {
		t.Fatalf("target = %s, want %s", got, source)
	}
	if !strings.Contains(run.out.String(), "RECLAIMED") {
		t.Fatalf("reclaim did not run after the interrupt: %s", run.out.String())
	}
}

// interruptRunFixture declares runner as the target readiness and leaves the
// test inside a pushed task worktree one commit ahead of develop.
func interruptRunFixture(t *testing.T, runner string) *testutil.WorktreeOrigin {
	t.Helper()
	t.Setenv("HOST_SLOTS_HELD", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "branch", "develop")
	runGit(t, fx.Clone, "checkout", "develop")
	if err := os.MkdirAll(filepath.Join(fx.Clone, ".gz-git", "readiness"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: dev/*\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n")
	writeFile(t, fx.Clone, ".gz-git/readiness/check", runner)
	if err := os.Chmod(filepath.Join(fx.Clone, ".gz-git", "readiness", "check"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "declare readiness")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/fix/interrupt", fx.Remote+"/develop")
	writeFile(t, fx.Worktree, "task.txt", "task\n")
	runGit(t, fx.Worktree, "add", "task.txt")
	runGit(t, fx.Worktree, "commit", "-m", "task")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	t.Chdir(fx.Worktree)
	return fx
}

type interruptibleRun struct {
	finished    chan struct{}
	err         error
	out, stderr bytes.Buffer
}

// startInterruptibleRun runs integrate run in the background. Cleanup waits
// for it before restoring the command globals it reads.
func startInterruptibleRun(t *testing.T) *interruptibleRun {
	t.Helper()
	restore := setIntegrateRunGlobals(t)
	run := &interruptibleRun{finished: make(chan struct{})}
	oldOut, oldErr := integrateRunCmd.OutOrStdout(), integrateRunCmd.ErrOrStderr()
	integrateRunCmd.SetOut(&run.out)
	integrateRunCmd.SetErr(&run.stderr)
	go func() {
		defer close(run.finished)
		run.err = runIntegrateRun(integrateRunCmd, nil)
	}()
	t.Cleanup(func() {
		select {
		case <-run.finished:
		case <-time.After(60 * time.Second):
		}
		integrateRunCmd.SetOut(oldOut)
		integrateRunCmd.SetErr(oldErr)
		restore()
	})
	return run
}

func (r *interruptibleRun) wait(t *testing.T, limit time.Duration) error {
	t.Helper()
	select {
	case <-r.finished:
		return r.err
	case <-time.After(limit):
		t.Fatal("integrate run did not return after SIGINT")
		return nil
	}
}

func signalSelf(t *testing.T) {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
}

func waitForInterruptRunner(t *testing.T, marker string, run *interruptibleRun) (runnerPID, childPID int, runnerPath string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-run.finished:
			t.Fatalf("integrate run returned before readiness started: %v stderr=%s", run.err, run.stderr.String())
		default:
		}
		data, err := os.ReadFile(marker)
		if err == nil {
			fields := strings.Fields(string(data))
			if len(fields) != 3 {
				t.Fatalf("runner marker %q", data)
			}
			runnerPID, err = strconv.Atoi(fields[0])
			if err != nil {
				t.Fatal(err)
			}
			childPID, err = strconv.Atoi(fields[1])
			if err != nil {
				t.Fatal(err)
			}
			return runnerPID, childPID, fields[2]
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("readiness runner never started")
	return 0, 0, ""
}

// waitForProcessGone allows the reparented child a moment to be reaped.
func waitForProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("readiness process %d survived the interrupt", pid)
}
