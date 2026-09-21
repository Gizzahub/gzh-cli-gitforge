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
)

// TestRun_NoFetchFinishIntegratesWithoutFetch proves interpretation A of the
// no-fetch finish capability: when CheckOptions.NoFetch is set, Run integrates
// and reclaims without a fetch, using the target tracking ref already in the
// local repository and the checked target SHA as the push lease.
// See TASK-221.
func TestRun_NoFetchFinishIntegratesWithoutFetch(t *testing.T) {
	fx := runFixture(t, "dev/*")
	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{
			RepoPath: fx.Worktree,
			Branch:   "dev/actor/feat/task",
			NoFetch:  true,
		},
	})
	if err != nil {
		t.Fatalf("no-fetch Run: %v\n%s", err, FormatRun(report))
	}
	if !report.Integrated {
		t.Fatalf("want integrated under no-fetch:\n%s", FormatRun(report))
	}
	if report.Reclaim.Incomplete() {
		t.Fatalf("no-fetch reclaim incomplete: %+v", report.Reclaim)
	}
	if refExists(t, fx.Origin, "refs/heads/dev/actor/feat/task") {
		t.Fatal("remote task branch should be deleted by no-fetch reclaim")
	}
	if _, err := os.Stat(fx.Worktree); err == nil {
		t.Fatal("worktree should be removed by no-fetch reclaim")
	}
}

// TestCheck_NoFetchMissingTargetTrackingRefFails proves no-fetch check fails
// closed when the remote tracking ref for the declared integration branch is
// absent from the local clone; it must not silently fall back to the stale
// local integration branch.
func TestCheck_NoFetchMissingTargetTrackingRefFails(t *testing.T) {
	fx := runFixture(t, "dev/*")
	// Remove the remote-tracking ref without touching the remote itself:
	// exactly what a fetch-less clone looks like.
	runGit(t, fx.Worktree, "update-ref", "-d", "refs/remotes/"+fx.Remote+"/develop")

	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{
		RepoPath: fx.Worktree,
		Branch:   "dev/actor/feat/task",
		NoFetch:  true,
	})
	if err == nil {
		t.Fatalf("no-fetch check must fail without the tracking ref:\n%s", FormatCheck(report))
	}
}

// TestRun_NoFetchStaleTargetRefusesLease proves the cached-ref safety: the
// remote target is advanced past the task worktree's local snapshot by a
// second, separate clone that pushes an extra commit to the shared remote;
// the task worktree's own tracking ref is never rewound, it is simply never
// re-fetched, so under --no-fetch it stays stale relative to the remote.
// Against that staleness, the lease push must refuse and run must fail closed.
func TestRun_NoFetchStaleTargetRefusesLease(t *testing.T) {
	fx := runFixture(t, "dev/*")

	// Simulate a target that moved remotely after the local snapshot:
	other := t.TempDir()
	runGit(t, other, "clone", fx.Origin, ".")
	runGit(t, other, "config", "user.email", "other@test.com")
	runGit(t, other, "config", "user.name", "Other")
	runGit(t, other, "checkout", "-B", "develop", "origin/develop")
	writeFile(t, other, "sneak.txt", "sneak\n")
	runGit(t, other, "add", "sneak.txt")
	runGit(t, other, "commit", "-m", "sneak")
	runGit(t, other, "push", "origin", "develop")
	// Do NOT fetch in the task worktree: its tracking ref is now stale.

	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{
			RepoPath: fx.Worktree,
			Branch:   "dev/actor/feat/task",
			NoFetch:  true,
		},
	})
	if err == nil {
		t.Fatalf("stale target must fail closed under no-fetch:\n%s", FormatRun(report))
	}
	if report != nil && report.Integrated {
		t.Fatalf("integration must not complete when the lease target moved:\n%s", FormatRun(report))
	}
	if !refExists(t, fx.Origin, "refs/heads/develop") || gitOutput(t, other, "rev-parse", "refs/heads/develop") != gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop") {
		t.Fatal("remote develop must still hold the sneaked commit")
	}
}

// TestReclaimRemoteBranch_NoFetchUnverifiableFailsClosed proves the no-fetch
// reclaim contract: when the leased remote delete fails, the result must fail
// closed instead of claiming "already-deleted" on the basis of an ls-remote
// probe, because that probe is itself a network read --no-fetch forbids.
// See TASK-221.
func TestReclaimRemoteBranch_NoFetchUnverifiableFailsClosed(t *testing.T) {
	fx := runFixture(t, "dev/*")
	task := "dev/actor/feat/task"
	sha := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	// Delete the remote branch but keep the local tracking ref, exactly like
	// TestReclaimRemoteBranch_AlreadyDeleted.
	runGit(t, fx.Clone, "push", fx.Remote, ":"+task)
	runGit(t, fx.Clone, "update-ref", "refs/remotes/"+fx.Remote+"/"+task, sha)
	if !refExists(t, fx.Clone, "refs/remotes/"+fx.Remote+"/"+task) {
		t.Fatal("tracking ref must remain so reclaim still attempts the delete")
	}

	// Wrapper git binary: force every push to fail, delegate the rest
	// (including ls-remote, which truthfully reports the deleted branch
	// as absent) to the real git. This deterministically drives reclaim
	// onto the unverifiable path.
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("look up git: %v", err)
	}
	wrapper := filepath.Join(t.TempDir(), "git-push-fails.sh")
	script := "#!/bin/sh\nif [ \"$1\" = \"push\" ]; then echo \"injected push failure\" >&2; exit 1; fi\nexec \"" + realGit + "\" \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}

	var out ReclaimResult
	ok := reclaimRemoteBranch(context.Background(), newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(wrapper)), fx.Clone), reclaimOpts{
		Branch:  task,
		Remote:  fx.Remote,
		TaskSHA: sha,
		NoFetch: true,
	}, &out)
	if ok {
		t.Fatalf("unverifiable no-fetch delete must fail closed, got success: %+v", out)
	}
	if len(out.Failed) == 0 {
		t.Fatalf("want a recorded reclaim failure, got %+v", out)
	}
	for _, done := range out.Done {
		if strings.Contains(done, "already-deleted") {
			t.Fatalf("must not claim already-deleted under --no-fetch: %+v", out)
		}
	}
}
