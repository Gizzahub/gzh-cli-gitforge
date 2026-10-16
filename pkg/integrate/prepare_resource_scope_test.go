// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func prepareScopeQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func prepareScopeFixture(t *testing.T, declaration, recipe string, failSource bool) *testutil.WorktreeOrigin {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "checkout", "-b", "develop")
	writeFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n"+declaration)
	if err := os.MkdirAll(filepath.Join(fx.Clone, "ent"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, fx.Clone, "ent/.keep", "")
	writeFile(t, fx.Clone, ".gitignore", "ent/generated/\n")
	writeFile(t, fx.Clone, "Makefile", "check:\n\t"+recipe+"\n")
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "prepare resource fixture target")
	runGit(t, fx.Clone, "push", "-u", "origin", "develop")
	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	writeFile(t, fx.Worktree, "task.txt", "task\n")
	if failSource {
		writeFile(t, fx.Worktree, "Makefile", "check:\n\t"+recipe+"\n\t@false\n")
	}
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "prepare resource fixture source")
	runGit(t, fx.Worktree, "push", "-u", "origin", "HEAD")
	return fx
}

func startPrepareScopeCheck(t *testing.T, fx *testutil.WorktreeOrigin, opts CheckOptions) (<-chan checkResult, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	opts.RepoPath, opts.Branch = fx.Worktree, "dev/actor/feat/task"
	if opts.LockNotice == nil {
		opts.LockNotice = io.Discard
	}
	done := make(chan checkResult, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		report, err := Check(ctx, gitcmd.NewExecutor(), opts)
		done <- checkResult{report: report, err: err}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-stopped:
		case <-time.After(10 * time.Second):
			t.Error("own preparation check did not stop after cancellation")
		}
	})
	return done, cancel
}

func awaitPrepareScopeCheck(t *testing.T, done <-chan checkResult) checkResult {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(10 * time.Second):
		t.Fatal("preparation check did not complete")
		return checkResult{}
	}
}

