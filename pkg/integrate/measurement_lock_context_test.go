// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestRunRecordsBoundOutcomeWhenHostLockWaitExpires(t *testing.T) {
	fastHostLockPolling(t)
	marker := filepath.Join(t.TempDir(), "gate-ran")
	fx := measurementLockFixture(t, "touch "+marker, "1m")
	execution := outcomeExecution(t, fx.Worktree, "pre-host-wait")
	holdHostLockInProcess(t, "/holder/repo", "dev/holder")
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", LockWait: 50 * time.Millisecond, LockNotice: io.Discard},
		Execution:    execution,
	})
	if !errors.Is(err, ErrHostLockWait) || report == nil || report.Check == nil || report.Check.Ready || report.Integrated || report.Receipt != nil {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if report.PreMutationOutcome == nil || report.PreMutationOutcome.Phase != preMutationPhaseReadinessRejected {
		t.Fatalf("missing bound refusal: %+v", report.PreMutationOutcome)
	}
	p := BuildRunProtocol(report, err, 2)
	if p.ExitCode != 2 || p.SourcePushed || p.ReceiptPhase != "" || p.ExecutionID != execution.ExecutionID || p.PreMutationOutcome == nil || p.ResourceWaitDurationMS <= 0 || p.ValidationDurationMS != 0 {
		t.Fatalf("protocol=%+v", p)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("gate ran: %v", err)
	}
}
