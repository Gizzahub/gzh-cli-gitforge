// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Exit-3 receipt recognition and purpose-aware state: the recovery gate must
// read exactly what the provider stamps, and a non-delivery execution must
// never be offered run-finish.

package runtask

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// partialCleanupExecution is a delivery execution whose identity the
// recognition gate binds receipts against.
func partialCleanupExecution() TaskExecution {
	now := time.Now().UTC()
	return TaskExecution{
		Task: "cleanup-fail", Type: TaskExecutionTypeChore, Purpose: string(TaskExecutionPurposeDelivery), Source: "main",
		Owner: TaskOwner{Actor: "actor", Host: "host"}, Branch: "dev/actor/host/chore/cleanup-fail",
		Worktree: "/wt/actor__host__chore__cleanup-fail", StartedAt: now, UpdatedAt: now,
	}
}

// partialCleanupReceipt builds the exit-3 finish receipt recognition reads,
// its integrate-run diagnostic stamped by the real provider argument builder
// for the requested network mode.
func partialCleanupReceipt(execution TaskExecution, target string, noFetch bool) TaskReceipt {
	return TaskReceipt{
		Task: execution.Task, Purpose: execution.Purpose, Operation: "finish", Status: TaskExecutionStatusBlocked,
		Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree,
		Reason: partialCleanupReason, CreatedAt: time.Now().UTC(),
		SourceHeadBefore: "1111111111111111111111111111111111111111", TaskHead: "2222222222222222222222222222222222222222", SourcePushed: true,
		Diagnostics: []CommandDiagnostics{{
			Command:  integrationProviderGZGit,
			Args:     integrateArgs(target, noFetch),
			Cwd:      execution.Worktree,
			ExitCode: 3,
		}},
	}
}

func TestPartialCleanupReceiptRecognition(t *testing.T) {
	execution := partialCleanupExecution()
	target := integrationTarget(execution)

	t.Run("recognizes the provider-stamped target for both network modes", func(t *testing.T) {
		for _, noFetch := range []bool{false, true} {
			if !isPartialCleanupReceipt(execution, partialCleanupReceipt(execution, target, noFetch)) {
				t.Fatalf("no-fetch=%t: the provider-stamped exit-3 receipt must be recognized", noFetch)
			}
		}
	})
	t.Run("still recognizes the legacy bare and no-fetch-only argv", func(t *testing.T) {
		for _, args := range [][]string{{"integrate", "run"}, {"integrate", "run", integrationNoFetchFlag}} {
			receipt := partialCleanupReceipt(execution, target, false)
			receipt.Diagnostics = []CommandDiagnostics{{Command: integrationProviderGZGit, Args: args, ExitCode: 3}}
			if !isPartialCleanupReceipt(execution, receipt) {
				t.Fatalf("legacy argv %v must stay recognized", args)
			}
		}
	})
	t.Run("refuses a run against any other target", func(t *testing.T) {
		if isPartialCleanupReceipt(execution, partialCleanupReceipt(execution, "origin/other", false)) {
			t.Fatal("a --target run against another ref must not be read as this execution's landing")
		}
	})
	t.Run("refuses a different exit code, command, or broken binding", func(t *testing.T) {
		wrongExit := partialCleanupReceipt(execution, target, false)
		wrongExit.Diagnostics[0].ExitCode = 2
		if isPartialCleanupReceipt(execution, wrongExit) {
			t.Fatal("an exit-2 integrate run is not a partial cleanup")
		}
		wrongCommand := partialCleanupReceipt(execution, target, false)
		wrongCommand.Diagnostics[0].Command = "other"
		if isPartialCleanupReceipt(execution, wrongCommand) {
			t.Fatal("a non-provider diagnostic is not a partial cleanup")
		}
		unpushed := partialCleanupReceipt(execution, target, false)
		unpushed.SourcePushed = false
		if isPartialCleanupReceipt(execution, unpushed) {
			t.Fatal("a receipt that does not prove the source push is not a partial cleanup")
		}
	})
	t.Run("refuses a receipt whose purpose contradicts the execution", func(t *testing.T) {
		receipt := partialCleanupReceipt(execution, target, true)
		receipt.Purpose = string(TaskExecutionPurposeExperiment)
		if isPartialCleanupReceipt(execution, receipt) {
			t.Fatal("a receipt claiming another purpose must not bind this execution")
		}
	})
	t.Run("refuses a non-delivery execution outright", func(t *testing.T) {
		experiment := partialCleanupExecution()
		experiment.Purpose = string(TaskExecutionPurposeExperiment)
		experiment.Branch = "exp/actor/host/chore/cleanup-fail"
		if isPartialCleanupReceipt(experiment, partialCleanupReceipt(experiment, integrationTarget(experiment), true)) {
			t.Fatal("experiment executions never integrate, so they never carry an exit-3 cleanup receipt")
		}
	})
}

