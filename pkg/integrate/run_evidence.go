// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// Run-side evidence recording (TASK-302). A run that carries an execution
// context writes its intent receipt before the push, marks the push
// immediately after the real push and before reclaim, and records the
// reclaim results as they happen. Every write happens while the run's
// original target lock is held, so the receipt sequence is linearizable
// against the destination.

// beginRunEvidence writes the intent receipt. The durable run identity is
// immutable: a receipt already exists for this execution-id only if this same
// identity crashed mid-run, so any difference in generation, owner,
// repository, task, or target is a replay collision and the run pushes
// nothing. The identity check and the write are one atomic step under the
// per-execution write lock: two concurrent begins with the same execution-id
// and different targets hold different target locks, so without that lock
// both would pass the check and the loser would overwrite the winner's
// receipt. With it, the loser fails here — before run proceeds to any push.
func beginRunEvidence(ctx context.Context, exec *gitcmd.Executor, g gitRepo, common string, opts RunOptions, check *CheckReport, sourceSHA, targetName string) (*Receipt, error) {
	e := opts.Execution
	worktree, err := findTaskWorktree(ctx, g, check.Plan.Branch)
	if err != nil {
		return nil, fmt.Errorf("discover task worktree before integration: %w", err)
	}
	// Worktree discovery and provenance read git; they stay outside the
	// execution write lock, which permits file work only.
	candidate := &Receipt{
		SchemaVersion:       evidenceSchemaVersion,
		ExecutionID:         e.ExecutionID,
		Generation:          e.Generation,
		Owner:               e.Owner,
		RepositoryCommonDir: common,
		TaskBranch:          check.Plan.Branch,
		TaskWorktree:        worktree,
		TaskSHA:             sourceSHA,
		TargetRef:           "refs/heads/" + targetName,
		TargetBeforeSHA:     check.Plan.TargetSHA,
		Remote:              check.Plan.Remote,
		PushRemote:          controllerPushRemote(check),
		Phase:               evidencePhaseIntent,
		Provenance:          runProvenance(ctx, g, check),
	}
	lock, err := acquireReceiptWriteLock(ctx, common, e.ExecutionID)
	if err != nil {
		return nil, err
	}
	defer lock.release()
	existing, ok, err := readReceipt(common, e.ExecutionID)
	if err != nil {
		return nil, err
	}
	expect := (*Receipt)(nil)
	if ok {
		if conflicts := existing.identityConflicts(candidate); len(conflicts) > 0 {
			return nil, fmt.Errorf("%w: execution-id %s recorded %s as %s; refusing to replay as %s", ErrEvidenceReplay, e.ExecutionID, strings.Join(conflicts, ", "), describeReceiptIdentity(existing), describeReceiptIdentity(candidate))
		}
		// Same identity may resume only from a crash before the push: a
		// receipt that already pushed or advanced past intent proves the
		// integration happened, and re-running check or push could push work
		// the caller never approved a second time.
		if existing.Phase != evidencePhaseIntent || existing.SourcePushed {
			return nil, fmt.Errorf("%w: execution-id %s already reached phase=%s source_pushed=%t; finish it with `gz-git integrate cleanup --execution-id %s --generation %s` or inspect it with `gz-git integrate receipt --execution-id %s`", ErrEvidenceReplay, e.ExecutionID, existing.Phase, existing.SourcePushed, e.ExecutionID, e.Generation, e.ExecutionID)
		}
		candidate.Revision = existing.Revision
		candidate.SourcePushed = existing.SourcePushed
		candidate.WorktreeRemoved = existing.WorktreeRemoved
		candidate.LocalBranchRemoved = existing.LocalBranchRemoved
		candidate.RemoteBranchRemoved = existing.RemoteBranchRemoved
		if phaseRank(existing.Phase) > phaseRank(candidate.Phase) {
			candidate.Phase = existing.Phase
		}
		expect = existing
	}
	if err := writeReceiptLocked(common, candidate, expect); err != nil {
		return nil, err
	}
	// The registration marker binds the physical worktree to this execution,
	// so a later branch reused at the same commit cannot satisfy this receipt.
	if candidate.TaskWorktree != "" {
		if err := writeRegistrationMarker(ctx, exec, candidate.TaskWorktree, registrationMarker{
			SchemaVersion: evidenceSchemaVersion,
			ExecutionID:   candidate.ExecutionID,
			Generation:    candidate.Generation,
			TaskSHA:       candidate.TaskSHA,
		}); err != nil {
			return nil, err
		}
	}
	return candidate, nil
}

