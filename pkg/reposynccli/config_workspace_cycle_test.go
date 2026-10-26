// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
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

// TestLoadPathlessConfigWorkspaceResolvesToWorkspaceName pins ISSUE-102: a
// `type: config` workspace without a path defaults to the workspace name
// (compact config convention, matching `workspace sync`), so it loads the
// workspace's own directory instead of reloading the parent config.
func TestLoadPathlessConfigWorkspaceResolvesToWorkspaceName(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	mkdirAll(t, child)
	writeGzGitConfig(t, root, "workspaces:\n  child:\n    type: config\n    sync:\n      recursive: true\n")
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
	writeGzGitConfig(t, root, "workspaces:\n  child:\n    type: config\n    sync:\n      recursive: true\n")

	stderr := loadWarnings(t, filepath.Join(root, ".gz-git.yaml"))
	if n := strings.Count(stderr, "workspace 'child' skipped"); n != 1 {
		t.Errorf("child skip warnings = %d, want 1; stderr:\n%s", n, stderr)
	}
}

// TestLoadConfigWorkspaceCycles covers config workspaces that point back at
// an enclosing config directory: each is a config error (ISSUE-104).
func TestLoadConfigWorkspaceCycles(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string)
	}{
		{
			name: "explicit self path",
			setup: func(t *testing.T, root string) {
				t.Helper()
				writeGzGitConfig(t, root, "workspaces:\n  child:\n    type: config\n    sync:\n      recursive: true\n    path: .\n")
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
				writeGzGitConfig(t, root, "workspaces:\n  a:\n    type: config\n    sync:\n      recursive: true\n    path: a\n")
				writeGzGitConfig(t, a, "workspaces:\n  b:\n    type: config\n    sync:\n      recursive: true\n    path: ../b\n")
				writeGzGitConfig(t, b, "workspaces:\n  a:\n    type: config\n    sync:\n      recursive: true\n    path: ../a\n")
			},
		},
		{
			name: "symlink to the config directory",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Symlink(root, filepath.Join(root, "link")); err != nil {
					t.Skipf("symlink: %v", err)
				}
				writeGzGitConfig(t, root, "workspaces:\n  link:\n    type: config\n    sync:\n      recursive: true\n    path: link\n")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			_, err := (FileSpecLoader{}).Load(context.Background(), filepath.Join(root, ".gz-git.yaml"))
			if !errors.Is(err, config.ErrConfigCycle) {
				t.Fatalf("Load err = %v, want ErrConfigCycle", err)
			}
		})
	}
}

// TestLoadSiblingConfigWorkspacesSharingNestedConfig pins the false-positive
// guard: two sibling config workspaces may both delegate to the same nested
// config; only a directory already on the current nesting path is a cycle.
// The repo dedupes to one entry either way, so the guard is that Load
// succeeds.
func TestLoadSiblingConfigWorkspacesSharingNestedConfig(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	shared := filepath.Join(root, "shared")
	mkdirAll(t, one)
	mkdirAll(t, two)
	mkdirAll(t, shared)
	writeGzGitConfig(t, root, "workspaces:\n  x:\n    type: config\n    sync:\n      recursive: true\n    path: one\n  y:\n    type: config\n    sync:\n      recursive: true\n    path: two\n")
	writeGzGitConfig(t, one, "workspaces:\n  s:\n    type: config\n    sync:\n      recursive: true\n    path: ../shared\n")
	writeGzGitConfig(t, two, "workspaces:\n  s:\n    type: config\n    sync:\n      recursive: true\n    path: ../shared\n")
	writeGzGitConfig(t, shared, "repositories:\n  - name: lib\n    url: https://example.invalid/lib.git\n")

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

// writeNestedConfigDirs writes dirs nested type:config workspaces under root
// (d0). Each directory dN points at dN+1 with sync.recursive. The last
// directory gets leafBody as its config.
func writeNestedConfigDirs(t *testing.T, root string, dirs int, leafBody string) {
	t.Helper()
	cur := root
	for i := 1; i < dirs; i++ {
		childName := fmt.Sprintf("d%d", i)
		child := filepath.Join(cur, childName)
		mkdirAll(t, child)
		writeGzGitConfig(t, cur, "workspaces:\n  "+childName+":\n    type: config\n    path: "+childName+"\n    sync:\n      recursive: true\n")
		cur = child
	}
	writeGzGitConfig(t, cur, leafBody)
}

func TestLoadConfigWorkspaceDepth(t *testing.T) {
	t.Run("12 nested config dirs exceed depth", func(t *testing.T) {
		root := t.TempDir()
		writeNestedConfigDirs(t, root, 12, "workspaces:\n  leaf:\n    type: git\n    url: https://example.invalid/leaf.git\n")

		_, err := (FileSpecLoader{}).Load(context.Background(), filepath.Join(root, ".gz-git.yaml"))
		if !errors.Is(err, config.ErrConfigDepth) {
			t.Fatalf("Load err = %v, want ErrConfigDepth", err)
		}
	})

	t.Run("11 levels total load without a depth error", func(t *testing.T) {
		root := t.TempDir()
		writeNestedConfigDirs(t, root, 11, "repositories:\n  - name: lib\n    url: https://example.invalid/lib.git\n")

		data, err := (FileSpecLoader{}).Load(context.Background(), filepath.Join(root, ".gz-git.yaml"))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(data.Plan.Input.Repos) != 1 {
			t.Fatalf("repos = %+v, want 1", data.Plan.Input.Repos)
		}
	})
}
