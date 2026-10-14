// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// GitReclaimProof contains actual Git facts from the existing target-locked
// reclaim. It carries no execution, generation, owner, or authorization ledger.
// The native caller saves this optional JSON object from the run protocol.
type GitReclaimProof struct {
	RepositoryCommonDir string                 `json:"repository_common_dir"`
	TaskOID             string                 `json:"task_oid"`
	TaskBranch          string                 `json:"task_branch"`
	TaskWorktree        string                 `json:"task_worktree"`
	ObservedTarget      reclaimObservedTarget  `json:"observed_target"`
	Proofs              *gitReclaimChecks      `json:"proofs"`
	ResourceBefore      reclaimBeforeInventory `json:"resource_before"`
	Action              reclaimAction          `json:"action"`
	After               reclaimAfterInventory  `json:"after"`
}

type reclaimObservedTarget struct {
	Ref          string `json:"ref"`
	OID          string `json:"oid"`
	PushEndpoint string `json:"push_endpoint"`
}

type gitReclaimChecks struct {
	TaskOIDPushedFull           bool `json:"task_oid_pushed_full"`
	RemoteTargetContainsTaskOID bool `json:"remote_target_contains_task_oid"`
	TargetAndRemoteExactMatch   bool `json:"target_and_remote_exact_match"`
	WorktreeNonPrimary          bool `json:"worktree_non_primary"`
	WorktreeClean               bool `json:"worktree_clean"`
	WorktreeUnlocked            bool `json:"worktree_unlocked"`
	SafePath                    bool `json:"safe_path"`
	TaskPattern                 bool `json:"task_pattern"`
	ProtectedRefs               bool `json:"protected_refs"`
	InventoryExactMatch         bool `json:"inventory_exact_match"`
}

func (p gitReclaimChecks) complete() bool {
	return p.TaskOIDPushedFull && p.RemoteTargetContainsTaskOID && p.TargetAndRemoteExactMatch &&
		p.WorktreeNonPrimary && p.WorktreeClean && p.WorktreeUnlocked && p.SafePath &&
		p.TaskPattern && p.ProtectedRefs && p.InventoryExactMatch
}

type reclaimBeforeInventory struct {
	RepositoryCommonDir string            `json:"repository_common_dir"`
	TaskOID             string            `json:"task_oid"`
	Branch              string            `json:"branch"`
	Worktree            string            `json:"worktree"`
	LocalRef            string            `json:"local_ref"`
	RemoteRef           string            `json:"remote_ref"`
	LocalOID            string            `json:"local_oid"`
	RemoteOID           string            `json:"remote_oid"`
	Safety              *gitReclaimChecks `json:"safety"`
}

type reclaimAction struct {
	ExitCode int      `json:"exit_code"`
	Done     []string `json:"done"`
	Failed   []string `json:"failed"`
}

type reclaimAfterInventory struct {
	WorktreeAbsent     bool `json:"worktree_absent"`
	LocalBranchAbsent  bool `json:"local_branch_absent"`
	RemoteBranchAbsent bool `json:"remote_branch_absent"`
}

func captureGitReclaimProof(ctx context.Context, g gitRepo, opts reclaimOpts, taskWT, mainWT string, trees []listedWorktree) (*GitReclaimProof, error) {
	before, checks, err := captureReclaimInventory(ctx, g, opts, taskWT, mainWT, trees)
	if err != nil {
		return nil, err
	}
	target, err := captureReclaimRemote(ctx, g, opts, &before, checks)
	if err != nil {
		return nil, err
	}
	if !checks.complete() {
		return nil, fmt.Errorf("pre-delete Git safety proof is incomplete: %+v", *checks)
	}
	if err := verifyGitOnlyReclaimScope(before.RepositoryCommonDir, taskWT); err != nil {
		return nil, err
	}
	return &GitReclaimProof{
		RepositoryCommonDir: before.RepositoryCommonDir, TaskOID: opts.TaskSHA,
		TaskBranch: opts.Branch, TaskWorktree: taskWT, ObservedTarget: target,
		Proofs: checks, ResourceBefore: before,
	}, nil
}

