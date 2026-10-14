// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

// Cleanup is authorized by the caller resolving to the receipt's owner
// through the native identity chain. A different actor, a different logical
// host, or an unconfigured identity refuses without touching anything.
func TestCleanupOnlyRetryOwnerGuard(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})

	t.Setenv("GZ_GIT_TASK_ACTOR", "codex")
	_, err := f.cleanup(t, "g7")
	if !errors.Is(err, ErrCleanupOwnerMismatch) {
		t.Fatalf("err = %v, want ErrCleanupOwnerMismatch", err)
	}
	f.assertResidualsIntact(t, wt)

	t.Setenv("GZ_GIT_TASK_ACTOR", "claude")
	t.Setenv("GIT_WORK_HOST", "other-host")
	_, err = f.cleanup(t, "g7")
	if !errors.Is(err, ErrCleanupOwnerMismatch) {
		t.Fatalf("err = %v, want ErrCleanupOwnerMismatch for a different logical host", err)
	}
	f.assertResidualsIntact(t, wt)

	t.Setenv("GIT_WORK_HOST", "testhost")
	report, err := f.cleanup(t, "g7")
	if err != nil || report == nil || !report.Complete {
		t.Fatalf("matching owner cleanup: err=%v report=%+v", err, report)
	}
	f.assertResidualsGone(t, wt)
}

// The remote target must actually contain the task SHA before any removal.
// A crash after the push reconciles from remote proof; a crash before the
// push refuses and preserves the residuals, which hold the only copy.
func TestCleanupOnlyRetryTargetProof(t *testing.T) {
	t.Run("crash after push completes from receipt", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		report, err := f.cleanup(t, "g7")
		if err != nil || !report.Complete {
			t.Fatalf("cleanup: err=%v report=%+v", err, report)
		}
		if !report.RemoteTargetContainsTask {
			t.Fatalf("proof not recorded: %+v", report)
		}
		f.assertResidualsGone(t, wt)
		if r := f.receipt(t); r.Phase != evidencePhaseComplete || !r.SourcePushed {
			t.Fatalf("receipt = %+v, want complete and pushed", r)
		}
	})
	t.Run("crash before receipt update reconciles without re-pushing", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhaseIntent})
		before := f.remoteTargetTip(t)
		report, err := f.cleanup(t, "g7")
		if err != nil || !report.Complete {
			t.Fatalf("cleanup: err=%v report=%+v", err, report)
		}
		if after := f.remoteTargetTip(t); after != before {
			t.Fatalf("cleanup moved the target: %s -> %s", before, after)
		}
		if r := f.receipt(t); !r.SourcePushed || r.Phase != evidencePhaseComplete {
			t.Fatalf("receipt not reconciled: %+v", r)
		}
		f.assertResidualsGone(t, wt)
	})
	t.Run("crash before push preserves residuals", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhaseIntent})
		base := gitOutput(t, f.fx.Clone, "rev-parse", f.taskSHA+"^")
		runGit(t, f.fx.Clone, "push", f.fx.Remote, "--force", base+":refs/heads/develop")
		_, err := f.cleanup(t, "g7")
		if !errors.Is(err, ErrCleanupNotPushed) {
			t.Fatalf("err = %v, want ErrCleanupNotPushed", err)
		}
		f.assertResidualsIntact(t, wt)
		if r := f.receipt(t); r.SourcePushed || r.Phase != evidencePhaseIntent {
			t.Fatalf("unpushed receipt was mutated: %+v", r)
		}
	})
}

