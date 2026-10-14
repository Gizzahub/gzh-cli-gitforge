// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupOnlyRetryLocalExpectedOld(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	f.crashState(t, crashOpts{phase: evidencePhasePushed})
	moved := gitOutput(t, f.fx.Clone, "rev-parse", f.taskSHA+"^")
	t.Setenv("TASK302_TEST_REF", "refs/heads/"+f.taskBranch)
	t.Setenv("TASK302_TEST_MOVED", moved)
	f.interceptGit(t, `if [[ "$1" == "update-ref" && "${2:-}" == "-d" ]] || [[ "$1" == "branch" && "${2:-}" == "-d" ]]; then
  "$TASK302_TEST_GIT" update-ref "$TASK302_TEST_REF" "$TASK302_TEST_MOVED"
fi`)
	report, err := f.cleanup(t, "g7")
	if !errors.Is(err, ErrCleanupPartial) || CleanupExitCode(err) != 3 {
		t.Fatalf("moved ref must refuse atomic deletion with partial exit 3: report=%+v err=%v", report, err)
	}
	if got := gitOutput(t, f.fx.Clone, "rev-parse", "refs/heads/"+f.taskBranch); got != moved {
		t.Fatalf("moved local ref was not preserved: got %s want %s", got, moved)
	}
	if gitOutput(t, f.fx.Clone, "ls-remote", f.fx.Remote, "refs/heads/"+f.taskBranch) == "" {
		t.Fatal("remote ref was removed after local CAS refusal")
	}
	if r := f.receipt(t); !r.WorktreeRemoved || r.LocalBranchRemoved || r.RemoteBranchRemoved {
		t.Fatalf("durable partial flags do not match actual removals: %+v", r)
	}
}

func TestCleanupOnlyRetryRemoteExactLease(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	f.crashState(t, crashOpts{phase: evidencePhasePushed})
	tree := gitOutput(t, f.fx.Clone, "rev-parse", f.taskSHA+"^{tree}")
	moved := gitOutput(t, f.fx.Clone, "commit-tree", tree, "-p", f.taskSHA, "-m", "concurrent remote work")
	t.Setenv("TASK302_TEST_REF", "refs/heads/"+f.taskBranch)
	t.Setenv("TASK302_TEST_MOVED", moved)
	t.Setenv("TASK302_TEST_REMOTE", f.fx.Remote)
	f.interceptGit(t, `if [[ "$1" == "push" && "${!#}" == ":$TASK302_TEST_REF" ]]; then
  "$TASK302_TEST_GIT" push "$TASK302_TEST_REMOTE" "$TASK302_TEST_MOVED:$TASK302_TEST_REF"
fi`)
	report, err := f.cleanup(t, "g7")
	if !errors.Is(err, ErrCleanupPartial) || CleanupExitCode(err) != 3 {
		t.Fatalf("remote lease must preserve concurrent work: report=%+v err=%v", report, err)
	}
	if got := gitOutput(t, f.fx.Clone, "ls-remote", f.fx.Remote, "refs/heads/"+f.taskBranch); len(got) < len(moved) || got[:len(moved)] != moved {
		t.Fatalf("concurrent remote ref was deleted or moved: %q", got)
	}
	if r := f.receipt(t); !r.WorktreeRemoved || !r.LocalBranchRemoved || r.RemoteBranchRemoved {
		t.Fatalf("durable partial flags do not match actual removals: %+v", r)
	}
}

func TestCleanupOnlyRetryPartialThenResume(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
	f.interceptGit(t, `if [[ "$1" == "update-ref" && "${2:-}" == "-d" ]]; then
  printf '%s\n' 'injected local deletion failure' >&2
  exit 42
fi`)
	report, err := f.cleanup(t, "g7")
	if !errors.Is(err, ErrCleanupPartial) || CleanupExitCode(err) != 3 || report.Complete {
		t.Fatalf("expected actual partial removal: report=%+v err=%v", report, err)
	}
	if r := f.receipt(t); !r.WorktreeRemoved || r.LocalBranchRemoved || r.RemoteBranchRemoved {
		t.Fatalf("durable partial flags incorrect: %+v", r)
	}
	// Model a subsequent CLI invocation after the first holder has exited.
	f.amendReceiptRaw(t, func(r *Receipt) { r.Claim = deadHolderClaim(t) })
	f.interceptGit(t, "")
	before := f.remoteTargetTip(t)
	retry, err := f.cleanup(t, "g7")
	if err != nil || !retry.Complete {
		t.Fatalf("partial retry failed: report=%+v err=%v", retry, err)
	}
	f.assertResidualsGone(t, wt)
	if after := f.remoteTargetTip(t); after != before {
		t.Fatalf("cleanup re-pushed target: %s -> %s", before, after)
	}
}