func captureReclaimInventory(ctx context.Context, g gitRepo, opts reclaimOpts, taskWT, mainWT string, trees []listedWorktree) (reclaimBeforeInventory, *gitReclaimChecks, error) {
	checks := &gitReclaimChecks{}
	before := reclaimBeforeInventory{
		TaskOID: opts.TaskSHA, Branch: opts.Branch, Worktree: taskWT,
		LocalRef: "refs/heads/" + opts.Branch, RemoteRef: "refs/heads/" + opts.Branch, Safety: checks,
	}
	common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return before, checks, err
	}
	before.RepositoryCommonDir = common
	if taskWT == "" || len(trees) != 1 {
		return before, checks, fmt.Errorf("task worktree inventory is not exact")
	}
	canonical, err := filepath.EvalSymlinks(taskWT)
	if err != nil {
		return before, checks, err
	}
	tg := newGitRepo(g.exec, taskWT)
	taskCommon, err := tg.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return before, checks, err
	}
	status, err := tg.porcelain(ctx)
	if err != nil {
		return before, checks, err
	}
	local, present, err := probeLocalBranch(ctx, g, opts.Branch)
	if err != nil {
		return before, checks, fmt.Errorf("query local task inventory: %w", err)
	}
	if !present {
		return before, checks, fmt.Errorf("local task ref is absent")
	}
	before.LocalOID = local
	checks.WorktreeNonPrimary = taskWT != mainWT
	checks.WorktreeClean = strings.TrimSpace(status) == ""
	checks.WorktreeUnlocked = !trees[0].Locked
	checks.SafePath = filepath.IsAbs(taskWT) && filepath.Clean(taskWT) == taskWT && canonical == taskWT && taskCommon == common
	checks.TaskPattern = reclaimPatternGuardReason(opts) == ""
	checks.ProtectedRefs = reclaimRefGuardReason(opts) == ""
	checks.InventoryExactMatch = trees[0].Path == taskWT && trees[0].Branch == opts.Branch && trees[0].Head == opts.TaskSHA && local == opts.TaskSHA
	return before, checks, nil
}

func captureReclaimRemote(ctx context.Context, g gitRepo, opts reclaimOpts, before *reclaimBeforeInventory, checks *gitReclaimChecks) (reclaimObservedTarget, error) {
	r := opts.Receipt
	remote, err := receiptPushEndpoint(ctx, g, r)
	if err != nil {
		return reclaimObservedTarget{}, err
	}
	selected, err := boundPushEndpoint(ctx, g, opts.Remote, opts.PushRemote)
	if err != nil {
		return reclaimObservedTarget{}, err
	}
	target := reclaimObservedTarget{Ref: "refs/heads/" + opts.TargetBranch}
	target.PushEndpoint = remote
	checks.TargetAndRemoteExactMatch = r.TargetRef == target.Ref && r.TaskSHA == opts.TaskSHA &&
		r.TaskBranch == opts.Branch && r.TaskWorktree == before.Worktree && r.RepositoryCommonDir == before.RepositoryCommonDir && remote == selected
	normalized, err := NormalizeExpectSource(opts.TaskSHA)
	if err != nil || normalized != opts.TaskSHA {
		return target, fmt.Errorf("captured task OID is not a full canonical object ID")
	}
	taskOID, present, err := lsRemoteHead(ctx, g, remote, opts.Branch)
	if err != nil {
		return target, fmt.Errorf("query pushed task inventory: %w", err)
	}
	if !present {
		return target, fmt.Errorf("pushed task ref is absent")
	}
	before.RemoteOID = taskOID
	checks.TaskOIDPushedFull = taskOID == opts.TaskSHA && before.LocalOID == opts.TaskSHA
	targetOID, present, err := lsRemoteHead(ctx, g, remote, opts.TargetBranch)
	if err != nil {
		return target, fmt.Errorf("query live target inventory: %w", err)
	}
	if !present {
		return target, fmt.Errorf("live target ref is absent")
	}
	target.OID = targetOID
	contains, err := g.isAncestor(ctx, opts.TaskSHA, targetOID)
	if err != nil {
		return target, fmt.Errorf("cannot prove observed target %s contains the task: %w", targetOID, err)
	}
	checks.RemoteTargetContainsTaskOID = contains
	return target, nil
}

