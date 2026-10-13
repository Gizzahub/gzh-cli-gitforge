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
)

func TestCheck_ExpectedTargetMismatchStopsBeforeGate(t *testing.T) {
	fx := readyTaskFixture(t)
	marker := filepath.Join(t.TempDir(), "gate-ran")
	writeRepoFile(t, fx.Worktree, "Makefile", "check:\n\t@touch "+marker+"\n")
	runGit(t, fx.Worktree, "add", "Makefile")
	runGit(t, fx.Worktree, "commit", "-m", "mark gate")
	runGit(t, fx.Worktree, "push", fx.Remote, "HEAD")

	_, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", ExpectTarget: strings.Repeat("0", 40),
	})
	if !errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("Check() error = %v, want ErrTargetMismatch", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("gate marker = %v, want absent: expected-target must stop before measurement", statErr)
	}
}

func TestCheck_ExpectedTargetPostGateMovementIsRetryable(t *testing.T) {
	fx := readyTaskFixture(t)
	before := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")
	source := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	writeRepoFile(t, fx.Worktree, "Makefile", "check:\n\t@git --git-dir="+fx.Origin+" update-ref refs/heads/develop "+source+"\n")
	runGit(t, fx.Worktree, "add", "Makefile")
	runGit(t, fx.Worktree, "commit", "-m", "move target from gate")
	runGit(t, fx.Worktree, "push", fx.Remote, "HEAD")

	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", ExpectTarget: before,
	})
	if !errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("Check() error = %v, want ErrTargetMismatch", err)
	}
	if report == nil || report.Plan.TargetSHA != before {
		t.Fatalf("report = %#v, want original target %s", report, before)
	}
}

func TestExpectedTargetAndSourceKeepDifferentRoles(t *testing.T) {
	fx := readyTaskFixture(t)
	source := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	target := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")
	if _, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", ExpectSource: target, ExpectTarget: source}); !errors.Is(err, ErrSourceMismatch) {
		t.Fatalf("swapped source expectation error = %v, want ErrSourceMismatch", err)
	}
	if _, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", ExpectSource: source, ExpectTarget: source}); !errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("swapped target expectation error = %v, want ErrTargetMismatch", err)
	}
}

func TestNormalizeExpectTargetRejectsAbbreviations(t *testing.T) {
	if _, err := NormalizeExpectTarget("deadbeef"); !errors.Is(err, ErrInvalidExpectTarget) {
		t.Fatalf("NormalizeExpectTarget() error = %v, want ErrInvalidExpectTarget", err)
	}
}

func TestPushFastForwardClassifiesOnlyMovedTargetAsRetryable(t *testing.T) {
	fx := runFixture(t, "")
	checked := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")
	source := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	other := filepath.Join(t.TempDir(), "other")
	runGit(t, "", "clone", fx.Origin, other)
	runGit(t, other, "config", "user.email", "test@example.invalid")
	runGit(t, other, "config", "user.name", "Target Mover")
	runGit(t, other, "checkout", "develop")
	writeFile(t, other, "moved.txt", "moved\n")
	runGit(t, other, "add", "moved.txt")
	runGit(t, other, "commit", "-m", "move target")
	runGit(t, other, "push", "origin", "develop")

	err := pushFastForwardWithExpectedTarget(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), fx.Remote, fx.Remote, true, source, "develop", checked)
	if !errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("moved lease error = %v, want ErrTargetMismatch", err)
	}
	if err := pushFastForwardWithExpectedTarget(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), filepath.Join(t.TempDir(), "missing-remote"), fx.Remote, true, source, "develop", gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")); errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("unrelated push error = %v, must not be ErrTargetMismatch", err)
	}
	if isExplicitLeaseRejection(&gitcmd.Result{ExitCode: 1, Stderr: "transport interrupted"}) {
		t.Fatal("ambiguous transport failure must not classify as lease rejection")
	}
	if !isExplicitLeaseRejection(&gitcmd.Result{ExitCode: 1, Stderr: "! [rejected] deadbeef -> develop (stale info)"}) {
		t.Fatal("explicit stale lease rejection was not recognized")
	}
}

func TestLeaseRejectionRetainsExistingIntentReceipt(t *testing.T) {
	fx := runFixture(t, "")
	exec := gitcmd.NewExecutor()
	ctx := context.Background()
	check, err := Check(ctx, exec, CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
	if err != nil || !check.Ready {
		t.Fatalf("Check() = %#v, %v", check, err)
	}
	g := newGitRepo(exec, fx.Worktree)
	common := gitOutput(t, fx.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	execution := &ExecutionContext{
		ExecutionID: "expect-target-lease", Generation: "g1",
		Owner:        OwnerIdentity{Actor: "codex", Host: "testhost", Kind: "agent"},
		ExpectSource: check.Plan.BranchSHA,
	}
	opts := RunOptions{CheckOptions: CheckOptions{RepoPath: fx.Worktree, Branch: check.Plan.Branch}, Execution: execution}
	receipt, err := beginRunEvidence(ctx, exec, g, common, opts, check, check.Plan.BranchSHA, "develop")
	if err != nil {
		t.Fatalf("beginRunEvidence() = %v", err)
	}

	other := filepath.Join(t.TempDir(), "other")
	runGit(t, "", "clone", fx.Origin, other)
	runGit(t, other, "config", "user.email", "test@example.invalid")
	runGit(t, other, "config", "user.name", "Target Mover")
	runGit(t, other, "checkout", "develop")
	writeFile(t, other, "moved.txt", "moved\n")
	runGit(t, other, "add", "moved.txt")
	runGit(t, other, "commit", "-m", "move target")
	runGit(t, other, "push", "origin", "develop")

	err = pushFastForwardWithExpectedTarget(ctx, g, fx.Remote, fx.Remote, true, check.Plan.BranchSHA, "develop", check.Plan.TargetSHA)
	if !errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("pushFastForward() error = %v, want ErrTargetMismatch", err)
	}
	stored, ok, err := readReceipt(common, execution.ExecutionID)
	if err != nil || !ok {
		t.Fatalf("readReceipt() ok=%t err=%v", ok, err)
	}
	if stored.Phase != evidencePhaseIntent || stored.SourcePushed || stored.Revision != receipt.Revision {
		t.Fatalf("lease rejection changed intent receipt: %+v", stored)
	}
}

func TestLegacyTargetMovementDoesNotBecomeExpectedTargetMismatch(t *testing.T) {
	fx := readyTaskFixture(t)
	check, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
	if err != nil || !check.Ready {
		t.Fatalf("Check() = %#v, %v", check, err)
	}
	runGit(t, fx.Clone, "checkout", "develop")
	writeFile(t, fx.Clone, "target-moved.txt", "moved\n")
	runGit(t, fx.Clone, "add", "target-moved.txt")
	runGit(t, fx.Clone, "commit", "-m", "move target")
	runGit(t, fx.Clone, "push", fx.Remote, "develop")
	_, _, _, err = revalidateCheckedRefs(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), check, false)
	if err == nil || errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("legacy revalidation error = %v, want generic target movement", err)
	}
}

