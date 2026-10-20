// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

const refreshTask = "dev/actor/feat/task"

type refreshFixture struct {
	*testutil.WorktreeOrigin
	// Other is a second clone that moves origin/develop the way another
	// session's landing would.
	Other string
}

// newRefreshFixture builds a target develop whose committed .gz-git.yaml
// does or does not opt in, and a pushed task branch in a linked worktree.
func newRefreshFixture(t *testing.T, optIn bool) *refreshFixture {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Worktree, "checkout", "-B", "develop")
	body := "branch:\n  integrationBranch: develop\n"
	if optIn {
		body += "  refreshUnderTargetLock: true\n"
	}
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", body)
	writeRepoFile(t, fx.Worktree, "Makefile", "check:\n\t@true\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "target declarations")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "checkout", "-B", refreshTask, "develop")
	writeFile(t, fx.Worktree, "task.txt", "task\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "task work")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")

	other := filepath.Join(t.TempDir(), "other")
	runGit(t, "", "clone", "-b", "develop", fx.Origin, other)
	runGit(t, other, "config", "user.email", "other@test.com")
	runGit(t, other, "config", "user.name", "Other")
	runGit(t, other, "config", "commit.gpgsign", "false")
	return &refreshFixture{WorktreeOrigin: fx, Other: other}
}

// moveTarget lands one commit on origin/develop from the other clone.
func (fx *refreshFixture) moveTarget(t *testing.T, name, body string) string {
	t.Helper()
	runGit(t, fx.Other, "pull", "--ff-only", "origin", "develop")
	writeFile(t, fx.Other, name, body)
	runGit(t, fx.Other, "add", name)
	runGit(t, fx.Other, "commit", "-m", "land "+name)
	runGit(t, fx.Other, "push", "origin", "develop")
	return gitOutput(t, fx.Other, "rev-parse", "HEAD")
}

// run targets <remote>/develop, the spelling CE passes (integrationTarget).
func (fx *refreshFixture) run(t *testing.T, refresh bool) (*RunReport, error) {
	t.Helper()
	return Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions:           CheckOptions{RepoPath: fx.Worktree, Branch: refreshTask, Target: fx.Remote + "/develop"},
		RefreshUnderTargetLock: refresh,
	})
}

func (fx *refreshFixture) assertTaskUnchanged(t *testing.T, before string) {
	t.Helper()
	if head := gitOutput(t, fx.Worktree, "rev-parse", "HEAD"); head != before {
		t.Fatalf("task worktree HEAD moved: %s -> %s", before, head)
	}
	if remote := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/"+refreshTask); remote != before {
		t.Fatalf("remote task branch moved: %s -> %s", before, remote)
	}
	if op, err := gitOperationInProgress(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree)); err != nil || op != "" {
		t.Fatalf("operation left in progress: %q %v", op, err)
	}
}

func TestRun_RefreshUnderTargetLockRebasesStaleBranchAndLands(t *testing.T) {
	fx := newRefreshFixture(t, true)
	before := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	target := fx.moveTarget(t, "other.txt", "other\n")

	report, err := fx.run(t, true)
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, FormatRun(report))
	}
	if !report.Integrated || len(report.Refreshes) != 1 {
		t.Fatalf("want integrated after one refresh: %+v\n%s", report.Refreshes, FormatRun(report))
	}
	r := report.Refreshes[0]
	if r.Reason != refreshReasonStaleBase || r.BeforeSHA != before || r.TargetSHA != target || r.AfterSHA == before {
		t.Fatalf("refresh record = %+v (before %s, target %s)", r, before, target)
	}
	landed := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop")
	if landed != r.AfterSHA {
		t.Fatalf("landed %s, want refreshed head %s", landed, r.AfterSHA)
	}
	if parent := gitOutput(t, fx.Origin, "rev-parse", landed+"^"); parent != target {
		t.Fatalf("refreshed commit parent = %s, want target %s", parent, target)
	}
	if !strings.HasPrefix(report.Printed[0], "REFRESHED "+refreshTask+" "+before) {
		t.Fatalf("refresh not printed first: %q", report.Printed)
	}
	p := BuildRunProtocol(report, nil, 0)
	if p.TaskSHABeforeRefresh != before || len(p.Refreshes) != 1 || p.TaskSHA != r.AfterSHA {
		t.Fatalf("protocol refresh fields: before=%q refreshes=%+v task=%q", p.TaskSHABeforeRefresh, p.Refreshes, p.TaskSHA)
	}
}

func TestRun_RefreshUnderTargetLockNeedsFlagAndTargetOptIn(t *testing.T) {
	for _, tc := range []struct {
		name           string
		optIn, refresh bool
	}{
		{"flag off", true, false},
		{"target not opted in", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newRefreshFixture(t, tc.optIn)
			before := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
			fx.moveTarget(t, "other.txt", "other\n")
			report, err := fx.run(t, tc.refresh)
			if err == nil || report == nil || report.Integrated || len(report.Refreshes) != 0 {
				t.Fatalf("stale run must refuse as before: err=%v report=%+v", err, report)
			}
			if report.Check == nil || !report.Check.StaleBase {
				t.Fatalf("stale base not reported: %+v", report.Check)
			}
			fx.assertTaskUnchanged(t, before)
		})
	}
}

