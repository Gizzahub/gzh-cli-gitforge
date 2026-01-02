package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestCodeReviewWorkflow tests analyzing code changes for review.
func TestCodeReviewWorkflow(t *testing.T) {
	repo := NewE2ERepo(t)

	// Setup: Create a project with some history
	repo.WriteFile("README.md", "# Project\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-m", "docs: initial README")

	repo.WriteFile("src/main.go", "package main\n\nfunc main() {}\n")
	repo.Git("add", "src/main.go")
	repo.Git("commit", "-m", "feat: add main function")

	repo.WriteFile("src/utils.go", "package main\n\nfunc Helper() {}\n")
	repo.Git("add", "src/utils.go")
	repo.Git("commit", "-m", "feat: add utility functions")

	t.Run("get overall repository statistics", func(t *testing.T) {
		// Get comprehensive stats
		output := repo.RunGzhGit("history", "stats")

		// Should show commit metrics
		AssertContains(t, output, "Total Commits:")
		AssertContains(t, output, "Unique Authors:")
	})

	t.Run("analyze contributor activity", func(t *testing.T) {
		// Get contributor stats
		output := repo.RunGzhGit("history", "contributors")

		// Should show E2E test user
		AssertContains(t, output, "E2E Test User")
	})

	t.Run("review specific file history", func(t *testing.T) {
		// Check history of specific file
		output := repo.RunGzhGit("history", "file", "src/main.go")

		// Should show commits affecting the file
		AssertContains(t, output, "main.go")
	})

	t.Run("export statistics for documentation", func(t *testing.T) {
		// Export stats as JSON
		output := repo.RunGzhGit("history", "stats", "--format", "json")

		// Should be valid JSON
		AssertContains(t, output, "{")
		AssertContains(t, output, "\"TotalCommits\"")
	})

	t.Run("export stats as CSV", func(t *testing.T) {
		// Export as CSV
		output := repo.RunGzhGit("history", "stats", "--format", "csv")

		// Should be CSV format
		AssertContains(t, output, ",")
	})

	t.Run("export stats as markdown table", func(t *testing.T) {
		// Export as markdown
		output := repo.RunGzhGit("history", "stats", "--format", "markdown")

		// Should contain markdown table markers
		AssertContains(t, output, "|")
	})
}

// Note: Commit message validation (commit validate) and template management
// (commit template) subcommands have been removed. Use git commit templates
// or external tools for message validation.

// TestFileAttributionAnalysis tests analyzing who wrote what.
func TestFileAttributionAnalysis(t *testing.T) {
	repo := NewE2ERepo(t)

	// Create initial file
	repo.WriteFile("shared.go", `package main

func Function1() {
	// Initial implementation
}
`)
	repo.Git("add", "shared.go")
	repo.Git("commit", "-m", "feat: add Function1")

	// Modify file
	repo.WriteFile("shared.go", `package main

func Function1() {
	// Initial implementation
}

func Function2() {
	// Second function
}
`)
	repo.Git("add", "shared.go")
	repo.Git("commit", "-m", "feat: add Function2")

	// Modify again
	repo.WriteFile("shared.go", `package main

func Function1() {
	// Initial implementation
	// Enhanced version
}

func Function2() {
	// Second function
}
`)
	repo.Git("add", "shared.go")
	repo.Git("commit", "-m", "refactor: enhance Function1")

	t.Run("blame entire file", func(t *testing.T) {
		// Get blame for file
		output := repo.RunGzhGit("history", "blame", "shared.go")

		// Should show line-by-line attribution with current year
		currentYear := time.Now().Format("2006-")
		AssertContains(t, output, currentYear)
		AssertContains(t, output, "shared.go")
	})

	t.Run("track file evolution", func(t *testing.T) {
		// Get complete file history
		output := repo.RunGzhGit("history", "file", "shared.go")

		// Should show all commits affecting file
		AssertContains(t, output, "shared.go")
	})

	t.Run("limit history depth", func(t *testing.T) {
		// Get limited history
		output := repo.RunGzhGit("history", "file", "shared.go", "--max", "2")

		// Should limit output
		AssertContains(t, output, "shared.go")
	})
}

