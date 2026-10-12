// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

// cleanupRetryFX is a real bare-remote fixture for cleanup-only retry: a
// bare origin, a clone, a linked task-branch worktree, a durable receipt,
// and — for crash simulations — reconstructed residuals at the captured SHA.
type cleanupRetryFX struct {
	fx         *testutil.WorktreeOrigin
	exec       *gitcmd.Executor
	taskBranch string
	taskSHA    string
	target     string
	common     string
	execution  ExecutionContext
}

// newCleanupRetryFX builds the fixture and runs a real evidenced integrate
// (unless skipRun), leaving either a complete receipt or the caller's chosen
// crash state.
func newCleanupRetryFX(t *testing.T, checkRecipe string) *cleanupRetryFX {
	t.Helper()
	t.Setenv("GZ_GIT_TASK_ACTOR", "claude")
	t.Setenv("GIT_WORK_HOST", "testhost")
	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeRepoFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern:\n    - dev/*\n")
	writeRepoFile(t, fx.Clone, "Makefile", "check:\n\t"+checkRecipe+"\n")
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "declare policy")
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/main")
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/develop")
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", "dev/claude/x1", fx.Remote+"/develop")
	writeRepoFile(t, fx.Worktree, "task.txt", "work\n")
	runGit(t, fx.Worktree, "add", "task.txt")
	runGit(t, fx.Worktree, "commit", "-m", "task work")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "dev/claude/x1")
	f := &cleanupRetryFX{
		fx:         fx,
		exec:       gitcmd.NewExecutor(),
		taskBranch: "dev/claude/x1",
		target:     fx.Remote + "/develop",
		common:     gitOutput(t, fx.Clone, "rev-parse", "--path-format=absolute", "--git-common-dir"),
	}
	f.taskSHA = gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	f.execution = ExecutionContext{
		ExecutionID:  "run-42",
		Generation:   "g7",
		Owner:        OwnerIdentity{Actor: "claude", Host: "testhost", Kind: "agent"},
		ExpectSource: f.taskSHA,
	}
	return f
}

// realRun performs the real evidenced integrate and requires success.
func (f *cleanupRetryFX) realRun(t *testing.T) *RunReport {
	t.Helper()
	execution := f.execution
	report, err := Run(context.Background(), f.exec, RunOptions{
		CheckOptions: CheckOptions{RepoPath: f.fx.Worktree, Target: f.target},
		Execution:    &execution,
	})
	if err != nil {
		t.Fatalf("evidenced Run: %v\n%s", err, FormatRun(report))
	}
	if report.Receipt == nil || !report.Integrated {
		t.Fatalf("run left no receipt or did not integrate: %+v", report)
	}
	return report
}

// cleanup calls CleanupRetry as the CLI would, for the fixture's own
// execution id and the given generation.
func (f *cleanupRetryFX) cleanup(t *testing.T, generation string) (*CleanupReport, error) {
	t.Helper()
	return CleanupRetry(context.Background(), f.exec, CleanupOptions{
		RepoPath:    f.fx.Clone,
		ExecutionID: f.execution.ExecutionID,
		Generation:  generation,
	})
}

type crashOpts struct {
	phase     string
	noMarker  bool
	markerGen string
}

// crashState reconstructs the residuals an interrupted run left behind: the
// task branch and remote branch at the captured SHA, a clean worktree
// holding it, the registration marker, and a receipt at the chosen phase
// with all removal flags cleared.
// resolvePath returns Git's canonical (symlink-resolved) form of a path — on
// macOS Git lists worktrees under /private/var while t.TempDir returns /var,
// and receipt identity comparisons are exact.
func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve %s: %v", path, err)
	}
	return resolved
}

func (f *cleanupRetryFX) crashState(t *testing.T, opts crashOpts) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "taskwt")
	runGit(t, f.fx.Clone, "branch", f.taskBranch, f.taskSHA)
	// -u restores the upstream a real run's branch would still carry, so the
	// reconstructed branch is deletable by `git branch -d` like the original.
	runGit(t, f.fx.Clone, "push", "-u", f.fx.Remote, f.taskBranch+":refs/heads/"+f.taskBranch)
	runGit(t, f.fx.Clone, "fetch", f.fx.Remote)
	runGit(t, f.fx.Clone, "worktree", "add", wt, f.taskBranch)
	wt = resolvePath(t, wt)
	gen := opts.markerGen
	if gen == "" {
		gen = f.execution.Generation
	}
	if !opts.noMarker {
		f.markWorktree(t, wt, gen)
	}
	// The reconstructed holder must be positively dead: cleanup refuses a
	// live holder with no same-PID exemption.
	f.amendReceiptRaw(t, func(r *Receipt) {
		r.Phase = opts.phase
		r.SourcePushed = opts.phase != evidencePhaseIntent
		r.WorktreeRemoved = false
		r.LocalBranchRemoved = false
		r.RemoteBranchRemoved = false
		r.TaskWorktree = wt
		r.Claim = deadHolderClaim(t)
	})
	return wt
}

// deadHolderClaim returns the claim of a holder that has positively exited:
// kill(pid, 0) reports ESRCH, the only answer cleanup treats as gone.
func deadHolderClaim(t *testing.T) evidenceClaim {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn short-lived holder: %v", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("reap short-lived holder: %v", err)
	}
	return evidenceClaim{PID: pid, ProcessHost: physicalHost()}
}

// markWorktree writes the registration marker binding a worktree to an
// execution and generation.
func (f *cleanupRetryFX) markWorktree(t *testing.T, worktree, generation string) {
	t.Helper()
	if err := writeRegistrationMarker(context.Background(), f.exec, worktree, registrationMarker{
		SchemaVersion: evidenceSchemaVersion,
		ExecutionID:   f.execution.ExecutionID,
		Generation:    generation,
		TaskSHA:       f.taskSHA,
	}); err != nil {
		t.Fatalf("write registration marker: %v", err)
	}
}

// seedReceipt writes the first receipt without a real run, for fixtures that
// must not execute any gate.
func (f *cleanupRetryFX) seedReceipt(t *testing.T, phase, worktree string) {
	t.Helper()
	base := gitOutput(t, f.fx.Clone, "rev-parse", f.taskSHA+"^")
	r := &Receipt{
		SchemaVersion:       evidenceSchemaVersion,
		ExecutionID:         f.execution.ExecutionID,
		Generation:          f.execution.Generation,
		Owner:               f.execution.Owner,
		RepositoryCommonDir: f.common,
		TaskBranch:          f.taskBranch,
		TaskWorktree:        worktree,
		TaskSHA:             f.taskSHA,
		TargetRef:           "refs/heads/develop",
		TargetBeforeSHA:     base,
		Remote:              f.fx.Remote,
		SourcePushed:        phase != evidencePhaseIntent,
		Phase:               phase,
		Provenance:          &evidenceProvenance{DeclarationPatterns: []string{"dev/*"}},
	}
	if err := createReceipt(context.Background(), f.common, r); err != nil {
		t.Fatalf("seed receipt: %v", err)
	}
	// createReceipt stamps the current process as holder; replace it with a
	// positively dead one so cleanup's claim check can pass.
	f.amendReceiptRaw(t, func(r *Receipt) { r.Claim = deadHolderClaim(t) })
}

// amendReceiptRaw mutates the receipt without refreshClaim — the only way to
// place a foreign holder's claim, which writeReceipt would overwrite with the
// current process.
func (f *cleanupRetryFX) amendReceiptRaw(t *testing.T, mutate func(*Receipt)) {
	t.Helper()
	r := f.receipt(t)
	mutate(r)
	if err := atomicStateWrite(evidenceReceiptPath(f.common, r.ExecutionID), mustMarshalJSON(r)); err != nil {
		t.Fatalf("raw write receipt: %v", err)
	}
}

// receipt returns the current durable receipt.
func (f *cleanupRetryFX) receipt(t *testing.T) *Receipt {
	t.Helper()
	r, ok, err := readReceipt(f.common, f.execution.ExecutionID)
	if err != nil || !ok {
		t.Fatalf("read receipt: ok=%v err=%v", ok, err)
	}
	return r
}

// assertResidualsIntact requires every residual to still exist exactly at
// the captured SHA, with no removal flag set.
func (f *cleanupRetryFX) assertResidualsIntact(t *testing.T, wt string) {
	t.Helper()
	if got := gitOutput(t, f.fx.Clone, "rev-parse", "--verify", "refs/heads/"+f.taskBranch); got != f.taskSHA {
		t.Fatalf("local task branch = %q, want %q", got, f.taskSHA)
	}
	if remote := gitOutput(t, f.fx.Clone, "ls-remote", f.fx.Remote, "refs/heads/"+f.taskBranch); remote == "" {
		t.Fatalf("remote task branch was removed")
	}
	if _, err := exec.CommandContext(context.Background(), "git", "-C", f.fx.Clone, "worktree", "list", "--porcelain").Output(); err != nil {
		t.Fatalf("list worktrees: %v", err)
	}
	if wt != "" {
		listed := gitOutput(t, f.fx.Clone, "worktree", "list", "--porcelain")
		if !containsLine(listed, "worktree "+wt) {
			t.Fatalf("task worktree %s was removed:\n%s", wt, listed)
		}
	}
	if r := f.receipt(t); r.WorktreeRemoved || r.LocalBranchRemoved || r.RemoteBranchRemoved {
		t.Fatalf("receipt records removals despite refusal: %+v", r)
	}
}

// assertResidualsGone requires every residual to be absent.
func (f *cleanupRetryFX) assertResidualsGone(t *testing.T, wt string) {
	t.Helper()
	// rev-parse --verify --quiet exits 1 for an absent branch, which gitOutput
	// treats as fatal; probe it softly instead.
	if out, err := exec.CommandContext(context.Background(), "git", "-C", f.fx.Clone, "rev-parse", "--verify", "--quiet", "refs/heads/"+f.taskBranch).Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		t.Fatalf("local task branch still exists: %s", strings.TrimSpace(string(out)))
	}
	if remote := gitOutput(t, f.fx.Clone, "ls-remote", f.fx.Remote, "refs/heads/"+f.taskBranch); remote != "" {
		t.Fatalf("remote task branch still exists: %s", remote)
	}
	if wt != "" {
		listed := gitOutput(t, f.fx.Clone, "worktree", "list", "--porcelain")
		if containsLine(listed, "worktree "+wt) {
			t.Fatalf("task worktree %s still listed:\n%s", wt, listed)
		}
	}
}

func containsLine(block, want string) bool {
	return slices.Contains(splitNonEmpty(block), want)
}

// remoteTargetTip reads the live remote target tip SHA.
func (f *cleanupRetryFX) remoteTargetTip(t *testing.T) string {
	t.Helper()
	line := gitOutput(t, f.fx.Clone, "ls-remote", f.fx.Remote, "refs/heads/develop")
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// spawnSleeper starts a live process and returns its pid, dying with the test.
func spawnSleeper(t *testing.T) int {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn sleeper: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd.Process.Pid
}
