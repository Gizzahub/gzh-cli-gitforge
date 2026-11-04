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

func TestSameExecutionRefusesWhileAnotherRunOwnsIt(t *testing.T) {
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
		t.Fatalf("Run() report = %#v, want no report without execution ownership", report)
	}
}