func TestPartialCleanupStateOffersRecover(t *testing.T) {
	execution := partialCleanupExecution()
	svc := &Service{}
	state := svc.deriveState(t.Context(), execution, partialCleanupReceipt(execution, integrationTarget(execution), true), worktrunkInventory{}, false)
	if state.Status != TaskExecutionStatusBlocked {
		t.Fatalf("partial-cleanup status = %s, want BLOCKED", state.Status)
	}
	if strings.Join(state.AllowedActions, ",") != "run-status,run-recover,run-abort" {
		t.Fatalf("allowed actions = %v, want run-status, run-recover, and run-abort", state.AllowedActions)
	}
	if state.NextAction != "run gz-git run recover cleanup-fail" {
		t.Fatalf("next action = %q, want the run-recover answer", state.NextAction)
	}
}

// TestPartialCleanupLiveVerbGates proves the same recognition feeds the live
// verbs: recover gets past the receipt gate into the evidence check, and
// run-discard refuses to reclaim an execution whose integration landed.
// Run-abort stays the advertised terminal exit (it is not gated on this
// receipt in the reference either), which the derive-state test above pins.
func TestPartialCleanupLiveVerbGates(t *testing.T) {
	svc, fx := newWorktrunkContractRepo(t, newWorktrunkStubRunner())
	execution := claimedExecution(fx)
	seedExecutionAndReceipt(t, svc, execution, partialCleanupReceipt(execution, integrationTarget(execution), true))

	recovered, code := svc.Recover(t.Context(), worktrunkContractTask)
	if code != 1 || !strings.Contains(recovered.Reason, "partial cleanup recovery is not proven") {
		t.Fatalf("recover = %d/%q; want past the recognition gate into the evidence check", code, recovered.Reason)
	}
	discarded, code := svc.DiscardWithOptions(t.Context(), worktrunkContractTask, "reclaim", "", DiscardOptions{})
	if code != 1 || discarded.Reason != "run-discard refuses an execution with recorded integration cleanup failure" {
		t.Fatalf("discard = %d/%q, want the recorded cleanup-failure refusal", code, discarded.Reason)
	}
}

// purposeProbeRunner answers the inventory read and the Git state probes
// state derivation runs, delegating everything else to the real executor so
// the store and configuration paths stay real.
type purposeProbeRunner struct {
	executor *Executor
	branch   string
	envelope string
	calls    []CommandRequest
}

func (r *purposeProbeRunner) Run(ctx context.Context, req CommandRequest) (CommandResult, error) {
	r.calls = append(r.calls, req)
	probe := func(stdout string) (CommandResult, error) {
		return CommandResult{Command: req.Command, Args: append([]string(nil), req.Args...), WorkDir: req.WorkDir, ExitCode: 0, Stdout: stdout}, nil
	}
	if req.Command == "wt" {
		return probe(r.envelope)
	}
	if req.Command == "git" && len(req.Args) > 0 {
		switch req.Args[0] {
		case "branch":
			return probe(r.branch + "\n")
		case "status":
			return probe("")
		case "rev-parse":
			if len(req.Args) == 4 && req.Args[1] == "--abbrev-ref" {
				return probe("origin/" + r.branch + "\n")
			}
		case "rev-list":
			return probe("0\t0\n")
		}
	}
	return r.executor.Run(ctx, req)
}