// Cleanup requires the receipt's exact generation, the durable identity is
// immutable against replay, and a live or unknowable holder is refused —
// an elapsed lease never authorizes stealing.
func TestCleanupOnlyRetryLeaseGeneration(t *testing.T) {
	t.Run("wrong generation refuses", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		_, err := f.cleanup(t, "stale-gen")
		if !errors.Is(err, ErrCleanupGenerationMismatch) {
			t.Fatalf("err = %v, want ErrCleanupGenerationMismatch", err)
		}
		f.assertResidualsIntact(t, wt)
	})
	t.Run("identity replay fails before push", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		writeRepoFile(t, wt, "more.txt", "more\n")
		runGit(t, wt, "add", "more.txt")
		runGit(t, wt, "commit", "-m", "newer work")
		runGit(t, wt, "push", f.fx.Remote, f.taskBranch)
		moved := gitOutput(t, wt, "rev-parse", "HEAD")
		replay := f.execution
		replay.Generation = "g8"
		replay.ExpectSource = moved
		_, err := Run(context.Background(), f.exec, RunOptions{
			CheckOptions: CheckOptions{RepoPath: wt, Target: f.target},
			Execution:    &replay,
		})
		if !errors.Is(err, ErrEvidenceReplay) {
			t.Fatalf("err = %v, want ErrEvidenceReplay", err)
		}
		if tip := f.remoteTargetTip(t); tip != f.taskSHA {
			t.Fatalf("replay pushed: target moved to %s", tip)
		}
	})
	t.Run("live holder refuses", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		pid := spawnSleeper(t)
		f.amendReceiptRaw(t, func(r *Receipt) {
			r.Claim = evidenceClaim{PID: pid, ProcessHost: physicalHost()}
		})
		_, err := f.cleanup(t, "g7")
		if !errors.Is(err, ErrCleanupClaimLive) {
			t.Fatalf("err = %v, want ErrCleanupClaimLive", err)
		}
		f.assertResidualsIntact(t, wt)
	})
	t.Run("unknown holder refuses", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		f.amendReceiptRaw(t, func(r *Receipt) {
			r.Claim = evidenceClaim{PID: 4194304, ProcessHost: "somewhere-else"}
		})
		_, err := f.cleanup(t, "g7")
		if !errors.Is(err, ErrCleanupClaimUnknown) {
			t.Fatalf("err = %v, want ErrCleanupClaimUnknown", err)
		}
		f.assertResidualsIntact(t, wt)
	})
	t.Run("own pid refuses fail closed", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		// A claim PID equal to the deciding process is indistinguishable from
		// PID reuse; it must refuse like any other live holder.
		f.amendReceiptRaw(t, func(r *Receipt) {
			r.Claim = evidenceClaim{PID: os.Getpid(), ProcessHost: physicalHost()}
		})
		_, err := f.cleanup(t, "g7")
		if !errors.Is(err, ErrCleanupClaimLive) {
			t.Fatalf("err = %v, want ErrCleanupClaimLive for a reused own pid", err)
		}
		f.assertResidualsIntact(t, wt)
	})
	t.Run("same identity after push refuses re-run", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		execution := f.execution
		_, err := Run(context.Background(), f.exec, RunOptions{
			CheckOptions: CheckOptions{RepoPath: wt, Target: f.target},
			Execution:    &execution,
		})
		if !errors.Is(err, ErrEvidenceReplay) {
			t.Fatalf("err = %v, want ErrEvidenceReplay for a same-identity re-run after push", err)
		}
		if tip := f.remoteTargetTip(t); tip != f.taskSHA {
			t.Fatalf("re-run moved the target: %s", tip)
		}
	})
	t.Run("damaged receipt refuses", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		f.crashState(t, crashOpts{phase: evidencePhasePushed})
		if err := os.WriteFile(evidenceReceiptPath(f.common, "run-42"), []byte("{not json"), 0o600); err != nil {
			t.Fatalf("damage receipt: %v", err)
		}
		_, err := f.cleanup(t, "g7")
		if err == nil || errors.Is(err, ErrCleanupPartial) {
			t.Fatalf("err = %v, want a parse refusal", err)
		}
		if code := CleanupExitCode(err); code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
	})
}

