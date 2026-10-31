// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

const checkpointPrefix = "refs/gz-git/tasksync-checkpoints/"

// ApplyPlan binds a single local task-branch apply operation.
type ApplyPlan struct {
	PlanID, GenerationDigest, AuthorityDigest, ProviderBindingDigest  string
	CanonicalWorktree, TaskWorktree, CommonDir, TaskBranch, SourceRef string
	ExpectedTaskHEAD, ExpectedSourceOID                               string
	AllowedPaths, ProtectedPaths                                      []string
}

// ApplyFenceProvider obtains the exclusive caller-owned apply lease.
type ApplyFenceProvider interface {
	Acquire(context.Context, ApplyPlan) (ApplyLease, error)
}

// ApplyLease carries both the exclusive fence and durable owner progress log.
type ApplyLease interface {
	Check(context.Context) error
	Record(context.Context, ApplyRecord) error
	Close() error
}

// ApplyRecord is a durable owner event before or after each mutation.
type ApplyRecord struct{ PlanID, Step, CheckpointRef, CandidateOID, OriginalHEAD, ActualHEAD string }

// ApplyRequest supplies a prepared candidate and mandatory owner fence.
type ApplyRequest struct {
	Plan      ApplyPlan
	Candidate CandidateDescriptor
	Fence     ApplyFenceProvider
}

// ApplyResult reports observed branch state; Applied is never true on uncertainty.
type ApplyResult struct {
	Outcome                                         Outcome
	Reason, CheckpointRef, OriginalHEAD, ActualHEAD string
	Applied                                         bool
}

