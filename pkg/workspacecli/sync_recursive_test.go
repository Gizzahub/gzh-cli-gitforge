// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func TestScanForChildConfigs(t *testing.T) {
	// Create temp directory structure with child configs
	tmpDir := t.TempDir()

	// repo-a: existing with .gz-git.yaml
	repoA := filepath.Join(tmpDir, "repo-a")
	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoA, DefaultConfigFile), []byte("repositories: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// repo-b: existing without .gz-git.yaml
	repoB := filepath.Join(tmpDir, "repo-b")
	if err := os.MkdirAll(repoB, 0o755); err != nil {
		t.Fatal(err)
	}

	// repo-c: does not exist (will be cloned)
	repoC := filepath.Join(tmpDir, "repo-c")

	actions := []reposync.Action{
		{Repo: reposync.RepoSpec{Name: "repo-a", TargetPath: repoA}, Type: reposync.ActionUpdate},
		{Repo: reposync.RepoSpec{Name: "repo-b", TargetPath: repoB}, Type: reposync.ActionUpdate},
		{Repo: reposync.RepoSpec{Name: "repo-c", TargetPath: repoC}, Type: reposync.ActionClone},
	}

	var buf bytes.Buffer
	opts := recursiveSyncOpts{
		MaxDepth: 3,
		Visited:  map[string]bool{tmpDir: true},
		Out:      &buf,
		Depth:    0,
		DryRun:   true,
	}

	scanForChildConfigs(&buf, actions, opts)
	output := buf.String()

	// Should detect repo-a as having child config
	if !bytes.Contains([]byte(output), []byte("repo-a")) {
		t.Errorf("expected repo-a in output, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte(DefaultConfigFile)) {
		t.Errorf("expected config file name in output, got:\n%s", output)
	}

	// Should show repo-c as pending (will check after clone)
	if !bytes.Contains([]byte(output), []byte("repo-c")) {
		t.Errorf("expected repo-c in output, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("will check after clone")) {
		t.Errorf("expected 'will check after clone' in output, got:\n%s", output)
	}

	// repo-b should NOT appear (exists but no config)
	if bytes.Contains([]byte(output), []byte("repo-b")) {
		t.Errorf("repo-b should not appear in output (no child config), got:\n%s", output)
	}
}

func TestGetConfigWorkspaces(t *testing.T) {
	cfg := &config.Config{
		Workspaces: map[string]*config.Workspace{
			"mywork": {
				Path: "~/mywork",
				Sync: &config.SyncConfig{Recursive: true},
			},
			"mydevbox": {
				Path:   "~/mydevbox",
				Source: &config.ForgeSource{Provider: "gitlab", Org: "devbox"},
			},
			"mynote": {
				Path: "~/mynote",
				Sync: &config.SyncConfig{Strategy: "pull"},
				// Recursive NOT set
			},
		},
	}

	result := config.GetConfigWorkspaces(cfg)

	if len(result) != 1 {
		t.Fatalf("expected 1 config workspace, got %d", len(result))
	}
	if _, ok := result["mywork"]; !ok {
		t.Error("expected 'mywork' in config workspaces")
	}
	if _, ok := result["mydevbox"]; ok {
		t.Error("forge workspace 'mydevbox' should not be in config workspaces")
	}
	if _, ok := result["mynote"]; ok {
		t.Error("non-recursive 'mynote' should not be in config workspaces")
	}
}

func TestPlanConfigWorkspaces(t *testing.T) {
	tmpDir := t.TempDir()

	// Create child workspace directory with config
	wsDir := filepath.Join(tmpDir, "mywork")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write child config with a repository
	childConfig := `strategy: pull
parallel: 4
repositories:
  - name: my-repo
    url: https://github.com/user/my-repo.git
`
	if err := os.WriteFile(filepath.Join(wsDir, DefaultConfigFile), []byte(childConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		ConfigPath: filepath.Join(tmpDir, ".gz-git.yaml"),
		Workspaces: map[string]*config.Workspace{
			"mywork": {
				Path: wsDir,
				Sync: &config.SyncConfig{Recursive: true},
			},
		},
	}

	var buf bytes.Buffer
	actions, err := planConfigWorkspaces(context.Background(), cfg, tmpDir, &buf, "")
	if err != nil {
		t.Fatalf("planConfigWorkspaces failed: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	if actions[0].Repo.Name != "my-repo" {
		t.Errorf("expected repo name 'my-repo', got %q", actions[0].Repo.Name)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Planning config workspace")) {
		t.Errorf("expected planning message in output, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("1 repositories")) {
		t.Errorf("expected '1 repositories' in output, got:\n%s", output)
	}
}

func TestRecursiveDepthLimit(t *testing.T) {
	var buf bytes.Buffer

	// Simulate being at max depth already
	result := reposync.ExecutionResult{
		Succeeded: []reposync.ActionResult{
			{Action: reposync.Action{Repo: reposync.RepoSpec{Name: "repo-a", TargetPath: "/some/path"}}},
		},
	}

	opts := recursiveSyncOpts{
		MaxDepth: 2,
		Visited:  map[string]bool{},
		Out:      &buf,
		Depth:    2, // already at max depth
	}

	syncChildWorkspaces(context.Background(), result, opts)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("Max depth 2 reached")) {
		t.Errorf("expected max depth message, got:\n%s", output)
	}
}

func TestRecursiveCircularReference(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a repo directory with config that points back to itself
	repoA := filepath.Join(tmpDir, "repo-a")
	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoA, DefaultConfigFile), []byte("repositories: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	result := reposync.ExecutionResult{
		Succeeded: []reposync.ActionResult{
			{Action: reposync.Action{Repo: reposync.RepoSpec{Name: "repo-a", TargetPath: repoA}}},
		},
	}

	// Mark repoA as already visited (circular reference)
	absRepoA, _ := filepath.Abs(repoA)
	opts := recursiveSyncOpts{
		MaxDepth: 3,
		Visited:  map[string]bool{absRepoA: true},
		Out:      &buf,
		Depth:    0,
	}

	syncChildWorkspaces(context.Background(), result, opts)
	output := buf.String()

	// Should NOT find any child workspaces (repoA is already visited)
	if bytes.Contains([]byte(output), []byte("Found")) {
		t.Errorf("should not find child workspaces when already visited, got:\n%s", output)
	}
}

func TestSyncChildWorkspaces_EmptyResult(t *testing.T) {
	var buf bytes.Buffer

	// Empty execution result
	result := reposync.ExecutionResult{}

	opts := recursiveSyncOpts{
		MaxDepth: 3,
		Visited:  map[string]bool{},
		Out:      &buf,
		Depth:    0,
	}

	syncChildWorkspaces(context.Background(), result, opts)
	output := buf.String()

	// Should produce no output for empty result
	if output != "" {
		t.Errorf("expected empty output for empty result, got:\n%s", output)
	}
}

func TestScanForChildConfigs_AtMaxDepth(t *testing.T) {
	tmpDir := t.TempDir()

	repoA := filepath.Join(tmpDir, "repo-a")
	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoA, DefaultConfigFile), []byte("repositories: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	actions := []reposync.Action{
		{Repo: reposync.RepoSpec{Name: "repo-a", TargetPath: repoA}, Type: reposync.ActionUpdate},
	}

	var buf bytes.Buffer
	opts := recursiveSyncOpts{
		MaxDepth: 1,
		Visited:  map[string]bool{},
		Out:      &buf,
		Depth:    0, // depth+1 == 1 == MaxDepth, so scan should skip
	}

	scanForChildConfigs(&buf, actions, opts)
	output := buf.String()

	// Should produce no output since depth+1 >= MaxDepth
	if output != "" {
		t.Errorf("expected empty output at max depth, got:\n%s", output)
	}
}

func TestEnsureChildConfigs_ConfigLink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source config file (simulates workstation/gz-git/mywork-gz-git.yaml)
	configDir := filepath.Join(tmpDir, "workstation")
	gzGitDir := filepath.Join(configDir, "gz-git")
	if err := os.MkdirAll(gzGitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	srcConfig := filepath.Join(gzGitDir, "mywork-gz-git.yaml")
	if err := os.WriteFile(srcConfig, []byte("repositories: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Target workspace directory (simulates ~/mywork)
	wsDir := filepath.Join(tmpDir, "mywork")

	// Parent config path (for resolving relative configLink)
	parentConfigPath := filepath.Join(configDir, ".gz-git.yaml")

	cfg := &config.Config{
		ConfigPath: parentConfigPath,
		Workspaces: map[string]*config.Workspace{
			"mywork": {
				Path:       wsDir,
				ConfigLink: "./gz-git/mywork-gz-git.yaml",
			},
		},
	}

	var buf bytes.Buffer
	err := ensureChildConfigs(&buf, cfg)
	if err != nil {
		t.Fatalf("ensureChildConfigs failed: %v", err)
	}

	output := buf.String()

	// Should mention linking
	if !bytes.Contains([]byte(output), []byte("Linking workspace")) {
		t.Errorf("expected 'Linking workspace' in output, got:\n%s", output)
	}

	// Verify symlink was created
	linkPath := filepath.Join(wsDir, DefaultConfigFile)
	fi, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file")
	}

	// Verify symlink target resolves correctly
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}

	// Resolve to absolute for comparison
	absTarget, _ := filepath.Abs(target)
	absSrc, _ := filepath.Abs(srcConfig)
	if absTarget != absSrc {
		t.Errorf("symlink target = %s, want %s", absTarget, absSrc)
	}
}

func TestScanForChildConfigs_SkipsVisited(t *testing.T) {
	tmpDir := t.TempDir()

	repoA := filepath.Join(tmpDir, "repo-a")
	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoA, DefaultConfigFile), []byte("repositories: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	absRepoA, _ := filepath.Abs(repoA)
	actions := []reposync.Action{
		{Repo: reposync.RepoSpec{Name: "repo-a", TargetPath: repoA}, Type: reposync.ActionUpdate},
	}

	var buf bytes.Buffer
	opts := recursiveSyncOpts{
		MaxDepth: 3,
		Visited:  map[string]bool{absRepoA: true},
		Out:      &buf,
		Depth:    0,
	}

	scanForChildConfigs(&buf, actions, opts)
	output := buf.String()

	// Should produce no output since repo-a is already visited
	if output != "" {
		t.Errorf("expected empty output for visited repo, got:\n%s", output)
	}
}
