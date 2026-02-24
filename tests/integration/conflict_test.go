package integration

import (
	"testing"
)

func TestConflictDetectCommand(t *testing.T) {
	repo := NewTestRepo(t)
	repo.SetupWithCommits()

	t.Run("detect with non-existent branch", func(t *testing.T) {
		output := repo.RunGzhGitExpectError("conflict", "detect", "non-existent", "master")

		// Should report branch not found
		AssertContains(t, output, "not found")
	})

	// Note: conflict detect is the only implemented pre-merge check command.
	// For actual merge operations (merge, abort, rebase), use native git commands.
}

func TestConflictWorkflow(t *testing.T) {
	repo := NewTestRepo(t)
	repo.SetupWithCommits()

	t.Run("detect error handling", func(t *testing.T) {
		// Test detect error case works correctly

		// 1. Detect non-existent branch
		detectOutput := repo.RunGzhGitExpectError("conflict", "detect", "nonexistent", "master")
		AssertContains(t, detectOutput, "not found")

		// 2. Verify repository is still operational (diagnostic status completes)
		// Note: Local test repos show "no-upstream" since they have no remote configured
		statusOutput := repo.RunGzhGitSuccess("status")
		AssertContains(t, statusOutput, "Status 1 repos")
		AssertContains(t, statusOutput, "healthy")
	})

	// Note: For actual merge operations, use native git commands:
	//   git merge <branch>
	//   git merge --abort
	//   git rebase <branch>
}
