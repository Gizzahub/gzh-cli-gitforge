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
const (
	hostLockChildEnv          = "GZ_GIT_TEST_HOST_LOCK_CHILD"
	inheritedResourceChildEnv = "GZ_GIT_TEST_INHERITED_RESOURCE_CHILD"
	testWaitTimeout           = 10 * time.Second
)

// TestMain points the host measurement lock at a private directory, so no
// test in this package ever takes, or waits on, this host's real lock.
func TestMain(m *testing.M) {
	for _, env := range testBinaryChildEnvs {
		if os.Getenv(env) != "" {
			os.Exit(m.Run())
		}
	}
	// A caller may itself inherit a real host slot. Package tests must never
	// borrow it accidentally: each test owns an isolated state root and opts
	// into inheritance only in its dedicated child-process fixtures.
	if err := os.Unsetenv(hostSlotsHeldEnv); err != nil {
		fmt.Fprintln(os.Stderr, "clear inherited host slots for tests:", err)
		os.Exit(1)
	}
	reclaimDeadTestStateDirs(os.TempDir())
	state, err := newTestStateDir()
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

func TestInheritedResourceChild(t *testing.T) {
	if os.Getenv(inheritedResourceChildEnv) == "" {
		t.Skip("inherited resource child")
	}
	repo := os.Getenv("GZ_GIT_TEST_INHERITED_RESOURCE_REPO")
	marker := os.Getenv("GZ_GIT_TEST_INHERITED_RESOURCE_MARKER")
	record := os.Getenv("GZ_GIT_TEST_INHERITED_RESOURCE_RECORD")
	dir, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(dir, integrateLockName+".json")
	beforeSidecar, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatalf("read parent slot sidecar: %v", err)
	}
	beforeTickets, err := liveAdmissionTickets(dir)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: repo, Branch: "dev/actor/feat/task", LockNotice: io.Discard})
	if err != nil || report == nil || !report.Ready {
		t.Fatalf("inherited child Check = %#v, %v", report, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("child gate marker: %v", err)
	}
	afterSidecar, err := os.ReadFile(sidecar)
	if err != nil || !bytes.Equal(afterSidecar, beforeSidecar) {
		t.Fatalf("child changed parent slot sidecar: before=%q after=%q err=%v", beforeSidecar, afterSidecar, err)
	}
	afterTickets, err := liveAdmissionTickets(dir)
	if err != nil || len(afterTickets) != len(beforeTickets) {
		t.Fatalf("child changed admission tickets: before=%#v after=%#v err=%v", beforeTickets, afterTickets, err)
	}
	if err := os.WriteFile(record, []byte(os.Getenv(hostSlotsHeldEnv)), 0o600); err != nil {
		t.Fatal(err)
	}
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

// measurementLockReadiness is the high-load readiness declaration the
// measurement-lock fixtures share; extra lines are appended under branch.
func measurementLockReadiness(timeout, extra string) string {
	return "branch:\n  integrationBranch: develop\n  resourceHighLoad: true\n" + extra +
		"  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    timeout: " + timeout + "\n"
}

// measurementLockFixture is a task branch on develop whose target declares a
// high-load readiness contract. Its runner runs script under sh -e and then
// reports ready, within the declared readiness timeout. gz-git runs no
// repository code without such a contract, so only a contract runner can
// hold the host measurement lock.
func measurementLockFixture(t *testing.T, script, timeout string) *testutil.WorktreeOrigin {
	t.Helper()
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "checkout", "-b", "develop")
	if err := os.MkdirAll(filepath.Join(fx.Clone, ".gz-git", "readiness"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, fx.Clone, ".gz-git.yaml", measurementLockReadiness(timeout, ""))
	writeRepoFile(t, fx.Clone, ".gz-git/readiness/check", "#!/bin/sh\nset -e\n"+script+"\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"ok\"}'\n")
	if err := os.Chmod(filepath.Join(fx.Clone, ".gz-git", "readiness", "check"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "declare high-load readiness")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", fx.Remote+"/develop")
	writeRepoFile(t, fx.Worktree, "task.txt", "task\n")
	runGit(t, fx.Worktree, "add", "task.txt")
	runGit(t, fx.Worktree, "commit", "-m", "task")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	return fx
}

// setMeasurementLockWait declares lockWait on the source side, where the
// effective wait is resolved; the readiness contract itself is unchanged.
func setMeasurementLockWait(t *testing.T, fx *testutil.WorktreeOrigin, wait string) {
	t.Helper()
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", measurementLockReadiness("1m", "  lockWait: "+wait+"\n"))
	runGit(t, fx.Worktree, "add", ".gz-git.yaml")
	runGit(t, fx.Worktree, "commit", "-m", "configure lock wait")
	runGit(t, fx.Worktree, "push")
}

func TestInheritedHostSlotPassesFromRunnerToChildCheck(t *testing.T) {
	fastHostLockPolling(t)
	state := t.TempDir()
	childMarker := filepath.Join(state, "child-ran")
	record := filepath.Join(state, "held-slots")
	child := measurementLockFixture(t, "touch "+strconv.Quote(childMarker), "1m")
	t.Setenv(inheritedResourceChildEnv, "1")
	t.Setenv("GZ_GIT_TEST_INHERITED_RESOURCE_REPO", child.Worktree)
	t.Setenv("GZ_GIT_TEST_INHERITED_RESOURCE_MARKER", childMarker)
	t.Setenv("GZ_GIT_TEST_INHERITED_RESOURCE_RECORD", record)
	// Existing names are retained once and the parent adds only the slot it
	// actually holds for the runner subprocess.
	t.Setenv(hostSlotsHeldEnv, "alpha, alpha")
	// The child's test output goes to stderr: the runner's stdout is its
	// json-v1 result.
	parentRecipe := strconv.Quote(os.Args[0]) + " -test.run=^TestInheritedResourceChild$ >&2"
	parent := measurementLockFixture(t, parentRecipe, "1m")

	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: parent.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard})
	if err != nil || report == nil || !report.Ready {
		t.Fatalf("parent Check = %#v, %v", report, err)
	}
	data, err := os.ReadFile(record)
	if err != nil || string(data) != "alpha,integrate" {
		t.Fatalf("child %s = %q, %v", hostSlotsHeldEnv, data, err)
	}
	if got := os.Getenv(hostSlotsHeldEnv); got != "alpha, alpha" {
		t.Fatalf("parent environment mutated: %q", got)
	}
	dir, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	tickets, err := liveAdmissionTickets(dir)
	if err != nil || len(tickets) != 0 {
		t.Fatalf("tickets after parent/child Check = %#v, %v", tickets, err)
	}
	if _, err := os.Stat(filepath.Join(dir, integrateLockName+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("slot sidecar remained after parent/child Check: %v", err)
	}
}

func TestReadinessRunnerReceivesHeldHostSlot(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	record := filepath.Join(t.TempDir(), "held-slots")
	fx := readinessFixture(t)
	manifest := "branch:\n  integrationBranch: develop\n  resourceHighLoad: true\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n"
	runner := "#!/bin/sh\nprintf '%s' \"$" + hostSlotsHeldEnv + "\" > " + strconv.Quote(record) + "\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"ready\"}'\n"
	advanceReadinessTarget(t, fx, manifest, runner, "declare high-load readiness")
	t.Setenv(hostSlotsHeldEnv, "alpha,alpha")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard})
	if err != nil || report == nil || !report.Ready {
		t.Fatalf("readiness Check = %#v, %v", report, err)
	}
	data, err := os.ReadFile(record)
	if err != nil || string(data) != "alpha,integrate" {
		t.Fatalf("readiness %s = %q, %v", hostSlotsHeldEnv, data, err)
	}
}

