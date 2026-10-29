// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fixture struct {
	canonical string
	task      string
	common    string
	source    string
	taskHead  string
}

type testLease struct {
	records []ApplyRecord
	fail    string
}

func (lease *testLease) Check(context.Context) error { return nil }
func (lease *testLease) Record(_ context.Context, record ApplyRecord) error {
	lease.records = append(lease.records, record)
	if record.Step == lease.fail {
		return errors.New("record failed")
	}
	return nil
}
func (lease *testLease) Close() error { return nil }

type testFence struct{ lease *testLease }

func (fence testFence) Acquire(context.Context, ApplyPlan) (ApplyLease, error) {
	return fence.lease, nil
}

type deniedFence struct{}

func (deniedFence) Acquire(context.Context, ApplyPlan) (ApplyLease, error) {
	return nil, errors.New("denied")
}

func newFixture(t *testing.T, conflict bool) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(root, "control")
	runGit(t, root, "-c", "init.defaultBranch=main", "init", canonical)
	configure(t, canonical)
	write(t, filepath.Join(canonical, "shared.txt"), "base\n")
	write(t, filepath.Join(canonical, ".gitignore"), "generated/\n")
	runGit(t, canonical, "add", "shared.txt", ".gitignore")
	runGit(t, canonical, "commit", "-m", "base")
	task := filepath.Join(root, "task")
	runGit(t, canonical, "worktree", "add", "-b", "task/demo", task, "HEAD")
	write(t, filepath.Join(task, "task.txt"), "task\n")
	if conflict {
		write(t, filepath.Join(task, "shared.txt"), "task\n")
		runGit(t, task, "add", "shared.txt", "task.txt")
	} else {
		runGit(t, task, "add", "task.txt")
	}
	runGit(t, task, "commit", "-m", "task")
	taskHead := gitOutput(t, task, "rev-parse", "HEAD")
	write(t, filepath.Join(canonical, "shared.txt"), "source\n")
	write(t, filepath.Join(canonical, "generated", "source.txt"), "source\n")
	runGit(t, canonical, "add", "shared.txt")
	runGit(t, canonical, "add", "-f", "generated/source.txt")
	runGit(t, canonical, "commit", "-m", "source")
	return fixture{canonical: canonical, task: task, common: gitOutput(t, canonical, "rev-parse", "--path-format=absolute", "--git-common-dir"), source: gitOutput(t, canonical, "rev-parse", "HEAD"), taskHead: taskHead}
}

func (f fixture) request() Request {
	return Request{CanonicalWorktree: f.canonical, TaskWorktree: f.task, CommonDir: f.common, TaskBranch: "task/demo", ExpectedTaskHEAD: f.taskHead, SourceRef: "main", SourceOID: f.source}
}

func TestPrepareCleanRetainsCandidateWithoutChangingOriginal(t *testing.T) {
	f := newFixture(t, false)
	worktreesBefore := worktreeCount(t, f.canonical)
	result := PrepareClean(context.Background(), f.request())
	if result.Outcome != OutcomePrepared || result.Prepared == nil || !result.OriginalUnchanged {
		t.Fatalf("prepare = %+v, want prepared unchanged candidate", result)
	}
	if got := gitOutput(t, f.task, "rev-parse", "HEAD"); got != f.taskHead {
		t.Fatalf("task HEAD = %s, want unchanged %s", got, f.taskHead)
	}
	if got := gitOutput(t, f.canonical, "rev-parse", "main"); got != f.source {
		t.Fatalf("source = %s, want unchanged %s", got, f.source)
	}
	if got := gitOutput(t, f.canonical, "rev-parse", refPrefix+result.Prepared.Handle); got != result.Prepared.CandidateOID {
		t.Fatalf("candidate ref = %s, want %s", got, result.Prepared.CandidateOID)
	}
	if result.Prepared.CandidateOID == f.taskHead || result.Prepared.CandidateOID == f.source {
		t.Fatalf("candidate was not a rebased commit: %s", result.Prepared.CandidateOID)
	}
	if got := gitOutput(t, f.canonical, "merge-base", f.source, result.Prepared.CandidateOID); got != f.source {
		t.Fatalf("source is not candidate ancestor: %s", got)
	}
	if got := gitOutput(t, f.canonical, "show", result.Prepared.CandidateOID+":shared.txt"); got != "source" {
		t.Fatalf("candidate shared content = %q, want source", got)
	}
	if got := gitOutput(t, f.canonical, "show", result.Prepared.CandidateOID+":task.txt"); got != "task" {
		t.Fatalf("candidate task content = %q, want task", got)
	}
	if got := worktreeCount(t, f.canonical); got != worktreesBefore {
		t.Fatalf("scratch worktree remained: got %d, want %d", got, worktreesBefore)
	}
	restored, err := Restore(context.Background(), f.canonical, result.Prepared.Descriptor())
	if err != nil || restored.CandidateOID != result.Prepared.CandidateOID {
		t.Fatalf("restore durable candidate: candidate=%+v err=%v", restored, err)
	}
	if err := Release(context.Background(), f.canonical, *restored); err != nil {
		t.Fatalf("release candidate: %v", err)
	}
	if cmd := exec.CommandContext(context.Background(), "git", "-C", f.canonical, "rev-parse", "--verify", refPrefix+result.Prepared.Handle); cmd.Run() == nil { //nolint:gosec // fixed fixture argv
		t.Fatal("released candidate ref remains")
	}
}

