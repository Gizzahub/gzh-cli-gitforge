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
	if isSolePorcelainStaleLeaseRejection(&gitcmd.Result{ExitCode: 1, Stderr: "transport interrupted"}, source, "refs/heads/develop") {
		t.Fatal("ambiguous transport failure must not classify as lease rejection")
	}
	if isSolePorcelainStaleLeaseRejection(&gitcmd.Result{ExitCode: 1, Stderr: " ! [rejected] " + source + " -> develop (stale info)"}, source, "refs/heads/develop") {
		t.Fatal("stderr must never prove a lease rejection")
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

// TestExpectedTargetPorcelainLeaseIsBoundToExactRef feeds the classifier the
// actual bytes a real git binary prints for a stale --force-with-lease push
// run with the exact opt-in argument list, then proves the classification is
// bound to the recorded source SHA and refs/heads/<target> alone.
func TestExpectedTargetPorcelainLeaseIsBoundToExactRef(t *testing.T) {
	fx := runFixture(t, "")
	ctx := context.Background()
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	source := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	checked := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")
	lease := "--force-with-lease=refs/heads/develop:" + checked
	refspec := source + ":refs/heads/develop"

	// Move the leased branch so the real push fails and git prints its real
	// porcelain stale-info record for exactly this source and ref.
	moveTargetForExpectedTargetTest(t, fx, "porcelain-bound")
	fixture, err := g.run(ctx, "push", "--porcelain", "--no-follow-tags", lease, fx.Remote, refspec)
	if err != nil {
		t.Fatalf("git push --porcelain: %v", err)
	}
	if fixture.ExitCode == 0 {
		t.Fatalf("porcelain push unexpectedly succeeded: %+v", fixture)
	}
	if !isSolePorcelainStaleLeaseRejection(fixture, source, "refs/heads/develop") {
		t.Fatalf("real porcelain fixture not classified: stdout=%q stderr=%q", fixture.Stdout, fixture.Stderr)
	}
	// The same real bytes must not classify for any other source or ref: the
	// record binds the approval, it is not a substring about staleness.
	if isSolePorcelainStaleLeaseRejection(fixture, checked, "refs/heads/develop") {
		t.Fatal("real fixture classified for a source it does not record")
	}
	if isSolePorcelainStaleLeaseRejection(fixture, source, "refs/heads/master") {
		t.Fatal("real fixture classified for a ref it does not record")
	}
	// End to end, the opt-in push turns only that exact record, confirmed
	// against the leased destination, into the retryable mismatch.
	if err := pushFastForwardWithExpectedTarget(ctx, g, fx.Remote, fx.Remote, true, source, "develop", checked); !errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("pushFastForward() error = %v, want ErrTargetMismatch", err)
	}
	if got := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop"); got == source {
		t.Fatal("rejected lease push must not have moved the remote target to the source")
	}
}

// TestExpectedTargetPorcelainRejectsAmbiguousOutput drives the classifier
// directly with parser-level cases. Every non-control case must stay generic:
// stderr never proves, and spoofed, partial, malformed, other-ref, accepted,
// mixed, and policy/remote/transport shapes never earn the retryable exit.
func TestExpectedTargetPorcelainRejectsAmbiguousOutput(t *testing.T) {
	const source = "8d635ba34957c594544fc393f79316b9f51b965a"
	const other = "1111111111111111111111111111111111111111"
	const targetRef = "refs/heads/develop"
	header := "To /fixtures/origin.git\n"
	record := "!\t" + source + ":" + targetRef + "\t[rejected] (stale info)\n"
	footer := "Done\n"
	result := func(stdout string, stderr ...string) *gitcmd.Result {
		push := &gitcmd.Result{ExitCode: 1, Stdout: stdout}
		if len(stderr) > 0 {
			push.Stderr = stderr[0]
		}
		return push
	}
	tests := []struct {
		name   string
		push   *gitcmd.Result
		source string
		ref    string
		want   bool
	}{
		{
			name:   "sole complete rejection record",
			push:   result(header+record+footer, "error: failed to push some refs\n"),
			source: source,
			ref:    targetRef,
			want:   true,
		},
		{
			name:   "stale text on stderr only, non-porcelain grammar",
			push:   result("", " ! [rejected] "+source+" -> develop (stale info)\n"),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "record without To header and Done footer",
			push:   result(record, ""),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "record without Done footer",
			push:   result(header+record, ""),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "record without To header",
			push:   result(record+footer, ""),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "record for another ref",
			push:   result(header + "!\t" + source + ":refs/heads/master\t[rejected] (stale info)\n" + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name: "mixed accepted other ref and rejected leased ref",
			push: result(header +
				"*\t" + source + ":refs/heads/side\t[new branch]\n" +
				record + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "accepted record for the leased ref",
			push:   result(header + "*\t" + source + ":" + targetRef + "\t[new branch]\n" + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "remote policy rejection for the leased ref",
			push:   result(header + "!\t" + source + ":" + targetRef + "\t[remote rejected] (pre-receive hook declined)\n" + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "record names a different source",
			push:   result(header + "!\t" + other + ":" + targetRef + "\t[rejected] (stale info)\n" + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "approval names a different source",
			push:   result(header+record+footer, ""),
			source: other,
			ref:    targetRef,
		},
		{
			name:   "non-fast-forward reason is not the lease",
			push:   result(header + "!\t" + source + ":" + targetRef + "\t[rejected] (non-fast-forward)\n" + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "record truncated before closing parenthesis",
			push:   result(header + "!\t" + source + ":" + targetRef + "\t[rejected] (stale info\n" + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "record truncated before summary field",
			push:   result(header + "!\t" + source + ":" + targetRef + "\n" + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "space-separated spoof prose instead of porcelain tabs",
			push:   result(header + "! " + source + ":" + targetRef + " [rejected] (stale info)\n" + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "unrecognized transport failure line on stdout",
			push:   result(header + "fatal: the remote end hung up unexpectedly\n" + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "no record at all",
			push:   result(header+footer, ""),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "duplicated rejection record",
			push:   result(header + record + record + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "footer before the record",
			push:   result(header + footer + record),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "header after the record",
			push:   result(record + header + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "To header without a destination",
			push:   result("To \n" + record + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "To header with padded destination",
			push:   result("To  /fixtures/origin.git \n" + record + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "record smuggled inside the To header",
			push:   result("To !\t" + source + ":" + targetRef + "\t[rejected] (stale info)\n" + footer),
			source: source,
			ref:    targetRef,
		},
		{
			name:   "report missing its final newline",
			push:   result(strings.TrimSuffix(header+record+footer, "\n")),
			source: source,
			ref:    targetRef,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSolePorcelainStaleLeaseRejection(tt.push, tt.source, tt.ref); got != tt.want {
				t.Fatalf("isSolePorcelainStaleLeaseRejection() = %t, want %t (stdout=%q stderr=%q)", got, tt.want, tt.push.Stdout, tt.push.Stderr)
			}
		})
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
