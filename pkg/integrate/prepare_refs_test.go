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

// headGuard binds only the HEAD of dir, for profile tests that have no plan.
func headGuard(t *testing.T, dir string) preparedRefGuard {
	t.Helper()
	return preparedRefGuard{head: runGitInTest(t, dir, "rev-parse", "HEAD")}
}

// refGuardFixture is a repository whose bound refs are all present: source
// "src", target "tgt" and its remote-tracking ref origin/tgt, plus a
// bystander branch and remote-tracking ref that no plan binds. The worktree
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

// Shell fragments run inside the prepared directory by the fake tools. They
// carry their own identity because the go profile isolates git configuration.
const (
	refsNewCommit = `new=$(GIT_AUTHOR_NAME=a GIT_AUTHOR_EMAIL=a@b GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@b git commit-tree -p HEAD -m moved 'HEAD^{tree}')`

	refsUnrelatedChurn = `git branch created-meanwhile HEAD
git branch -D bystander
git update-ref -d refs/remotes/origin/bystander
git update-ref refs/remotes/origin/created-meanwhile HEAD`
)

type refGuardProfile struct {
	name    string
	profile string
	files   map[string]string
	// install puts a fake tool on PATH that runs action after the profile's
	// own legitimate output has been written.
	install func(t *testing.T, action string)
}

func refGuardProfiles() []refGuardProfile {
	return []refGuardProfile{
		{
			name:    "cargo",
			profile: cargoWorkspacePrepareV1,
			files:   map[string]string{".gitignore": "target/\n", "Cargo.toml": "[workspace]\nmembers = []\n"},
			install: func(t *testing.T, action string) {
				t.Helper()
				t.Setenv("PATH", fakeCargo(t, "mkdir -p target\nif [ \"$1\" = fetch ]; then exit 0; fi\n"+action)+":"+os.Getenv("PATH"))
			},
		},
		{
			name:    "pnpm",
			profile: pnpmFrozenLockfilePrepareV1,
			files:   map[string]string{".gitignore": "node_modules/\n", "package.json": "{\"name\":\"fixture\",\"private\":true}\n"},
			install: func(t *testing.T, action string) {
				t.Helper()
				bin := fakeCargo(t, "mkdir -p node_modules\n"+action)
				if err := os.Rename(filepath.Join(bin, "cargo"), filepath.Join(bin, "pnpm")); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			},
		},
		{
			name:    "ent",
			profile: familybookEntPrepareV1,
			files:   map[string]string{".gitignore": "ent/generated/\n", "ent/.keep": ""},
			install: func(t *testing.T, action string) {
				t.Helper()
				t.Setenv("PATH", fakeGo(t, "mkdir -p ent/generated\n"+action)+":"+os.Getenv("PATH"))
			},
		},
	}
}

func TestPrepareProfilesTolerateUnrelatedRefChurn(t *testing.T) {
	for _, p := range refGuardProfiles() {
		t.Run(p.name, func(t *testing.T) {
			fx := newRefGuardFixture(t, p.files)
			p.install(t, refsUnrelatedChurn)
			if err := runPrepareProfile(context.Background(), fx.g, fx.dir, p.profile, fx.guard); err != nil {
				t.Fatalf("unrelated branch creation/deletion failed preparation: %v", err)
			}
			if got := runGitInTest(t, fx.dir, "branch", "--list", "created-meanwhile"); got == "" {
				t.Fatal("fake tool did not create the unrelated branch; the test exercised nothing")
			}
		})
	}
}

func TestPrepareProfilesRejectMovedBoundRefsAndHead(t *testing.T) {
	cases := map[string]struct {
		action string
		want   string
	}{
		"source moved":        {refsNewCommit + "\ngit update-ref refs/heads/src \"$new\"", "refs/heads/src"},
		"target moved":        {refsNewCommit + "\ngit update-ref refs/heads/tgt \"$new\"", "refs/heads/tgt"},
		"remote target moved": {refsNewCommit + "\ngit update-ref refs/remotes/origin/tgt \"$new\"", "refs/remotes/origin/tgt"},
		"target deleted":      {"git branch -D tgt", "refs/heads/tgt"},
		"remote target gone":  {"git update-ref -d refs/remotes/origin/tgt", "refs/remotes/origin/tgt"},
		"head moved":          {refsNewCommit + "\ngit update-ref --no-deref HEAD \"$new\"", "prepared worktree HEAD"},
	}
	for _, p := range refGuardProfiles() {
		for name, tc := range cases {
			t.Run(p.name+"/"+name, func(t *testing.T) {
				fx := newRefGuardFixture(t, p.files)
				p.install(t, tc.action)
				err := runPrepareProfile(context.Background(), fx.g, fx.dir, p.profile, fx.guard)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("err = %v, want it to name %q", err, tc.want)
				}
			})
		}
	}
}

func TestPreparedRefSnapshotDetectsMovedRefWithUnchangedNames(t *testing.T) {
	// A names-only snapshot cannot see this: every ref name is still there.
	// This also stands in for the flow profile, which shares snapshot/verify.
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
	runGitInTest(t, fx.dir, "update-ref", "refs/heads/tgt", other)
	if err := snap.verify(ctx, fx.g, fx.dir); err == nil || !strings.Contains(err.Error(), "refs/heads/tgt") {
		t.Fatalf("moved target not detected: %v", err)
	}
}

func TestNewPreparedRefGuardBindsOnlyPlanRefs(t *testing.T) {
	fx := newRefGuardFixture(t, map[string]string{"f": "x\n"})
	ctx := context.Background()
	want := []string{"refs/heads/src", "refs/heads/tgt", "refs/remotes/origin/tgt"}
	if strings.Join(fx.guard.refs, " ") != strings.Join(want, " ") {
		t.Fatalf("bound refs = %v, want %v", fx.guard.refs, want)
	}
	// A target without a local branch binds only its remote-tracking ref, so
	// a local branch another session creates later is not a change.
	runGitInTest(t, fx.dir, "branch", "-D", "tgt")
	plan := TargetPlan{Branch: "src", Remote: "origin", Target: "origin/tgt"}
	guard, err := newPreparedRefGuard(ctx, fx.g, plan, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(guard.refs, " ") != "refs/heads/src refs/remotes/origin/tgt" {
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
