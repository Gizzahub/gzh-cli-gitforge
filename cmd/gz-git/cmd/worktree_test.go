package cmd

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/branch"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// ─── pure function tests ──────────────────────────────────────────────────────

func TestWorktreeDefaultPath(t *testing.T) {
	tests := []struct {
		name       string
		repoPath   string
		branchName string
		want       string
	}{
		{
			name:       "simple branch",
			repoPath:   "/home/user/myrepo",
			branchName: "main",
			want:       "/home/user/myrepo-main",
		},
		{
			name:       "feature branch with slash",
			repoPath:   "/home/user/myrepo",
			branchName: "feature/my-work",
			want:       "/home/user/myrepo-feature-my-work",
		},
		{
			name:       "nested branch path",
			repoPath:   "/home/user/myrepo",
			branchName: "release/v1.0/rc1",
			want:       "/home/user/myrepo-release-v1.0-rc1",
		},
		{
			name:       "repo in nested directory",
			repoPath:   "/workspace/projects/myrepo",
			branchName: "fix/issue-42",
			want:       "/workspace/projects/myrepo-fix-issue-42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := worktreeDefaultPath(tt.repoPath, tt.branchName)
			if got != tt.want {
				t.Errorf("worktreeDefaultPath(%q, %q) = %q, want %q", tt.repoPath, tt.branchName, got, tt.want)
			}
		})
	}
}

// ─── integration tests (require git binary) ───────────────────────────────────

// TestWorktreeList_SingleRepo verifies that list returns worktrees for the current repo.
func TestWorktreeList_SingleRepo(t *testing.T) {
	repoDir := testutil.TempGitRepoWithCommit(t)
	t.Chdir(repoDir)

	// Add a worktree using git directly so we have something to list.
	worktreeDir := t.TempDir()
	addCmd := exec.Command("git", "worktree", "add", worktreeDir, "HEAD") //nolint:noctx // test helper
	addCmd.Dir = repoDir
	if err := addCmd.Run(); err != nil {
		t.Skipf("git worktree add not available or failed: %v", err)
	}

	// Reset flag state before running.
	worktreeListFlags = BulkCommandFlags{}

	err := runSingleWorktreeList(context.Background())
	if err != nil {
		t.Errorf("runSingleWorktreeList() error = %v", err)
	}
}

// TestWorktreeList_NotARepo verifies an error is returned outside of a git repo.
func TestWorktreeList_NotARepo(t *testing.T) {
	nonRepo := t.TempDir()
	t.Chdir(nonRepo)

	worktreeListFlags = BulkCommandFlags{}

	err := runSingleWorktreeList(context.Background())
	if err == nil {
		t.Error("runSingleWorktreeList() expected error for non-repo directory, got nil")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("runSingleWorktreeList() error = %q, want containing 'not a git repository'", err.Error())
	}
}

