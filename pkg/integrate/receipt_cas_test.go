// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

// seedCASReceipt builds a minimal valid receipt for file-level CAS tests.
func seedCASReceipt(executionID, targetRef string) *Receipt {
	return &Receipt{
		SchemaVersion:       evidenceSchemaVersion,
		ExecutionID:         executionID,
		Generation:          "g1",
		Owner:               OwnerIdentity{Actor: "claude", Host: "testhost", Kind: "agent"},
		RepositoryCommonDir: "/repo/.git",
		TaskBranch:          "dev/claude/x1",
		TaskSHA:             "0123456789abcdef0123456789abcdef01234567",
		TargetRef:           targetRef,
		Remote:              "origin",
		Phase:               evidencePhaseIntent,
	}
}

// Parallel same-execution-id creations with different targets must produce
// exactly one winner: the per-execution write lock plus the create-vs-
// existing check means every loser fails instead of overwriting the winner.
func TestReceiptCreateCollisionExactlyOneWinner(t *testing.T) {
	common := t.TempDir()
	const workers = 8
	start := make(chan struct{})
	errs := make(chan error, workers)
	for i := range workers {
		go func(i int) {
			<-start
			errs <- createReceipt(context.Background(), common, seedCASReceipt("exec-collide", fmt.Sprintf("refs/heads/target-%d", i)))
		}(i)
	}
	close(start)
	wins, refusals := 0, 0
	for range workers {
		if err := <-errs; err == nil {
			wins++
		} else if errors.Is(err, ErrReceiptCASConflict) && errors.Is(err, ErrEvidenceReplay) {
			refusals++
		} else {
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	if wins != 1 || refusals != workers-1 {
		t.Fatalf("wins=%d refusals=%d, want exactly 1 winner and %d refusals", wins, refusals, workers-1)
	}
	onDisk, ok, err := readReceipt(common, "exec-collide")
	if err != nil || !ok {
		t.Fatalf("read winner receipt: ok=%v err=%v", ok, err)
	}
	if onDisk.Revision != 1 || onDisk.Phase != evidencePhaseIntent {
		t.Fatalf("winner receipt = revision %d phase %s, want revision 1 at intent", onDisk.Revision, onDisk.Phase)
	}
	if !strings.HasPrefix(onDisk.TargetRef, "refs/heads/target-") {
		t.Fatalf("winner receipt records a foreign target: %s", onDisk.TargetRef)
	}
	// Exactly one receipt document survives; the lock file is not a receipt.
	dir := filepath.Dir(evidenceReceiptPath(common, "exec-collide"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("list receipts dir: %v", err)
	}
	json := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			json++
		}
	}
	if json != 1 {
		t.Fatalf("found %d receipt documents, want exactly 1", json)
	}
}

// A writer holding a stale revision must not overwrite a receipt another
// writer advanced, and the refused CAS must leave its in-memory receipt —
// revision and claim included — exactly as supplied.
func TestReceiptUpdateStaleRevisionRefused(t *testing.T) {
	common := t.TempDir()
	if err := createReceipt(context.Background(), common, seedCASReceipt("exec-stale", "refs/heads/develop")); err != nil {
		t.Fatalf("seed create: %v", err)
	}
	stale, ok, err := readReceipt(common, "exec-stale")
	if err != nil || !ok {
		t.Fatalf("read stale: ok=%v err=%v", ok, err)
	}
	// A concurrent writer advances the durable document.
	fresh, _, err := readReceipt(common, "exec-stale")
	if err != nil {
		t.Fatalf("read fresh: %v", err)
	}
	fresh.SourcePushed = true
	fresh.Phase = evidencePhasePushed
	if err := writeReceipt(context.Background(), common, fresh); err != nil {
		t.Fatalf("advance receipt: %v", err)
	}
	advanced, _, err := readReceipt(common, "exec-stale")
	if err != nil {
		t.Fatalf("read advanced: %v", err)
	}

	staleBefore := *stale
	err = writeReceipt(context.Background(), common, stale)
	if !errors.Is(err, ErrReceiptCASConflict) {
		t.Fatalf("err = %v, want ErrReceiptCASConflict", err)
	}
	if stale.Revision != staleBefore.Revision {
		t.Fatalf("failed CAS mutated revision: %d -> %d", staleBefore.Revision, stale.Revision)
	}
	if stale.Claim != staleBefore.Claim {
		t.Fatalf("failed CAS mutated claim: %+v -> %+v", staleBefore.Claim, stale.Claim)
	}
	after, _, err := readReceipt(common, "exec-stale")
	if err != nil {
		t.Fatalf("read after refused write: %v", err)
	}
	if after.Revision != advanced.Revision || !after.SourcePushed {
		t.Fatalf("stale writer overwrote the advanced receipt: %+v", after)
	}
}

// A create that finds an existing receipt refuses — reporting a replay when
// the identity differs — and never mutates the candidate.
func TestReceiptCreateVsExistingRefused(t *testing.T) {
	t.Run("different identity reports replay", func(t *testing.T) {
		common := t.TempDir()
		if err := createReceipt(context.Background(), common, seedCASReceipt("exec-exists", "refs/heads/develop")); err != nil {
			t.Fatalf("seed create: %v", err)
		}
		other := seedCASReceipt("exec-exists", "refs/heads/master")
		other.Generation = "g2"
		before := *other
		err := createReceipt(context.Background(), common, other)
		if !errors.Is(err, ErrReceiptCASConflict) || !errors.Is(err, ErrEvidenceReplay) {
			t.Fatalf("err = %v, want CAS conflict wrapping ErrEvidenceReplay", err)
		}
		if other.Revision != before.Revision || other.Claim != before.Claim {
			t.Fatalf("failed create mutated the candidate: %+v", other)
		}
	})
	t.Run("same identity still refuses a second create", func(t *testing.T) {
		common := t.TempDir()
		if err := createReceipt(context.Background(), common, seedCASReceipt("exec-exists", "refs/heads/develop")); err != nil {
			t.Fatalf("seed create: %v", err)
		}
		again := seedCASReceipt("exec-exists", "refs/heads/develop")
		err := createReceipt(context.Background(), common, again)
		if !errors.Is(err, ErrReceiptCASConflict) {
			t.Fatalf("err = %v, want ErrReceiptCASConflict for a duplicate create", err)
		}
		if again.Revision != 0 || again.Claim != (evidenceClaim{}) {
			t.Fatalf("failed create mutated the candidate: %+v", again)
		}
	})
}

// Concurrent begins with one execution-id but different targets are the
// blocker scenario: target locks differ, so only the receipt write lock and
// the atomic identity check stop the loser. The loser must fail with
// ErrEvidenceReplay before run ever reaches a push.
func TestBeginRunEvidenceConcurrentCollision(t *testing.T) {
	repo := testutil.TempGitRepoWithCommit(t)
	g := newGitRepo(gitcmd.NewExecutor(), repo)
	head, ok, err := g.revParse(context.Background(), "HEAD")
	if err != nil || !ok {
		t.Fatalf("resolve HEAD: ok=%v err=%v", ok, err)
	}
	const workers = 4
	common := gitOutput(t, repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	start := make(chan struct{})
	type result struct {
		receipt *Receipt
		err     error
	}
	results := make(chan result, workers)
	begin := func(i int) {
		execution := &ExecutionContext{
			ExecutionID:  "exec-begin",
			Generation:   "g1",
			Owner:        OwnerIdentity{Actor: "claude", Host: "testhost", Kind: "agent"},
			ExpectSource: head,
		}
		opts := RunOptions{Execution: execution}
		check := &CheckReport{Plan: TargetPlan{Branch: "dev/claude/x1", BranchSHA: head, Remote: "origin", TargetSHA: head}}
		r, err := beginRunEvidence(context.Background(), g.exec, g, common, opts, check, head, fmt.Sprintf("target-%d", i))
		results <- result{receipt: r, err: err}
	}
	for i := range workers {
		go func(i int) {
			<-start
			begin(i)
		}(i)
	}
	close(start)
	wins, replays := 0, 0
	for range workers {
		res := <-results
		switch {
		case res.err == nil:
			wins++
		case errors.Is(res.err, ErrEvidenceReplay):
			replays++
		default:
			t.Fatalf("unexpected begin error: %v", res.err)
		}
	}
	if wins != 1 || replays != workers-1 {
		t.Fatalf("wins=%d replays=%d, want exactly 1 winner and %d replay refusals", wins, replays, workers-1)
	}
	onDisk, ok, err := readReceipt(common, "exec-begin")
	if err != nil || !ok {
		t.Fatalf("read receipt: ok=%v err=%v", ok, err)
	}
	if onDisk.Revision != 1 {
		t.Fatalf("receipt revision = %d, want 1 after one winner", onDisk.Revision)
	}
	if !strings.HasPrefix(onDisk.TargetRef, "refs/heads/target-") {
		t.Fatalf("receipt target = %s, want one of the racing targets", onDisk.TargetRef)
	}
}

// cleanupRereadGuard is the after-lock re-read gate: the receipt must still
// name the exact locked target and carry an unchanged immutable identity.
func TestCleanupRereadGuard(t *testing.T) {
	locked := seedCASReceipt("exec-guard", "refs/heads/develop")
	t.Run("changed worktree refuses", func(t *testing.T) {
		moved := seedCASReceipt("exec-guard", "refs/heads/develop")
		moved.TaskWorktree = locked.TaskWorktree + "-replacement"
		if err := cleanupRereadGuard(locked, moved); !errors.Is(err, ErrEvidenceReplay) {
			t.Fatalf("err = %v, want ErrEvidenceReplay for a changed worktree", err)
		}
		common := t.TempDir()
		if err := createReceipt(context.Background(), common, locked); err != nil {
			t.Fatal(err)
		}
		moved.Revision = locked.Revision
		if err := writeReceipt(context.Background(), common, moved); !errors.Is(err, ErrEvidenceReplay) {
			t.Fatalf("changed worktree receipt overwrite: %v", err)
		}
		stored, ok, err := readReceipt(common, locked.ExecutionID)
		if err != nil || !ok || stored.TaskWorktree != locked.TaskWorktree {
			t.Fatalf("original worktree evidence changed: receipt=%+v err=%v", stored, err)
		}
	})

	t.Run("moved target refuses", func(t *testing.T) {
		moved := seedCASReceipt("exec-guard", "refs/heads/master")
		if err := cleanupRereadGuard(locked, moved); !errors.Is(err, ErrEvidenceReplay) {
			t.Fatalf("err = %v, want ErrEvidenceReplay for a moved target ref", err)
		}
	})
	t.Run("changed task identity refuses", func(t *testing.T) {
		moved := seedCASReceipt("exec-guard", "refs/heads/develop")
		moved.TaskSHA = "fedcba9876543210fedcba9876543210fedcba98"
		if err := cleanupRereadGuard(locked, moved); !errors.Is(err, ErrEvidenceReplay) {
			t.Fatalf("err = %v, want ErrEvidenceReplay for a changed task identity", err)
		}
	})
	t.Run("changed owner refuses", func(t *testing.T) {
		moved := seedCASReceipt("exec-guard", "refs/heads/develop")
		moved.Owner = OwnerIdentity{Actor: "codex", Host: "testhost", Kind: "agent"}
		if err := cleanupRereadGuard(locked, moved); !errors.Is(err, ErrEvidenceReplay) {
			t.Fatalf("err = %v, want ErrEvidenceReplay for a changed owner", err)
		}
	})
	t.Run("mutable-only changes pass", func(t *testing.T) {
		resumed := seedCASReceipt("exec-guard", "refs/heads/develop")
		resumed.SourcePushed = true
		resumed.Phase = evidencePhasePushed
		resumed.Revision = locked.Revision + 1
		if err := cleanupRereadGuard(locked, resumed); err != nil {
			t.Fatalf("mutable-only rewrite refused: %v", err)
		}
	})
}

// signalledWriter closes ch on the first write, so a test can observe that a
// lock waiter has announced itself.
type signalledWriter struct {
	once sync.Once
	ch   chan struct{}
}

func (w *signalledWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.ch) })
	return len(p), nil
}