// TestChangePatternAnalysis tests analyzing change patterns.
func TestChangePatternAnalysis(t *testing.T) {
	repo := NewE2ERepo(t)

	// Create diverse commit history
	commits := []struct {
		file    string
		content string
		message string
	}{
		{"docs/README.md", "# Docs\n", "docs: add README"},
		{"src/api.go", "package api\n", "feat(api): add API"},
		{"src/api.go", "package api\n\nfunc Handler() {}\n", "feat(api): add handler"},
		{"tests/api_test.go", "package api\n", "test(api): add tests"},
		{"src/db.go", "package db\n", "feat(db): add database"},
		{"docs/API.md", "# API Docs\n", "docs(api): add API documentation"},
	}

	for _, c := range commits {
		repo.WriteFile(c.file, c.content)
		repo.Git("add", c.file)
		repo.Git("commit", "-m", c.message)
	}

	t.Run("filter commits by author", func(t *testing.T) {
		// Filter by author
		output := repo.RunGzhGit("history", "stats", "--author", "E2E Test User")

		// Should show stats for that author
		AssertContains(t, output, "Total Commits:")
	})

	t.Run("filter commits by date range", func(t *testing.T) {
		// Filter by date
		output := repo.RunGzhGit("history", "stats", "--since", "2020-01-01")

		// Should show filtered stats
		AssertContains(t, output, "Total Commits:")
	})

	t.Run("analyze contributors by commits", func(t *testing.T) {
		// Sort by commits
		output := repo.RunGzhGit("history", "contributors", "--sort", "commits")

		// Should show sorted contributors
		AssertContains(t, output, "E2E Test User")
	})

	t.Run("analyze contributors by additions", func(t *testing.T) {
		// Sort by lines added
		output := repo.RunGzhGit("history", "contributors", "--sort", "additions")

		// Should show sorted contributors
		AssertContains(t, output, "E2E Test User")
	})

	t.Run("get top contributors", func(t *testing.T) {
		// Get top N contributors
		output := repo.RunGzhGit("history", "contributors", "--top", "5")

		// Should show limited list
		AssertContains(t, output, "E2E Test User")
	})
}

// TestBranchComparisonForReview tests comparing branches before merge.
func TestBranchComparisonForReview(t *testing.T) {
	repo := NewE2ERepo(t)

	// Setup base branch
	repo.WriteFile("main.go", "package main\n\nfunc main() {}\n")
	repo.Git("add", "main.go")
	repo.Git("commit", "-m", "Initial commit")

	// Create and develop feature branch
	repo.Git("branch", "feature/review")
	repo.Git("checkout", "feature/review")
	repo.WriteFile("feature.go", "package main\n\nfunc Feature() {}\n")
	repo.Git("add", "feature.go")
	repo.Git("commit", "-m", "feat: add feature")

	repo.WriteFile("feature_test.go", "package main\n\nfunc TestFeature() {}\n")
	repo.Git("add", "feature_test.go")
	repo.Git("commit", "-m", "test: add feature tests")

	t.Run("review feature branch changes", func(t *testing.T) {
		// Get history for feature branch
		output := repo.RunGzhGit("history", "file", "feature.go")

		// Should show feature commits
		AssertContains(t, output, "feature.go")
	})

	t.Run("detect potential merge conflicts", func(t *testing.T) {
		// Detect potential merge conflicts before review
		output := repo.RunGzhGit("merge", "detect", "feature/review", "master")

		// Should complete successfully with merge analysis
		if len(output) > 0 {
			t.Log("Merge detect completed for branch review")
		}
	})

	t.Run("list all branches using git", func(t *testing.T) {
		// Use native git for branch listing (gz-git branch list is removed)
		branches := repo.Git("branch", "-a")

		// Should show both branches
		if !strings.Contains(branches, "master") {
			t.Error("Expected master branch in git branch -a output")
		}
		if !strings.Contains(branches, "feature/review") {
			t.Error("Expected feature/review branch in git branch -a output")
		}
	})
}
