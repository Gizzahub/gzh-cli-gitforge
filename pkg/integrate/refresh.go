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

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// ADR-0003 (ce-devenv decisions/adr/0003-integration-refresh-under-target-lock.md):
// a repository that declares branch.refreshUnderTargetLock may have a stale
// task branch rebased onto the target tip while this run still holds the
// target lock. The rebase is the one a task session would otherwise do after
// a stale-base refusal; doing it under the lock keeps the next landing from
// racing it. Conflicts are never resolved: the rebase is aborted and the run
// refuses as before.

const (
	refreshReasonStaleBase   = "stale-base"
	refreshReasonTargetMoved = "target-moved-after-gates"
	// maxTargetMovedRefreshes is the retry bound ADR-0003 decision 5 fixes
	// for a target another writer moved while gates ran; maxTargetRefreshes
	// adds the initial refresh of a stale base. Changing either is an ADR
	// review trigger.
	maxTargetMovedRefreshes = 2
	maxTargetRefreshes      = maxTargetMovedRefreshes + 1
)

// ErrRefreshRefused marks a refresh that was attempted and refused. The
// message keeps "not ready" so callers treat it as a readiness refusal.
var ErrRefreshRefused = errors.New("not ready: refresh under target lock refused")

// TargetRefresh records one task-branch rebase performed under the target lock.
type TargetRefresh struct {
	Reason    string `json:"reason"`
	BeforeSHA string `json:"before_sha"`
	AfterSHA  string `json:"after_sha"`
	TargetSHA string `json:"target_sha"`
}

// refreshReason names why a not-ready check may be refreshed, or "" when it
// may not. Only a stale base, or a target that moved after every gate passed,
// qualifies; any other failure is a real refusal.
func refreshReason(check *CheckReport) string {
	switch {
	case check == nil || check.Ready:
		return ""
	case check.StaleBase:
		return refreshReasonStaleBase
	case check.TargetMovedAfterGates:
		return refreshReasonTargetMoved
	default:
		return ""
	}
}

// refreshEligible reports whether this run may refresh the checked plan. An
// ineligible run is not an error: it refuses exactly as it did before.
func refreshEligible(ctx context.Context, g gitRepo, opts RunOptions, check *CheckReport) (bool, error) {
	if !opts.RefreshUnderTargetLock || opts.ExpectTarget != "" || check.Controller != nil {
		return false, nil
	}
	plan := check.Plan
	if plan.Release || plan.releasesRemoteRef() || plan.Remote == "" {
		return false, nil
	}
	// The opt-in is read from the target commit only: a task branch cannot
	// grant itself permission to be rewritten.
	return loadCommitRefreshUnderTargetLock(ctx, g, plan.TargetSHA)
}

func loadCommitRefreshUnderTargetLock(ctx context.Context, g gitRepo, sha string) (bool, error) {
	for _, name := range []string{".gz-git.yaml", ".gz-git.yml", ".gz-git.json"} {
		entry, present, err := g.treeEntry(ctx, sha, name)
		if err != nil {
			return false, err
		}
		if !present {
			continue
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return false, fmt.Errorf("%s must be a regular file", name)
		}
		data, _, err := g.showFile(ctx, sha, name)
		if err != nil {
			return false, err
		}
		value, err := config.ParseRefreshUnderTargetLockDocument(data, strings.HasSuffix(name, ".json"))
		if err != nil {
			return false, fmt.Errorf("target %s: %w", name, err)
		}
		return value, nil
	}
	return false, nil
}

// attachRefreshes puts the refreshes ahead of the run output, in the order
// they happened.
func attachRefreshes(report *RunReport, branch string, refreshes []TargetRefresh) {
	if report == nil {
		return
	}
	report.Refreshes = refreshes
	printed := make([]string, 0, len(refreshes)+len(report.Printed))
	for _, r := range refreshes {
		printed = append(printed, fmt.Sprintf("REFRESHED %s %s -> %s onto %s (%s)", branch, r.BeforeSHA, r.AfterSHA, r.TargetSHA, r.Reason))
	}
	report.Printed = append(printed, report.Printed...)
}

