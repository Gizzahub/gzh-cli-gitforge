// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAddAdditionalRemotes(t *testing.T) {
	// Create a temporary git repository
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	// Initialize git repo
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Add origin remote (simulates cloned repo)
	cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/test/repo.git")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add origin remote: %v", err)
	}

	ctx := context.Background()
	logger := nopGitLogger{}

	t.Run("add new remote", func(t *testing.T) {
		remotes := map[string]string{
			"backup": "https://gitlab.com/test/repo.git",
		}

		msg, err := addAdditionalRemotes(ctx, repoPath, remotes, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if msg != "configured 1 remote(s)" {
			t.Errorf("got message %q, want %q", msg, "configured 1 remote(s)")
		}

		// Verify remote was added
		cmd := exec.Command("git", "remote", "get-url", "backup")
		cmd.Dir = repoPath
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("failed to get remote url: %v", err)
		}

		expectedURL := "https://gitlab.com/test/repo.git\n"
		if string(output) != expectedURL {
			t.Errorf("got URL %q, want %q", string(output), expectedURL)
		}
	})

	t.Run("add multiple remotes", func(t *testing.T) {
		remotes := map[string]string{
			"mirror1": "https://bitbucket.org/test/repo.git",
			"mirror2": "git@github.com:test/repo.git",
		}

		msg, err := addAdditionalRemotes(ctx, repoPath, remotes, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if msg != "configured 2 remote(s)" {
			t.Errorf("got message %q, want %q", msg, "configured 2 remote(s)")
		}

		// Verify both remotes were added
		for name, expectedURL := range remotes {
			cmd := exec.Command("git", "remote", "get-url", name)
			cmd.Dir = repoPath
			output, err := cmd.Output()
			if err != nil {
				t.Errorf("failed to get remote url for %s: %v", name, err)
				continue
			}

			if string(output) != expectedURL+"\n" {
				t.Errorf("remote %s: got URL %q, want %q", name, string(output), expectedURL)
			}
		}
	})

	t.Run("update existing remote", func(t *testing.T) {
		// Add a remote first
		cmd := exec.Command("git", "remote", "add", "test-remote", "https://old-url.com/repo.git")
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to add test remote: %v", err)
		}

		// Update it with new URL
		remotes := map[string]string{
			"test-remote": "https://new-url.com/repo.git",
		}

		msg, err := addAdditionalRemotes(ctx, repoPath, remotes, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if msg != "configured 1 remote(s)" {
			t.Errorf("got message %q, want %q", msg, "configured 1 remote(s)")
		}

		// Verify URL was updated
		cmd = exec.Command("git", "remote", "get-url", "test-remote")
		cmd.Dir = repoPath
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("failed to get remote url: %v", err)
		}

		expectedURL := "https://new-url.com/repo.git\n"
		if string(output) != expectedURL {
			t.Errorf("got URL %q, want %q", string(output), expectedURL)
		}
	})

	t.Run("empty remotes map", func(t *testing.T) {
		msg, err := addAdditionalRemotes(ctx, repoPath, nil, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if msg != "" {
			t.Errorf("got message %q, want empty string", msg)
		}
	})
}