// Cleanup never runs gates, readiness, watchers, merge, or a target push.
// A gate that would leave a sentinel never runs; the remote target never
// moves; no commit appears that a merge would have created.
func TestCleanupOnlyRetryNeverRunsProvider(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "gate-ran")
	f := newCleanupRetryFX(t, "@touch "+sentinel)
	// Simulate the after-push crash without ever running the engine: push
	// the target in fixture setup, leave the residuals, seed the receipt.
	runGit(t, f.fx.Clone, "push", f.fx.Remote, f.taskSHA+":refs/heads/develop")
	wt := resolvePath(t, f.fx.Worktree)
	f.markWorktree(t, wt, f.execution.Generation)
	f.seedReceipt(t, evidencePhasePushed, wt)
	cloneHead := gitOutput(t, f.fx.Clone, "rev-parse", "HEAD")

	report, err := f.cleanup(t, "g7")
	if err != nil || !report.Complete {
		t.Fatalf("cleanup: err=%v report=%+v", err, report)
	}
	if _, statErr := os.Stat(sentinel); !os.IsNotExist(statErr) {
		t.Fatalf("gate ran during cleanup (sentinel exists, statErr=%v)", statErr)
	}
	if tip := f.remoteTargetTip(t); tip != f.taskSHA {
		t.Fatalf("target moved: %s, want %s", tip, f.taskSHA)
	}
	if got := gitOutput(t, f.fx.Clone, "rev-parse", "HEAD"); got != cloneHead {
		t.Fatalf("clone HEAD moved %s -> %s; a merge or push ran", cloneHead, got)
	}
	f.assertResidualsGone(t, wt)
}

// Every mismatch preserves all residuals: a dirty tree, a moved local or
// remote ref, a worktree without this execution's registration, and a
// worktree checked out on a foreign branch.
func TestCleanupOnlyRetryPreservesResiduals(t *testing.T) {
	t.Run("dirty worktree", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		writeRepoFile(t, wt, "uncommitted.txt", "dirty\n")
		_, err := f.cleanup(t, "g7")
		if !errors.Is(err, ErrCleanupDirtyWorktree) {
			t.Fatalf("err = %v, want ErrCleanupDirtyWorktree", err)
		}
		f.assertResidualsIntact(t, wt)
	})
	t.Run("local branch moved", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		writeRepoFile(t, wt, "moved.txt", "moved\n")
		runGit(t, wt, "add", "moved.txt")
		runGit(t, wt, "commit", "-m", "branch moved on")
		moved := gitOutput(t, wt, "rev-parse", "HEAD")
		_, err := f.cleanup(t, "g7")
		if !errors.Is(err, ErrCleanupRefMoved) {
			t.Fatalf("err = %v, want ErrCleanupRefMoved", err)
		}
		// Preservation means unchanged as-found: the branch stays at its moved
		// tip, which is newer work this execution never captured.
		if got := gitOutput(t, f.fx.Clone, "rev-parse", "--verify", "refs/heads/"+f.taskBranch); got != moved {
			t.Fatalf("local task branch = %q, want the moved tip %q", got, moved)
		}
		if remote := gitOutput(t, f.fx.Clone, "ls-remote", f.fx.Remote, "refs/heads/"+f.taskBranch); remote == "" {
			t.Fatalf("remote task branch was removed")
		}
		if !containsLine(gitOutput(t, f.fx.Clone, "worktree", "list", "--porcelain"), "worktree "+wt) {
			t.Fatalf("task worktree %s was removed", wt)
		}
		if r := f.receipt(t); r.WorktreeRemoved || r.LocalBranchRemoved || r.RemoteBranchRemoved {
			t.Fatalf("receipt records removals despite refusal: %+v", r)
		}
	})
	t.Run("remote branch moved", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		writeRepoFile(t, wt, "moved.txt", "moved\n")
		runGit(t, wt, "add", "moved.txt")
		runGit(t, wt, "commit", "-m", "branch moved on")
		runGit(t, wt, "push", f.fx.Remote, f.taskBranch)
		runGit(t, wt, "reset", "--hard", f.taskSHA)
		_, err := f.cleanup(t, "g7")
		if !errors.Is(err, ErrCleanupRefMoved) {
			t.Fatalf("err = %v, want ErrCleanupRefMoved", err)
		}
		f.assertResidualsIntact(t, wt)
	})
	t.Run("same-sha worktree without registration", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed, noMarker: true})
		_, err := f.cleanup(t, "g7")
		if !errors.Is(err, ErrCleanupRegistration) {
			t.Fatalf("err = %v, want ErrCleanupRegistration", err)
		}
		f.assertResidualsIntact(t, wt)
	})
	t.Run("same-sha worktree registered to another generation", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed, markerGen: "g6"})
		_, err := f.cleanup(t, "g7")
		if !errors.Is(err, ErrCleanupRegistration) {
			t.Fatalf("err = %v, want ErrCleanupRegistration", err)
		}
		f.assertResidualsIntact(t, wt)
	})
	t.Run("worktree on a foreign branch", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		runGit(t, wt, "checkout", "-b", "other/branch")
		_, err := f.cleanup(t, "g7")
		if !errors.Is(err, ErrCleanupRegistration) {
			t.Fatalf("err = %v, want ErrCleanupRegistration for a foreign worktree", err)
		}
		f.assertResidualsIntact(t, wt)
	})
}

