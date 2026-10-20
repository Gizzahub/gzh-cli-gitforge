// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

// refGuardFixture is a repository whose bound refs are all present: source
// "src" and the remote-tracking target origin/tgt, plus a local branch tgt of
// the same name, a bystander branch and a bystander remote-tracking ref that
// the plan does not bind. The worktree
// is detached at the common commit, as a prepared worktree is.
type refGuardFixture struct {
	dir   string
	g     gitRepo
	guard preparedRefGuard
}

func newRefGuardFixture(t *testing.T, files map[string]string) refGuardFixture {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(fx.Worktree, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, fx.Worktree, name, body)
	}
	runGitInTest(t, fx.Worktree, "add", ".")
	runGitInTest(t, fx.Worktree, "commit", "-m", "fixture")
	sha := runGitInTest(t, fx.Worktree, "rev-parse", "HEAD")
	for _, branch := range []string{"src", "tgt", "bystander"} {
		runGitInTest(t, fx.Worktree, "branch", branch, sha)
	}
	runGitInTest(t, fx.Worktree, "update-ref", "refs/remotes/origin/tgt", sha)
	runGitInTest(t, fx.Worktree, "update-ref", "refs/remotes/origin/bystander", sha)
	runGitInTest(t, fx.Worktree, "checkout", "--detach", sha)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	plan := TargetPlan{Branch: "src", BranchSHA: sha, Remote: "origin", Target: "origin/tgt", TargetSHA: sha}
	guard, err := newPreparedRefGuard(context.Background(), g, plan, sha)
	if err != nil {
		t.Fatal(err)
	}
	return refGuardFixture{dir: fx.Worktree, g: g, guard: guard}
}

func TestPreparedRefSnapshotDetectsMovedRefWithUnchangedNames(t *testing.T) {
	t.Parallel()
	// A names-only snapshot cannot see this: every ref name is still there.
	fx := newRefGuardFixture(t, map[string]string{"f": "x\n"})
	ctx := context.Background()
	snap, err := fx.guard.snapshot(ctx, fx.g)
	if err != nil {
		t.Fatal(err)
	}
	if err := snap.verify(ctx, fx.g, fx.dir); err != nil {
		t.Fatalf("unchanged refs failed verification: %v", err)
	}
	other := runGitInTest(t, fx.dir, "commit-tree", "-p", "HEAD", "-m", "moved", "HEAD^{tree}")
	runGitInTest(t, fx.dir, "update-ref", "refs/heads/src", other)
	if err := snap.verify(ctx, fx.g, fx.dir); err == nil || !strings.Contains(err.Error(), "refs/heads/src") {
		t.Fatalf("moved target not detected: %v", err)
	}
}

func TestNewPreparedRefGuardBindsOnlyPlanRefs(t *testing.T) {
	t.Parallel()
	fx := newRefGuardFixture(t, map[string]string{"f": "x\n"})
	ctx := context.Background()
	// A remote-tracking target binds only that ref, even though a local
	// branch of the same name exists.
	want := []string{"refs/heads/src", "refs/remotes/origin/tgt"}
	if strings.Join(fx.guard.refs, " ") != strings.Join(want, " ") || !fx.guard.detached {
		t.Fatalf("bound refs = %v detached=%v, want %v detached", fx.guard.refs, fx.guard.detached, want)
	}
	// A local target binds the local branch.
	plan := TargetPlan{Branch: "src", Remote: "origin", Target: "tgt"}
	guard, err := newPreparedRefGuard(ctx, fx.g, plan, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(guard.refs, " ") != "refs/heads/src refs/heads/tgt" {
		t.Fatalf("bound refs = %v", guard.refs)
	}
	// A target that resolves to no ref binds nothing.
	plan = TargetPlan{Branch: "src", Remote: "origin", Target: "origin/missing"}
	guard, err = newPreparedRefGuard(ctx, fx.g, plan, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(guard.refs, " ") != "refs/heads/src" {
		t.Fatalf("bound refs = %v", guard.refs)
	}
	// A remote release source is bound as the remote-tracking ref.
	plan = TargetPlan{Branch: "origin/tgt", Remote: "origin", Target: "origin/tgt", Release: true, Integration: Resolution{Name: "tgt"}}
	guard, err = newPreparedRefGuard(ctx, fx.g, plan, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(guard.refs, " ") != "refs/remotes/origin/tgt" {
		t.Fatalf("bound refs = %v", guard.refs)
	}
}
