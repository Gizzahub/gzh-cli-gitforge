// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
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

func TestNormalReclaimSymbolicRefPreservesReferent(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	runGit(t, f.fx.Clone, "worktree", "remove", f.fx.Worktree)
	protected := "refs/heads/develop"
	runGit(t, f.fx.Clone, "update-ref", protected, f.taskSHA)
	t.Setenv("TASK302_TEST_REF", "refs/heads/"+f.taskBranch)
	t.Setenv("TASK302_TEST_REFERENT", protected)
	f.interceptGit(t, `if [[ "$1" == "update-ref" && "${2:-}" == "-d" ]]; then
  "$TASK302_TEST_GIT" symbolic-ref "$TASK302_TEST_REF" "$TASK302_TEST_REFERENT"
fi`)
	var result ReclaimResult
	deleteLocalTaskBranch(context.Background(), newGitRepo(f.exec, f.fx.Clone), f.taskBranch, f.taskSHA, &result)
	got, present, err := newGitRepo(gitcmd.NewExecutor(), f.fx.Clone).revParse(context.Background(), protected)
	if err != nil || !present || got != f.taskSHA {
		t.Fatalf("symbolic normal deletion damaged protected referent: oid=%s present=%t err=%v", got, present, err)
	}
}

func TestRunGitReclaimDistinctPushEndpoint(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	actual := gitOutput(t, f.fx.Clone, "remote", "get-url", f.fx.Remote)
	mirror := filepath.Join(t.TempDir(), "fetch.git")
	runGit(t, f.fx.Clone, "clone", "--bare", actual, mirror)
	runGit(t, f.fx.Clone, "remote", "set-url", "--push", f.fx.Remote, actual)
	runGit(t, f.fx.Clone, "remote", "set-url", f.fx.Remote, mirror)
	report := f.realRun(t)
	if report.Reclaim.Incomplete() || report.Reclaim.GitProof == nil || !report.Reclaim.GitProof.After.RemoteBranchAbsent {
		t.Fatalf("proof followed fetch endpoint instead of actual push: %+v", report.Reclaim)
	}
	if gitOutput(t, f.fx.Clone, "ls-remote", actual, "refs/heads/"+f.taskBranch) != "" {
		t.Fatal("actual push task branch remains")
	}
	if gitOutput(t, f.fx.Clone, "ls-remote", mirror, "refs/heads/"+f.taskBranch) == "" {
		t.Fatal("caller fetch mirror was modified")
	}
}

func TestRunGitReclaimSymlinkParentEndpoint(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	writeRepoFile(t, f.fx.Worktree, ".gitignore", "link\nremote.git/\n")
	runGit(t, f.fx.Worktree, "add", ".gitignore")
	runGit(t, f.fx.Worktree, "commit", "-m", "ignore endpoint fixture resources")
	runGit(t, f.fx.Worktree, "push", f.fx.Remote, f.taskBranch)
	f.taskSHA = gitOutput(t, f.fx.Worktree, "rev-parse", "HEAD")
	f.execution.ExpectSource = f.taskSHA
	origin := gitOutput(t, f.fx.Clone, "remote", "get-url", f.fx.Remote)
	parent := t.TempDir()
	actual := filepath.Join(parent, "remote.git")
	decoy := filepath.Join(f.fx.Worktree, "remote.git")
	subdir := filepath.Join(parent, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, f.fx.Clone, "clone", "--bare", origin, actual)
	runGit(t, f.fx.Clone, "clone", "--bare", origin, decoy)
	before := gitOutput(t, actual, "rev-parse", "refs/heads/develop")
	if err := os.Symlink(subdir, filepath.Join(f.fx.Worktree, "link")); err != nil {
		t.Fatal(err)
	}
	runGit(t, f.fx.Clone, "remote", "set-url", "--push", f.fx.Remote, "./link/../remote.git")
	report := f.realRun(t)
	if got := gitOutput(t, actual, "rev-parse", "refs/heads/develop"); got != f.taskSHA {
		t.Fatalf("configured physical target was not pushed: %s want %s", got, f.taskSHA)
	}
	if got := gitOutput(t, decoy, "rev-parse", "refs/heads/develop"); got != before {
		t.Fatalf("lexical decoy target was pushed: %s -> %s", before, got)
	}
	if report.Receipt.PushRemote != resolvePath(t, actual) {
		t.Fatalf("receipt bound the wrong symlink-parent endpoint: %s", report.Receipt.PushRemote)
	}
}

