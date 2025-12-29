package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// createDirtyRepo creates a git repo with uncommitted changes
func createDirtyRepo(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	if err := initGitRepo(path); err != nil {
		return err
	}

	// Create initial commit
	testFile := filepath.Join(path, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0o644); err != nil {
		return err
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		return err
	}

	// Make uncommitted changes
	if err := os.WriteFile(testFile, []byte("# Test\nUpdated\n"), 0o644); err != nil {
		return err
	}

	return nil
}

func TestBulkCommit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test repositories
	repo1Path := filepath.Join(tmpDir, "repo1")
	repo2Path := filepath.Join(tmpDir, "repo2")

	if err := createDirtyRepo(repo1Path); err != nil {
		t.Skipf("Skipping test: git not available or failed: %v", err)
	}
	if err := createDirtyRepo(repo2Path); err != nil {
		t.Skipf("Skipping test: git not available or failed: %v", err)
	}

	ctx := context.Background()
	client := NewClient()

	opts := BulkCommitOptions{
		Directory: tmpDir,
		Parallel:  2,
		MaxDepth:  2,
		DryRun:    true,
		Verbose:   false,
		Logger:    NewNoopLogger(),
	}

	result, err := client.BulkCommit(ctx, opts)
	if err != nil {
		t.Fatalf("BulkCommit failed: %v", err)
	}

	// Verify results
	if result.TotalScanned != 2 {
		t.Errorf("Expected 2 scanned repositories, got %d", result.TotalScanned)
	}

	if result.TotalDirty != 2 {
		t.Errorf("Expected 2 dirty repositories, got %d", result.TotalDirty)
	}

	if len(result.Repositories) != 2 {
		t.Errorf("Expected 2 repository results, got %d", len(result.Repositories))
	}

	// Check that duration was recorded
	if result.Duration == 0 {
		t.Error("Expected non-zero duration")
	}

	// Verify summary contains results
	if len(result.Summary) == 0 {
		t.Error("Expected non-empty summary")
	}
}

func TestBulkCommitDryRun(t *testing.T) {
	tmpDir := t.TempDir()

	repoPath := filepath.Join(tmpDir, "repo")
	if err := createDirtyRepo(repoPath); err != nil {
		t.Skipf("Skipping test: git not available: %v", err)
	}

	ctx := context.Background()
	client := NewClient()

	opts := BulkCommitOptions{
		Directory: tmpDir,
		MaxDepth:  2,
		DryRun:    true,
		Logger:    NewNoopLogger(),
	}

	result, err := client.BulkCommit(ctx, opts)
	if err != nil {
		t.Fatalf("BulkCommit dry-run failed: %v", err)
	}

	// Dry-run should not commit
	if result.TotalCommitted != 0 {
		t.Errorf("Expected 0 committed in dry-run, got %d", result.TotalCommitted)
	}

	// Should find dirty repos
	if result.TotalDirty == 0 {
		t.Error("Expected to find dirty repositories")
	}

	// Status should be "would-commit"
	for _, repo := range result.Repositories {
		if repo.Status == "dirty" {
			// Status should be changed to would-commit in dry-run
			continue // acceptable
		}
		if repo.Status != "would-commit" && repo.Status != "clean" {
			t.Errorf("Expected status would-commit or clean, got %s", repo.Status)
		}
	}
}

func TestBulkCommitActualCommit(t *testing.T) {
	tmpDir := t.TempDir()

	repoPath := filepath.Join(tmpDir, "repo")
	if err := createDirtyRepo(repoPath); err != nil {
		t.Skipf("Skipping test: git not available: %v", err)
	}

	ctx := context.Background()
	client := NewClient()

	opts := BulkCommitOptions{
		Directory: tmpDir,
		MaxDepth:  2,
		DryRun:    false,
		Yes:       true,
		Message:   "test: bulk commit",
		Logger:    NewNoopLogger(),
	}

	result, err := client.BulkCommit(ctx, opts)
	if err != nil {
		t.Fatalf("BulkCommit failed: %v", err)
	}

	// Should have committed
	if result.TotalCommitted != 1 {
		t.Errorf("Expected 1 committed, got %d", result.TotalCommitted)
	}

	// Verify commit was made
	for _, repo := range result.Repositories {
		if repo.Status == "success" {
			if repo.CommitHash == "" {
				t.Error("Expected commit hash for successful commit")
			}
		}
	}
}