func TestRun_RefreshUnderTargetLockOptInOnTaskBranchOnlyIsIgnored(t *testing.T) {
	fx := newRefreshFixture(t, false)
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  refreshUnderTargetLock: true\n")
	runGit(t, fx.Worktree, "commit", "-am", "grant myself a refresh")
	runGit(t, fx.Worktree, "push")
	before := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	fx.moveTarget(t, "other.txt", "other\n")
	report, err := fx.run(t, true)
	if err == nil || len(report.Refreshes) != 0 {
		t.Fatalf("task-branch opt-in must not refresh: err=%v refreshes=%+v", err, report.Refreshes)
	}
	fx.assertTaskUnchanged(t, before)
}

func TestRun_RefreshUnderTargetLockKeepsPinnedSource(t *testing.T) {
	fx := newRefreshFixture(t, true)
	before := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	fx.moveTarget(t, "other.txt", "other\n")
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions:           CheckOptions{RepoPath: fx.Worktree, Branch: refreshTask, Target: fx.Remote + "/develop", ExpectSource: before},
		RefreshUnderTargetLock: true,
	})
	if err == nil || report == nil || report.Integrated || len(report.Refreshes) != 0 {
		t.Fatalf("a pinned source must not be rewritten: err=%v report=%+v", err, report)
	}
	fx.assertTaskUnchanged(t, before)
}

func TestRun_RefreshUnderTargetLockConflictAbortsAndRefuses(t *testing.T) {
	fx := newRefreshFixture(t, true)
	before := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	fx.moveTarget(t, "task.txt", "theirs\n")
	report, err := fx.run(t, true)
	if !errors.Is(err, ErrRefreshRefused) || !strings.Contains(err.Error(), "task.txt") {
		t.Fatalf("want refused conflict naming task.txt, got %v", err)
	}
	if report == nil || report.Integrated || len(report.Refreshes) != 0 {
		t.Fatalf("conflict report = %+v", report)
	}
	if !strings.HasPrefix(err.Error(), "not ready") {
		t.Fatalf("refusal must read as not ready (exit 1): %v", err)
	}
	fx.assertTaskUnchanged(t, before)
}

func TestRun_RefreshUnderTargetLockRefusesUnsafeWorktrees(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		prepare    func(t *testing.T, fx *refreshFixture)
	}{
		{"dirty", "uncommitted", func(t *testing.T, fx *refreshFixture) {
			t.Helper()
			writeFile(t, fx.Worktree, "scratch.txt", "untracked\n")
		}},
		{"bisect in progress", "bisect", func(t *testing.T, fx *refreshFixture) {
			t.Helper()
			runGit(t, fx.Worktree, "bisect", "start")
		}},
		{"unpushed", "not pushed", func(t *testing.T, fx *refreshFixture) {
			t.Helper()
			runGit(t, fx.Worktree, "commit", "--allow-empty", "-m", "local only")
		}},
		{"merge commit", "merge commit", func(t *testing.T, fx *refreshFixture) {
			t.Helper()
			runGit(t, fx.Worktree, "checkout", "-b", "side", "HEAD~1")
			writeFile(t, fx.Worktree, "side.txt", "side\n")
			runGit(t, fx.Worktree, "add", "side.txt")
			runGit(t, fx.Worktree, "commit", "-m", "side")
			runGit(t, fx.Worktree, "checkout", refreshTask)
			runGit(t, fx.Worktree, "merge", "--no-ff", "-m", "merge side", "side")
			runGit(t, fx.Worktree, "push")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newRefreshFixture(t, true)
			fx.moveTarget(t, "other.txt", "other\n")
			tc.prepare(t, fx)
			before := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
			remoteBefore := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/"+refreshTask)
			report, err := fx.run(t, true)
			if err == nil || report == nil || report.Integrated || len(report.Refreshes) != 0 {
				t.Fatalf("unsafe worktree must refuse: err=%v report=%+v", err, report)
			}
			// Readiness may refuse a dirty or unpushed branch before the
			// refresh is attempted; either way it names the condition.
			if !strings.Contains(err.Error()+FormatCheck(report.Check), tc.want) {
				t.Fatalf("refusal does not name %q: %v\n%s", tc.want, err, FormatCheck(report.Check))
			}
			if head := gitOutput(t, fx.Worktree, "rev-parse", "HEAD"); head != before {
				t.Fatalf("task HEAD moved: %s -> %s", before, head)
			}
			if remote := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/"+refreshTask); remote != remoteBefore {
				t.Fatalf("remote task branch moved: %s -> %s", remoteBefore, remote)
			}
		})
	}
}

