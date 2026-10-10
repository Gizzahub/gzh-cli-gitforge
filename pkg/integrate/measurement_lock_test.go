// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

// hostLockChildEnv marks the re-executed test binary that holds the lock in
// TestMeasurementLockCrash. TestMain leaves XDG_STATE_HOME alone in that
// child so it locks the parent's directory.
const hostLockChildEnv = "GZ_GIT_TEST_HOST_LOCK_CHILD"

// TestMain points the host measurement lock at a private directory, so no
// test in this package ever takes, or waits on, this host's real lock.
func TestMain(m *testing.M) {
	if os.Getenv(hostLockChildEnv) != "" {
		os.Exit(m.Run())
	}
	state, err := os.MkdirTemp("", "gz-git-integrate-state-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create test state dir:", err)
		os.Exit(1)
	}
	if err := os.Setenv("XDG_STATE_HOME", state); err != nil {
		fmt.Fprintln(os.Stderr, "set XDG_STATE_HOME:", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(state)
	os.Exit(code)
}

// lockedBuffer collects the waiting notice while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fastHostLockPolling makes waits observable within a test's runtime.
func fastHostLockPolling(t *testing.T) {
	t.Helper()
	poll, notice := hostLockPollInterval, hostLockNoticeInterval
	hostLockPollInterval, hostLockNoticeInterval = 10*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { hostLockPollInterval, hostLockNoticeInterval = poll, notice })
}

// measurementLockFixture is a task branch on develop whose check and lint
// recipes are both recipe, with the declared make budget.
func measurementLockFixture(t *testing.T, recipe, budget string) *testutil.WorktreeOrigin {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	makefile := "check:\n\t" + recipe + "\nlint:\n\t" + recipe + "\n"
	writeRepoFile(t, fx.Clone, "Makefile", makefile)
	runGit(t, fx.Clone, "add", "Makefile")
	runGit(t, fx.Clone, "commit", "-m", "gate")
	runGit(t, fx.Clone, "branch", "develop")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", fx.Remote+"/develop")
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  makeTimeout: "+budget+"\n")
	writeRepoFile(t, fx.Worktree, "task.txt", "task\n")
	runGit(t, fx.Worktree, "add", ".gz-git.yaml", "task.txt")
	runGit(t, fx.Worktree, "commit", "-m", "task")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	return fx
}

// holdHostLockInProcess takes the lock as a stand-in holder. flock belongs
// to the open file description, so Check's own open conflicts with it just
// as another process's would.
func holdHostLockInProcess(t *testing.T, repository, branch string) *hostLock {
	t.Helper()
	lock, err := acquireHostLock(context.Background(), hostLockHolder{Repository: repository, Branch: branch, Command: "run"}, time.Second, io.Discard)
	if err != nil {
		t.Fatalf("hold host lock: %v", err)
	}
	t.Cleanup(lock.release)
	return lock
}

func waitForText(t *testing.T, what string, timeout time.Duration, read func() string, want string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !strings.Contains(read(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("%s never contained %q within %s; got:\n%s", what, want, timeout, read())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type checkResult struct {
	report *CheckReport
	err    error
}

// TestMeasurementLock runs two integrations of two different repositories at
// once. Every make recipe claims one shared directory with an atomic mkdir
// and fails with 9 if another measurement already holds it, so the runs are
// both integrated only if no two measurements overlapped.
func TestMeasurementLock(t *testing.T) {
	busy := filepath.Join(t.TempDir(), "busy")
	recipe := "@mkdir " + busy + " || exit 9; sleep 0.3; rmdir " + busy
	fixtures := []*testutil.WorktreeOrigin{
		measurementLockFixture(t, recipe, "1m"),
		measurementLockFixture(t, recipe, "1m"),
	}
	type runResult struct {
		report *RunReport
		err    error
	}
	results := make([]runResult, len(fixtures))
	var wg sync.WaitGroup
	for i, fx := range fixtures {
		wg.Add(1)
		go func() {
			defer wg.Done()
			report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{
				RepoPath:   fx.Worktree,
				Branch:     "dev/actor/feat/task",
				LockNotice: io.Discard,
			}})
			results[i] = runResult{report, err}
		}()
	}
	wg.Wait()
	for i, r := range results {
		if r.err != nil {
			detail := ""
			if r.report != nil && r.report.Check != nil {
				detail = FormatCheck(r.report.Check)
			}
			t.Fatalf("run %d: %v\n%s", i, r.err, detail)
		}
		if !r.report.Integrated {
			t.Fatalf("run %d not integrated\n%s", i, FormatRun(r.report))
		}
	}
}

// TestMeasurementLockBudget holds the lock longer than the declared make
// budget. The waiting integration must still measure and pass: the budget
// clock starts when the lock is acquired.
func TestMeasurementLockBudget(t *testing.T) {
	fastHostLockPolling(t)
	fx := measurementLockFixture(t, "@true", "1s")
	lock := holdHostLockInProcess(t, "/elsewhere/repo", "dev/other")
	const hold = 2500 * time.Millisecond
	released := make(chan struct{})
	go func() {
		defer close(released)
		time.Sleep(hold)
		lock.release()
	}()
	defer func() { <-released }()

	notice := &lockedBuffer{}
	start := time.Now()
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath:   fx.Worktree,
		Branch:     "dev/actor/feat/task",
		LockNotice: notice,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if elapsed < hold {
		t.Fatalf("Check finished after %s, before the holder released at %s: it never waited", elapsed, hold)
	}
	if !strings.Contains(notice.String(), "waiting") {
		t.Fatalf("no waiting notice:\n%s", notice.String())
	}
	if !report.Ready {
		t.Fatalf("waiting %s for the lock was charged to the 1s make budget\n%s", hold, FormatCheck(report))
	}
}

// TestMeasurementLockCrashHolderProcess is not a test of its own: it is the
// holder process TestMeasurementLockCrash starts and kills.
func TestMeasurementLockCrashHolderProcess(t *testing.T) {
	if os.Getenv(hostLockChildEnv) == "" {
		t.Skip("holder process for TestMeasurementLockCrash")
	}
	lock, err := acquireHostLock(context.Background(), hostLockHolder{Repository: "/crashed/repo", Branch: "dev/crashed", Command: "run"}, 0, io.Discard)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	fmt.Println("held")
	// Blocks until killed. The lock stays reachable: a collected *os.File
	// would close the descriptor and release the lock before the kill.
	for {
		time.Sleep(time.Hour)
		runtime.KeepAlive(lock)
	}
}

// TestMeasurementLockCrash kills a real holder process with SIGKILL, so it
// runs no release code and leaves its sidecar behind. The kernel drops the
// lock, and the waiter proceeds.
func TestMeasurementLockCrash(t *testing.T) {
	fastHostLockPolling(t)
	fx := measurementLockFixture(t, "@true", "1m")

	child := exec.Command(os.Args[0], "-test.run=^TestMeasurementLockCrashHolderProcess$") //nolint:noctx // killed explicitly below
	child.Env = append(os.Environ(), hostLockChildEnv+"=1")
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "held" {
		t.Fatalf("holder process did not take the lock: %q, %v", line, err)
	}

	notice := &lockedBuffer{}
	done := make(chan checkResult, 1)
	go func() {
		report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
			RepoPath:   fx.Worktree,
			Branch:     "dev/actor/feat/task",
			LockNotice: notice,
		})
		done <- checkResult{report, err}
	}()
	childPID := "pid " + strconv.Itoa(child.Process.Pid)
	waitForText(t, "waiting notice", 10*time.Second, notice.String, childPID)
	if !strings.Contains(notice.String(), "branch dev/crashed") {
		t.Fatalf("waiting notice does not name the holder's branch:\n%s", notice.String())
	}
	select {
	case r := <-done:
		t.Fatalf("Check returned while the holder was alive: %+v", r)
	case <-time.After(300 * time.Millisecond):
	}

	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Check after the holder died: %v", r.err)
		}
		if !r.report.Ready {
			t.Fatalf("Check after the holder died is not ready\n%s", FormatCheck(r.report))
		}
	case <-time.After(60 * time.Second):
		t.Fatalf("waiter never proceeded after the holder was killed; notice:\n%s", notice.String())
	}
}

