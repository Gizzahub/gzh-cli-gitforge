// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

// TestCheck_StaleFreshnessWithoutContractRunsNothing pins that a stale source
// in a repository with no readiness contract fails freshness alone and never
// reaches the gate stage, even when the repository carries a Makefile.
func TestCheck_StaleFreshnessWithoutContractRunsNothing(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	marker := filepath.Join(t.TempDir(), "commands-ran")
	t.Setenv("MARKER", marker)

	runGit(t, fx.Clone, "checkout", "-B", "develop")
	writeRepoFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n")
	writeRepoFile(t, fx.Clone, "Makefile", "check:\n\t@printf check >> \"$$MARKER\"\nlint:\n\t@printf lint >> \"$$MARKER\"\n")
	runGit(t, fx.Clone, "add", ".gz-git.yaml", "Makefile")
	runGit(t, fx.Clone, "commit", "-m", "declare integration prerequisites")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")

	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	writeRepoFile(t, fx.Worktree, "task.txt", "task\n")
	runGit(t, fx.Worktree, "add", "task.txt")
	runGit(t, fx.Worktree, "commit", "-m", "task work")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")

	writeRepoFile(t, fx.Clone, "later.txt", "later\n")
	runGit(t, fx.Clone, "add", "later.txt")
	runGit(t, fx.Clone, "commit", "-m", "develop moves")
	runGit(t, fx.Clone, "push", fx.Remote, "develop")

	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Ready || !hasCheckDetail(report, "freshness", checkFail, "commits behind") || report.Failures != 1 || report.GateMode != "" {
		t.Fatalf("stale source must report only freshness failure:\n%s", FormatCheck(report))
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("stale source ran repository commands: stat marker = %v", err)
	}
}
