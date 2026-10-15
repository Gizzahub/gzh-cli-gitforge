// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// errTargetNotContaining is the definitive negative: the remote target was
// queried live and provably does not contain the task commit. It is distinct
// from an unknown answer, which is never evidence of anything.
var errTargetNotContaining = errors.New("remote target provably does not contain the task commit")

func (r *Receipt) pushRemote() string {
	if r.PushRemote != "" {
		return r.PushRemote
	}
	return r.Remote
}

func receiptPushEndpoint(ctx context.Context, g gitRepo, receipt *Receipt) (string, error) {
	return boundPushEndpoint(ctx, g, receipt.Remote, receipt.PushRemote)
}

// receiptTargetBranch validates and extracts the plain target branch name the
// receipt's refs/heads target ref names.
func receiptTargetBranch(receipt *Receipt) (string, error) {
	name := strings.TrimPrefix(receipt.TargetRef, "refs/heads/")
	if name == receipt.TargetRef {
		return "", fmt.Errorf("receipt target_ref %q is not a refs/heads ref", receipt.TargetRef)
	}
	if err := gitcmd.SanitizeBranchName(name); err != nil {
		return "", fmt.Errorf("receipt target_ref: %w", err)
	}
	if err := gitcmd.SanitizeBranchName(receipt.TaskBranch); err != nil {
		return "", fmt.Errorf("receipt task_branch: %w", err)
	}
	return name, nil
}

// refuseLiveOrUnknownClaim blocks cleanup while the receipt's last mutating
// process may still be running. A lease age never authorizes stealing: only a
// positively dead holder lets cleanup proceed. There is deliberately no
// same-PID exemption — a claim PID equal to os.Getpid() is just as likely a
// reused PID stamped by an earlier process, and reuse is indistinguishable
// from self, so it fails closed like any other live holder. The physical
// process host is used for liveness only, never as owner identity.
func refuseLiveOrUnknownClaim(receipt *Receipt) error {
	if receipt.Phase == evidencePhaseComplete {
		return nil
	}
	if receipt.Claim.PID <= 0 || receipt.Claim.ProcessHost == "" || receipt.Claim.ProcessHost != physicalHost() {
		return fmt.Errorf("%w: claim pid=%d process_host=%q", ErrCleanupClaimUnknown, receipt.Claim.PID, receipt.Claim.ProcessHost)
	}
	alive, err := processAlive(receipt.Claim.PID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCleanupClaimUnknown, err)
	}
	if alive {
		return fmt.Errorf("%w: pid %d on %s", ErrCleanupClaimLive, receipt.Claim.PID, receipt.Claim.ProcessHost)
	}
	return nil
}

// cleanupGuards applies the same protected/integration/task-pattern rules as
// reclaim, using the run's recorded provenance — the declaration and
// controller the caller approved, not whatever the dying worktree says now.
func cleanupGuards(receipt *Receipt, targetName string) error {
	if receipt.PushRemote == "" {
		return fmt.Errorf("%w: run did not record its bound push endpoint", ErrCleanupGuard)
	}
	if receipt.Provenance == nil || receipt.Provenance.ReclaimRefs == nil {
		return fmt.Errorf("%w: run did not record resolved reclaim branch names", ErrCleanupGuard)
	}
	refs := receipt.Provenance.ReclaimRefs
	if reason := reclaimRefGuardReason(reclaimOpts{
		Branch: receipt.TaskBranch, TargetBranch: targetName,
		DefaultName: refs.Default, Integration: refs.Integration, Remote: receipt.Remote,
	}); reason != "" {
		return fmt.Errorf("%w: %s", ErrCleanupGuard, reason)
	}
	patterns := receipt.ProvenanceControllerPatterns()
	if len(patterns) == 0 {
		return fmt.Errorf("%w: run recorded no taskPattern declaration", ErrCleanupGuard)
	}
	if !config.MatchesAnyTaskPattern(receipt.TaskBranch, patterns) {
		return fmt.Errorf("%w: %s does not match taskPattern", ErrCleanupGuard, receipt.TaskBranch)
	}
	return nil
}