// refreshUnderTargetLock applies ADR-0003 to a not-ready check while Run
// still holds the target lock: a stale task branch is rebased onto the
// target tip, lease-pushed, and checked again. It returns the last check and
// every refresh performed. An ineligible run returns its check unchanged.
func refreshUnderTargetLock(ctx context.Context, exec *gitcmd.Executor, opts *RunOptions, check *CheckReport, err error) (*CheckReport, []TargetRefresh, error) {
	dir := strings.TrimSpace(opts.RepoPath)
	if dir == "" {
		dir = "."
	}
	// A caller that pinned the source (an approved revision or a native
	// execution context) must land exactly that revision, so a rewrite would
	// override the approval. The loop below sets ExpectSource itself, which is
	// why the pin is read once, before any refresh.
	if opts.ExpectSource != "" || opts.Execution != nil {
		return check, nil, err
	}
	g := newGitRepo(exec, dir)
	var refreshes []TargetRefresh
	moved := 0
	// Each pass either refreshes or re-resolves a moved target, so bounding
	// passes as well as refreshes keeps a target that keeps moving finite.
	for pass := 0; err == nil && len(refreshes) < maxTargetRefreshes && pass < 2*maxTargetRefreshes; pass++ {
		reason := refreshReason(check)
		if reason == "" {
			break
		}
		eligible, eligibleErr := refreshEligible(ctx, g, *opts, check)
		if eligibleErr != nil {
			return check, refreshes, eligibleErr
		}
		if !eligible {
			break
		}
		if reason == refreshReasonTargetMoved {
			if moved >= maxTargetMovedRefreshes {
				break
			}
			moved++
			// The checked plan names the target the gates saw. Resolve the
			// tip that moved before rebasing onto it; freshness fails before
			// any gate runs, so this check is cheap.
			check, err = Check(ctx, exec, opts.CheckOptions)
			if err != nil || check == nil || !check.StaleBase {
				continue
			}
		}
		refresh, refreshErr := refreshTaskBranch(ctx, g, check.Plan, reason)
		if refreshErr != nil {
			return check, refreshes, refreshErr
		}
		refreshes = append(refreshes, refresh)
		// The refreshed head is now the only source this run may land, and a
		// negative outcome bound to the approved head would no longer be true.
		opts.ExpectSource = refresh.AfterSHA
		opts.preMutation = nil
		check, err = Check(ctx, exec, opts.CheckOptions)
	}
	return check, refreshes, err
}

// refreshTaskBranch rebases the checked task branch onto the checked target
// tip in its own worktree and lease-pushes the result. Every precondition
// failure, conflict, or lease rejection leaves the local and remote task
// branch at their checked commit.
func refreshTaskBranch(ctx context.Context, g gitRepo, plan TargetPlan, reason string) (TargetRefresh, error) {
	branch, before, target := plan.Branch, plan.BranchSHA, plan.TargetSHA
	refuse := func(format string, args ...any) (TargetRefresh, error) {
		return TargetRefresh{}, refreshRefused(format, args...)
	}
	wt, commits, err := refreshWorktree(ctx, g, plan)
	if err != nil {
		return TargetRefresh{}, err
	}

	res, err := wt.run(ctx, "-c", "rebase.autoStash=false", "-c", "rebase.updateRefs=false",
		"rebase", "--reapply-cherry-picks", "--empty=stop", "--no-autosquash", target)
	if err != nil {
		return TargetRefresh{}, err
	}
	if res.ExitCode != 0 {
		conflicts, diffErr := wt.output(ctx, "diff", "--name-only", "--diff-filter=U")
		if abortErr := abortRefresh(ctx, wt, before); abortErr != nil {
			return TargetRefresh{}, fmt.Errorf("%w: rebase onto %s failed and abort did not restore %s: %w", ErrRefreshRefused, target, before, abortErr)
		}
		if conflicts = strings.Join(strings.Fields(conflicts), ", "); diffErr == nil && conflicts != "" {
			return refuse("rebase onto %s conflicts in %s; rebase in the task worktree", target, conflicts)
		}
		return refuse("rebase onto %s stopped: %s", target, firstLine(res.Stderr))
	}
	after, err := wt.headSHA(ctx)
	if err != nil {
		return TargetRefresh{}, err
	}
	if n, err := wt.revCount(ctx, target+".."+after); err != nil || n != commits {
		if restoreErr := restoreRefresh(ctx, wt, before); restoreErr != nil {
			return TargetRefresh{}, fmt.Errorf("%w: refreshed commit count changed and restore failed: %w", ErrRefreshRefused, restoreErr)
		}
		if err != nil {
			return TargetRefresh{}, err
		}
		return refuse("rebase onto %s changed the commit count from %d to %d", target, commits, n)
	}
	lease := "--force-with-lease=refs/heads/" + branch + ":" + before
	push, err := wt.run(ctx, "push", "--porcelain", "--no-follow-tags", lease, plan.Remote, after+":refs/heads/"+branch)
	if err != nil || push.ExitCode != 0 {
		detail := ""
		if err != nil {
			detail = err.Error()
		} else {
			detail = firstLine(push.Stderr + push.Stdout)
		}
		if restoreErr := restoreRefresh(ctx, wt, before); restoreErr != nil {
			return TargetRefresh{}, fmt.Errorf("%w: lease push failed (%s) and restore failed: %w", ErrRefreshRefused, detail, restoreErr)
		}
		return refuse("lease push of %s to %s failed: %s", branch, plan.Remote, detail)
	}
	return TargetRefresh{Reason: reason, BeforeSHA: before, AfterSHA: after, TargetSHA: target}, nil
}

