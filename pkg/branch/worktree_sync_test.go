// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package branch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

type syncFixture struct {
	main, develop, feature, remote, writer string
}

func syncGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	result, err := runGit(context.Background(), gitcmd.NewExecutor(), dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(result.Stdout)
}

func syncWrite(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func syncCommit(t *testing.T, dir, name, content string) string {
	t.Helper()
	syncWrite(t, dir, name, content)
	syncGit(t, dir, "add", "--", name)
	syncGit(t, dir, "commit", "-m", name)
	return syncGit(t, dir, "rev-parse", "HEAD")
}

func newSyncFixture(t *testing.T) syncFixture {
	t.Helper()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "Sync Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "sync@example.test")
	t.Setenv("GIT_COMMITTER_NAME", "Sync Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "sync@example.test")
	root := t.TempDir()
	f := syncFixture{main: filepath.Join(root, "main"), develop: filepath.Join(root, "develop"), feature: filepath.Join(root, "feature"), remote: filepath.Join(root, "remote.git"), writer: filepath.Join(root, "writer")}
	syncGit(t, root, "init", "--bare", "--initial-branch=master", f.remote)
	syncGit(t, root, "init", "--initial-branch=master", f.main)
	syncCommit(t, f.main, "base.txt", "base\n")
	syncGit(t, f.main, "remote", "add", "origin", f.remote)
	syncGit(t, f.main, "push", "-u", "origin", "master")
	syncGit(t, f.main, "worktree", "add", "-b", "develop", f.develop, "master")
	syncGit(t, f.develop, "push", "-u", "origin", "develop")
	syncGit(t, f.main, "worktree", "add", "-b", "feature/api", f.feature, "develop")
	syncGit(t, root, "clone", f.remote, f.writer)
	return f
}

func (f syncFixture) config() *WorktreeSyncConfig {
	return &WorktreeSyncConfig{Rules: []WorktreeSyncRule{{Match: "develop", Parent: "master", Strategy: "ff-only"}, {Match: "feature/*", Parent: "develop", Strategy: "rebase"}}}
}

func syncResult(t *testing.T, plan *WorktreeSyncPlan, branch string) *WorktreeSyncResult {
	t.Helper()
	for _, result := range plan.Results {
		if result.Branch == branch {
			return result
		}
	}
	t.Fatalf("no result for %s: %+v", branch, plan.Results)
	return nil
}

func TestWorktreeSyncFetchFFAndRebase(t *testing.T) {
	f := newSyncFixture(t)
	oldFeature := syncCommit(t, f.feature, "feature.txt", "own work\n")
	newMaster := syncCommit(t, f.writer, "remote.txt", "remote work\n")
	syncGit(t, f.writer, "push", "origin", "master")
	// Rebase config must not rewrite a second branch pointing at the feature.
	syncGit(t, f.feature, "branch", "saved-feature", oldFeature)
	syncGit(t, f.feature, "config", "rebase.updateRefs", "true")
	plan, err := PlanWorktreeSync(context.Background(), f.feature, WorktreeSyncOptions{Config: f.config()})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Fetched {
		t.Fatal("expected fetch")
	}
	if err := ExecuteWorktreeSync(context.Background(), plan); err != nil {
		t.Fatalf("%v: %+v", err, plan.Results)
	}
	for _, dir := range []string{f.main, f.develop} {
		if got := syncGit(t, dir, "rev-parse", "HEAD"); got != newMaster {
			t.Fatalf("%s: %s != %s", dir, got, newMaster)
		}
	}
	if got := syncGit(t, f.feature, "rev-parse", "HEAD"); got == oldFeature {
		t.Fatal("feature was not rebased")
	}
	if got := syncGit(t, f.feature, "rev-list", "--count", "develop..HEAD"); got != "1" {
		t.Fatalf("feature commits not preserved: %s", got)
	}
	if got := syncGit(t, f.feature, "rev-parse", "saved-feature"); got != oldFeature {
		t.Fatal("unselected branch rewritten by rebase.updateRefs")
	}
	if got := syncResult(t, plan, "feature/api"); got.Status != "done" || got.Steps[0].Action != "rebase" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestWorktreeSyncDryRunDoesNotFetchOrMutate(t *testing.T) {
	f := newSyncFixture(t)
	before := syncGit(t, f.main, "rev-parse", "origin/master")
	syncCommit(t, f.writer, "remote.txt", "new remote\n")
	syncGit(t, f.writer, "push", "origin", "master")
	plan, err := PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: f.config(), DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := ExecuteWorktreeSync(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if plan.Fetched || syncGit(t, f.main, "rev-parse", "origin/master") != before || syncGit(t, f.main, "rev-parse", "HEAD") != before {
		t.Fatal("dry-run changed refs")
	}
	if syncGit(t, f.main, "status", "--porcelain") != "" {
		t.Fatal("dry-run changed files")
	}
}

func TestWorktreeSyncDirtyAndDependencyFilter(t *testing.T) {
	f := newSyncFixture(t)
	syncWrite(t, f.develop, "dirty.txt", "uncommitted\n")
	plan, err := PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: f.config(), Include: "feature/*"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ExecuteWorktreeSync(context.Background(), plan); err == nil {
		t.Fatal("dirty parent accepted")
	}
	if !syncResult(t, plan, "develop").Implicit || !syncResult(t, plan, "master").Implicit {
		t.Fatal("required ancestors were not included")
	}
	if got := syncResult(t, plan, "feature/api"); got.Status != "blocked" || !strings.Contains(got.Message, "parent") {
		t.Fatalf("dependent not blocked: %+v", got)
	}
	if got := syncResult(t, plan, "master").Status; got != "done" {
		t.Fatalf("independent work blocked: %s", got)
	}
	plan, err = PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: f.config(), Include: "feature/*", Exclude: "develop", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Results) != 1 || !strings.Contains(plan.Results[0].Message, "no included worktree") {
		t.Fatalf("excluded parent was included: %+v", plan.Results)
	}
}

func TestWorktreeSyncUpstreamDivergenceBlocksDescendants(t *testing.T) {
	f := newSyncFixture(t)
	local := syncCommit(t, f.main, "local.txt", "local\n")
	syncCommit(t, f.writer, "remote.txt", "remote\n")
	syncGit(t, f.writer, "push", "origin", "master")
	plan, err := PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: f.config()})
	if err != nil {
		t.Fatal(err)
	}
	if err := ExecuteWorktreeSync(context.Background(), plan); err == nil {
		t.Fatal("divergence accepted")
	}
	if syncGit(t, f.main, "rev-parse", "HEAD") != local {
		t.Fatal("shared branch rewritten")
	}
	for _, result := range plan.Results {
		if result.Status != "blocked" {
			t.Fatalf("dependent chain not blocked: %+v", result)
		}
	}
}

func TestWorktreeSyncConflictLeavesRebaseAndContinuesIndependent(t *testing.T) {
	f := newSyncFixture(t)
	syncCommit(t, f.feature, "base.txt", "feature version\n")
	syncGit(t, f.writer, "switch", "develop")
	syncCommit(t, f.writer, "base.txt", "develop version\n")
	syncGit(t, f.writer, "push", "origin", "develop")
	plan, err := PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: f.config()})
	if err != nil {
		t.Fatal(err)
	}
	if err := ExecuteWorktreeSync(context.Background(), plan); err == nil {
		t.Fatal("conflict accepted")
	}
	result := syncResult(t, plan, "feature/api")
	if result.Status != "blocked" || !strings.Contains(result.Message, f.feature) {
		t.Fatalf("conflict lacks recovery location: %+v", result)
	}
	if syncResult(t, plan, "master").Status != "done" {
		t.Fatal("independent master was not completed")
	}
	if syncGit(t, f.feature, "diff", "--name-only", "--diff-filter=U") != "base.txt" {
		t.Fatal("conflict not retained")
	}
}

