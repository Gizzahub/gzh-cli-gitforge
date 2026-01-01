// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package stash

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// tempGitRepo creates a temporary git repository for testing.
func tempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	_ = cmd.Run()

	return dir
}

// tempGitRepoWithCommit creates a temp git repo with an initial commit.
func tempGitRepoWithCommit(t *testing.T) string {
	t.Helper()
	dir := tempGitRepo(t)

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test"), 0o644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	_ = cmd.Run()

	return dir
}

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestManager_Save(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	// Create a modified file to stash
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("modified"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		opts    SaveOptions
		wantErr bool
	}{
		{
			name:    "save with message",
			opts:    SaveOptions{Message: "test stash"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Save(ctx, repo, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_Save_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	err := m.Save(ctx, nil, SaveOptions{Message: "test"})
	if err == nil {
		t.Error("Save() expected error for nil repo, got nil")
	}
}

func TestManager_List(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	// List should work even with no stashes
	stashes, err := m.List(ctx, repo, ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should be empty initially
	if len(stashes) != 0 {
		t.Errorf("List() expected 0 stashes, got %d", len(stashes))
	}
}

func TestManager_List_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	_, err := m.List(ctx, nil, ListOptions{})
	if err == nil {
		t.Error("List() expected error for nil repo, got nil")
	}
}

func TestManager_Count(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	count, err := m.Count(ctx, repo)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}

	if count != 0 {
		t.Errorf("Count() expected 0, got %d", count)
	}
}

func TestManager_Count_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	_, err := m.Count(ctx, nil)
	if err == nil {
		t.Error("Count() expected error for nil repo, got nil")
	}
}

func TestManager_Pop_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	err := m.Pop(ctx, nil, PopOptions{})
	if err == nil {
		t.Error("Pop() expected error for nil repo, got nil")
	}
}

func TestManager_Apply_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	err := m.Apply(ctx, nil, PopOptions{})
	if err == nil {
		t.Error("Apply() expected error for nil repo, got nil")
	}
}

func TestManager_Drop_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	err := m.Drop(ctx, nil, DropOptions{})
	if err == nil {
		t.Error("Drop() expected error for nil repo, got nil")
	}
}

func TestManager_Clear_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	err := m.Clear(ctx, nil)
	if err == nil {
		t.Error("Clear() expected error for nil repo, got nil")
	}
}

func TestManager_StashWorkflow(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	// Create and commit a file first (must be tracked to stash)
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd := exec.Command("git", "add", "test.txt")
	cmd.Dir = dir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "add test file")
	cmd.Dir = dir
	_ = cmd.Run()

	// Now modify the file (this can be stashed)
	if err := os.WriteFile(testFile, []byte("modified content"), 0o644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Save stash
	err := m.Save(ctx, repo, SaveOptions{Message: "workflow test"})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Count should be 1
	count, err := m.Count(ctx, repo)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Errorf("Count() expected 1, got %d", count)
	}

	// List should return 1 stash
	stashes, err := m.List(ctx, repo, ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(stashes) != 1 {
		t.Errorf("List() expected 1 stash, got %d", len(stashes))
	}

	// Pop stash
	err = m.Pop(ctx, repo, PopOptions{})
	if err != nil {
		t.Fatalf("Pop() error = %v", err)
	}

	// Count should be 0 again
	count, err = m.Count(ctx, repo)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Count() expected 0 after pop, got %d", count)
	}
}
