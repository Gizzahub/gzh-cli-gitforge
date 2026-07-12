// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package tag

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// runTagGit runs a git command in dir, failing the test on error.
func runTagGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:noctx // test helper, no context available
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
	}
}

// tagGitOutput runs a git command in dir and returns its trimmed stdout.
func tagGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:noctx // test helper, no context available
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// tempGitRepo creates a temporary git repository for testing.
func tempGitRepo(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()

	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	cmd = exec.CommandContext(ctx, "git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	_ = cmd.Run()

	cmd = exec.CommandContext(ctx, "git", "config", "user.name", "Test")
	cmd.Dir = dir
	_ = cmd.Run()

	return dir
}

// tempGitRepoWithCommit creates a temp git repo with an initial commit.
func tempGitRepoWithCommit(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	dir := tempGitRepo(t)

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test"), 0o644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}

	cmd := exec.CommandContext(ctx, "git", "add", ".")
	cmd.Dir = dir
	_ = cmd.Run()

	cmd = exec.CommandContext(ctx, "git", "commit", "-m", "Initial commit")
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

func TestManager_Create(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	tests := []struct {
		name    string
		opts    CreateOptions
		wantErr bool
	}{
		{
			name:    "create lightweight tag",
			opts:    CreateOptions{Name: "v1.0.0"},
			wantErr: false,
		},
		{
			name:    "create annotated tag",
			opts:    CreateOptions{Name: "v1.1.0", Message: "Release 1.1.0"},
			wantErr: false,
		},
		{
			name:    "empty tag name",
			opts:    CreateOptions{Name: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Create(ctx, repo, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_Create_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	err := m.Create(ctx, nil, CreateOptions{Name: "v1.0.0"})
	if err == nil {
		t.Error("Create() expected error for nil repo, got nil")
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

	// List should work even with no tags
	tags, err := m.List(ctx, repo, ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should be empty initially
	if len(tags) != 0 {
		t.Errorf("List() expected 0 tags, got %d", len(tags))
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

func TestManager_Exists(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	// Tag should not exist
	exists, err := m.Exists(ctx, repo, "v1.0.0")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() expected false for non-existent tag")
	}

	// Create tag
	err = m.Create(ctx, repo, CreateOptions{Name: "v1.0.0"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Tag should exist now
	exists, err = m.Exists(ctx, repo, "v1.0.0")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() expected true for existing tag")
	}
}

func TestManager_Exists_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	_, err := m.Exists(ctx, nil, "v1.0.0")
	if err == nil {
		t.Error("Exists() expected error for nil repo, got nil")
	}
}

func TestManager_Latest(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	// No tags - should return ErrNoTags
	latest, err := m.Latest(ctx, repo)
	if err == nil || !errors.Is(err, ErrNoTags) {
		t.Fatalf("Latest() expected ErrNoTags for no tags, got err=%v, latest=%v", err, latest)
	}

	// Create some tags
	_ = m.Create(ctx, repo, CreateOptions{Name: "v1.0.0"})
	_ = m.Create(ctx, repo, CreateOptions{Name: "v1.1.0"})
	_ = m.Create(ctx, repo, CreateOptions{Name: "v2.0.0"})

	// Latest should be v2.0.0
	latest, err = m.Latest(ctx, repo)
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if latest == nil {
		t.Fatal("Latest() expected non-nil tag")
	}
	if latest.Name != "v2.0.0" {
		t.Errorf("Latest() expected v2.0.0, got %s", latest.Name)
	}
}

func TestManager_NextVersion(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	tests := []struct {
		name         string
		existingTags []string
		bump         string
		want         string
	}{
		{
			name:         "no tags - default",
			existingTags: nil,
			bump:         "patch",
			want:         "v0.1.0",
		},
		{
			name:         "patch bump",
			existingTags: []string{"v1.0.0"},
			bump:         "patch",
			want:         "v1.0.1",
		},
		{
			name:         "minor bump",
			existingTags: []string{"v1.0.0"},
			bump:         "minor",
			want:         "v1.1.0",
		},
		{
			name:         "major bump",
			existingTags: []string{"v1.0.0"},
			bump:         "major",
			want:         "v2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh repo for each test
			testDir := tempGitRepoWithCommit(t)
			testRepo := &repository.Repository{
				Path:     testDir,
				GitDir:   filepath.Join(testDir, ".git"),
				WorkTree: testDir,
			}

			// Create existing tags
			for _, tag := range tt.existingTags {
				_ = m.Create(ctx, testRepo, CreateOptions{Name: tag})
			}

			got, err := m.NextVersion(ctx, testRepo, tt.bump)
			if err != nil {
				t.Fatalf("NextVersion() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("NextVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_Delete(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	// Create a tag
	err := m.Create(ctx, repo, CreateOptions{Name: "v1.0.0"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify tag exists
	exists, _ := m.Exists(ctx, repo, "v1.0.0")
	if !exists {
		t.Fatal("Tag should exist before delete")
	}

	// Delete tag
	err = m.Delete(ctx, repo, DeleteOptions{Name: "v1.0.0"})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify tag is gone
	exists, _ = m.Exists(ctx, repo, "v1.0.0")
	if exists {
		t.Error("Tag should not exist after delete")
	}
}

func TestManager_Delete_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	err := m.Delete(ctx, nil, DeleteOptions{Name: "v1.0.0"})
	if err == nil {
		t.Error("Delete() expected error for nil repo, got nil")
	}
}

func TestManager_Delete_EmptyName(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	err := m.Delete(ctx, repo, DeleteOptions{Name: ""})
	if err == nil {
		t.Error("Delete() expected error for empty name, got nil")
	}
}

func TestManager_Push_NilRepo(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	err := m.Push(ctx, nil, PushOptions{All: true})
	if err == nil {
		t.Error("Push() expected error for nil repo, got nil")
	}
}

func TestManager_Push_NoTagOrAll(t *testing.T) {
	ctx := context.Background()
	m := NewManager()
	dir := tempGitRepoWithCommit(t)

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	err := m.Push(ctx, repo, PushOptions{})
	if err == nil {
		t.Error("Push() expected error when neither All nor Name is set")
	}
}

// TestManager_Push_SingleTag verifies that PushOptions.Name pushes only the named
// tag to the remote, leaving other local tags unpushed.
func TestManager_Push_SingleTag(t *testing.T) {
	ctx := context.Background()
	m := NewManager()

	// Bare remote to receive the push.
	remote := t.TempDir()
	runTagGit(t, remote, "init", "--bare")

	// Work repo with a commit, two local tags, wired to the bare remote.
	dir := tempGitRepoWithCommit(t)
	runTagGit(t, dir, "remote", "add", "origin", remote)
	runTagGit(t, dir, "tag", "v1.2.3")
	runTagGit(t, dir, "tag", "v2.0.0")

	repo := &repository.Repository{
		Path:     dir,
		GitDir:   filepath.Join(dir, ".git"),
		WorkTree: dir,
	}

	if err := m.Push(ctx, repo, PushOptions{Name: "v1.2.3"}); err != nil {
		t.Fatalf("Push(Name: v1.2.3) error = %v", err)
	}

	// Only the named tag reached the remote; the other local tag stayed behind.
	remoteTags := tagGitOutput(t, remote, "tag", "--list")
	if !strings.Contains(remoteTags, "v1.2.3") {
		t.Errorf("remote missing pushed tag v1.2.3; got: %q", remoteTags)
	}
	if strings.Contains(remoteTags, "v2.0.0") {
		t.Errorf("single-tag push should not push v2.0.0; got: %q", remoteTags)
	}
}

func TestParseSemVer(t *testing.T) {
	tests := []struct {
		version   string
		wantMajor int
		wantMinor int
		wantPatch int
	}{
		{"v1.2.3", 1, 2, 3},
		{"1.2.3", 1, 2, 3},
		{"v0.0.0", 0, 0, 0},
		{"v10.20.30", 10, 20, 30},
		{"invalid", 0, 0, 0},
		{"", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			major, minor, patch := parseSemVer(tt.version)
			if major != tt.wantMajor || minor != tt.wantMinor || patch != tt.wantPatch {
				t.Errorf("parseSemVer(%q) = (%d, %d, %d), want (%d, %d, %d)",
					tt.version, major, minor, patch, tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}

func TestCompareSemVer(t *testing.T) {
	tests := []struct {
		a, b string
		want int // >0 if a>b, <0 if a<b, 0 if equal
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v2.0.0", "v1.0.0", 1},
		{"v1.0.0", "v2.0.0", -1},
		{"v1.1.0", "v1.0.0", 1},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.0.0", "v1.0.1", -1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := compareSemVer(tt.a, tt.b)
			if (tt.want > 0 && got <= 0) || (tt.want < 0 && got >= 0) || (tt.want == 0 && got != 0) {
				t.Errorf("compareSemVer(%q, %q) = %d, want sign of %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
