package integration

import (
	"testing"
)

func TestStatusCommand(t *testing.T) {
	// NOTE: gz-git status is a BULK status command that scans directories for repositories.
	// It outputs bulk status format, not standard git status format.

	t.Run("clean repository", func(t *testing.T) {
		repo := NewTestRepo(t)
		repo.SetupWithCommits()

		output := repo.RunGzhGitSuccess("status")

		// Bulk status output format
		AssertContains(t, output, "Bulk Status Results")
		AssertContains(t, output, "clean")
	})

	t.Run("with uncommitted changes", func(t *testing.T) {
		repo := NewTestRepo(t)
		repo.SetupWithCommits()
		repo.WriteFile("modified.txt", "changed content")
		repo.GitAdd("modified.txt")

		output := repo.RunGzhGitSuccess("status")

		// Bulk status shows "dirty" with uncommitted count
		AssertContains(t, output, "Bulk Status Results")
		AssertContains(t, output, "dirty")
		AssertContains(t, output, "uncommitted")
	})

	t.Run("with untracked files", func(t *testing.T) {
		repo := NewTestRepo(t)
		repo.SetupWithCommits()
		repo.WriteFile("untracked.txt", "untracked content")

		output := repo.RunGzhGitSuccess("status")

		// Bulk status shows "dirty" with untracked count
		AssertContains(t, output, "Bulk Status Results")
		AssertContains(t, output, "dirty")
		AssertContains(t, output, "untracked")
	})
}

func TestInfoCommand(t *testing.T) {
	repo := NewTestRepo(t)
	repo.SetupWithCommits()

	t.Run("basic repository info", func(t *testing.T) {
		output := repo.RunGzhGitSuccess("info")

		AssertContains(t, output, "Repository:")
		AssertContains(t, output, "Branch:")
		AssertContains(t, output, "Status:")
	})

	t.Run("with multiple branches", func(t *testing.T) {
		repo.GitBranch("feature-1")
		repo.GitBranch("feature-2")

		output := repo.RunGzhGitSuccess("info")

		AssertContains(t, output, "Repository:")
	})

	t.Run("verbose output", func(t *testing.T) {
		output := repo.RunGzhGitSuccess("info", "--verbose")

		AssertContains(t, output, "Repository:")
		// Verbose mode should show more details
	})
}

func TestCloneCommand(t *testing.T) {
	t.Run("invalid URL shows error in results", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := &TestRepo{Path: tmpDir, T: t}

		// Bulk clone mode: command succeeds but shows errors in results
		// Use --url flag pattern (consistent with commit --messages)
		output := repo.RunGzhGitSuccess("clone", "--url", "invalid-url")

		// Should show error status in results
		AssertContains(t, output, "error")
		AssertContains(t, output, "Total failed")
	})

	t.Run("clone from local repository", func(t *testing.T) {
		// Create source repository
		source := NewTestRepo(t)
		source.SetupWithCommits()

		// Create target directory
		targetDir := t.TempDir()

		// Clone should work with local path (bulk mode)
		// Pattern: gz-git clone [directory] --url <url>
		target := &TestRepo{Path: targetDir, T: t}
		output := target.RunGzhGitSuccess("clone", "--url", source.Path)

		AssertContains(t, output, "Cloning")
		AssertContains(t, output, "Bulk Clone Results")
	})

	t.Run("clone multiple URLs", func(t *testing.T) {
		// Create two source repositories
		source1 := NewTestRepo(t)
		source1.SetupWithCommits()

		source2 := NewTestRepo(t)
		source2.SetupWithCommits()

		// Create target directory
		targetDir := t.TempDir()

		// Clone both repositories using multiple --url flags
		target := &TestRepo{Path: targetDir, T: t}
		output := target.RunGzhGitSuccess("clone", "--url", source1.Path, "--url", source2.Path)

		AssertContains(t, output, "Cloning 2 repositories")
		AssertContains(t, output, "Total cloned")
	})
}

func TestStatusNotARepository(t *testing.T) {
	// NOTE: gz-git status is a BULK status command that scans directories for repositories.
	// It does NOT fail when run in a non-git directory; it simply reports "no repositories found".
	tmpDir := t.TempDir()
	repo := &TestRepo{Path: tmpDir, T: t}

	output := repo.RunGzhGitSuccess("status")

	// Bulk status completes successfully but finds no repositories
	AssertContains(t, output, "Bulk Status Results")
	AssertContains(t, output, "Total scanned:   0")
}