func TestWorktreeSyncConcurrentHEADChange(t *testing.T) {
	f := newSyncFixture(t)
	syncCommit(t, f.writer, "remote.txt", "remote\n")
	syncGit(t, f.writer, "push", "origin", "master")
	plan, err := PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: f.config()})
	if err != nil {
		t.Fatal(err)
	}
	newLocal := syncCommit(t, f.main, "concurrent.txt", "concurrent work\n")
	if err := ExecuteWorktreeSync(context.Background(), plan); err == nil {
		t.Fatal("concurrent change accepted")
	}
	if !strings.Contains(syncResult(t, plan, "master").Message, "HEAD changed") || syncGit(t, f.main, "rev-parse", "HEAD") != newLocal {
		t.Fatal("concurrent work not preserved")
	}
}

func TestWorktreeSyncRejectsMultiLevelRebase(t *testing.T) {
	f := newSyncFixture(t)
	syncCommit(t, f.develop, "local-develop.txt", "develop work\n")
	syncCommit(t, f.writer, "remote.txt", "remote work\n")
	syncGit(t, f.writer, "push", "origin", "master")
	cfg := f.config()
	cfg.Rules[0].Strategy = "rebase"
	plan, err := PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(syncResult(t, plan, "develop").Message, "multi-level rebase") || syncResult(t, plan, "feature/api").Status != "blocked" {
		t.Fatal("multi-level rebase was not blocked")
	}
}

