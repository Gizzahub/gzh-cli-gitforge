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