// End to end: while cleanup waits for the target lock, the receipt is
// rewritten underneath it. The after-lock re-read must refuse the moved
// identity before any proof or deletion, leaving every residual intact.
func TestCleanupRereadGuardWhileWaitingForLock(t *testing.T) {
	t.Run("target moved", func(t *testing.T) {
		runCleanupRereadWait(t, func(r *Receipt) { r.TargetRef = "refs/heads/master" })
	})
	t.Run("task sha changed", func(t *testing.T) {
		runCleanupRereadWait(t, func(r *Receipt) { r.TaskSHA += "00" })
	})
}

func runCleanupRereadWait(t *testing.T, rewrite func(*Receipt)) {
	t.Helper()
	f := newCleanupRetryFX(t, "@true")
	f.realRun(t)
	wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})

	g := newGitRepo(f.exec, f.fx.Clone)
	lock, err := acquireTargetLock(context.Background(), g, "refs/heads/develop", nil)
	if err != nil {
		t.Fatalf("hold target lock: %v", err)
	}
	waiting := &signalledWriter{ch: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		_, err := CleanupRetry(context.Background(), f.exec, CleanupOptions{
			RepoPath:    f.fx.Clone,
			ExecutionID: "run-42",
			Generation:  "g7",
			LockNotice:  waiting,
		})
		done <- err
	}()
	<-waiting.ch
	f.amendReceiptRaw(t, rewrite)
	amended := f.receipt(t)
	lock.release()

	select {
	case err := <-done:
		if !errors.Is(err, ErrEvidenceReplay) {
			t.Fatalf("err = %v, want ErrEvidenceReplay from the re-read guard", err)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("cleanup did not finish after the target lock was released")
	}
	f.assertResidualsIntact(t, wt)
	if after := f.receipt(t); after.Revision != amended.Revision {
		t.Fatalf("refused cleanup rewrote the receipt: revision %d -> %d", amended.Revision, after.Revision)
	}
}
