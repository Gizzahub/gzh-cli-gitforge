package repository

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPullError(t *testing.T) {
	t.Run("returns execErr when non-nil", func(t *testing.T) {
		execErr := errors.New("process start failed")
		got := buildPullError(execErr, 128, "some stderr", nil)
		if got != execErr {
			t.Errorf("expected execErr returned directly, got %v", got)
		}
	})

	t.Run("includes stderr when available", func(t *testing.T) {
		got := buildPullError(nil, 128, "error: cannot pull with rebase: You have unstaged changes.\n", nil)
		want := "pull exited with code 128: error: cannot pull with rebase: You have unstaged changes."
		if got.Error() != want {
			t.Errorf("expected %q, got %q", want, got.Error())
		}
	})

	t.Run("falls back to cmdErr when stderr empty", func(t *testing.T) {
		cmdErr := errors.New("exit status 1")
		got := buildPullError(nil, 1, "", cmdErr)
		if !strings.Contains(got.Error(), "exit status 1") {
			t.Errorf("expected cmdErr in message, got %q", got.Error())
		}
		if !strings.Contains(got.Error(), "code 1") {
			t.Errorf("expected exit code in message, got %q", got.Error())
		}
	})

	t.Run("returns exit code only when nothing else available", func(t *testing.T) {
		got := buildPullError(nil, 128, "", nil)
		want := "pull exited with code 128"
		if got.Error() != want {
			t.Errorf("expected %q, got %q", want, got.Error())
		}
	})

	t.Run("trims whitespace from stderr", func(t *testing.T) {
		got := buildPullError(nil, 1, "  \n  some error  \n  ", nil)
		if strings.HasPrefix(got.Error(), "pull exited with code 1:  ") {
			t.Errorf("stderr should be trimmed, got %q", got.Error())
		}
		if !strings.Contains(got.Error(), "some error") {
			t.Errorf("expected trimmed stderr, got %q", got.Error())
		}
	})
}

func TestBulkPullDirtyRepoWithMergeStrategy(t *testing.T) {
	// This test verifies the fix for the bug where user's global pull.rebase=true
	// caused bulk pull to fail on dirty repos with exit code 128.
	// The fix: pass --no-rebase explicitly when merge strategy is selected.

	tmpDir := t.TempDir()

	// Create a "remote" bare repository
	remotePath := filepath.Join(tmpDir, "remote.git")
	if err := os.MkdirAll(remotePath, 0o755); err != nil {
		t.Fatalf("Failed to create remote dir: %v", err)
	}
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = remotePath
	if err := cmd.Run(); err != nil {
		t.Skipf("Skipping: git not available: %v", err)
	}

	// Create the working repository
	repoPath := filepath.Join(tmpDir, "workspace", "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Init, configure, add remote, initial commit, push
	cmds := [][]string{
		{"init"},
		{"config", "user.name", "Test User"},
		{"config", "user.email", "test@example.com"},
		// Simulate user's global pull.rebase=true (the root cause of the original bug)
		{"config", "pull.rebase", "true"},
		{"remote", "add", "origin", remotePath},
	}
	for _, args := range cmds {
		c := exec.Command("git", args...)
		c.Dir = repoPath
		if err := c.Run(); err != nil {
			t.Fatalf("git %s failed: %v", args[0], err)
		}
	}

	// Create initial commit and push
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "Initial commit"},
		{"push", "-u", "origin", "master"},
	} {
		c := exec.Command("git", args...)
		c.Dir = repoPath
		if err := c.Run(); err != nil {
			t.Fatalf("git %s failed: %v", args[0], err)
		}
	}

	// Push a new commit to remote from a separate clone, so our repo is behind
	clonePath := filepath.Join(tmpDir, "clone-for-push")
	for _, args := range [][]string{
		{"clone", remotePath, clonePath},
		{"-C", clonePath, "config", "user.name", "Other User"},
		{"-C", clonePath, "config", "user.email", "other@example.com"},
	} {
		c := exec.Command("git", args...)
		if err := c.Run(); err != nil {
			t.Fatalf("git %s failed: %v", args[0], err)
		}
	}
	if err := os.WriteFile(filepath.Join(clonePath, "remote-change.txt"), []byte("from remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-C", clonePath, "add", "."},
		{"-C", clonePath, "commit", "-m", "Remote commit"},
		{"-C", clonePath, "push"},
	} {
		c := exec.Command("git", args...)
		if err := c.Run(); err != nil {
			t.Fatalf("git %s failed: %v", args[0], err)
		}
	}

	// Make the repo dirty with a TRACKED file modification (not just untracked)
	// Untracked files don't prevent rebase; only unstaged changes to tracked files do.
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# Modified locally\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	client := NewClient()

	t.Run("merge strategy succeeds on dirty repo despite pull.rebase=true", func(t *testing.T) {
		opts := BulkPullOptions{
			Directory: filepath.Join(tmpDir, "workspace"),
			MaxDepth:  1,
			Strategy:  "merge", // default strategy
			Logger:    NewNoopLogger(),
		}

		result, err := client.BulkPull(ctx, opts)
		if err != nil {
			t.Fatalf("BulkPull failed: %v", err)
		}

		if result.TotalScanned != 1 {
			t.Fatalf("Expected 1 repo scanned, got %d", result.TotalScanned)
		}

		repo := result.Repositories[0]
		if repo.Status == StatusError {
			t.Errorf("Pull should succeed on dirty repo with merge strategy, but got error: %v", repo.Error)
		}
		if repo.Status != StatusUpToDate && repo.Status != StatusPulled {
			t.Errorf("Expected up-to-date or pulled status, got %q", repo.Status)
		}
	})

	t.Run("rebase strategy fails on dirty repo as expected", func(t *testing.T) {
		opts := BulkPullOptions{
			Directory: filepath.Join(tmpDir, "workspace"),
			MaxDepth:  1,
			Strategy:  "rebase",
			Logger:    NewNoopLogger(),
		}

		result, err := client.BulkPull(ctx, opts)
		if err != nil {
			t.Fatalf("BulkPull failed: %v", err)
		}

		if result.TotalScanned != 1 {
			t.Fatalf("Expected 1 repo scanned, got %d", result.TotalScanned)
		}

		repo := result.Repositories[0]
		if repo.Status != StatusError {
			t.Errorf("Rebase strategy should fail on dirty repo, got status %q", repo.Status)
		}
		if repo.Error == nil {
			t.Error("Expected error to be set for failed rebase on dirty repo")
		}
	})
}
