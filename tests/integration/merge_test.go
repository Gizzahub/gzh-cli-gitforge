package integration

import (
	"testing"
)

func TestMergeDetectCommand(t *testing.T) {
	repo := NewTestRepo(t)
	repo.SetupWithCommits()

	t.Run("detect with non-existent branch", func(t *testing.T) {
		output := repo.RunGzhGitExpectError("merge", "detect", "non-existent", "master")

		// Should report branch not found
		AssertContains(t, output, "not found")
	})

	// Note: merge detect is the only implemented merge subcommand.
	// For actual merge operations (merge, abort, rebase), use native git commands.
}

func TestMergeWorkflow(t *testing.T) {
	repo := NewTestRepo(t)
	repo.SetupWithCommits()

	t.Run("detect error handling", func(t *testing.T) {
		// Test detect error case works correctly

		// 1. Detect non-existent branch
		detectOutput := repo.RunGzhGitExpectError("merge", "detect", "nonexistent", "master")
		AssertContains(t, detectOutput, "not found")

		// 2. Verify repository is still operational (diagnostic status completes)
		// Note: Local test repos show "no-upstream" since they have no remote configured
		statusOutput := repo.RunGzhGitSuccess("status")
		AssertContains(t, statusOutput, "Repository Health Status")
		AssertContains(t, statusOutput, "Total repositories: 1")
	})

	// Note: For actual merge operations, use native git commands:
	//   git merge <branch>
	//   git merge --abort
	//   git rebase <branch>
}
