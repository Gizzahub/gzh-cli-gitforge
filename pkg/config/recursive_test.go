// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigRecursive(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create workstation config
	workstationConfig := `parallel: 10
cloneProto: ssh
workspaces:
  workspace:
    path: workspace
    type: config
    profile: opensource
    parallel: 10
  single-repo:
    path: single-repo
    type: git
    profile: personal
metadata:
  name: workstation
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git-config.yaml"), []byte(workstationConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create workspace directory and config
	workspaceDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	workspaceConfig := `profile: opensource
sync:
  strategy: reset
workspaces:
  project1:
    path: project1
    type: git
  project2:
    path: project2
    type: config
    sync:
      strategy: pull
metadata:
  name: workspace
  type: development
`
	if err := os.WriteFile(filepath.Join(workspaceDir, ".gz-git.yaml"), []byte(workspaceConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create project1 (git repo)
	project1Dir := filepath.Join(workspaceDir, "project1")
	if err := os.MkdirAll(filepath.Join(project1Dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create project2 directory and config
	project2Dir := filepath.Join(workspaceDir, "project2")
	if err := os.MkdirAll(project2Dir, 0o755); err != nil {
		t.Fatal(err)
	}

	project2Config := `sync:
  strategy: pull
workspaces:
  vendor:
    path: vendor
    type: git
    sync:
      strategy: skip
metadata:
  name: project2
`
	if err := os.WriteFile(filepath.Join(project2Dir, ".gz-git.yaml"), []byte(project2Config), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create vendor (git repo)
	vendorDir := filepath.Join(project2Dir, "vendor")
	if err := os.MkdirAll(filepath.Join(vendorDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create single-repo (git repo)
	singleRepoDir := filepath.Join(tmpDir, "single-repo")
	if err := os.MkdirAll(filepath.Join(singleRepoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Test loading workstation config
	config, err := LoadConfigRecursive(tmpDir, ".gz-git-config.yaml")
	if err != nil {
		t.Fatalf("LoadConfigRecursive failed: %v", err)
	}

	// Verify workstation config
	if config.Parallel != 10 {
		t.Errorf("Expected parallel=10, got %d", config.Parallel)
	}
	if config.CloneProto != "ssh" {
		t.Errorf("Expected cloneProto=ssh, got %s", config.CloneProto)
	}
	if config.Metadata == nil || config.Metadata.Name != "workstation" {
		t.Error("Expected metadata.name=workstation")
	}

	// Verify workspaces
	if len(config.Workspaces) != 2 {
		t.Fatalf("Expected 2 workspaces, got %d", len(config.Workspaces))
	}

	// Verify workspace workspace
	workspaceWs := config.Workspaces["workspace"]
	if workspaceWs == nil {
		t.Fatal("Expected 'workspace' workspace to exist")
	}
	if workspaceWs.Path != "workspace" {
		t.Errorf("Expected path=workspace, got %s", workspaceWs.Path)
	}
	if workspaceWs.Type != WorkspaceTypeConfig {
		t.Errorf("Expected type=config, got %s", workspaceWs.Type)
	}
	if workspaceWs.Profile != "opensource" {
		t.Errorf("Expected profile=opensource, got %s", workspaceWs.Profile)
	}
	if workspaceWs.Parallel != 10 {
		t.Errorf("Expected parallel=10, got %d", workspaceWs.Parallel)
	}

	// Verify single-repo workspace
	singleRepoWs := config.Workspaces["single-repo"]
	if singleRepoWs == nil {
		t.Fatal("Expected 'single-repo' workspace to exist")
	}
	if singleRepoWs.Path != "single-repo" {
		t.Errorf("Expected path=single-repo, got %s", singleRepoWs.Path)
	}
	if singleRepoWs.Type != WorkspaceTypeGit {
		t.Errorf("Expected type=git, got %s", singleRepoWs.Type)
	}
}

func TestLoadConfigRecursive_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := LoadConfigRecursive(tmpDir, ".gz-git-config.yaml")
	if err == nil {
		t.Error("Expected error for missing config file")
	}
}

func TestLoadConfigRecursive_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid YAML
	invalidYAML := `parallel: 10
