// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

const (
	declWithRemovedKey = "branch:\n  integrationBranch: develop\n  taskPattern: [dev/*]\n  prepareProfile: pnpm-frozen-lockfile-v1\n"
	declClean          = "branch:\n  integrationBranch: develop\n  taskPattern: [dev/*]\n"
)

// releaseDeclarationFixture is the ISSUE-109 deadlock: the primary checkout
// sits on the default branch, whose committed .gz-git.yaml is targetDecl, and
// the release source origin/develop commits sourceDecl. The default branch
// changes only by release, so a removed key in targetDecl can be deleted only
// by the release that targetDecl would otherwise block.
func releaseDeclarationFixture(t *testing.T, targetDecl, sourceDecl string) *testutil.WorktreeOrigin {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeRepoFile(t, fx.Clone, ".gz-git.yaml", targetDecl)
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "declare policy")
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/main")
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", "develop", fx.Remote+"/main")
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", sourceDecl)
	writeRepoFile(t, fx.Worktree, "develop-only.txt", "release\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "develop work")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "checkout", "--detach")
	runGit(t, fx.Worktree, "branch", "-D", "develop")
	runGit(t, fx.Clone, "fetch", fx.Remote)
	return fx
}

func releaseDeclarationOptions(t *testing.T, fx *testutil.WorktreeOrigin) CheckOptions {
	t.Helper()
	opts := releaseRemoteOptions(fx)
	opts.ExpectSource = strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/develop"))
	return opts
}

// The ISSUE-109 deadlock: the target still carries a removed key next to
// integrationBranch: develop, and the clean source is the release that
// deletes it. The target is read leniently for its integration branch only.
func TestCheck_ReleaseReadsDeclarationFromSourceCommit(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declWithRemovedKey, declClean)
	opts := releaseDeclarationOptions(t, fx)
	report, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Ready {
		t.Fatalf("want READY, got\n%s", FormatCheck(report))
	}
	if report.Declaration.Commit != opts.ExpectSource {
		t.Fatalf("declaration commit = %q, want release source %s", report.Declaration.Commit, opts.ExpectSource)
	}
	if want := []string{"dev/*"}; len(report.Declaration.Patterns) != 1 || report.Declaration.Patterns[0] != want[0] {
		t.Fatalf("declaration patterns = %v, want %v", report.Declaration.Patterns, want)
	}
	if report.Plan.Integration.Name != "develop" || !strings.HasPrefix(report.Plan.Integration.Source, SourceConfigPrefix) {
		t.Fatalf("integration = %+v, want develop from config", report.Plan.Integration)
	}
	out := FormatCheck(report)
	targetSHA := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/main"))
	for _, want := range []string{
		"declaration — " + report.Declaration.Source + " at release source commit " + opts.ExpectSource[:12],
		"integrationBranch source=develop, target=develop (from .gz-git.yaml at target commit " + targetSHA[:12] + ", gates the release)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("report does not name the declaration provenance %q:\n%s", want, out)
		}
	}
}

// pushRemoteBranch commits decl as .gz-git.yaml on a branch cut from the
// remote default branch and pushes it, returning the remote-tracking name.
func pushRemoteBranch(t *testing.T, fx *testutil.WorktreeOrigin, branch, decl string) string {
	t.Helper()
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", branch, fx.Remote+"/main")
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", decl)
	writeRepoFile(t, fx.Worktree, branch+".txt", "work\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", branch+" work")
	runGit(t, fx.Worktree, "push", fx.Remote, branch)
	runGit(t, fx.Worktree, "checkout", "--detach")
	runGit(t, fx.Worktree, "branch", "-D", branch)
	runGit(t, fx.Clone, "fetch", fx.Remote)
	return fx.Remote + "/" + branch
}