// TestMeasurementLockFetch moves the target while the check waits for the
// lock. The measured target must be the tip after the wait, so the fetch and
// target resolution happen after the lock is acquired.
func TestMeasurementLockFetch(t *testing.T) {
	fastHostLockPolling(t)
	fx := measurementLockFixture(t, "@true", "1m")
	lock := holdHostLockInProcess(t, "/elsewhere/repo", "dev/other")

	notice := &lockedBuffer{}
	done := make(chan checkResult, 1)
	go func() {
		report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
			RepoPath:   fx.Worktree,
			Branch:     "dev/actor/feat/task",
			LockNotice: notice,
		})
		done <- checkResult{report, err}
	}()
	waitForText(t, "waiting notice", 10*time.Second, notice.String, "waiting")

	runGit(t, fx.Clone, "checkout", "develop")
	writeRepoFile(t, fx.Clone, "moved.txt", "moved\n")
	runGit(t, fx.Clone, "add", "moved.txt")
	runGit(t, fx.Clone, "commit", "-m", "target moves during the wait")
	runGit(t, fx.Clone, "push", fx.Remote, "develop")
	tip := gitOutput(t, fx.Clone, "rev-parse", "HEAD")
	lock.release()

	r := <-done
	if r.err != nil {
		t.Fatalf("Check: %v", r.err)
	}
	if r.report.Plan.TargetSHA != tip {
		t.Fatalf("measured target %s, want the tip after the wait %s", r.report.Plan.TargetSHA, tip)
	}
}

// TestMeasurementLockWait gives up on a held lock after --lock-wait. Both
// commands must fail, name the holder, and run no make target.
func TestMeasurementLockWait(t *testing.T) {
	fastHostLockPolling(t)
	ran := filepath.Join(t.TempDir(), "ran")
	fx := measurementLockFixture(t, "@touch "+ran, "1m")
	holdHostLockInProcess(t, "/holder/repo", "dev/holder/branch")
	opts := CheckOptions{
		RepoPath:   fx.Worktree,
		Branch:     "dev/actor/feat/task",
		LockWait:   300 * time.Millisecond,
		LockNotice: io.Discard,
	}

	checkReport, checkErr := Check(context.Background(), gitcmd.NewExecutor(), opts)
	runReport, runErr := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: opts})
	for name, err := range map[string]error{"check": checkErr, "run": runErr} {
		if !errors.Is(err, ErrHostLockWait) {
			t.Fatalf("%s err = %v, want ErrHostLockWait", name, err)
		}
		for _, want := range []string{"pid " + strconv.Itoa(os.Getpid()), "repository /holder/repo", "branch dev/holder/branch", "300ms"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%s err %q does not name %q", name, err, want)
			}
		}
	}
	if checkReport != nil || runReport != nil {
		t.Fatalf("expired wait returned a report: check=%v run=%v", checkReport, runReport)
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a make target ran without the lock (stat %s: %v)", ran, err)
	}
}