// ProvenanceControllerPatterns returns the controller task patterns when a
// controller authorized the run, matching finishRunReclaim's precedence.
func (r *Receipt) ProvenanceControllerPatterns() []string {
	if r.Provenance == nil {
		return nil
	}
	if r.Provenance.Controller != nil {
		return r.Provenance.Controller.TaskPattern
	}
	return r.Provenance.DeclarationPatterns
}

// proveRemoteTargetContainsTask proves the actual remote target contains the
// captured task SHA: the tip is read live with ls-remote, and either equals
// the task SHA or is locally resolvable history that contains it. Anything
// else is unknown, and unknown is a refusal.
func proveRemoteTargetContainsTask(ctx context.Context, g gitRepo, receipt *Receipt, targetName, remote string) (bool, error) {
	if remote == "" {
		return false, fmt.Errorf("%w: receipt records no remote", ErrCleanupProofUnknown)
	}
	tip, exists, err := lsRemoteHead(ctx, g, remote, targetName)
	if err != nil {
		return false, fmt.Errorf("%w: query remote target %s: %w", ErrCleanupProofUnknown, targetName, err)
	}
	if !exists {
		return false, fmt.Errorf("%w: remote target %s does not exist", ErrCleanupProofUnknown, targetName)
	}
	if tip == receipt.TaskSHA {
		return true, nil
	}
	tipSHA, ok, err := g.revParse(ctx, tip)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrCleanupProofUnknown, err)
	}
	if !ok {
		return false, fmt.Errorf("%w: remote target %s advanced to %s, which this repository does not know; land or fetch that history, then retry", ErrCleanupProofUnknown, targetName, tip)
	}
	contains, err := g.isAncestor(ctx, receipt.TaskSHA, tipSHA)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrCleanupProofUnknown, err)
	}
	if !contains {
		return false, fmt.Errorf("%w: %w: remote target %s (%s) does not contain the task commit %s", ErrCleanupProofUnknown, errTargetNotContaining, targetName, tip, receipt.TaskSHA)
	}
	return true, nil
}

// probeLocalBranch is an explicit successful ref probe: exit 0 from
// rev-parse --verify proves the branch exists; anything else reports it
// missing without guessing from tracking refs.
func probeLocalBranch(ctx context.Context, g gitRepo, branch string) (sha string, present bool, err error) {
	ref := "refs/heads/" + branch
	if err := refuseSymbolicTaskRef(ctx, g, ref); err != nil {
		return "", false, err
	}
	res, err := g.output(ctx, "for-each-ref", "--format=%(refname)%09%(objectname)", ref)
	if err != nil {
		return "", false, err
	}
	for _, line := range strings.Split(res, "\n") {
		name, oid, ok := strings.Cut(line, "\t")
		if name != ref {
			continue
		}
		if !ok {
			return "", false, fmt.Errorf("malformed local ref inventory for %s", ref)
		}
		sha, err := NormalizeExpectSource(oid)
		if err != nil {
			return "", false, fmt.Errorf("invalid local ref inventory for %s: %w", ref, err)
		}
		return sha, true, nil
	}
	return "", false, nil
}

