package repository

import (
	"os"
	"path/filepath"
)

// IsRebaseInProgress checks if a rebase operation is in progress for the given repository path.
// It checks for the existence of .git/rebase-merge or .git/rebase-apply directories.
func IsRebaseInProgress(repoPath string) bool {
	rebaseMerge := filepath.Join(repoPath, ".git", "rebase-merge")
	if _, err := os.Stat(rebaseMerge); err == nil {
		return true
	}
	rebaseApply := filepath.Join(repoPath, ".git", "rebase-apply")
	if _, err := os.Stat(rebaseApply); err == nil {
		return true
	}
	return false
}

// IsMergeInProgress checks if a merge operation is in progress for the given repository path.
// It checks for the existence of .git/MERGE_HEAD file.
func IsMergeInProgress(repoPath string) bool {
	mergeHead := filepath.Join(repoPath, ".git", "MERGE_HEAD")
	_, err := os.Stat(mergeHead)
	return err == nil
}