func TestRunGitReclaimRelativePushEndpoint(t *testing.T) {
	for _, retry := range []bool{false, true} {
		for _, spelling := range []string{"remote.git", "./store@host:repo.git"} {
			name := "normal"
			if retry {
				name = "retry"
			}
			name += "/" + strings.ReplaceAll(spelling, "/", "_")
			t.Run(name, func(t *testing.T) {
				f := newCleanupRetryFX(t, "@true")
				origin := gitOutput(t, f.fx.Clone, "remote", "get-url", f.fx.Remote)
				sourceParent := t.TempDir()
				actual := filepath.Join(sourceParent, "actual.git")
				aliasName := strings.TrimPrefix(spelling, "./")
				decoy := filepath.Join(f.fx.Clone, aliasName)
				moved := filepath.Join(sourceParent, "taskwt")
				runGit(t, f.fx.Clone, "worktree", "move", f.fx.Worktree, moved)
				f.fx.Worktree = moved
				writeRepoFile(t, moved, ".gitignore", aliasName+"\n")
				runGit(t, moved, "add", ".gitignore")
				runGit(t, moved, "commit", "-m", "ignore relative endpoint alias")
				runGit(t, moved, "push", f.fx.Remote, f.taskBranch)
				f.taskSHA = gitOutput(t, moved, "rev-parse", "HEAD")
				f.execution.ExpectSource = f.taskSHA
				runGit(t, f.fx.Clone, "clone", "--bare", origin, actual)
				runGit(t, f.fx.Clone, "clone", "--bare", origin, decoy)
				runGit(t, decoy, "update-ref", "refs/heads/develop", f.taskSHA)
				if err := os.Symlink(actual, filepath.Join(moved, aliasName)); err != nil {
					t.Fatal(err)
				}
				runGit(t, f.fx.Clone, "remote", "set-url", "--push", f.fx.Remote, spelling)
				if retry {
					f.interceptGit(t, `if [[ "$1" == "update-ref" && "${2:-}" == "-d" ]]; then
  printf '%s\n' 'injected local delete interruption' >&2
  exit 42
fi`)
				}
				report := f.realRun(t)
				if retry {
					if !report.Reclaim.Incomplete() {
						t.Fatal("retry fixture did not interrupt normal cleanup")
					}
					f.interceptGit(t, "")
					f.amendReceiptRaw(t, func(r *Receipt) { r.Claim = deadHolderClaim(t) })
					if cleanup, err := f.cleanup(t, "g7"); err != nil || !cleanup.Complete {
						t.Fatalf("different-directory retry failed: %+v %v", cleanup, err)
					}
				} else if report.Reclaim.Incomplete() {
					t.Fatalf("relative endpoint normal cleanup failed: %+v", report.Reclaim)
				}
				if gitOutput(t, f.fx.Clone, "ls-remote", actual, "refs/heads/"+f.taskBranch) != "" {
					t.Fatal("actual pushed repository retained its task branch")
				}
				if gitOutput(t, f.fx.Clone, "ls-remote", decoy, "refs/heads/"+f.taskBranch) == "" {
					t.Fatal("matching caller repository was deleted through relative reinterpretation")
				}
				if !filepath.IsAbs(report.Receipt.PushRemote) || report.Receipt.PushRemote != resolvePath(t, actual) {
					t.Fatalf("receipt did not bind physical push address: %s", report.Receipt.PushRemote)
				}
			})
		}
	}
}

