// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// nestedConfigLayout is the directory tree written by writeNestedConfigLayout.
type nestedConfigLayout struct {
	// root holds the top-level config and the child workspace.
	root string
	// configPath is the top-level config file.
	configPath string
	// elsewhere is an absolute target path outside root.
	elsewhere string
}

// writeNestedConfigLayout writes a top-level workspaces config that
// delegates to a nested config workspace at child/, whose entries carry a
// plain relative, a nested relative, and an absolute target path.
func writeNestedConfigLayout(t *testing.T) nestedConfigLayout {
	t.Helper()
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0o750); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	top := "workspaces:\n  child:\n    type: config\n    sync:\n      recursive: true\n    path: child\n"
	if err := os.WriteFile(filepath.Join(root, ".gz-git.yaml"), []byte(top), 0o600); err != nil {
		t.Fatal(err)
	}
	childCfg := "repositories:\n" +
		"  - name: lib\n    url: https://example.invalid/lib.git\n" +
		"  - name: custom\n    url: https://example.invalid/custom.git\n    path: sub/custom\n" +
		"  - name: abs\n    url: https://example.invalid/abs.git\n    path: " + elsewhere + "\n"
	if err := os.WriteFile(filepath.Join(child, ".gz-git.yaml"), []byte(childCfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return nestedConfigLayout{
		root:       root,
		configPath: filepath.Join(root, ".gz-git.yaml"),
		elsewhere:  elsewhere,
	}
}

// TestLoadRebasesNestedConfigReposOntoNestedConfigDir pins ISSUE-96: a
// nested config's relative repository paths are placed inside the nested
// config's own directory, where `workspace sync` clones them, and absolute
// paths are kept.
func TestLoadRebasesNestedConfigReposOntoNestedConfigDir(t *testing.T) {
	f := writeNestedConfigLayout(t)

	data, err := (FileSpecLoader{}).Load(context.Background(), f.configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]string{
		"lib":    filepath.Join(f.root, "child", "lib"),
		"custom": filepath.Join(f.root, "child", "sub", "custom"),
		"abs":    f.elsewhere,
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

// TestLoadRebasesNestedConfigReposWithRelativeConfigPath loads the same
// layout through a relative config path: the rebase must still land on the
// absolute child directory, not on a working-directory-relative path that a
// later join would double-prefix.
func TestLoadRebasesNestedConfigReposWithRelativeConfigPath(t *testing.T) {
	f := writeNestedConfigLayout(t)
	t.Chdir(f.root)

	data, err := (FileSpecLoader{}).Load(context.Background(), ".gz-git.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]string{
		"lib":    filepath.Join(f.root, "child", "lib"),
		"custom": filepath.Join(f.root, "child", "sub", "custom"),
		"abs":    f.elsewhere,
	}
	if len(data.Plan.Input.Repos) != len(want) {
		t.Fatalf("repos = %+v, want %d", data.Plan.Input.Repos, len(want))
	}
	for _, repo := range data.Plan.Input.Repos {
		if !filepath.IsAbs(repo.TargetPath) {
			t.Errorf("%s: TargetPath = %q, want an absolute path", repo.Name, repo.TargetPath)
			continue
		}
		if repo.TargetPath != want[repo.Name] {
			t.Errorf("%s: TargetPath = %q, want %q", repo.Name, repo.TargetPath, want[repo.Name])
		}
	}
}

// TestLoadRebasesGrandchildNestedConfigRepos covers two config hops: the
// child config itself delegates to a grandchild config workspace, whose
// relative entries must resolve inside the grandchild directory.
func TestLoadRebasesGrandchildNestedConfigRepos(t *testing.T) {
	f := writeNestedConfigLayout(t)
	gc := filepath.Join(f.root, "child", "gc")
	if err := os.MkdirAll(gc, 0o750); err != nil {
		t.Fatal(err)
	}
	childCfg := "workspaces:\n  gc:\n    type: config\n    sync:\n      recursive: true\n    path: gc\n"
	if err := os.WriteFile(filepath.Join(f.root, "child", ".gz-git.yaml"), []byte(childCfg), 0o600); err != nil {
		t.Fatal(err)
	}
	gcCfg := "repositories:\n  - name: deep\n    url: https://example.invalid/deep.git\n"
	if err := os.WriteFile(filepath.Join(gc, ".gz-git.yaml"), []byte(gcCfg), 0o600); err != nil {
		t.Fatal(err)
	}

	data, err := (FileSpecLoader{}).Load(context.Background(), f.configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(data.Plan.Input.Repos) != 1 {
		t.Fatalf("repos = %+v, want 1", data.Plan.Input.Repos)
	}
	if got := data.Plan.Input.Repos[0].Name; got != "deep" {
		t.Fatalf("repo = %s, want deep", got)
	}
	if got, want := data.Plan.Input.Repos[0].TargetPath, filepath.Join(f.root, "child", "gc", "deep"); got != want {
		t.Errorf("deep: TargetPath = %q, want %q", got, want)
	}
}