func TestReadinessRunnerDoesNotStartWhenTargetMovesDuringResourceWait(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	fastHostLockPolling(t)
	ran := filepath.Join(t.TempDir(), "runner-ran")
	fx := readinessFixture(t)
	manifest := "branch:\n  integrationBranch: develop\n  resourceHighLoad: true\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n"
	runner := "#!/bin/sh\ntouch " + strconv.Quote(ran) + "\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"ready\"}'\n"
	advanceReadinessTarget(t, fx, manifest, runner, "declare high-load readiness")
	lock := holdHostLockInProcess(t, "/elsewhere/repo", "dev/other")

	notice := &lockedBuffer{}
	done := make(chan checkResult, 1)
	go func() {
		report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
			RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: notice,
		})
		done <- checkResult{report, err}
	}()
	waitForText(t, "readiness waiting notice", notice.String, "waiting")
	moveMeasurementTarget(t, fx, "moved-readiness.txt")
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Check: %v", r.err)
		}
		if r.report.Ready || !hasCheckDetail(r.report, "readiness contract", checkFail, "target branch changed while waiting") {
			t.Fatalf("target movement must stop readiness before the runner:\n%s", FormatCheck(r.report))
		}
		if r.report.ResourceWaitDuration <= 0 {
			t.Fatalf("target movement lost resource wait duration: %s", r.report.ResourceWaitDuration)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("readiness Check did not stop after target movement")
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("readiness runner started after target movement: %v", err)
	}
	lock.release()
}

