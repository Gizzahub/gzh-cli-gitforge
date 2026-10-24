// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// The finish claim: the FINISHING receipt that stands in for the mutation
// lock while the integration provider runs, and the outcomes that supersede
// it when the provider answers.

package runtask

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
)

// finishClaimReason names why the mutation lock is released while the
// integration provider runs.
const finishClaimReason = "integration provider is running; the mutation lock is released until it exits"

// appendFinishClaim records the FINISHING receipt that supersedes the
// mutation lock for the provider run: every other verb reads it and stands
// down, and a recover names it when the process died without an outcome.
func (s *Service) appendFinishClaim(store stateStore, execution TaskExecution, preSourceHead, baseHead, preTaskHead string) (TaskReceipt, error) {
	claim := TaskReceipt{
		Task: execution.Task, Purpose: string(executionPurpose(execution)), Operation: "finish", Status: TaskExecutionStatusFinishing, Owner: execution.Owner,
		Branch: execution.Branch, Worktree: execution.Worktree, Reason: finishClaimReason, CreatedAt: s.now().UTC(),
		SourceHeadBefore: preSourceHead, BaseHead: baseHead, TaskHead: preTaskHead,
		ToolVersion: toolVersion(), ToolRevision: toolRevision(), ProcessID: os.Getpid(), ProcessHost: lockHost(),
	}
	err := store.appendReceipt(claim)
	return claim, err
}

// lockHost names this primary the way the mutation lock metadata does.
func lockHost() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(host), "_")
}

// finishClaimChange names what moved while the lock was released, or returns
// "" when the execution record and the claim are as finish left them. No verb
// moves a live claim, so a change here means an older binary, a hand edit, or
// a recover that judged this process dead.
func finishClaimChange(store stateStore, execution TaskExecution, claim TaskReceipt) string {
	executions, err := store.load()
	if err != nil {
		return "read task executions: " + err.Error()
	}
	unchanged := false
	for i := range executions {
		if reflect.DeepEqual(executions[i], execution) {
			unchanged = true
			break
		}
	}
	if !unchanged {
		return "the execution record changed"
	}
	receipts, err := store.latestReceipts()
	if err != nil {
		return "read task receipts: " + err.Error()
	}
	latest := receipts[execution.Task]
	if !reflect.DeepEqual(latest, claim) {
		return fmt.Sprintf("the latest receipt became %s %s at %s", latest.Operation, latest.Status, latest.CreatedAt.UTC().Format(time.RFC3339))
	}
	return ""
}

// unrecordedFinishOutcome answers a provider run whose outcome cannot be
// recorded because the runtime state it left was moved or the lock did not
// come back. The claim stands so recovery names it; nothing is appended.
func unrecordedFinishOutcome(execution TaskExecution, task string, diagnostics []CommandDiagnostics, result CommandResult, runErr error, reason string) (resp TaskResponse, code int) {
	r := response(TaskExecutionStatusBlocked, task, reason, "inspect provider diagnostics, then run-status", "run-status")
	r.Execution = &execution
	r.Diagnostics = append(append([]CommandDiagnostics{}, diagnostics...), diagnostic(result))
	if runErr == nil && (result.ExitCode == 0 || result.ExitCode == 3) {
		return r, 3
	}
	return r, 1
}

