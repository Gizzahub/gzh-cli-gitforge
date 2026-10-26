// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package branch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func (p *WorktreeSyncPlan) inspectNode(ctx context.Context, node *worktreeSyncNode) {
	if err := p.safeWorktree(ctx, node, node.tip); err != nil {
		blockSyncNode(node, err.Error())
		return
	}
	output, err := p.output(ctx, node.worktree.Path, "for-each-ref", "--format=%(refname)%00%(upstream)%00%(upstream:remotename)%00%(upstream:remoteref)", "refs/heads/"+node.result.Branch)
	if err != nil {
		blockSyncNode(node, err.Error())
		return
	}
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		fields := strings.Split(line, "\x00")
		if len(fields) == 4 && fields[0] == "refs/heads/"+node.result.Branch {
			node.upstream, node.remote, node.remoteRef = fields[1], fields[2], fields[3]
		}
	}
	if node.upstream == "" && node.rule == nil {
		blockSyncNode(node, "no upstream or parent rule configured")
	}
	if node.remote == "." {
		blockSyncNode(node, "local upstreams are not supported; unset the local upstream and configure an explicit parent rule")
	}
	if node.remote != "" && node.remote != "." {
		if err := gitcmd.SanitizeRemoteName(node.remote); err != nil {
			blockSyncNode(node, err.Error())
		}
		if !strings.HasPrefix(node.upstream, "refs/remotes/") || !strings.HasPrefix(node.remoteRef, "refs/heads/") {
			blockSyncNode(node, "upstream must map a remote branch into refs/remotes/")
		}
		for _, ref := range []string{node.upstream, node.remoteRef} {
			if err := gitcmd.SanitizeBranchName(ref); err != nil {
				blockSyncNode(node, err.Error())
			}
		}
	}
}

func (p *WorktreeSyncPlan) safeWorktree(ctx context.Context, node *worktreeSyncNode, expected string) error {
	wt := node.worktree
	if wt.IsDetached || wt.Branch == "" {
		return fmt.Errorf("detached HEAD")
	}
	if wt.IsLocked || wt.IsPrunable {
		return fmt.Errorf("worktree is locked or prunable")
	}
	if err := gitcmd.SanitizePath(wt.Path); err != nil {
		return err
	}
	branch, err := p.output(ctx, wt.Path, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(branch) != "refs/heads/"+wt.Branch {
		return fmt.Errorf("checked-out branch changed since discovery")
	}
	head, err := p.resolve(ctx, wt.Path, "HEAD")
	if err != nil {
		return err
	}
	if head != expected {
		return fmt.Errorf("HEAD changed since planning")
	}
	status, err := p.output(ctx, wt.Path, "status", "--porcelain=v1", "-z", "--untracked-files=normal", "--ignore-submodules=none")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("uncommitted changes")
	}
	for _, marker := range []string{"rebase-merge", "rebase-apply", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "sequencer", "BISECT_START", "index.lock", "locked"} {
		location, err := p.output(ctx, wt.Path, "rev-parse", "--git-path", marker)
		if err != nil {
			return err
		}
		location = strings.TrimSuffix(location, "\n")
		if !filepath.IsAbs(location) {
			location = filepath.Join(wt.Path, location)
		}
		if _, err := os.Stat(location); err == nil {
			return fmt.Errorf("git operation in progress (%s)", marker)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect %s: %w", marker, err)
		}
	}
	return nil
}

func (p *WorktreeSyncPlan) fetch(ctx context.Context) {
	remotes := make(map[string]string)
	for _, node := range p.nodes {
		if node.result.Status != "blocked" && node.remote != "" && node.remote != "." {
			remotes[node.remote] = node.worktree.Path
		}
	}
	names := make([]string, 0, len(remotes))
	for remote := range remotes {
		names = append(names, remote)
	}
	sort.Strings(names)
	for _, remote := range names {
		if !p.sameFetchConfig(ctx, remote) {
			continue
		}
		if p.DryRun {
			continue
		}
		// Explicit refspecs prevent a configured fetch mapping from updating
		// local branches or tags. Only required tracking refs are fetched.
		args := []string{"-c", "fetch.recurseSubmodules=false", "fetch", "--no-tags", "--no-recurse-submodules", "--refmap=", "--", remote}
		seen := make(map[string]bool)
		for _, node := range p.nodes {
			if node.remote == remote && node.result.Status != "blocked" {
				refspec := "+" + node.remoteRef + ":" + node.upstream
				if !seen[refspec] {
					args = append(args, refspec)
					seen[refspec] = true
				}
			}
		}
		_, err := p.output(ctx, remotes[remote], args...)
		if err != nil {
			for _, node := range p.nodes {
				if node.remote == remote {
					blockSyncNode(node, "fetch failed: "+err.Error())
				}
			}
		} else {
			p.Fetched = true
		}
	}
}

// Git configuration (including URL rewrites and conditional includes) can be
// worktree-specific. Fetching once is safe only when every participant would
// use the same effective configuration. Never put configuration values in a
// diagnostic: they can contain credentials.
func (p *WorktreeSyncPlan) sameFetchConfig(ctx context.Context, remote string) bool {
	var config, url string
	first := true
	message := ""
	for _, node := range p.nodes {
		if node.remote != remote || node.result.Status == "blocked" {
			continue
		}
		current, err := p.output(ctx, node.worktree.Path, "config", "--null", "--list")
		if err != nil {
			message = "cannot inspect effective Git configuration before fetch"
			break
		}
		resolved, err := p.output(ctx, node.worktree.Path, "remote", "get-url", "--", remote)
		if err != nil {
			message = "cannot inspect effective remote before fetch"
			break
		}
		if !first && (current != config || resolved != url) {
			message = "worktrees have different effective Git configurations for the same remote"
			break
		}
		config, url, first = current, resolved, false
	}
	if message == "" {
		return true
	}
	for _, node := range p.nodes {
		if node.remote == remote {
			blockSyncNode(node, message)
		}
	}
	return false
}