func TestCleanupOnlyRetryPathRefGuard(t *testing.T) {
	for _, target := range []string{"refs/remotes/origin/develop", "develop", "refs/heads/../foreign"} {
		t.Run(target, func(t *testing.T) {
			if _, err := receiptTargetBranch(&Receipt{TargetRef: target, TaskBranch: "dev/actor/task"}); err == nil {
				t.Fatalf("unsafe target ref accepted: %q", target)
			}
		})
	}
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
	f.amendReceiptRaw(t, func(r *Receipt) { r.Provenance.DeclarationPatterns = []string{"foreign/*"} })
	if _, err := f.cleanup(t, "g7"); !errors.Is(err, ErrCleanupGuard) {
		t.Fatalf("task-pattern mismatch must refuse cleanup: %v", err)
	}
	f.assertResidualsIntact(t, wt)
}

func TestCleanupOnlyRetryConcurrentTarget(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lock, err := acquireTargetLock(ctx, newGitRepo(f.exec, f.fx.Clone), "refs/heads/develop", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()
	revision := f.receipt(t).Revision
	waiting := []*signalledWriter{{ch: make(chan struct{})}, {ch: make(chan struct{})}}
	done := make(chan error, len(waiting))
	for _, notice := range waiting {
		go func() {
			_, err := CleanupRetry(ctx, f.exec, CleanupOptions{RepoPath: f.fx.Clone, ExecutionID: f.execution.ExecutionID, Generation: "g7", LockNotice: notice})
			done <- err
		}()
	}
	for _, notice := range waiting {
		select {
		case <-notice.ch:
		case err := <-done:
			t.Fatalf("cleanup bypassed held target ownership: %v", err)
		case <-ctx.Done():
			t.Fatal("cleanup did not report target contention")
		}
	}
	f.assertResidualsIntact(t, wt)
	lock.release()
	for range waiting {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("cleanup after target release: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("cleanup did not finish after target release")
		}
	}
	f.assertResidualsGone(t, wt)
	if got := f.receipt(t).Revision; got != revision+4 {
		t.Fatalf("concurrent follower rewrote completed cleanup: revision %d -> %d", revision, got)
	}
}

func TestCleanupOnlyRetryReceiptFailureIsPartial(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
	dir := filepath.Dir(evidenceReceiptPath(f.common, f.execution.ExecutionID))
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	mode := info.Mode().Perm()
	t.Cleanup(func() {
		if err := os.Chmod(dir, mode); err != nil {
			t.Errorf("restore receipt directory: %v", err)
		}
	})
	t.Setenv("TASK302_TEST_RECEIPTS", dir)
	f.interceptGit(t, `if [[ "$1" == "worktree" && "${2:-}" == "remove" ]]; then
  "$TASK302_TEST_GIT" "$@"
  chmod 500 "$TASK302_TEST_RECEIPTS"
  exit 0
fi`)
	report, err := f.cleanup(t, "g7")
	if !errors.Is(err, ErrCleanupPartial) || CleanupExitCode(err) != 3 || report == nil || !report.Receipt.WorktreeRemoved {
		t.Fatalf("receipt failure after removal must retain partial report and exit 3: report=%+v err=%v", report, err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("worktree removal did not occur: %v", err)
	}
	if r := f.receipt(t); r.WorktreeRemoved || r.LocalBranchRemoved || r.RemoteBranchRemoved {
		t.Fatalf("failed write falsely advanced durable receipt: %+v", r)
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	f.interceptGit(t, "")
	before := f.remoteTargetTip(t)
	retry, err := f.cleanup(t, "g7")
	if err != nil || !retry.Complete {
		t.Fatalf("cleanup-only reconciliation failed: %+v %v", retry, err)
	}
	f.assertResidualsGone(t, wt)
	if after := f.remoteTargetTip(t); after != before {
		t.Fatalf("retry re-pushed integration target: %s -> %s", before, after)
	}
}
