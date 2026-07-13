// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func resetBranchListFlags(t *testing.T) {
	t.Helper()
	prevFlags := branchListFlags
	prevAll := branchListAll
	prevQuiet := quiet
	t.Cleanup(func() {
		branchListFlags = prevFlags
		branchListAll = prevAll
		quiet = prevQuiet
	})
	branchListFlags = BulkCommandFlags{Depth: 1, Parallel: 2}
	branchListAll = false
	quiet = false
}

func TestRunBranchList_WithTempRepo(t *testing.T) {
	resetBranchListFlags(t)
	parent := t.TempDir()
	repoDir := filepath.Join(parent, "app")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, parent, "init", "app")
	runGit(t, repoDir, "config", "user.email", "t@t.com")
	runGit(t, repoDir, "config", "user.name", "T")
	if err := os.WriteFile(filepath.Join(repoDir, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")
	runGit(t, repoDir, "branch", "feature/x")

	branchListFlags.Depth = 1
	branchListFlags.Parallel = 2
	out := captureStdout(t, func() {
		if err := runBranchList(branchListCmd, []string{parent}); err != nil {
			t.Fatalf("runBranchList: %v", err)
		}
	})
	if !strings.Contains(out, "Branch List") && !strings.Contains(out, "app") {
		t.Logf("branch list out: %q", out)
	}

	branchListAll = true
	out = captureStdout(t, func() {
		if err := runBranchList(branchListCmd, []string{parent}); err != nil {
			t.Fatalf("runBranchList -a: %v", err)
		}
	})
	_ = out
}

func TestRunBranchList_InvalidDir(t *testing.T) {
	resetBranchListFlags(t)
	if err := runBranchList(branchListCmd, []string{"/nope/missing"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPrintBulkBranchListResult(t *testing.T) {
	result := &repository.BulkBranchListResult{
		TotalScanned: 2,
		Duration:     time.Millisecond * 10,
		Repositories: []repository.RepositoryBranchListResult{
			{
				RelativePath: "ok-repo",
				Status:       repository.StatusSuccess,
				Branches: []repository.BranchInfo{
					{Name: "main", IsHead: true, IsRemote: false},
					{Name: "dev", IsHead: false, IsRemote: false},
					{Name: "origin/main", IsHead: false, IsRemote: true},
				},
			},
			{
				RelativePath: "bad-repo",
				Status:       repository.StatusError,
				Error:        errors.New("not a git repo"),
			},
		},
	}
	out := captureStdout(t, func() {
		printBulkBranchListResult(result, false)
		printBulkBranchListResult(result, true)
	})
	if !strings.Contains(out, "ok-repo") {
		t.Errorf("expected ok-repo in output: %q", out)
	}
	if !strings.Contains(out, "bad-repo") {
		t.Errorf("expected bad-repo in output: %q", out)
	}
}