func TestReadinessRunnerDoesNotStartWhenResourceWaitIsCanceled(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("readiness runners are unsupported on this platform")
	}
	fastHostLockPolling(t)
	ran := filepath.Join(t.TempDir(), "runner-ran")
	fx := readinessFixture(t)
	manifest := "branch:\n  integrationBranch: develop\n  resourceHighLoad: true\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n"
	runner := "#!/bin/sh\ntouch " + strconv.Quote(ran) + "\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"ready\"}'\n"
	advanceReadinessTarget(t, fx, manifest, runner, "declare high-load readiness")
	holdHostLockInProcess(t, "/elsewhere/repo", "dev/other")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notice := &lockedBuffer{}
	done := make(chan checkResult, 1)
	go func() {
		report, err := Check(ctx, gitcmd.NewExecutor(), CheckOptions{
			RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: notice,
		})
		done <- checkResult{report, err}
	}()
	waitForText(t, "readiness waiting notice", notice.String, "waiting")
	cancel()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Check: %v", r.err)
		}
		if r.report.Ready || !hasCheckDetail(r.report, "readiness contract", checkFail, "context canceled") {
			t.Fatalf("canceled resource wait must fail readiness before runner:\n%s", FormatCheck(r.report))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("readiness Check did not stop after cancellation")
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("readiness runner started after cancellation: %v", err)
	}
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