func TestApplyLocalMovesPinnedBranchWithCheckpoint(t *testing.T) {
	f := newFixture(t, false)
	prepared := PrepareClean(context.Background(), f.request())
	if prepared.Prepared == nil {
		t.Fatalf("prepare = %+v", prepared)
	}
	plan := ApplyPlan{PlanID: "apply-1", GenerationDigest: "generation", AuthorityDigest: "authority", ProviderBindingDigest: "provider", CanonicalWorktree: f.canonical, TaskWorktree: f.task, CommonDir: f.common, TaskBranch: "task/demo", SourceRef: "main", ExpectedTaskHEAD: f.taskHead, ExpectedSourceOID: f.source, AllowedPaths: []string{"generated/source.txt", "shared.txt"}}
	lease := &testLease{}
	plan.ProviderBindingDigest = BindingDigest(plan, prepared.Prepared.Descriptor())
	result := ApplyLocal(context.Background(), ApplyRequest{Plan: plan, Candidate: prepared.Prepared.Descriptor(), Fence: testFence{lease: lease}})
	if result.Outcome != OutcomeApplied || !result.Applied || result.ActualHEAD != prepared.Prepared.CandidateOID {
		t.Fatalf("apply = %+v", result)
	}
	if got := gitOutput(t, f.task, "symbolic-ref", "HEAD"); got != "refs/heads/task/demo" {
		t.Fatalf("head = %s", got)
	}
	if got := gitOutput(t, f.canonical, "rev-parse", result.CheckpointRef); got != f.taskHead {
		t.Fatalf("checkpoint = %s", got)
	}
	if len(lease.records) != 8 {
		t.Fatalf("records = %d", len(lease.records))
	}
}

func TestApplyLocalFenceDenialDoesNotMoveTask(t *testing.T) {
	f := newFixture(t, false)
	p := PrepareClean(context.Background(), f.request()).Prepared
	plan := ApplyPlan{PlanID: "p", GenerationDigest: "g", AuthorityDigest: "a", ProviderBindingDigest: "b", CanonicalWorktree: f.canonical, TaskWorktree: f.task, CommonDir: f.common, TaskBranch: "task/demo", SourceRef: "main", ExpectedTaskHEAD: f.taskHead, ExpectedSourceOID: f.source}
	for _, fence := range []ApplyFenceProvider{nil, deniedFence{}} {
		got := ApplyLocal(context.Background(), ApplyRequest{Plan: plan, Candidate: p.Descriptor(), Fence: fence})
		if got.Outcome != OutcomeRefused || gitOutput(t, f.task, "rev-parse", "HEAD") != f.taskHead {
			t.Fatalf("denial=%+v", got)
		}
	}
}