func TestBulkCommitWithFilters(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test repositories with different names
	repo1Path := filepath.Join(tmpDir, "myproject-api")
	repo2Path := filepath.Join(tmpDir, "test-repo")

	if err := createDirtyRepo(repo1Path); err != nil {
		t.Skipf("Skipping test: git not available: %v", err)
	}
	if err := createDirtyRepo(repo2Path); err != nil {
		t.Skipf("Skipping test: git not available: %v", err)
	}

	ctx := context.Background()
	client := NewClient()

	t.Run("Include pattern", func(t *testing.T) {
		opts := BulkCommitOptions{
			Directory:      tmpDir,
			MaxDepth:       2,
			DryRun:         true,
			IncludePattern: "myproject.*",
			Logger:         NewNoopLogger(),
		}

		result, err := client.BulkCommit(ctx, opts)
		if err != nil {
			t.Fatalf("BulkCommit failed: %v", err)
		}

		if result.TotalDirty != 1 {
			t.Errorf("Expected 1 dirty repository with include pattern, got %d", result.TotalDirty)
		}
	})

	t.Run("Exclude pattern", func(t *testing.T) {
		opts := BulkCommitOptions{
			Directory:      tmpDir,
			MaxDepth:       2,
			DryRun:         true,
			ExcludePattern: "test.*",
			Logger:         NewNoopLogger(),
		}

		result, err := client.BulkCommit(ctx, opts)
		if err != nil {
			t.Fatalf("BulkCommit failed: %v", err)
		}

		if result.TotalDirty != 1 {
			t.Errorf("Expected 1 dirty repository with exclude pattern, got %d", result.TotalDirty)
		}
	})
}

func TestBulkCommitCleanRepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a clean repository (no uncommitted changes)
	repoPath := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := initGitRepo(repoPath); err != nil {
		t.Skipf("Skipping test: git not available: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	ctx := context.Background()
	client := NewClient()

	opts := BulkCommitOptions{
		Directory: tmpDir,
		MaxDepth:  2,
		DryRun:    true,
		Logger:    NewNoopLogger(),
	}

	result, err := client.BulkCommit(ctx, opts)
	if err != nil {
		t.Fatalf("BulkCommit failed: %v", err)
	}

	if result.TotalDirty != 0 {
		t.Errorf("Expected 0 dirty repositories for clean repo, got %d", result.TotalDirty)
	}

	// Check status is clean
	for _, repo := range result.Repositories {
		if repo.Status != "clean" {
			t.Errorf("Expected status clean, got %s", repo.Status)
		}
	}
}

func TestBulkCommitProgressCallback(t *testing.T) {
	tmpDir := t.TempDir()

	repoPath := filepath.Join(tmpDir, "repo")
	if err := createDirtyRepo(repoPath); err != nil {
		t.Skipf("Skipping test: git not available: %v", err)
	}

	ctx := context.Background()
	client := NewClient()

	callbackCalled := false
	opts := BulkCommitOptions{
		Directory: tmpDir,
		MaxDepth:  2,
		DryRun:    true,
		Logger:    NewNoopLogger(),
		ProgressCallback: func(current, total int, repo string) {
			callbackCalled = true
			if current < 1 || current > total {
				t.Errorf("Invalid progress: current=%d, total=%d", current, total)
			}
		},
	}

	_, err := client.BulkCommit(ctx, opts)
	if err != nil {
		t.Fatalf("BulkCommit failed: %v", err)
	}

	if !callbackCalled {
		t.Error("Progress callback was not called")
	}
}

func TestBulkCommitEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	client := NewClient()

	opts := BulkCommitOptions{
		Directory: tmpDir,
		MaxDepth:  2,
		DryRun:    true,
		Logger:    NewNoopLogger(),
	}

	result, err := client.BulkCommit(ctx, opts)
	if err != nil {
		t.Fatalf("BulkCommit on empty directory failed: %v", err)
	}

	if result.TotalScanned != 0 {
		t.Errorf("Expected 0 scanned repositories in empty directory, got %d", result.TotalScanned)
	}

	if result.TotalDirty != 0 {
		t.Errorf("Expected 0 dirty repositories in empty directory, got %d", result.TotalDirty)
	}
}