func waitForText(t *testing.T, what string, read func() string, want string) {
	t.Helper()
	deadline := time.Now().Add(testWaitTimeout)
	for !strings.Contains(read(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("%s never contained %q within %s; got:\n%s", what, want, testWaitTimeout, read())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type checkResult struct {
	report *CheckReport
	err    error
}

// TestMeasurementLock runs two integrations of two different repositories at
// once. Every readiness runner claims one shared directory with an atomic mkdir
// and fails with 9 if another measurement already holds it, so the runs are
// both integrated only if no two measurements overlapped.
func TestMeasurementLock(t *testing.T) {
	busy := filepath.Join(t.TempDir(), "busy")
	recipe := "mkdir " + busy + " || exit 9; sleep 0.3; rmdir " + busy
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

// TestMeasurementLockBudget holds the lock longer than the declared readiness
// timeout. The waiting integration must still measure and pass: the budget
// clock starts when the lock is acquired.
func TestMeasurementLockBudget(t *testing.T) {
	fastHostLockPolling(t)
	fx := measurementLockFixture(t, "true", "1s")
	lock := holdHostLockInProcess(t, "/elsewhere/repo", "dev/other")
	const hold = 2500 * time.Millisecond
	notice := &lockedBuffer{}
	stop := make(chan struct{})
	released := make(chan struct{})
	go func() {
		defer close(released)
		// Start the held-slot delay only once Check reaches the lock. Git
		// preparation can take longer under a loaded host and is not resource wait.
		for !strings.Contains(notice.String(), "waiting") {
			select {
			case <-stop:
				lock.release()
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
		select {
		case <-stop:
		case <-time.After(hold):
		}
		lock.release()
	}()
	defer func() { close(stop); <-released }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	report, err := Check(ctx, gitcmd.NewExecutor(), CheckOptions{
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
		t.Fatalf("waiting %s for the lock was charged to the 1s readiness timeout\n%s", hold, FormatCheck(report))
	}
	if report.ResourceWaitDuration < time.Second {
		t.Fatalf("resource wait = %s, want a separately reported held-slot delay", report.ResourceWaitDuration)
	}
	if !strings.Contains(FormatCheck(report), "INFO  resource wait — Duration=") {
		t.Fatalf("formatted report omitted separate resource wait:\n%s", FormatCheck(report))
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
	fx := measurementLockFixture(t, "true", "1m")

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
	waitForText(t, "waiting notice", notice.String, childPID)
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

func moveMeasurementTarget(t *testing.T, fx *testutil.WorktreeOrigin, name string) {
	t.Helper()
	runGit(t, fx.Clone, "checkout", "develop")
	writeRepoFile(t, fx.Clone, name, "moved\n")
	runGit(t, fx.Clone, "add", name)
	runGit(t, fx.Clone, "commit", "-m", "target moves during resource wait")
	runGit(t, fx.Clone, "push", fx.Remote, "develop")
}

// TestMeasurementLockStopsWhenTargetMovesDuringOSWait detects a remote
// destination advance before the held OS slot is released. No gate command
// can start because the Check returns while the unrelated holder still owns
// capacity.
func TestMeasurementLockStopsWhenTargetMovesDuringOSWait(t *testing.T) {
	fastHostLockPolling(t)
	ran := filepath.Join(t.TempDir(), "gate-ran")
	fx := measurementLockFixture(t, "touch "+strconv.Quote(ran), "1m")
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
	waitForText(t, "waiting notice", notice.String, "waiting")
	moveMeasurementTarget(t, fx, "moved-os.txt")
	var r checkResult
	select {
	case r = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Check did not stop after target movement during OS wait")
	}
	if r.err != nil {
		t.Fatalf("Check: %v", r.err)
	}
	if r.report.Ready || !hasCheckDetail(r.report, "readiness contract", checkFail, "target branch changed while waiting") {
		t.Fatalf("target movement during a resource wait must require revalidation:\n%s", FormatCheck(r.report))
	}
	if r.report.ResourceWaitDuration <= 0 {
		t.Fatalf("target movement lost resource wait duration: %s", r.report.ResourceWaitDuration)
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("readiness runner started after target movement: %v", err)
	}
	lock.release()
}

// TestMeasurementLockStopsWhenTargetMovesDuringFIFOAdmission exercises the
// same check path before it reaches the OS lock. The first admission ticket
// deliberately stays live, so the check must observe the moved target from
// the FIFO wait loop and remove its own ticket.
func TestMeasurementLockStopsWhenTargetMovesDuringFIFOAdmission(t *testing.T) {
	fastHostLockPolling(t)
	ran := filepath.Join(t.TempDir(), "gate-ran")
	fx := measurementLockFixture(t, "touch "+strconv.Quote(ran), "1m")
	dir, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	release, err := acquireAdmission(context.Background(), dir, 0, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)

	done := make(chan checkResult, 1)
	go func() {
		report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
			RepoPath:   fx.Worktree,
			Branch:     "dev/actor/feat/task",
			LockNotice: io.Discard,
		})
		done <- checkResult{report, err}
	}()
	waitAdmissionTickets(t, 2)
	moveMeasurementTarget(t, fx, "moved-fifo.txt")
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Check: %v", r.err)
		}
		if r.report.Ready || !hasCheckDetail(r.report, "readiness contract", checkFail, "target branch changed while waiting") {
			t.Fatalf("target movement during FIFO admission must require revalidation:\n%s", FormatCheck(r.report))
		}
		if r.report.ResourceWaitDuration <= 0 {
			t.Fatalf("target movement lost resource wait duration: %s", r.report.ResourceWaitDuration)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Check did not stop after target movement during FIFO admission")
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("readiness runner started after target movement: %v", err)
	}
	tickets, err := liveAdmissionTickets(dir)
	if err != nil || len(tickets) != 1 {
		t.Fatalf("moved FIFO waiter ticket was not removed: %#v, %v", tickets, err)
	}
}

func TestTargetWaitObserverTreatsDeletedTargetAsMovement(t *testing.T) {
	fx := measurementLockFixture(t, "true", "1m")
	gitExecutor := gitcmd.NewExecutor()
	g := newGitRepo(gitExecutor, fx.Worktree)
	plan, err := resolveTarget(context.Background(), g, gitExecutor, CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", IntegrationConfig: []string{"develop"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Remote == "" {
		t.Fatalf("test requires remote target plan: %#v", plan)
	}
	runGit(t, fx.Clone, "push", fx.Remote, ":develop")
	if err := targetWaitObserver(g, plan, false, io.Discard)(context.Background()); !errors.Is(err, errTargetMovedWhileWaiting) {
		t.Fatalf("deleted target observer error = %v, want target movement", err)
	}
}

func TestTargetWaitObserverContinuesAfterOneFetchFailure(t *testing.T) {
	fx := measurementLockFixture(t, "true", "1m")
	runner := gitcmd.NewExecutor()
	g := newGitRepo(runner, fx.Worktree)
	plan, err := resolveTarget(context.Background(), g, runner, CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", IntegrationConfig: []string{"develop"}})
	if err != nil {
		t.Fatal(err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(t.TempDir(), "first-fetch-failed")
	args := filepath.Join(t.TempDir(), "fetch-args")
	wrapper := filepath.Join(t.TempDir(), "git")
	script := "#!/bin/sh\nif [ \"$1\" = fetch ]; then\n  printf '%s\\n' \"$*\" > " + strconv.Quote(args) + "\n  case \"$*\" in *--prune*) echo unexpected-prune >&2; exit 2;; esac\n  if [ ! -e " + strconv.Quote(state) + " ]; then\n    : > " + strconv.Quote(state) + "\n    echo transient-fetch >&2\n    exit 1\n  fi\nfi\nexec " + strconv.Quote(gitPath) + " \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	notice := &lockedBuffer{}
	observer := targetWaitObserver(newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(wrapper)), fx.Worktree), plan, false, notice)
	if err := observer(context.Background()); err != nil {
		t.Fatalf("first observer call: %v", err)
	}
	if err := observer(context.Background()); err != nil {
		t.Fatalf("observer did not continue after transient fetch failure: %v", err)
	}
	if got := strings.Count(notice.String(), "refresh target while waiting"); got != 1 {
		t.Fatalf("transient fetch warning count = %d, want 1; notice:\n%s", got, notice.String())
	}
	data, err := os.ReadFile(args)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "+refs/heads/develop:refs/remotes/origin/develop"; !strings.Contains(got, want) {
		t.Fatalf("wait refresh = %q, want only captured target refspec %q", got, want)
	}
}

func TestTargetWaitObserverTreatsNonFastForwardTargetAsMovement(t *testing.T) {
	fx := measurementLockFixture(t, "true", "1m")
	gitExecutor := gitcmd.NewExecutor()
	g := newGitRepo(gitExecutor, fx.Worktree)
	plan, err := resolveTarget(context.Background(), g, gitExecutor, CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", IntegrationConfig: []string{"develop"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Remote == "" {
		t.Fatalf("test requires remote target plan: %#v", plan)
	}
	runGit(t, fx.Clone, "checkout", "--orphan", "force-target")
	writeRepoFile(t, fx.Clone, "replacement.txt", "replacement\n")
	runGit(t, fx.Clone, "add", "replacement.txt")
	runGit(t, fx.Clone, "commit", "-m", "replace target history")
	runGit(t, fx.Clone, "push", "--force", fx.Remote, "HEAD:develop")
	if err := targetWaitObserver(g, plan, false, io.Discard)(context.Background()); !errors.Is(err, errTargetMovedWhileWaiting) {
		t.Fatalf("non-fast-forward target observer error = %v, want target movement", err)
	}
}

func TestResourceWaitObserverCannotExtendFiniteAcquisitionBudget(t *testing.T) {
	fastHostLockPolling(t)
	holder := holdHostLockInProcess(t, "/holder", "holder")
	defer holder.release()
	ctx := withResourceWaitObserver(context.Background(), func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	started := time.Now()
	_, err := acquireHostLockWithSource(ctx, hostLockHolder{Branch: "waiter"}, 75*time.Millisecond, "flag", io.Discard)
	if !errors.Is(err, ErrHostLockWait) || !strings.Contains(err.Error(), "flag; per acquisition") {
		t.Fatalf("finite wait error = %v, want source-labeled acquisition timeout", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("observer extended finite acquisition budget to %s", elapsed)
	}
}

// TestMeasurementLockWait gives up on a held lock after --lock-wait. Both
// commands must fail, name the holder, and start no readiness runner.
func TestMeasurementLockWait(t *testing.T) {
	fastHostLockPolling(t)
	ran := filepath.Join(t.TempDir(), "ran")
	fx := measurementLockFixture(t, "touch "+ran, "1m")
	holdHostLockInProcess(t, "/holder/repo", "dev/holder/branch")
	opts := CheckOptions{
		RepoPath:   fx.Worktree,
		Branch:     "dev/actor/feat/task",
		LockWait:   300 * time.Millisecond,
		LockNotice: io.Discard,
	}

	checkReport, checkErr := Check(context.Background(), gitcmd.NewExecutor(), opts)
	runReport, runErr := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: opts})
	if !errors.Is(checkErr, ErrHostLockWait) || checkReport == nil || checkReport.Ready || !hasCheckDetail(checkReport, "readiness contract", checkFail, "host measurement lock wait expired") {
		t.Fatalf("check = %#v, %v; want a not-ready resource-slot report", checkReport, checkErr)
	}
	if !hasCheckDetailParts(checkReport, "readiness contract", checkFail, "300ms", "flag", "per acquisition", "no gate measured") {
		t.Fatalf("check lock-wait diagnostic:\n%s", FormatCheck(checkReport))
	}
	if runErr == nil || runReport == nil || runReport.Check == nil || runReport.Check.Ready {
		t.Fatalf("run = %#v, %v; want not-ready after resource wait", runReport, runErr)
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a readiness runner ran without the lock (stat %s: %v)", ran, err)
	}
}

func TestEffectiveLockWaitPrecedence(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, ".gz-git.yaml", "branch:\n  lockWait: 45m\n")
	if wait, source, err := effectiveLockWait(CheckOptions{}, root); err != nil || wait != 45*time.Minute || source != "repository config" {
		t.Fatalf("repository lock wait = (%s, %q, %v)", wait, source, err)
	}
	if wait, source, err := effectiveLockWait(CheckOptions{LockWaitSet: true}, root); err != nil || wait != 0 || source != "flag" {
		t.Fatalf("explicit unbounded flag = (%s, %q, %v)", wait, source, err)
	}
	if wait, source, err := effectiveLockWait(CheckOptions{LockWait: time.Minute}, root); err != nil || wait != time.Minute || source != "flag" {
		t.Fatalf("flag wait = (%s, %q, %v)", wait, source, err)
	}
	if wait, source, err := effectiveLockWait(CheckOptions{}, t.TempDir()); err != nil || wait != defaultHostLockWait || source != "default" {
		t.Fatalf("default lock wait = (%s, %q, %v)", wait, source, err)
	}
	if _, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: root, LockWait: -time.Second}); err == nil || err.Error() != "--lock-wait must not be negative" {
		t.Fatalf("negative lock wait error = %v", err)
	}
}

func TestMeasurementLockConfiguredWait(t *testing.T) {
	fastHostLockPolling(t)
	ran := filepath.Join(t.TempDir(), "ran")
	fx := measurementLockFixture(t, "touch "+ran, "1m")
	setMeasurementLockWait(t, fx, "50ms")
	holdHostLockInProcess(t, "/holder/repo", "dev/holder/branch")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard})
	if !errors.Is(err, ErrHostLockWait) || report == nil || report.Ready || !hasCheckDetailParts(report, "readiness contract", checkFail, "50ms", "repository config", "per acquisition", "no gate measured") {
		t.Fatalf("configured Check = %#v, %v", report, err)
	}
	if _, statErr := os.Stat(ran); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("configured wait ran gate: %v", statErr)
	}
}

func TestMeasurementLockDefaultWait(t *testing.T) {
	fastHostLockPolling(t)
	oldDefault := defaultHostLockWait
	defaultHostLockWait = 50 * time.Millisecond
	t.Cleanup(func() { defaultHostLockWait = oldDefault })
	ran := filepath.Join(t.TempDir(), "ran")
	fx := measurementLockFixture(t, "touch "+ran, "1m")
	holdHostLockInProcess(t, "/holder/repo", "dev/holder/branch")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard})
	if !errors.Is(err, ErrHostLockWait) || report == nil || report.Ready || !hasCheckDetailParts(report, "readiness contract", checkFail, "50ms", "default", "per acquisition", "no gate measured") {
		t.Fatalf("default Check = %#v, %v", report, err)
	}
	if _, statErr := os.Stat(ran); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("default wait ran gate: %v", statErr)
	}
}