func installPrepareScopeGo(t *testing.T, body string) {
	t.Helper()
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	// The preparation command is synthetic; unrelated go env probes retain
	// their real behavior and do not pretend to hold the preparation slot.
	bin := fakeGo(t, "if [ \"${1-}\" != generate ]; then exec "+prepareScopeQuote(realGo)+" \"$@\"; fi\n"+body)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestHeavyPreparationSourceAndLazyBaselineReleaseBeforeMakeAndCleanup(t *testing.T) {
	fastHostLockPolling(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv(hostSlotsHeldEnv, "aux")
	state := t.TempDir()
	record, violations := filepath.Join(state, "prepared"), filepath.Join(state, "violations")
	slots, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(slots, integrateLockName+".json")
	installPrepareScopeGo(t, "[ \"$HOST_SLOTS_HELD\" = aux,integrate ]; [ -f "+prepareScopeQuote(sidecar)+" ]; printf '%s\\n' \"$PWD\" >> "+prepareScopeQuote(record))
	recipe := "@if [ -f " + prepareScopeQuote(sidecar) + " ] || [ \"$${HOST_SLOTS_HELD-}\" != aux ]; then echo make-held >> " + prepareScopeQuote(violations) + "; fi"
	fx := prepareScopeFixture(t, "  prepareProfile: familybook-ent-v1\n  prepareHighLoad: true\n", recipe, true)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(state, "git")
	body := "#!/bin/sh\nset -eu\nif [ \"${1-}\" = worktree ] && [ -f " + prepareScopeQuote(sidecar) + " ]; then echo worktree-held >> " + prepareScopeQuote(violations) + "; exit 90; fi\nexec " + prepareScopeQuote(realGit) + " \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	report, err := Check(context.Background(), gitcmd.NewExecutor(gitcmd.WithGitBinary(wrapper)), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard})
	if err != nil || report == nil || report.Ready {
		t.Fatalf("failing source/passing baseline Check = %#v, %v", report, err)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	trees := strings.Fields(string(data))
	if len(trees) != 2 || filepath.Base(trees[0]) != "source" || filepath.Base(trees[1]) != "target" {
		t.Fatalf("actual preparation executions = %q, want source then lazy target", data)
	}
	for _, tree := range trees {
		if _, err := os.Stat(tree); !os.IsNotExist(err) {
			t.Fatalf("prepared worktree survived: %s (%v)", tree, err)
		}
	}
	if _, err := os.Stat(filepath.Dir(trees[0])); !os.IsNotExist(err) {
		t.Fatalf("prepared root survived: %v", err)
	}
	if _, err := os.Stat(violations); !os.IsNotExist(err) {
		t.Fatalf("Make or worktree materialization/cleanup held a preparation slot: %v", err)
	}
	if report.ResourceWaitDuration <= 0 || os.Getenv(hostSlotsHeldEnv) != "aux" {
		t.Fatalf("wait or parent environment invalid: wait=%s env=%q", report.ResourceWaitDuration, os.Getenv(hostSlotsHeldEnv))
	}
	assertPrepareScopeTickets(t, 0)
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("preparation holder remained: %v", err)
	}
}

func assertPrepareScopeTickets(t *testing.T, want int) {
	t.Helper()
	dir, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	tickets, err := liveAdmissionTickets(dir)
	if err != nil || len(tickets) != want {
		t.Fatalf("remaining tickets = %#v, %v; want %d", tickets, err, want)
	}
}

func TestHeavyPreparationWaitAllowsLightIntegrationAndCancellation(t *testing.T) {
	fastHostLockPolling(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv(hostSlotsHeldEnv, "aux")
	marker := filepath.Join(t.TempDir(), "prepare-ran")
	installPrepareScopeGo(t, "touch "+prepareScopeQuote(marker))
	heavy := prepareScopeFixture(t, "  prepareProfile: familybook-ent-v1\n  prepareHighLoad: true\n", "@true", false)
	light := readyTaskFixture(t)
	holder := holdHostLockInProcess(t, "/other/isolated-repository", "dev/other")
	notice := &lockedBuffer{}
	done, cancel := startPrepareScopeCheck(t, heavy, CheckOptions{LockNotice: notice})
	waitForText(t, "heavy preparation admission", notice.String, "waiting")
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{RepoPath: light.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard}})
	if err != nil || !report.Integrated {
		t.Fatalf("light integration while preparation waits: %#v, %v", report, err)
	}
	cancel()
	result := awaitPrepareScopeCheck(t, done)
	if result.report != nil && result.report.Ready {
		t.Fatal("canceled preparation was reported ready")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("canceled waiter executed preparation: %v", err)
	}
	assertPrepareScopeTickets(t, 0)
	holder.release()
}

func TestHeavyPreparationAndGateContendWithoutBlockingLightIntegration(t *testing.T) {
	fastHostLockPolling(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv(hostSlotsHeldEnv, "aux")
	state := t.TempDir()
	started, release, gateMarker := filepath.Join(state, "prepare-started"), filepath.Join(state, "release"), filepath.Join(state, "gate-ran")
	slots, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	body := "[ \"$HOST_SLOTS_HELD\" = aux,integrate ]; [ -f " + prepareScopeQuote(filepath.Join(slots, integrateLockName+".json")) + " ]; touch " + prepareScopeQuote(started) + "; while [ ! -f " + prepareScopeQuote(release) + " ]; do sleep 0.02; done"
	installPrepareScopeGo(t, body)
	fx := prepareScopeFixture(t, "  prepareProfile: familybook-ent-v1\n  prepareHighLoad: true\n", "@true", false)
	gate := measurementLockFixture(t, "@touch "+prepareScopeQuote(gateMarker), "1m")
	light := readyTaskFixture(t)
	prepDone, _ := startPrepareScopeCheck(t, fx, CheckOptions{})
	waitForText(t, "actual preparation", func() string {
		if _, err := os.Stat(started); err == nil {
			return "started"
		}
		return ""
	}, "started")
	gateDone, _ := startPrepareScopeCheck(t, gate, CheckOptions{})
	waitAdmissionTickets(t, 1)
	if _, err := os.Stat(gateMarker); !os.IsNotExist(err) {
		t.Fatalf("heavy gate ran while preparation owned capacity: %v", err)
	}
	run, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{RepoPath: light.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard}})
	if err != nil || !run.Integrated {
		t.Fatalf("light Run while heavy preparation executes = %#v, %v", run, err)
	}
	if err := os.WriteFile(release, []byte("release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, done := range []<-chan checkResult{prepDone, gateDone} {
		r := awaitPrepareScopeCheck(t, done)
		if r.err != nil || r.report == nil || !r.report.Ready {
			t.Fatalf("released heavy Check = %#v, %v", r.report, r.err)
		}
	}
	if _, err := os.Stat(gateMarker); err != nil {
		t.Fatal(err)
	}
	assertPrepareScopeTickets(t, 0)
}

func TestPreparationDeclarationWithoutProfileDoesNotAcquireSlot(t *testing.T) {
	fastHostLockPolling(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	fx := prepareScopeFixture(t, "  prepareHighLoad: true\n", "@true", false)
	holder := holdHostLockInProcess(t, "/other/isolated-repository", "dev/other")
	done, _ := startPrepareScopeCheck(t, fx, CheckOptions{LockWait: time.Millisecond, LockWaitSet: true})
	r := awaitPrepareScopeCheck(t, done)
	if r.err != nil || r.report == nil || !r.report.Ready || r.report.ResourceWaitDuration != 0 {
		t.Fatalf("no-profile Check acquired preparation capacity: %#v, %v", r.report, r.err)
	}
	assertPrepareScopeTickets(t, 0)
	holder.release()
}

func TestControllerOnlyPreparationReceivesDeclaredSlot(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv(hostSlotsHeldEnv, "aux")
	marker := filepath.Join(t.TempDir(), "prepared")
	installPrepareScopeGo(t, "[ \"$HOST_SLOTS_HELD\" = aux,integrate ]; touch "+prepareScopeQuote(marker))
	fx := prepareScopeFixture(t, "  prepareHighLoad: true\n", "@true", false)
	controller := filepath.Join(t.TempDir(), "controller.yaml")
	remoteURL := strings.TrimSpace(runGitInTest(t, fx.Worktree, "remote", "get-url", "origin"))
	body := fmt.Sprintf("workspaces:\n  fixture:\n    url: %q\n    branch:\n      integrationBranch: develop\n      taskPattern: dev/*\n    integration:\n      prepareProfile: familybook-ent-v1\n", remoteURL)
	writeFile(t, filepath.Dir(controller), filepath.Base(controller), body)
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", ControllerConfig: controller, LockNotice: io.Discard})
	if err != nil || report == nil || !report.Ready {
		t.Fatalf("controller-only preparation Check = %#v, %v", report, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
	assertPrepareScopeTickets(t, 0)
}

func TestHeavyPreparationAdmissionBudgetRemovesOnlyItsFIFOTicket(t *testing.T) {
	fastHostLockPolling(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	state := t.TempDir()
	prepared, measured := filepath.Join(state, "prepare-ran"), filepath.Join(state, "gate-ran")
	installPrepareScopeGo(t, "touch "+prepareScopeQuote(prepared))
	fx := prepareScopeFixture(t, "  prepareProfile: familybook-ent-v1\n  prepareHighLoad: true\n", "@true", false)
	gate := measurementLockFixture(t, "@touch "+prepareScopeQuote(measured), "1m")
	holder := holdHostLockInProcess(t, "/other/isolated-repository", "dev/other")
	gateDone, cancelGate := startPrepareScopeCheck(t, gate, CheckOptions{})
	waitAdmissionTickets(t, 1)
	const budget = 120 * time.Millisecond
	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockWait: budget, LockWaitSet: true, LockNotice: io.Discard})
	if !errors.Is(err, ErrHostLockWait) || report == nil || report.Ready {
		t.Fatalf("preparation admission timeout = %#v, %v", report, err)
	}
	text := FormatCheck(report)
	for _, want := range []string{"120ms", "flag", "per acquisition", "no gate measured"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing original acquisition budget diagnostic %q:\n%s", want, text)
		}
	}
	if report.ResourceWaitDuration < budget {
		t.Fatalf("resource wait lost admission duration: %s", report.ResourceWaitDuration)
	}
	assertPrepareScopeTickets(t, 1)
	for _, marker := range []string{prepared, measured} {
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("queued preparation/gate executed before capacity was released: %s (%v)", marker, err)
		}
	}
	cancelGate()
	r := awaitPrepareScopeCheck(t, gateDone)
	if r.report != nil && r.report.Ready {
		t.Fatal("canceled older gate was reported ready")
	}
	assertPrepareScopeTickets(t, 0)
	holder.release()
}

func TestHeavyPreparationCancellationStopsRunningDescendantBeforeRelease(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	started := filepath.Join(t.TempDir(), "prepare-started")
	// The child inherits the output pipe. Killing only its shell parent
	// would leave Cmd.Wait and the resource slot waiting for the child.
	installPrepareScopeGo(t, "sleep 4 &\ntouch "+prepareScopeQuote(started)+"\nwait")
	fx := prepareScopeFixture(t, "  prepareProfile: familybook-ent-v1\n  prepareHighLoad: true\n", "@true", false)
	done, cancel := startPrepareScopeCheck(t, fx, CheckOptions{})
	waitForText(t, "running preparation", func() string {
		if _, err := os.Stat(started); err == nil {
			return "started"
		}
		return ""
	}, "started")
	begin := time.Now()
	cancel()
	r := awaitPrepareScopeCheck(t, done)
	if elapsed := time.Since(begin); elapsed > 2*time.Second {
		t.Fatalf("cancellation waited for the preparation descendant: %s", elapsed)
	}
	if r.report != nil && r.report.Ready {
		t.Fatal("canceled running preparation was reported ready")
	}
	assertPrepareScopeTickets(t, 0)
	slots, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(slots, integrateLockName+".json")); !os.IsNotExist(err) {
		t.Fatalf("canceled preparation still owns the slot: %v", err)
	}
}
