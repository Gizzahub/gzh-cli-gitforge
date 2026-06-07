// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package testutil provides helpers for creating temporary git repositories in tests.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TempGitRepo creates a temporary git repository.
// Returns the repository path. Automatically cleaned up.
func TempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Initialize git repo.
	cmd := exec.Command("git", "init") //nolint:noctx // test helper; no context.Context available in *testing.T API
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits.
	cmd = exec.Command("git", "config", "user.email", "test@test.com") //nolint:noctx // test helper; no context.Context available
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Logf("git config user.email warning (non-fatal in test setup): %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test") //nolint:noctx // test helper; no context.Context available
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Logf("git config user.name warning (non-fatal in test setup): %v", err)
	}

	return dir
}

// TempGitRepoWithCommit creates a temp git repo with an initial commit.
func TempGitRepoWithCommit(t *testing.T) string {
	t.Helper()
	dir := TempGitRepo(t)

	// Create a file and commit.
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test"), 0o600); err != nil { //nolint:gosec // 0o600 satisfies G306; test file needs no broader access
		t.Fatalf("failed to create README: %v", err)
	}

	cmd := exec.Command("git", "add", ".") //nolint:noctx // test helper; no context.Context available
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Logf("git add warning (non-fatal in test setup): %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit") //nolint:noctx // test helper; no context.Context available
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Logf("git commit warning (non-fatal in test setup): %v", err)
	}

	return dir
}

// TempGitRepoWithBranch creates a temp git repo with an initial commit and a branch.
func TempGitRepoWithBranch(t *testing.T, branchName string) string {
	t.Helper()
	dir := TempGitRepoWithCommit(t)

	cmd := exec.Command("git", "checkout", "-b", branchName) //nolint:noctx // test helper; no context.Context available
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch %s: %v", branchName, err)
	}

	return dir
}