func TestApplyLocalRecordFailure(t *testing.T) {
	stages := []string{"checkpoint-intent", "checkpoint-created", "checkout-intent", "checked-out", "branch-intent", "branch-updated", "attach-intent", "attached"}
	for index, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			f := newFixture(t, false)
			prepared := PrepareClean(context.Background(), f.request()).Prepared
			plan := ApplyPlan{PlanID: "p", GenerationDigest: "g", AuthorityDigest: "a", ProviderBindingDigest: "b", CanonicalWorktree: f.canonical, TaskWorktree: f.task, CommonDir: f.common, TaskBranch: "task/demo", SourceRef: "main", ExpectedTaskHEAD: f.taskHead, ExpectedSourceOID: f.source, AllowedPaths: []string{"generated/source.txt", "shared.txt"}}
			lease := &testLease{fail: stage}
			plan.ProviderBindingDigest = BindingDigest(plan, prepared.Descriptor())
			got := ApplyLocal(context.Background(), ApplyRequest{Plan: plan, Candidate: prepared.Descriptor(), Fence: testFence{lease: lease}})
			if got.Outcome == OutcomeApplied || len(lease.records) != index+1 {
				t.Fatalf("result=%+v records=%d", got, len(lease.records))
			}
			if index < 3 && gitOutput(t, f.task, "rev-parse", "HEAD") != f.taskHead {
				t.Fatal("pre-checkout failure moved task")
			}
			if index > 0 {
				if head := gitOutput(t, f.canonical, "rev-parse", got.CheckpointRef); head != f.taskHead {
					t.Fatalf("checkpoint=%s", head)
				}
			}
		})
	}
}

func TestPrepareCleanConflictLeavesOriginalUnchanged(t *testing.T) {
	f := newFixture(t, true)
	result := PrepareClean(context.Background(), f.request())
	if result.Outcome != OutcomeConflict || !result.OriginalUnchanged {
		t.Fatalf("prepare = %+v, want conflict with unchanged original", result)
	}
	if got := gitOutput(t, f.task, "rev-parse", "HEAD"); got != f.taskHead {
		t.Fatalf("task HEAD changed to %s", got)
	}
	if status := gitOutput(t, f.task, "status", "--porcelain"); status != "" {
		t.Fatalf("original task became dirty: %q", status)
	}
}

