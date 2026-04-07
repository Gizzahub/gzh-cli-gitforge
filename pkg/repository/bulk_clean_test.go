// Copyright (c) 2026 Archmagece
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func TestBulkCleanDryRun(t *testing.T) {
	// Create a repo with an untracked file
	repoDir := testutil.TempGitRepoWithCommit(t)
	untrackedFile := filepath.Join(repoDir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("temp"), 0o644); err != nil {
		t.Fatalf("failed to create untracked file: %v", err)
	}

	parentDir := filepath.Dir(repoDir)
	client := NewClient()

	result, err := client.BulkClean(context.Background(), BulkCleanOptions{
		Directory: parentDir,
		DryRun:    true,
		MaxDepth:  1,
	})
	if err != nil {
		t.Fatalf("BulkClean failed: %v", err)
	}

	if len(result.Repositories) == 0 {
		t.Fatal("expected at least one repository result")
	}

	found := false
	for _, repo := range result.Repositories {
		if repo.Path == repoDir {
			found = true
			if repo.Status != StatusWouldClean {
				t.Errorf("expected status %q, got %q", StatusWouldClean, repo.Status)
			}
			if repo.FilesCount != 1 {
				t.Errorf("expected 1 file, got %d", repo.FilesCount)
			}
			// File should still exist (dry-run)
			if _, err := os.Stat(untrackedFile); os.IsNotExist(err) {
				t.Error("file should not have been deleted in dry-run mode")
			}
		}
	}
	if !found {
		t.Errorf("repo %s not found in results", repoDir)
	}
}

func TestBulkCleanForce(t *testing.T) {
	repoDir := testutil.TempGitRepoWithCommit(t)
	untrackedFile := filepath.Join(repoDir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("temp"), 0o644); err != nil {
		t.Fatalf("failed to create untracked file: %v", err)
	}

	parentDir := filepath.Dir(repoDir)
	client := NewClient()

	result, err := client.BulkClean(context.Background(), BulkCleanOptions{
		Directory: parentDir,
		DryRun:    false,
		MaxDepth:  1,
	})
	if err != nil {
		t.Fatalf("BulkClean failed: %v", err)
	}

	for _, repo := range result.Repositories {
		if repo.Path == repoDir {
			if repo.Status != StatusCleaned {
				t.Errorf("expected status %q, got %q", StatusCleaned, repo.Status)
			}
			if repo.FilesCount != 1 {
				t.Errorf("expected 1 file removed, got %d", repo.FilesCount)
			}
			// File should be deleted
			if _, err := os.Stat(untrackedFile); !os.IsNotExist(err) {
				t.Error("file should have been deleted in force mode")
			}
		}
	}
}

func TestBulkCleanNothingToClean(t *testing.T) {
	repoDir := testutil.TempGitRepoWithCommit(t)
	parentDir := filepath.Dir(repoDir)
	client := NewClient()

	result, err := client.BulkClean(context.Background(), BulkCleanOptions{
		Directory: parentDir,
		DryRun:    true,
		MaxDepth:  1,
	})
	if err != nil {
		t.Fatalf("BulkClean failed: %v", err)
	}

	for _, repo := range result.Repositories {
		if repo.Path == repoDir {
			if repo.Status != StatusNothingToClean {
				t.Errorf("expected status %q, got %q", StatusNothingToClean, repo.Status)
			}
			if repo.FilesCount != 0 {
				t.Errorf("expected 0 files, got %d", repo.FilesCount)
			}
		}
	}
}