func TestTargetMoveProofUsesOneExactPushEndpoint(t *testing.T) {
	fx := runFixture(t, "")
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	expected := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")
	pushOnly := filepath.Join(t.TempDir(), "push-only.git")
	runGit(t, "", "clone", "--bare", fx.Origin, pushOnly)
	runGit(t, fx.Worktree, "config", "remote."+fx.Remote+".pushurl", pushOnly)

	writeFile(t, fx.Clone, "fetch-only-move.txt", "moved\n")
	runGit(t, fx.Clone, "add", "fetch-only-move.txt")
	runGit(t, fx.Clone, "commit", "-m", "move fetch endpoint")
	runGit(t, fx.Clone, "push", fx.Remote, "develop")
	moved, err := targetMovedAtPushEndpoint(context.Background(), g, fx.Remote, fx.Remote, "develop", expected, gitOutput(t, fx.Worktree, "rev-parse", "HEAD"))
	if err != nil || moved {
		t.Fatalf("push endpoint proof moved=%t err=%v, want unchanged push endpoint", moved, err)
	}

	runGit(t, fx.Worktree, "config", "--add", "remote."+fx.Remote+".pushurl", fx.Origin)
	if _, err := targetMovedAtPushEndpoint(context.Background(), g, fx.Remote, fx.Remote, "develop", expected, gitOutput(t, fx.Worktree, "rev-parse", "HEAD")); err == nil {
		t.Fatal("multiple push endpoints must refuse retryable classification")
	}
}

func TestExpectedTargetPushSuppressesFollowTagsOnLeaseRejection(t *testing.T) {
	fx := runFixture(t, "")
	source := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	checked := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")
	runGit(t, fx.Worktree, "tag", "-a", "expect-target-no-follow-tags", "-m", "test tag")
	runGit(t, fx.Worktree, "config", "push.followTags", "true")
	moveTargetForExpectedTargetTest(t, fx, "follow-tags")

	err := pushFastForwardWithExpectedTarget(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), fx.Remote, fx.Remote, true, source, "develop", checked)
	if !errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("push error = %v, want ErrTargetMismatch", err)
	}
	if refExists(t, fx.Origin, "refs/tags/expect-target-no-follow-tags") {
		t.Fatal("opt-in expected-target push accepted an unrelated follow-tag")
	}
}

func TestExpectedTargetPushRejectsMirrorRemoteBeforePush(t *testing.T) {
	fx := runFixture(t, "")
	source := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	checked := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")
	runGit(t, fx.Worktree, "config", "remote."+fx.Remote+".mirror", "true")
	err := pushFastForwardWithExpectedTarget(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), fx.Remote, fx.Remote, true, source, "develop", checked)
	if err == nil || !strings.Contains(err.Error(), "mirror-configured") {
		t.Fatalf("mirror push error = %v, want pre-push mirror refusal", err)
	}
	if got := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != checked {
		t.Fatalf("mirror refusal moved target: got %s, want %s", got, checked)
	}
}

func TestTargetMoveProofRejectsAlreadyAcceptedSource(t *testing.T) {
	fx := runFixture(t, "")
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	expected := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")
	source := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	runGit(t, fx.Clone, "checkout", "develop")
	runGit(t, fx.Clone, "reset", "--hard", source)
	runGit(t, fx.Clone, "push", "--force", fx.Remote, "develop")
	moved, err := targetMovedAtPushEndpoint(context.Background(), g, fx.Remote, fx.Remote, "develop", expected, source)
	if err != nil || moved {
		t.Fatalf("accepted source proof moved=%t err=%v, want non-retryable", moved, err)
	}
}

func moveTargetForExpectedTargetTest(t *testing.T, fixture *testutil.WorktreeOrigin, suffix string) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other-"+suffix)
	runGit(t, "", "clone", fixture.Origin, other)
	runGit(t, other, "config", "user.email", "test@example.invalid")
	runGit(t, other, "config", "user.name", "Target Mover")
	runGit(t, other, "checkout", "develop")
	writeFile(t, other, "moved-"+suffix+".txt", "moved\n")
	runGit(t, other, "add", ".")
	runGit(t, other, "commit", "-m", "move target "+suffix)
	runGit(t, other, "push", "origin", "develop")
}