func TestPrepareCleanRejectsDirtyAndUnsafeConfiguration(t *testing.T) {
	f := newFixture(t, false)
	write(t, filepath.Join(f.task, "untracked.txt"), "x\n")
	if got := PrepareClean(context.Background(), f.request()); got.Outcome != OutcomeRefused {
		t.Fatalf("dirty prepare = %+v, want refused", got)
	}
	if err := os.Remove(filepath.Join(f.task, "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	runGit(t, f.task, "config", "--local", "filter.unsafe.smudge", "cat")
	if got := PrepareClean(context.Background(), f.request()); got.Outcome != OutcomeRefused {
		t.Fatalf("unsafe config prepare = %+v, want refused", got)
	}
	runGit(t, f.task, "config", "--unset", "filter.unsafe.smudge")
	runGit(t, f.task, "config", "--local", "notes.rewrite.rebase", "true")
	if got := PrepareClean(context.Background(), f.request()); got.Outcome != OutcomeRefused {
		t.Fatalf("notes config prepare = %+v, want refused", got)
	}
	if got := gitOutput(t, f.task, "rev-parse", "HEAD"); got != f.taskHead {
		t.Fatalf("rejected task changed to %s", got)
	}
}

func TestPrepareCleanRejectsIgnoredCollisionSymlinkAndWrongPin(t *testing.T) {
	f := newFixture(t, false)
	write(t, filepath.Join(f.task, "generated", "local.txt"), "local\n")
	if got := PrepareClean(context.Background(), f.request()); got.Outcome != OutcomeRefused {
		t.Fatalf("ignored collision prepare = %+v, want refused", got)
	}
	if err := os.Remove(filepath.Join(f.task, "generated", "local.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.task, "generated")); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(f.task), "task-alias")
	if err := os.Symlink(f.task, alias); err != nil {
		t.Fatal(err)
	}
	symlinkRequest := f.request()
	symlinkRequest.TaskWorktree = alias
	if got := PrepareClean(context.Background(), symlinkRequest); got.Outcome != OutcomeRefused {
		t.Fatalf("symlink worktree prepare = %+v, want refused", got)
	}
	wrongPin := f.request()
	wrongPin.SourceOID = strings.Repeat("0", 40)
	if got := PrepareClean(context.Background(), wrongPin); got.Outcome != OutcomeRefused {
		t.Fatalf("wrong source pin prepare = %+v, want refused", got)
	}
	if got := gitOutput(t, f.task, "rev-parse", "HEAD"); got != f.taskHead {
		t.Fatalf("rejected task changed to %s", got)
	}
}

func TestPrepareCleanRejectsCandidateRefCollision(t *testing.T) {
	f := newFixture(t, false)
	handle := strings.Repeat("a", 32)
	ref := refPrefix + handle
	runGit(t, f.canonical, "update-ref", ref, f.source)
	result := prepareClean(context.Background(), f.request(), func() (string, error) { return handle, nil })
	if result.Outcome != OutcomeRefused {
		t.Fatalf("collision prepare = %+v, want refused after collision proof", result)
	}
	if got := gitOutput(t, f.canonical, "rev-parse", ref); got != f.source {
		t.Fatalf("collision ref changed to %s, want %s", got, f.source)
	}
	if got := gitOutput(t, f.task, "rev-parse", "HEAD"); got != f.taskHead {
		t.Fatalf("collision changed original task to %s", got)
	}
}

func TestPrepareCleanRefusesSymlinkedMetadataSurfaces(t *testing.T) {
	f := newFixture(t, false)
	outside := t.TempDir()
	namespace := filepath.Join(f.common, "refs", "gz-git", "tasksync")
	if err := os.MkdirAll(filepath.Dir(namespace), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, namespace); err != nil {
		t.Fatal(err)
	}
	if got := PrepareClean(context.Background(), f.request()); got.Outcome != OutcomeRefused {
		t.Fatalf("symlinked candidate namespace prepare = %+v, want refused", got)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside namespace changed: entries=%v err=%v", entries, err)
	}
	if err := os.Remove(namespace); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(f.common, "objects", "aa")); err != nil {
		t.Fatal(err)
	}
	if err := safeCommonSurfaces(f.common, ""); err == nil {
		t.Fatal("symlinked object fanout accepted")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	control := filepath.Join(root, "control")
	runGit(t, root, "-c", "init.defaultBranch=main", "init", control)
	common := gitOutput(t, control, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err := os.Symlink(outside, filepath.Join(common, "worktrees")); err != nil {
		t.Fatal(err)
	}
	if err := safeCommonSurfaces(common, ""); err == nil {
		t.Fatal("symlinked worktrees metadata accepted")
	}
	if err := safeExistingPath(f.common, "../escape", false); err == nil {
		t.Fatal("metadata traversal accepted")
	}
	if err := safeExistingPath(f.common, "missing/child", true); err != nil {
		t.Fatalf("missing metadata path must be safe before creation: %v", err)
	}
}

func TestPrepareCleanCleansScratchWhenHandleCreationFails(t *testing.T) {
	f := newFixture(t, false)
	before := worktreeCount(t, f.canonical)
	result := prepareClean(context.Background(), f.request(), func() (string, error) {
		return "", errors.New("entropy unavailable")
	})
	if result.Outcome != OutcomeRefused || result.RetainedScratch {
		t.Fatalf("handle failure = %+v, want clean refusal", result)
	}
	if got := worktreeCount(t, f.canonical); got != before {
		t.Fatalf("scratch worktree remained: got %d, want %d", got, before)
	}
}

func TestValidationRestoreAndCleanupDefenses(t *testing.T) {
	f := newFixture(t, false)
	request := f.request()
	request.TaskBranch = "bad branch"
	if err := validateRequest(request); err == nil {
		t.Fatal("invalid branch accepted")
	}
	request = f.request()
	request.TaskWorktree = request.CanonicalWorktree
	if err := validateRequest(request); err == nil {
		t.Fatal("canonical task worktree accepted")
	}
	request = f.request()
	request.CommonDir = filepath.Join(f.common, "missing")
	if err := validateRequest(request); err == nil {
		t.Fatal("missing common directory accepted")
	}
	if _, err := Restore(context.Background(), f.canonical, CandidateDescriptor{}); err == nil {
		t.Fatal("invalid descriptor accepted")
	}
	if _, err := Restore(context.Background(), f.canonical, CandidateDescriptor{Handle: strings.Repeat("a", 32), CandidateOID: strings.Repeat("a", 40), SourceOID: strings.Repeat("b", 40), OriginalHEAD: strings.Repeat("c", 40), CommonDir: f.common}); err == nil {
		t.Fatal("missing candidate ref accepted")
	}
	gitExec, err := newExecutor()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanupScratch(context.Background(), gitExec, f.canonical, root, filepath.Join(root, "worktree"), ""); err != nil {
		t.Fatalf("empty owned scratch cleanup: %v", err)
	}
	if err := cleanupScratch(context.Background(), gitExec, f.canonical, f.canonical, filepath.Join(root, "worktree"), ""); err == nil {
		t.Fatal("mismatched scratch root accepted")
	}
	remainingRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	remainingScratch := filepath.Join(remainingRoot, "worktree")
	if err := os.Mkdir(remainingScratch, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := cleanupScratch(context.Background(), gitExec, f.canonical, remainingRoot, remainingScratch, ""); err == nil {
		t.Fatal("unregistered remaining scratch accepted")
	}
	if err := os.Remove(remainingScratch); err != nil {
		t.Fatal(err)
	}
	linkedRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	linkedScratch := filepath.Join(linkedRoot, "worktree")
	if _, err := run(context.Background(), gitExec, f.canonical, "worktree", "add", "--detach", "--", linkedScratch, f.taskHead); err != nil {
		t.Fatal(err)
	}
	adminID, err := output(context.Background(), gitExec, linkedScratch, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanupScratch(context.Background(), gitExec, f.canonical, linkedRoot, linkedScratch, ""); err == nil {
		t.Fatal("linked scratch without administrative identity accepted")
	}
	if err := cleanupScratch(context.Background(), gitExec, f.canonical, linkedRoot, linkedScratch, filepath.Join(linkedRoot, "wrong")); err == nil {
		t.Fatal("linked scratch with changed administrative identity accepted")
	}
	write(t, filepath.Join(linkedScratch, "unexpected.txt"), "do not remove\n")
	if err := cleanupScratch(context.Background(), gitExec, f.canonical, linkedRoot, linkedScratch, adminID); err == nil {
		t.Fatal("dirty linked scratch accepted for removal")
	}
	if err := os.Remove(filepath.Join(linkedScratch, "unexpected.txt")); err != nil {
		t.Fatal(err)
	}
	if err := cleanupScratch(context.Background(), gitExec, f.canonical, linkedRoot, linkedScratch, adminID); err != nil {
		t.Fatalf("linked owned scratch cleanup: %v", err)
	}
}

func TestReleaseRejectsCandidateFromAnotherRepositoryAndMovedRef(t *testing.T) {
	f := newFixture(t, false)
	result := PrepareClean(context.Background(), f.request())
	if result.Prepared == nil {
		t.Fatalf("prepare = %+v", result)
	}
	other := newFixture(t, false)
	if err := Release(context.Background(), other.canonical, *result.Prepared); err == nil {
		t.Fatal("release accepted candidate from another repository")
	}
	runGit(t, f.canonical, "update-ref", refPrefix+result.Prepared.Handle, f.source)
	if err := Release(context.Background(), f.canonical, *result.Prepared); err == nil {
		t.Fatal("release accepted candidate ref whose value moved")
	}
}

func TestInternalGuardsFailClosed(t *testing.T) {
	f := newFixture(t, false)
	gitExec, err := newExecutor()
	if err != nil {
		t.Fatal(err)
	}
	if conflicted, err := ownedRebaseConflict(context.Background(), gitExec, f.task); err != nil || conflicted {
		t.Fatalf("clean worktree conflict = %t, %v", conflicted, err)
	}
	rebaseState, err := output(context.Background(), gitExec, f.task, "rev-parse", "--path-format=absolute", "--git-path", "rebase-merge")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(rebaseState, "state"), "owned test state\n")
	if conflicted, err := ownedRebaseConflict(context.Background(), gitExec, f.task); err != nil || conflicted {
		t.Fatalf("non-conflicting rebase state = %t, %v", conflicted, err)
	}
	if err := os.Remove(filepath.Join(rebaseState, "state")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(rebaseState); err != nil {
		t.Fatal(err)
	}
	if _, err := oid(context.Background(), gitExec, f.task, "not-a-ref"); err == nil {
		t.Fatal("unknown revision accepted")
	}
	if _, err := output(context.Background(), gitExec, f.task, "rev-parse", "--verify", "not-a-ref"); err == nil {
		t.Fatal("failed inspection accepted")
	}
	if err := safeWorktreePath(""); err == nil {
		t.Fatal("empty path accepted")
	}
	runGit(t, f.task, "config", "--local", "extensions.worktreeConfig", "true")
	if err := rejectUnsupported(context.Background(), gitExec, f.request()); err == nil {
		t.Fatal("worktree config extension accepted")
	}
	runGit(t, f.task, "config", "--unset", "extensions.worktreeConfig")
	marker := filepath.Join(f.common, "info", "grafts")
	write(t, marker, "# unsafe\n")
	if err := rejectUnsupported(context.Background(), gitExec, f.request()); err == nil {
		t.Fatal("grafts metadata accepted")
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runResult(canceled, gitExec, f.task, "status", "--porcelain"); err == nil {
		t.Fatal("canceled Git inspection accepted")
	}
	for name, mutate := range map[string]func(*Request){
		"common":  func(r *Request) { r.CommonDir = filepath.Join(r.CommonDir, "wrong") },
		"primary": func(r *Request) { r.CanonicalWorktree = r.TaskWorktree },
		"branch":  func(r *Request) { r.TaskBranch = "other" },
		"head":    func(r *Request) { r.ExpectedTaskHEAD = strings.Repeat("0", 40) },
		"source":  func(r *Request) { r.SourceOID = strings.Repeat("0", 40) },
	} {
		t.Run(name, func(t *testing.T) {
			r := f.request()
			mutate(&r)
			if _, err := inspect(context.Background(), gitExec, r); err == nil {
				t.Fatal("unsafe inspection state accepted")
			}
		})
	}
	t.Setenv("PATH", "")
	if _, err := newExecutor(); err == nil {
		t.Fatal("missing configured Git executable accepted")
	}
}

func TestPrepareCleanRejectsMergeHistoryAndInvalidRelease(t *testing.T) {
	f := newFixture(t, false)
	runGit(t, f.task, "checkout", "-b", "side")
	write(t, filepath.Join(f.task, "side.txt"), "side\n")
	runGit(t, f.task, "add", "side.txt")
	runGit(t, f.task, "commit", "-m", "side")
	runGit(t, f.task, "checkout", "task/demo")
	runGit(t, f.task, "merge", "--no-ff", "side", "-m", "merge side")
	f.taskHead = gitOutput(t, f.task, "rev-parse", "HEAD")
	if got := PrepareClean(context.Background(), f.request()); got.Outcome != OutcomeRefused {
		t.Fatalf("merge history prepare = %+v, want refused", got)
	}
	if err := Release(context.Background(), f.canonical, PreparedCandidate{}); err == nil {
		t.Fatal("invalid release candidate accepted")
	}
	if validOID("not-an-oid") || !validOID(strings.Repeat("a", 40)) || validOID(strings.Repeat("g", 40)) {
		t.Fatal("object identity validation accepted invalid input")
	}
}

func configure(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "config", "user.name", "test")
	runGit(t, dir, "config", "user.email", "test@example.invalid")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- fixed test argv
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- fixed test argv
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output))
}

func write(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func worktreeCount(t *testing.T, dir string) int {
	t.Helper()
	count := 0
	for _, line := range strings.Split(gitOutput(t, dir, "worktree", "list", "--porcelain"), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			count++
		}
	}
	return count
}
