// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"context"
	"fmt"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// headObservingLease stamps every durable record with the HEAD Git reports at
// that step. The owner compares ActualHEAD against its own read-only observation,
// so an unobserved or derived value would make every record fail.
type headObservingLease struct {
	ApplyLease
	exec     *gitcmd.Executor
	worktree string
}

// Record refuses to log progress whose HEAD cannot be observed.
func (lease headObservingLease) Record(ctx context.Context, record ApplyRecord) error {
	actual, err := oid(ctx, lease.exec, lease.worktree, "HEAD")
	if err != nil {
		return fmt.Errorf("observe task HEAD for %s: %w", record.Step, err)
	}
	record.ActualHEAD = actual
	return lease.ApplyLease.Record(ctx, record)
}

func applyMutation(ctx context.Context, exec *gitcmd.Executor, p ApplyPlan, lease ApplyLease, record ApplyRecord, step string, args ...string) error {
	if err := recordAndCheck(ctx, lease, record, step); err != nil {
		return err
	}
	if err := safeApplySurfaces(p); err != nil {
		return err
	}
	if err := safeCheckpointSurface(p.CommonDir, record.CheckpointRef[len(checkpointPrefix):]); err != nil {
		return err
	}
	if err := verifyApplyBoundary(ctx, exec, p, record, step); err != nil {
		return err
	}
	_, err := run(ctx, exec, p.TaskWorktree, args...)
	return err
}

func verifyApplyBoundary(ctx context.Context, exec *gitcmd.Executor, p ApplyPlan, record ApplyRecord, step string) error {
	head, err := oid(ctx, exec, p.TaskWorktree, "HEAD")
	if err != nil {
		return err
	}
	branch, err := oid(ctx, exec, p.TaskWorktree, "refs/heads/"+p.TaskBranch)
	if err != nil {
		return err
	}
	attached, attachErr := runResult(ctx, exec, p.TaskWorktree, "symbolic-ref", "--quiet", "HEAD")
	if attachErr != nil || attached == nil || (attached.ExitCode != 0 && attached.ExitCode != 1) {
		return fmt.Errorf("task attachment unavailable")
	}
	wantHead, wantBranch, wantAttached := record.OriginalHEAD, record.OriginalHEAD, "refs/heads/"+p.TaskBranch+"\n"
	switch step {
	case "branch-intent":
		wantHead, wantAttached = record.CandidateOID, ""
	case "attach-intent":
		wantHead, wantBranch, wantAttached = record.CandidateOID, record.CandidateOID, ""
	}
	if head != wantHead || branch != wantBranch || attached.Stdout != wantAttached {
		return fmt.Errorf("task state changed before mutation")
	}
	source, err := oid(ctx, exec, p.CanonicalWorktree, p.SourceRef)
	if err != nil || source != p.ExpectedSourceOID {
		return fmt.Errorf("source changed before mutation")
	}
	if step != "checkpoint-intent" {
		checkpoint, err := oid(ctx, exec, p.CanonicalWorktree, record.CheckpointRef)
		if err != nil || checkpoint != record.OriginalHEAD {
			return fmt.Errorf("original checkpoint is unavailable")
		}
	}
	return cleanStatus(ctx, exec, p.request(head))
}
