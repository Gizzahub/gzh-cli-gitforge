// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
)

// TestConfigDirSpecLoaderResolvesNestedConfigReposAgainstChildDir covers the
// status side of ISSUE-96: a nested config workspace's relative repository
// paths resolve inside the nested config's own directory, where sync placed
// them, instead of the top-level config directory; absolute paths are kept.
func TestConfigDirSpecLoaderResolvesNestedConfigReposAgainstChildDir(t *testing.T) {
	root := t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0o750); err != nil {
		t.Fatal(err)
	}
	topBody := "workspaces:\n  child:\n    type: config\n    path: child\n"
	if err := os.WriteFile(filepath.Join(root, DefaultConfigFile), []byte(topBody), 0o600); err != nil {
		t.Fatal(err)
	}
	childBody := "repositories:\n" +
		"  - name: lib\n    url: https://example.invalid/lib.git\n" +
		"  - name: custom\n    url: https://example.invalid/custom.git\n    path: sub/custom\n" +
		"  - name: abs\n    url: https://example.invalid/abs.git\n    path: " + elsewhere + "\n"
	if err := os.WriteFile(filepath.Join(child, DefaultConfigFile), []byte(childBody), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	loader := configDirSpecLoader{base: reposynccli.FileSpecLoader{}}
	data, err := loader.Load(context.Background(), filepath.Join(root, DefaultConfigFile))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]string{
		"lib":    filepath.Join(root, "child", "lib"),
		"custom": filepath.Join(root, "child", "sub", "custom"),
		"abs":    elsewhere,
	}
	if len(data.Plan.Input.Repos) != len(want) {
		t.Fatalf("repos = %+v, want %d", data.Plan.Input.Repos, len(want))
	}
	for _, repo := range data.Plan.Input.Repos {
		if repo.TargetPath != want[repo.Name] {
			t.Errorf("%s: TargetPath = %q, want %q", repo.Name, repo.TargetPath, want[repo.Name])
		}
	}
}