func (p *WorktreeSyncPlan) resolve(ctx context.Context, dir, ref string) (string, error) {
	output, err := p.output(ctx, dir, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	return strings.TrimSpace(output), err
}

func (p *WorktreeSyncPlan) ancestor(ctx context.Context, dir, older, newer string) (bool, error) {
	result, err := p.git.Run(ctx, dir, "merge-base", "--is-ancestor", older, newer)
	if err != nil {
		return false, err
	}
	switch result.ExitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("merge-base failed: %s", strings.TrimSpace(result.Stderr))
	}
}

func (p *WorktreeSyncPlan) planNode(ctx context.Context, node *worktreeSyncNode) {
	if node.parent != nil && node.parent.result.Status == "blocked" {
		blockSyncNode(node, "parent "+node.parent.result.Branch+" is blocked")
	}
	if node.result.Status == "blocked" {
		return
	}
	if node.upstream != "" {
		target, err := p.resolve(ctx, node.worktree.Path, node.upstream)
		if err != nil {
			blockSyncNode(node, "upstream unavailable: "+err.Error())
			return
		}
		p.planStep(ctx, node, node.upstream, target, "ff-only")
	}
	if node.parent != nil && node.result.Status != "blocked" {
		p.planStep(ctx, node, "refs/heads/"+node.parent.result.Branch, node.parent.final, node.rule.Strategy)
	}
}

func (p *WorktreeSyncPlan) planStep(ctx context.Context, node *worktreeSyncNode, source, target, strategy string) {
	step := WorktreeSyncStep{Source: source, Strategy: strategy, From: node.final, To: target, Status: "planned", Action: "up-to-date"}
	ahead, err := p.ancestor(ctx, node.worktree.Path, target, node.final)
	if err != nil {
		blockSyncNode(node, err.Error())
		return
	}
	if !ahead {
		behind, err := p.ancestor(ctx, node.worktree.Path, node.final, target)
		if err != nil {
			blockSyncNode(node, err.Error())
			return
		}
		switch {
		case behind:
			step.Action = "ff"
			node.final = target
		case strategy == "rebase":
			if err := p.checkRebaseRange(ctx, node, target); err != nil {
				blockSyncNode(node, err.Error())
				return
			}
			step.Action = "rebase"
		default:
			blockSyncNode(node, "diverged from "+source+" (ff-only)")
			return
		}
	}
	node.result.Steps = append(node.result.Steps, step)
}

func (p *WorktreeSyncPlan) checkRebaseRange(ctx context.Context, node *worktreeSyncNode, target string) error {
	if _, err := p.output(ctx, node.worktree.Path, "merge-base", node.final, target); err != nil {
		return fmt.Errorf("cannot rebase unrelated histories: %w", err)
	}
	merges, err := p.output(ctx, node.worktree.Path, "rev-list", "--merges", target+".."+node.final)
	if err != nil {
		return err
	}
	if strings.TrimSpace(merges) != "" {
		return fmt.Errorf("rebase of local merge commits is not supported")
	}
	return nil
}

// ExecuteWorktreeSync applies a plan once. It keeps partial success and leaves a
// conflicting rebase in place for the operator; it never stashes or pushes.
func ExecuteWorktreeSync(ctx context.Context, plan *WorktreeSyncPlan) error {
	if plan == nil || plan.git == nil {
		return fmt.Errorf("worktree sync requires a generated plan")
	}
	for _, node := range plan.nodes {
		if node.result.Status != "planned" {
			continue
		}
		if node.parent != nil && node.parent.result.Status == "blocked" {
			blockSyncNode(node, "parent "+node.parent.result.Branch+" is blocked")
			continue
		}
		if plan.DryRun {
			continue
		}
		plan.executeNode(ctx, node)
	}
	for _, result := range plan.Results {
		if result.Status == "blocked" {
			return fmt.Errorf("worktree sync incomplete: inspect blocked worktrees")
		}
	}
	return nil
}

func (p *WorktreeSyncPlan) executeNode(ctx context.Context, node *worktreeSyncNode) {
	for i := range node.result.Steps {
		step := &node.result.Steps[i]
		if err := p.safeWorktree(ctx, node, step.From); err != nil {
			blockSyncNode(node, err.Error())
			return
		}
		actual, err := p.resolve(ctx, node.worktree.Path, step.Source)
		if err != nil || actual != step.To {
			blockSyncNode(node, "source changed since planning: "+step.Source)
			return
		}
		if step.Action != "up-to-date" {
			args := []string{"-c", "merge.autoStash=false", "-c", "rebase.autoStash=false", "-c", "rebase.updateRefs=false", "-c", "submodule.recurse=false"}
			if step.Action == "ff" {
				args = append(args, "merge", "--ff-only", "--no-edit", "--no-autostash", step.To)
			} else {
				args = append(args, "rebase", "--no-autostash", "--no-update-refs", "--no-autosquash", "--no-fork-point", step.To)
			}
			if _, err := p.output(ctx, node.worktree.Path, args...); err != nil {
				step.Status = "failed"
				blockSyncNode(node, err.Error()+"; resolve the Git operation in "+node.worktree.Path)
				return
			}
		}
		step.Status = "done"
	}
	node.result.Status = "done"
}
