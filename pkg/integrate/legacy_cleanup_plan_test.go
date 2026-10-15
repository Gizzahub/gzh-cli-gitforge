// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func validLegacyCleanupPlanForTest(t *testing.T) *LegacyCleanupPlan {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	artifactA := filepath.Join(t.TempDir(), "done.json")
	artifactB := filepath.Join(t.TempDir(), "released.json")
	for _, path := range []string{artifactA, artifactB} {
		if err := os.WriteFile(path, []byte(path), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	hashA, err := hashArtifact(artifactA)
	if err != nil {
		t.Fatal(err)
	}
	hashB, err := hashArtifact(artifactB)
	if err != nil {
		t.Fatal(err)
	}
	p := &LegacyCleanupPlan{
		SchemaVersion: legacyCleanupPlanSchema, Capability: legacyCleanupCapability,
		Owner:          OwnerIdentity{Actor: "codex", Host: "fixture", Kind: "agent"},
		RepositoryRoot: root, PrimaryRoot: root, CommonDir: filepath.Join(root, ".git"), GitDir: filepath.Join(root, ".git"), TaskWorktree: filepath.Join(root, "task"),
		TaskBranch: "dev/actor/task", TaskOID: strings.Repeat("a", 40), Remote: "origin", Endpoint: "https://example.test/repo.git",
		TargetRef: "refs/heads/develop", TargetOID: strings.Repeat("b", 40), LocalRemoteOID: strings.Repeat("b", 40), TaskPatterns: []string{"dev/*"}, ProtectedBranches: []string{"main", "develop"}, RetiredDisposition: legacyRetiredDisposition,
		DoneArtifactPath: artifactA, DoneArtifactSHA256: hashA, ReleasedArtifactPath: artifactB, ReleasedArtifactSHA256: hashB,
	}
	p.WorktreeIdentity = &LegacyWorktreeIdentity{GitDir: p.GitDir, Worktree: LegacyFileIdentity{Device: 1, Inode: 2}, GitDirectory: LegacyFileIdentity{Device: 1, Inode: 3}, GitFile: LegacyFileIdentity{Device: 1, Inode: 4}}
	digest, err := legacyCleanupPlanDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	p.PlanDigest = digest
	return p
}

func TestLegacyCleanupPlanRejectsForeignMovedAndTargetFacts(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*LegacyCleanupPlan)
		want   string
	}{
		{"foreign owner", func(p *LegacyCleanupPlan) { p.Owner.Actor = "foreign/actor" }, "owner actor"},
		{"moved worktree", func(p *LegacyCleanupPlan) { p.TaskWorktree = filepath.Join(filepath.Dir(p.TaskWorktree), "moved") }, "digest mismatch"},
		{"target branch collision", func(p *LegacyCleanupPlan) { p.TargetRef = "refs/heads/dev/actor/task" }, "target identity"},
		{"target OID drift", func(p *LegacyCleanupPlan) { p.TargetOID = strings.Repeat("c", 40) }, "digest mismatch"},
		{"endpoint changed", func(p *LegacyCleanupPlan) { p.Endpoint = "ssh://foreign.example/repo" }, "digest mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validLegacyCleanupPlanForTest(t)
			tc.mutate(p)
			if tc.name == "moved worktree" || tc.name == "target OID drift" || tc.name == "endpoint changed" {
				path := filepath.Join(t.TempDir(), "plan.json")
				if err := WriteLegacyCleanupPlan(path, p); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("err=%v want %q", err, tc.want)
				}
				return
			}
			if err := p.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
		})
	}
}

func TestLegacyCleanupPlanBindsRuntimeGuardIntoDigest(t *testing.T) {
	p := validLegacyCleanupPlanForTest(t)
	p.RuntimeGuard = &LegacyRuntimeGuard{
		SchemaVersion:      legacyRuntimeGuardSchema,
		Capability:         legacyRuntimeGuardCapability,
		OperationID:        strings.Repeat("a", 32),
		RepositoryRoot:     p.RepositoryRoot,
		CommonDir:          p.CommonDir,
		TaskBranch:         p.TaskBranch,
		TaskWorktree:       p.TaskWorktree,
		TaskOID:            p.TaskOID,
		Owner:              p.Owner,
		VerifierExecutable: "/usr/bin/true",
		VerifierSHA256:     strings.Repeat("b", 64),
	}
	digest, err := legacyCleanupPlanDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	p.PlanDigest = digest
	if err := p.Validate(); err != nil {
		t.Fatalf("guard-bound plan invalid: %v", err)
	}
	p.RuntimeGuard.OperationID = strings.Repeat("c", 32)
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := WriteLegacyCleanupPlan(path, p); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("modified runtime guard err=%v, want digest mismatch", err)
	}
}

func TestLegacyRuntimeGuardRejectsMissingAttempt(t *testing.T) {
	p := validLegacyCleanupPlanForTest(t)
	p.RuntimeGuard = &LegacyRuntimeGuard{
		SchemaVersion: legacyRuntimeGuardSchema, Capability: legacyRuntimeGuardCapability,
		OperationID: strings.Repeat("a", 32), RepositoryRoot: p.RepositoryRoot, CommonDir: p.CommonDir,
		TaskBranch: p.TaskBranch, TaskWorktree: p.TaskWorktree, TaskOID: p.TaskOID, Owner: p.Owner,
		VerifierExecutable: "/usr/bin/true", VerifierSHA256: strings.Repeat("b", 64),
	}
	if _, err := newLegacyGuardChecker(p, ""); err == nil || !strings.Contains(err.Error(), "attempt") {
		t.Fatalf("missing runtime attempt err=%v", err)
	}
}

func writePlanFixture(t *testing.T, p *LegacyCleanupPlan) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := WriteLegacyCleanupPlan(path, p); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLegacyCleanupApplyRejectsCERuntimeAfterPlanValidation(t *testing.T) {
	t.Setenv("GZ_GIT_TASK_ACTOR", "codex")
	t.Setenv("GIT_WORK_HOST", "fixture")
	p := validLegacyCleanupPlanForTest(t)
	root := p.RepositoryRoot
	if err := os.MkdirAll(filepath.Join(root, ".ce"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ce", "task-runtime.yaml"), []byte("schema-version: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	planPath := writePlanFixture(t, p)
	err := ApplyLegacyCleanup(context.Background(), gitcmd.NewExecutor(), LegacyCleanupApplyOptions{PlanPath: planPath, ApproveDigest: p.PlanDigest})
	if err == nil {
		t.Fatal("unsupported or invalid native fixture was accepted")
	}
}
