// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

// TestValidationDurationMeasuresContractRunnersOnly pins TASK-304's
// contract for CheckReport.ValidationDuration: the accumulator counts only
// time spent inside real gate subprocesses — each contract runner execution —
// never slot admission, probe, or preparation time, and never fabricates a
// value for a gate that launched nothing (gate mode none launches nothing).
func TestValidationDurationMeasuresContractRunnersOnly(t *testing.T) {
	t.Run("no readiness contract never launches a subprocess and stays zero", func(t *testing.T) {
		// Gate mode none runs no repository code: even a Makefile that would
		// sleep is never invoked, so validation time is exactly zero.
		fx := testutil.TempWorktreeWithBareOrigin(t)
		writeRepoFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n")
		writeRepoFile(t, fx.Clone, "Makefile", "check:\n\t@sleep 0.15\n")
		runGit(t, fx.Clone, "add", ".")
		runGit(t, fx.Clone, "commit", "-m", "makefile without contract")
		runGit(t, fx.Clone, "branch", "develop")
		runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
		runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
		writeRepoFile(t, fx.Worktree, "task.txt", "task\n")
		runGit(t, fx.Worktree, "add", "task.txt")
		runGit(t, fx.Worktree, "commit", "-m", "task")
		runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")

		report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
			RepoPath: fx.Worktree, Branch: "dev/actor/feat/task",
		})
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if !report.Ready || report.GateMode != gateModeNone {
			t.Fatalf("want a READY gate in mode none:\n%s", FormatCheck(report))
		}
		if report.ValidationDuration != 0 {
			t.Fatalf("validation duration = %s, want exactly 0 when no gate subprocess ran", report.ValidationDuration)
		}
	})

	t.Run("contract runner accumulates exactly its own execution", func(t *testing.T) {
		fx := readinessFixture(t)
		advanceReadinessTarget(t, fx, "", "#!/bin/sh\nsleep 0.15\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"ok\"}'\n", "slow readiness runner")
		report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
			RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", IntegrationConfig: []string{"develop"},
		})
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if !report.Ready || report.GateMode != "contract-v1" || report.ReadinessStatus != "ready" {
			t.Fatalf("want a READY contract gate:\n%s", FormatCheck(report))
		}
		if report.ReadinessDuration < 140*time.Millisecond {
			t.Fatalf("readiness duration = %s, want at least the runner's 150ms sleep", report.ReadinessDuration)
		}
		// In contract mode the one gate subprocess is the runner, measured
		// after slot acquisition; ReadinessDuration still spans the JSON
		// decode around it, so validation never exceeds it.
		if report.ValidationDuration > report.ReadinessDuration {
			t.Fatalf("validation duration = %s, want at most readiness duration %s", report.ValidationDuration, report.ReadinessDuration)
		}
		if report.ResourceWaitDuration != 0 {
			t.Fatalf("resource wait = %s, want 0 without a high-load declaration", report.ResourceWaitDuration)
		}
	})

	t.Run("contract runner failure still accumulates", func(t *testing.T) {
		fx := readinessFixture(t)
		advanceReadinessTarget(t, fx, "", "#!/bin/sh\nsleep 0.1\nexit 1\n", "failing readiness runner")
		report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
			RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", IntegrationConfig: []string{"develop"},
		})
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if report.Ready || !hasCheckDetail(report, "readiness contract", checkFail, "runner failed") {
			t.Fatalf("the failing runner must fail the gate:\n%s", FormatCheck(report))
		}
		if report.ValidationDuration < 90*time.Millisecond {
			t.Fatalf("validation duration = %s, want at least the failed runner's 100ms run", report.ValidationDuration)
		}
	})

	t.Run("a real high-load slot wait stays outside the contract runner's validation time", func(t *testing.T) {
		if !readinessRunnerSupported() {
			t.Skip("readiness runners are unsupported on this platform")
		}
		fastHostLockPolling(t)
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		fx := readinessFixture(t)
		manifest := "branch:\n  integrationBranch: develop\n  resourceHighLoad: true\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n"
		runner := "#!/bin/sh\nsleep 0.15\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"ready\"}'\n"
		advanceReadinessTarget(t, fx, manifest, runner, "declare high-load readiness")
		holder := holdHostLockInProcess(t, "/elsewhere/repo", "dev/other")
		notice := &lockedBuffer{}
		done := make(chan checkResult, 1)
		go func() {
			report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: notice})
			done <- checkResult{report, err}
		}()
		waitForText(t, "readiness waiting notice", notice.String, "waiting")
		// Release promptly once the check is verifiably queued; the holder's
		// deferred cleanup release was installed before any assertion.
		holder.release()
		var report *CheckReport
		select {
		case r := <-done:
			if r.err != nil {
				t.Fatalf("Check: %v", r.err)
			}
			report = r.report
		case <-time.After(10 * time.Second):
			t.Fatal("readiness Check did not finish after the slot was released")
		}
		if report == nil || !report.Ready || report.GateMode != "contract-v1" {
			t.Fatalf("want a READY contract gate after admission:\n%s", FormatCheck(report))
		}
		// The admission wait is real and measured, but its length is not the
		// assertion — only that it happened (nonzero).
		if report.ResourceWaitDuration <= 0 {
			t.Fatalf("resource wait = %s, want a measured nonzero admission wait", report.ResourceWaitDuration)
		}
		if report.ReadinessDuration < 140*time.Millisecond {
			t.Fatalf("readiness duration = %s, want at least the runner's 150ms sleep", report.ReadinessDuration)
		}
		// Load-robust core: validation is the started runner process alone,
		// measured after admission, while ReadinessDuration still spans the
		// decode around it — so validation never exceeds readiness, whatever
		// a loaded worker does to either wall clock.
		if report.ValidationDuration > report.ReadinessDuration {
			t.Fatalf("validation duration = %s, want at most readiness duration %s", report.ValidationDuration, report.ReadinessDuration)
		}
		// The run protocol copies both durations from this real run as
		// nonzero integer milliseconds.
		data, err := json.Marshal(BuildRunProtocol(&RunReport{Check: report, Integrated: true}, nil, 0))
		if err != nil {
			t.Fatalf("marshal run protocol: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("unmarshal run protocol: %v", err)
		}
		if wait, ok := doc["resource_wait_duration_ms"].(float64); !ok || int(wait) != int(report.ResourceWaitDuration.Milliseconds()) || wait <= 0 {
			t.Fatalf("resource_wait_duration_ms = %#v, want the run's nonzero %dms", doc["resource_wait_duration_ms"], report.ResourceWaitDuration.Milliseconds())
		}
		if validation, ok := doc["validation_duration_ms"].(float64); !ok || int(validation) != int(report.ValidationDuration.Milliseconds()) || validation <= 0 {
			t.Fatalf("validation_duration_ms = %#v, want the run's nonzero %dms", doc["validation_duration_ms"], report.ValidationDuration.Milliseconds())
		}
	})

	t.Run("a contract runner that never starts leaves validation exactly zero", func(t *testing.T) {
		if !readinessRunnerSupported() {
			t.Skip("readiness runners are unsupported on this platform")
		}
		fx := readinessFixture(t)
		// The blob is a valid executable regular file whose shebang points
		// nowhere, so contract loading succeeds and cmd.Start itself fails
		// without ever launching a process.
		advanceReadinessTarget(t, fx, "", "#!/nonexistent/readiness-interp\n", "unstartable readiness runner")
		report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
			RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", IntegrationConfig: []string{"develop"},
		})
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if report.Ready || !hasCheckDetail(report, "readiness contract", checkFail, "runner failed") {
			t.Fatalf("the unstartable runner must fail the gate:\n%s", FormatCheck(report))
		}
		// The outer readiness span still records the attempt, proving the
		// zero comes from the Start boundary, not a skipped execution.
		if report.ReadinessDuration <= 0 {
			t.Fatalf("readiness duration = %s, want the outer attempt span recorded", report.ReadinessDuration)
		}
		if report.ValidationDuration != 0 {
			t.Fatalf("validation duration = %s, want exactly 0 when no runner process ever started", report.ValidationDuration)
		}
	})
}
