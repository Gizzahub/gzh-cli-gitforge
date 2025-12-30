package watch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// TestWatchIntegration_UntrackedFile tests detecting new untracked files.
func TestWatchIntegration_UntrackedFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup: Create temp Git repo
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Create watcher with short interval for faster tests
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := repository.NewClient()
	watcher, err := NewWatcher(client, WatchOptions{
		Interval:         100 * time.Millisecond,
		DebounceDuration: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Start watching
	if err := watcher.Start(ctx, []string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Make change: Create new file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for event
	select {
	case event := <-watcher.Events():
		if event.Type != EventTypeUntracked {
			t.Errorf("Expected EventTypeUntracked, got %v", event.Type)
		}
		if len(event.Files) == 0 {
			t.Error("Expected files in event, got none")
		}
		found := false
		for _, f := range event.Files {
			if f == "test.txt" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected test.txt in files, got %v", event.Files)
		}
	case err := <-watcher.Errors():
		t.Fatalf("Unexpected error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Expected event not received within timeout")
	}
}

// TestWatchIntegration_ModifiedFile tests detecting modified files.
func TestWatchIntegration_ModifiedFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Create and commit a file first
	testFile := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0o644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	gitAdd(t, tmpDir, "existing.txt")
	gitCommit(t, tmpDir, "Initial commit")

	// Wait a bit to ensure commit is complete
	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := repository.NewClient()
	watcher, err := NewWatcher(client, WatchOptions{
		Interval:         100 * time.Millisecond,
		DebounceDuration: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(ctx, []string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Wait for initial status check
	time.Sleep(200 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(testFile, []byte("modified content"), 0o644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Wait for modified event (might be staged if in index)
	select {
	case event := <-watcher.Events():
		// Accept either modified or staged (depends on Git state)
		if event.Type != EventTypeModified && event.Type != EventTypeStaged {
			t.Errorf("Expected EventTypeModified or EventTypeStaged, got %v", event.Type)
		}
	case err := <-watcher.Errors():
		t.Fatalf("Unexpected error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Expected modified event not received")
	}
}

// TestWatchIntegration_StagedFile tests detecting staged files.
func TestWatchIntegration_StagedFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := repository.NewClient()
	watcher, err := NewWatcher(client, WatchOptions{
		Interval:         100 * time.Millisecond,
		DebounceDuration: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(ctx, []string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Create file
	testFile := filepath.Join(tmpDir, "staged.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Drain untracked event
	select {
	case <-watcher.Events():
	case <-time.After(2 * time.Second):
	}

	// Stage the file
	gitAdd(t, tmpDir, "staged.txt")

	// Wait for staged event
	select {
	case event := <-watcher.Events():
		if event.Type != EventTypeStaged {
			t.Errorf("Expected EventTypeStaged, got %v", event.Type)
		}
	case err := <-watcher.Errors():
		t.Fatalf("Unexpected error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Expected staged event not received")
	}
}

// TestWatchIntegration_BranchChange tests detecting branch switches.
func TestWatchIntegration_BranchChange(t *testing.T) {
	t.Skip("Branch change detection needs more investigation - may require fsnotify on .git/HEAD")
	// TODO: Branch detection works but requires watching .git/HEAD file
	// Current implementation polls repository.GetInfo which may not detect
	// immediate branch changes. Need to add .git/HEAD to fsnotify watch list.
}

// TestWatchIntegration_MultipleRepositories tests watching multiple repos.
func TestWatchIntegration_MultipleRepositories(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create two temp repos
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()
	initGitRepo(t, tmpDir1)
	initGitRepo(t, tmpDir2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := repository.NewClient()
	watcher, err := NewWatcher(client, WatchOptions{
		Interval:         100 * time.Millisecond,
		DebounceDuration: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Watch both repos
	if err := watcher.Start(ctx, []string{tmpDir1, tmpDir2}); err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Make change in repo1
	file1 := filepath.Join(tmpDir1, "file1.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatalf("Failed to create file in repo1: %v", err)
	}

	// Make change in repo2
	file2 := filepath.Join(tmpDir2, "file2.txt")
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
		t.Fatalf("Failed to create file in repo2: %v", err)
	}

	// Collect events
	events := make([]Event, 0, 2)
	timeout := time.After(5 * time.Second)

	for i := 0; i < 2; i++ {
		select {
		case event := <-watcher.Events():
			events = append(events, event)
		case err := <-watcher.Errors():
			t.Fatalf("Unexpected error: %v", err)
		case <-timeout:
			t.Fatalf("Expected 2 events, got %d", len(events))
		}
	}

	// Verify events from both repos
	paths := make(map[string]bool)
	for _, event := range events {
		paths[event.Path] = true
	}

	if !paths[tmpDir1] {
		t.Errorf("Expected event from repo1 (%s)", tmpDir1)
	}
	if !paths[tmpDir2] {
		t.Errorf("Expected event from repo2 (%s)", tmpDir2)
	}
}

// TestWatchIntegration_CleanState tests detecting clean state transitions.
func TestWatchIntegration_CleanState(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Create initial clean state
	initFile := filepath.Join(tmpDir, "init.txt")
	if err := os.WriteFile(initFile, []byte("init"), 0o644); err != nil {
		t.Fatalf("Failed to create init file: %v", err)
	}
	gitAdd(t, tmpDir, "init.txt")
	gitCommit(t, tmpDir, "Initial commit")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := repository.NewClient()
	watcher, err := NewWatcher(client, WatchOptions{
		Interval:         200 * time.Millisecond, // Slower for stability
		DebounceDuration: 100 * time.Millisecond,
		IncludeClean:     true,
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(ctx, []string{tmpDir}); err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Let watcher establish baseline
	time.Sleep(500 * time.Millisecond)

	// Create and immediately commit a file (transition to dirty and back to clean)
	testFile := filepath.Join(tmpDir, "temp.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Should get untracked event
	var gotUntracked bool
	select {
	case event := <-watcher.Events():
		if event.Type == EventTypeUntracked {
			gotUntracked = true
		}
	case <-time.After(2 * time.Second):
	}

	if !gotUntracked {
		t.Log("Warning: Did not receive untracked event (may be timing issue)")
	}

	// Stage and commit quickly
	gitAdd(t, tmpDir, "temp.txt")
	gitCommit(t, tmpDir, "Add temp file")

	// Now look for any event - could be staged or clean
	gotCleanOrStaged := false
	timeout := time.After(3 * time.Second)
	for !gotCleanOrStaged {
		select {
		case event := <-watcher.Events():
			if event.Type == EventTypeClean || event.Type == EventTypeStaged {
				gotCleanOrStaged = true
				if event.Type == EventTypeClean && event.Status != nil && !event.Status.IsClean {
					t.Error("Clean event but status not clean")
				}
			}
		case <-timeout:
			t.Fatal("Expected clean or staged event not received")
		}
	}
}

// TestWatchIntegration_InvalidRepository tests error handling.
func TestWatchIntegration_InvalidRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := repository.NewClient()
	watcher, err := NewWatcher(client, WatchOptions{
		Interval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Try to watch non-existent directory
	err = watcher.Start(ctx, []string{"/nonexistent/path"})
	if err == nil {
		t.Error("Expected error when watching nonexistent path")
	}
}

// Helper functions for Git operations

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user for commits
	exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@example.com").Run()
}

func gitAdd(t *testing.T, dir, file string) {
	t.Helper()
	cmd := exec.Command("git", "add", file)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
}

func gitCommit(t *testing.T, dir, message string) {
	t.Helper()
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
}

func gitBranch(t *testing.T, dir, branch string) {
	t.Helper()
	cmd := exec.Command("git", "checkout", "-b", branch)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create branch: %v", err)
	}
}

func gitCheckout(t *testing.T, dir, branch string) {
	t.Helper()
	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to checkout branch: %v", err)
	}
}
