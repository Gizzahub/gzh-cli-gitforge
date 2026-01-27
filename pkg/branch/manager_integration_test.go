package branch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// initTestGitRepo creates a temporary git repository with initial commit for testing.
// Returns the resolved (real) path to avoid symlink issues on macOS.
func initTestGitRepo(t *testing.T, dir string) string {
	t.Helper()

	// Resolve symlinks (macOS /var -> /private/var)
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		realDir = dir
	}

	// Create a file first
	testFile := filepath.Join(realDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repository\n"), 0o644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "add", "."},
		{"git", "commit", "-m", "Initial commit"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = realDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Failed to run %v: %v\nOutput: %s", args, err, output)
		}
	}

	return realDir
}

// TestIntegration_BranchManager_Create tests branch creation with a real git repository.
func TestIntegration_BranchManager_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	// Test creating a new branch
	err := mgr.Create(ctx, repo, CreateOptions{
		Name:     "feature/test-branch",
		Validate: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify branch exists
	exists, err := mgr.Exists(ctx, repo, "feature/test-branch")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Branch should exist after creation")
	}
}

// TestIntegration_BranchManager_Create_WithCheckout tests branch creation with checkout.
func TestIntegration_BranchManager_Create_WithCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	// Create and checkout branch
	err := mgr.Create(ctx, repo, CreateOptions{
		Name:     "feature/checkout-test",
		Checkout: true,
		Validate: true,
	})
	if err != nil {
		t.Fatalf("Create() with checkout error = %v", err)
	}

	// Verify it's the current branch
	current, err := mgr.Current(ctx, repo)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.Name != "feature/checkout-test" {
		t.Errorf("Current branch = %q, want %q", current.Name, "feature/checkout-test")
	}
}

// TestIntegration_BranchManager_List tests listing branches.
func TestIntegration_BranchManager_List(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	// Create multiple branches
	branches := []string{"feature/a", "feature/b", "fix/bug-1"}
	for _, name := range branches {
		err := mgr.Create(ctx, repo, CreateOptions{Name: name})
		if err != nil {
			t.Fatalf("Create(%s) error = %v", name, err)
		}
	}

	// List all branches
	list, err := mgr.List(ctx, repo, ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should have main/master + 3 created branches
	if len(list) < 4 {
		t.Errorf("List() returned %d branches, want at least 4", len(list))
	}

	// Verify our branches exist in the list
	branchNames := make(map[string]bool)
	for _, b := range list {
		branchNames[b.Name] = true
	}

	for _, name := range branches {
		if !branchNames[name] {
			t.Errorf("Branch %q not found in list", name)
		}
	}
}

// TestIntegration_BranchManager_List_WithPattern tests listing branches with pattern filter.
func TestIntegration_BranchManager_List_WithPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	// Create branches with different prefixes
	mgr.Create(ctx, repo, CreateOptions{Name: "feature/x"})
	mgr.Create(ctx, repo, CreateOptions{Name: "feature/y"})
	mgr.Create(ctx, repo, CreateOptions{Name: "fix/z"})

	// List only feature branches
	list, err := mgr.List(ctx, repo, ListOptions{Pattern: "feature/*"})
	if err != nil {
		t.Fatalf("List() with pattern error = %v", err)
	}

	// Should have at least 2 feature branches
	featureCount := 0
	for _, b := range list {
		if len(b.Name) >= 8 && b.Name[:8] == "feature/" {
			featureCount++
		}
	}

	if featureCount < 2 {
		t.Errorf("Expected at least 2 feature branches, got %d", featureCount)
	}
}

// TestIntegration_BranchManager_Delete tests branch deletion.
func TestIntegration_BranchManager_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	// Create a branch
	branchName := "feature/to-delete"
	err := mgr.Create(ctx, repo, CreateOptions{Name: branchName})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify it exists
	exists, _ := mgr.Exists(ctx, repo, branchName)
	if !exists {
		t.Fatal("Branch should exist before deletion")
	}

	// Delete the branch (use Force since it's not merged to current branch)
	err = mgr.Delete(ctx, repo, DeleteOptions{Name: branchName, Force: true})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it no longer exists
	exists, _ = mgr.Exists(ctx, repo, branchName)
	if exists {
		t.Error("Branch should not exist after deletion")
	}
}

// TestIntegration_BranchManager_Get tests getting a specific branch.
func TestIntegration_BranchManager_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	// Create a branch
	branchName := "feature/get-test"
	err := mgr.Create(ctx, repo, CreateOptions{Name: branchName})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get the branch
	branch, err := mgr.Get(ctx, repo, branchName)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if branch.Name != branchName {
		t.Errorf("Get().Name = %q, want %q", branch.Name, branchName)
	}

	if branch.SHA == "" {
		t.Error("Get().SHA should not be empty")
	}
}

// TestIntegration_BranchManager_Current tests getting current branch.
func TestIntegration_BranchManager_Current(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	// Get current branch (should be main or master)
	current, err := mgr.Current(ctx, repo)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	// Should be main or master (depends on git version)
	if current.Name != "main" && current.Name != "master" {
		t.Errorf("Current().Name = %q, want 'main' or 'master'", current.Name)
	}

	if !current.IsHead {
		t.Error("Current branch should have IsHead = true")
	}
}

// TestIntegration_BranchManager_Create_AlreadyExists tests creating duplicate branch.
func TestIntegration_BranchManager_Create_AlreadyExists(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	branchName := "feature/duplicate"

	// Create branch first time
	err := mgr.Create(ctx, repo, CreateOptions{Name: branchName})
	if err != nil {
		t.Fatalf("First Create() error = %v", err)
	}

	// Try to create same branch again (should fail)
	err = mgr.Create(ctx, repo, CreateOptions{Name: branchName})
	if err == nil {
		t.Error("Second Create() should return error for existing branch")
	}
}

// TestIntegration_BranchManager_Create_Force tests force creating existing branch.
func TestIntegration_BranchManager_Create_Force(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	branchName := "feature/force-test"

	// Create branch
	err := mgr.Create(ctx, repo, CreateOptions{Name: branchName})
	if err != nil {
		t.Fatalf("First Create() error = %v", err)
	}

	// Force create same branch (should succeed)
	err = mgr.Create(ctx, repo, CreateOptions{Name: branchName, Force: true})
	if err != nil {
		t.Errorf("Force Create() error = %v", err)
	}
}

// TestIntegration_BranchManager_Delete_CurrentBranch tests deleting current branch.
func TestIntegration_BranchManager_Delete_CurrentBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	// Get current branch name
	current, _ := mgr.Current(ctx, repo)

	// Try to delete current branch (should fail)
	err := mgr.Delete(ctx, repo, DeleteOptions{Name: current.Name})
	if err == nil {
		t.Error("Delete() on current branch should return error")
	}
}

// TestIntegration_BranchManager_Get_NotFound tests getting non-existent branch.
func TestIntegration_BranchManager_Get_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	mgr := NewManager()
	repo := &repository.Repository{Path: repoDir}

	// Try to get non-existent branch
	_, err := mgr.Get(ctx, repo, "non-existent-branch")
	if err == nil {
		t.Error("Get() on non-existent branch should return error")
	}
}