func hasCheckDetailParts(report *CheckReport, name, status string, want ...string) bool {
	for _, item := range report.Items {
		if item.Name != name || item.Status != status {
			continue
		}
		matched := true
		for _, part := range want {
			if !strings.Contains(item.Detail, part) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func TestMeasurementLockExplicitZeroOutlivesConfigAndDefault(t *testing.T) {
	fastHostLockPolling(t)
	oldDefault := defaultHostLockWait
	defaultHostLockWait = 20 * time.Millisecond
	t.Cleanup(func() { defaultHostLockWait = oldDefault })
	ran := filepath.Join(t.TempDir(), "ran")
	fx := measurementLockFixture(t, "touch "+ran, "1m")
	setMeasurementLockWait(t, fx, "10ms")
	lock := holdHostLockInProcess(t, "/holder", "holder")
	released := make(chan struct{})
	go func() {
		time.Sleep(60 * time.Millisecond)
		lock.release()
		close(released)
	}()
	started := time.Now()
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockWaitSet: true, LockNotice: io.Discard})
	<-released
	if err != nil || report == nil || !report.Ready || time.Since(started) < 60*time.Millisecond {
		t.Fatalf("explicit zero Check = %#v, %v after %s", report, err, time.Since(started))
	}
	if _, statErr := os.Stat(ran); statErr != nil {
		t.Fatalf("explicit zero did not run gate: %v", statErr)
	}
}

// A killed Git probe returns a nonzero Result, which must not become a
// fabricated target deletion when the observer's admission budget expires.
func TestTargetWaitObserverDeadlinePreservesResourceWaitDiagnostic(t *testing.T) {
	fastHostLockPolling(t)
	fx := measurementLockFixture(t, "true", "1m")
	runner := gitcmd.NewExecutor()
	plan, err := resolveTarget(context.Background(), newGitRepo(runner, fx.Worktree), runner, CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", IntegrationConfig: []string{"develop"}})
	if err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	holder := holdHostLockInProcess(t, "/holder", "holder")
	defer holder.release()
	for _, stage := range []string{"ls-remote", "fetch", "rev-parse"} {
		t.Run(stage, func(t *testing.T) {
			wrapper := filepath.Join(t.TempDir(), "git")
			script := "#!/bin/sh\nif [ \"$1\" = " + shellSingleQuoteForTest(stage) + " ]; then exec sleep 2; fi\nexec " + shellSingleQuoteForTest(realGit) + " \"$@\"\n"
			if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			observer := targetWaitObserver(newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(wrapper)), fx.Worktree), plan, false, io.Discard)
			ctx := withResourceWaitObserver(context.Background(), observer)
			_, err := acquireHostLockWithSource(ctx, hostLockHolder{Branch: "waiter"}, 150*time.Millisecond, "flag", io.Discard)
			if !errors.Is(err, ErrHostLockWait) || errors.Is(err, errTargetMovedWhileWaiting) || !strings.Contains(err.Error(), "150ms (flag; per acquisition); no gate measured") {
				t.Fatalf("%s deadline error = %v, want original resource wait diagnostic", stage, err)
			}
		})
	}
}

// shellSingleQuoteForTest quotes value as one POSIX shell word.
func shellSingleQuoteForTest(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
