// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Judging whether the run-finish that wrote a FINISHING claim still runs, and
// every answer the runtime gives while a claim stands.
//
// The provider run can wait tens of minutes for the host integration slot
// without touching this repository. Finish therefore records a FINISHING claim
// under the lock, releases the lock for the run, and takes it back to record
// the outcome. The claim, not the lock, is what keeps every other verb off the
// claimed execution during the run.

package runtask

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const interruptedFinishReason = "run-finish ended without recording an integration outcome"

// finishRelockWait bounds how long a finish waits to take the lock back after
// the run. Variables so tests need not wait the production bound.
var (
	finishRelockWait = 10 * time.Minute
	finishRelockPoll = 200 * time.Millisecond
)

// finishInterruptContext is canceled by SIGINT or SIGTERM and, until stop,
// keeps them from ending gz-git. A variable so tests need not send signals.
var finishInterruptContext = func(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
}

// claimLiveness is what gz-git can prove about a claimant: dead and alive need
// a pid on this host; anything else is unknown and only the owner can declare
// it.
type claimLiveness int

const (
	claimUnknown claimLiveness = iota
	claimAlive
	claimDead
)

// claimStartSlack absorbs ps's whole-second elapsed time: the claimant starts
// before it writes the claim, but rounding can place its start up to a second
// after.
const claimStartSlack = 5 * time.Second

// isFinishClaim reports whether a receipt is a FINISHING claim. Any claim for
// the task counts, bound or not: refusing a verb on a claim that does not bind
// costs a retry, while letting one through could race the provider.
func isFinishClaim(receipt TaskReceipt) bool {
	return receipt.Status == TaskExecutionStatusFinishing
}

// finishClaimLive reports whether the process that wrote a claim may still be
// running: only proof of death releases a claim without the owner.
func finishClaimLive(claim TaskReceipt) bool {
	return finishClaimLiveness(claim) != claimDead
}

// finishClaimLiveness judges a claim by the same proof that reclaims a dead
// mutation lock, a dead pid on this host, plus one more: a running pid whose
// process started after the claim was written is a reused pid, not the
// claimant. A running pid whose start time ps cannot report is unknown: it
// keeps the claim, and only the owner's declaration releases it.
func finishClaimLiveness(claim TaskReceipt) claimLiveness {
	if claim.ProcessID <= 0 || claim.ProcessHost == "" || claim.ProcessHost != lockHost() {
		return claimUnknown
	}
	if !pidRunning(claim.ProcessID) {
		return claimDead
	}
	started, err := pidStartedAt(claim.ProcessID)
	if err != nil {
		return claimUnknown
	}
	if started.After(claim.CreatedAt.Add(claimStartSlack)) {
		return claimDead
	}
	return claimAlive
}

// pidStartedAt reports when a running pid's process started. A variable so
// tests can stage a reused pid.
var pidStartedAt = func(pid int) (time.Time, error) {
	cmd := exec.Command("ps", "-o", "etime=", "-p", strconv.Itoa(pid)) // #nosec G204 -- fixed argv; the only variable is an integer pid
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("ps -o etime= -p %d: %w", pid, err)
	}
	elapsed, err := parseElapsed(strings.TrimSpace(string(out)))
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(-elapsed), nil
}

// parseElapsed reads ps's etime field, [[dd-]hh:]mm:ss, which macOS and
// procps both print.
func parseElapsed(field string) (time.Duration, error) {
	days, clock := 0, field
	if d, rest, ok := strings.Cut(field, "-"); ok {
		n, err := strconv.Atoi(d)
		if err != nil {
			return 0, fmt.Errorf("unrecognized ps elapsed time %q", field)
		}
		days, clock = n, rest
	}
	parts := strings.Split(clock, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("unrecognized ps elapsed time %q", field)
	}
	seconds := 0
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return 0, fmt.Errorf("unrecognized ps elapsed time %q", field)
		}
		seconds = seconds*60 + n
	}
	return time.Duration(days)*24*time.Hour + time.Duration(seconds)*time.Second, nil
}

// waitForClaim is the next action for a live claim. --claim-dead is offered
// only where gz-git cannot judge the pid; a claimant gz-git sees running is
// waited for.
func waitForClaim(task, wait string, claim TaskReceipt) string {
	if finishClaimLiveness(claim) == claimAlive {
		return fmt.Sprintf("%s; run-finish pid=%d is running on this host", wait, claim.ProcessID)
	}
	return wait + "; if that process is gone, the recorded owner runs gz-git run recover " + task + " --claim-dead"
}

func finishClaimOwner(claim TaskReceipt) string {
	return fmt.Sprintf("run-finish pid=%d host=%s (claimed %s)", claim.ProcessID, claim.ProcessHost, claim.CreatedAt.UTC().Format(time.RFC3339))
}

