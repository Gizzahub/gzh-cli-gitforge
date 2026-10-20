// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stashPullFixture builds a bare remote and a workspace clone that is one
// commit behind, where the remote commit rewrites README.md.
func stashPullFixture(t *testing.T) (workspace, repoPath string) {
	t.Helper()
	// BulkPull runs git in this process, so isolate it from the user's global
	// and system config: commit.gpgsign or a rebase hook there would turn the
	// fixture's commits and the pull's rebase into unrelated failures.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	tmpDir := t.TempDir()
	remotePath := filepath.Join(tmpDir, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", remotePath).CombinedOutput(); err != nil { //nolint:noctx // test setup
		t.Skipf("Skipping: git not available: %v %s", err, out)
	}

	seedPath := filepath.Join(tmpDir, "seed")
	workspace = filepath.Join(tmpDir, "workspace")
	repoPath = filepath.Join(workspace, "repo")

	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...) //nolint:noctx // test setup
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("clone", "-q", remotePath, seedPath)
	run("-C", seedPath, "config", "user.name", "Seed")
	run("-C", seedPath, "config", "user.email", "seed@example.com")
	write(filepath.Join(seedPath, "README.md"), "base\n")
	run("-C", seedPath, "add", ".")
	run("-C", seedPath, "commit", "-q", "-m", "base")
	run("-C", seedPath, "push", "-q", "origin", "HEAD")

	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	run("clone", "-q", remotePath, repoPath)
	run("-C", repoPath, "config", "user.name", "Test User")
	run("-C", repoPath, "config", "user.email", "test@example.com")

	write(filepath.Join(seedPath, "README.md"), "upstream\n")
	run("-C", seedPath, "commit", "-q", "-am", "upstream")
	run("-C", seedPath, "push", "-q")

	return workspace, repoPath
}

func stashCount(t *testing.T, repoPath string) int {
	t.Helper()
	out, err := exec.Command("git", "-C", repoPath, "stash", "list").Output() //nolint:noctx // test helper
	if err != nil {
		t.Fatalf("git stash list failed: %v", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

func TestBulkPullDryRunStashLeavesWorkingTree(t *testing.T) {
	workspace, repoPath := stashPullFixture(t)
	local := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(local, []byte("local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := NewClient().BulkPull(context.Background(), BulkPullOptions{
		Directory: workspace,
		MaxDepth:  1,
		Strategy:  "rebase",
		Stash:     true,
		DryRun:    true,
		Logger:    NewNoopLogger(),
	})
	if err != nil {
		t.Fatalf("BulkPull failed: %v", err)
	}

	if got := stashCount(t, repoPath); got != 0 {
		t.Errorf("dry run created %d stash entries, want 0", got)
	}
	content, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "local edit\n" {
		t.Errorf("dry run changed the working tree: README.md = %q", content)
	}
	repo := result.Repositories[0]
	if repo.Stashed {
		t.Error("dry run reported Stashed = true")
	}
	if !strings.Contains(repo.Message, "would stash") {
		t.Errorf("dry run message %q does not mention the stash it would make", repo.Message)
	}
}

func TestBulkPullStashPopConflictIsReported(t *testing.T) {
	workspace, repoPath := stashPullFixture(t)
	// The local edit touches the same line the upstream commit rewrote, so the
	// pull succeeds but restoring the stash conflicts.
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := NewClient().BulkPull(context.Background(), BulkPullOptions{
		Directory: workspace,
		MaxDepth:  1,
		Strategy:  "rebase",
		Stash:     true,
		Logger:    NewNoopLogger(),
	})
	if err != nil {
		t.Fatalf("BulkPull failed: %v", err)
	}

	repo := result.Repositories[0]
	if repo.Status != StatusConflict {
		t.Errorf("status = %q, want %q after a failed stash restore", repo.Status, StatusConflict)
	}
	if repo.Error == nil {
		t.Error("a failed stash restore must set Error")
	}
	if !strings.Contains(repo.Message, "stash entry is kept") {
		t.Errorf("message %q does not say the stash entry is kept", repo.Message)
	}
	if got := stashCount(t, repoPath); got != 1 {
		t.Errorf("stash entries = %d, want the kept entry (1)", got)
	}
}
