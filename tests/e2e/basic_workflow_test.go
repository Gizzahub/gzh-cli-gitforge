package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestNewProjectSetup tests the workflow of setting up a new project.
func TestNewProjectSetup(t *testing.T) {
	repo := NewE2ERepo(t)

	t.Run("initialize and create first commit", func(t *testing.T) {
		// Create README file
		repo.WriteFile("README.md", "# Test Project\n\nThis is a test project.\n")

		// Stage the file
		repo.Git("add", "README.md")

		// Create commit using git
		repo.Git("commit", "-m", "docs(root): add project README")

		// Verify commit exists
		if !repo.CommitExists("add project README") {
			t.Error("Commit not found in git log")
		}
	})

	t.Run("check repository status", func(t *testing.T) {
		// Check status using gz-git
		output := repo.RunGzhGit("status")

		// Bulk status shows "no-remote" for repos without remote configured
		// or shows the repository in the results
		if !strings.Contains(output, "Bulk Status Results") && !strings.Contains(output, "repositories") {
			t.Errorf("Expected status output to contain bulk results, got: %s", output)
		}
	})

	t.Run("get repository info", func(t *testing.T) {
		// Get repository info
		output := repo.RunGzhGit("info")

		// Should show repository details (may show master or detached HEAD)
		if !strings.Contains(output, "master") && !strings.Contains(output, "Branch:") {
			t.Errorf("Expected info to show branch info, got: %s", output)
		}
	})
}

// TestBasicCommitWorkflow tests basic commit operations (bulk mode).
func TestBasicCommitWorkflow(t *testing.T) {
	repo := NewE2ERepo(t)

	// Create initial commit
	repo.WriteFile("README.md", "# Test\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-m", "Initial commit")

	t.Run("bulk commit dry run", func(t *testing.T) {
		// Create changes
		repo.WriteFile("feature.go", "package main\n\nfunc NewFeature() {}\n")
		repo.Git("add", "feature.go")

		// Test bulk commit dry run (now the default)
		output := repo.RunGzhGit("commit", "--dry-run")

		// Should show bulk commit results
		AssertContains(t, output, "Bulk Commit")
	})

	t.Run("commit with message", func(t *testing.T) {
		// Clean up and create new change
		repo.Git("reset", "--hard", "HEAD")
		repo.WriteFile("test.txt", "test content\n")
		repo.Git("add", "test.txt")

		// Test commit with message flag
		output := repo.RunGzhGit("commit", "-m", "test: add test file", "--dry-run")

		// Should accept the message
		AssertContains(t, output, "Bulk Commit")
	})
}

// Note: The following commit subcommands have been removed:
// - commit auto: Use 'commit' with --messages for per-repo messages
// - commit validate: Use git hooks or external tools for message validation
// - commit template: Use git commit templates via .gitmessage or hooks

// TestBasicBranchWorkflow tests basic branch operations.
func TestBasicBranchWorkflow(t *testing.T) {
	repo := NewE2ERepo(t)

	// Create initial commit
	repo.WriteFile("README.md", "# Test\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-m", "Initial commit")

	t.Run("use git for branch operations", func(t *testing.T) {
		// Use native git for basic branch operations
		repo.Git("branch", "feature/test")

		// Verify branch exists
		branches := repo.Git("branch", "-a")
		if !strings.Contains(branches, "feature/test") {
			t.Error("Branch feature/test should exist")
		}
	})

	t.Run("switch branches and make changes", func(t *testing.T) {
		// Switch to feature branch
		repo.Git("checkout", "feature/test")

		// Verify current branch
		if repo.GetCurrentBranch() != "feature/test" {
			t.Errorf("Expected current branch to be 'feature/test', got %s",
				repo.GetCurrentBranch())
		}

		// Make changes
		repo.WriteFile("feature.txt", "New feature\n")
		repo.Git("add", "feature.txt")
		repo.Git("commit", "-m", "feat: add feature file")

		// Verify commit exists
		if !repo.CommitExists("add feature file") {
			t.Error("Feature commit not found")
		}
	})

	t.Run("cleanup merged branches", func(t *testing.T) {
		// Go back to master and merge
		repo.Git("checkout", "master")
		repo.Git("merge", "feature/test")

		// Run cleanup (dry run)
		output := repo.RunGzhGit("cleanup", "branch", "--merged", "--dry-run")

		// Should complete successfully
		AssertContains(t, output, "Branch Cleanup")
	})
}

