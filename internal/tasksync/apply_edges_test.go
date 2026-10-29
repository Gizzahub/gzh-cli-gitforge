// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestApplyLocalRejectsBoundInputMismatches(t *testing.T) {
	for name, mutate := range map[string]func(*ApplyPlan, *CandidateDescriptor){
		"source":     func(p *ApplyPlan, _ *CandidateDescriptor) { p.ExpectedSourceOID = strings.Repeat("0", 40) },
		"allowed":    func(p *ApplyPlan, _ *CandidateDescriptor) { p.AllowedPaths = []string{"other"} },
		"protected":  func(p *ApplyPlan, _ *CandidateDescriptor) { p.ProtectedPaths = []string{"shared.txt"} },
		"descriptor": func(_ *ApplyPlan, c *CandidateDescriptor) { c.CandidateOID = strings.Repeat("0", 40) },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, false)
			prepared := PrepareClean(context.Background(), f.request()).Prepared
			plan := ApplyPlan{PlanID: "p", GenerationDigest: "g", AuthorityDigest: "a", ProviderBindingDigest: "b", CanonicalWorktree: f.canonical, TaskWorktree: f.task, CommonDir: f.common, TaskBranch: "task/demo", SourceRef: "main", ExpectedTaskHEAD: f.taskHead, ExpectedSourceOID: f.source, AllowedPaths: []string{"generated/source.txt", "shared.txt"}}
			candidate := prepared.Descriptor()
			mutate(&plan, &candidate)
			plan.ProviderBindingDigest = BindingDigest(plan, candidate)
			got := ApplyLocal(context.Background(), ApplyRequest{Plan: plan, Candidate: candidate, Fence: testFence{lease: &testLease{}}})
			if got.Outcome != OutcomeRefused || gitOutput(t, f.task, "rev-parse", "HEAD") != f.taskHead {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}

type edgeLease struct {
	onRecord func(ApplyRecord)
	revoked  bool
	closeErr error
}

func (l *edgeLease) Check(context.Context) error {
	if l.revoked {
		return errors.New("revoked")
	}
	return nil
}

func (l *edgeLease) Record(_ context.Context, r ApplyRecord) error {
	if l.onRecord != nil {
		l.onRecord(r)
	}
	return nil
}
func (l *edgeLease) Close() error { return l.closeErr }

type edgeFence struct{ lease ApplyLease }

func (f edgeFence) Acquire(context.Context, ApplyPlan) (ApplyLease, error) { return f.lease, nil }

func TestApplyLocalRetainsPartialEffects(t *testing.T) {
	for _, name := range []string{"revoked-after-checkout", "foreign-branch-cas", "close-failure", "bad-binding", "lost-checkpoint"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, false)
			prepared := PrepareClean(context.Background(), f.request()).Prepared
			if prepared == nil {
				t.Fatal("prepare failed")
			}
			plan := ApplyPlan{PlanID: "p", GenerationDigest: "g", AuthorityDigest: "a", CanonicalWorktree: f.canonical, TaskWorktree: f.task, CommonDir: f.common, TaskBranch: "task/demo", SourceRef: "main", ExpectedTaskHEAD: f.taskHead, ExpectedSourceOID: f.source, AllowedPaths: []string{"generated/source.txt", "shared.txt"}}
			plan.ProviderBindingDigest = BindingDigest(plan, prepared.Descriptor())
			lease := &edgeLease{}
			switch name {
			case "lost-checkpoint":
				lease.onRecord = func(r ApplyRecord) {
					if r.Step == "attached" {
						runGit(t, f.canonical, "update-ref", "--no-deref", "-d", r.CheckpointRef, f.taskHead)
					}
				}
			case "revoked-after-checkout":
				lease.onRecord = func(r ApplyRecord) {
					if r.Step == "checked-out" {
						lease.revoked = true
					}
				}
			case "foreign-branch-cas":
				lease.onRecord = func(r ApplyRecord) {
					if r.Step == "branch-intent" {
						runGit(t, f.canonical, "update-ref", "refs/heads/task/demo", f.source, f.taskHead)
					}
				}
			case "close-failure":
				lease.closeErr = errors.New("close failed")
			case "bad-binding":
				plan.ProviderBindingDigest = strings.Repeat("0", 64)
			}
			got := ApplyLocal(context.Background(), ApplyRequest{Plan: plan, Candidate: prepared.Descriptor(), Fence: edgeFence{lease: lease}})
			if name == "bad-binding" {
				if got.Outcome != OutcomeRefused || gitOutput(t, f.task, "rev-parse", "HEAD") != f.taskHead {
					t.Fatalf("result=%+v", got)
				}
				return
			}
			if got.Outcome != OutcomeUncertain || got.Applied || got.ActualHEAD != prepared.CandidateOID || got.OriginalHEAD != f.taskHead {
				t.Fatalf("result=%+v", got)
			}
			if name != "lost-checkpoint" && gitOutput(t, f.canonical, "rev-parse", got.CheckpointRef) != f.taskHead {
				t.Fatal("original checkpoint lost")
			}
			if name == "foreign-branch-cas" && gitOutput(t, f.canonical, "rev-parse", "refs/heads/task/demo") != f.source {
				t.Fatal("foreign branch was overwritten")
			}
		})
	}
}
