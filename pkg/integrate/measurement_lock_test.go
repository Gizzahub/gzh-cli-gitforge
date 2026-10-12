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
)

// TestMain points the host measurement lock at a private directory, so no
// test in this package ever takes, or waits on, this host's real lock.
func TestMain(m *testing.M) {
	if os.Getenv(hostLockChildEnv) != "" || os.Getenv(admissionChildEnv) != "" || os.Getenv(inheritedResourceChildEnv) != "" {
		os.Exit(m.Run())
	}
	// A caller may itself inherit a real host slot. Package tests must never
	// borrow it accidentally: each test owns an isolated state root and opts
	// into inheritance only in its dedicated child-process fixtures.
	if err := os.Unsetenv(hostSlotsHeldEnv); err != nil {
		fmt.Fprintln(os.Stderr, "clear inherited host slots for tests:", err)
		os.Exit(1)
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
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  makeTimeout: "+budget+"\n  resourceHighLoad: true\n")
	writeRepoFile(t, fx.Worktree, "task.txt", "task\n")
	runGit(t, fx.Worktree, "add", ".gz-git.yaml", "task.txt")
	runGit(t, fx.Worktree, "commit", "-m", "task")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	return fx
}

func setMeasurementLockWait(t *testing.T, fx *testutil.WorktreeOrigin, wait string) {
	t.Helper()
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  makeTimeout: 1m\n  resourceHighLoad: true\n  lockWait: "+wait+"\n")
	runGit(t, fx.Worktree, "add", ".gz-git.yaml")
	runGit(t, fx.Worktree, "commit", "-m", "configure lock wait")
	runGit(t, fx.Worktree, "push")
}

func TestInheritedHostSlotPassesFromMakeToChildCheck(t *testing.T) {
	fastHostLockPolling(t)
	state := t.TempDir()
	childMarker := filepath.Join(state, "child-ran")
	record := filepath.Join(state, "held-slots")
	child := measurementLockFixture(t, "@touch "+strconv.Quote(childMarker), "1m")
	t.Setenv(inheritedResourceChildEnv, "1")
	t.Setenv("GZ_GIT_TEST_INHERITED_RESOURCE_REPO", child.Worktree)
	t.Setenv("GZ_GIT_TEST_INHERITED_RESOURCE_MARKER", childMarker)
	t.Setenv("GZ_GIT_TEST_INHERITED_RESOURCE_RECORD", record)
	// Existing names are retained once and the parent adds only the slot it
	// actually holds for the make subprocess.
	t.Setenv(hostSlotsHeldEnv, "alpha, alpha")
	parentRecipe := "@" + strconv.Quote(os.Args[0]) + " -test.run=^TestInheritedResourceChild$"
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
	if report.ResourceWaitDuration < time.Second {
		t.Fatalf("resource wait = %s, want a separately reported held-slot delay", report.ResourceWaitDuration)
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
	lock.release()

	r := <-done
	if r.err != nil {
		t.Fatalf("Check: %v", r.err)
	}
	if r.report.Ready || !hasCheckDetail(r.report, "post-gate freshness", checkFail, "target branch changed") {
		t.Fatalf("target movement during a resource wait must require revalidation:\n%s", FormatCheck(r.report))
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
	if checkErr != nil || checkReport == nil || checkReport.Ready || !hasCheckDetail(checkReport, "make check", checkFail, "resource slot") {
		t.Fatalf("check = %#v, %v; want a not-ready resource-slot report", checkReport, checkErr)
	}
	if !hasCheckDetailParts(checkReport, "make check", checkFail, "300ms", "flag", "per acquisition", "no gate measured") {
		t.Fatalf("check lock-wait diagnostic:\n%s", FormatCheck(checkReport))
	}
	if runErr == nil || runReport == nil || runReport.Check == nil || runReport.Check.Ready {
		t.Fatalf("run = %#v, %v; want not-ready after resource wait", runReport, runErr)
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a make target ran without the lock (stat %s: %v)", ran, err)
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
	fx := measurementLockFixture(t, "@touch "+ran, "1m")
	setMeasurementLockWait(t, fx, "50ms")
	holdHostLockInProcess(t, "/holder/repo", "dev/holder/branch")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard})
	if err != nil || report == nil || report.Ready || !hasCheckDetailParts(report, "make check", checkFail, "50ms", "repository config", "per acquisition", "no gate measured") {
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
	fx := measurementLockFixture(t, "@touch "+ran, "1m")
	holdHostLockInProcess(t, "/holder/repo", "dev/holder/branch")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard})
	if err != nil || report == nil || report.Ready || !hasCheckDetailParts(report, "make check", checkFail, "50ms", "default", "per acquisition", "no gate measured") {
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
	fx := measurementLockFixture(t, "@touch "+ran, "1m")
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
