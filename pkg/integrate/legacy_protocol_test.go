// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestLegacyRunProtocolReportsActualEffects(t *testing.T) {
	f := newCleanupRetryFX(t, "@true")
	report, err := Run(context.Background(), f.exec, RunOptions{
		CheckOptions: CheckOptions{RepoPath: f.fx.Worktree, Target: f.target},
	})
	if err != nil || report == nil || !report.Integrated || report.Receipt != nil {
		t.Fatalf("legacy run: report=%+v err=%v", report, err)
	}
	if f.remoteTargetTip(t) != f.taskSHA {
		t.Fatal("remote target does not contain the task")
	}
	f.assertResidualsGone(t, f.fx.Worktree)
	if _, err := os.Stat(f.fx.Worktree); !os.IsNotExist(err) {
		t.Fatalf("task worktree still exists or absence is unproven: %v", err)
	}
	if _, present, err := newGitRepo(f.exec, f.fx.Clone).revParse(context.Background(), "refs/heads/"+f.taskBranch); err != nil || present {
		t.Fatalf("local task ref absence unproven: present=%v err=%v", present, err)
	}
	data, err := json.Marshal(BuildRunProtocol(report, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	var doc RunProtocol
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.SourcePushed || !doc.WorktreeRemoved || !doc.LocalBranchRemoved || !doc.RemoteBranchRemoved {
		t.Fatalf("actual successful effects serialized as false: %s", data)
	}
	if doc.SchemaVersion != 1 || doc.ExecutionID != "" || doc.Generation != "" || doc.Owner != nil || doc.ReceiptPhase != "" {
		t.Fatalf("legacy result fabricated durable identity or changed schema: %s", data)
	}
}

func TestLegacyRunProtocolPartialEffects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		run  RunReport
		want [4]bool
	}{
		{name: "before push"},
		{name: "push only", run: RunReport{Integrated: true}, want: [4]bool{true, false, false, false}},
		{name: "worktree only", run: RunReport{Reclaim: ReclaimResult{RemovedWorktree: true}}, want: [4]bool{false, true, false, false}},
		{name: "local only", run: RunReport{Reclaim: ReclaimResult{RemovedLocalBranch: true}}, want: [4]bool{false, false, true, false}},
		{name: "remote only", run: RunReport{Reclaim: ReclaimResult{RemovedRemoteBranch: true}}, want: [4]bool{false, false, false, true}},
		{name: "prose is not proof", run: RunReport{Reclaim: ReclaimResult{Done: []string{"worktree(x)", "local-branch", "remote-branch"}}}},
		{name: "receipt overrides true report", run: RunReport{Integrated: true, Reclaim: ReclaimResult{RemovedWorktree: true, RemovedLocalBranch: true, RemovedRemoteBranch: true}, Receipt: &Receipt{}}},
		{name: "receipt overrides false report", run: RunReport{Receipt: &Receipt{SourcePushed: true, WorktreeRemoved: true, LocalBranchRemoved: true, RemoteBranchRemoved: true}}, want: [4]bool{true, true, true, true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := BuildRunProtocol(&tc.run, nil, 0)
			got := [4]bool{doc.SourcePushed, doc.WorktreeRemoved, doc.LocalBranchRemoved, doc.RemoteBranchRemoved}
			if got != tc.want {
				t.Fatalf("effects = %v, want %v", got, tc.want)
			}
		})
	}
}
