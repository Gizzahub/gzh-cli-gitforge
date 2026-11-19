// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
)

// TestSyncAndStatusResolveNestedWorkspaceBaseDir pins that the sync loader
// (pkg/config) and the status loader (pkg/reposynccli) resolve a relative
// workspace path against the directory of the config that declares it, for a
// config nested two levels deep (ISSUE-391).
func TestSyncAndStatusResolveNestedWorkspaceBaseDir(t *testing.T) {
	root := t.TempDir()
	childDir := filepath.Join(root, "dev", "child")
	if err := os.MkdirAll(childDir, 0o750); err != nil {
		t.Fatal(err)
	}
	write := func(dir, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, ".gz-git.yaml"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(root, `workspaces:
  child:
    type: config
    path: dev/child
    sync:
      recursive: true
`)
	write(childDir, `workspaces:
  leaf:
    type: git
    url: https://example.com/team/leaf.git
    path: ./sub/leaf
`)

	// Sync side: the root load resolves the child workspace, and the child
	// config, loaded from there, resolves the leaf.
	rootCfg, err := config.LoadConfigRecursive(root, ".gz-git.yaml")
	if err != nil {
		t.Fatalf("LoadConfigRecursive(root): %v", err)
	}
	syncChild := rootCfg.Workspaces["child"].Path
	childCfg, err := config.LoadConfigRecursive(syncChild, ".gz-git.yaml")
	if err != nil {
		t.Fatalf("LoadConfigRecursive(child): %v", err)
	}
	syncLeaf := childCfg.Workspaces["leaf"].Path

	// Status side: the root config yields the leaf as a repository target.
	data, err := reposynccli.FileSpecLoader{}.Load(context.Background(), filepath.Join(root, ".gz-git.yaml"))
	if err != nil {
		t.Fatalf("FileSpecLoader.Load: %v", err)
	}
	var statusLeaf string
	for _, repo := range data.Plan.Input.Repos {
		if repo.Name == "leaf" {
			statusLeaf = repo.TargetPath
		}
	}

	want := filepath.Join(childDir, "sub", "leaf")
	if syncChild != childDir {
		t.Errorf("sync child path = %q, want %q", syncChild, childDir)
	}
	if syncLeaf != want {
		t.Errorf("sync leaf path = %q, want %q", syncLeaf, want)
	}
	if statusLeaf != want {
		t.Errorf("status leaf target = %q, want %q", statusLeaf, want)
	}
}

// TestResolveWorkspacePathUsesSharedRule keeps the workspacecli adapter on
// config.ResolvePath.
func TestResolveWorkspacePathUsesSharedRule(t *testing.T) {
	base := filepath.FromSlash("/cfg/dir")
	for _, path := range []string{"", ".", "x", "./x", "~/x", "~", filepath.FromSlash("/abs/x")} {
		want, err := config.ResolvePath(base, path)
		if err != nil {
			t.Fatal(err)
		}
		if got := resolveWorkspacePath(path, base); got != want {
			t.Errorf("resolveWorkspacePath(%q) = %q, want shared %q", path, got, want)
		}
	}
}
