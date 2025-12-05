package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsRebaseInProgress(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(repoPath string) error
		expected bool
	}{
		{
			name: "No rebase in progress",
			setup: func(repoPath string) error {
				return nil
			},
			expected: false,
		},
		{
			name: "Rebase merge in progress",
			setup: func(repoPath string) error {
				gitDir := filepath.Join(repoPath, ".git")
				if err := os.MkdirAll(gitDir, 0o755); err != nil {
					return err
				}
				rebaseMerge := filepath.Join(gitDir, "rebase-merge")
				return os.MkdirAll(rebaseMerge, 0o755)
			},
			expected: true,
		},
		{
			name: "Rebase apply in progress",
			setup: func(repoPath string) error {
				gitDir := filepath.Join(repoPath, ".git")
				if err := os.MkdirAll(gitDir, 0o755); err != nil {
					return err
				}
				rebaseApply := filepath.Join(gitDir, "rebase-apply")
				return os.MkdirAll(rebaseApply, 0o755)
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir := t.TempDir()

			// Run setup
			if err := tt.setup(tempDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Test function
			result := IsRebaseInProgress(tempDir)
			if result != tt.expected {
				t.Errorf("IsRebaseInProgress() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsMergeInProgress(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(repoPath string) error
		expected bool
	}{
		{
			name: "No merge in progress",
			setup: func(repoPath string) error {
				return nil
			},
			expected: false,
		},
		{
			name: "Merge in progress",
			setup: func(repoPath string) error {
				gitDir := filepath.Join(repoPath, ".git")
				if err := os.MkdirAll(gitDir, 0o755); err != nil {
					return err
				}
				mergeHead := filepath.Join(gitDir, "MERGE_HEAD")
				return os.WriteFile(mergeHead, []byte("commit-hash"), 0o644)
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir := t.TempDir()

			// Run setup
			if err := tt.setup(tempDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Test function
			result := IsMergeInProgress(tempDir)
			if result != tt.expected {
				t.Errorf("IsMergeInProgress() = %v, want %v", result, tt.expected)
			}
		})
	}
}