// claimedState is the derived state of an execution whose latest receipt is a
// claim: ACTIVE and not finish-ready while the claimant lives, BLOCKED with
// run-recover once it has died.
func claimedState(state DerivedTaskState, claim TaskReceipt) DerivedTaskState {
	if finishClaimLive(claim) {
		state.AllowedActions = []string{"run-status"}
		state.Reason = finishClaimOwner(claim) + " is integrating this execution"
		state.NextAction = waitForClaim(state.Execution.Task, "wait for run-finish to record its outcome", claim)
		return state
	}
	state.Status = TaskExecutionStatusBlocked
	state.AllowedActions = []string{"run-status", "run-recover"}
	state.Reason = finishClaimOwner(claim) + " ended without recording an integration outcome"
	state.NextAction = "run gz-git run recover " + state.Execution.Task
	return state
}

// refuseClaimed is the answer every other per-task mutation gives a claimed
// execution, naming the claimant and the one verb that can move it.
func refuseClaimed(task, verb string, claim TaskReceipt) (resp TaskResponse, code int) {
	if finishClaimLive(claim) {
		return response(TaskExecutionStatusBlocked, task, verb+" refuses an execution that "+finishClaimOwner(claim)+" is integrating",
			waitForClaim(task, "wait for run-finish to record its outcome, then retry", claim), "run-status"), 1
	}
	return response(TaskExecutionStatusBlocked, task, verb+" refuses an execution whose "+finishClaimOwner(claim)+" ended without recording an outcome",
		"run gz-git run recover "+task, "run-status", "run-recover"), 1
}

// recoverInterruptedFinish releases the claim of a run-finish that died. It
// records what Git shows now and never calls the provider: the provider may
// have pushed, removed the worktree, or done nothing, and only the operator
// can choose the next verb from that evidence. A non-nil declaredBy is the
// owner's --claim-dead declaration, which stands in for the pid proof gz-git
// cannot make and is recorded as performedBy; it is refused for a claimant
// gz-git sees running on this host, and a finish that was in fact alive
// elsewhere names the changed receipt when it takes the lock back.
func (s *Service) recoverInterruptedFinish(ctx context.Context, store stateStore, execution TaskExecution, claim TaskReceipt, cfg TaskRuntimeConfig, declaredBy *TaskOwner) (resp TaskResponse, code int) {
	task := execution.Task
	reason := interruptedFinishReason + ": " + finishClaimOwner(claim)
	if declaredBy != nil && finishClaimLiveness(claim) == claimAlive {
		return response(TaskExecutionStatusBlocked, task, fmt.Sprintf("run-recover --claim-dead refuses a claim whose run-finish pid=%d is running on this host", claim.ProcessID),
			"wait for run-finish to record its outcome", "run-status"), 1
	}
	if declaredBy != nil {
		reason += "; " + declaredBy.String() + " declared the claim dead with --claim-dead"
	} else if finishClaimLive(claim) {
		return refuseClaimed(task, "run-recover", claim)
	}
	noFetch := cfg.IntegrationNetworkPolicy == IntegrationNetworkPolicyNoFetch
	receipt, evidenceErr := s.receipt(ctx, execution, TaskExecutionStatusBlocked, reason, nil, claim.SourceHeadBefore, claim.BaseHead, claim.TaskHead, noFetch)
	receipt.Operation = "recover"
	receipt.PerformedBy = declaredBy
	if evidenceErr != nil && !errors.Is(evidenceErr, errRecoveryIncomplete) {
		receipt.Reason += "; post-operation evidence failed: " + evidenceErr.Error()
	}
	if err := store.appendReceipt(receipt); err != nil {
		return response(TaskExecutionStatusBlocked, task, "persist recovery receipt failed: "+err.Error(), "repair task runtime storage and retry"), 1
	}
	status, next := TaskExecutionStatusActive, "run gz-git run finish "+task+" to retry the integration"
	if receipt.WorktreeRemoved {
		status, next = TaskExecutionStatusBlocked, "inspect the source branch; end it with gz-git run abort "+task+" --reason R"
	}
	r := response(status, task, receipt.Reason, next, "run-status")
	r.Execution, r.Receipt, r.Diagnostics = &execution, &receipt, receipt.Diagnostics
	return r, 0
}

// standingClaimWarnings names every FINISHING claim in task order, for
// run-doctor: a claim nobody can prove dead otherwise only shows on the
// claimed task's own status.
func standingClaimWarnings(receipts map[string]TaskReceipt) []string {
	var tasks []string
	for task, receipt := range receipts {
		if isFinishClaim(receipt) {
			tasks = append(tasks, task)
		}
	}
	sort.Strings(tasks)
	warnings := make([]string, 0, len(tasks))
	for _, task := range tasks {
		claim := receipts[task]
		if finishClaimLive(claim) {
			warnings = append(warnings, fmt.Sprintf("task %s is claimed by %s; %s", task, finishClaimOwner(claim), waitForClaim(task, "wait for it to record its outcome", claim)))
			continue
		}
		warnings = append(warnings, fmt.Sprintf("task %s is claimed by %s, which ended without recording an outcome; run gz-git run recover %s", task, finishClaimOwner(claim), task))
	}
	return warnings
}