// Cleanup is idempotent: missing worktree, local branch, and remote branch
// are each proven absent by a successful probe; queries create nothing; the
// published JSON protocol and exit mapping hold their contract.
func TestCleanupOnlyRetryIdempotent(t *testing.T) {
	t.Run("repeat cleanup stays complete", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		report, err := f.cleanup(t, "g7")
		if err != nil || !report.Complete {
			t.Fatalf("first cleanup: err=%v report=%+v", err, report)
		}
		revision := f.receipt(t).Revision
		report, err = f.cleanup(t, "g7")
		if err != nil || !report.Complete {
			t.Fatalf("second cleanup: err=%v report=%+v", err, report)
		}
		if got := f.receipt(t).Revision; got != revision {
			t.Fatalf("idempotent cleanup rewrote the receipt: %d -> %d", revision, got)
		}
	})
	t.Run("remote-only residual finishes", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		runGit(t, f.fx.Clone, "push", f.fx.Remote, f.taskSHA+":refs/heads/"+f.taskBranch)
		report, err := f.cleanup(t, "g7")
		if err != nil || !report.Complete {
			t.Fatalf("remote-only cleanup: err=%v report=%+v", err, report)
		}
		f.assertResidualsGone(t, "")
	})
	t.Run("stale flags reconcile from absence probes", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		// Simulate a receipt whose removal flags were never written, with
		// every residual already gone: successful absence probes must
		// reconcile the flags durably and complete, not deadlock as partial.
		f.amendReceiptRaw(t, func(r *Receipt) {
			r.Phase = evidencePhasePushed
			r.SourcePushed = true
			r.WorktreeRemoved = false
			r.LocalBranchRemoved = false
			r.RemoteBranchRemoved = false
			r.Claim = deadHolderClaim(t)
		})
		report, err := f.cleanup(t, "g7")
		if err != nil || !report.Complete {
			t.Fatalf("cleanup: err=%v report=%+v", err, report)
		}
		f.assertResidualsGone(t, "")
		r := f.receipt(t)
		if !r.WorktreeRemoved || !r.LocalBranchRemoved || !r.RemoteBranchRemoved || r.Phase != evidencePhaseComplete {
			t.Fatalf("receipt not reconciled: %+v", r)
		}
	})
	t.Run("reclaim completion requires all removals", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		partial := f.receipt(t)
		if err := recordEvidenceReclaim(context.Background(), f.common, partial, ReclaimResult{RemovedWorktree: true, RemovedLocalBranch: true}); err != nil {
			t.Fatalf("record partial reclaim: %v", err)
		}
		if partial.Phase == evidencePhaseComplete || partial.RemoteBranchRemoved {
			t.Fatalf("skipped removal completed the receipt: %+v", partial)
		}
		full := f.receipt(t)
		if err := recordEvidenceReclaim(context.Background(), f.common, full, ReclaimResult{RemovedWorktree: true, RemovedLocalBranch: true, RemovedRemoteBranch: true}); err != nil {
			t.Fatalf("record full reclaim: %v", err)
		}
		if full.Phase != evidencePhaseComplete {
			t.Fatalf("proven removals did not complete the receipt: %+v", full)
		}
	})
	t.Run("missing receipt creates nothing", func(t *testing.T) {
		bare := testutil.TempGitRepoWithCommit(t)
		common := gitOutput(t, bare, "rev-parse", "--path-format=absolute", "--git-common-dir")
		executor := gitcmd.NewExecutor()
		_, err := QueryReceipt(context.Background(), executor, ReceiptOptions{RepoPath: bare, ExecutionID: "nope", Generation: "g"})
		if !errors.Is(err, ErrCleanupReceiptMissing) {
			t.Fatalf("err = %v, want ErrCleanupReceiptMissing", err)
		}
		_, err = CleanupRetry(context.Background(), executor, CleanupOptions{RepoPath: bare, ExecutionID: "nope", Generation: "g"})
		if !errors.Is(err, ErrCleanupReceiptMissing) {
			t.Fatalf("err = %v, want ErrCleanupReceiptMissing", err)
		}
		if _, statErr := os.Stat(evidenceRoot(common)); !os.IsNotExist(statErr) {
			t.Fatalf("query created evidence state (statErr=%v)", statErr)
		}
	})
	t.Run("protocol schema and exit mapping", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		report := f.realRun(t)
		proto := BuildRunProtocol(report, nil, 0)
		data, err := json.Marshal(proto)
		if err != nil {
			t.Fatalf("marshal run protocol: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("unmarshal run protocol: %v", err)
		}
		for _, key := range []string{"schema_version", "command", "integrated", "execution_id", "generation", "owner", "task_sha", "target_ref", "source_pushed", "receipt_phase", "worktree_removed", "local_branch_removed", "remote_branch_removed", "exit_code"} {
			if _, ok := doc[key]; !ok {
				t.Fatalf("run protocol lacks %q: %s", key, data)
			}
		}
		owner, ok := doc["owner"].(map[string]any)
		if !ok || owner["actor"] != "claude" || owner["host"] != "testhost" || owner["kind"] != "agent" {
			t.Fatalf("run protocol owner = %v", doc["owner"])
		}
		if proto.ExitCode != 0 || !proto.Integrated || proto.ReceiptPhase != evidencePhaseComplete {
			t.Fatalf("run protocol = %+v", proto)
		}
		cleanupDoc, err := json.Marshal(BuildCleanupProtocol(&CleanupReport{Receipt: report.Receipt, Complete: true}, nil, 0))
		if err != nil {
			t.Fatalf("marshal cleanup protocol: %v", err)
		}
		if !strings.Contains(string(cleanupDoc), `"remote_target_contains_task"`) || !strings.Contains(string(cleanupDoc), `"residuals_preserved"`) {
			t.Fatalf("cleanup protocol = %s", cleanupDoc)
		}
		if CleanupExitCode(nil) != 0 || CleanupExitCode(errors.New("boom")) != 2 || CleanupExitCode(fmt.Errorf("wrapped: %w", ErrCleanupPartial)) != 3 {
			t.Fatalf("exit mapping broken")
		}
	})
}

