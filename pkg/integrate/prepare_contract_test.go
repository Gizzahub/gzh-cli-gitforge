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

func TestResolvePrepareProfileCommitOwnership(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	base := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  prepareProfile: flow-taskchain-local-subprojects-v1\n")
	runGitInTest(t, fx.Worktree, "add", ".gz-git.yaml")
	runGitInTest(t, fx.Worktree, "commit", "-m", "declare profile")
	declared := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  prepareProfile: familybook-ent-v1\n")
	runGitInTest(t, fx.Worktree, "add", ".gz-git.yaml")
	runGitInTest(t, fx.Worktree, "commit", "-m", "change profile")
	changed := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	for _, tc := range []struct {
		name, target, source, controller, want string
		fail                                   bool
	}{
		{name: "legacy", target: base, source: base},
		{name: "source bootstrap", target: base, source: declared, want: "flow-taskchain-local-subprojects-v1"},
		{name: "matching", target: declared, source: declared, want: "flow-taskchain-local-subprojects-v1"},
		{name: "target removed", target: declared, source: base, fail: true},
		{name: "different", target: declared, source: changed, fail: true},
		{name: "controller only", target: base, source: base, controller: "familybook-ent-v1", want: "familybook-ent-v1"},
		{name: "controller conflict", target: base, source: declared, controller: "familybook-ent-v1", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var controller *controllerBinding
			if tc.controller != "" {
				controller = &controllerBinding{PrepareProfile: tc.controller}
			}
			got, err := resolvePrepareProfile(context.Background(), g, TargetPlan{TargetSHA: tc.target, BranchSHA: tc.source}, controller)
			if (err != nil) != tc.fail || (!tc.fail && got != tc.want) {
				t.Fatalf("profile=%q err=%v; want=%q fail=%v", got, err, tc.want, tc.fail)
			}
		})
	}
}

func TestLoadCommitPrepareProfilePreservesLargeLegacyManifest(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	padding := strings.Repeat("# legacy comment\n", 5000)
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: master\n"+padding)
	runGitInTest(t, fx.Worktree, "add", ".gz-git.yaml")
	runGitInTest(t, fx.Worktree, "commit", "-m", "large legacy config")
	legacy := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	profile, declared, err := loadCommitPrepareProfile(context.Background(), g, legacy)
	if err != nil || declared || profile != "" {
		t.Fatalf("large legacy config profile=%q declared=%v err=%v", profile, declared, err)
	}
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  prepareProfile: familybook-ent-v1\n"+padding)
	runGitInTest(t, fx.Worktree, "add", ".gz-git.yaml")
	runGitInTest(t, fx.Worktree, "commit", "-m", "large declared config")
	declaredSHA := strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))
	if _, _, err := loadCommitPrepareProfile(context.Background(), g, declaredSHA); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("large declared config err=%v", err)
	}
}
