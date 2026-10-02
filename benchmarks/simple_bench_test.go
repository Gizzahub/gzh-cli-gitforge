package benchmarks

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// BenchmarkCLIStatus benchmarks the status command.
func BenchmarkCLIStatus(b *testing.B) {
	// Setup
	tempDir := b.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")
	setupGitRepoWithCommit(b, repoPath)

	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "status")
		cmd.Dir = repoPath
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// BenchmarkCLIInfo benchmarks the info command.
func BenchmarkCLIInfo(b *testing.B) {
	// Setup
	tempDir := b.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")
	setupGitRepoWithCommit(b, repoPath)

	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "info")
		cmd.Dir = repoPath
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// BenchmarkCLIBranchList benchmarks the branch list command.
func BenchmarkCLIBranchList(b *testing.B) {
	// Setup
	tempDir := b.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")
	setupGitRepoWithCommit(b, repoPath)

	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "branch", "list")
		cmd.Dir = repoPath
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// BenchmarkCLIHistoryStats benchmarks the history stats command.
func BenchmarkCLIHistoryStats(b *testing.B) {
	// Setup
	tempDir := b.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")
	setupLargeRepo(b, repoPath, 50)

	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "history", "stats")
		cmd.Dir = repoPath
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// BenchmarkCLIHistoryContributors benchmarks the history contributors command.
func BenchmarkCLIHistoryContributors(b *testing.B) {
	// Setup
	tempDir := b.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")
	setupLargeRepo(b, repoPath, 50)

	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "history", "contributors")
		cmd.Dir = repoPath
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// BenchmarkCLIHistoryFile benchmarks the history file command.
func BenchmarkCLIHistoryFile(b *testing.B) {
	// Setup
	tempDir := b.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")
	setupGitRepoWithCommit(b, repoPath)

	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "history", "file", "README.md")
		cmd.Dir = repoPath
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// BenchmarkCLIHistoryBlame benchmarks the history blame command.
func BenchmarkCLIHistoryBlame(b *testing.B) {
	// Setup
	tempDir := b.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")
	setupGitRepoWithCommit(b, repoPath)

	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "history", "blame", "README.md")
		cmd.Dir = repoPath
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// BenchmarkCLICommitValidate benchmarks the commit validate command.
func BenchmarkCLICommitValidate(b *testing.B) {
	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "commit", "validate", "feat(api): add user endpoint")
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// BenchmarkCLICommitTemplateList benchmarks the template list command.
func BenchmarkCLICommitTemplateList(b *testing.B) {
	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "commit", "template", "list")
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// BenchmarkCLIStatusLargeRepo benchmarks status on large repository.
func BenchmarkCLIStatusLargeRepo(b *testing.B) {
	// Setup
	tempDir := b.TempDir()
	repoPath := filepath.Join(tempDir, "large-repo")
	setupLargeRepo(b, repoPath, 100)

	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "status")
		cmd.Dir = repoPath
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// BenchmarkCLIHistoryStatsLargeRepo benchmarks stats on large repository.
func BenchmarkCLIHistoryStatsLargeRepo(b *testing.B) {
	// Setup
	tempDir := b.TempDir()
	repoPath := filepath.Join(tempDir, "large-repo")
	setupLargeRepo(b, repoPath, 200)

	binaryPath := findOrBuildBinary(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.CommandContext(b.Context(), binaryPath, "history", "stats")
		cmd.Dir = repoPath
		if _, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}

// findOrBuildBinary builds gz-git from the current source into a private
// temporary directory and returns its path. It never builds into the
// repository root and never uses a gz-git from PATH, so measured samples
// always describe this exact source tree.
func findOrBuildBinary(b *testing.B) string {
	b.Helper()
	return buildPrivateBinary(b)
}
