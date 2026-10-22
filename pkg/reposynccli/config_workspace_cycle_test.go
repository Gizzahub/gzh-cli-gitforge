// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeGzGitConfig writes a .gz-git.yaml document into dir.
func writeGzGitConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".gz-git.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// mkdirAll creates path with parents inside the temp layout.
func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatal(err)
	}
}

// loadWarnings loads configPath and returns what the loader printed to
// stderr. The load error is ignored: skipped workspaces may leave no
// repositories, and the regression under test was a fatal stack overflow.
func loadWarnings(t *testing.T, configPath string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = f
	_, _ = (FileSpecLoader{}).Load(context.Background(), configPath)
	os.Stderr = orig
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// cycleWarnings counts the config workspace cycle warnings in stderr output.
func cycleWarnings(stderr string) int {
	return strings.Count(stderr, "config workspace cycle")
}

// TestLoadPathlessConfigWorkspaceResolvesToWorkspaceName pins ISSUE-102: a
// `type: config` workspace without a path defaults to the workspace name
// (compact config convention, matching `workspace sync`), so it loads the
// workspace's own directory instead of reloading the parent config.
func TestLoadPathlessConfigWorkspaceResolvesToWorkspaceName(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	mkdirAll(t, child)
	writeGzGitConfig(t, root, "workspaces:\n  child:\n    type: config\n")
	writeGzGitConfig(t, child, "repositories:\n  - name: lib\n    url: https://example.invalid/lib.git\n")

	data, err := (FileSpecLoader{}).Load(context.Background(), filepath.Join(root, ".gz-git.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(data.Plan.Input.Repos) != 1 {
		t.Fatalf("repos = %+v, want 1", data.Plan.Input.Repos)
	}
	if got, want := data.Plan.Input.Repos[0].TargetPath, filepath.Join(root, "child", "lib"); got != want {
		t.Errorf("lib: TargetPath = %q, want %q", got, want)
	}
}

// TestLoadPathlessConfigWorkspaceWithoutDirectoryDoesNotRecurse covers the
// ISSUE-102 overflow shape: a pathless config workspace whose directory does
// not exist is skipped as missing, not reloaded from the parent config.
func TestLoadPathlessConfigWorkspaceWithoutDirectoryDoesNotRecurse(t *testing.T) {
	root := t.TempDir()
	writeGzGitConfig(t, root, "workspaces:\n  child:\n    type: config\n")

	stderr := loadWarnings(t, filepath.Join(root, ".gz-git.yaml"))
	if n := strings.Count(stderr, "workspace 'child' skipped"); n != 1 {
		t.Errorf("child skip warnings = %d, want 1; stderr:\n%s", n, stderr)
	}
}

// TestLoadConfigWorkspaceCycles covers config workspaces that point back at
// an enclosing config directory: each must be skipped once as a cycle.
func TestLoadConfigWorkspaceCycles(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string)
	}{
		{
			name: "explicit self path",
			setup: func(t *testing.T, root string) {
				t.Helper()
				writeGzGitConfig(t, root, "workspaces:\n  child:\n    type: config\n    path: .\n")
			},
		},
		{
			name: "A to B to A",
			setup: func(t *testing.T, root string) {
				t.Helper()
				a := filepath.Join(root, "a")
				b := filepath.Join(root, "b")
				mkdirAll(t, a)
				mkdirAll(t, b)
				writeGzGitConfig(t, root, "workspaces:\n  a:\n    type: config\n    path: a\n")
				writeGzGitConfig(t, a, "workspaces:\n  b:\n    type: config\n    path: ../b\n")
				writeGzGitConfig(t, b, "workspaces:\n  a:\n    type: config\n    path: ../a\n")
			},
		},
		{
			name: "symlink to the config directory",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Symlink(root, filepath.Join(root, "link")); err != nil {
					t.Skipf("symlink: %v", err)
				}
				writeGzGitConfig(t, root, "workspaces:\n  link:\n    type: config\n    path: link\n")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			stderr := loadWarnings(t, filepath.Join(root, ".gz-git.yaml"))
			if n := cycleWarnings(stderr); n != 1 {
				t.Errorf("cycle warnings = %d, want 1; stderr:\n%s", n, stderr)
			}
		})
	}
}

// TestLoadSiblingConfigWorkspacesSharingNestedConfig pins the false-positive
// guard: two sibling config workspaces may both delegate to the same nested
// config; only a directory already on the current nesting path is a cycle.
// The repo dedupes to one entry either way, so the guard is the absence of a
// cycle warning.
func TestLoadSiblingConfigWorkspacesSharingNestedConfig(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	shared := filepath.Join(root, "shared")
	mkdirAll(t, one)
	mkdirAll(t, two)
	mkdirAll(t, shared)
	writeGzGitConfig(t, root, "workspaces:\n  x:\n    type: config\n    path: one\n  y:\n    type: config\n    path: two\n")
	writeGzGitConfig(t, one, "workspaces:\n  s:\n    type: config\n    path: ../shared\n")
	writeGzGitConfig(t, two, "workspaces:\n  s:\n    type: config\n    path: ../shared\n")
	writeGzGitConfig(t, shared, "repositories:\n  - name: lib\n    url: https://example.invalid/lib.git\n")

	if stderr := loadWarnings(t, filepath.Join(root, ".gz-git.yaml")); cycleWarnings(stderr) != 0 {
		t.Errorf("unexpected cycle warning; stderr:\n%s", stderr)
	}

	data, err := (FileSpecLoader{}).Load(context.Background(), filepath.Join(root, ".gz-git.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(data.Plan.Input.Repos) != 1 {
		t.Fatalf("repos = %+v, want 1", data.Plan.Input.Repos)
	}
	if got, want := data.Plan.Input.Repos[0].TargetPath, filepath.Join(shared, "lib"); got != want {
		t.Errorf("lib: TargetPath = %q, want %q", got, want)
	}
}
