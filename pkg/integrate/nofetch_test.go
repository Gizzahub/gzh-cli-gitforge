// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

const noFetchTask = "dev/actor/feat/task"

// unreadableRemote points the remote's fetch URL at a path that does not
// exist while keeping pushes on the real bare origin. Every network read
// (fetch, ls-remote) then fails, so a passing no-fetch run proves none
// happened, and pushes still reach the remote the test inspects.
func unreadableRemote(t *testing.T, fx *testutil.WorktreeOrigin) {
	t.Helper()
	runGit(t, fx.Clone, "remote", "set-url", fx.Remote, filepath.Join(t.TempDir(), "unreadable.git"))
	runGit(t, fx.Clone, "config", "remote."+fx.Remote+".pushurl", fx.Origin)
}

func TestRun_NoFetchIntegratesAndReclaimsWithoutReadingRemote(t *testing.T) {
	fx := runFixture(t, "dev/*")
	unreadableRemote(t, fx)
	taskSHA := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")

	// Control: the same fixture without --no-fetch must fail on the fetch,
	// otherwise the unreadable URL would not prove anything below.
	if _, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{RepoPath: fx.Worktree, Branch: noFetchTask},
	}); err == nil || !strings.Contains(err.Error(), "fetch") {
		t.Fatalf("control run without --no-fetch: err = %v, want a fetch failure", err)
	}

	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{RepoPath: fx.Worktree, Branch: noFetchTask, NoFetch: true},
	})
	if err != nil {
		t.Fatalf("Run --no-fetch: %v\n%s", err, FormatRun(report))
	}
	if !report.Integrated || report.Reclaim.Incomplete() || report.Reclaim.Skipped != "" {
		t.Fatalf("want integrated and fully reclaimed:\n%s", FormatRun(report))
	}
	out := FormatRun(report)
	for _, want := range []string{"SKIP  fetch — --no-fetch: freshness and merge-tree judged from local ref origin/develop", "without fetching", "(local ref, not fetched)"} {
		if !strings.Contains(out, want) {
			t.Errorf("run transcript missing %q:\n%s", want, out)
		}
	}
	if got := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != taskSHA {
		t.Fatalf("origin develop = %s, want integrated %s", got, taskSHA)
	}
	if _, err := os.Stat(fx.Worktree); !os.IsNotExist(err) {
		t.Fatalf("task worktree stat = %v, want removed", err)
	}
	if refExists(t, fx.Clone, "refs/heads/"+noFetchTask) {
		t.Fatal("local task branch should be deleted")
	}
	if refExists(t, fx.Origin, "refs/heads/"+noFetchTask) {
		t.Fatal("remote task branch should be deleted")
	}
}

func TestRun_NoFetchPushRejectedWhenRemoteTargetMoved(t *testing.T) {
	fx := runFixture(t, "dev/*")
	checkedTarget := gitOutput(t, fx.Clone, "rev-parse", "refs/remotes/origin/develop")

	// Another writer advances develop on the real remote. The local
	// remote-tracking ref stays at the old tip because nothing fetches.
	other := filepath.Join(t.TempDir(), "other")
	runGit(t, "", "clone", "--branch", "develop", fx.Origin, other)
	runGit(t, other, "config", "user.email", "other@example.com")
	runGit(t, other, "config", "user.name", "Other")
	writeFile(t, other, "other.txt", "other\n")
	runGit(t, other, "add", ".")
	runGit(t, other, "commit", "-m", "someone else landed first")
	runGit(t, other, "push", "origin", "develop")
	movedTarget := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop")
	if movedTarget == checkedTarget {
		t.Fatal("precondition: remote develop did not move")
	}
	unreadableRemote(t, fx)

	// The check cannot see the move: that is the documented limit of
	// --no-fetch, and why the push lease is the freshness guard.
	check, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: noFetchTask, NoFetch: true})
	if err != nil {
		t.Fatalf("Check --no-fetch: %v", err)
	}
	if !check.Ready || check.Plan.TargetSHA != checkedTarget {
		t.Fatalf("want READY against stale local ref %s:\n%s", checkedTarget, FormatCheck(check))
	}

	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{RepoPath: fx.Worktree, Branch: noFetchTask, NoFetch: true},
	})
	if err == nil {
		t.Fatalf("Run --no-fetch against a moved remote must fail:\n%s", FormatRun(report))
	}
	if !strings.Contains(err.Error(), "--no-fetch judged origin/develop from local refs") {
		t.Fatalf("rejection must name the stale local judgement: %v", err)
	}
	if report == nil || report.Integrated || len(report.Reclaim.Done) > 0 {
		t.Fatalf("nothing may integrate or reclaim after a rejected push: %+v", report)
	}
	if got := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != movedTarget {
		t.Fatalf("origin develop = %s, want untouched %s", got, movedTarget)
	}
	if _, err := os.Stat(fx.Worktree); err != nil {
		t.Fatalf("task worktree must remain: %v", err)
	}
	assertRef(t, fx.Clone, "refs/heads/"+noFetchTask)
	assertRef(t, fx.Origin, "refs/heads/"+noFetchTask)
}

func TestCheck_WithoutNoFetchPrintsNoFetchRow(t *testing.T) {
	fx := runFixture(t, "dev/*")
	check, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: noFetchTask})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	out := FormatCheck(check)
	if strings.Contains(out, "--no-fetch") || strings.Contains(out, "not fetched") {
		t.Fatalf("default check must not mention no-fetch:\n%s", out)
	}
}