func TestWorktreeSyncInvalidGraphsAndRules(t *testing.T) {
	f := newSyncFixture(t)
	for _, tc := range []struct {
		name  string
		rules []WorktreeSyncRule
		want  string
	}{
		{"self", []WorktreeSyncRule{{"develop", "develop", "rebase"}}, "cycle"},
		{"cycle", []WorktreeSyncRule{{"develop", "master", "ff-only"}, {"master", "develop", "ff-only"}}, "cycle"},
		{"ambiguous", []WorktreeSyncRule{{"feature/*", "develop", "rebase"}, {"*/api", "master", "rebase"}}, "ambiguous"},
		{"duplicate", []WorktreeSyncRule{{"develop", "master", "ff-only"}, {"develop", "master", "rebase"}}, "duplicate"},
		{"invalid-pattern", []WorktreeSyncRule{{"[", "master", "rebase"}}, "invalid"},
		{"option-injection", []WorktreeSyncRule{{"develop", "--exec=bad", "rebase"}}, "invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: &WorktreeSyncConfig{Rules: tc.rules}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v; want %s", err, tc.want)
			}
		})
	}
}

func TestWorktreeSyncExactRuleOverridesGlob(t *testing.T) {
	cfg := &WorktreeSyncConfig{Rules: []WorktreeSyncRule{{"feature/*", "develop", "rebase"}, {"*/api", "master", "rebase"}, {"feature/api", "release", "ff-only"}}}
	rule, err := syncRule(cfg, "feature/api")
	if err != nil || rule == nil || rule.Parent != "release" {
		t.Fatalf("exact match lost precedence: %+v, %v", rule, err)
	}
}