// TestWorktreeAdd_NewBranch verifies that a worktree can be created with a new branch.
func TestWorktreeAdd_NewBranch(t *testing.T) {
	repoDir := testutil.TempGitRepoWithCommit(t)
	t.Chdir(repoDir)

	// Use a non-existent subdirectory — git worktree add creates it.
	// Resolve symlinks on the base dir so git path comparison works (macOS /var → /private/var).
	baseDir, evalErr := filepath.EvalSymlinks(t.TempDir())
	if evalErr != nil {
		t.Fatalf("failed to resolve temp dir symlinks: %v", evalErr)
	}
	worktreeTarget := filepath.Join(baseDir, "wt-new")

	// Reset flag state before running.
	worktreeAddCreate = true
	worktreeAddForce = false

	err := runWorktreeAdd(worktreeAddCmd, []string{"feature-test-branch", worktreeTarget})
	if err != nil {
		t.Errorf("runWorktreeAdd() error = %v", err)
		return
	}

	// Verify the worktree appears in the list.
	ctx := context.Background()
	client := repository.NewClient()
	repo, err := client.Open(ctx, repoDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	mgr := branch.NewWorktreeManager()
	worktrees, err := mgr.List(ctx, repo)
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}

	found := false
	for _, wt := range worktrees {
		absTarget, _ := filepath.Abs(worktreeTarget)
		absWt, _ := filepath.Abs(wt.Path)
		if absWt == absTarget {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("added worktree not found in list; got %d worktrees", len(worktrees))
	}
}

// TestWorktreeAdd_InvalidBranchName verifies that an invalid branch name is rejected before git runs.
func TestWorktreeAdd_InvalidBranchName(t *testing.T) {
	repoDir := testutil.TempGitRepoWithCommit(t)
	t.Chdir(repoDir)

	worktreeAddCreate = false
	worktreeAddForce = false

	// Branch names with spaces are invalid.
	err := runWorktreeAdd(worktreeAddCmd, []string{"invalid branch name"})
	if err == nil {
		t.Error("runWorktreeAdd() expected error for invalid branch name, got nil")
	}
	if !strings.Contains(err.Error(), "invalid branch name") {
		t.Errorf("runWorktreeAdd() error = %q, want containing 'invalid branch name'", err.Error())
	}
}

// TestWorktreeRemove verifies that a worktree can be added and then removed.
func TestWorktreeRemove(t *testing.T) {
	repoDir := testutil.TempGitRepoWithCommit(t)
	t.Chdir(repoDir)

	ctx := context.Background()
	client := repository.NewClient()

	repo, err := client.Open(ctx, repoDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	// Add a worktree using the library directly (non-existent path so git creates it).
	// Resolve symlinks on the base dir so git path comparison works (macOS /var → /private/var).
	removeBase, evalErr := filepath.EvalSymlinks(t.TempDir())
	if evalErr != nil {
		t.Fatalf("failed to resolve temp dir symlinks: %v", evalErr)
	}
	worktreeTarget := filepath.Join(removeBase, "wt-remove")
	mgr := branch.NewWorktreeManager()
	_, err = mgr.Add(ctx, repo, branch.AddOptions{
		Path:         worktreeTarget,
		Branch:       "rm-test-branch",
		CreateBranch: true,
	})
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Now remove via the CLI run function.
	worktreeRmForce = false

	err = runWorktreeRemove(worktreeRemoveCmd, []string{worktreeTarget})
	if err != nil {
		t.Errorf("runWorktreeRemove() error = %v", err)
		return
	}

	// Verify the worktree is gone.
	worktrees, err := mgr.List(ctx, repo)
	if err != nil {
		t.Fatalf("failed to list worktrees after remove: %v", err)
	}

	for _, wt := range worktrees {
		absTarget, _ := filepath.Abs(worktreeTarget)
		absWt, _ := filepath.Abs(wt.Path)
		if absWt == absTarget {
			t.Errorf("worktree still present after remove: %s", wt.Path)
		}
	}
}

// TestWorktreeRemove_NotARepo verifies an error when not in a git repository.
func TestWorktreeRemove_NotARepo(t *testing.T) {
	nonRepo := t.TempDir()
	t.Chdir(nonRepo)

	worktreeRmForce = false

	err := runWorktreeRemove(worktreeRemoveCmd, []string{"/some/path"})
	if err == nil {
		t.Error("runWorktreeRemove() expected error for non-repo directory, got nil")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("runWorktreeRemove() error = %q, want containing 'not a git repository'", err.Error())
	}
}

// TestPrintWorktreesJSON verifies the JSON marshalling of worktree results.
func TestPrintWorktreesJSON(t *testing.T) {
	results := []repoWorktreeResult{
		{
			Path: "/home/user/myrepo",
			Worktrees: []*branch.Worktree{
				{
					Path:   "/home/user/myrepo",
					Branch: "main",
					IsMain: true,
				},
				{
					Path:   "/home/user/myrepo-feature",
					Branch: "feature/x",
				},
			},
		},
	}

	// Should not error.
	err := printWorktreesJSON(results)
	if err != nil {
		t.Errorf("printWorktreesJSON() error = %v", err)
	}
}
