package integration

import (
	"testing"
)

// TestCommitBulkCommand tests the bulk commit functionality (now the default behavior)
func TestCommitBulkCommand(t *testing.T) {
	t.Run("dry run with no changes", func(t *testing.T) {
		repo := NewTestRepo(t)
		repo.SetupWithCommits()

		// With no changes, commit should report nothing to commit
		output := repo.RunGzhGitSuccess("commit", "--dry-run")

		// Should complete without error (preview mode)
		AssertContains(t, output, "Bulk Commit")
	})

	t.Run("dry run with staged changes", func(t *testing.T) {
		repo := NewTestRepo(t)
		repo.SetupWithCommits()

		// Create and stage changes
		repo.WriteFile("feature.go", "package main\n\nfunc NewFeature() {}\n")
		repo.GitAdd("feature.go")

		output := repo.RunGzhGitSuccess("commit", "--dry-run")

		// Should show the repository as dirty
		AssertContains(t, output, "Bulk Commit")
	})

	t.Run("with message flag", func(t *testing.T) {
		repo := NewTestRepo(t)
		repo.SetupWithCommits()

		repo.WriteFile("test.txt", "test content")
		repo.GitAdd("test.txt")

		output := repo.RunGzhGitSuccess("commit", "-m", "test: add test file", "--dry-run")

		// Should accept the message
		AssertContains(t, output, "Bulk Commit")
	})
}

// Note: The following subcommands have been removed in favor of bulk-only commit:
// - commit auto: Use 'commit' with --messages for per-repo messages
// - commit validate: Use git hooks or external tools for message validation
// - commit template: Use git commit templates via .gitmessage or hooks
//
// For bulk operations:
//   gz-git commit [directory]           # Bulk commit (default)
//   gz-git commit -m "message"          # Common message for all repos
//   gz-git commit --messages "repo:msg" # Per-repo messages
//   gz-git commit --messages-file file  # Messages from JSON file
