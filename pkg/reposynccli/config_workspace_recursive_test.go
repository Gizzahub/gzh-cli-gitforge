// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

// TestLoadSkipsConfigWorkspaceWithoutSyncRecursive pins ISSUE-101: like
// `workspace sync`, the status loader leaves out a config workspace that does
// not set sync.recursive, so its repositories are neither listed nor scanned
// as an orphan root, while a recursive sibling still loads.
func TestLoadSkipsConfigWorkspaceWithoutSyncRecursive(t *testing.T) {
	root := t.TempDir()
	plain := filepath.Join(root, "plain")
	deep := filepath.Join(root, "deep")
	mkdirAll(t, plain)
	mkdirAll(t, deep)
	writeGzGitConfig(t, root, "workspaces:\n"+
		"  plain:\n    type: config\n"+
		"  deep:\n    type: config\n    sync:\n      recursive: true\n")
	writeGzGitConfig(t, plain, "repositories:\n  - name: skipped\n    url: https://example.invalid/skipped.git\n")
	writeGzGitConfig(t, deep, "repositories:\n  - name: lib\n    url: https://example.invalid/lib.git\n")

	data, err := (FileSpecLoader{}).Load(context.Background(), filepath.Join(root, ".gz-git.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(data.Plan.Input.Repos) != 1 || data.Plan.Input.Repos[0].Name != "lib" {
		t.Fatalf("repos = %+v, want only lib", data.Plan.Input.Repos)
	}
	if slices.Contains(data.Plan.Options.Roots, plain) {
		t.Errorf("roots = %v, want %s left out", data.Plan.Options.Roots, plain)
	}
}