func verifyGitOnlyReclaimScope(common, taskWT string) error {
	if err := filepath.WalkDir(taskWT, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == filepath.Join(taskWT, ".git") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == ".git" {
			return fmt.Errorf("nested repository requires caller-owned cleanup: %s", path)
		}
		if entry.IsDir() {
			bare, err := bareRepositoryLayout(path)
			if err != nil {
				return err
			}
			if bare {
				return fmt.Errorf("bare repository requires caller-owned cleanup: %s", path)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return refuseTaskSiblingLease(common, taskWT)
}

func bareRepositoryLayout(path string) (bool, error) {
	for _, name := range []string{"HEAD", "objects", "refs"} {
		info, err := os.Stat(filepath.Join(path, name))
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if (name == "HEAD" && !info.Mode().IsRegular()) || (name != "HEAD" && !info.IsDir()) {
			return false, nil
		}
	}
	return true, nil
}

func refuseTaskSiblingLease(common, taskWT string) error {
	dir := filepath.Join(common, "sibling-provision-leases")
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("sibling lease directory is not a plain directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("sibling lease is not a regular readable receipt")
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name())) // #nosec G304 -- regular receipt under verified Git common directory
		if err != nil {
			return err
		}
		var lease SiblingLease
		if err := json.Unmarshal(data, &lease); err != nil {
			return err
		}
		if filepath.Clean(lease.Root) == taskWT {
			return fmt.Errorf("provisioned sibling lease requires caller-owned cleanup: %s", entry.Name())
		}
	}
	return nil
}

func finishGitReclaimProof(ctx context.Context, g gitRepo, opts reclaimOpts, out *ReclaimResult) {
	p := out.GitProof
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		out.Failed = append(out.Failed, "verify worktree absence: "+err.Error())
	} else {
		_, statErr := os.Lstat(p.TaskWorktree)
		p.After.WorktreeAbsent = os.IsNotExist(statErr)
		for _, tree := range trees {
			if tree.Path == p.TaskWorktree || tree.Branch == p.TaskBranch {
				p.After.WorktreeAbsent = false
			}
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			out.Failed = append(out.Failed, "verify worktree path absence: "+statErr.Error())
		}
	}
	refs, err := g.output(ctx, "for-each-ref", "--format=%(refname)", "refs/heads/"+opts.Branch)
	if err != nil {
		out.Failed = append(out.Failed, "verify local ref absence: "+err.Error())
	} else {
		p.After.LocalBranchAbsent = !containsExactRef(refs, "refs/heads/"+opts.Branch)
	}
	_, present, err := lsRemoteHead(ctx, g, p.ObservedTarget.PushEndpoint, opts.Branch)
	if err != nil {
		out.Failed = append(out.Failed, "verify remote ref absence: "+err.Error())
	} else {
		p.After.RemoteBranchAbsent = !present
	}
	out.RemovedWorktree = p.After.WorktreeAbsent
	out.RemovedLocalBranch = p.After.LocalBranchAbsent
	out.RemovedRemoteBranch = p.After.RemoteBranchAbsent
	if !out.RemovedWorktree || !out.RemovedLocalBranch || !out.RemovedRemoteBranch {
		out.Failed = append(out.Failed, "reclaim resources remain or absence is unproven")
	}
	if out.Incomplete() {
		p.Action.ExitCode = 3
	}
	p.Action.Done = append([]string(nil), out.Done...)
	p.Action.Failed = append([]string(nil), out.Failed...)
}

func containsExactRef(lines, ref string) bool {
	for _, line := range strings.Split(lines, "\n") {
		if strings.TrimSpace(line) == ref {
			return true
		}
	}
	return false
}
