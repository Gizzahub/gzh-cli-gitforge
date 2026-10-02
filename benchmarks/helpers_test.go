package benchmarks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupGitRepo initializes a git repository.
func setupGitRepo(b *testing.B, path string) {
	b.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}

	runCmd(b, path, "git", "init")
	runCmd(b, path, "git", "config", "user.name", "Benchmark User")
	runCmd(b, path, "git", "config", "user.email", "benchmark@example.com")
}

// setupGitRepoWithCommit creates a git repo with initial commit.
func setupGitRepoWithCommit(b *testing.B, path string) {
	b.Helper()

	setupGitRepo(b, path)

	// Create and commit initial file
	writeFile(b, filepath.Join(path, "README.md"), "# Benchmark Repo\n")
	runCmd(b, path, "git", "add", "README.md")
	runCmd(b, path, "git", "commit", "-m", "Initial commit")
}

// setupLargeRepo creates a repository with many commits.
func setupLargeRepo(b *testing.B, path string, commits int) {
	b.Helper()

	setupGitRepo(b, path)

	for i := range commits {
		filename := filepath.Join(path, fmt.Sprintf("file%d.txt", i))
		writeFile(b, filename, fmt.Sprintf("Content %d\n", i))
		runCmd(b, path, "git", "add", filepath.Base(filename))
		runCmd(b, path, "git", "commit", "-m", fmt.Sprintf("Commit %d", i))
	}
}

// runCmd executes a command in the specified directory.
//
//nolint:unparam // generic command runner; name kept for clarity
func runCmd(b *testing.B, dir, name string, args ...string) {
	b.Helper()

	cmd := exec.CommandContext(b.Context(), name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		b.Fatalf("Command %s %v failed: %v\nOutput: %s", name, args, err, output)
	}
}

// writeFile writes content to a file.
func writeFile(b *testing.B, path, content string) {
	b.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatalf("Failed to write file %s: %v", path, err)
	}
}

// buildPrivateBinary builds gz-git from this repository's current source into
// a private TempDir via `make build BINARY=<absolute path>` and returns the
// binary's path. Benchmarks must measure this binary and nothing else:
// building into the repository root would leave an artifact in the checkout
// and replace whatever the operator keeps there, and a gz-git found on PATH
// could be any revision from any install.
func buildPrivateBinary(tb testing.TB) string {
	tb.Helper()

	root, err := filepath.Abs("..")
	if err != nil {
		tb.Fatalf("Failed to resolve repository root: %v", err)
	}
	binPath := filepath.Join(tb.TempDir(), "gz-git")

	cmd := exec.CommandContext(tb.Context(), "make", "build", "BINARY="+binPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		tb.Fatalf("Private build of %s failed: %v\nOutput: %s", binPath, err, output)
	}

	info, err := os.Stat(binPath)
	if err != nil {
		tb.Fatalf("Built binary %s is missing: %v", binPath, err)
	}
	if info.Mode()&0o111 == 0 {
		tb.Fatalf("Built binary %s is not executable: %v", binPath, info.Mode())
	}
	return binPath
}