// recordFinish turns the provider result into the receipt that supersedes the
// claim. Every path appends one, so no outcome leaves the claim standing.
func (s *Service) recordFinish(ctx context.Context, store stateStore, execution TaskExecution, task string, noFetch bool, diagnostics []CommandDiagnostics, result CommandResult, runErr error, preSourceHead, baseHead, preTaskHead string) (resp TaskResponse, code int) {
	diagnostics = append(diagnostics, diagnostic(result))
	if runErr != nil {
		reason := "integration provider invocation failed: " + runErr.Error()
		receipt, evidenceErr := s.receipt(ctx, execution, TaskExecutionStatusBlocked, reason, diagnostics, preSourceHead, baseHead, preTaskHead, noFetch)
		if evidenceErr != nil {
			reason += "; post-operation evidence failed: " + evidenceErr.Error()
		}
		if err := store.appendReceipt(receipt); err != nil {
			reason += "; persist receipt failed: " + err.Error()
		}
		r := response(TaskExecutionStatusBlocked, task, reason, "inspect integration provider diagnostics and retry")
		r.Receipt, r.Diagnostics = &receipt, receipt.Diagnostics
		return r, 1
	}
	if result.ExitCode != 0 {
		reason, exitCode := "integration did not complete", result.ExitCode
		next := "resolve gz-git diagnostics and retry"
		if result.ExitCode == 3 {
			reason = partialCleanupReason
			if sibling := siblingReleaseNext(result); sibling != "" {
				next = sibling
			}
		} else if exitCode < 1 || exitCode > 2 {
			exitCode = 2
		}
		// A stale base is a fact git can answer without reading gz-git's
		// prose. Name it, and do not bury it under the recovery sentence
		// that only describes the worktree still being there. Exit 3 already
		// pushed, so the source tip it left is the task head, not a staleness.
		stale := result.ExitCode != 3 && s.branchBaseIsStale(ctx, execution)
		failing := ""
		if !stale && reason == "integration did not complete" {
			failing = failingCheckReason(result)
		}
		receipt, evidenceErr := s.receipt(ctx, execution, TaskExecutionStatusBlocked, reason, diagnostics, preSourceHead, baseHead, preTaskHead, noFetch)
		switch {
		case stale:
			// Name the freshly observed tip, not the pre-operation snapshot.
			tip := receipt.SourceHead
			if tip == "" {
				tip = preSourceHead
			}
			reason = fmt.Sprintf("task branch is based on %s, but the current source tip is %s; the branch is based on a commit that is no longer the source tip", baseHead, tip)
			receipt.Reason = reason
			receipt.ReasonCode = reasonCodeStaleBase
			next = "rebase onto the source tip in the worktree, then retry run-finish"
			// A stale base is the same refusal as before the run, so it exits
			// the same.
			exitCode = 1
		case failing != "":
			// The failing check explains the block; the recovery-incomplete
			// evidence error is only its consequence.
			reason = failing
			if evidenceErr != nil && !errors.Is(evidenceErr, errRecoveryIncomplete) {
				reason += "; post-operation evidence failed: " + evidenceErr.Error()
			}
			receipt.Reason = reason
		case evidenceErr != nil:
			reason += "; post-operation evidence failed: " + evidenceErr.Error()
		}
		if err := store.appendReceipt(receipt); err != nil {
			receipt.ReasonCode = ""
			reason += "; persist receipt failed: " + err.Error()
		}
		r := response(TaskExecutionStatusBlocked, task, reason, next)
		if receipt.ReasonCode == reasonCodeStaleBase {
			r.ReasonCode = reasonCodeStaleBase
		}
		r.Receipt, r.Diagnostics = &receipt, diagnostics
		return r, exitCode
	}
	return s.finishDone(ctx, store, execution, task, noFetch, diagnostics, preSourceHead, baseHead, preTaskHead)
}

// reasonCodeStaleBase classifies a finish refusal whose task branch is based
// on a commit that is no longer the source tip.
const reasonCodeStaleBase = "stale-base"

// refuseStaleFinish records a BLOCKED finish for a task branch whose base is
// no longer the source tip, naming both commits and the rebase that resolves
// it. The evidence probe is best-effort: the refusal is fully determined by
// the pre-operation head facts and its text is parity-pinned, so a failed
// probe must not change the response.
func (s *Service) refuseStaleFinish(ctx context.Context, store stateStore, execution TaskExecution, diagnostics []CommandDiagnostics, preSourceHead, baseHead, preTaskHead string, noFetch bool) (resp TaskResponse, code int) {
	receipt, _ := s.receipt(ctx, execution, TaskExecutionStatusBlocked, "", diagnostics, preSourceHead, baseHead, preTaskHead, noFetch) //nolint:errcheck // best-effort evidence; the refusal text is parity-pinned
	tip := receipt.SourceHead
	if tip == "" {
		tip = preSourceHead
	}
	reason := fmt.Sprintf("task branch is based on %s, but the current source tip is %s; the branch is based on a commit that is no longer the source tip", baseHead, tip)
	receipt.Reason = reason
	receipt.ReasonCode = reasonCodeStaleBase
	if err := store.appendReceipt(receipt); err != nil {
		receipt.ReasonCode = ""
		reason += "; persist receipt failed: " + err.Error()
	}
	r := response(TaskExecutionStatusBlocked, execution.Task, reason, "rebase onto the source tip in the worktree, then retry run-finish")
	if receipt.ReasonCode == reasonCodeStaleBase {
		r.ReasonCode = reasonCodeStaleBase
	}
	r.Receipt, r.Diagnostics = &receipt, diagnostics
	return r, 1
}