func TestWorktreeSyncWorktreeGuards(t *testing.T) {
	for _, guard := range []string{"locked", "detached", "rebase", "missing-parent", "missing-upstream"} {
		t.Run(guard, func(t *testing.T) {
			f := newSyncFixture(t)
			cfg := f.config()
			switch guard {
			case "locked":
				syncGit(t, f.main, "worktree", "lock", f.feature)
			case "detached":
				syncGit(t, f.feature, "switch", "--detach")
			case "rebase":
				marker := syncGit(t, f.feature, "rev-parse", "--git-path", "rebase-merge")
				if !filepath.IsAbs(marker) {
					marker = filepath.Join(f.feature, marker)
				}
				if err := os.Mkdir(marker, 0o700); err != nil {
					t.Fatal(err)
				}
			case "missing-parent":
				cfg.Rules[1].Parent = "missing"
			case "missing-upstream":
				cfg.Rules = nil
			}
			plan, err := PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: cfg, DryRun: true})
			if err != nil {
				t.Fatal(err)
			}
			if err := ExecuteWorktreeSync(context.Background(), plan); err == nil {
				t.Fatalf("%s guard was bypassed", guard)
			}
		})
	}
}

func TestWorktreeSyncTrackingRefChangesAfterPlanning(t *testing.T) {
	f := newSyncFixture(t)
	syncCommit(t, f.writer, "remote.txt", "remote\n")
	syncGit(t, f.writer, "push", "origin", "master")
	plan, err := PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: f.config()})
	if err != nil {
		t.Fatal(err)
	}
	before := syncGit(t, f.main, "rev-parse", "HEAD")
	syncGit(t, f.main, "update-ref", "refs/remotes/origin/master", before)
	if err := ExecuteWorktreeSync(context.Background(), plan); err == nil {
		t.Fatal("moving source accepted")
	}
	if !strings.Contains(syncResult(t, plan, "master").Message, "source changed") || syncGit(t, f.main, "rev-parse", "HEAD") != before {
		t.Fatal("source race did not preserve local branch")
	}
}

func TestWorktreeSyncRejectsWorktreeSpecificFetchConfig(t *testing.T) {
	f := newSyncFixture(t)
	before := syncGit(t, f.main, "rev-parse", "origin/master")
	syncGit(t, f.main, "config", "extensions.worktreeConfig", "true")
	other := filepath.Join(filepath.Dir(f.remote), "other.git")
	syncGit(t, filepath.Dir(f.main), "init", "--bare", "--initial-branch=master", other)
	syncCommit(t, f.writer, "wrong-server.txt", "wrong server\n")
	syncGit(t, f.writer, "push", other, "HEAD:master")
	for _, setting := range []struct{ key, value string }{
		{"url." + other + ".insteadOf", f.remote},
		{"remote.origin.uploadpack", "must-not-execute"},
	} {
		syncGit(t, f.develop, "config", "--worktree", setting.key, setting.value)
		for _, dryRun := range []bool{false, true} {
			plan, err := PlanWorktreeSync(context.Background(), f.main, WorktreeSyncOptions{Config: f.config(), DryRun: dryRun})
			if err != nil {
				t.Fatal(err)
			}
			if err := ExecuteWorktreeSync(context.Background(), plan); err == nil {
				t.Fatal("different effective remote config accepted")
			}
			if plan.Fetched || syncGit(t, f.main, "rev-parse", "origin/master") != before {
				t.Fatal("fetched from mismatched worktree config")
			}
			message := syncResult(t, plan, "master").Message
			if !strings.Contains(message, "different effective Git configurations") || strings.Contains(message, other) {
				t.Fatalf("unsafe/missing diagnostic: %s", message)
			}
		}
		syncGit(t, f.develop, "config", "--worktree", "--unset", setting.key)
	}
}

func TestWorktreeSyncRejectsLocalUpstreamEvenWithParentRule(t *testing.T) {
	f := newSyncFixture(t)
	syncGit(t, f.feature, "branch", "--set-upstream-to=develop")
	plan, err := PlanWorktreeSync(context.Background(), f.feature, WorktreeSyncOptions{Config: f.config(), DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := ExecuteWorktreeSync(context.Background(), plan); err == nil {
		t.Fatal("implicit local dependency accepted")
	}
	if !strings.Contains(syncResult(t, plan, "feature/api").Message, "unset the local upstream") {
		t.Fatal("missing remediation for unsupported local upstream")
	}
}