func refreshRefused(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrRefreshRefused, fmt.Sprintf(format, args...))
}

// refreshWorktree finds the linked worktree that may be rebased for the
// checked plan and returns it with the number of task commits to replay. It
// refuses any state a rebase could lose or silently change.
func refreshWorktree(ctx context.Context, g gitRepo, plan TargetPlan) (gitRepo, int, error) {
	branch, before, target := plan.Branch, plan.BranchSHA, plan.TargetSHA
	refuse := func(format string, args ...any) (gitRepo, int, error) {
		return gitRepo{}, 0, refreshRefused(format, args...)
	}
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return gitRepo{}, 0, err
	}
	worktree := ""
	for i, wt := range trees {
		if wt.Branch != branch {
			continue
		}
		if i == 0 {
			return refuse("%s is checked out in the primary checkout", branch)
		}
		worktree = wt.Path
	}
	if worktree == "" {
		return refuse("%s is not checked out in a task worktree", branch)
	}
	wt := gitRepo{exec: g.exec, dir: worktree}
	if head, err := wt.headSHA(ctx); err != nil {
		return gitRepo{}, 0, err
	} else if head != before {
		return refuse("task worktree HEAD %s is not the checked source %s", head, before)
	}
	if out, err := wt.porcelain(ctx); err != nil {
		return gitRepo{}, 0, err
	} else if strings.TrimSpace(out) != "" {
		return refuse("task worktree %s has uncommitted or untracked changes", worktree)
	}
	if op, err := gitOperationInProgress(ctx, wt); err != nil {
		return gitRepo{}, 0, err
	} else if op != "" {
		return refuse("task worktree %s has a %s in progress", worktree, op)
	}
	tracking := plan.Remote + "/" + branch
	if upstream, ok, err := wt.upstreamName(ctx, branch); err != nil {
		return gitRepo{}, 0, err
	} else if !ok || upstream != tracking {
		return refuse("%s does not track %s", branch, tracking)
	}
	if remoteSHA, ok, err := wt.revParse(ctx, "refs/remotes/"+tracking); err != nil {
		return gitRepo{}, 0, err
	} else if !ok || remoteSHA != before {
		return refuse("%s is not pushed: %s is not %s", branch, tracking, before)
	}
	// rebase linearizes merges, which would silently change what was reviewed.
	if merges, err := wt.output(ctx, "rev-list", "--min-parents=2", target+".."+before); err != nil {
		return gitRepo{}, 0, err
	} else if merges != "" {
		return refuse("%s contains merge commit(s) %s", branch, strings.Join(strings.Fields(merges), ", "))
	}
	commits, err := wt.revCount(ctx, target+".."+before)
	if err != nil {
		return gitRepo{}, 0, err
	}
	return wt, commits, nil
}

// gitOperationInProgress names an unfinished rebase, merge, cherry-pick,
// revert, or bisect in the worktree, or "" when there is none.
func gitOperationInProgress(ctx context.Context, wt gitRepo) (string, error) {
	gitDir, err := wt.output(ctx, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return "", err
	}
	for _, marker := range []struct{ path, op string }{
		{"rebase-merge", "rebase"},
		{"rebase-apply", "rebase"},
		{"MERGE_HEAD", "merge"},
		{"CHERRY_PICK_HEAD", "cherry-pick"},
		{"REVERT_HEAD", "revert"},
		{"sequencer", "cherry-pick or revert sequence"},
		{"BISECT_LOG", "bisect"},
	} {
		if _, err := os.Lstat(filepath.Join(gitDir, marker.path)); err == nil {
			return marker.op, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return "", nil
}

func abortRefresh(ctx context.Context, wt gitRepo, before string) error {
	if res, err := wt.run(ctx, "rebase", "--abort"); err != nil {
		return err
	} else if res.ExitCode != 0 {
		return fmt.Errorf("git rebase --abort: %s", firstLine(res.Stderr))
	}
	return requireHead(ctx, wt, before)
}

// restoreRefresh returns a completed rebase to its checked commit. The
// worktree was clean before the rebase, so --keep cannot discard user work.
func restoreRefresh(ctx context.Context, wt gitRepo, before string) error {
	if res, err := wt.run(ctx, "reset", "--keep", before); err != nil {
		return err
	} else if res.ExitCode != 0 {
		return fmt.Errorf("git reset --keep %s: %s", before, firstLine(res.Stderr))
	}
	return requireHead(ctx, wt, before)
}

func requireHead(ctx context.Context, wt gitRepo, want string) error {
	head, err := wt.headSHA(ctx)
	if err != nil {
		return err
	}
	if head != want {
		return fmt.Errorf("task worktree HEAD is %s, want %s", head, want)
	}
	return nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}
