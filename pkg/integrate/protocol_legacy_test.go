// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import "testing"

func TestBuildRunProtocolLegacyMutationOutcomes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		report *RunReport
		want   [4]bool
	}{
		{"no report", nil, [4]bool{}},
		{"not integrated", &RunReport{}, [4]bool{}},
		{"unbound reclaim without push", &RunReport{Reclaim: ReclaimResult{RemovedWorktree: true, RemovedLocalBranch: true, RemovedRemoteBranch: true}}, [4]bool{}},
		{"push only", &RunReport{Integrated: true}, [4]bool{true, false, false, false}},
		{"partial reclaim", &RunReport{Integrated: true, Reclaim: ReclaimResult{RemovedWorktree: true, Failed: []string{"local branch refused"}}}, [4]bool{true, true, false, false}},
		{"all reclaimed", &RunReport{Integrated: true, Reclaim: ReclaimResult{RemovedWorktree: true, RemovedLocalBranch: true, RemovedRemoteBranch: true}}, [4]bool{true, true, true, true}},
		{"remote already deleted", &RunReport{Integrated: true, Reclaim: ReclaimResult{RemovedRemoteBranch: true}}, [4]bool{true, false, false, true}},
		{"local only", &RunReport{Integrated: true, Reclaim: ReclaimResult{RemovedLocalBranch: true}}, [4]bool{true, false, true, false}},
		{"exact action text is not proof", &RunReport{Integrated: true, Reclaim: ReclaimResult{Done: []string{"worktree(x)", "local-branch", "remote-branch"}}}, [4]bool{true, false, false, false}},
		{"unrelated actions", &RunReport{Integrated: true, Reclaim: ReclaimResult{Done: []string{"workspace-clone(child)", "parent-dir(/tmp)", "remote-branch-failed"}}}, [4]bool{true, false, false, false}},
		{"receipt authoritative", &RunReport{Integrated: true, Receipt: &Receipt{}, Reclaim: ReclaimResult{RemovedWorktree: true, RemovedLocalBranch: true, RemovedRemoteBranch: true}}, [4]bool{}},
		{"positive receipt authoritative", &RunReport{Receipt: &Receipt{SourcePushed: true, WorktreeRemoved: true, LocalBranchRemoved: true, RemoteBranchRemoved: true}}, [4]bool{true, true, true, true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := BuildRunProtocol(tt.report, nil, 0)
			got := [4]bool{p.SourcePushed, p.WorktreeRemoved, p.LocalBranchRemoved, p.RemoteBranchRemoved}
			if got != tt.want {
				t.Fatalf("mutation flags = %v, want %v", got, tt.want)
			}
			if tt.report == nil || tt.report.Receipt == nil {
				if p.ExecutionID != "" || p.Generation != "" || p.Owner != nil || p.ReceiptPhase != "" {
					t.Fatalf("legacy report fabricated receipt identity: %+v", p)
				}
			}
		})
	}
}
