//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

// SIGINT before the intent boundary ends integrate run with exit 2 and
// ErrInterrupted, without moving the target. gz-git holds no lock it could
// wait on, so the run is held inside its readiness fetch instead: the
// remote's upload-pack blocks until the test releases it.
func TestIntegrateRunInterruptBeforeIntentStopsWithoutPush(t *testing.T) {
	fx := interruptRunFixture(t)
	before := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/develop")
	marker := holdReadinessFetch(t, fx.Worktree, fx.Remote)
	run := startInterruptibleRun(t)
	waitForRunMarker(t, run, marker)
	signalSelf(t)
	err := run.wait(t, 30*time.Second)
	if cliutil.ExitCodeForError(err) != 2 || !errors.Is(err, integrate.ErrInterrupted) {
		t.Fatalf("exit=%d err=%v stdout=%s stderr=%s", cliutil.ExitCodeForError(err), err, run.out.String(), run.stderr.String())
	}
	if got := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != before {
		t.Fatalf("target moved after interrupt: %s -> %s", before, got)
	}
}

// Past the intent boundary a signal no longer applies: the push in flight,
// the local fast-forward and reclaim all finish, because CE relies on gz-git
// completing them.
func TestIntegrateRunInterruptDuringPushFinishesIntegration(t *testing.T) {
	fx := interruptRunFixture(t)
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
	canceled := watchInterruptNotice(t)
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
	// The handler prints its notice immediately before canceling; wait for
	// it, then give the cancellation time to reach a still-cancelable push.
	select {
	case <-canceled:
	case <-time.After(30 * time.Second):
		t.Fatal("interrupt handler never ran")
	}
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

// interruptRunFixture declares develop as the integration branch and leaves
// the test inside a pushed task worktree one commit ahead of develop.
func interruptRunFixture(t *testing.T) *testutil.WorktreeOrigin {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "branch", "develop")
	runGit(t, fx.Clone, "checkout", "develop")
	writeFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: dev/*\n")
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "declare integration branch")
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

// holdReadinessFetch makes every fetch from remote in dir block inside
// upload-pack until the test ends, and returns the file the wrapper creates
// once a fetch has reached it. The wait is bounded so a killed fetch cannot
// leave the wrapper running for long, and the wrapper drops the stderr it
// inherits so a canceled fetch does not wait on the pipe it would hold open.
func holdReadinessFetch(t *testing.T, dir, remote string) string {
	t.Helper()
	scratch := t.TempDir()
	marker := filepath.Join(scratch, "fetching")
	release := filepath.Join(scratch, "release")
	wrapper := filepath.Join(scratch, "upload-pack")
	script := "#!/bin/sh\nexec 2>/dev/null\n: > '" + marker + "'\nn=0\nwhile [ ! -e '" + release + "' ] && [ $n -lt 600 ]; do sleep 0.1; n=$((n+1)); done\nexec git upload-pack \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil { // #nosec G306 -- test wrapper must be executable
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.WriteFile(release, nil, 0o600) })
	runGit(t, dir, "config", "remote."+remote+".uploadpack", wrapper)
	return marker
}

type interruptibleRun struct {
	finished chan struct{}
	err      error
	out      bytes.Buffer
	stderr   lockedStderr
}

// lockedStderr lets the test poll stderr while the run goroutine writes it.
type lockedStderr struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedStderr) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedStderr) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
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

// watchInterruptNotice routes os.Stderr through a pipe until the run is over
// and reports when withInterruptCancel announces the cancellation. It must
// run before startInterruptibleRun so the handler goroutine reads the swapped
// os.Stderr without a data race; its cleanup runs after the run finished.
func watchInterruptNotice(t *testing.T) <-chan struct{} {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	notice := make(chan struct{})
	drained := make(chan struct{})
	go func(seen chan struct{}) {
		defer close(drained)
		var buf bytes.Buffer
		chunk := make([]byte, 512)
		for {
			n, readErr := r.Read(chunk)
			buf.Write(chunk[:n])
			if seen != nil && strings.Contains(buf.String(), "Interrupted, canceling") {
				close(seen)
				seen = nil
			}
			if readErr != nil {
				return
			}
		}
	}(notice)
	t.Cleanup(func() {
		os.Stderr = old
		_ = w.Close()
		<-drained
		_ = r.Close()
	})
	return notice
}

// signalSelf sends SIGINT to the whole test process. That is safe only while
// an integrate run has registered its handler (the callers wait for a marker
// the run produces first) and because t.Chdir keeps these tests serial; a
// SIGINT with no handler registered would end the test binary.
func signalSelf(t *testing.T) {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
}

// waitForRunMarker waits until the background run has created marker, which
// shows it reached the wait the test is about to interrupt.
func waitForRunMarker(t *testing.T, run *interruptibleRun, marker string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-run.finished:
			t.Fatalf("integrate run returned before reaching %s: %v stderr=%s", marker, run.err, run.stderr.String())
		default:
		}
		if _, err := os.Stat(marker); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("integrate run never reached %s", marker)
}