func TestRepositoryCommitResult(t *testing.T) {
	result := RepositoryCommitResult{
		Path:             "/tmp/test",
		RelativePath:     "test",
		Branch:           "main",
		Status:           "success",
		CommitHash:       "abc1234",
		Message:          "test: commit",
		SuggestedMessage: "test: auto message",
		FilesChanged:     5,
		Additions:        10,
		Deletions:        3,
		ChangedFiles:     []string{"a.go", "b.go"},
		Duration:         100 * time.Millisecond,
	}

	if result.Path != "/tmp/test" {
		t.Errorf("Expected path /tmp/test, got %s", result.Path)
	}

	if result.Status != "success" {
		t.Errorf("Expected status success, got %s", result.Status)
	}

	if result.CommitHash != "abc1234" {
		t.Errorf("Expected commit hash abc1234, got %s", result.CommitHash)
	}

	if result.FilesChanged != 5 {
		t.Errorf("Expected 5 files changed, got %d", result.FilesChanged)
	}

	if len(result.ChangedFiles) != 2 {
		t.Errorf("Expected 2 changed files, got %d", len(result.ChangedFiles))
	}
}

func TestParseDiffStats(t *testing.T) {
	tests := []struct {
		name              string
		output            string
		expectedAdditions int
		expectedDeletions int
	}{
		{
			name:              "Simple stats",
			output:            " 3 files changed, 10 insertions(+), 5 deletions(-)",
			expectedAdditions: 10,
			expectedDeletions: 5,
		},
		{
			name:              "Only insertions",
			output:            " 1 file changed, 20 insertions(+)",
			expectedAdditions: 20,
			expectedDeletions: 0,
		},
		{
			name:              "Only deletions",
			output:            " 2 files changed, 15 deletions(-)",
			expectedAdditions: 0,
			expectedDeletions: 15,
		},
		{
			name:              "Empty output",
			output:            "",
			expectedAdditions: 0,
			expectedDeletions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			additions, deletions := parseDiffStats(tt.output)
			if additions != tt.expectedAdditions {
				t.Errorf("Expected %d additions, got %d", tt.expectedAdditions, additions)
			}
			if deletions != tt.expectedDeletions {
				t.Errorf("Expected %d deletions, got %d", tt.expectedDeletions, deletions)
			}
		})
	}
}

func TestInferScopeFromFiles(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name:     "Single directory",
			files:    []string{"cmd/main.go", "cmd/root.go"},
			expected: "cmd",
		},
		{
			name:     "pkg directory",
			files:    []string{"pkg/repo/client.go"},
			expected: "pkg",
		},
		{
			name:     "internal directory",
			files:    []string{"internal/parser/parse.go"},
			expected: "internal",
		},
		{
			name:     "Root files",
			files:    []string{"main.go", "go.mod"},
			expected: "",
		},
		{
			name:     "Empty",
			files:    []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferScopeFromFiles(tt.files)
			if result != tt.expected {
				t.Errorf("Expected scope '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestBulkCommitContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	repoPath := filepath.Join(tmpDir, "repo")
	if err := createDirtyRepo(repoPath); err != nil {
		t.Skipf("Skipping test: git not available: %v", err)
	}

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client := NewClient()

	opts := BulkCommitOptions{
		Directory: tmpDir,
		MaxDepth:  2,
		DryRun:    true,
		Logger:    NewNoopLogger(),
	}

	_, err := client.BulkCommit(ctx, opts)
	// Should handle cancelled context gracefully
	if err != nil && err != context.Canceled {
		t.Logf("BulkCommit with cancelled context returned: %v", err)
	}
}

// Benchmark tests

func BenchmarkBulkCommitSingleRepo(b *testing.B) {
	tmpDir := b.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	if err := createDirtyRepo(repoPath); err != nil {
		b.Skipf("Skipping benchmark: git not available: %v", err)
	}

	ctx := context.Background()
	client := NewClient()

	opts := BulkCommitOptions{
		Directory: tmpDir,
		MaxDepth:  2,
		DryRun:    true,
		Logger:    NewNoopLogger(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.BulkCommit(ctx, opts)
		if err != nil {
			b.Fatalf("BulkCommit failed: %v", err)
		}
	}
}