func TestBulkCleanWithDirectories(t *testing.T) {
	repoDir := testutil.TempGitRepoWithCommit(t)

	// Create an untracked directory with a file
	untrackedDir := filepath.Join(repoDir, "tmpdir")
	if err := os.Mkdir(untrackedDir, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(untrackedDir, "file.txt"), []byte("temp"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	parentDir := filepath.Dir(repoDir)
	client := NewClient()

	// Without -d flag, directory should not be removed
	result, err := client.BulkClean(context.Background(), BulkCleanOptions{
		Directory:         parentDir,
		DryRun:            true,
		RemoveDirectories: false,
		MaxDepth:          1,
	})
	if err != nil {
		t.Fatalf("BulkClean failed: %v", err)
	}

	for _, repo := range result.Repositories {
		if repo.Path == repoDir {
			if repo.Status != StatusNothingToClean {
				t.Errorf("expected %q without -d flag, got %q", StatusNothingToClean, repo.Status)
			}
		}
	}

	// With -d flag, directory should be listed
	result, err = client.BulkClean(context.Background(), BulkCleanOptions{
		Directory:         parentDir,
		DryRun:            true,
		RemoveDirectories: true,
		MaxDepth:          1,
	})
	if err != nil {
		t.Fatalf("BulkClean failed: %v", err)
	}

	for _, repo := range result.Repositories {
		if repo.Path == repoDir {
			if repo.Status != StatusWouldClean {
				t.Errorf("expected %q with -d flag, got %q", StatusWouldClean, repo.Status)
			}
			if repo.FilesCount == 0 {
				t.Error("expected files to clean with -d flag")
			}
		}
	}
}

func TestBulkCleanWithIgnored(t *testing.T) {
	repoDir := testutil.TempGitRepoWithCommit(t)

	// Create .gitignore and an ignored file
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatalf("failed to create .gitignore: %v", err)
	}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "add", ".gitignore")
	cmd.Dir = repoDir
	_ = cmd.Run()
	cmd = exec.CommandContext(ctx, "git", "commit", "-m", "add gitignore")
	cmd.Dir = repoDir
	_ = cmd.Run()

	if err := os.WriteFile(filepath.Join(repoDir, "debug.log"), []byte("log"), 0o644); err != nil {
		t.Fatalf("failed to create ignored file: %v", err)
	}

	parentDir := filepath.Dir(repoDir)
	client := NewClient()

	// Without -x, ignored file should not be listed
	result, err := client.BulkClean(context.Background(), BulkCleanOptions{
		Directory: parentDir,
		DryRun:    true,
		MaxDepth:  1,
	})
	if err != nil {
		t.Fatalf("BulkClean failed: %v", err)
	}

	for _, repo := range result.Repositories {
		if repo.Path == repoDir {
			if repo.Status != StatusNothingToClean {
				t.Errorf("expected %q without -x, got %q", StatusNothingToClean, repo.Status)
			}
		}
	}

	// With OnlyIgnored (-X), ignored file should be listed
	result, err = client.BulkClean(context.Background(), BulkCleanOptions{
		Directory:   parentDir,
		DryRun:      true,
		OnlyIgnored: true,
		MaxDepth:    1,
	})
	if err != nil {
		t.Fatalf("BulkClean failed: %v", err)
	}

	for _, repo := range result.Repositories {
		if repo.Path == repoDir {
			if repo.Status != StatusWouldClean {
				t.Errorf("expected %q with -X flag, got %q", StatusWouldClean, repo.Status)
			}
		}
	}
}

func TestBulkCleanIncludeExcludeFilter(t *testing.T) {
	// Create two repos under a parent directory
	parentDir := t.TempDir()

	repo1 := filepath.Join(parentDir, "project-a")
	repo2 := filepath.Join(parentDir, "project-b")

	ctx := context.Background()
	for _, dir := range []string{repo1, repo2} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		cmd := exec.CommandContext(ctx, "git", "init")
		cmd.Dir = dir
		_ = cmd.Run()
		cmd = exec.CommandContext(ctx, "git", "config", "user.email", "test@test.com")
		cmd.Dir = dir
		_ = cmd.Run()
		cmd = exec.CommandContext(ctx, "git", "config", "user.name", "Test")
		cmd.Dir = dir
		_ = cmd.Run()
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0o644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		cmd = exec.CommandContext(ctx, "git", "add", ".")
		cmd.Dir = dir
		_ = cmd.Run()
		cmd = exec.CommandContext(ctx, "git", "commit", "-m", "init")
		cmd.Dir = dir
		_ = cmd.Run()

		// Add untracked file
		if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("temp"), 0o644); err != nil {
			t.Fatalf("failed to create untracked file: %v", err)
		}
	}

	client := NewClient()

	// Include only project-a
	result, err := client.BulkClean(context.Background(), BulkCleanOptions{
		Directory:      parentDir,
		DryRun:         true,
		IncludePattern: "project-a",
		MaxDepth:       1,
	})
	if err != nil {
		t.Fatalf("BulkClean failed: %v", err)
	}

	if result.TotalProcessed != 1 {
		t.Errorf("expected 1 processed repo (include filter), got %d", result.TotalProcessed)
	}

	// Exclude project-a
	result, err = client.BulkClean(context.Background(), BulkCleanOptions{
		Directory:      parentDir,
		DryRun:         true,
		ExcludePattern: "project-a",
		MaxDepth:       1,
	})
	if err != nil {
		t.Fatalf("BulkClean failed: %v", err)
	}

	if result.TotalProcessed != 1 {
		t.Errorf("expected 1 processed repo (exclude filter), got %d", result.TotalProcessed)
	}
}

func TestBulkCleanEmptyDirectory(t *testing.T) {
	emptyDir := t.TempDir()
	client := NewClient()

	result, err := client.BulkClean(context.Background(), BulkCleanOptions{
		Directory: emptyDir,
		DryRun:    true,
		MaxDepth:  1,
	})
	if err != nil {
		t.Fatalf("BulkClean failed: %v", err)
	}

	if result.TotalProcessed != 0 {
		t.Errorf("expected 0 processed repos, got %d", result.TotalProcessed)
	}
}

func TestParseCleanOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			name:   "dry-run output",
			output: "Would remove untracked.txt\nWould remove tmp/\n",
			want:   2,
		},
		{
			name:   "force output",
			output: "Removing untracked.txt\nRemoving tmp/\n",
			want:   2,
		},
		{
			name:   "empty output",
			output: "",
			want:   0,
		},
		{
			name:   "whitespace only",
			output: "  \n  \n",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := parseCleanOutput(tt.output)
			if len(files) != tt.want {
				t.Errorf("parseCleanOutput() returned %d files, want %d", len(files), tt.want)
			}
		})
	}
}