workspaces:
  foo:
    path: foo
    type: config
    invalid yaml here!!!
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git-config.yaml"), []byte(invalidYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigRecursive(tmpDir, ".gz-git-config.yaml")
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoadConfigRecursive_InvalidWorkspaceType(t *testing.T) {
	tmpDir := t.TempDir()

	// Write config with invalid workspace type
	invalidConfig := `parallel: 10
workspaces:
  foo:
    path: foo
    type: invalid
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git-config.yaml"), []byte(invalidConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// LoadConfigRecursive should succeed - type validation is done separately
	config, err := LoadConfigRecursive(tmpDir, ".gz-git-config.yaml")
	if err != nil {
		t.Fatalf("LoadConfigRecursive failed: %v", err)
	}

	// Verify the invalid type is preserved
	ws := config.Workspaces["foo"]
	if ws == nil {
		t.Fatal("Expected 'foo' workspace to exist")
	}
	if ws.Type != "invalid" {
		t.Errorf("Expected type=invalid, got %s", ws.Type)
	}
}

func TestResolvePath(t *testing.T) {
	tests := []struct {
		name       string
		parentPath string
		childPath  string
		want       string
	}{
		{
			name:       "relative path",
			parentPath: "/home/user/workspace",
			childPath:  "project",
			want:       "/home/user/workspace/project",
		},
		{
			name:       "relative path with dot",
			parentPath: "/home/user/workspace",
			childPath:  "./project",
			want:       "/home/user/workspace/project",
		},
		{
			name:       "absolute path",
			parentPath: "/home/user/workspace",
			childPath:  "/home/user/other",
			want:       "/home/user/other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolvePath(tt.parentPath, tt.childPath)
			if err != nil {
				t.Fatalf("resolvePath failed: %v", err)
			}

			if got != tt.want {
				t.Errorf("resolvePath() = %v, want %v", got, tt.want)
			}
		})
	}

	// Test home-relative path separately (requires HOME env var)
	t.Run("home-relative path", func(t *testing.T) {
		// Set HOME for this test
		home := t.TempDir()
		t.Setenv("HOME", home)

		got, err := resolvePath("/home/user/workspace", "~/mydevbox")
		if err != nil {
			t.Fatalf("resolvePath failed: %v", err)
		}

		expected := filepath.Join(home, "mydevbox")
		if got != expected {
			t.Errorf("resolvePath() = %v, want %v", got, expected)
		}
	})
}

func TestMergeWorkspaceOverrides(t *testing.T) {
	config := &Config{
		Profile:  "original",
		Parallel: 10,
		Sync: &SyncConfig{
			Strategy: "reset",
		},
	}

	ws := &Workspace{
		Profile:  "override",
		Parallel: 20,
		Sync: &SyncConfig{
			Strategy: "pull",
		},
	}

	mergeWorkspaceOverrides(config, ws)

	if config.Profile != "override" {
		t.Errorf("Expected profile=override, got %s", config.Profile)
	}
	if config.Parallel != 20 {
		t.Errorf("Expected parallel=20, got %d", config.Parallel)
	}
	if config.Sync.Strategy != "pull" {
		t.Errorf("Expected sync.strategy=pull, got %s", config.Sync.Strategy)
	}
}

func TestLoadWorkspaces_ExplicitMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some git repos
	repo1 := filepath.Join(tmpDir, "repo1")
	if err := os.MkdirAll(filepath.Join(repo1, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	repo2 := filepath.Join(tmpDir, "repo2")
	if err := os.MkdirAll(filepath.Join(repo2, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Config with explicit workspaces
	config := &Config{
		Workspaces: map[string]*Workspace{
			"repo1": {Path: "repo1", Type: WorkspaceTypeGit},
		},
	}

	// Explicit mode should NOT auto-discover repo2
	err := LoadWorkspaces(tmpDir, config, ExplicitMode)
	if err != nil {
		t.Fatalf("LoadWorkspaces failed: %v", err)
	}

	if len(config.Workspaces) != 1 {
		t.Errorf("Expected 1 workspace (explicit), got %d", len(config.Workspaces))
	}
}

func TestLoadWorkspaces_AutoMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some git repos
	repo1 := filepath.Join(tmpDir, "repo1")
	if err := os.MkdirAll(filepath.Join(repo1, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	repo2 := filepath.Join(tmpDir, "repo2")
	if err := os.MkdirAll(filepath.Join(repo2, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Config with explicit workspaces (will be cleared in auto mode)
	config := &Config{
		Workspaces: map[string]*Workspace{
			"repo1": {Path: "repo1", Type: WorkspaceTypeGit},
		},
	}

	// Auto mode should discover both repos
	err := LoadWorkspaces(tmpDir, config, AutoMode)
	if err != nil {
		t.Fatalf("LoadWorkspaces failed: %v", err)
	}

	if len(config.Workspaces) != 2 {
		t.Errorf("Expected 2 workspaces (auto-discovered), got %d", len(config.Workspaces))
	}
}

func TestLoadWorkspaces_HybridMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some git repos
	repo1 := filepath.Join(tmpDir, "repo1")
	if err := os.MkdirAll(filepath.Join(repo1, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	repo2 := filepath.Join(tmpDir, "repo2")
	if err := os.MkdirAll(filepath.Join(repo2, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Test 1: Config with explicit workspaces - should use explicit
	config1 := &Config{
		Workspaces: map[string]*Workspace{
			"repo1": {Path: "repo1", Type: WorkspaceTypeGit},
		},
	}

	err := LoadWorkspaces(tmpDir, config1, HybridMode)
	if err != nil {
		t.Fatalf("LoadWorkspaces failed: %v", err)
	}

	if len(config1.Workspaces) != 1 {
		t.Errorf("Expected 1 workspace (explicit), got %d", len(config1.Workspaces))
	}

	// Test 2: Config without workspaces - should auto-discover
	config2 := &Config{
		Workspaces: map[string]*Workspace{},
	}

	err = LoadWorkspaces(tmpDir, config2, HybridMode)
	if err != nil {
		t.Fatalf("LoadWorkspaces failed: %v", err)
	}

	if len(config2.Workspaces) != 2 {
		t.Errorf("Expected 2 workspaces (auto-discovered), got %d", len(config2.Workspaces))
	}
}

func TestFindConfigRecursive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure
	// tmpDir/
	//   .gz-git-config.yaml
	//   workspace/
	//     .gz-git.yaml
	//     project/
	//       (no config)

	// Write root config
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git-config.yaml"), []byte("parallel: 10"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create workspace with config
	workspaceDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, ".gz-git.yaml"), []byte("profile: test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create project without config
	projectDir := filepath.Join(workspaceDir, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Test 1: Find .gz-git.yaml from project dir (should find workspace config)
	found, err := FindConfigRecursive(projectDir, ".gz-git.yaml")
	if err != nil {
		t.Fatalf("FindConfigRecursive failed: %v", err)
	}
	if found != workspaceDir {
		t.Errorf("Expected %s, got %s", workspaceDir, found)
	}

	// Test 2: Find .gz-git-config.yaml from project dir (should find root config)
	found, err = FindConfigRecursive(projectDir, ".gz-git-config.yaml")
	if err != nil {
		t.Fatalf("FindConfigRecursive failed: %v", err)
	}
	if found != tmpDir {
		t.Errorf("Expected %s, got %s", tmpDir, found)
	}

	// Test 3: Find nonexistent config (should fail)
	_, err = FindConfigRecursive(projectDir, ".nonexistent.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent config")
	}
}

func TestWorkspaceType_IsValid(t *testing.T) {
	tests := []struct {
		wsType WorkspaceType
		want   bool
	}{
		{WorkspaceTypeConfig, true},
		{WorkspaceTypeGit, true},
		{WorkspaceTypeForge, true},
		{"", true}, // Empty is valid (inferred from context)
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.wsType), func(t *testing.T) {
			got := tt.wsType.IsValid()
			if got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkspaceType_Resolve(t *testing.T) {
	tests := []struct {
		wsType    WorkspaceType
		hasSource bool
		want      WorkspaceType
	}{
		{WorkspaceTypeConfig, false, WorkspaceTypeConfig},
		{WorkspaceTypeGit, false, WorkspaceTypeGit},
		{WorkspaceTypeForge, false, WorkspaceTypeForge},
		{"", true, WorkspaceTypeForge},  // Empty + hasSource = forge
		{"", false, WorkspaceTypeGit},   // Empty + no source = git
	}

	for _, tt := range tests {
		t.Run(string(tt.wsType), func(t *testing.T) {
			got := tt.wsType.Resolve(tt.hasSource)
			if got != tt.want {
				t.Errorf("Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscoveryMode_IsValid(t *testing.T) {
	tests := []struct {
		mode DiscoveryMode
		want bool
	}{
		{ExplicitMode, true},
		{AutoMode, true},
		{HybridMode, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			got := tt.mode.IsValid()
			if got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscoveryMode_Default(t *testing.T) {
	tests := []struct {
		mode DiscoveryMode
		want DiscoveryMode
	}{
		{ExplicitMode, ExplicitMode},
		{AutoMode, AutoMode},
		{HybridMode, HybridMode},
		{"", HybridMode}, // Empty string defaults to hybrid
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			got := tt.mode.Default()
			if got != tt.want {
				t.Errorf("Default() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetWorkspaceByName(t *testing.T) {
	config := &Config{
		Workspaces: map[string]*Workspace{
			"mydevbox": {Path: "~/mydevbox", Type: WorkspaceTypeConfig},
			"mywork":   {Path: "~/mywork", Type: WorkspaceTypeConfig},
		},
	}

	// Test existing workspace
	ws := GetWorkspaceByName(config, "mydevbox")
	if ws == nil {
		t.Fatal("Expected workspace to exist")
	}
	if ws.Path != "~/mydevbox" {
		t.Errorf("Expected path=~/mydevbox, got %s", ws.Path)
	}

	// Test non-existing workspace
	ws = GetWorkspaceByName(config, "nonexistent")
	if ws != nil {
		t.Error("Expected nil for non-existing workspace")
	}

	// Test nil config
	ws = GetWorkspaceByName(nil, "mydevbox")
	if ws != nil {
		t.Error("Expected nil for nil config")
	}
}

func TestGetAllWorkspaces(t *testing.T) {
	config := &Config{
		Workspaces: map[string]*Workspace{
			"mydevbox": {Path: "~/mydevbox", Type: WorkspaceTypeConfig},
			"mywork":   {Path: "~/mywork", Type: WorkspaceTypeConfig},
		},
	}

	all := GetAllWorkspaces(config)
	if len(all) != 2 {
		t.Errorf("Expected 2 workspaces, got %d", len(all))
	}

	// Test nil config
	all = GetAllWorkspaces(nil)
	if all != nil {
		t.Error("Expected nil for nil config")
	}
}

func TestGetForgeWorkspaces(t *testing.T) {
	config := &Config{
		Workspaces: map[string]*Workspace{
			"devbox": {
				Path: "~/devbox",
				Type: WorkspaceTypeForge,
				Source: &ForgeSource{
					Provider: "gitlab",
					Org:      "devbox",
				},
			},
			"personal": {
				Path: "~/personal",
				Type: WorkspaceTypeGit,
			},
			"inferred-forge": {
				Path: "~/inferred",
				Source: &ForgeSource{
					Provider: "github",
					Org:      "myorg",
				},
			},
		},
	}

	forgeWs := GetForgeWorkspaces(config)
	if len(forgeWs) != 2 {
		t.Errorf("Expected 2 forge workspaces, got %d", len(forgeWs))
	}

	if _, ok := forgeWs["devbox"]; !ok {
		t.Error("Expected 'devbox' to be in forge workspaces")
	}
	if _, ok := forgeWs["inferred-forge"]; !ok {
		t.Error("Expected 'inferred-forge' to be in forge workspaces")
	}
	if _, ok := forgeWs["personal"]; ok {
		t.Error("Expected 'personal' NOT to be in forge workspaces")
	}

	// Test nil config
	forgeWs = GetForgeWorkspaces(nil)
	if len(forgeWs) != 0 {
		t.Error("Expected empty map for nil config")
	}
}
