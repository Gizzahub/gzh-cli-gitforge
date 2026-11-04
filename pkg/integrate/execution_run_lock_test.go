// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

// TestExecutionRunLockSharesOneFileAcrossPathSpellings pins the receipt
// canonicalization on the run lock: a symlinked spelling of the common
// directory derives the same lock file as the canonical spelling, so a
// second invocation through a different path refuses with ErrResourceBusy.
// The exclusion is flock-based and would hold across spellings regardless
// (same inode at open); this test pins the derivation consistency
// canonicalCommonDir adds, not the exclusion mechanism itself.
func TestExecutionRunLockSharesOneFileAcrossPathSpellings(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	execution := outcomeExecution(t, fx.Worktree, "run-lock-alias")
	canonical := gitOutput(t, fx.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	alias := filepath.Join(t.TempDir(), "common-through-link")
	if err := os.Symlink(canonical, alias); err != nil {
		t.Fatal(err)
	}
	owner, err := acquireExecutionRunLock(canonical, execution.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.release()
	_, err = acquireExecutionRunLock(alias, execution.ExecutionID)
	if !errors.Is(err, ErrResourceBusy) || !strings.Contains(err.Error(), "integration run "+execution.ExecutionID) {
		t.Fatalf("symlinked spelling error = %v, want ErrResourceBusy naming the execution", err)
	}
}