func TestPurposeShapesDerivedState(t *testing.T) {
	for _, tc := range []struct {
		name        string
		purpose     TaskExecutionPurpose
		branch      string
		wantReady   bool
		wantActions string
		wantReason  string
	}{
		{
			name: "delivery reads the upstream and stays finish-ready", purpose: TaskExecutionPurposeDelivery,
			branch:    "dev/" + worktrunkContractActor + "/" + worktrunkContractHost + "/chore/" + worktrunkContractTask,
			wantReady: true, wantActions: "run-status,run-finish,run-abort,run-discard", wantReason: "task execution is ready to finish",
		},
		{
			name: "experiment skips the upstream and offers no finish", purpose: TaskExecutionPurposeExperiment,
			branch:    "exp/" + worktrunkContractActor + "/" + worktrunkContractHost + "/chore/" + worktrunkContractTask,
			wantReady: false, wantActions: "run-status,run-abort,run-discard", wantReason: "non-delivery execution is ready for abort or discard",
		},
		{
			name: "review skips the upstream and offers no finish", purpose: TaskExecutionPurposeReview,
			branch:    "review/" + worktrunkContractActor + "/" + worktrunkContractHost + "/chore/" + worktrunkContractTask,
			wantReady: false, wantActions: "run-status,run-abort,run-discard", wantReason: "non-delivery execution is ready for abort or discard",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &purposeProbeRunner{executor: NewExecutor(), branch: tc.branch}
			svc, fx := newWorktrunkContractRepo(t, runner)
			runner.envelope = stubListEnvelope(fx.root, tc.branch, expectedWorktreePath(fx.worktreeRoot, tc.branch))
			worktree := expectedWorktreePath(fx.worktreeRoot, tc.branch)
			if err := os.MkdirAll(worktree, 0o755); err != nil {
				t.Fatalf("stage the worktree directory: %v", err)
			}
			now := time.Now().UTC()
			execution := TaskExecution{
				Task: worktrunkContractTask, Type: TaskExecutionTypeChore, Purpose: string(tc.purpose), Source: "main",
				Owner: TaskOwner{Actor: worktrunkContractActor, Host: worktrunkContractHost}, Branch: tc.branch,
				Worktree: worktree, StartedAt: now, UpdatedAt: now,
			}
			seedExecutionAndReceipt(t, svc, execution)

			resp := svc.Status(t.Context(), worktrunkContractTask)
			if resp.Status != TaskExecutionStatusActive {
				t.Fatalf("status = %s (%s), want ACTIVE", resp.Status, resp.Reason)
			}
			if resp.FinishReady != tc.wantReady {
				t.Fatalf("finish-ready = %t, want %t", resp.FinishReady, tc.wantReady)
			}
			if strings.Join(resp.AllowedActions, ",") != tc.wantActions {
				t.Fatalf("allowed actions = %v, want %s", resp.AllowedActions, tc.wantActions)
			}
			if resp.Reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", resp.Reason, tc.wantReason)
			}
			if resp.Execution == nil || resp.Execution.Purpose != string(tc.purpose) {
				t.Fatalf("execution purpose = %+v, want %q in the response document", resp.Execution, tc.purpose)
			}
			upstreamProbes := 0
			for _, call := range runner.calls {
				if call.Command != "git" || len(call.Args) == 0 {
					continue
				}
				if call.Args[0] == "rev-list" || (call.Args[0] == "rev-parse" && len(call.Args) == 4 && call.Args[1] == "--abbrev-ref") {
					upstreamProbes++
					if tc.purpose != TaskExecutionPurposeDelivery {
						t.Fatalf("non-delivery status probed landing evidence: git %v", call.Args)
					}
				}
			}
			if want := map[bool]int{true: 2, false: 0}[tc.wantReady]; upstreamProbes != want {
				t.Fatalf("upstream probes = %d, want %d", upstreamProbes, want)
			}
			if tc.purpose != TaskExecutionPurposeDelivery {
				if _, code := svc.Finish(t.Context(), worktrunkContractTask); code != 1 {
					t.Fatalf("finish on a %s execution exited %d, want the purpose refusal", tc.purpose, code)
				}
			}
		})
	}
}

