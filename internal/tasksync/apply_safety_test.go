// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boundApplyRequest(f fixture, candidate CandidateDescriptor) ApplyRequest {
	plan := ApplyPlan{PlanID: "p", GenerationDigest: "g", AuthorityDigest: "a", CanonicalWorktree: f.canonical, TaskWorktree: f.task, CommonDir: f.common, TaskBranch: "task/demo", SourceRef: "main", ExpectedTaskHEAD: f.taskHead, ExpectedSourceOID: f.source, AllowedPaths: []string{"generated/source.txt", "shared.txt"}}
	plan.ProviderBindingDigest = BindingDigest(plan, candidate)
	return ApplyRequest{Plan: plan, Candidate: candidate, Fence: edgeFence{lease: &edgeLease{}}}
}

func TestApplyRejectsUnsafeOriginalMetadata(t *testing.T) {
	for _, surface := range []string{"branch-parent", "reflog-parent", "admin-head"} {
		t.Run(surface, func(t *testing.T) {
			f := newFixture(t, false)
			prepared := PrepareClean(context.Background(), f.request()).Prepared
			if prepared == nil {
				t.Fatal("prepare failed")
			}
			var target string
			switch surface {
			case "branch-parent":
				target = filepath.Join(f.common, "refs", "heads", "task")
			case "reflog-parent":
				target = filepath.Join(f.common, "logs", "refs", "heads", "task")
			case "admin-head":
				target = filepath.Join(gitOutput(t, f.task, "rev-parse", "--absolute-git-dir"), "HEAD")
			}
			outside := filepath.Join(t.TempDir(), "preserved")
			if err := os.Rename(target, outside); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, target); err != nil {
				t.Fatal(err)
			}
			got := ApplyLocal(context.Background(), boundApplyRequest(f, prepared.Descriptor()))
			if got.Outcome != OutcomeRefused || got.Applied || got.CheckpointRef != "" {
				t.Fatalf("result=%+v", got)
			}
			if gitOutput(t, f.canonical, "rev-parse", "refs/heads/task/demo") != f.taskHead {
				t.Fatal("unsafe metadata caused a task mutation")
			}
		})
	}
}

func TestApplyChecksBothSidesOfProtectedRename(t *testing.T) {
	f := newFixture(t, false)
	write(t, filepath.Join(f.canonical, "shared.txt"), "base\n")
	runGit(t, f.canonical, "mv", "shared.txt", "renamed.txt")
	runGit(t, f.canonical, "commit", "-am", "rename source")
	f.source = gitOutput(t, f.canonical, "rev-parse", "HEAD")
	prepared := PrepareClean(context.Background(), f.request()).Prepared
	if prepared == nil {
		t.Fatal("prepare failed")
	}
	for _, allowed := range [][]string{{"generated/source.txt", "renamed.txt"}, {"generated/source.txt", "renamed.txt", "shared.txt"}} {
		req := boundApplyRequest(f, prepared.Descriptor())
		req.Plan.AllowedPaths, req.Plan.ProtectedPaths = allowed, []string{"shared.txt"}
		req.Plan.ProviderBindingDigest = BindingDigest(req.Plan, req.Candidate)
		got := ApplyLocal(context.Background(), req)
		if got.Outcome != OutcomeRefused || !strings.Contains(got.Reason, "path") || gitOutput(t, f.task, "rev-parse", "HEAD") != f.taskHead {
			t.Fatalf("result=%+v", got)
		}
	}
}

func TestApplyRejectsChangedSymlinkAndGitlink(t *testing.T) {
	for _, mode := range []string{"symlink", "gitlink"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t, false)
			if mode == "symlink" {
				if err := os.Symlink("shared.txt", filepath.Join(f.canonical, "link")); err != nil {
					t.Fatal(err)
				}
				runGit(t, f.canonical, "add", "link")
			} else {
				runGit(t, f.canonical, "update-index", "--add", "--cacheinfo", "160000,"+f.taskHead+",link")
			}
			runGit(t, f.canonical, "commit", "-m", "special mode")
			f.source = gitOutput(t, f.canonical, "rev-parse", "HEAD")
			prepared := PrepareClean(context.Background(), f.request()).Prepared
			if prepared == nil {
				t.Fatal("prepare failed")
			}
			req := boundApplyRequest(f, prepared.Descriptor())
			req.Plan.AllowedPaths = []string{"generated/source.txt", "link", "shared.txt"}
			req.Plan.ProviderBindingDigest = BindingDigest(req.Plan, req.Candidate)
			got := ApplyLocal(context.Background(), req)
			if got.Outcome != OutcomeRefused || !strings.Contains(got.Reason, "mode") {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}
