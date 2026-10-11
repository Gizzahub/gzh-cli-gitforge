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

const cloneTaskBranch = "dev/actor/feat/task"

// cloneReclaimFixture is a task worktree whose gate cloned a workspace into an
// ignored directory, the way `make prepare` runs `gz-git sync`. workspaces is
// the YAML body under the `workspaces:` key; "%URL%" is replaced by the clone
// source. It returns the fixture and the clone directory.
func cloneReclaimFixture(t *testing.T, workspaces string) (fx *testutil.WorktreeOrigin, clone string) {
	t.Helper()
	upstream := testutil.TempGitRepoWithCommit(t)
	fx = testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "branch", "develop")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "checkout", "-B", cloneTaskBranch, "develop")
	writeFile(t, fx.Worktree, "task.txt", "task\n")
	writeFile(t, fx.Worktree, ".gitignore", "ws-a/\n")
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: dev/*\nworkspaces:\n"+strings.ReplaceAll(workspaces, "%URL%", upstream))
	writeGateMakefile(t, fx.Worktree)
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "task work")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	clone = filepath.Join(fx.Worktree, "ws-a")
	runGit(t, fx.Worktree, "clone", upstream, clone)
	return fx, clone
}

const declaredWsA = "  ws-a:\n    url: %URL%\n    type: git\n"

func runCloneReclaim(t *testing.T, fx *testutil.WorktreeOrigin) *RunReport {
	t.Helper()
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{RepoPath: fx.Worktree, Branch: cloneTaskBranch},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !report.Integrated {
		t.Fatalf("want integrated:\n%s", FormatRun(report))
	}
	return report
}

func TestRun_ReclaimRemovesDeclaredCleanWorkspaceClone(t *testing.T) {
	fx, clone := cloneReclaimFixture(t, declaredWsA)
	report := runCloneReclaim(t, fx)
	if report.Reclaim.Incomplete() {
		t.Fatalf("reclaim incomplete: %+v", report.Reclaim)
	}
	if _, err := os.Stat(fx.Worktree); err == nil {
		t.Fatal("worktree should be removed")
	}
	if refExists(t, fx.Clone, "refs/heads/"+cloneTaskBranch) {
		t.Fatal("local task branch should be deleted")
	}
	if refExists(t, fx.Origin, "refs/heads/"+cloneTaskBranch) {
		t.Fatal("remote task branch should be deleted")
	}
	if _, err := os.Stat(clone); err == nil {
		t.Fatal("declared clone should be removed with the worktree")
	}
}

func TestRun_ReclaimRefusesNestedRepoThatIsNotProvablyDisposable(t *testing.T) {
	cases := map[string]struct {
		workspaces string
		mutate     func(t *testing.T, clone string)
		want       string
	}{
		"undeclared": {
			workspaces: "  other:\n    url: %URL%\n    type: git\n",
			want:       "not a declared workspace clone",
		},
		"declared at another path": {
			workspaces: "  ws-a:\n    path: ./elsewhere\n    url: %URL%\n    type: git\n",
			want:       "not a declared workspace clone",
		},
		"url mismatch": {
			workspaces: "  ws-a:\n    url: https://example.invalid/other.git\n    type: git\n",
			want:       "is not the declared",
		},
		"untracked file": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				t.Helper()
				writeFile(t, clone, "scratch.txt", "x\n")
			},
			want: "uncommitted or untracked changes",
		},
		"modified file": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				t.Helper()
				writeFile(t, clone, "README.md", "changed\n")
			},
			want: "uncommitted or untracked changes",
		},
		"unpushed commit": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				runGit(t, clone, "-c", "user.name=a", "-c", "user.email=a@b", "commit", "--allow-empty", "-m", "local only")
			},
			want: "is ahead of its upstream",
		},
		"branch without upstream": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				runGit(t, clone, "branch", "local-only")
			},
			want: "has no upstream",
		},
		"stash entry": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				writeFile(t, clone, "README.md", "stashed\n")
				runGit(t, clone, "-c", "user.name=a", "-c", "user.email=a@b", "stash", "push")
			},
			want: "stash entries",
		},
		"detached unpushed commit": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				runGit(t, clone, "checkout", "--detach")
				runGit(t, clone, "-c", "user.name=a", "-c", "user.email=a@b", "commit", "--allow-empty", "-m", "detached")
			},
			want: "detached HEAD",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fx, clone := cloneReclaimFixture(t, tc.workspaces)
			if tc.mutate != nil {
				tc.mutate(t, clone)
			}
			report := runCloneReclaim(t, fx)
			if !report.Reclaim.Incomplete() {
				t.Fatalf("reclaim must be incomplete: %+v", report.Reclaim)
			}
			failed := strings.Join(report.Reclaim.Failed, "\n")
			if !strings.Contains(failed, "ws-a/") || !strings.Contains(failed, tc.want) {
				t.Fatalf("failure must name the path and %q, got %q", tc.want, failed)
			}
			if _, err := os.Stat(clone); err != nil {
				t.Fatalf("refused clone must be left untouched: %v", err)
			}
			if _, err := os.Stat(fx.Worktree); err != nil {
				t.Fatalf("worktree must remain: %v", err)
			}
			assertRef(t, fx.Clone, "refs/heads/"+cloneTaskBranch)
			assertRef(t, fx.Origin, "refs/heads/"+cloneTaskBranch)
		})
	}
}

func TestNormalizeCloneURL(t *testing.T) {
	for in, want := range map[string]string{
		"ssh://git@host:22/a/b.git":  "ssh://git@host:22/a/b",
		"https://host/a/b.git/":      "https://host/a/b",
		"https://host/a/b.github":    "https://host/a/b.github",
		"https://other-host/a/b.git": "https://other-host/a/b",
	} {
		if got := normalizeCloneURL(in); got != want {
			t.Errorf("normalizeCloneURL(%q) = %q, want %q", in, got, want)
		}
	}
}
