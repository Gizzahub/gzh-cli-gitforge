package integration

import (
	"testing"
)

// TestBranchCleanupCommand tests the branch cleanup functionality
// Note: Basic branch operations (list, create, delete) now use native git.
// Only cleanup functionality remains in gz-git.
func TestBranchCleanupCommand(t *testing.T) {
	repo := NewTestRepo(t)
	repo.SetupWithCommits()

	t.Run("cleanup with no merged branches", func(t *testing.T) {
		// Create and checkout a feature branch
		repo.GitBranch("feature/test")
		repo.GitCheckout("feature/test")

		// Make changes on feature branch
		repo.WriteFile("feature.txt", "feature content")
		repo.GitAdd("feature.txt")
		repo.GitCommit("Add feature")

		// Go back to master
		repo.GitCheckout("master")

		// Run cleanup - should not delete unmerged branch
		// Note: cleanup branch is now under 'gz-git cleanup branch'
		// Must specify at least one cleanup type: --merged, --stale, or --gone
		output := repo.RunGzhGitSuccess("cleanup", "branch", "--merged", "--dry-run")

		// Should complete without error (dry-run is default)
		AssertContains(t, output, "Branch Cleanup")
	})
}

// Note: The following branch subcommands have been removed:
// - branch list: Use 'git branch -a'
// - branch create: Use 'git checkout -b <name>'
// - branch delete: Use 'git branch -d <name>'
//
// For branch cleanup across repositories:
//   gz-git branch cleanup [directory]
//   gz-git cleanup branch [directory]  (alternative location)
