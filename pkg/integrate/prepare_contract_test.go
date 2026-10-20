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

func TestPrepareHighLoadForPlanUsesEitherCommittedDeclaration(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	base := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  prepareHighLoad: true\n")
	runGitInTest(t, fx.Worktree, "add", ".gz-git.yaml")
	runGitInTest(t, fx.Worktree, "commit", "-m", "declare heavy preparation")
	heavy := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	for _, tc := range []struct{ target, source string }{{base, heavy}, {heavy, base}, {heavy, heavy}} {
		got, err := prepareHighLoadForPlan(context.Background(), g, TargetPlan{TargetSHA: tc.target, BranchSHA: tc.source})
		if err != nil || !got {
			t.Fatalf("prepareHighLoadForPlan(%s,%s) = %v, %v", tc.target, tc.source, got, err)
		}
	}
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  prepareHighLoad: null\n")
	runGitInTest(t, fx.Worktree, "add", ".gz-git.yaml")
	runGitInTest(t, fx.Worktree, "commit", "-m", "malformed preparation declaration")
	bad := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	if _, err := prepareHighLoadForPlan(context.Background(), g, TargetPlan{TargetSHA: base, BranchSHA: bad}); err == nil {
		t.Fatal("malformed committed source declaration did not fail closed")
	}
}

func TestPrepareHighLoadDeclarationSizeLimit(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	padding := strings.Repeat("# padding\n", 8000)
	for _, declaration := range []string{"prepareHighLoad: true"} {
		writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  "+declaration+"\n"+padding)
		runGitInTest(t, fx.Worktree, "add", ".gz-git.yaml")
		runGitInTest(t, fx.Worktree, "commit", "-m", "oversized preparation declaration")
		sha := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
		if _, err := loadCommitPrepareHighLoad(context.Background(), g, sha); err == nil || !strings.Contains(err.Error(), "size limit") {
			t.Fatalf("declaration %q error = %v", declaration, err)
		}
	}
}
