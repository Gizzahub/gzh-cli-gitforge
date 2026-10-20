// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestRunEvidenceDiscoveryFailurePreservesNoIntent(t *testing.T) {
	t.Parallel()
	// A real failing executable exercises the existing executor error path.
	// This unit fixture neither shadows Git nor substitutes an acceptance engine.
	failing, err := exec.LookPath("false")
	if err != nil {
		t.Fatal(err)
	}
	common := t.TempDir()
	g := newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(failing)), common)
	opts := RunOptions{Execution: &ExecutionContext{
		ExecutionID: "discovery-failure", Generation: "g1",
		Owner: OwnerIdentity{Actor: "codex", Host: "testhost", Kind: "agent"},
	}}
	check := &CheckReport{Plan: TargetPlan{Branch: "dev/codex/testhost/feat/task"}}
	receipt, err := beginRunEvidence(context.Background(), g.exec, g, common, opts, check,
		"0123456789abcdef0123456789abcdef01234567", "master")
	if err == nil || !strings.Contains(err.Error(), "discover task worktree before integration") {
		t.Fatalf("discovery failure hidden: receipt=%v err=%v", receipt, err)
	}
	if receipt != nil {
		t.Fatalf("failed discovery returned an intent receipt: %v", receipt)
	}
	if _, err := os.Stat(filepath.Join(common, "gz-integrate")); !os.IsNotExist(err) {
		t.Fatalf("failed discovery created evidence state: %v", err)
	}
}