// A source branch that declares itself the integration branch must not
// authorize its own promotion: the gate reads the target's declaration, and
// with none declared there the heuristic refuses rather than borrowing the
// source's value.
func TestCheck_ReleaseSourceCannotDeclareItselfIntegrationBranch(t *testing.T) {
	t.Parallel()
	selfDeclared := "branch:\n  integrationBranch: feature-x\n  taskPattern: [dev/*]\n"
	for name, tc := range map[string]struct{ targetDecl, refusal string }{
		"target declares develop": {declClean, "release promotion must start from origin/develop"},
		// The heuristic resolves the remote HEAD (main), the release target.
		"target declares none": {"branch:\n  taskPattern: [dev/*]\n", "against the integration branch"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := releaseDeclarationFixture(t, tc.targetDecl, declClean)
			opts := releaseRemoteOptions(fx)
			opts.Branch = pushRemoteBranch(t, fx, "feature-x", selfDeclared)
			main := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/main"))
			report, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
			if err == nil {
				t.Fatalf("a source declaring itself the integration branch was not refused:\n%s", FormatCheck(report))
			}
			if !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("error %q does not contain %q", err, tc.refusal)
			}
			if got := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/main")); got != main {
				t.Fatalf("target moved to %s", got)
			}
		})
	}
}

// A source that changes integrationBranch does not move the gate: the
// release keeps promoting the branch the target declares, and the report
// names both values.
func TestCheck_ReleaseGatesOnTargetIntegrationBranch(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declClean, "branch:\n  integrationBranch: staging\n  taskPattern: [dev/*]\n")
	opts := releaseDeclarationOptions(t, fx)
	report, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Ready {
		t.Fatalf("want READY, got\n%s", FormatCheck(report))
	}
	if report.Plan.Integration.Name != "develop" {
		t.Fatalf("integration = %+v, want the target's develop", report.Plan.Integration)
	}
	if got := report.Declaration.IntegrationBranch; len(got) != 1 || got[0] != "staging" {
		t.Fatalf("source declaration integrationBranch = %v, want [staging]", got)
	}
	if out := FormatCheck(report); !strings.Contains(out, "integrationBranch source=staging, target=develop") {
		t.Fatalf("report does not name both integration branches:\n%s", out)
	}

	// The branch the source names is not promotable by its own say-so.
	opts = releaseRemoteOptions(fx)
	opts.Branch = pushRemoteBranch(t, fx, "staging", "branch:\n  integrationBranch: staging\n  taskPattern: [dev/*]\n")
	if report, err := Check(context.Background(), gitcmd.NewExecutor(), opts); err == nil && report.Ready {
		t.Fatalf("release from the source-declared staging branch was READY:\n%s", FormatCheck(report))
	}
}

// The target declaration is read for its integration branch only, but a
// syntax error there still fails the release and names the target commit.
func TestCheck_ReleaseRejectsUnparsableTargetDeclaration(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, "branch: [unclosed\n", declClean)
	opts := releaseDeclarationOptions(t, fx)
	main := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/main"))
	_, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
	if err == nil {
		t.Fatal("an unparsable target declaration must fail the release")
	}
	for _, want := range []string{"release target declaration", ".gz-git.yaml", "commit " + main[:12]} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestCheck_ReleaseWithoutExpectSourceReadsSourceRef(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declWithRemovedKey, declClean)
	report, err := Check(context.Background(), gitcmd.NewExecutor(), releaseRemoteOptions(fx))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	develop := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/develop"))
	if !report.Ready || report.Declaration.Commit != develop {
		t.Fatalf("want READY from %s, got commit %q\n%s", develop, report.Declaration.Commit, FormatCheck(report))
	}
}

func TestCheck_ReleaseRejectsRemovedKeyInSourceCommit(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declClean, declWithRemovedKey)
	opts := releaseDeclarationOptions(t, fx)
	_, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
	if err == nil {
		t.Fatal("a release source that still carries a removed key must not pass")
	}
	for _, want := range []string{"branch.prepareProfile was removed", ".gz-git.yaml", "commit " + opts.ExpectSource[:12]} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestCheck_NonReleaseStillRejectsRemovedKeyInWorkingTree(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declWithRemovedKey, declClean)
	runGit(t, fx.Clone, "checkout", "-b", "dev/task")
	_, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Clone})
	if err == nil || !strings.Contains(err.Error(), "branch.prepareProfile was removed") {
		t.Fatalf("non-release check must read the working tree and reject its removed key, got %v", err)
	}
	if strings.Contains(err.Error(), "commit") {
		t.Fatalf("non-release error must name the working-tree file, not a commit: %v", err)
	}
}

