package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

func TestCheckRebaseInProgress(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	t.Run("no rebase in progress", func(t *testing.T) {
		if repository.IsRebaseInProgress(tmpDir) {
			t.Error("Expected no rebase in progress")
		}
	})

	t.Run("rebase-merge exists", func(t *testing.T) {
		rebaseMergeDir := filepath.Join(gitDir, "rebase-merge")
		if err := os.MkdirAll(rebaseMergeDir, 0o755); err != nil {
			t.Fatalf("Failed to create rebase-merge dir: %v", err)
		}

		if !repository.IsRebaseInProgress(tmpDir) {
			t.Error("Expected rebase in progress")
		}

		os.RemoveAll(rebaseMergeDir)
	})

	t.Run("rebase-apply exists", func(t *testing.T) {
		rebaseApplyDir := filepath.Join(gitDir, "rebase-apply")
		if err := os.MkdirAll(rebaseApplyDir, 0o755); err != nil {
			t.Fatalf("Failed to create rebase-apply dir: %v", err)
		}

		if !repository.IsRebaseInProgress(tmpDir) {
			t.Error("Expected rebase in progress")
		}

		os.RemoveAll(rebaseApplyDir)
	})
}

func TestCheckMergeInProgress(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	t.Run("no merge in progress", func(t *testing.T) {
		if repository.IsMergeInProgress(tmpDir) {
			t.Error("Expected no merge in progress")
		}
	})

	t.Run("MERGE_HEAD exists", func(t *testing.T) {
		mergeHeadFile := filepath.Join(gitDir, "MERGE_HEAD")
		if err := os.WriteFile(mergeHeadFile, []byte("fake-commit-hash"), 0o644); err != nil {
			t.Fatalf("Failed to create MERGE_HEAD: %v", err)
		}

		if !repository.IsMergeInProgress(tmpDir) {
			t.Error("Expected merge in progress")
		}

		os.Remove(mergeHeadFile)
	})
}
