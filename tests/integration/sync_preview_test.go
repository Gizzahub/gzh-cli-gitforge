// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestWorkspaceSyncDryRunShowsPreview verifies that --dry-run shows the detailed
// preview without executing any changes.
func TestWorkspaceSyncDryRunShowsPreview(t *testing.T) {
	sourceRepo := NewTestRepo(t)
	sourceRepo.SetupWithCommits()

	workspaceDir := t.TempDir()
	configPath := filepath.Join(workspaceDir, ".gz-git.yaml")
	configContent := fmt.Sprintf(`repositories:
  - name: test-repo
    url: %s
`, sourceRepo.Path)
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	workspaceTestRepo := &TestRepo{Path: workspaceDir, T: t}
	output := workspaceTestRepo.RunGzhGitSuccess("workspace", "sync", "-c", configPath, "--dry-run")

	// Should show detailed preview (note: flat config always shows "update", executor handles clone)
	AssertContains(t, output, "Sync Preview")
	AssertContains(t, output, "Total: 1")
	AssertContains(t, output, "test-repo")

	// Dry-run should NOT actually clone
	AssertContains(t, output, "[dry-run] No changes made")

	// Target directory should NOT have been created
	clonedPath := filepath.Join(workspaceDir, "test-repo")
	if _, err := os.Stat(clonedPath); err == nil {
		t.Error("dry-run should not have cloned the repository")
	}
}

// TestWorkspaceSyncShowsRepositoryDetails verifies that sync shows the
// Repository Details section with repo names.
func TestWorkspaceSyncShowsRepositoryDetails(t *testing.T) {
	sourceRepo := NewTestRepo(t)
	sourceRepo.SetupWithCommits()

	workspaceDir := t.TempDir()
	configPath := filepath.Join(workspaceDir, ".gz-git.yaml")
	configContent := fmt.Sprintf(`repositories:
  - name: source-lib
    url: %s
`, sourceRepo.Path)
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	workspaceTestRepo := &TestRepo{Path: workspaceDir, T: t}
	output := workspaceTestRepo.RunGzhGitSuccess("workspace", "sync", "-c", configPath, "--dry-run")

	// Should show repository details section
	AssertContains(t, output, "Repository Details")
	AssertContains(t, output, "source-lib")
}

// TestWorkspaceSyncDirtyWorktreeShowsWarning verifies that a repository with
// uncommitted local changes triggers a conflict warning in the preview.
func TestWorkspaceSyncDirtyWorktreeShowsWarning(t *testing.T) {
	// Create a source repo (simulates remote)
	sourceRepo := NewTestRepo(t)
	sourceRepo.SetupWithCommits()

	workspaceDir := t.TempDir()
	localRepoPath := filepath.Join(workspaceDir, "my-repo")

	// Clone the source repo using standard git
	cloneCmd := exec.Command("git", "clone", sourceRepo.Path, localRepoPath)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone repo: %v\n%s", err, out)
	}

	// Make the local repo dirty (uncommitted changes)
	dirtyFile := filepath.Join(localRepoPath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("uncommitted change"), 0o644); err != nil {
		t.Fatalf("Failed to write dirty file: %v", err)
	}

	// Write config pointing to the source repo
	configPath := filepath.Join(workspaceDir, ".gz-git.yaml")
	configContent := fmt.Sprintf(`repositories:
  - name: my-repo
    url: %s
`, sourceRepo.Path)
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	workspaceTestRepo := &TestRepo{Path: workspaceDir, T: t}
	output := workspaceTestRepo.RunGzhGitSuccess("workspace", "sync", "-c", configPath, "--dry-run")

	// Preview should include my-repo
	AssertContains(t, output, "my-repo")

	// Since repo exists and is dirty, should show a warning
	AssertContains(t, output, "Warning")
}

// TestWorkspaceSyncMultiRepoSummary verifies correct total count
// when syncing multiple repositories.
func TestWorkspaceSyncMultiRepoSummary(t *testing.T) {
	sourceRepo1 := NewTestRepo(t)
	sourceRepo1.SetupWithCommits()

	sourceRepo2 := NewTestRepo(t)
	sourceRepo2.SetupWithCommits()

	workspaceDir := t.TempDir()
	configPath := filepath.Join(workspaceDir, ".gz-git.yaml")
	configContent := fmt.Sprintf(`repositories:
  - name: repo-one
    url: %s
  - name: repo-two
    url: %s
`, sourceRepo1.Path, sourceRepo2.Path)
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	workspaceTestRepo := &TestRepo{Path: workspaceDir, T: t}
	output := workspaceTestRepo.RunGzhGitSuccess("workspace", "sync", "-c", configPath, "--dry-run")

	// Should show total = 2
	AssertContains(t, output, "Total: 2")

	// Should show both repos in details
	AssertContains(t, output, "repo-one")
	AssertContains(t, output, "repo-two")

	// Note: flat config uses ActionUpdate internally (executor handles clone if missing)
	// so preview shows "will be updated" even for new repos
	if !containsAny(output, "will be updated", "will be cloned") {
		t.Errorf("expected 'will be updated' or 'will be cloned' in output, got:\n%s", output)
	}
}

// TestWorkspaceSyncYesFlagSkipsConfirmation verifies that --yes bypasses
// the interactive confirmation prompt.
func TestWorkspaceSyncYesFlagSkipsConfirmation(t *testing.T) {
	sourceRepo := NewTestRepo(t)
	sourceRepo.SetupWithCommits()

	workspaceDir := t.TempDir()
	configPath := filepath.Join(workspaceDir, ".gz-git.yaml")
	configContent := fmt.Sprintf(`repositories:
  - name: auto-repo
    url: %s
`, sourceRepo.Path)
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	workspaceTestRepo := &TestRepo{Path: workspaceDir, T: t}
	output := workspaceTestRepo.RunGzhGitSuccess("workspace", "sync", "-c", configPath, "--yes")

	// Should NOT show "Sync cancelled"
	AssertNotContains(t, output, "Sync cancelled")

	// Repo should actually be cloned
	clonedPath := filepath.Join(workspaceDir, "auto-repo")
	if _, err := os.Stat(clonedPath); err != nil {
		t.Errorf("Expected repo to be cloned at %s, but it wasn't: %v", clonedPath, err)
	}
}

// containsAny checks if output contains any of the given strings.
func containsAny(output string, candidates ...string) bool {
	for _, c := range candidates {
		if len(output) >= len(c) {
			for i := 0; i <= len(output)-len(c); i++ {
				if output[i:i+len(c)] == c {
					return true
				}
			}
		}
	}
	return false
}