func TestRun_ReleaseReadsDeclarationFromSourceCommit(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declWithRemovedKey, declClean)
	opts := releaseDeclarationOptions(t, fx)
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: opts})
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, FormatRun(report))
	}
	if report.SHA != opts.ExpectSource {
		t.Fatalf("integrated %s, want %s", report.SHA, opts.ExpectSource)
	}
	// Reclaim read the source declaration too: a working-tree read would
	// have skipped reclaim with the removed-key error.
	if strings.Contains(report.Reclaim.Skipped, "was removed") {
		t.Fatalf("reclaim read a removed key: %+v", report.Reclaim)
	}
	if got := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", "HEAD")); got != opts.ExpectSource {
		t.Fatalf("default checkout HEAD = %s, want fast-forward to %s", got, opts.ExpectSource)
	}
}

// TestCommittedDeclarationFiles_ReadsBlobsAndRefusesRootSymlink reads the
// declaration blobs by object ID and refuses a repo-root declaration committed
// as a symbolic link, whose bytes would be a link target.
func TestCommittedDeclarationFiles_ReadsBlobsAndRefusesRootSymlink(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeRepoFile(t, fx.Clone, ".gz-git.yaml", declClean)
	if err := os.MkdirAll(filepath.Join(fx.Clone, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, fx.Clone, "sub/.gz-git.yml", "branch:\n  taskPattern: [nested/*]\n")
	writeRepoFile(t, fx.Clone, "sub/other.yaml", declClean)
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "declare policy")
	g := newGitRepo(gitcmd.NewExecutor(), fx.Clone)
	head := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", "HEAD"))

	files, err := committedDeclarationFiles(context.Background(), g, head)
	if err != nil {
		t.Fatalf("committedDeclarationFiles: %v", err)
	}
	got := map[string]string{}
	for _, f := range files {
		got[f.Path] = string(f.Data)
	}
	if len(got) != 2 || got[".gz-git.yaml"] != declClean || got["sub/.gz-git.yml"] == "" {
		t.Fatalf("files = %v", got)
	}

	runGit(t, fx.Clone, "rm", "-q", ".gz-git.yaml")
	if err := os.Symlink("sub/.gz-git.yml", filepath.Join(fx.Clone, ".gz-git.yaml")); err != nil {
		t.Fatal(err)
	}
	runGit(t, fx.Clone, "add", ".gz-git.yaml")
	runGit(t, fx.Clone, "commit", "-q", "-m", "link policy")
	head = strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", "HEAD"))
	if _, err := committedDeclarationFiles(context.Background(), g, head); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("err = %v, want symbolic link refusal", err)
	}
	if _, err := committedDeclarationFiles(context.Background(), g, "develop"); err == nil {
		t.Fatal("a ref name must not be accepted as the release source commit")
	}
}

// advanceRemoteDevelop pushes a commit to the remote develop that sets the
// declaration to decl, without fetching it into the clone. It returns that
// commit, which exists only on the remote until the next fetch.
func advanceRemoteDevelop(t *testing.T, fx *testutil.WorktreeOrigin, decl string) string {
	t.Helper()
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", "develop", fx.Remote+"/develop")
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", decl)
	runGit(t, fx.Worktree, "commit", "-am", "drop removed key")
	runGit(t, fx.Worktree, "push", fx.Remote, "develop")
	sha := strings.TrimSpace(gitOutput(t, fx.Worktree, "rev-parse", "HEAD"))
	runGit(t, fx.Worktree, "checkout", "--detach")
	runGit(t, fx.Worktree, "branch", "-D", "develop")
	return sha
}

// TestCheck_ReleaseStaleLocalSourceReadsFetchedCommit is the stale-clone
// shape of ISSUE-109: the local origin/develop still carries the removed key,
// the remote already deleted it, and the approved source exists only on the
// remote. Nothing may read the stale local ref before the fetch.
func TestCheck_ReleaseStaleLocalSourceReadsFetchedCommit(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declWithRemovedKey, declWithRemovedKey)
	approved := advanceRemoteDevelop(t, fx, declClean)
	opts := releaseRemoteOptions(fx)
	opts.ExpectSource = approved
	report, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Ready || report.Declaration.Commit != approved {
		t.Fatalf("want READY from %s, got commit %q\n%s", approved, report.Declaration.Commit, FormatCheck(report))
	}
}