// Note: Basic branch operations (list, create, delete) now use native git.
// Only cleanup functionality remains in gz-git.

// TestBasicHistoryWorkflow tests basic history operations.
func TestBasicHistoryWorkflow(t *testing.T) {
	repo := NewE2ERepo(t)

	// Create some commit history
	repo.WriteFile("README.md", "# Test\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-m", "docs: add README")

	repo.WriteFile("src/main.go", "package main\n")
	repo.Git("add", "src/main.go")
	repo.Git("commit", "-m", "feat: add main")

	repo.WriteFile("src/util.go", "package main\n")
	repo.Git("add", "src/util.go")
	repo.Git("commit", "-m", "feat: add util")

	t.Run("get repository statistics", func(t *testing.T) {
		// Get stats
		output := repo.RunGzhGit("history", "stats")

		// Should show commit count
		AssertContains(t, output, "Total Commits:")
		AssertContains(t, output, "Unique Authors:")
	})

	t.Run("list contributors", func(t *testing.T) {
		// Get contributors
		output := repo.RunGzhGit("history", "contributors")

		// Should show test user
		AssertContains(t, output, "E2E Test User")
	})

	t.Run("get file history", func(t *testing.T) {
		// Get file history
		output := repo.RunGzhGit("history", "file", "README.md")

		// Should show commits affecting README
		AssertContains(t, output, "README")
	})

	t.Run("blame file", func(t *testing.T) {
		// Blame file
		output := repo.RunGzhGit("history", "blame", "README.md")

		// Should show line-by-line attribution with current year
		currentYear := time.Now().Format("2006-")
		AssertContains(t, output, currentYear)
	})

	t.Run("export stats as JSON", func(t *testing.T) {
		// Get stats in JSON format
		output := repo.RunGzhGit("history", "stats", "--format", "json")

		// Should be valid JSON
		AssertContains(t, output, "{")
		AssertContains(t, output, "\"TotalCommits\"")
	})

	t.Run("filter by date range", func(t *testing.T) {
		// Get stats with date filter
		output := repo.RunGzhGit("history", "stats", "--since", "2020-01-01")

		// Should show filtered stats
		AssertContains(t, output, "Total Commits:")
	})
}

// TestRepositoryCleanupWorkflow tests cleanup operations.
func TestRepositoryCleanupWorkflow(t *testing.T) {
	repo := NewE2ERepo(t)

	// Create initial commit
	repo.WriteFile("README.md", "# Test\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-m", "Initial commit")

	t.Run("check status of clean repo", func(t *testing.T) {
		// Check status
		output := repo.RunGzhGit("status")

		// Bulk status shows results (may show "no-remote" for local-only repos)
		if !strings.Contains(output, "Bulk Status Results") && !strings.Contains(output, "repositories") {
			t.Errorf("Expected status output to contain bulk results, got: %s", output)
		}
	})

	t.Run("status with untracked files", func(t *testing.T) {
		// Create untracked file
		repo.WriteFile("temp.txt", "temporary\n")

		// Check status
		output := repo.RunGzhGit("status")

		// Diagnostic status shows healthy (untracked files alone don't trigger warning)
		// But repository health check still completes successfully
		if !strings.Contains(output, "healthy") {
			t.Errorf("Expected status to show healthy, got: %s", output)
		}
	})

	t.Run("status with modified files", func(t *testing.T) {
		// Modify existing file
		repo.WriteFile("README.md", "# Test\n\nModified\n")

		// Check status
		output := repo.RunGzhGit("status")

		// Diagnostic status shows warning (modified files need attention)
		// Repository health check completes successfully with warning
		if !strings.Contains(output, "warning") {
			t.Errorf("Expected status to show warning, got: %s", output)
		}
		if !strings.Contains(output, "modified") {
			t.Errorf("Expected status to show modified files, got: %s", output)
		}
	})
}