func TestCleanupOnlyRetrySymbolicRefPreservesReferent(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	f.crashState(t, crashOpts{phase: evidencePhasePushed})
	protected := "refs/heads/develop"
	runGit(t, f.fx.Clone, "update-ref", protected, f.taskSHA)
	t.Setenv("TASK302_TEST_REF", "refs/heads/"+f.taskBranch)
	t.Setenv("TASK302_TEST_REFERENT", protected)
	f.interceptGit(t, `if [[ "$1" == "update-ref" && "${2:-}" == "-d" ]]; then
  "$TASK302_TEST_GIT" symbolic-ref "$TASK302_TEST_REF" "$TASK302_TEST_REFERENT"
fi`)
	_, _ = f.cleanup(t, "g7")
	got, present, err := newGitRepo(gitcmd.NewExecutor(), f.fx.Clone).revParse(context.Background(), protected)
	if err != nil || !present || got != f.taskSHA {
		t.Fatalf("symbolic task deletion damaged protected referent: oid=%s present=%t err=%v", got, present, err)
	}
}

func TestCleanupOnlyRetryMovedWorktree(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
	moved := filepath.Join(t.TempDir(), "moved-taskwt")
	runGit(t, f.fx.Clone, "worktree", "move", wt, moved)
	report, err := f.cleanup(t, "g7")
	if err == nil || (report != nil && report.Complete) {
		t.Fatalf("moved branch worktree was treated as absent: report=%+v err=%v", report, err)
	}
	f.assertResidualsIntact(t, resolvePath(t, moved))
	if r := f.receipt(t); r.WorktreeRemoved || r.LocalBranchRemoved || r.RemoteBranchRemoved {
		t.Fatalf("moved worktree advanced removal evidence: %+v", r)
	}
}