// BindingDigest returns the mechanical plan/candidate binding, excluding grant digests.
func BindingDigest(plan ApplyPlan, candidate CandidateDescriptor) string {
	paths := append([]string{}, plan.AllowedPaths...)
	sort.Strings(paths)
	protected := append([]string{}, plan.ProtectedPaths...)
	sort.Strings(protected)
	plan.AllowedPaths, plan.ProtectedPaths = paths, protected
	plan.AuthorityDigest, plan.ProviderBindingDigest = "", ""
	value, err := json.Marshal(struct {
		Plan      ApplyPlan
		Candidate CandidateDescriptor
	}{plan, candidate})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

// ApplyLocal applies a pinned candidate only while the supplied owner lease remains valid.
func ApplyLocal(ctx context.Context, request ApplyRequest) (result ApplyResult) {
	lease, refused := acquireApplyLease(ctx, request)
	if refused != nil {
		return *refused
	}
	defer closeApplyLease(lease, &result)
	if err := validateApply(request); err != nil {
		return applyRefused(err.Error())
	}
	if err := lease.Check(ctx); err != nil {
		return applyUncertain("apply fence check failed")
	}
	if err := safeApplySurfaces(request.Plan); err != nil {
		return applyRefused(err.Error())
	}
	candidate, err := Restore(ctx, request.Plan.CanonicalWorktree, request.Candidate)
	if err != nil {
		return applyRefused("prepared candidate no longer matches")
	}
	exec, err := newExecutor()
	if err != nil {
		return applyRefused(err.Error())
	}
	state, err := inspect(ctx, exec, request.Plan.request(request.Plan.ExpectedTaskHEAD))
	if !applyInitialStateMatches(state, candidate, err) {
		return applyRefused("task or source binding changed")
	}
	if err := verifyApplyPaths(ctx, exec, request.Plan, candidate.CandidateOID); err != nil {
		return applyRefused(err.Error())
	}
	lease = headObservingLease{ApplyLease: lease, exec: exec, worktree: request.Plan.TaskWorktree}
	handle, err := randomHandle()
	if err != nil {
		return applyUncertain("checkpoint handle unavailable")
	}
	checkpoint := checkpointPrefix + handle
	if err := safeCheckpointSurface(request.Plan.CommonDir, handle); err != nil {
		return applyRefused(err.Error())
	}
	defer observeApplyResult(ctx, exec, request.Plan.TaskWorktree, checkpoint, candidate.OriginalHEAD, &result)
	base := ApplyRecord{PlanID: request.Plan.PlanID, CheckpointRef: checkpoint, CandidateOID: candidate.CandidateOID, OriginalHEAD: candidate.OriginalHEAD}
	if err := applyMutation(ctx, exec, request.Plan, lease, base, "checkpoint-intent", "update-ref", "--no-deref", checkpoint, candidate.OriginalHEAD, strings.Repeat("0", len(candidate.OriginalHEAD))); err != nil {
		return applyUncertain("checkpoint creation outcome is uncertain")
	}
	if err := recordAndCheck(ctx, lease, base, "checkpoint-created"); err != nil {
		return applyUncertain("checkpoint progress was not recorded")
	}
	if err := applyMutation(ctx, exec, request.Plan, lease, base, "checkout-intent", "checkout", "--detach", "--no-overwrite-ignore", candidate.CandidateOID); err != nil {
		return applyUncertain("checkout outcome is uncertain")
	}
	if err := recordAndCheck(ctx, lease, base, "checked-out"); err != nil {
		return applyUncertain("checkout progress was not recorded")
	}
	if err := applyMutation(ctx, exec, request.Plan, lease, base, "branch-intent", "update-ref", "--no-deref", "refs/heads/"+request.Plan.TaskBranch, candidate.CandidateOID, candidate.OriginalHEAD); err != nil {
		return applyUncertain("branch update outcome is uncertain")
	}
	if err := recordAndCheck(ctx, lease, base, "branch-updated"); err != nil {
		return applyUncertain("branch progress was not recorded")
	}
	if err := applyMutation(ctx, exec, request.Plan, lease, base, "attach-intent", "symbolic-ref", "HEAD", "refs/heads/"+request.Plan.TaskBranch); err != nil {
		return applyUncertain("branch attachment outcome is uncertain")
	}
	actual, actualErr := oid(ctx, exec, request.Plan.TaskWorktree, "HEAD")
	if actualErr != nil {
		return applyUncertain("final task head cannot be observed")
	}
	if err := recordAndCheck(ctx, lease, ApplyRecord{PlanID: request.Plan.PlanID, Step: "attached", CheckpointRef: checkpoint, CandidateOID: candidate.CandidateOID, OriginalHEAD: candidate.OriginalHEAD, ActualHEAD: actual}, "attached"); err != nil {
		return applyUncertain("attachment progress was not recorded")
	}
	if verifyApplyCompletion(ctx, exec, request.Plan, base, actual) != nil {
		return applyUncertain("final task/source pair changed")
	}
	return ApplyResult{Outcome: OutcomeApplied, CheckpointRef: checkpoint, OriginalHEAD: candidate.OriginalHEAD, ActualHEAD: actual, Applied: actual == candidate.CandidateOID}
}

func applyInitialStateMatches(state snapshot, candidate *PreparedCandidate, err error) bool {
	return err == nil && state.TaskHEAD == candidate.OriginalHEAD
}

func closeApplyLease(lease ApplyLease, result *ApplyResult) {
	if err := lease.Close(); err != nil {
		result.Outcome, result.Applied = OutcomeUncertain, false
		result.Reason = "apply lease closure failed; retain actual effects"
	}
}

func observeApplyResult(ctx context.Context, exec *gitcmd.Executor, worktree, checkpoint, original string, result *ApplyResult) {
	result.CheckpointRef, result.OriginalHEAD = checkpoint, original
	// A bounded read preserves partial effects even after cancellation. It
	// never retries a mutation or rolls one back.
	observeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if actual, err := oid(observeCtx, exec, worktree, "HEAD"); err == nil {
		if result.Applied && actual != result.ActualHEAD {
			result.Outcome, result.Applied = OutcomeUncertain, false
			result.Reason = "task HEAD changed during final observation"
		}
		result.ActualHEAD = actual
	} else if result.Applied {
		result.Outcome, result.Applied = OutcomeUncertain, false
		result.Reason = "final task HEAD observation failed"
	}
}

func verifyApplyCompletion(ctx context.Context, exec *gitcmd.Executor, plan ApplyPlan, record ApplyRecord, actual string) error {
	if actual != record.CandidateOID {
		return fmt.Errorf("final task HEAD differs")
	}
	if err := verifyAppliedRefs(ctx, exec, plan, record.CandidateOID); err != nil {
		return err
	}
	if err := safeCheckpointSurface(plan.CommonDir, strings.TrimPrefix(record.CheckpointRef, checkpointPrefix)); err != nil {
		return err
	}
	checkpoint, err := oid(ctx, exec, plan.CanonicalWorktree, record.CheckpointRef)
	if err != nil || checkpoint != record.OriginalHEAD {
		return fmt.Errorf("final checkpoint differs from original HEAD")
	}
	_, err = inspect(ctx, exec, plan.request(record.CandidateOID))
	return err
}

func acquireApplyLease(ctx context.Context, request ApplyRequest) (ApplyLease, *ApplyResult) {
	if request.Fence == nil {
		result := applyRefused("missing apply fence")
		return nil, &result
	}
	lease, err := request.Fence.Acquire(ctx, request.Plan)
	if err != nil || lease == nil {
		result := applyRefused("apply fence was denied")
		return nil, &result
	}
	return lease, nil
}

func recordAndCheck(ctx context.Context, lease ApplyLease, record ApplyRecord, step string) error {
	record.Step = step
	if err := lease.Check(ctx); err != nil {
		return err
	}
	if err := lease.Record(ctx, record); err != nil {
		return err
	}
	return lease.Check(ctx)
}

func applyRefused(reason string) ApplyResult {
	return ApplyResult{Outcome: OutcomeRefused, Reason: reason}
}

func applyUncertain(reason string) ApplyResult {
	return ApplyResult{Outcome: OutcomeUncertain, Reason: reason}
}

func validateApply(r ApplyRequest) error {
	p := r.Plan
	if err := validateRequest(p.request(p.ExpectedTaskHEAD)); err != nil {
		return err
	}
	if p.PlanID == "" || p.GenerationDigest == "" || p.AuthorityDigest == "" || p.SourceRef == "" || p.ProviderBindingDigest != BindingDigest(p, r.Candidate) {
		return fmt.Errorf("invalid apply plan")
	}
	if r.Candidate.OriginalHEAD != p.ExpectedTaskHEAD || r.Candidate.SourceOID != p.ExpectedSourceOID || r.Candidate.CommonDir != p.CommonDir {
		return fmt.Errorf("candidate differs from pinned task/source identity")
	}
	return nil
}

func (p ApplyPlan) request(head string) Request {
	return Request{CanonicalWorktree: p.CanonicalWorktree, TaskWorktree: p.TaskWorktree, CommonDir: p.CommonDir, TaskBranch: p.TaskBranch, ExpectedTaskHEAD: head, SourceRef: p.SourceRef, SourceOID: p.ExpectedSourceOID}
}

func verifyAppliedRefs(ctx context.Context, exec *gitcmd.Executor, plan ApplyPlan, candidate string) error {
	branch, err := oid(ctx, exec, plan.TaskWorktree, "refs/heads/"+plan.TaskBranch)
	if err != nil || branch != candidate {
		return fmt.Errorf("task branch differs from candidate")
	}
	source, err := oid(ctx, exec, plan.CanonicalWorktree, plan.SourceRef)
	if err != nil || source != plan.ExpectedSourceOID {
		return fmt.Errorf("source differs from plan")
	}
	return nil
}

func verifyApplyPaths(ctx context.Context, exec *gitcmd.Executor, plan ApplyPlan, candidate string) error {
	raw, err := output(ctx, exec, plan.TaskWorktree, "diff", "--no-renames", "--no-ext-diff", "--no-textconv", "--raw", "-z", plan.ExpectedTaskHEAD, candidate)
	if err != nil {
		return err
	}
	for _, record := range strings.Split(raw, "\x00") {
		if !strings.HasPrefix(record, ":") {
			continue
		}
		fields := strings.Fields(record)
		if len(fields) < 3 || !allowedMode(fields[0][1:]) || !allowedMode(fields[1]) {
			return fmt.Errorf("candidate has unsupported changed mode")
		}
	}
	changed, err := output(ctx, exec, plan.TaskWorktree, "diff", "--no-renames", "--no-ext-diff", "--no-textconv", "--name-only", "-z", plan.ExpectedTaskHEAD, candidate)
	if err != nil {
		return err
	}
	paths := strings.Split(strings.TrimSuffix(changed, "\x00"), "\x00")
	sort.Strings(paths)
	allowed := append([]string{}, plan.AllowedPaths...)
	sort.Strings(allowed)
	if strings.Join(paths, "\x00") != strings.Join(allowed, "\x00") {
		return fmt.Errorf("candidate paths differ from plan")
	}
	for _, path := range paths {
		if filepath.IsAbs(path) || path == "" || strings.HasPrefix(path, ".git/") {
			return fmt.Errorf("unsupported candidate path")
		}
		for _, prefix := range plan.ProtectedPaths {
			if path == prefix || strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/") {
				return fmt.Errorf("candidate changes protected path")
			}
		}
	}
	return nil
}

func regularMode(mode string) bool { return strings.HasPrefix(mode, "100") }
func allowedMode(mode string) bool { return mode == "000000" || regularMode(mode) }