func TestReclaimRemoteBranch_NoFetchDoesNotConfirmWithLsRemote(t *testing.T) {
	fx := runFixture(t, "dev/*")
	taskSHA := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	// The remote branch is already gone but the local tracking ref remains,
	// so the leased delete is refused. Default reclaim would ls-remote to
	// call that already-deleted; --no-fetch must not read the remote.
	runGit(t, fx.Origin, "update-ref", "-d", "refs/heads/"+noFetchTask)
	unreadableRemote(t, fx)

	var out ReclaimResult
	ok := reclaimRemoteBranch(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Clone), reclaimOpts{
		Branch:  noFetchTask,
		Remote:  fx.Remote,
		TaskSHA: taskSHA,
		NoFetch: true,
	}, &out)
	if ok || !out.Incomplete() {
		t.Fatalf("want an incomplete reclaim, got ok=%v %+v", ok, out)
	}
	if !strings.Contains(strings.Join(out.Failed, "\n"), "--no-fetch: not confirmed with ls-remote") {
		t.Fatalf("failure must say ls-remote was skipped: %+v", out)
	}
}

// gitEnvShim puts a git on PATH that records GIT_NO_LAZY_FETCH for every
// invocation before running the real git, and returns the log it writes.
func gitEnvShim(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("look up git: %v", err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "git-env.log")
	script := "#!/bin/sh\nprintf '%s %s\\n' \"${GIT_NO_LAZY_FETCH-unset}\" \"$1\" >> '" + log + "'\nexec '" + realGit + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o700); err != nil { //nolint:gosec // the shim must be executable
		t.Fatalf("write git shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_NO_LAZY_FETCH", "")
	if err := os.Unsetenv("GIT_NO_LAZY_FETCH"); err != nil {
		t.Fatalf("unset GIT_NO_LAZY_FETCH: %v", err)
	}
	return log
}

// shimLines reads and clears the shim log, failing if git never ran.
func shimLines(t *testing.T, log string) []string {
	t.Helper()
	raw, err := os.ReadFile(log) //nolint:gosec // test-owned path
	if err != nil {
		t.Fatalf("read git shim log: %v", err)
	}
	if err := os.Remove(log); err != nil {
		t.Fatalf("clear git shim log: %v", err)
	}
	lines := splitNonEmpty(string(raw))
	if len(lines) == 0 {
		t.Fatal("git shim recorded no invocations")
	}
	return lines
}

// A partial clone fetches missing objects on demand; --no-fetch must switch
// that off for every git it starts, check and run alike, and only then.
func TestRun_NoFetchGitSubprocessesDisableLazyFetch(t *testing.T) {
	fx := runFixture(t, "dev/*")
	unreadableRemote(t, fx)
	log := gitEnvShim(t)

	_, _ = Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: noFetchTask})
	for _, line := range shimLines(t, log) {
		if !strings.HasPrefix(line, "unset ") {
			t.Fatalf("default check must not set GIT_NO_LAZY_FETCH: %q", line)
		}
	}

	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{RepoPath: fx.Worktree, Branch: noFetchTask, NoFetch: true},
	})
	lines := shimLines(t, log)
	if err != nil || !report.Integrated || report.Reclaim.Incomplete() {
		t.Fatalf("Run --no-fetch: %v\n%s", err, FormatRun(report))
	}
	seen := map[string]bool{}
	for _, line := range lines {
		if !strings.HasPrefix(line, "1 ") {
			t.Fatalf("--no-fetch git ran without GIT_NO_LAZY_FETCH=1: %q\nall:\n%s", line, strings.Join(lines, "\n"))
		}
		seen[strings.TrimPrefix(line, "1 ")] = true
	}
	// Both halves went through the shim: check's merge-tree and run's push.
	for _, sub := range []string{"merge-tree", "push"} {
		if !seen[sub] {
			t.Fatalf("shim never saw git %s; recorded:\n%s", sub, strings.Join(lines, "\n"))
		}
	}
}

// A lease is stricter than a fast-forward. When the remote target was rewound
// to an ancestor of the checked tip, a plain push would land as a
// fast-forward from there; the lease names the checked tip and must refuse.
func TestRun_NoFetchLeaseRefusesRemoteRewoundToAncestor(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "branch", "develop")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
	ancestor := gitOutput(t, fx.Clone, "rev-parse", "develop")
	runGit(t, fx.Worktree, "checkout", "-B", "develop-next", "develop")
	writeFile(t, fx.Worktree, "develop.txt", "second\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "develop second")
	runGit(t, fx.Worktree, "push", fx.Remote, "HEAD:develop")
	checkedTarget := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	runGit(t, fx.Worktree, "checkout", "-B", noFetchTask, checkedTarget)
	writeFile(t, fx.Worktree, "task.txt", "task\n")
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: dev/*\n")
	writeGateMakefile(t, fx.Worktree)
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "task work")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	if got := gitOutput(t, fx.Clone, "rev-parse", "refs/remotes/origin/develop"); got != checkedTarget {
		t.Fatalf("precondition: local origin/develop = %s, want %s", got, checkedTarget)
	}

	runGit(t, fx.Origin, "update-ref", "refs/heads/develop", ancestor)
	unreadableRemote(t, fx)

	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{RepoPath: fx.Worktree, Branch: noFetchTask, NoFetch: true},
	})
	if err == nil {
		t.Fatalf("Run --no-fetch against a rewound remote must fail:\n%s", FormatRun(report))
	}
	if !strings.Contains(err.Error(), "--no-fetch judged origin/develop from local refs") {
		t.Fatalf("rejection must come from the push: %v", err)
	}
	if report == nil || !report.Check.Ready || report.Check.Plan.TargetSHA != checkedTarget || report.Integrated {
		t.Fatalf("want READY against %s and nothing integrated:\n%s", checkedTarget, FormatRun(report))
	}
	if got := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != ancestor {
		t.Fatalf("origin develop = %s, want still rewound to %s", got, ancestor)
	}
	assertRef(t, fx.Origin, "refs/heads/"+noFetchTask)
}
