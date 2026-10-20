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
	writeFile(t, fx.Worktree, ".gitignore", "ws-*/\n")
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: dev/*\nworkspaces:\n"+strings.ReplaceAll(workspaces, "%URL%", upstream))
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
	t.Parallel()
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
	t.Parallel()
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
				writeFile(t, clone, "scratch.txt", "x\n")
			},
			want: "uncommitted or untracked changes",
		},
		"modified file": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
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
			want: "commits not on any remote-tracking ref",
		},
		"local-only branch with a commit": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				runGit(t, clone, "checkout", "-b", "local-only")
				runGit(t, clone, "-c", "user.name=a", "-c", "user.email=a@b", "commit", "--allow-empty", "-m", "local only")
				runGit(t, clone, "checkout", "-")
			},
			want: "commits not on any remote-tracking ref",
		},
		"unreachable annotated tag after reset": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				runGit(t, clone, "-c", "user.name=a", "-c", "user.email=a@b", "commit", "--allow-empty", "-m", "tagged only")
				runGit(t, clone, "-c", "user.name=a", "-c", "user.email=a@b", "tag", "-a", "-m", "keep", "keep")
				runGit(t, clone, "reset", "--hard", "@{u}")
			},
			want: "commits not on any remote-tracking ref",
		},
		"mutually local upstream loop": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				runGit(t, clone, "branch", "loop-a")
				runGit(t, clone, "branch", "loop-b")
				runGit(t, clone, "branch", "--set-upstream-to=loop-b", "loop-a")
				runGit(t, clone, "branch", "--set-upstream-to=loop-a", "loop-b")
			},
			want: "tracks a non-remote ref",
		},
		"ignored nested repo inside the clone": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				excludeInClone(t, clone, "vendor/")
				runGit(t, clone, "init", filepath.Join(clone, "vendor", "deep", "inner"))
			},
			want: "contains a git repo",
		},
		"ignored nested gitfile inside the clone": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				excludeInClone(t, clone, "vendor/")
				if err := os.MkdirAll(filepath.Join(clone, "vendor", "sub"), 0o755); err != nil {
					t.Fatal(err)
				}
				// git lists an ignored directory only when it holds a file
				// besides the .git entry, as a real checkout does.
				writeFile(t, filepath.Join(clone, "vendor", "sub"), "src.txt", "x\n")
				writeFile(t, filepath.Join(clone, "vendor", "sub"), ".git", "gitdir: /elsewhere\n")
			},
			want: "contains a git repo",
		},
		"submodule with an unpushed commit": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				sub := testutil.TempGitRepoWithCommit(t)
				runGit(t, clone, "-c", "protocol.file.allow=always", "submodule", "add", sub, "sub")
				runGit(t, clone, "-c", "user.name=a", "-c", "user.email=a@b", "commit", "-m", "add submodule")
				runGit(t, filepath.Join(clone, "sub"), "-c", "user.name=a", "-c", "user.email=a@b", "commit", "--allow-empty", "-m", "sub local only")
				// Detach the sub at the recorded commit so the super's status is
				// clean; the local commit stays on the sub's branch.
				runGit(t, filepath.Join(clone, "sub"), "checkout", "--detach", "HEAD~1")
				runGit(t, clone, "update-ref", "refs/remotes/origin/pushed", "HEAD")
			},
			want: "has initialized submodules",
		},
		"repository at an uninitialized submodule path": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				src := testutil.TempGitRepoWithCommit(t)
				sha := runGitInTest(t, src, "rev-parse", "HEAD")
				writeFile(t, clone, ".gitmodules", "[submodule \"sub\"]\n\tpath = sub\n\turl = "+src+"\n")
				runGit(t, clone, "update-index", "--add", "--cacheinfo", "160000,"+sha+",sub")
				runGit(t, clone, "add", ".gitmodules")
				runGit(t, clone, "-c", "user.name=a", "-c", "user.email=a@b", "commit", "-m", "declare submodule")
				runGit(t, clone, "update-ref", "refs/remotes/origin/pushed", "HEAD")
				// Hand-clone into the declared, uninitialized path and keep a local
				// commit on a branch, detached at the recorded commit.
				runGit(t, clone, "clone", src, "sub")
				sub := filepath.Join(clone, "sub")
				runGit(t, sub, "-c", "user.name=a", "-c", "user.email=a@b", "commit", "--allow-empty", "-m", "sub local only")
				runGit(t, sub, "checkout", "--detach", "HEAD~1")
			},
			want: "uninitialized submodule path",
		},
		"linked worktree": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				runGit(t, clone, "worktree", "add", "--detach", filepath.Join(t.TempDir(), "linked"))
			},
			want: "has linked worktrees",
		},
		"gitfile instead of a git directory": {
			workspaces: declaredWsA,
			mutate: func(t *testing.T, clone string) {
				t.Helper()
				moved := filepath.Join(t.TempDir(), "gitdir")
				if err := os.Rename(filepath.Join(clone, ".git"), moved); err != nil {
					t.Fatal(err)
				}
				writeFile(t, clone, ".git", "gitdir: "+moved+"\n")
			},
			want: ".git is not a directory",
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
			want: "commits not on any remote-tracking ref",
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

// excludeInClone ignores pattern in the clone without touching its tree, so
// the clone stays clean while holding an ignored entry.
func excludeInClone(t *testing.T, clone, pattern string) {
	t.Helper()
	exclude := filepath.Join(clone, ".git", "info", "exclude")
	f, err := os.OpenFile(exclude, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(pattern + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRun_ReclaimDeletesIgnoredNonRepoFilesWithTheClone(t *testing.T) {
	t.Parallel()
	// Policy: ignored files that are not repos go with the clone, as
	// `git worktree remove` already does for the parent's ignored files.
	fx, clone := cloneReclaimFixture(t, declaredWsA)
	excludeInClone(t, clone, ".env")
	writeFile(t, clone, ".env", "SECRET=1\n")
	report := runCloneReclaim(t, fx)
	if report.Reclaim.Incomplete() {
		t.Fatalf("reclaim incomplete: %+v", report.Reclaim)
	}
	if _, err := os.Stat(clone); err == nil {
		t.Fatal("clone with an ignored .env should be removed")
	}
}

func TestRun_ReclaimAcceptsUninitializedSubmodulePathWithoutRepository(t *testing.T) {
	t.Parallel()
	fx, clone := cloneReclaimFixture(t, declaredWsA)
	src := testutil.TempGitRepoWithCommit(t)
	sha := runGitInTest(t, src, "rev-parse", "HEAD")
	writeFile(t, clone, ".gitmodules", "[submodule \"sub\"]\n\tpath = sub\n\turl = "+src+"\n")
	runGit(t, clone, "update-index", "--add", "--cacheinfo", "160000,"+sha+",sub")
	runGit(t, clone, "add", ".gitmodules")
	runGit(t, clone, "-c", "user.name=a", "-c", "user.email=a@b", "commit", "-m", "declare submodule")
	runGit(t, clone, "update-ref", "refs/remotes/origin/pushed", "HEAD")
	// An uninitialized submodule leaves an empty directory at its path.
	if err := os.Mkdir(filepath.Join(clone, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	report := runCloneReclaim(t, fx)
	if report.Reclaim.Incomplete() {
		t.Fatalf("reclaim incomplete: %+v", report.Reclaim)
	}
}

func TestRun_ReclaimAcceptsLocalBranchAtPushedCommit(t *testing.T) {
	t.Parallel()
	// No upstream is fine when every commit is already on a remote.
	fx, clone := cloneReclaimFixture(t, declaredWsA)
	runGit(t, clone, "branch", "no-upstream")
	report := runCloneReclaim(t, fx)
	if report.Reclaim.Incomplete() {
		t.Fatalf("reclaim incomplete: %+v", report.Reclaim)
	}
}

func TestRun_ReclaimVerificationIsAllOrNothing(t *testing.T) {
	t.Parallel()
	fx, cloneA := cloneReclaimFixture(t, declaredWsA+"  ws-b:\n    url: %URL%\n    type: git\n")
	url := runGitInTest(t, cloneA, "config", "--get", "remote.origin.url")
	cloneB := filepath.Join(fx.Worktree, "ws-b")
	runGit(t, fx.Worktree, "clone", url, cloneB)
	writeFile(t, cloneB, "scratch.txt", "dirty\n")
	report := runCloneReclaim(t, fx)
	if !report.Reclaim.Incomplete() {
		t.Fatalf("reclaim must be incomplete: %+v", report.Reclaim)
	}
	for _, c := range []string{cloneA, cloneB} {
		if _, err := os.Stat(c); err != nil {
			t.Fatalf("%s must survive when another clone fails verification: %v", c, err)
		}
	}
}

func TestRemoveVerifiedCloneRefusesSymlinkedParent(t *testing.T) {
	t.Parallel()
	taskWT := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "ws-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(outside, "ws-a"), "keep.txt", "precious\n")
	if err := os.Symlink(outside, filepath.Join(taskWT, "link")); err != nil {
		t.Fatal(err)
	}
	err := removeVerifiedClone(taskWT, "link/ws-a", filepath.Join(taskWT, "link", "ws-a"))
	if err == nil {
		t.Fatal("deletion through a symlinked parent must be refused")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "ws-a", "keep.txt")); statErr != nil {
		t.Fatalf("file outside the worktree was touched: %v", statErr)
	}
}

func TestRemoveVerifiedCloneRemovesPlainPath(t *testing.T) {
	t.Parallel()
	taskWT := t.TempDir()
	if err := os.MkdirAll(filepath.Join(taskWT, "ws-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := removeVerifiedClone(taskWT, "ws-a", filepath.Join(taskWT, "ws-a")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(taskWT, "ws-a")); err == nil {
		t.Fatal("plain path should be removed")
	}
}

func TestNormalizeCloneURL(t *testing.T) {
	t.Parallel()
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