func refuseSymbolicTaskRef(ctx context.Context, g gitRepo, ref string) error {
	res, err := g.run(ctx, "symbolic-ref", "--quiet", ref)
	if err != nil {
		return err
	}
	if res.ExitCode == 0 {
		return fmt.Errorf("task ref is unexpectedly symbolic: %s -> %s", ref, strings.TrimSpace(res.Stdout))
	}
	if res.ExitCode != 1 {
		return fmt.Errorf("cannot prove direct task ref %s: %s", ref, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// validateWorktreeResidual proves the physical worktree recorded in the
// receipt is still this execution's worktree: listed, on the task branch,
// registration-marked for this execution and generation, and clean.
func validateWorktreeResidual(ctx context.Context, exec *gitcmd.Executor, g gitRepo, receipt *Receipt) (bool, error) {
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return false, err
	}
	var listed bool
	for _, wt := range trees {
		if wt.Branch == receipt.TaskBranch && wt.Path != receipt.TaskWorktree {
			return false, fmt.Errorf("%w: task branch is held by another worktree: %s", ErrCleanupRegistration, wt.Path)
		}
		if wt.Path == receipt.TaskWorktree {
			listed = true
			if wt.Locked {
				return false, fmt.Errorf("%w: task worktree is locked: %s", ErrCleanupRegistration, wt.Path)
			}
			if wt.Branch != receipt.TaskBranch {
				return false, fmt.Errorf("%w: worktree %s is on branch %s, not the task branch %s", ErrCleanupRegistration, receipt.TaskWorktree, wt.Branch, receipt.TaskBranch)
			}
		}
	}
	if !listed {
		if receipt.TaskWorktree == "" {
			return false, nil
		}
		if _, err := os.Lstat(receipt.TaskWorktree); err == nil {
			return false, fmt.Errorf("%w: unregistered residual path %s", ErrCleanupRegistration, receipt.TaskWorktree)
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("%w: probe residual path %s: %w", ErrCleanupRegistration, receipt.TaskWorktree, err)
		}
		return false, nil
	}
	marker, ok, err := readRegistrationMarker(ctx, exec, receipt.TaskWorktree)
	if err != nil {
		return false, err
	}
	want := registrationMarker{SchemaVersion: registrationSchemaVersion, ExecutionID: receipt.ExecutionID, Generation: receipt.Generation, TaskSHA: receipt.TaskSHA}
	if !ok || marker != want {
		return false, fmt.Errorf("%w: %s", ErrCleanupRegistration, receipt.TaskWorktree)
	}
	porcelain, err := newGitRepo(exec, receipt.TaskWorktree).porcelain(ctx)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(porcelain) != "" {
		return false, fmt.Errorf("%w: %s", ErrCleanupDirtyWorktree, receipt.TaskWorktree)
	}
	common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return false, err
	}
	if err := verifyGitOnlyReclaimScope(common, receipt.TaskWorktree); err != nil {
		return false, fmt.Errorf("%w: caller-owned resources: %w", ErrCleanupRegistration, err)
	}
	return true, nil
}

func refuseCheckedOutTaskBranch(ctx context.Context, g gitRepo, branch string) error {
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return err
	}
	for _, wt := range trees {
		if wt.Branch == branch {
			return fmt.Errorf("task branch is still checked out at %s", wt.Path)
		}
	}
	return nil
}

// lsRemoteHead queries one remote head live. exists=false is reported only
// by a successful empty query, never by a stale tracking ref.
func lsRemoteHead(ctx context.Context, g gitRepo, remote, branch string) (tip string, exists bool, err error) {
	res, err := g.run(ctx, "ls-remote", remote, "refs/heads/"+branch)
	if err != nil {
		return "", false, err
	}
	if res.ExitCode != 0 {
		return "", false, fmt.Errorf("git ls-remote %s refs/heads/%s failed: %s", remote, branch, strings.TrimSpace(res.Stderr))
	}
	for _, line := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == "refs/heads/"+branch {
			return fields[0], true, nil
		}
	}
	return "", false, nil
}

func residualNames(receipt *Receipt, worktree, local, remote bool) []string {
	var names []string
	if worktree {
		names = append(names, "worktree("+receipt.TaskWorktree+")")
	}
	if local {
		names = append(names, "local-branch("+receipt.TaskBranch+")")
	}
	if remote {
		names = append(names, "remote-branch("+receipt.TaskBranch+")")
	}
	return names
}
