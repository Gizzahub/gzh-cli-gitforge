// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
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

// TestLoadInfersConfigWorkspaceLikeSync pins ISSUE-104: an untyped workspace
// without a url and with sync.recursive is a config workspace for `workspace
// sync` (config.ClassifyWorkspace), so status loads its child config too.
func TestLoadInfersConfigWorkspaceLikeSync(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "deep")
	mkdirAll(t, deep)
	writeGzGitConfig(t, root, "workspaces:\n  deep:\n    sync:\n      recursive: true\n")
	writeGzGitConfig(t, deep, "repositories:\n  - name: lib\n    url: https://example.invalid/lib.git\n")

	data, err := (FileSpecLoader{}).Load(context.Background(), filepath.Join(root, ".gz-git.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(data.Plan.Input.Repos) != 1 || data.Plan.Input.Repos[0].Name != "lib" {
		t.Fatalf("repos = %+v, want only lib", data.Plan.Input.Repos)
	}
}

// TestLoadRefusesConfigWorkspaceWithSource pins ISSUE-104: `type: config`
// with a forge source is contradictory, so status fails like sync does, also
// when the entry sits in a nested config.
func TestLoadRefusesConfigWorkspaceWithSource(t *testing.T) {
	conflict := "workspaces:\n  mixed:\n    type: config\n    source:\n      provider: github\n      org: example\n"

	t.Run("top level", func(t *testing.T) {
		root := t.TempDir()
		writeGzGitConfig(t, root, conflict)

		_, err := (FileSpecLoader{}).Load(context.Background(), filepath.Join(root, ".gz-git.yaml"))
		if !errors.Is(err, config.ErrWorkspaceTypeConflict) {
			t.Fatalf("Load error = %v, want ErrWorkspaceTypeConflict", err)
		}
	})

	t.Run("nested", func(t *testing.T) {
		root := t.TempDir()
		deep := filepath.Join(root, "deep")
		mkdirAll(t, deep)
		writeGzGitConfig(t, root, "workspaces:\n  deep:\n    type: config\n    sync:\n      recursive: true\n")
		writeGzGitConfig(t, deep, conflict)

		_, err := (FileSpecLoader{}).Load(context.Background(), filepath.Join(root, ".gz-git.yaml"))
		if !errors.Is(err, config.ErrWorkspaceTypeConflict) {
			t.Fatalf("Load error = %v, want ErrWorkspaceTypeConflict", err)
		}
	})
}
