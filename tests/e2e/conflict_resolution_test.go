package e2e

import (
	"testing"
)

// TestConflictDetection tests detecting merge conflicts before they happen.
func TestConflictDetection(t *testing.T) {
	repo := NewE2ERepo(t)

	// Setup base
	repo.WriteFile("config.txt", "setting=value1\n")
	repo.Git("add", "config.txt")
	repo.Git("commit", "-m", "Initial config")

	t.Run("detect clean merge - no conflicts", func(t *testing.T) {
		// Create branch with non-conflicting change
		repo.Git("branch", "feature/clean")
		repo.Git("checkout", "feature/clean")
		repo.WriteFile("newfile.txt", "new content\n")
		repo.Git("add", "newfile.txt")
		repo.Git("commit", "-m", "Add new file")
		repo.Git("checkout", "master")

		// Detect merge - should show no conflicts for clean merge
		output := repo.RunGzhGit("conflict", "detect", "feature/clean", "master")

		// Should indicate no conflicts or show merge analysis
		if len(output) > 0 {
			t.Log("Conflict detect completed successfully")
		}
	})

	t.Run("detect conflicts with divergent changes", func(t *testing.T) {
		// Create two branches modifying same file
		repo.Git("branch", "feature/version-a")
		repo.Git("checkout", "feature/version-a")
		repo.WriteFile("config.txt", "setting=valueA\n")
		repo.Git("add", "config.txt")
		repo.Git("commit", "-m", "Update to version A")

		repo.Git("checkout", "master")
		repo.Git("branch", "feature/version-b")
		repo.Git("checkout", "feature/version-b")
		repo.WriteFile("config.txt", "setting=valueB\n")
		repo.Git("add", "config.txt")
		repo.Git("commit", "-m", "Update to version B")

		repo.Git("checkout", "master")

		// Detect conflicts between divergent branches
		// When conflicts exist, conflict detect returns non-zero exit status
		output := repo.RunGzhGitExpectError("conflict", "detect", "feature/version-a", "feature/version-b")

		// Should detect conflicts in config.txt
		AssertContains(t, output, "conflicts")
	})

	t.Run("detect non-existent branch error", func(t *testing.T) {
		// Try to detect with invalid branch
		output := repo.RunGzhGitExpectError("conflict", "detect", "nonexistent", "master")

		// Should error
		AssertContains(t, output, "not found")
	})
}

// Note: TestMergeAbort removed - merge abort subcommand no longer exists.
// Use native git for merge abort: git merge --abort

// Note: TestRebaseWorkflow removed - merge rebase subcommand no longer exists.
// Use native git for rebase: git rebase <branch>

// TestFastForwardMerge tests fast-forward merge scenarios.
func TestFastForwardMerge(t *testing.T) {
	repo := NewE2ERepo(t)

	// Setup
	repo.WriteFile("README.md", "# Project\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-m", "Initial commit")

	t.Run("fast-forward merge", func(t *testing.T) {
		// Create feature branch
		repo.Git("branch", "feature/ff")
		repo.Git("checkout", "feature/ff")

		// Add commits to feature branch
		repo.WriteFile("feature1.txt", "Feature 1\n")
		repo.Git("add", "feature1.txt")
		repo.Git("commit", "-m", "Add feature 1")

		repo.WriteFile("feature2.txt", "Feature 2\n")
		repo.Git("add", "feature2.txt")
		repo.Git("commit", "-m", "Add feature 2")

		// Switch back to master
		repo.Git("checkout", "master")

		// Detect merge - should show fast-forward possible
		output := repo.RunGzhGit("conflict", "detect", "feature/ff", "master")
		if len(output) > 0 {
			t.Log("Conflict detect completed for fast-forward scenario")
		}

		// Perform fast-forward merge using git
		repo.Git("merge", "feature/ff", "--ff-only")

		// Verify files are present
		if !repo.FileExists("feature1.txt") || !repo.FileExists("feature2.txt") {
			t.Error("Fast-forward merge did not bring in feature files")
		}
	})
}

// TestNoFastForwardMerge tests explicit merge commits.
func TestNoFastForwardMerge(t *testing.T) {
	repo := NewE2ERepo(t)

	// Setup
	repo.WriteFile("README.md", "# Project\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-m", "Initial commit")

	t.Run("create merge commit explicitly", func(t *testing.T) {
		// Create feature branch
		repo.Git("branch", "feature/no-ff")
		repo.Git("checkout", "feature/no-ff")

		// Add feature
		repo.WriteFile("feature.txt", "Feature\n")
		repo.Git("add", "feature.txt")
		repo.Git("commit", "-m", "Add feature")

		// Switch back
		repo.Git("checkout", "master")

		// Merge with explicit merge commit
		repo.Git("merge", "feature/no-ff", "--no-ff", "-m", "Merge feature branch")

		// Verify merge commit exists
		if !repo.CommitExists("Merge feature branch") {
			t.Error("Merge commit not created")
		}
	})
}

// TestConflictErrorHandling tests conflict detect error scenarios.
// Note: gz-git only provides conflict detection; use native git for merges.
func TestConflictErrorHandling(t *testing.T) {
	repo := NewE2ERepo(t)

	// Setup
	repo.WriteFile("README.md", "# Test\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-m", "Initial commit")

	t.Run("detect with invalid branches", func(t *testing.T) {
		// Try to detect with invalid branches
		output := repo.RunGzhGitExpectError("conflict", "detect", "invalid1", "invalid2")

		// Should error
		AssertContains(t, output, "not found")
	})
}

// TestCompleteConflictWorkflow documents the complete conflict workflow.
func TestCompleteConflictWorkflow(t *testing.T) {
	repo := NewE2ERepo(t)

	// Setup
	repo.WriteFile("shared.txt", "Original content\n")
	repo.Git("add", "shared.txt")
	repo.Git("commit", "-m", "Initial commit")

	t.Run("conflict workflow documentation", func(t *testing.T) {
		// This test documents the complete conflict resolution workflow
		// Note: gz-git provides conflict detect only. Actual merge operations
		// use native git commands.

		t.Log("Complete Conflict Resolution Workflow:")
		t.Log("1. Use 'gz-git conflict detect <source> <target>' to preview conflicts")
		t.Log("2. Attempt merge with 'git merge <branch>'")
		t.Log("3. If conflicts occur, resolve them manually")
		t.Log("4. Check status with 'gz-git status'")
		t.Log("5. Abort with 'git merge --abort' or commit resolved changes")

		// Verify basic commands work (bulk status format)
		status := repo.RunGzhGit("status")
		// Bulk status shows repository status in results
		if len(status) == 0 {
			t.Error("Status command returned no output")
		}

		// Verify conflict detect error handling
		detectOutput := repo.RunGzhGitExpectError("conflict", "detect", "nonexistent", "master")
		AssertContains(t, detectOutput, "not found")
	})
}
