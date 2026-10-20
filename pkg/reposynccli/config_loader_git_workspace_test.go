// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func mkGitDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func loadGitWorkspaceConfig(t *testing.T, configDir, body string) ([]reposync.RepoSpec, error) {
	t.Helper()
	path := filepath.Join(configDir, ".gz-git.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := FileSpecLoader{}.Load(context.Background(), path)
	if err != nil {
		return nil, err
	}
	repos := data.Plan.Input.Repos
	sort.Slice(repos, func(i, j int) bool { return repos[i].Name < repos[j].Name })
	return repos, nil
}

// A type: git workspace whose path is itself a checkout is one repository,
// as workspace sync plans it (ISSUE-72).
func TestGitWorkspacePathIsOneRepository(t *testing.T) {
	configDir := t.TempDir()
	clone := filepath.Join(t.TempDir(), "engine")
	mkGitDir(t, clone)
	mkGitDir(t, filepath.Join(clone, "vendor-child"))
	worktree := filepath.Join(t.TempDir(), "engine-wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: /elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	repos, err := loadGitWorkspaceConfig(t, configDir, "workspaces:\n"+
		"  engine:\n    type: git\n    path: "+clone+"\n    url: git@example.com:team/engine.git\n"+
		"  engine-wt:\n    type: git\n    path: "+worktree+"\n    url: git@example.com:team/engine.git\n")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []reposync.RepoSpec{
		{Name: "engine", TargetPath: clone, CloneURL: "git@example.com:team/engine.git"},
		{Name: "engine-wt", TargetPath: worktree, CloneURL: "git@example.com:team/engine.git"},
	}
	if len(repos) != len(want) {
		t.Fatalf("repos = %+v, want %+v", repos, want)
	}
	for i := range want {
		got := repos[i]
		if got.Name != want[i].Name || got.TargetPath != want[i].TargetPath || got.CloneURL != want[i].CloneURL {
			t.Errorf("repo[%d] = %+v, want %+v", i, got, want[i])
		}
	}
}

// A declared repository that is not cloned yet stays in the plan, so status
// reports it rather than dropping it; without a url there is nothing to
// name, and the workspace is reported and skipped.
func TestGitWorkspaceMissingPath(t *testing.T) {
	configDir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "not-cloned")

	repos, err := loadGitWorkspaceConfig(t, configDir, "workspaces:\n"+
		"  engine:\n    type: git\n    path: "+missing+"\n    url: git@example.com:team/engine.git\n")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "engine" || repos[0].TargetPath != missing {
		t.Fatalf("repos = %+v, want the declared engine at %s", repos, missing)
	}

	_, err = loadGitWorkspaceConfig(t, configDir, "workspaces:\n"+
		"  engine:\n    type: git\n    path: "+missing+"\n")
	if err == nil || !strings.Contains(err.Error(), "no git repositories found in workspaces") {
		t.Fatalf("Load without url = %v, want the empty-plan error", err)
	}
	if _, err := loadGitWorkspace("engine", &workspaceEntry{}, missing, configDir); err == nil {
		t.Fatal("loadGitWorkspace without url on a missing path = nil error, want it reported")
	}
}

// A path that cannot be read is reported, not taken for an empty one.
func TestGitWorkspaceUnreadablePathIsReported(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode 000 directory")
	}
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	ws := &workspaceEntry{URL: "git@example.com:team/engine.git"}
	repos, err := loadGitWorkspace("engine", ws, filepath.Join(locked, "engine"), t.TempDir())
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("loadGitWorkspace = %+v, %v; want a permission error", repos, err)
	}
}

// Sync plans no type: git workspace without a url, so status keeps reading
// such a path as a parent directory, as it always has.
func TestGitWorkspaceWithoutURLKeepsScan(t *testing.T) {
	configDir := t.TempDir()
	clone := filepath.Join(t.TempDir(), "devbox")
	mkGitDir(t, clone)
	mkGitDir(t, filepath.Join(clone, "child"))

	repos, err := loadGitWorkspaceConfig(t, configDir, "workspaces:\n"+
		"  devbox:\n    type: git\n    path: "+clone+"\n")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "child" {
		t.Fatalf("repos = %+v, want the scanned child", repos)
	}
}

// A directory that is not a repository keeps the parent-directory scan, and
// so does the config directory itself even when it is a checkout, however
// its path is spelled: a devbox whose `path: .` workspace lists its member
// clones must not collapse into the devbox repository.
func TestGitWorkspaceKeepsParentDirectoryScan(t *testing.T) {
	configDir := t.TempDir()
	mkGitDir(t, configDir)
	mkGitDir(t, filepath.Join(configDir, "member"))
	parent := filepath.Join(t.TempDir(), "repos")
	mkGitDir(t, filepath.Join(parent, "alpha"))
	mkGitDir(t, filepath.Join(parent, "beta"))
	link := filepath.Join(t.TempDir(), "devbox-link")
	if err := os.Symlink(configDir, link); err != nil {
		t.Fatal(err)
	}

	repos, err := loadGitWorkspaceConfig(t, configDir, "workspaces:\n"+
		"  self:\n    path: .\n"+
		"  parent:\n    type: git\n    path: "+parent+"\n    url: git@example.com:team/repos.git\n")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	names := make([]string, 0, len(repos))
	for _, r := range repos {
		names = append(names, r.Name)
	}
	if got := strings.Join(names, ","); got != "alpha,beta,member" {
		t.Fatalf("repo names = %s, want alpha,beta,member", got)
	}

	// Loaded alone, so dedup by target path does not hide a collapse.
	repos, err = loadGitWorkspaceConfig(t, configDir, "workspaces:\n"+
		"  linked:\n    type: git\n    path: "+link+"\n    url: git@example.com:team/devbox.git\n")
	if err != nil {
		t.Fatalf("Load through symlink: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "member" {
		t.Fatalf("repos through symlink = %+v, want the scanned member", repos)
	}
}