func TestRun_RefreshUnderTargetLockRefusesPrimaryCheckout(t *testing.T) {
	fx := newRefreshFixture(t, true)
	runGit(t, fx.Worktree, "checkout", "--detach")
	runGit(t, fx.Clone, "checkout", refreshTask)
	before := gitOutput(t, fx.Clone, "rev-parse", "HEAD")
	fx.moveTarget(t, "other.txt", "other\n")
	g := newGitRepo(gitcmd.NewExecutor(), fx.Clone)
	_, err := refreshTaskBranch(context.Background(), g, TargetPlan{
		Branch: refreshTask, BranchSHA: before, Remote: fx.Remote,
		TargetSHA: gitOutput(t, fx.Other, "rev-parse", "HEAD"),
	}, refreshReasonStaleBase)
	if !errors.Is(err, ErrRefreshRefused) || !strings.Contains(err.Error(), "primary checkout") {
		t.Fatalf("want primary checkout refusal, got %v", err)
	}
	if head := gitOutput(t, fx.Clone, "rev-parse", "HEAD"); head != before {
		t.Fatalf("primary checkout moved: %s -> %s", before, head)
	}
}

func TestRefreshTaskBranch_LeaseRaceRestoresLocalBranch(t *testing.T) {
	fx := newRefreshFixture(t, true)
	before := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	target := fx.moveTarget(t, "other.txt", "other\n")
	runGit(t, fx.Worktree, "fetch", fx.Remote)
	// Another writer moves the remote task branch after this worktree last
	// fetched it, so the tracking ref still names the checked commit.
	runGit(t, fx.Other, "fetch", "origin", refreshTask)
	runGit(t, fx.Other, "checkout", "-b", "race", "FETCH_HEAD")
	runGit(t, fx.Other, "commit", "--allow-empty", "-m", "racing push")
	runGit(t, fx.Other, "push", "origin", "HEAD:refs/heads/"+refreshTask)
	raced := gitOutput(t, fx.Other, "rev-parse", "HEAD")

	g := newGitRepo(gitcmd.NewExecutor(), fx.Clone)
	_, err := refreshTaskBranch(context.Background(), g, TargetPlan{
		Branch: refreshTask, BranchSHA: before, Remote: fx.Remote, TargetSHA: target,
	}, refreshReasonStaleBase)
	if !errors.Is(err, ErrRefreshRefused) || !strings.Contains(err.Error(), "lease push") {
		t.Fatalf("want lease refusal, got %v", err)
	}
	if head := gitOutput(t, fx.Worktree, "rev-parse", "HEAD"); head != before {
		t.Fatalf("local task branch not restored: %s -> %s", before, head)
	}
	if remote := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/"+refreshTask); remote != raced {
		t.Fatalf("racing push overwritten: remote %s, want %s", remote, raced)
	}
}

// moveTargetGate is a check gate that lands one commit on origin/develop from
// the other clone each time it runs, up to limit times, simulating another
// device moving the target while gates run.
func moveTargetGate(t *testing.T, other string, limit int) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "move.sh")
	counter := filepath.Join(dir, "count")
	body := "#!/bin/sh\nset -e\nn=$(cat '" + counter + "' 2>/dev/null || echo 0)\n" +
		"if [ \"$n\" -lt " + strconv.Itoa(limit) + " ]; then\n" +
		"  env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE git -C '" + other + "' commit -q --allow-empty -m \"move $n\"\n" +
		"  env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE git -C '" + other + "' push -q origin develop\n" +
		"  echo $((n+1)) > '" + counter + "'\nfi\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil { //nolint:gosec // test gate script must be executable
		t.Fatal(err)
	}
	return "check:\n\t@sh '" + script + "'\n"
}

func TestRun_RefreshUnderTargetLockRetriesTargetMovedAfterGates(t *testing.T) {
	fx := newRefreshFixture(t, true)
	// Installed after the fixture so the gate exists only on the task branch.
	writeRepoFile(t, fx.Worktree, "Makefile", moveTargetGate(t, fx.Other, 1))
	runGit(t, fx.Worktree, "commit", "-am", "moving gate")
	runGit(t, fx.Worktree, "push")
	runGit(t, fx.Other, "pull", "--ff-only", "origin", "develop")

	report, err := fx.run(t, true)
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, FormatRun(report))
	}
	if !report.Integrated || len(report.Refreshes) != 1 || report.Refreshes[0].Reason != refreshReasonTargetMoved {
		t.Fatalf("want one target-moved refresh then landing: %+v", report.Refreshes)
	}
}

func TestRun_RefreshUnderTargetLockBoundsTargetMovedRetries(t *testing.T) {
	fx := newRefreshFixture(t, true)
	writeRepoFile(t, fx.Worktree, "Makefile", moveTargetGate(t, fx.Other, 9))
	runGit(t, fx.Worktree, "commit", "-am", "always moving gate")
	runGit(t, fx.Worktree, "push")
	runGit(t, fx.Other, "pull", "--ff-only", "origin", "develop")

	report, err := fx.run(t, true)
	if err == nil || report == nil || report.Integrated {
		t.Fatalf("a target that keeps moving must refuse: err=%v", err)
	}
	if len(report.Refreshes) != maxTargetMovedRefreshes {
		t.Fatalf("refreshes = %d, want post-gate bound %d", len(report.Refreshes), maxTargetMovedRefreshes)
	}
	if !report.Check.TargetMovedAfterGates {
		t.Fatalf("final refusal is not the post-gate move: %s", FormatCheck(report.Check))
	}
}
