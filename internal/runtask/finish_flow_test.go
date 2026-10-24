// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// The finish claim in live use: a standing claim answers a second finish
// before any provider work, a failed relock leaves the claim standing with
// the provider's outcome unrecorded, and a local-only discard shapes its own
// command line.

package runtask

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// recordingEngine counts provider invocations and answers from its recorded
// result, so a test can prove the provider never ran behind a gate.
type recordingEngine struct {
	result CommandResult
	err    error
	runs   int
}

func (e *recordingEngine) Run(context.Context, string, string, bool) (CommandResult, error) {
	e.runs++
	return e.result, e.err
}

// lockTakingEngine stands in for the integration provider and takes the
// mutation lock while the lifecycle believes it released it, staging the
// relock timeout Finish must survive without recording an outcome.
type lockTakingEngine struct {
	store  stateStore
	unlock func()
	runs   int
}

func (e *lockTakingEngine) Run(context.Context, string, string, bool) (CommandResult, error) {
	e.runs++
	unlock, err := e.store.lock()
	if err != nil {
		return CommandResult{ExitCode: 1}, fmt.Errorf("stage the foreign lock: %w", err)
	}
	e.unlock = unlock
	return CommandResult{ExitCode: 1}, nil
}

func (e *lockTakingEngine) release() {
	if e.unlock != nil {
		e.unlock()
		e.unlock = nil
	}
}

func seedExecutionAndReceipt(t *testing.T, svc *Service, execution TaskExecution, receipts ...TaskReceipt) {
	t.Helper()
	store, err := svc.store(t.Context())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.save([]TaskExecution{execution}); err != nil {
		t.Fatalf("seed execution: %v", err)
	}
	for _, receipt := range receipts {
		if err := store.appendReceipt(receipt); err != nil {
			t.Fatalf("seed receipt %s %s: %v", receipt.Operation, receipt.Status, err)
		}
	}
}

// stubListEnvelope is the schema 2 `wt list --format=json` answer naming
// exactly the main checkout and one task worktree at their real paths.
func stubListEnvelope(mainPath, branch, worktree string) string {
	return fmt.Sprintf(`{"schema": 2, "repo": {"default_branch": "main"}, "items": [
		{"branch": "main", "worktree": {"path": %q}},
		{"branch": %q, "worktree": {"path": %q}}
	]}`, mainPath, branch, worktree)
}

func TestFinishRefusedWhileClaimStands(t *testing.T) {
	engine := &recordingEngine{result: CommandResult{ExitCode: 1}}
	svc, fx := newWorktrunkContractRepo(t, newWorktrunkStubRunner())
	svc.engine = engine
	execution := claimedExecution(fx)
	claim := liveClaimReceipt(execution)
	seedExecutionAndReceipt(t, svc, execution, claim)

	resp, code := svc.Finish(t.Context(), worktrunkContractTask)
	if code != 1 || resp.Status == TaskExecutionStatusDone {
		t.Fatalf("second finish = %s exit %d, want the refused answer", resp.Status, code)
	}
	if resp.FinishReady {
		t.Fatal("a claimed execution must not answer a second finish as finish-ready")
	}
	pid := strconv.Itoa(os.Getpid())
	if !strings.Contains(resp.Reason, "run-finish pid="+pid) || !strings.Contains(resp.Reason, "is integrating this execution") {
		t.Fatalf("second finish reason = %q, want the named live claimant", resp.Reason)
	}
	if engine.runs != 0 {
		t.Fatalf("the integration provider ran %d time(s); a standing claim must answer before any provider work", engine.runs)
	}
	store, err := svc.store(t.Context())
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	receipts, err := store.latestReceipts()
	if err != nil {
		t.Fatalf("read receipts: %v", err)
	}
	if len(receipts) != 1 || receipts[worktrunkContractTask].Status != TaskExecutionStatusFinishing {
		t.Fatalf("receipts after the refused finish = %+v, want the untouched claim only", receipts)
	}
}

// newFinishReadyRepo stages an execution the finish gate accepts: a real git
// worktree at the recorded path, clean, on its branch, pushed, and current
// with the source tip, with the stubbed inventory naming it. Git runs for
// real; only wt is stubbed.
func newFinishReadyRepo(t *testing.T, runner CommandRunner) (*Service, wtContractRepo) {
	t.Helper()
	svc, fx := newWorktrunkContractRepo(t, runner)
	worktree := expectedWorktreePath(fx.worktreeRoot, fx.branch)
	fixtureGit(t, fx.root, "worktree", "add", "-b", fx.branch, worktree, "main")
	fixtureGit(t, worktree, "push", "-q", "-u", "origin", fx.branch)
	if stub, ok := runner.(*worktrunkStubRunner); ok {
		stub.list.stdout = stubListEnvelope(fx.root, fx.branch, worktree)
	}
	return svc, fx
}

