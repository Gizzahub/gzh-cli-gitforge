// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

// releaseRemoteFixture is the CE run-release shape: the primary checkout is
// on the default branch, and the release source is the remote-tracking
// integration ref, which is never checked out and has no upstream. Its gate
// passes only in a tree carrying develop-only content, so a make run in the
// live default-branch checkout fails.
func releaseRemoteFixture(t *testing.T) *testutil.WorktreeOrigin {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeRepoFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n")
	writeRepoFile(t, fx.Clone, "Makefile", "check:\n\t@test -f develop-only.txt\n")
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "declare policy")
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/main")
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", "develop", fx.Remote+"/main")
	writeRepoFile(t, fx.Worktree, "develop-only.txt", "release\n")
	runGit(t, fx.Worktree, "add", "develop-only.txt")
	runGit(t, fx.Worktree, "commit", "-m", "develop work")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "checkout", "--detach")
	runGit(t, fx.Worktree, "branch", "-D", "develop")
	runGit(t, fx.Clone, "fetch", fx.Remote)
	return fx
}

func releaseRemoteOptions(fx *testutil.WorktreeOrigin) CheckOptions {
	return CheckOptions{
		RepoPath: fx.Clone,
		Branch:   fx.Remote + "/develop",
		Target:   fx.Remote + "/main",
		Release:  true,
	}
}

func TestCheck_ReleaseFromRemoteRefIgnoresHeadAndUpstream(t *testing.T) {
	fx := releaseRemoteFixture(t)
	report, err := Check(context.Background(), gitcmd.NewExecutor(), releaseRemoteOptions(fx))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Ready {
		t.Fatalf("want READY, got\n%s", FormatCheck(report))
	}
	want := map[string]string{"working-tree": checkPass, "push": checkPass, "make check": checkPass}
	for _, item := range report.Items {
		if status, ok := want[item.Name]; ok {
			if item.Status != status {
				t.Fatalf("%s = %+v, want %s", item.Name, item, status)
			}
			delete(want, item.Name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing rows %v:\n%s", want, FormatCheck(report))
	}
}

func TestCheck_ReleaseFromRemoteRefStillRequiresCleanCheckout(t *testing.T) {
	fx := releaseRemoteFixture(t)
	writeRepoFile(t, fx.Clone, "README.md", "dirty\n")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), releaseRemoteOptions(fx))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, item := range report.Items {
		if item.Name == "working-tree" {
			if item.Status != checkFail || !strings.Contains(item.Detail, "uncommitted") {
				t.Fatalf("working-tree = %+v", item)
			}
			return
		}
	}
	t.Fatalf("missing working-tree row:\n%s", FormatCheck(report))
}

func TestCheck_ReleaseFromLocalBranchStillRequiresHead(t *testing.T) {
	fx := releaseRemoteFixture(t)
	runGit(t, fx.Clone, "branch", "develop", fx.Remote+"/develop")
	opts := releaseRemoteOptions(fx)
	opts.Branch = "develop"
	report, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Ready {
		t.Fatalf("local release source off HEAD must not be READY:\n%s", FormatCheck(report))
	}
}

func TestRun_ReleaseFromRemoteRefFastForwardsDefaultAndKeepsIntegration(t *testing.T) {
	fx := releaseRemoteFixture(t)
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: releaseRemoteOptions(fx)})
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, FormatRun(report))
	}
	develop := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/develop"))
	if report.SHA != develop {
		t.Fatalf("integrated %s, want %s", report.SHA, develop)
	}
	runGit(t, fx.Clone, "fetch", fx.Remote)
	if got := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/main")); got != develop {
		t.Fatalf("remote main = %s, want %s", got, develop)
	}
	if got := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", "HEAD")); got != develop {
		t.Fatalf("default checkout HEAD = %s, want fast-forward to %s", got, develop)
	}
	if !strings.Contains(report.Reclaim.Skipped, "integration/default") {
		t.Fatalf("release must not reclaim the integration ref: %+v", report.Reclaim)
	}
}
