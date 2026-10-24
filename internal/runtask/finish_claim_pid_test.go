// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// The FINISHING claim contract: liveness judgment, every answer a standing
// claim produces, and the lock reclamation finish depends on while a claim
// stands.

package runtask

import (
	"bytes"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// overrideClaimSeams stages the pid answers liveness judges, restoring the
// production seams when the test ends.
func overrideClaimSeams(t *testing.T, running func(int) bool, started func(int) (time.Time, error)) {
	t.Helper()
	origRunning, origStarted := pidRunning, pidStartedAt
	pidRunning, pidStartedAt = running, started
	t.Cleanup(func() { pidRunning, pidStartedAt = origRunning, origStarted })
}

// claimedExecution is a minimal ACTIVE execution whose owner matches the
// contract-fixture identity, so the seeded record passes owner checks.
func claimedExecution(fx wtContractRepo) TaskExecution {
	now := time.Now().UTC()
	return TaskExecution{
		Task: worktrunkContractTask, Type: TaskExecutionTypeChore, Purpose: string(TaskExecutionPurposeDelivery),
		Source: "main", Owner: TaskOwner{Actor: worktrunkContractActor, Host: worktrunkContractHost},
		Branch: fx.branch, Worktree: expectedWorktreePath(fx.worktreeRoot, fx.branch),
		StartedAt: now, UpdatedAt: now,
	}
}

// liveClaimReceipt is a FINISHING claim whose claimant is this test process:
// the pid runs on this host and started long before the claim was written, so
// the production seams judge it alive without any override.
func liveClaimReceipt(execution TaskExecution) TaskReceipt {
	return TaskReceipt{
		Task: execution.Task, Purpose: execution.Purpose, Operation: "finish", Status: TaskExecutionStatusFinishing,
		Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree,
		ProcessID: os.Getpid(), ProcessHost: lockHost(), CreatedAt: time.Now().UTC(),
	}
}

func TestFinishClaimLiveness(t *testing.T) {
	claim := TaskReceipt{Status: TaskExecutionStatusFinishing, ProcessID: 4242, ProcessHost: lockHost(), CreatedAt: time.Now().UTC().Add(-time.Minute)}
	alive := func(int) bool { return true }
	dead := func(int) bool { return false }
	startedBefore := func(int) (time.Time, error) { return claim.CreatedAt.Add(-time.Minute), nil }
	startedAfter := func(int) (time.Time, error) { return claim.CreatedAt.Add(claimStartSlack).Add(time.Minute), nil }
	startedErr := func(int) (time.Time, error) { return time.Time{}, errors.New("ps unavailable") }

	cases := []struct {
		name    string
		running func(int) bool
		started func(int) (time.Time, error)
		host    string
		pid     int
		want    claimLiveness
	}{
		{name: "alive claimant", running: alive, started: startedBefore, host: claim.ProcessHost, pid: 4242, want: claimAlive},
		{name: "pid gone", running: dead, started: startedBefore, host: claim.ProcessHost, pid: 4242, want: claimDead},
		{name: "reused pid", running: alive, started: startedAfter, host: claim.ProcessHost, pid: 4242, want: claimDead},
		{name: "pid unreported", running: alive, started: startedErr, host: claim.ProcessHost, pid: 4242, want: claimUnknown},
		{name: "no pid", running: alive, started: startedBefore, host: claim.ProcessHost, pid: 0, want: claimUnknown},
		{name: "no host", running: alive, started: startedBefore, host: "", pid: 4242, want: claimUnknown},
		{name: "foreign host", running: alive, started: startedBefore, host: "other-host", pid: 4242, want: claimUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			overrideClaimSeams(t, tc.running, tc.started)
			got := finishClaimLiveness(TaskReceipt{Status: TaskExecutionStatusFinishing, ProcessID: tc.pid, ProcessHost: tc.host, CreatedAt: claim.CreatedAt})
			if got != tc.want {
				t.Fatalf("finishClaimLiveness = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClaimedStateThroughDeriveState(t *testing.T) {
	execution := TaskExecution{Task: "claimed-task", Branch: "dev/a/h/chore/claimed-task", Owner: TaskOwner{Actor: "a", Host: "h"}}
	svc := &Service{}
	ctx := t.Context()

	t.Run("live claim keeps the execution active and finish-only", func(t *testing.T) {
		overrideClaimSeams(t, func(int) bool { return true }, func(int) (time.Time, error) { return time.Now().Add(-time.Hour), nil })
		claim := TaskReceipt{Status: TaskExecutionStatusFinishing, ProcessID: 4242, ProcessHost: lockHost(), CreatedAt: time.Now().UTC()}
		state := svc.deriveState(ctx, execution, claim, worktrunkInventory{}, false)
		if state.Status != TaskExecutionStatusActive {
			t.Fatalf("claimed status = %s, want ACTIVE", state.Status)
		}
		if strings.Join(state.AllowedActions, ",") != "run-status" {
			t.Fatalf("allowed actions = %v, want only run-status", state.AllowedActions)
		}
		if !strings.Contains(state.Reason, "run-finish pid=4242 host="+lockHost()) || !strings.Contains(state.Reason, "is integrating this execution") {
			t.Fatalf("claimed reason = %q, want the named live claimant", state.Reason)
		}
		if !strings.Contains(state.NextAction, "pid=4242 is running on this host") {
			t.Fatalf("claimed next action = %q, want the wait-for-finish answer", state.NextAction)
		}
	})
	t.Run("dead claim hands the execution to run-recover", func(t *testing.T) {
		overrideClaimSeams(t, func(int) bool { return false }, func(int) (time.Time, error) { return time.Time{}, errors.New("unreachable") })
		claim := TaskReceipt{Status: TaskExecutionStatusFinishing, ProcessID: 4242, ProcessHost: lockHost(), CreatedAt: time.Now().UTC().Add(-time.Hour)}
		state := svc.deriveState(ctx, execution, claim, worktrunkInventory{}, false)
		if state.Status != TaskExecutionStatusBlocked {
			t.Fatalf("dead-claim status = %s, want BLOCKED", state.Status)
		}
		if strings.Join(state.AllowedActions, ",") != "run-status,run-recover" {
			t.Fatalf("allowed actions = %v, want run-status and run-recover", state.AllowedActions)
		}
		if state.NextAction != "run gz-git run recover claimed-task" {
			t.Fatalf("dead-claim next action = %q", state.NextAction)
		}
	})
}

// TestRefuseClaimedMutationGates proves the claim gate on the three per-task
// mutations: abort and discard refuse a standing claim before touching any
// Git or Worktrunk state, recover without --claim-dead refuses the same way,
// and --claim-dead refuses a claimant it can see running on this host. The
// claimant is this test process, so liveness needs no seam override.
func TestRefuseClaimedMutationGates(t *testing.T) {
	svc, fx := newWorktrunkContractRepo(t, newWorktrunkStubRunner())
	ctx := t.Context()
	execution := claimedExecution(fx)
	claim := liveClaimReceipt(execution)
	store, err := svc.store(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.save([]TaskExecution{execution}); err != nil {
		t.Fatalf("seed execution: %v", err)
	}
	if err := store.appendReceipt(claim); err != nil {
		t.Fatalf("seed claim receipt: %v", err)
	}
	pid := strconv.Itoa(os.Getpid())

	abort, code := svc.Abort(ctx, worktrunkContractTask, "never integrates")
	if code != 1 || abort.Status != TaskExecutionStatusBlocked {
		t.Fatalf("abort code = %d status = %s, want 1/BLOCKED", code, abort.Status)
	}
	if !strings.Contains(abort.Reason, "run-abort refuses an execution that run-finish pid="+pid+" host="+lockHost()) {
		t.Fatalf("abort reason = %q, want the named claimant", abort.Reason)
	}
	if strings.Join(abort.AllowedActions, ",") != "run-status" {
		t.Fatalf("abort allowed actions = %v, want only run-status", abort.AllowedActions)
	}

	discard, code := svc.DiscardWithOptions(ctx, worktrunkContractTask, "clean it up", "", DiscardOptions{})
	if code != 1 || !strings.Contains(discard.Reason, "run-discard refuses an execution that run-finish pid="+pid) {
		t.Fatalf("discard = %d/%s, want the claim refusal", code, discard.Reason)
	}

	recovered, code := svc.Recover(ctx, worktrunkContractTask)
	if code != 1 || !strings.Contains(recovered.Reason, "run-recover refuses an execution that run-finish pid="+pid) {
		t.Fatalf("recover = %d/%s, want the claim refusal", code, recovered.Reason)
	}

	declared, code := svc.RecoverClaimDead(ctx, worktrunkContractTask)
	if code != 1 || declared.Reason != "run-recover --claim-dead refuses a claim whose run-finish pid="+pid+" is running on this host" {
		t.Fatalf("recover --claim-dead on a live claim = %d/%q, want the running-claimant refusal", code, declared.Reason)
	}
}

func TestStoreLockWait(t *testing.T) {
	t.Run("reclaims the lock once the holder releases", func(t *testing.T) {
		dir := t.TempDir()
		holder, waiter := newStateStore(dir), newStateStore(dir)
		held, err := holder.lock()
		if err != nil {
			t.Fatalf("holder lock: %v", err)
		}
		go func() {
			time.Sleep(50 * time.Millisecond)
			held()
		}()
		unlock, err := waiter.lockWait(t.Context(), 5*time.Second, 5*time.Millisecond)
		if err != nil {
			t.Fatalf("lockWait after release: %v", err)
		}
		unlock()
		if _, err := holder.lock(); err != nil {
			t.Fatalf("holder cannot re-acquire after the waiter released: %v", err)
		}
	})
	t.Run("timeout keeps the foreign lock intact", func(t *testing.T) {
		dir := t.TempDir()
		holder, waiter := newStateStore(dir), newStateStore(dir)
		held, err := holder.lock()
		if err != nil {
			t.Fatalf("holder lock: %v", err)
		}
		path := waiter.lockPath()
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read held lock: %v", err)
		}
		got, err := waiter.lockWait(t.Context(), 40*time.Millisecond, 5*time.Millisecond)
		if err == nil {
			got()
			t.Fatal("lockWait succeeded while another holder kept the lock")
		}
		if !errors.Is(err, errMutationLockExists) {
			t.Fatalf("lockWait error = %v, want the mutation-lock-exists sentinel", err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed waiter removed the foreign lock: %v", err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("failed waiter changed the foreign lock: %q -> %q", before, after)
		}
		held()
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("holder unlock left its lock behind: %v", statErr)
		}
	})
}

func TestStandingClaimWarnings(t *testing.T) {
	overrideClaimSeams(t, func(int) bool { return true }, func(int) (time.Time, error) { return time.Now().Add(-time.Hour), nil })
	live := TaskReceipt{Task: "b-claimed", Status: TaskExecutionStatusFinishing, ProcessID: 4242, ProcessHost: lockHost(), CreatedAt: time.Now().UTC()}
	dead := TaskReceipt{Task: "a-claimed", Status: TaskExecutionStatusFinishing, ProcessID: 4242, ProcessHost: lockHost(), CreatedAt: time.Now().UTC().Add(-2 * time.Hour)}
	idle := TaskReceipt{Task: "c-idle", Operation: "start", Status: TaskExecutionStatusActive}
	warnings := standingClaimWarnings(map[string]TaskReceipt{"b-claimed": live, "a-claimed": dead, "c-idle": idle})
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want one per standing claim only", warnings)
	}
	if !strings.HasPrefix(warnings[0], "task a-claimed is claimed by run-finish pid=4242") || !strings.Contains(warnings[0], "ended without recording an outcome") {
		t.Fatalf("dead-claim warning = %q", warnings[0])
	}
	if !strings.HasPrefix(warnings[1], "task b-claimed is claimed by run-finish pid=4242") || !strings.Contains(warnings[1], "is running on this host") {
		t.Fatalf("live-claim warning = %q", warnings[1])
	}
}

func TestStartExistingExecutionResume(t *testing.T) {
	owner := TaskOwner{Actor: "actor", Host: "host"}
	started := time.Now().UTC().Add(-time.Hour)
	// The branch carries the recorded purpose's namespace: the ported
	// purpose validation refuses a record whose branch contradicts it, the
	// same way the reference does, before any resume logic runs.
	record := func(kind TaskExecutionType, purpose TaskExecutionPurpose) TaskExecution {
		return TaskExecution{Task: "resumable", Type: kind, Purpose: string(purpose), Owner: owner, Branch: purpose.BranchPrefix() + "/actor/host/feat/resumable", Worktree: "/wt/resumable", StartedAt: started, UpdatedAt: started}
	}
	if got, resumed, refused := startExistingExecution("resumable", TaskExecutionTypeFeat, TaskExecutionPurposeDelivery, true, owner, []TaskExecution{record(TaskExecutionTypeFeat, TaskExecutionPurposeDelivery)}, map[string]TaskReceipt{}); resumed == nil || refused != nil || got != -1 {
		t.Fatalf("resume match = %d/%v/%v, want the existing execution handed back", got, resumed, refused)
	} else if resumed.Status != TaskExecutionStatusActive || resumed.Reason != "existing execution returned" {
		t.Fatalf("resume match = %s/%q, want ACTIVE existing execution returned", resumed.Status, resumed.Reason)
	}
	if _, resumed, _ := startExistingExecution("resumable", TaskExecutionTypeFeat, "", true, owner, []TaskExecution{record(TaskExecutionTypeFeat, TaskExecutionPurposeExperiment)}, map[string]TaskReceipt{}); resumed == nil {
		t.Fatal("resume without --purpose must match any recorded purpose")
	}
	if _, _, refused := startExistingExecution("resumable", TaskExecutionTypeFix, TaskExecutionPurposeDelivery, true, owner, []TaskExecution{record(TaskExecutionTypeFeat, TaskExecutionPurposeDelivery)}, map[string]TaskReceipt{}); refused == nil ||
		refused.Reason != "--resume cannot change the recorded execution type or purpose" {
		t.Fatalf("mismatched-type resume refusal = %v", refused)
	}
	if _, _, refused := startExistingExecution("resumable", TaskExecutionTypeFeat, TaskExecutionPurposeExperiment, true, owner, []TaskExecution{record(TaskExecutionTypeFeat, TaskExecutionPurposeDelivery)}, map[string]TaskReceipt{}); refused == nil ||
		refused.Reason != "--resume cannot change the recorded execution type or purpose" {
		t.Fatalf("mismatched-purpose resume refusal = %v", refused)
	}
	if _, _, refused := startExistingExecution("resumable", TaskExecutionTypeFeat, TaskExecutionPurposeDelivery, false, owner, []TaskExecution{record(TaskExecutionTypeFeat, TaskExecutionPurposeDelivery)}, map[string]TaskReceipt{}); refused == nil ||
		!strings.Contains(refused.Reason, "task already has an unfinished execution") || !strings.Contains(refused.NextAction, "--resume") {
		t.Fatalf("bare re-start refusal = %v, want the unfinished-execution answer naming --resume", refused)
	}
	if _, _, refused := startExistingExecution("resumable", TaskExecutionTypeFeat, TaskExecutionPurposeDelivery, true, TaskOwner{Actor: "other", Host: "host"}, []TaskExecution{record(TaskExecutionTypeFeat, TaskExecutionPurposeDelivery)}, map[string]TaskReceipt{}); refused == nil ||
		!strings.Contains(refused.Reason, "owned by") {
		t.Fatalf("foreign-owner resume refusal = %v, want the owner answer before any resume logic", refused)
	}
}

func TestLocalOnlyDiscardScope(t *testing.T) {
	execution := TaskExecution{Task: "discardable", Owner: TaskOwner{Actor: "actor", Host: "host"}, Branch: "dev/actor/host/feat/discardable", Worktree: "/wt/discardable", StartedAt: time.Now().UTC().Add(-time.Hour)}
	now := time.Now().UTC()

	t.Run("receipt records the scope and the unchecked remote", func(t *testing.T) {
		receipt := discardReceipt(execution, TaskExecutionStatusAborted, "reason", nil, nil, "discard completed", now, true)
		if receipt.DiscardScope != "local-only" || receipt.RemoteBranchStatus != "unchecked" {
			t.Fatalf("local-only receipt scope/status = %q/%q, want local-only/unchecked", receipt.DiscardScope, receipt.RemoteBranchStatus)
		}
		if !strings.HasSuffix(receipt.Reason, "; local resources only; remote branch unchecked and unchanged") {
			t.Fatalf("local-only receipt reason = %q, want the scope sentence", receipt.Reason)
		}
		plain := discardReceipt(execution, TaskExecutionStatusAborted, "reason", nil, nil, "discard completed", now, false)
		if plain.DiscardScope != "" || plain.RemoteBranchStatus != "" || plain.RemoteBranchRemoved {
			t.Fatalf("default-scope receipt = %q/%q/%v, want scope and remote answers untouched", plain.DiscardScope, plain.RemoteBranchStatus, plain.RemoteBranchRemoved)
		}
	})
	t.Run("validDiscardIntentScope", func(t *testing.T) {
		cases := []struct {
			name    string
			receipt TaskReceipt
			want    bool
		}{
			{name: "default scope without remote answer", receipt: TaskReceipt{}, want: true},
			{name: "default scope with remote answer", receipt: TaskReceipt{RemoteBranchStatus: "absent"}, want: false},
			{name: "default scope claiming removal", receipt: TaskReceipt{RemoteBranchRemoved: true}, want: false},
			{name: "local-only with unchecked", receipt: TaskReceipt{DiscardScope: "local-only", RemoteBranchStatus: "unchecked"}, want: true},
			{name: "local-only claiming absence", receipt: TaskReceipt{DiscardScope: "local-only", RemoteBranchStatus: "absent"}, want: false},
			{name: "unknown scope", receipt: TaskReceipt{DiscardScope: "regional"}, want: false},
		}
		for _, tc := range cases {
			if got := validDiscardIntentScope(tc.receipt); got != tc.want {
				t.Fatalf("%s: validDiscardIntentScope = %v, want %v", tc.name, got, tc.want)
			}
		}
	})
	t.Run("gate refuses a scope-changing retry and accepts the original scope", func(t *testing.T) {
		intent := TaskReceipt{Task: execution.Task, Operation: "discard", Status: TaskExecutionStatusBlocked, Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree, CreatedAt: now, DiscardScope: "local-only", RemoteBranchStatus: "unchecked"}
		receipts := map[string]TaskReceipt{execution.Task: intent}
		refused, code, gated := discardReceiptGate(execution.Task, execution, receipts, false)
		if !gated || code != 1 || refused.Reason != "discard scope differs from the recorded intent or completion" ||
			refused.NextAction != "retry with the original discard scope; local-only requires --local-only" {
			t.Fatalf("scope-changing retry gate = %d/%q/%q (gated %v)", code, refused.Reason, refused.NextAction, gated)
		}
		if _, code, gated := discardReceiptGate(execution.Task, execution, receipts, true); gated {
			t.Fatalf("retry with the recorded scope gated with code %d", code)
		}
	})
	t.Run("gate refuses an intent whose recorded evidence contradicts its scope", func(t *testing.T) {
		intent := TaskReceipt{Task: execution.Task, Operation: "discard", Status: TaskExecutionStatusBlocked, Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree, CreatedAt: now, RemoteBranchRemoved: true}
		refused, code, gated := discardReceiptGate(execution.Task, execution, map[string]TaskReceipt{execution.Task: intent}, false)
		if !gated || code != 1 || refused.Reason != "discard intent scope or remote evidence is invalid" {
			t.Fatalf("invalid-intent gate = %d/%q (gated %v)", code, refused.Reason, gated)
		}
	})
	t.Run("gate answers an idempotent re-discard with exit 0", func(t *testing.T) {
		performedBy := TaskOwner{Actor: "actor", Host: "host"}
		done := TaskReceipt{
			Task: execution.Task, Operation: "discard", Status: TaskExecutionStatusAborted, Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree, CreatedAt: now,
			PerformedBy: &performedBy, WorktreeRemoved: true, LocalBranchRemoved: true, RemoteBranchRemoved: true, RemoteBranchStatus: "absent",
		}
		refused, code, gated := discardReceiptGate(execution.Task, execution, map[string]TaskReceipt{execution.Task: done}, false)
		if !gated || code != 0 || refused.Status != TaskExecutionStatusAborted || !strings.Contains(refused.Reason, "already discarded") {
			t.Fatalf("re-discard gate = %d/%q (gated %v), want the idempotent exit 0", code, refused.Reason, gated)
		}
	})
}