func TestFinishRelockTimeoutLeavesClaimStanding(t *testing.T) {
	svc, fx := newFinishReadyRepo(t, newWorktrunkStubRunner())
	execution := claimedExecution(fx)
	seedExecutionAndReceipt(t, svc, execution)
	store, err := svc.store(t.Context())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Shorten only this test's wait; the production bound stays in the source.
	origWait, origPoll := finishRelockWait, finishRelockPoll
	finishRelockWait, finishRelockPoll = 60*time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { finishRelockWait, finishRelockPoll = origWait, origPoll })

	engine := &lockTakingEngine{store: store}
	svc.engine = engine
	t.Cleanup(engine.release)

	resp, code := svc.Finish(t.Context(), worktrunkContractTask)
	if code != 1 || resp.Status != TaskExecutionStatusBlocked {
		t.Fatalf("finish with a lost lock = %s exit %d, want BLOCKED exit 1", resp.Status, code)
	}
	if !strings.Contains(resp.Reason, "integration provider exited but the mutation lock could not be reacquired") {
		t.Fatalf("finish reason = %q, want the relock failure", resp.Reason)
	}
	if engine.runs != 1 {
		t.Fatalf("provider ran %d time(s), want exactly the staged run", engine.runs)
	}
	if _, statErr := os.Stat(store.lockPath()); statErr != nil {
		t.Fatalf("the foreign lock disappeared during the failed relock: %v", statErr)
	}
	receipts, err := store.latestReceipts()
	if err != nil {
		t.Fatalf("read receipts: %v", err)
	}
	claim := receipts[worktrunkContractTask]
	if len(receipts) != 1 || claim.Status != TaskExecutionStatusFinishing || claim.ProcessID != os.Getpid() {
		t.Fatalf("receipts after the failed relock = %+v, want only this process's standing claim", receipts)
	}
}

// recordingStubRunner records every command and answers offline: a wt list
// returns an empty schema 2 envelope, git probes answer clean, on-branch, and
// locally absent, and everything else exits 0.
type recordingStubRunner struct {
	calls  []CommandRequest
	branch string
}

func (r *recordingStubRunner) Run(_ context.Context, req CommandRequest) (CommandResult, error) {
	r.calls = append(r.calls, req)
	result := CommandResult{Command: req.Command, Args: append([]string(nil), req.Args...), WorkDir: req.WorkDir, ExitCode: 0}
	switch {
	case req.Command == "wt":
		for _, arg := range req.Args {
			if arg == "list" {
				result.Stdout = `{"schema": 2, "repo": {"default_branch": "main"}, "items": []}`
			}
		}
	case req.Command == "git" && len(req.Args) > 0:
		switch req.Args[0] {
		case "branch":
			result.Stdout = r.branch + "\n"
		case "status":
		case "show-ref":
			result.ExitCode = 1 // the local branch is already gone
		}
	}
	return result, nil
}

func (r *recordingStubRunner) wtCalls() []CommandRequest {
	var out []CommandRequest
	for _, call := range r.calls {
		if call.Command == "wt" {
			out = append(out, call)
		}
	}
	return out
}

func (r *recordingStubRunner) gitCalls() []CommandRequest {
	var out []CommandRequest
	for _, call := range r.calls {
		if call.Command == "git" {
			out = append(out, call)
		}
	}
	return out
}

func TestLocalOnlyDiscardCommandLine(t *testing.T) {
	runner := &recordingStubRunner{branch: "dev/" + worktrunkContractActor + "/" + worktrunkContractHost + "/chore/local-only"}
	svc, fx := newWorktrunkContractRepo(t, runner)
	execution := TaskExecution{
		Task: "local-only", Type: TaskExecutionTypeChore, Purpose: string(TaskExecutionPurposeDelivery), Source: "main",
		Owner: TaskOwner{Actor: worktrunkContractActor, Host: worktrunkContractHost}, Branch: runner.branch,
		Worktree: expectedWorktreePath(fx.worktreeRoot, runner.branch), StartedAt: time.Now().UTC().Add(-time.Hour), UpdatedAt: time.Now().UTC(),
	}

	t.Run("remove carries --delete-branch before the path and runs offline", func(t *testing.T) {
		runner.calls = nil
		if _, err := svc.discardWorktrunkCommand(t.Context(), execution, true, "remove", "--no-hooks", "--foreground", "--format=json", "-y", "-D", execution.Worktree); err != nil {
			t.Fatalf("local-only wt remove: %v", err)
		}
		calls := runner.wtCalls()
		if len(calls) != 1 {
			t.Fatalf("wt calls = %d, want the single offline remove", len(calls))
		}
		args := calls[0].Args
		if len(args) < 2 || args[len(args)-1] != execution.Worktree || args[len(args)-2] != "--delete-branch" {
			t.Fatalf("remove argv = %q, want --delete-branch immediately before the worktree path", args)
		}
		for key, want := range map[string]string{
			"GIT_NO_LAZY_FETCH":             "1",
			"GIT_OPTIONAL_LOCKS":            "0",
			"WORKTRUNK_CONFIG_PATH":         os.DevNull,
			"WORKTRUNK_SYSTEM_CONFIG_PATH":  os.DevNull,
			"WORKTRUNK_PROJECT_CONFIG_PATH": os.DevNull,
		} {
			if got := calls[0].Env[key]; got != want {
				t.Fatalf("offline env %s = %q, want %q", key, got, want)
			}
		}
	})
	t.Run("evidence probes never read the remote", func(t *testing.T) {
		runner.calls = nil
		if _, err := svc.discardPreconditions(t.Context(), execution, execution.Worktree, true); err != nil {
			t.Fatalf("local-only preconditions: %v", err)
		}
		if _, err := svc.discardRemovalEvidence(t.Context(), execution, nil, true); err != nil {
			t.Fatalf("local-only removal evidence: %v", err)
		}
		if len(runner.gitCalls()) == 0 {
			t.Fatal("no git evidence ran; the assertion would pass vacuously")
		}
		for _, call := range runner.calls {
			if call.Command == "git" && len(call.Args) > 0 && call.Args[0] == "ls-remote" {
				t.Fatalf("a local-only discard ran a remote probe: git %v", call.Args)
			}
		}
		for _, call := range runner.gitCalls() {
			if call.Env["GIT_NO_LAZY_FETCH"] != "1" {
				t.Fatalf("git %v ran without the offline-observation env", call.Args)
			}
		}
	})
}
