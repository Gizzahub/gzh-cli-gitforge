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

const (
	gateTaskBranch  = "dev/task"
	declMainOnly    = "branch:\n  integrationBranch: main\n"
	declDevelopOnly = "branch:\n  integrationBranch: develop\n"
)

// gateFixture is a repository whose default branch main was prepared by
// commitMain (nil leaves it without a declaration) and whose develop starts
// at that commit. The linked worktree holds the pushed task branch dev/task,
// cut from main, which commits taskDecl as its declaration.
func gateFixture(t *testing.T, commitMain func(dir string), taskDecl string) *testutil.WorktreeOrigin {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	if commitMain != nil {
		commitMain(fx.Clone)
		runGit(t, fx.Clone, "add", ".")
		runGit(t, fx.Clone, "commit", "-m", "declare policy")
		runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/main")
	}
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/develop")
	gateTaskFromMain(t, fx, taskDecl)
	return fx
}

// gateTaskFromMain cuts dev/task from the fetched main in the linked
// worktree, commits taskDecl there, and pushes it. A second call replaces the
// task branch, so the push is forced.
func gateTaskFromMain(t *testing.T, fx *testutil.WorktreeOrigin, taskDecl string) {
	t.Helper()
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", gateTaskBranch, fx.Remote+"/main")
	runGit(t, fx.Worktree, "rm", "-q", "--ignore-unmatch", ".gz-git.yaml")
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", taskDecl)
	writeRepoFile(t, fx.Worktree, "task.txt", "work\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "task work")
	runGit(t, fx.Worktree, "push", "--force", "-u", fx.Remote, gateTaskBranch)
}

func declareOnMain(t *testing.T, decl string) func(string) {
	t.Helper()
	return func(dir string) { writeRepoFile(t, dir, ".gz-git.yaml", decl) }
}

func gateOptions(fx *testutil.WorktreeOrigin, target string) CheckOptions {
	return CheckOptions{RepoPath: fx.Worktree, Branch: gateTaskBranch, Target: fx.Remote + "/" + target}
}

// ISSUE-409: the change that first names a separate integration branch must
// reach the default branch while that branch still declares itself. The task
// branch's own value (develop) would turn the integration into a release
// that the default branch's declaration then refuses.
func TestCheck_DeclarationChangeLandsOnSelfDeclaredDefaultBranch(t *testing.T) {
	t.Parallel()
	fx := gateFixture(t, declareOnMain(t, declMainOnly), declDevelopOnly)
	report, err := Check(context.Background(), gitcmd.NewExecutor(), gateOptions(fx, "main"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Ready {
		t.Fatalf("want READY, got\n%s", FormatCheck(report))
	}
	if report.Plan.Integration.Name != "main" || !strings.HasPrefix(report.Plan.Integration.Source, SourceConfigPrefix) {
		t.Fatalf("integration = %+v, want main from the default branch declaration", report.Plan.Integration)
	}
	if got := report.Declaration.IntegrationBranch; len(got) != 1 || got[0] != "develop" {
		t.Fatalf("working-tree integrationBranch = %v, want [develop]", got)
	}
	main := strings.TrimSpace(gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/main"))
	want := "working tree declares integrationBranch develop; integrationBranch main from .gz-git.yaml at " +
		fx.Remote + "/main commit " + main[:12] + " gates the integration"
	if out := FormatCheck(report); !strings.Contains(out, want) {
		t.Fatalf("report does not name the gating declaration %q:\n%s", want, out)
	}
}

func TestRun_DeclarationChangeLandsOnSelfDeclaredDefaultBranch(t *testing.T) {
	t.Parallel()
	fx := gateFixture(t, declareOnMain(t, declMainOnly), declDevelopOnly)
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: gateOptions(fx, "main")})
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, FormatRun(report))
	}
	if got := strings.TrimSpace(gitOutput(t, fx.Origin, "rev-parse", "refs/heads/main")); got != report.SHA {
		t.Fatalf("origin main = %s, want integrated %s", got, report.SHA)
	}
	if got := gitOutput(t, fx.Origin, "show", "refs/heads/main:.gz-git.yaml"); strings.TrimSpace(got) != strings.TrimSpace(declDevelopOnly) {
		t.Fatalf("origin main declaration = %q, want %q", got, declDevelopOnly)
	}
}

// A task branch that declares the default branch must not land there by its
// own say-so while the default branch names another integration branch.
func TestCheck_TaskBranchCannotDeclareItsWayOntoDefaultBranch(t *testing.T) {
	t.Parallel()
	fx := gateFixture(t, declareOnMain(t, declDevelopOnly), declMainOnly)
	main := strings.TrimSpace(gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/main"))
	for name, opts := range map[string]CheckOptions{
		"explicit target": gateOptions(fx, "main"),
		// The working tree's value picks main as the default target.
		"default target": {RepoPath: fx.Worktree, Branch: gateTaskBranch},
	} {
		report, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
		if err == nil {
			t.Fatalf("%s: a self-declared task branch was not refused:\n%s", name, FormatCheck(report))
		}
		for _, want := range []string{
			"promoting onto the default branch requires --release",
			"integrationBranch develop from .gz-git.yaml at " + fx.Remote + "/main commit " + main[:12] + " gates this integration",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: error %q does not contain %q", name, err, want)
			}
		}
	}
}