func TestCleanupOnlyRetryPreservesCallerResources(t *testing.T) {
	for _, kind := range []string{"bare", "sibling-lease"} {
		t.Run(kind, func(t *testing.T) {
			f := newCleanupRetryFX(t, "@true")
			wt := resolvePath(t, f.fx.Worktree)
			var resource string
			if kind == "bare" {
				writeRepoFile(t, wt, ".gitignore", "nested/\n")
				runGit(t, wt, "add", ".gitignore")
				runGit(t, wt, "commit", "-m", "ignore retry resource")
				runGit(t, wt, "push", f.fx.Remote, f.taskBranch)
				f.taskSHA = gitOutput(t, wt, "rev-parse", "HEAD")
				f.execution.ExpectSource = f.taskSHA
				nested := filepath.Join(wt, "nested")
				runGit(t, wt, "init", "--bare", nested)
				resource = filepath.Join(nested, "HEAD")
			} else {
				dir := filepath.Join(f.common, "sibling-provision-leases")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(SiblingLease{Root: wt, ID: "caller-owned"})
				if err != nil {
					t.Fatal(err)
				}
				resource = filepath.Join(dir, "caller-owned.json")
				if err := os.WriteFile(resource, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if r := f.realRun(t); !r.Reclaim.Incomplete() || r.Reclaim.GitProof != nil {
				t.Fatalf("normal reclaim did not preserve caller resource: %+v", r.Reclaim)
			}
			before, err := os.ReadFile(resource)
			if err != nil {
				t.Fatal(err)
			}
			f.amendReceiptRaw(t, func(r *Receipt) { r.Claim = deadHolderClaim(t) })
			report, err := f.cleanup(t, "g7")
			if err == nil || (report != nil && report.Complete) {
				t.Fatalf("retry bypassed caller-resource refusal: report=%+v err=%v", report, err)
			}
			f.assertResidualsIntact(t, wt)
			after, err := os.ReadFile(resource)
			if err != nil || !bytes.Equal(after, before) {
				t.Fatalf("caller resource was changed: %v", err)
			}
		})
	}
}

func TestCleanupOnlyRetryLocalProbeFailure(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
	t.Setenv("TASK302_TEST_REF", "refs/heads/"+f.taskBranch)
	f.interceptGit(t, `if [[ "$1" == "rev-parse" && "${!#}" == "$TASK302_TEST_REF" ]] || [[ "$1" == "for-each-ref" && "${!#}" == "$TASK302_TEST_REF" ]]; then
  printf '%s\n' 'injected local inventory failure' >&2
  exit 42
fi`)
	report, err := f.cleanup(t, "g7")
	if err == nil || (report != nil && report.Complete) {
		t.Fatalf("failed local query became complete absence: report=%+v err=%v", report, err)
	}
	f.assertResidualsIntact(t, wt)
	if r := f.receipt(t); r.WorktreeRemoved || r.LocalBranchRemoved || r.RemoteBranchRemoved {
		t.Fatalf("failed inventory advanced removal evidence: %+v", r)
	}
}

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
