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
children:
  - path: workspace
    type: config
    profile: opensource
    parallel: 10
  - path: single-repo
    type: git
    profile: personal
metadata:
  name: workstation
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git-config.yaml"), []byte(workstationConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Create workspace directory and config
	workspaceDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	workspaceConfig := `profile: opensource
sync:
  strategy: reset
  parallel: 10
children:
  - path: project1
    type: git
  - path: project2
    type: config
    sync:
      strategy: pull
metadata:
  name: workspace
  type: development
`
	if err := os.WriteFile(filepath.Join(workspaceDir, ".gz-git.yaml"), []byte(workspaceConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Create project1 (git repo)
	project1Dir := filepath.Join(workspaceDir, "project1")
	if err := os.MkdirAll(filepath.Join(project1Dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create project2 directory and config
	project2Dir := filepath.Join(workspaceDir, "project2")
	if err := os.MkdirAll(project2Dir, 0755); err != nil {
		t.Fatal(err)
	}

	project2Config := `sync:
  strategy: pull
  parallel: 3
children:
  - path: vendor
    type: git
    sync:
      strategy: skip
metadata:
  name: project2
`
	if err := os.WriteFile(filepath.Join(project2Dir, ".gz-git.yaml"), []byte(project2Config), 0644); err != nil {
		t.Fatal(err)
	}

	// Create vendor (git repo)
	vendorDir := filepath.Join(project2Dir, "vendor")
	if err := os.MkdirAll(filepath.Join(vendorDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create single-repo (git repo)
	singleRepoDir := filepath.Join(tmpDir, "single-repo")
	if err := os.MkdirAll(filepath.Join(singleRepoDir, ".git"), 0755); err != nil {
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

	// Verify children
	if len(config.Children) != 2 {
		t.Fatalf("Expected 2 children, got %d", len(config.Children))
	}

	// Verify workspace child
	workspaceChild := config.Children[0]
	if workspaceChild.Path != "workspace" {
		t.Errorf("Expected path=workspace, got %s", workspaceChild.Path)
	}
	if workspaceChild.Type != ChildTypeConfig {
		t.Errorf("Expected type=config, got %s", workspaceChild.Type)
	}
	if workspaceChild.Profile != "opensource" {
		t.Errorf("Expected profile=opensource, got %s", workspaceChild.Profile)
	}
	if workspaceChild.Parallel != 10 {
		t.Errorf("Expected parallel=10, got %d", workspaceChild.Parallel)
	}

	// Verify single-repo child
	singleRepoChild := config.Children[1]
	if singleRepoChild.Path != "single-repo" {
		t.Errorf("Expected path=single-repo, got %s", singleRepoChild.Path)
	}
	if singleRepoChild.Type != ChildTypeGit {
		t.Errorf("Expected type=git, got %s", singleRepoChild.Type)
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
children:
  - path: foo
    type: config
    invalid yaml here!!!
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git-config.yaml"), []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigRecursive(tmpDir, ".gz-git-config.yaml")
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoadConfigRecursive_InvalidChildType(t *testing.T) {
	tmpDir := t.TempDir()

	// Write config with invalid child type
	invalidConfig := `parallel: 10
children:
  - path: foo
    type: invalid
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git-config.yaml"), []byte(invalidConfig), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigRecursive(tmpDir, ".gz-git-config.yaml")
	if err == nil {
		t.Error("Expected error for invalid child type")
	}
}

func TestLoadConfigRecursive_GitRepoNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Write config with git child that doesn't exist
	config := `parallel: 10
children:
  - path: nonexistent-repo
    type: git
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git-config.yaml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigRecursive(tmpDir, ".gz-git-config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent git repo")
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

func TestMergeInlineOverrides(t *testing.T) {
	config := &Config{
		Profile:  "original",
		Parallel: 10,
		Sync: &SyncConfig{
			Strategy: "reset",
		},
	}

	entry := &ChildEntry{
		Profile:  "override",
		Parallel: 10,
		Sync: &SyncConfig{
			Strategy: "pull",
		},
	}

	mergeInlineOverrides(config, entry)

	if config.Profile != "override" {
		t.Errorf("Expected profile=override, got %s", config.Profile)
	}
	if config.Parallel != 10 {
		t.Errorf("Expected parallel=10, got %d", config.Parallel)
	}
	if config.Sync.Strategy != "pull" {
		t.Errorf("Expected sync.strategy=pull, got %s", config.Sync.Strategy)
	}
}

func TestLoadChildren_ExplicitMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some git repos
	repo1 := filepath.Join(tmpDir, "repo1")
	if err := os.MkdirAll(filepath.Join(repo1, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	repo2 := filepath.Join(tmpDir, "repo2")
	if err := os.MkdirAll(filepath.Join(repo2, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Config with explicit children
	config := &Config{
		Children: []ChildEntry{
			{Path: "repo1", Type: ChildTypeGit},
		},
	}

	// Explicit mode should NOT auto-discover repo2
	err := LoadChildren(tmpDir, config, ExplicitMode)
	if err != nil {
		t.Fatalf("LoadChildren failed: %v", err)
	}

	if len(config.Children) != 1 {
		t.Errorf("Expected 1 child (explicit), got %d", len(config.Children))
	}
}

func TestLoadChildren_AutoMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some git repos
	repo1 := filepath.Join(tmpDir, "repo1")
	if err := os.MkdirAll(filepath.Join(repo1, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	repo2 := filepath.Join(tmpDir, "repo2")
	if err := os.MkdirAll(filepath.Join(repo2, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Config with explicit children (will be ignored in auto mode)
	config := &Config{
		Children: []ChildEntry{
			{Path: "repo1", Type: ChildTypeGit},
		},
	}

	// Auto mode should discover both repos
	err := LoadChildren(tmpDir, config, AutoMode)
	if err != nil {
		t.Fatalf("LoadChildren failed: %v", err)
	}

	if len(config.Children) != 2 {
		t.Errorf("Expected 2 children (auto-discovered), got %d", len(config.Children))
	}
}

func TestLoadChildren_HybridMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some git repos
	repo1 := filepath.Join(tmpDir, "repo1")
	if err := os.MkdirAll(filepath.Join(repo1, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	repo2 := filepath.Join(tmpDir, "repo2")
	if err := os.MkdirAll(filepath.Join(repo2, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Test 1: Config with explicit children - should use explicit
	config1 := &Config{
		Children: []ChildEntry{
			{Path: "repo1", Type: ChildTypeGit},
		},
	}

	err := LoadChildren(tmpDir, config1, HybridMode)
	if err != nil {
		t.Fatalf("LoadChildren failed: %v", err)
	}

	if len(config1.Children) != 1 {
		t.Errorf("Expected 1 child (explicit), got %d", len(config1.Children))
	}

	// Test 2: Config without children - should auto-discover
	config2 := &Config{
		Children: []ChildEntry{},
	}

	err = LoadChildren(tmpDir, config2, HybridMode)
	if err != nil {
		t.Fatalf("LoadChildren failed: %v", err)
	}

	if len(config2.Children) != 2 {
		t.Errorf("Expected 2 children (auto-discovered), got %d", len(config2.Children))
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
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git-config.yaml"), []byte("parallel: 10"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create workspace with config
	workspaceDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, ".gz-git.yaml"), []byte("profile: test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create project without config
	projectDir := filepath.Join(workspaceDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
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

func TestChildType_DefaultConfigFile(t *testing.T) {
	tests := []struct {
		childType ChildType
		want      string
	}{
		{ChildTypeConfig, ".gz-git.yaml"},
		{ChildTypeGit, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.childType), func(t *testing.T) {
			got := tt.childType.DefaultConfigFile()
			if got != tt.want {
				t.Errorf("DefaultConfigFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChildType_IsValid(t *testing.T) {
	tests := []struct {
		childType ChildType
		want      bool
	}{
		{ChildTypeConfig, true},
		{ChildTypeGit, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.childType), func(t *testing.T) {
			got := tt.childType.IsValid()
			if got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
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