func TestStartExistingExecutionPurposeGates(t *testing.T) {
	owner := TaskOwner{Actor: "actor", Host: "host"}
	started := time.Now().UTC().Add(-time.Hour)
	record := func(purpose TaskExecutionPurpose, branch string) TaskExecution {
		return TaskExecution{Task: "purposed", Type: TaskExecutionTypeFeat, Purpose: string(purpose), Owner: owner, Branch: branch, Worktree: "/wt/purposed", StartedAt: started, UpdatedAt: started}
	}
	dev := "dev/actor/host/feat/purposed"

	t.Run("an invalid recorded purpose refuses the start", func(t *testing.T) {
		_, _, refused := startExistingExecution("purposed", TaskExecutionTypeFeat, TaskExecutionPurposeDelivery, true, owner,
			[]TaskExecution{record("yolo", dev)}, map[string]TaskReceipt{})
		if refused == nil || refused.Reason != `execution purpose "yolo" is invalid` {
			t.Fatalf("invalid-purpose record = %v, want the invalid purpose refusal", refused)
		}
	})
	t.Run("a branch outside the purpose namespace refuses the start", func(t *testing.T) {
		_, _, refused := startExistingExecution("purposed", TaskExecutionTypeFeat, TaskExecutionPurposeDelivery, true, owner,
			[]TaskExecution{record(TaskExecutionPurposeExperiment, dev)}, map[string]TaskReceipt{})
		if refused == nil || refused.Reason != "execution branch does not match its purpose" {
			t.Fatalf("purpose/branch mismatch = %v, want the branch refusal", refused)
		}
	})
	t.Run("a receipt claiming another purpose refuses the start", func(t *testing.T) {
		receipts := map[string]TaskReceipt{"purposed": {Task: "purposed", Operation: "start", Status: TaskExecutionStatusActive, Purpose: string(TaskExecutionPurposeExperiment), CreatedAt: started}}
		_, _, refused := startExistingExecution("purposed", TaskExecutionTypeFeat, TaskExecutionPurposeDelivery, true, owner,
			[]TaskExecution{record(TaskExecutionPurposeDelivery, dev)}, receipts)
		if refused == nil || refused.Reason != "receipt purpose does not match the execution" || refused.Receipt == nil {
			t.Fatalf("receipt purpose mismatch = %v, want the refusal with the receipt attached", refused)
		}
	})
	t.Run("a legacy record and a purposeless receipt still resume", func(t *testing.T) {
		got, resumed, refused := startExistingExecution("purposed", TaskExecutionTypeFeat, TaskExecutionPurposeDelivery, true, owner,
			[]TaskExecution{record("", dev)}, map[string]TaskReceipt{})
		if refused != nil || resumed == nil || got != -1 || resumed.Status != TaskExecutionStatusActive {
			t.Fatalf("legacy resume = %d/%v/%v, want the existing execution returned", got, resumed, refused)
		}
	})
}

// purposeGateVerb is one lifecycle verb expressed against the contract
// fixture. The reads answer through the derived state and carry no exit code
// at the service seam; the mutating verbs refuse with exit 1.
type purposeGateVerb struct {
	name       string
	wantCode   int
	wantReason string
	call       func(svc *Service) (TaskResponse, int)
}

// purposeGateVerbs tabulates the whole surface the purpose gates must hold
// on: status and list read the derived state, finish reaches it through its
// own status read, and abort and discard gate the record directly.
func purposeGateVerbs(ctx context.Context) []purposeGateVerb {
	return []purposeGateVerb{
		{name: "run-status", call: func(svc *Service) (TaskResponse, int) { return svc.Status(ctx, worktrunkContractTask), 0 }},
		{name: "run-list", call: func(svc *Service) (TaskResponse, int) { return svc.List(ctx), 0 }},
		{name: "run-finish", wantCode: 1, call: func(svc *Service) (TaskResponse, int) { return svc.Finish(ctx, worktrunkContractTask) }},
		{name: "run-abort", wantCode: 1, call: func(svc *Service) (TaskResponse, int) { return svc.Abort(ctx, worktrunkContractTask, "close it") }},
		{name: "run-discard", wantCode: 1, call: func(svc *Service) (TaskResponse, int) {
			return svc.DiscardWithOptions(ctx, worktrunkContractTask, "reclaim", "", DiscardOptions{})
		}},
	}
}

// purposeGateExecution records an execution with the asked purpose and
// branch, so a case can compose a mismatch deliberately.
func purposeGateExecution(fx wtContractRepo, purpose TaskExecutionPurpose, branch string) TaskExecution {
	now := time.Now().UTC()
	return TaskExecution{
		Task: worktrunkContractTask, Type: TaskExecutionTypeChore, Purpose: string(purpose), Source: "main",
		Owner: TaskOwner{Actor: worktrunkContractActor, Host: worktrunkContractHost}, Branch: branch,
		Worktree: expectedWorktreePath(fx.worktreeRoot, branch), StartedAt: now, UpdatedAt: now,
	}
}

