// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"io"
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

func TestRunRecordsBoundOutcomeWhenTargetLockWaitExpires(t *testing.T) {
	fastHostLockPolling(t)
	fx := readyTaskFixture(t)
	execution := outcomeExecution(t, fx.Worktree, "pre-lock-wait")
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	lock, err := acquireTargetLock(context.Background(), g, "refs/heads/develop", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	report, err := Run(ctx, gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{
		RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard,
	}, Execution: execution})
	if !errors.Is(err, errTargetLockWait) {
		t.Fatalf("Run() error = %v, want target-lock wait", err)
	}
	if report == nil || report.PreMutationOutcome == nil {
		t.Fatalf("Run() report = %#v, want durable pre-mutation outcome", report)
	}
	outcome := report.PreMutationOutcome
	if outcome.Phase != preMutationPhaseTargetLockWait || outcome.ReasonClass != "target_lock_wait" || !outcome.PreMutation || outcome.TaskSHA != execution.ExpectSource || outcome.ExpectedSource != execution.ExpectSource || outcome.TargetRef != "refs/heads/develop" {
		t.Fatalf("outcome = %+v", outcome)
	}
	common := gitOutput(t, fx.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	stored, ok, readErr := readPreMutationOutcome(common, execution.ExecutionID)
	if readErr != nil || !ok || !samePreMutationIdentity(stored, outcome) {
		t.Fatalf("stored outcome = %#v, ok=%t, err=%v", stored, ok, readErr)
	}
	if receipt, exists, readErr := readReceipt(common, execution.ExecutionID); readErr != nil || exists || receipt != nil {
		t.Fatalf("lock refusal wrote receipt=%#v exists=%t err=%v", receipt, exists, readErr)
	}
	proto := BuildRunProtocol(report, err, 2)
	if proto.PreMutationOutcome == nil || proto.ExecutionID != execution.ExecutionID || proto.TaskSHA != execution.ExpectSource || proto.TargetRef != "refs/heads/develop" || proto.SourcePushed || proto.ReceiptPhase != "" {
		t.Fatalf("protocol = %+v", proto)
	}
}

func TestRunRecordsBoundOutcomeWhenReadinessRejects(t *testing.T) {
	fx := readyTaskFixture(t)
	writeRepoFile(t, fx.Worktree, "Makefile", "check:\n\t@false\n")
	runGit(t, fx.Worktree, "add", "Makefile")
	runGit(t, fx.Worktree, "commit", "-m", "reject readiness")
	runGit(t, fx.Worktree, "push", fx.Remote, "HEAD")
	execution := outcomeExecution(t, fx.Worktree, "pre-readiness-reject")
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{
		RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard,
	}, Execution: execution})
	if err == nil || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("Run() error = %v, want not ready", err)
	}
	if report == nil || report.PreMutationOutcome == nil || report.PreMutationOutcome.Phase != preMutationPhaseReadinessRejected || report.PreMutationOutcome.ReasonClass != "not_ready" {
		t.Fatalf("Run() report = %#v", report)
	}
}

func TestMalformedContextAndExistingReceiptCannotProduceSafeOutcome(t *testing.T) {
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
	receipt := &Receipt{SchemaVersion: evidenceSchemaVersion, ExecutionID: execution.ExecutionID, Generation: execution.Generation, Owner: execution.Owner, RepositoryCommonDir: common, TaskBranch: capture.candidate.TaskBranch, TaskSHA: capture.candidate.TaskSHA, TargetRef: capture.candidate.TargetRef, Phase: evidencePhaseIntent}
	if err := createReceipt(context.Background(), common, receipt); err != nil {
		t.Fatal(err)
	}
	if outcome, recordErr := recordPreMutationOutcome(context.Background(), capture, preMutationPhaseReadinessRejected); recordErr == nil || outcome != nil {
		t.Fatalf("existing receipt outcome=%#v err=%v, want refused", outcome, recordErr)
	}
}

func TestSameExecutionCannotRecordNegativeWhileAnotherRunOwnsIt(t *testing.T) {
	fx := readyTaskFixture(t)
	execution := outcomeExecution(t, fx.Worktree, "same-execution-owner")
	common := gitOutput(t, fx.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	owner, err := acquireExecutionRunLock(context.Background(), common, execution.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.release()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	report, err := Run(ctx, gitcmd.NewExecutor(), RunOptions{CheckOptions: CheckOptions{
		RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard,
	}, Execution: execution})
	if err == nil || !strings.Contains(err.Error(), "wait for integration run") {
		t.Fatalf("Run() error = %v, want execution ownership wait", err)
	}
	if report != nil {
		t.Fatalf("Run() report = %#v, want no negative outcome without execution ownership", report)
	}
	if outcome, exists, readErr := readPreMutationOutcome(common, execution.ExecutionID); readErr != nil || exists || outcome != nil {
		t.Fatalf("blocked peer wrote outcome=%#v exists=%t err=%v", outcome, exists, readErr)
	}
}

func TestExistingPushedReceiptSuppressesStaleNegativeOutcome(t *testing.T) {
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
		SchemaVersion:       evidenceSchemaVersion,
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
		RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockNotice: io.Discard,
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