// A local branch spelled like the remote-tracking default branch must not
// hide the default branch's declaration. Git then shortens <remote>/HEAD to
// "remotes/origin/main", so the gate cannot be read by short name.
func TestCheck_LocalBranchNamedLikeDefaultRefDoesNotSkipGate(t *testing.T) {
	t.Parallel()
	fx := gateFixture(t, declareOnMain(t, declDevelopOnly), declMainOnly)
	runGit(t, fx.Worktree, "branch", fx.Remote+"/main", "refs/remotes/"+fx.Remote+"/main")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), gateOptions(fx, "main"))
	if err == nil {
		t.Fatalf("the shadowing branch let a self-declared task branch through:\n%s", FormatCheck(report))
	}
	if want := "integrationBranch develop from .gz-git.yaml at "; !strings.Contains(err.Error(), want) ||
		!strings.Contains(err.Error(), "gates this integration") {
		t.Fatalf("error %q does not name the default branch declaration as the gate", err)
	}
}

// The default branch is the authority, not the target: an integration branch
// that still carries an older declaration takes the merge that updates it.
func TestCheck_StaleIntegrationBranchDeclarationDoesNotGate(t *testing.T) {
	t.Parallel()
	fx := gateFixture(t, declareOnMain(t, declMainOnly), declDevelopOnly)
	// main moves on to declare develop; origin/develop keeps declaring main.
	writeRepoFile(t, fx.Clone, ".gz-git.yaml", declDevelopOnly)
	runGit(t, fx.Clone, "commit", "-am", "name the integration branch")
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/main")
	gateTaskFromMain(t, fx, declDevelopOnly)
	if got := gitOutput(t, fx.Worktree, "show", fx.Remote+"/develop:.gz-git.yaml"); strings.TrimSpace(got) != strings.TrimSpace(declMainOnly) {
		t.Fatalf("fixture: origin/develop declares %q, want the stale %q", got, declMainOnly)
	}

	report, err := Check(context.Background(), gitcmd.NewExecutor(), gateOptions(fx, "develop"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Ready || report.Plan.Integration.Name != "develop" {
		t.Fatalf("want READY into develop, got integration %+v\n%s", report.Plan.Integration, FormatCheck(report))
	}
}

// A repository adopting the declaration has none on its default branch yet,
// so the working tree's value gates, as it always did.
func TestCheck_WorkingTreeGatesWhenDefaultBranchDeclaresNone(t *testing.T) {
	t.Parallel()
	for name, commitMain := range map[string]func(string){
		"no declaration":        nil,
		"no integration branch": declareOnMain(t, "branch:\n  taskPattern: [dev/*]\n"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := gateFixture(t, commitMain, declDevelopOnly)
			report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: gateTaskBranch})
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if !report.Ready || report.Plan.Target != fx.Remote+"/develop" {
				t.Fatalf("want READY into %s/develop, got target %q\n%s", fx.Remote, report.Plan.Target, FormatCheck(report))
			}
			if out := FormatCheck(report); strings.Contains(out, "gates the integration") || strings.Contains(out, "default branch declaration") {
				t.Fatalf("report names a default-branch gate that does not exist:\n%s", out)
			}
		})
	}
}

// A default branch whose declaration cannot be used must stay repairable: a
// release would refuse it, so the integration follows the working tree and
// says why.
func TestCheck_UnusableDefaultBranchDeclarationWarnsAndFallsBack(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		commitMain func(t *testing.T) func(string)
		problem    string
	}{
		"unparsable": {
			func(t *testing.T) func(string) {
				t.Helper()
				return declareOnMain(t, "branch: [unclosed\n")
			},
			".gz-git.yaml",
		},
		"symbolic link": {
			func(t *testing.T) func(string) {
				t.Helper()
				return func(dir string) {
					writeRepoFile(t, dir, "real-decl.yaml", declDevelopOnly)
					if err := os.Symlink("real-decl.yaml", filepath.Join(dir, ".gz-git.yaml")); err != nil {
						t.Fatal(err)
					}
				}
			},
			"is a symbolic link",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := gateFixture(t, tc.commitMain(t), declMainOnly)
			report, err := Check(context.Background(), gitcmd.NewExecutor(), gateOptions(fx, "main"))
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if !report.Ready || report.Plan.Integration.Name != "main" {
				t.Fatalf("want READY into main, got integration %+v\n%s", report.Plan.Integration, FormatCheck(report))
			}
			var warned bool
			for _, item := range report.Items {
				if item.Name != "default branch declaration" {
					continue
				}
				warned = item.Status == checkWarn && strings.Contains(item.Detail, tc.problem) &&
					strings.Contains(item.Detail, "the working-tree integrationBranch gates instead")
			}
			if !warned {
				t.Fatalf("no warning names the unusable declaration (%s):\n%s", tc.problem, FormatCheck(report))
			}
		})
	}
}

// A caller-supplied integration branch is not a repository declaration, so
// the default branch's value does not replace it.
func TestCheck_CallerIntegrationConfigIsNotGatedByDefaultBranch(t *testing.T) {
	t.Parallel()
	fx := gateFixture(t, declareOnMain(t, declDevelopOnly), declDevelopOnly)
	opts := gateOptions(fx, "main")
	opts.IntegrationConfig = []string{"main"}
	report, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Plan.Integration.Name != "main" {
		t.Fatalf("integration = %+v, want the caller's main", report.Plan.Integration)
	}
}