// assertPurposeRefusal pins one verb's refusal: the mutating verbs exit 1,
// the answer is BLOCKED, and the operator-facing reason names the gate -- on
// run-list through the one derived state the aggregate folded in, whose
// allowed actions stay run-status and run-abort.
func assertPurposeRefusal(t *testing.T, verb purposeGateVerb, resp TaskResponse, code int) {
	t.Helper()
	if code != verb.wantCode {
		t.Fatalf("%s exit = %d, want %d", verb.name, code, verb.wantCode)
	}
	if resp.Status != TaskExecutionStatusBlocked {
		t.Fatalf("%s status = %s (%s), want BLOCKED", verb.name, resp.Status, resp.Reason)
	}
	reason, actions := resp.Reason, resp.AllowedActions
	if verb.name == "run-list" {
		if len(resp.States) != 1 {
			t.Fatalf("run-list states = %d, want the one seeded execution", len(resp.States))
		}
		if resp.Reason != "one or more task executions are blocked" {
			t.Fatalf("run-list aggregate reason = %q, want the blocked rollup", resp.Reason)
		}
		reason, actions = resp.States[0].Reason, resp.States[0].AllowedActions
	}
	if reason != verb.wantReason {
		t.Fatalf("%s reason = %q, want %q", verb.name, reason, verb.wantReason)
	}
	if verb.name == "run-status" || verb.name == "run-list" {
		if strings.Join(actions, ",") != "run-status,run-abort" {
			t.Fatalf("%s allowed actions = %v, want run-status and run-abort", verb.name, actions)
		}
	}
}

// TestEveryVerbRefusesPurposeMismatch seeds one corrupt record per case and
// proves every verb refuses it: a record whose branch leaves its purpose
// namespace, and a record whose purpose value names no declared purpose, are
// metadata gz-git will not act on.
func TestEveryVerbRefusesPurposeMismatch(t *testing.T) {
	for _, tc := range []struct {
		name       string
		purpose    TaskExecutionPurpose
		branch     func(fx wtContractRepo) string
		wantReason string
	}{
		{
			name: "a record whose branch leaves its purpose namespace", purpose: TaskExecutionPurposeExperiment,
			branch:     func(fx wtContractRepo) string { return fx.branch },
			wantReason: "execution branch does not match its purpose",
		},
		{
			name: "a record whose purpose value is invalid", purpose: "yolo",
			branch: func(fx wtContractRepo) string {
				return TaskExecutionPurposeExperiment.BranchPrefix() + "/" + worktrunkContractActor + "/" + worktrunkContractHost + "/chore/" + worktrunkContractTask
			},
			wantReason: `execution purpose "yolo" is invalid`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, verb := range purposeGateVerbs(t.Context()) {
				verb.wantReason = tc.wantReason
				t.Run(verb.name, func(t *testing.T) {
					svc, fx := newWorktrunkContractRepo(t, newWorktrunkStubRunner())
					seedExecutionAndReceipt(t, svc, purposeGateExecution(fx, tc.purpose, tc.branch(fx)))
					resp, code := verb.call(svc)
					assertPurposeRefusal(t, verb, resp, code)
				})
			}
		})
	}
}

// TestEveryVerbRefusesReceiptPurposeMismatch binds receipts to their
// execution: a receipt claiming another purpose than the record it sits
// under is evidence that does not belong, and no verb reads past it. The
// derived state answers with the aggregate wording; abort and discard keep
// their own gate's sentence.
func TestEveryVerbRefusesReceiptPurposeMismatch(t *testing.T) {
	wantReasons := map[string]string{
		"run-status":  "receipt purpose is invalid or does not match the execution",
		"run-list":    "receipt purpose is invalid or does not match the execution",
		"run-finish":  "receipt purpose is invalid or does not match the execution",
		"run-abort":   "receipt purpose does not match the execution",
		"run-discard": "receipt purpose does not match the execution",
	}
	for _, verb := range purposeGateVerbs(t.Context()) {
		verb.wantReason = wantReasons[verb.name]
		t.Run(verb.name, func(t *testing.T) {
			svc, fx := newWorktrunkContractRepo(t, newWorktrunkStubRunner())
			execution := claimedExecution(fx)
			receipt := TaskReceipt{
				Task: execution.Task, Purpose: string(TaskExecutionPurposeExperiment), Operation: "start",
				Status: TaskExecutionStatusActive, Owner: execution.Owner, Branch: execution.Branch,
				Worktree: execution.Worktree, CreatedAt: time.Now().UTC(),
			}
			seedExecutionAndReceipt(t, svc, execution, receipt)
			resp, code := verb.call(svc)
			assertPurposeRefusal(t, verb, resp, code)
		})
	}
}