func TestCheck_ReleaseSourceRefAppearsOnlyAfterFetch(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declWithRemovedKey, declClean)
	runGit(t, fx.Clone, "update-ref", "-d", "refs/remotes/"+fx.Remote+"/develop")
	report, err := Check(context.Background(), gitcmd.NewExecutor(), releaseRemoteOptions(fx))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	develop := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/develop"))
	if !report.Ready || report.Declaration.Commit != develop {
		t.Fatalf("want READY from %s, got commit %q\n%s", develop, report.Declaration.Commit, FormatCheck(report))
	}
}

func TestCheck_ReleaseRequiresExplicitTarget(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declWithRemovedKey, declClean)
	opts := releaseDeclarationOptions(t, fx)
	opts.Target = ""
	if _, err := Check(context.Background(), gitcmd.NewExecutor(), opts); !errors.Is(err, ErrReleaseNeedsTarget) {
		t.Fatalf("Check err = %v, want ErrReleaseNeedsTarget", err)
	}
	if _, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: opts}); !errors.Is(err, ErrReleaseNeedsTarget) {
		t.Fatalf("Run err = %v, want ErrReleaseNeedsTarget", err)
	}
}

// TestCheck_ReleaseWithCallerIntegrationConfigStillRejectsRemovedKey: a
// caller-supplied integration branch replaces the declared one, but the
// committed declaration is still read and still guarded.
func TestCheck_ReleaseWithCallerIntegrationConfigStillRejectsRemovedKey(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declClean, declWithRemovedKey)
	opts := releaseDeclarationOptions(t, fx)
	opts.IntegrationConfig = []string{"develop"}
	_, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
	if err == nil || !strings.Contains(err.Error(), "branch.prepareProfile was removed") || !strings.Contains(err.Error(), "commit "+opts.ExpectSource[:12]) {
		t.Fatalf("err = %v, want the removed-key refusal at the source commit", err)
	}
}

func TestRun_ReleaseRefusesRemovedKeyInSourceCommit(t *testing.T) {
	t.Parallel()
	fx := releaseDeclarationFixture(t, declClean, declWithRemovedKey)
	opts := releaseDeclarationOptions(t, fx)
	mainBefore := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/main"))
	_, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{CheckOptions: opts})
	if err == nil || !strings.Contains(err.Error(), "branch.prepareProfile was removed") {
		t.Fatalf("Run err = %v, want the removed-key refusal", err)
	}
	runGit(t, fx.Clone, "fetch", fx.Remote)
	if got := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", fx.Remote+"/main")); got != mainBefore {
		t.Fatalf("refused release moved main from %s to %s", mainBefore, got)
	}
}

// TestCheckedPlanDeclaration_ReusesCheckRead: a step after Check that asks
// for the release commit Check read gets that declaration back unread.
func TestCheckedPlanDeclaration_ReusesCheckRead(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Clone)
	head := strings.TrimSpace(gitOutput(t, fx.Clone, "rev-parse", "HEAD"))
	check := &CheckReport{Plan: TargetPlan{Release: true}, Declaration: config.TaskPatternDecl{Commit: head, Patterns: []string{"read/*"}}}
	decl, err := checkedPlanDeclaration(context.Background(), g, fx.Clone, check, head)
	if err != nil || len(decl.Patterns) != 1 || decl.Patterns[0] != "read/*" {
		t.Fatalf("decl = %+v, err = %v; want Check's declaration reused", decl, err)
	}
}

func TestRunProvenance_RecordsUnreadableDeclaration(t *testing.T) {
	t.Parallel()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	writeRepoFile(t, fx.Clone, ".gz-git.yaml", declWithRemovedKey)
	p := runProvenance(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Clone), nil)
	if len(p.DeclarationPatterns) != 0 || len(p.DeclarationFacts) != 1 || !strings.Contains(p.DeclarationFacts[0], "was removed") {
		t.Fatalf("provenance = %+v, want one unreadable-declaration fact", p)
	}
}
