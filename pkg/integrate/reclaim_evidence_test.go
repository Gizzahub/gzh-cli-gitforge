// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunGitReclaimProof(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	wt := resolvePath(t, f.fx.Worktree)
	parent := filepath.Dir(wt)
	report := f.realRun(t)
	p := report.Reclaim.GitProof
	if p == nil || !p.Proofs.complete() || p.Action.ExitCode != 0 {
		t.Fatalf("missing complete Git reclaim proof: %+v", p)
	}
	if p.TaskOID != f.taskSHA || p.TaskBranch != f.taskBranch || p.TaskWorktree != wt || p.RepositoryCommonDir != f.common {
		t.Fatalf("wrong source/resource identity: %+v", p)
	}
	if p.ObservedTarget.Ref != "refs/heads/develop" || p.ObservedTarget.OID != f.taskSHA {
		t.Fatalf("wrong actual post-push target: %+v", p.ObservedTarget)
	}
	if p.ResourceBefore.LocalOID != f.taskSHA || p.ResourceBefore.RemoteOID != f.taskSHA || p.ResourceBefore.LocalRef != "refs/heads/"+f.taskBranch || p.ResourceBefore.RemoteRef != p.ResourceBefore.LocalRef {
		t.Fatalf("wrong pre-delete ref inventory: %+v", p.ResourceBefore)
	}
	if !p.After.WorktreeAbsent || !p.After.LocalBranchAbsent || !p.After.RemoteBranchAbsent {
		t.Fatalf("absence not proven: %+v", p.After)
	}
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		t.Fatalf("caller-owned parent was removed: %v", err)
	}
	data, err := json.Marshal(BuildRunProtocol(report, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		Command      string `json:"command"`
		SourcePushed bool   `json:"source_pushed"`
		ExitCode     int    `json:"exit_code"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil || legacy.Command != "integrate-run" || !legacy.SourcePushed || legacy.ExitCode != 0 {
		t.Fatalf("additive proof broke existing context decoder: %+v %v", legacy, err)
	}
	proofJSON, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(proofJSON, &doc); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"owner", "generation", "execution_id"} {
		if _, ok := doc[forbidden]; ok {
			t.Fatalf("Git proof became an authorization ledger: %s", forbidden)
		}
	}
}

func TestRunGitReclaimNoFetchOmitsProof(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	execution := f.execution
	report, err := Run(context.Background(), f.exec, RunOptions{
		CheckOptions: CheckOptions{RepoPath: f.fx.Worktree, Target: f.target, NoFetch: true}, Execution: &execution,
	})
	if err != nil || report.Reclaim.Incomplete() || report.Reclaim.GitProof != nil {
		t.Fatalf("no-fetch must preserve existing behavior without fabricated live proof: %+v %v", report, err)
	}
	data, err := json.Marshal(BuildRunProtocol(report, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["git_reclaim"]; ok {
		t.Fatal("no-fetch emitted an empty live Git proof")
	}
}

func TestNormalReclaimLocalExpectedOld(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	runGit(t, f.fx.Clone, "worktree", "remove", f.fx.Worktree)
	moved := gitOutput(t, f.fx.Clone, "rev-parse", f.taskSHA+"^")
	t.Setenv("TASK302_TEST_REF", "refs/heads/"+f.taskBranch)
	t.Setenv("TASK302_TEST_MOVED", moved)
	f.interceptGit(t, `if [[ "$1" == "update-ref" && "${2:-}" == "-d" ]] || [[ "$1" == "branch" && "${2:-}" == "-d" ]]; then
  "$TASK302_TEST_GIT" update-ref "$TASK302_TEST_REF" "$TASK302_TEST_MOVED"
fi`)
	var result ReclaimResult
	if deleteLocalTaskBranch(context.Background(), newGitRepo(f.exec, f.fx.Clone), f.taskBranch, f.taskSHA, &result) || result.RemovedLocalBranch {
		t.Fatal("normal reclaim deleted a ref moved after validation")
	}
	if got := gitOutput(t, f.fx.Clone, "rev-parse", "refs/heads/"+f.taskBranch); got != moved {
		t.Fatalf("normal reclaim lost moved ref: %s", got)
	}
}

func TestRunGitReclaimAfterQueryFailure(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	t.Setenv("TASK302_TEST_REF", "refs/heads/"+f.taskBranch)
	f.interceptGit(t, `if [[ "$1" == "for-each-ref" && "${!#}" == "$TASK302_TEST_REF" ]]; then
  printf '%s\n' 'injected post-delete query failure' >&2
  exit 42
fi`)
	report := f.realRun(t)
	p := report.Reclaim.GitProof
	if p == nil || p.After.LocalBranchAbsent || p.Action.ExitCode != 3 || !report.Reclaim.Incomplete() || report.Receipt.LocalBranchRemoved {
		t.Fatalf("query failure fabricated complete absence: proof=%+v report=%+v", p, report)
	}
}

func TestRunGitReclaimPreservesNestedRepo(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	writeRepoFile(t, f.fx.Worktree, ".gitignore", "nested/\n")
	runGit(t, f.fx.Worktree, "add", ".gitignore")
	runGit(t, f.fx.Worktree, "commit", "-m", "ignore nested fixture")
	runGit(t, f.fx.Worktree, "push", f.fx.Remote, f.taskBranch)
	f.taskSHA = gitOutput(t, f.fx.Worktree, "rev-parse", "HEAD")
	f.execution.ExpectSource = f.taskSHA
	nested := filepath.Join(f.fx.Worktree, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, nested, "init")
	report := f.realRun(t)
	if !report.Reclaim.Incomplete() || report.Reclaim.GitProof != nil {
		t.Fatalf("nested resource got a false task-only proof: %+v", report.Reclaim)
	}
	if _, err := os.Stat(filepath.Join(nested, ".git")); err != nil {
		t.Fatalf("caller-owned nested repository was deleted: %v", err)
	}
	f.assertResidualsIntact(t, resolvePath(t, f.fx.Worktree))
}

func TestRunGitReclaimPreservesSiblingLease(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	wt := resolvePath(t, f.fx.Worktree)
	dir := filepath.Join(f.common, "sibling-provision-leases")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(SiblingLease{Root: wt, ID: "caller-owned"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "caller-owned.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	report := f.realRun(t)
	if !report.Reclaim.Incomplete() || report.Reclaim.GitProof != nil {
		t.Fatalf("caller-owned lease got a false task-only proof: %+v", report.Reclaim)
	}
	f.assertResidualsIntact(t, wt)
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("caller-owned lease was changed: %v", err)
	}
}