// TestAbortNonDeliveryRequiresReason ports the CE rule that closing an
// experiment or review execution demands words: such a run never integrates,
// so the recorded reason is the only account of why it ended.
func TestAbortNonDeliveryRequiresReason(t *testing.T) {
	newExecution := func(fx wtContractRepo, purpose TaskExecutionPurpose) TaskExecution {
		return purposeGateExecution(fx, purpose, purpose.BranchPrefix()+"/"+worktrunkContractActor+"/"+worktrunkContractHost+"/chore/"+worktrunkContractTask)
	}
	t.Run("an experiment or review abort without a reason is refused", func(t *testing.T) {
		for _, purpose := range []TaskExecutionPurpose{TaskExecutionPurposeExperiment, TaskExecutionPurposeReview} {
			for _, reason := range []string{"", "   "} {
				svc, fx := newWorktrunkContractRepo(t, newWorktrunkStubRunner())
				seedExecutionAndReceipt(t, svc, newExecution(fx, purpose))
				resp, code := svc.Abort(t.Context(), worktrunkContractTask, reason)
				if code != 1 || resp.Status != TaskExecutionStatusBlocked ||
					resp.Reason != "experiment and review executions require a non-empty abort reason" {
					t.Fatalf("%s abort with reason %q = %d/%s (%s), want the non-empty-reason refusal", purpose, reason, code, resp.Status, resp.Reason)
				}
				if want := "run gz-git run abort " + worktrunkContractTask + " --reason R"; resp.NextAction != want {
					t.Fatalf("next action = %q, want %q", resp.NextAction, want)
				}
			}
		}
	})
	t.Run("an experiment closes once a reason is recorded", func(t *testing.T) {
		svc, fx := newWorktrunkContractRepo(t, newWorktrunkStubRunner())
		seedExecutionAndReceipt(t, svc, newExecution(fx, TaskExecutionPurposeExperiment))
		resp, code := svc.Abort(t.Context(), worktrunkContractTask, "question answered; branch kept for reference")
		if code != 0 || resp.Status != TaskExecutionStatusAborted {
			t.Fatalf("abort = %d/%s (%s), want the closed execution", code, resp.Status, resp.Reason)
		}
		if resp.Receipt == nil || !strings.Contains(resp.Receipt.Reason, "question answered; branch kept for reference") {
			t.Fatalf("receipt = %+v, want the caller's words recorded", resp.Receipt)
		}
	})
}

// TestDeliveryPurposePathsUnchanged pins the delivery surface the new gates
// must not disturb: a well-formed delivery execution with its start receipt
// still derives ACTIVE on status and one active state under a READY list,
// and run-abort still closes it without a reason.
func TestDeliveryPurposePathsUnchanged(t *testing.T) {
	branch := TaskExecutionPurposeDelivery.BranchPrefix() + "/" + worktrunkContractActor + "/" + worktrunkContractHost + "/chore/" + worktrunkContractTask
	runner := &purposeProbeRunner{executor: NewExecutor(), branch: branch}
	svc, fx := newWorktrunkContractRepo(t, runner)
	runner.envelope = stubListEnvelope(fx.root, branch, expectedWorktreePath(fx.worktreeRoot, branch))
	worktree := expectedWorktreePath(fx.worktreeRoot, branch)
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("stage the worktree directory: %v", err)
	}
	execution := claimedExecution(fx)
	start := TaskReceipt{
		Task: execution.Task, Purpose: execution.Purpose, Operation: "start", Status: TaskExecutionStatusActive,
		Owner: execution.Owner, Branch: execution.Branch, Worktree: execution.Worktree, CreatedAt: time.Now().UTC(),
	}
	seedExecutionAndReceipt(t, svc, execution, start)

	status := svc.Status(t.Context(), worktrunkContractTask)
	if status.Status != TaskExecutionStatusActive || status.Reason != "task execution is ready to finish" {
		t.Fatalf("status = %s (%s), want the unchanged ACTIVE delivery answer", status.Status, status.Reason)
	}
	list := svc.List(t.Context())
	if list.Status != TaskExecutionStatusReady || list.ActiveCount != 1 {
		t.Fatalf("list = %s active=%d, want READY with the one active execution", list.Status, list.ActiveCount)
	}
	resp, code := svc.Abort(t.Context(), worktrunkContractTask, "")
	if code != 0 || resp.Status != TaskExecutionStatusAborted || resp.Reason != "execution closed by run-abort; gz-git did not integrate it" {
		t.Fatalf("delivery abort = %d/%s (%s), want the unchanged no-reason close", code, resp.Status, resp.Reason)
	}
}
