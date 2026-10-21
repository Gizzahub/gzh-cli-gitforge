// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// TestLeaseRejectionNamesStaleTarget moves the target after readiness and
// pushes with the checked lease. With or without an approved target, the
// refusal names the target ref, the commit readiness checked, the current
// commit, and the rebase to do.
func TestLeaseRejectionNamesStaleTarget(t *testing.T) {
	t.Parallel()
	for _, expect := range []bool{true, false} {
		t.Run(fmt.Sprintf("expect-target=%t", expect), func(t *testing.T) {
			t.Parallel()
			fx := runFixture(t, "")
			source := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
			checked := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")
			moveTargetForExpectedTargetTest(t, fx, fmt.Sprintf("lease-%t", expect))
			current := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop")
			ref := fx.Remote + "/develop"

			err := pushFastForwardWithExpectedTarget(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), fx.Remote, fx.Remote, expect, source, "develop", checked)
			if err == nil {
				t.Fatal("stale lease push succeeded")
			}
			if expect {
				want := fmt.Sprintf("%v: target %s is %s, expected %s; rebase onto %s and re-run check", ErrTargetMismatch, ref, current, checked, ref)
				if !errors.Is(err, ErrTargetMismatch) || err.Error() != want {
					t.Fatalf("push error = %q, want %q", err, want)
				}
			} else {
				want := "target branch changed before the push: " + ref + " is " + current + ", readiness checked " + checked + "; rebase onto " + ref + " and re-run check (git push: "
				if errors.Is(err, ErrTargetMismatch) || !strings.HasPrefix(err.Error(), want) {
					t.Fatalf("push error = %q, want prefix %q", err, want)
				}
			}
			if got := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != current {
				t.Fatalf("rejected push moved the target: %s -> %s", current, got)
			}
		})
	}
}

// TestLeaseRejectionNamesStaleTargetWhenCurrentIsUnknown pushes to a bare URL
// with no remote to prove the endpoint against. The refusal still names the
// target, the checked commit and the rebase, and says the current commit is
// unknown instead of guessing it.
func TestLeaseRejectionNamesStaleTargetWhenCurrentIsUnknown(t *testing.T) {
	t.Parallel()
	fx := runFixture(t, "")
	source := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	checked := gitOutput(t, fx.Worktree, "rev-parse", fx.Remote+"/develop")
	moveTargetForExpectedTargetTest(t, fx, "lease-unknown")

	err := pushFastForwardWithExpectedTarget(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), fx.Origin, "", false, source, "develop", checked)
	want := "target branch changed before the push: develop current commit unknown ("
	if err == nil || !strings.HasPrefix(err.Error(), want) || !strings.Contains(err.Error(), "readiness checked "+checked+"; rebase onto develop and re-run check") {
		t.Fatalf("push error = %v, want named refusal with an unknown current commit", err)
	}
}

// TestConcurrentRunsOnOneTargetNeverWait holds the first run inside its
// target push, after the target has moved, and starts a second run of another
// task on the same target. The second run must not wait for the first: it
// refuses at once, naming the moved target and the rebase. The first run then
// finishes its integration.
func TestConcurrentRunsOnOneTargetNeverWait(t *testing.T) {
	fx := runFixture(t, "")
	executor := gitcmd.NewExecutor()
	first := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")

	second := filepath.Join(t.TempDir(), "task2")
	runGit(t, fx.Clone, "worktree", "add", "-b", "dev/actor/feat/task2", second, fx.Remote+"/develop")
	writeFile(t, second, "task2.txt", "task2\n")
	writeRepoFile(t, second, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n")
	runGit(t, second, "add", ".")
	runGit(t, second, "commit", "-m", "second task work")
	runGit(t, second, "push", "-u", fx.Remote, "HEAD")

	// The hook keeps the first run's push open after the target has
	// moved, so the second run sees a target the first is still landing.
	scratch := t.TempDir()
	landed := filepath.Join(scratch, "landed")
	release := filepath.Join(scratch, "release")
	hook := "#!/bin/sh\nwhile read old new ref; do\n  if [ \"$new\" = '" + first + "' ]; then\n    : > '" + landed + "'\n    n=0\n    while [ ! -e '" + release + "' ] && [ $n -lt 600 ]; do sleep 0.1; n=$((n+1)); done\n  fi\ndone\n"
	if err := os.WriteFile(filepath.Join(fx.Origin, "hooks", "post-receive"), []byte(hook), 0o755); err != nil { // #nosec G306 -- test hook must be executable
		t.Fatal(err)
	}
	released := false
	releaseFirst := func() {
		if !released {
			released = true
			if err := os.WriteFile(release, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	defer releaseFirst()

	type result struct {
		report *RunReport
		err    error
	}
	done := make(chan result, 1)
	go func() {
		report, err := Run(context.Background(), executor, RunOptions{CheckOptions: CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"}})
		done <- result{report, err}
	}()
	deadline := time.Now().Add(testWaitTimeout)
	for {
		if _, err := os.Stat(landed); err == nil {
			break
		}
		select {
		case got := <-done:
			t.Fatalf("first run returned before its push landed: report=%+v err=%v", got.report, got.err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("first run never reached its target push")
		}
		time.Sleep(20 * time.Millisecond)
	}

	started := time.Now()
	report, err := Run(context.Background(), executor, RunOptions{CheckOptions: CheckOptions{RepoPath: second, Branch: "dev/actor/feat/task2"}})
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("second run waited %s on the first; it must refuse at once", elapsed)
	}
	select {
	case got := <-done:
		t.Fatalf("first run finished while held; the second run did not overlap it: report=%+v err=%v", got.report, got.err)
	default:
	}
	if err == nil || report == nil || report.Integrated || report.Check == nil || report.Check.Ready {
		t.Fatalf("second run = %+v, %v; want a not-ready refusal", report, err)
	}
	freshness := report.Check.Items[0]
	for _, item := range report.Check.Items {
		if item.Name == "freshness" {
			freshness = item
		}
	}
	if freshness.Name != "freshness" || freshness.Status != checkFail || !strings.Contains(freshness.Detail, "behind "+fx.Remote+"/develop") || !strings.Contains(freshness.Detail, "git rebase "+fx.Remote+"/develop") {
		t.Fatalf("second run refusal must name the moved target and the rebase: %+v", report.Check.Items)
	}

	releaseFirst()
	got := <-done
	if got.err != nil || got.report == nil || !got.report.Integrated {
		t.Fatalf("first run = %+v, %v; want integrated", got.report, got.err)
	}
	if target := gitOutput(t, fx.Origin, "rev-parse", "refs/heads/develop"); target != first {
		t.Fatalf("target = %s, want the first run's %s", target, first)
	}
}