func TestRunGitReclaimAmbiguousPushEndpoints(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	actual := gitOutput(t, f.fx.Clone, "remote", "get-url", f.fx.Remote)
	mirror := filepath.Join(t.TempDir(), "other-push.git")
	runGit(t, f.fx.Clone, "clone", "--bare", actual, mirror)
	runGit(t, f.fx.Clone, "remote", "set-url", "--add", "--push", f.fx.Remote, actual)
	runGit(t, f.fx.Clone, "remote", "set-url", "--add", "--push", f.fx.Remote, mirror)
	before := f.remoteTargetTip(t)
	_, err := Run(context.Background(), f.exec, RunOptions{
		CheckOptions: CheckOptions{RepoPath: f.fx.Worktree, Target: f.target},
		Execution:    &f.execution,
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous push endpoint") {
		t.Fatalf("ambiguous push endpoints were not refused for their cause: %v", err)
	}
	if after := f.remoteTargetTip(t); after != before {
		t.Fatalf("ambiguous destination was pushed: %s -> %s", before, after)
	}
	if _, err := os.Stat(filepath.Join(f.fx.Worktree, ".git")); err != nil {
		t.Fatalf("task worktree was removed: %v", err)
	}
	if got := gitOutput(t, f.fx.Clone, "rev-parse", "refs/heads/"+f.taskBranch); got != f.taskSHA {
		t.Fatalf("task local ref changed: %s", got)
	}
	if gitOutput(t, f.fx.Clone, "ls-remote", actual, "refs/heads/"+f.taskBranch) == "" {
		t.Fatal("task remote ref was removed")
	}
	if _, ok, err := readReceipt(f.common, f.execution.ExecutionID); err != nil || ok {
		t.Fatalf("ambiguous destination created an integration intent: exists=%t err=%v", ok, err)
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
  if ! "$TASK302_TEST_GIT" show-ref --verify --quiet "$TASK302_TEST_REF"; then
    printf '%s\n' 'injected post-delete query failure' >&2
    exit 42
  fi
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

func TestRunGitReclaimPreservesBareRepo(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	writeRepoFile(t, f.fx.Worktree, ".gitignore", "nested/\n")
	runGit(t, f.fx.Worktree, "add", ".gitignore")
	runGit(t, f.fx.Worktree, "commit", "-m", "ignore bare fixture")
	runGit(t, f.fx.Worktree, "push", f.fx.Remote, f.taskBranch)
	f.taskSHA = gitOutput(t, f.fx.Worktree, "rev-parse", "HEAD")
	f.execution.ExpectSource = f.taskSHA
	nested := filepath.Join(f.fx.Worktree, "nested")
	runGit(t, f.fx.Worktree, "init", "--bare", nested)
	report := f.realRun(t)
	if !report.Reclaim.Incomplete() || report.Reclaim.GitProof != nil {
		t.Fatalf("bare repository got a false task-only proof: %+v", report.Reclaim)
	}
	if _, err := os.Stat(filepath.Join(nested, "HEAD")); err != nil {
		t.Fatalf("caller-owned bare repository was deleted: %v", err)
	}
	f.assertResidualsIntact(t, resolvePath(t, f.fx.Worktree))
}

func TestRunGitReclaimWithoutTrackingRef(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	t.Setenv("TASK302_TEST_TRACKING", "refs/remotes/"+f.fx.Remote+"/"+f.taskBranch)
	f.interceptGit(t, `if [[ "$1" == "worktree" && "${2:-}" == "remove" ]]; then
  "$TASK302_TEST_GIT" update-ref -d "$TASK302_TEST_TRACKING"
fi`)
	report := f.realRun(t)
	if report.Reclaim.Incomplete() || report.Reclaim.GitProof == nil || !report.Reclaim.GitProof.After.RemoteBranchAbsent {
		t.Fatalf("live remote proof still depended on tracking ref: %+v", report.Reclaim)
	}
	if gitOutput(t, f.fx.Clone, "ls-remote", f.fx.Remote, "refs/heads/"+f.taskBranch) != "" {
		t.Fatal("proved remote task ref was not reclaimed")
	}
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
