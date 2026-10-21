// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func outcomeExecution(t *testing.T, fxDir, id string) *ExecutionContext {
	t.Helper()
	return &ExecutionContext{
		ExecutionID:  id,
		Generation:   "g1",
		Owner:        OwnerIdentity{Actor: "codex", Host: "testhost", Kind: "agent"},
		ExpectSource: gitOutput(t, fxDir, "rev-parse", "HEAD"),
	}
}

// TestTargetLockWaitPhaseIsReadOnly keeps documents written before the target
// lock was removed readable while no run can record the retired phase.
func TestTargetLockWaitPhaseIsReadOnly(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	execution := outcomeExecution(t, fx.Worktree, "retired-lock-wait")
	if err := validatePreMutationPhase(preMutationPhaseTargetLockWait, "target_lock_wait"); err != nil {
		t.Fatalf("legacy target_lock_wait document must stay valid: %v", err)
	}
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	capture := &preMutationCapture{execution: *execution}
	if err := capture.capture(context.Background(), g, targetPreflight{branch: "dev/actor/feat/task", targetRef: "refs/heads/develop"}); err != nil {
		t.Fatal(err)
	}
	outcome, err := recordPreMutationOutcome(context.Background(), capture, preMutationPhaseTargetLockWait)
	if err == nil || outcome != nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("record target_lock_wait = %#v, %v; want read-only refusal", outcome, err)
	}
	common := gitOutput(t, fx.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if stored, exists, readErr := readPreMutationOutcome(common, execution.ExecutionID); readErr != nil || exists || stored != nil {
		t.Fatalf("refused phase wrote outcome=%#v exists=%t err=%v", stored, exists, readErr)
	}
}

// TestRunRecordsBoundOutcomeWhenReadinessRejects rejects through a git check
// (uncommitted changes), the only way readiness can reject now that gz-git
// runs no repository code.
func TestRunRecordsBoundOutcomeWhenReadinessRejects(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	execution := outcomeExecution(t, fx.Worktree, "pre-readiness-reject")
	writeRepoFile(t, fx.Worktree, "uncommitted.txt", "dirty\n")
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{
		RepoPath: fx.Worktree, Branch: "dev/actor/feat/task",
	}, Execution: execution})
	if err == nil || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("Run() error = %v, want not ready", err)
	}
	if report == nil || report.PreMutationOutcome == nil || report.PreMutationOutcome.Phase != preMutationPhaseReadinessRejected || report.PreMutationOutcome.ReasonClass != "not_ready" {
		t.Fatalf("Run() report = %#v", report)
	}
}

func TestMalformedContextAndExistingReceiptCannotProduceSafeOutcome(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	_, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{RepoPath: fx.Worktree}, Execution: &ExecutionContext{ExecutionID: "partial"}})
	if !errors.Is(err, ErrInvalidExecutionContext) {
		t.Fatalf("malformed context error = %v", err)
	}

	execution := outcomeExecution(t, fx.Worktree, "receipt-wins")
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	common := gitOutput(t, fx.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	capture := &preMutationCapture{execution: *execution}
	if err := capture.capture(context.Background(), g, targetPreflight{branch: "dev/actor/feat/task", targetRef: "refs/heads/develop"}); err != nil {
		t.Fatal(err)
	}
	if capture.candidate == nil {
		t.Fatal("capture did not bind candidate")
	}
	receipt := &Receipt{SchemaVersion: receiptSchemaVersion, ExecutionID: execution.ExecutionID, Generation: execution.Generation, Owner: execution.Owner, RepositoryCommonDir: common, TaskBranch: capture.candidate.TaskBranch, TaskSHA: capture.candidate.TaskSHA, TargetRef: capture.candidate.TargetRef, Phase: evidencePhaseIntent}
	if err := createReceipt(context.Background(), common, receipt); err != nil {
		t.Fatal(err)
	}
	if outcome, recordErr := recordPreMutationOutcome(context.Background(), capture, preMutationPhaseReadinessRejected); recordErr == nil || outcome != nil {
		t.Fatalf("existing receipt outcome=%#v err=%v, want refused", outcome, recordErr)
	}
}

func TestSameExecutionCannotRecordNegativeWhileAnotherRunOwnsIt(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	execution := outcomeExecution(t, fx.Worktree, "same-execution-owner")
	common := gitOutput(t, fx.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	owner, err := acquireExecutionRunLock(common, execution.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.release()
	started := time.Now()
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{
		RepoPath: fx.Worktree, Branch: "dev/actor/feat/task",
	}, Execution: execution})
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("Run waited %s for the held execution; it must refuse at once", elapsed)
	}
	if !errors.Is(err, ErrResourceBusy) || !strings.Contains(err.Error(), "integration run "+execution.ExecutionID) {
		t.Fatalf("Run() error = %v, want ErrResourceBusy naming the execution", err)
	}
	if report != nil {
		t.Fatalf("Run() report = %#v, want no negative outcome without execution ownership", report)
	}
	if outcome, exists, readErr := readPreMutationOutcome(common, execution.ExecutionID); readErr != nil || exists || outcome != nil {
		t.Fatalf("refused peer wrote outcome=%#v exists=%t err=%v", outcome, exists, readErr)
	}
}

func TestExistingPushedReceiptSuppressesStaleNegativeOutcome(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	execution := outcomeExecution(t, fx.Worktree, "pushed-receipt-wins")
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	common := gitOutput(t, fx.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	capture := &preMutationCapture{execution: *execution}
	if err := capture.capture(context.Background(), g, targetPreflight{branch: "dev/actor/feat/task", targetRef: "refs/heads/develop"}); err != nil {
		t.Fatal(err)
	}
	if outcome, err := recordPreMutationOutcome(context.Background(), capture, preMutationPhaseReadinessRejected); err != nil || outcome == nil {
		t.Fatalf("record stale outcome = %#v, %v", outcome, err)
	}
	receipt := &Receipt{
		SchemaVersion:       receiptSchemaVersion,
		ExecutionID:         execution.ExecutionID,
		Generation:          execution.Generation,
		Owner:               execution.Owner,
		RepositoryCommonDir: common,
		TaskBranch:          "dev/actor/feat/task",
		TaskWorktree:        fx.Worktree,
		TaskSHA:             execution.ExpectSource,
		TargetRef:           "refs/heads/develop",
		TargetBeforeSHA:     gitOutput(t, fx.Clone, "rev-parse", "develop"),
		Remote:              "origin",
		Phase:               evidencePhasePushed,
		SourcePushed:        true,
	}
	if err := createReceipt(context.Background(), common, receipt); err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{
		RepoPath: fx.Worktree, Branch: "dev/actor/feat/task",
	}, Execution: execution})
	if !errors.Is(err, ErrEvidenceReplay) {
		t.Fatalf("Run() error = %v, want existing pushed receipt refusal", err)
	}
	if report == nil || report.PreMutationOutcome != nil {
		t.Fatalf("Run() report = %#v, stale negative outcome must not attach", report)
	}
	if proto := BuildRunProtocol(report, err, 2); proto.PreMutationOutcome != nil || proto.SourcePushed || proto.ReceiptPhase != "" {
		t.Fatalf("protocol = %+v, current refusal must not reuse stale negative or receipt state", proto)
	}
}