// markEvidencePushed records that the real push happened, before reclaim
// starts. Failing to record this is a run failure even though the push
// succeeded: an unrecorded push is exactly the crash window cleanup exists
// to close.
func markEvidencePushed(ctx context.Context, common string, receipt *Receipt) error {
	receipt.SourcePushed = true
	if phaseRank(receipt.Phase) < phaseRank(evidencePhasePushed) {
		receipt.Phase = evidencePhasePushed
	}
	return writeReceipt(ctx, common, receipt)
}

// recordEvidenceReclaim writes the reclaim results. The receipt completes
// only when all three removals are actually proven — removed now or already
// gone; any skip stays explicitly incomplete at phase=pushed, leaving the
// durable flags for cleanup retry to reconcile from absence probes.
func recordEvidenceReclaim(ctx context.Context, common string, receipt *Receipt, res ReclaimResult) error {
	receipt.WorktreeRemoved = res.RemovedWorktree
	receipt.LocalBranchRemoved = res.RemovedLocalBranch
	receipt.RemoteBranchRemoved = res.RemovedRemoteBranch
	if res.RemovedWorktree && res.RemovedLocalBranch && res.RemovedRemoteBranch {
		receipt.Phase = evidencePhaseComplete
	} else {
		// The phase must track the actual flags in both directions: a
		// complete receipt rewritten with a false flag is no longer complete.
		receipt.Phase = evidencePhasePushed
	}
	return writeReceipt(ctx, common, receipt)
}

func phaseRank(phase string) int {
	switch phase {
	case evidencePhaseComplete:
		return 2
	case evidencePhasePushed:
		return 1
	default:
		return 0
	}
}

// findTaskWorktree locates the worktree holding the task branch. The main
// checkout (first listed) is recorded too — reclaim refuses it, and the
// receipt should say where the branch actually lives.
func findTaskWorktree(ctx context.Context, g gitRepo, branch string) (string, error) {
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return "", err
	}
	for _, wt := range trees {
		if wt.Branch == branch {
			return wt.Path, nil
		}
	}
	return "", nil
}

// runProvenance captures the approved declaration and controller facts at
// intent time, so cleanup judges guards against what was approved, not
// against whatever the dying worktree last said.
func runProvenance(_ context.Context, g gitRepo, check *CheckReport) *evidenceProvenance {
	p := &evidenceProvenance{PrepareProfile: check.PrepareProfile, PrepareInputs: check.PrepareInputs}
	if decl, err := config.LoadRepoRootTaskPattern(g.dir); err == nil {
		p.DeclarationPatterns = append([]string(nil), decl.Patterns...)
		p.DeclarationFacts = append([]string(nil), decl.Facts...)
	}
	if c := check.Controller; c != nil {
		p.Controller = &evidenceControllerProvenance{
			Path:           c.Path,
			Digest:         c.Digest,
			Workspace:      c.Workspace,
			Remote:         c.Remote,
			RemoteURL:      c.RemoteURL,
			Integration:    append([]string(nil), c.Integration...),
			TaskPattern:    append([]string(nil), c.TaskPattern...),
			PrepareProfile: c.PrepareProfile,
		}
	}
	return p
}

func describeReceiptIdentity(r *Receipt) string {
	return fmt.Sprintf("generation=%s owner=%s task=%s target=%s repo=%s", r.Generation, r.Owner, r.TaskSHA, r.TargetRef, r.RepositoryCommonDir)
}
