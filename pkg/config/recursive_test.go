// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigRecursive(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create workstation config
	workstationConfig := `defaults:
  clone:
    proto: ssh
  sync:
    parallel: 10
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
	if config.GetParallel() != 10 {
		t.Errorf("Expected parallel=10, got %d", config.GetParallel())
	}
	if config.GetCloneProto() != "ssh" {
		t.Errorf("Expected cloneProto=ssh, got %s", config.GetCloneProto())
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
	// Path should be expanded to absolute path
	expectedPath := filepath.Join(tmpDir, "workspace")
	if workspaceWs.Path != expectedPath {
		t.Errorf("Expected path=%s, got %s", expectedPath, workspaceWs.Path)
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
	// Path should be expanded to absolute path
	expectedSingleRepoPath := filepath.Join(tmpDir, "single-repo")
	if singleRepoWs.Path != expectedSingleRepoPath {
		t.Errorf("Expected path=%s, got %s", expectedSingleRepoPath, singleRepoWs.Path)
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
		Profile: "original",
		Defaults: &DefaultsConfig{
			Sync: &SyncDefaults{
				Parallel: 10,
			},
		},
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
	if config.GetParallel() != 20 {
		t.Errorf("Expected parallel=20, got %d", config.GetParallel())
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
		{"", true, WorkspaceTypeForge}, // Empty + hasSource = forge
		{"", false, WorkspaceTypeGit},  // Empty + no source = git
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

func TestGetGitWorkspaces(t *testing.T) {
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
				URL:  "git@github.com:user/personal.git",
				AdditionalRemotes: map[string]string{
					"upstream": "https://github.com/original/personal.git",
				},
			},
			"no-url-git": {
				Path: "~/no-url",
				Type: WorkspaceTypeGit,
				// No URL - should NOT be included
			},
			"inferred-git": {
				Path: "~/inferred-git",
				URL:  "https://github.com/user/inferred.git",
				// No Source, no explicit Type → inferred as git
			},
		},
	}

	gitWs := GetGitWorkspaces(config)
	if len(gitWs) != 2 {
		t.Errorf("Expected 2 git workspaces, got %d", len(gitWs))
	}

	if _, ok := gitWs["personal"]; !ok {
		t.Error("Expected 'personal' to be in git workspaces")
	}
	if _, ok := gitWs["inferred-git"]; !ok {
		t.Error("Expected 'inferred-git' to be in git workspaces")
	}
	if _, ok := gitWs["devbox"]; ok {
		t.Error("Expected 'devbox' NOT to be in git workspaces")
	}
	if _, ok := gitWs["no-url-git"]; ok {
		t.Error("Expected 'no-url-git' NOT to be in git workspaces (no URL)")
	}

	// Verify additionalRemotes
	personalWs := gitWs["personal"]
	if len(personalWs.AdditionalRemotes) != 1 {
		t.Errorf("Expected 1 additional remote, got %d", len(personalWs.AdditionalRemotes))
	}
	if personalWs.AdditionalRemotes["upstream"] != "https://github.com/original/personal.git" {
		t.Error("Additional remote 'upstream' has wrong URL")
	}

	// Test nil config
	gitWs = GetGitWorkspaces(nil)
	if len(gitWs) != 0 {
		t.Error("Expected empty map for nil config")
	}
}

// ================================================================================
// Parent Config Reference Tests
// ================================================================================

func TestLoadConfigRecursive_WithParent(t *testing.T) {
	// Create directory structure:
	// tmpDir/
	//   workstation/
	//     .gz-git.yaml (parent config with profiles)
	//   mydevbox/
	//     .gz-git.yaml (child config referencing parent)
	tmpDir := t.TempDir()

	// Create workstation config (parent)
	workstationDir := filepath.Join(tmpDir, "workstation")
	if err := os.MkdirAll(workstationDir, 0o755); err != nil {
		t.Fatal(err)
	}

	workstationConfig := `defaults:
  clone:
    proto: ssh
  sync:
    parallel: 10
provider: gitlab
baseURL: https://gitlab.example.com

profiles:
  polypia:
    name: polypia
    provider: gitlab
    baseURL: https://gitlab.polypia.net
    cloneProto: ssh
  github-personal:
    name: github-personal
    provider: github
`
	if err := os.WriteFile(filepath.Join(workstationDir, ".gz-git.yaml"), []byte(workstationConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create mydevbox config (child referencing parent)
	mydevboxDir := filepath.Join(tmpDir, "mydevbox")
	if err := os.MkdirAll(mydevboxDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mydevboxConfig := `parent: ../workstation/.gz-git.yaml
profile: polypia
defaults:
  sync:
    parallel: 5
sync:
  strategy: pull
`
	if err := os.WriteFile(filepath.Join(mydevboxDir, ".gz-git.yaml"), []byte(mydevboxConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load mydevbox config
	config, err := LoadConfigRecursive(mydevboxDir, ".gz-git.yaml")
	if err != nil {
		t.Fatalf("LoadConfigRecursive failed: %v", err)
	}

	// Verify child config values
	if config.Profile != "polypia" {
		t.Errorf("Expected profile=polypia, got %s", config.Profile)
	}
	if config.GetParallel() != 5 {
		t.Errorf("Expected parallel=5 (child override), got %d", config.GetParallel())
	}
	if config.Sync == nil || config.Sync.Strategy != "pull" {
		t.Error("Expected sync.strategy=pull")
	}

	// Verify inherited values from parent
	if config.GetCloneProto() != "ssh" {
		t.Errorf("Expected cloneProto=ssh (from parent), got %s", config.GetCloneProto())
	}
	if config.Provider != "gitlab" {
		t.Errorf("Expected provider=gitlab (from parent), got %s", config.Provider)
	}
	if config.BaseURL != "https://gitlab.example.com" {
		t.Errorf("Expected baseURL from parent, got %s", config.BaseURL)
	}

	// Verify parent config is linked
	if config.ParentConfig == nil {
		t.Fatal("Expected ParentConfig to be set")
	}
	if config.ParentConfig.GetParallel() != 10 {
		t.Errorf("Expected parent parallel=10, got %d", config.ParentConfig.GetParallel())
	}

	// Verify profile lookup through chain
	profile := GetProfileFromChain(config, "polypia")
	if profile == nil {
		t.Fatal("Expected to find 'polypia' profile in parent chain")
	}
	if profile.Provider != "gitlab" {
		t.Errorf("Expected profile provider=gitlab, got %s", profile.Provider)
	}
	if profile.BaseURL != "https://gitlab.polypia.net" {
		t.Errorf("Expected profile baseURL=https://gitlab.polypia.net, got %s", profile.BaseURL)
	}

	// Verify github-personal profile is also accessible
	profile = GetProfileFromChain(config, "github-personal")
	if profile == nil {
		t.Fatal("Expected to find 'github-personal' profile in parent chain")
	}
	if profile.Provider != "github" {
		t.Errorf("Expected profile provider=github, got %s", profile.Provider)
	}
}

func TestLoadConfigRecursive_CircularReference(t *testing.T) {
	// Create directory structure with circular reference:
	// tmpDir/
	//   a/
	//     .gz-git.yaml (parent: ../b/.gz-git.yaml)
	//   b/
	//     .gz-git.yaml (parent: ../a/.gz-git.yaml)
	tmpDir := t.TempDir()

	// Create config A
	dirA := filepath.Join(tmpDir, "a")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	configA := `parent: ../b/.gz-git.yaml
profile: test-a
`
	if err := os.WriteFile(filepath.Join(dirA, ".gz-git.yaml"), []byte(configA), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create config B
	dirB := filepath.Join(tmpDir, "b")
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatal(err)
	}
	configB := `parent: ../a/.gz-git.yaml
profile: test-b
`
	if err := os.WriteFile(filepath.Join(dirB, ".gz-git.yaml"), []byte(configB), 0o644); err != nil {
		t.Fatal(err)
	}

	// Loading either config should fail with circular reference error
	_, err := LoadConfigRecursive(dirA, ".gz-git.yaml")
	if err == nil {
		t.Fatal("Expected error for circular reference")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("Expected circular reference error, got: %v", err)
	}
}

func TestLoadConfigRecursive_SelfReference(t *testing.T) {
	// Create config that references itself
	tmpDir := t.TempDir()

	configContent := `parent: .gz-git.yaml
profile: test
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigRecursive(tmpDir, ".gz-git.yaml")
	if err == nil {
		t.Fatal("Expected error for self-reference")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("Expected circular reference error, got: %v", err)
	}
}

func TestLoadConfigRecursive_ParentNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `parent: ../nonexistent/.gz-git.yaml
profile: test
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".gz-git.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigRecursive(tmpDir, ".gz-git.yaml")
	if err == nil {
		t.Fatal("Expected error for missing parent")
	}
	if !strings.Contains(err.Error(), "failed to load parent") {
		t.Errorf("Expected parent load error, got: %v", err)
	}
}

func TestLoadConfigRecursive_DeepParentChain(t *testing.T) {
	// Create 3-level parent chain:
	// level3 → level2 → level1
	tmpDir := t.TempDir()

	// Level 1 (root parent)
	level1Dir := filepath.Join(tmpDir, "level1")
	if err := os.MkdirAll(level1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	level1Config := `defaults:
  sync:
    parallel: 100
profiles:
  root-profile:
    name: root-profile
    provider: github
`
	if err := os.WriteFile(filepath.Join(level1Dir, ".gz-git.yaml"), []byte(level1Config), 0o644); err != nil {
		t.Fatal(err)
	}

	// Level 2 (middle)
	level2Dir := filepath.Join(tmpDir, "level2")
	if err := os.MkdirAll(level2Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	level2Config := `parent: ../level1/.gz-git.yaml
defaults:
  clone:
    proto: https
  sync:
    parallel: 50
profiles:
  middle-profile:
    name: middle-profile
    provider: gitlab
`
	if err := os.WriteFile(filepath.Join(level2Dir, ".gz-git.yaml"), []byte(level2Config), 0o644); err != nil {
		t.Fatal(err)
	}

	// Level 3 (leaf)
	level3Dir := filepath.Join(tmpDir, "level3")
	if err := os.MkdirAll(level3Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	level3Config := `parent: ../level2/.gz-git.yaml
profile: root-profile
defaults:
  sync:
    parallel: 10
`
	if err := os.WriteFile(filepath.Join(level3Dir, ".gz-git.yaml"), []byte(level3Config), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load level3 config
	config, err := LoadConfigRecursive(level3Dir, ".gz-git.yaml")
	if err != nil {
		t.Fatalf("LoadConfigRecursive failed: %v", err)
	}

	// Verify level3 values
	if config.GetParallel() != 10 {
		t.Errorf("Expected parallel=10 (level3), got %d", config.GetParallel())
	}

	// Verify inherited from level2
	if config.GetCloneProto() != "https" {
		t.Errorf("Expected cloneProto=https (from level2), got %s", config.GetCloneProto())
	}

	// Verify parent chain depth
	chain := GetParentChain(config)
	if len(chain) != 3 {
		t.Errorf("Expected 3 configs in chain, got %d", len(chain))
	}

	// Verify profile lookup through chain
	profile := GetProfileFromChain(config, "root-profile")
	if profile == nil {
		t.Fatal("Expected to find 'root-profile' in chain")
	}
	if profile.Provider != "github" {
		t.Errorf("Expected provider=github, got %s", profile.Provider)
	}

	profile = GetProfileFromChain(config, "middle-profile")
	if profile == nil {
		t.Fatal("Expected to find 'middle-profile' in chain")
	}
	if profile.Provider != "gitlab" {
		t.Errorf("Expected provider=gitlab, got %s", profile.Provider)
	}
}

func TestGetProfileFromChain(t *testing.T) {
	// Create config chain manually
	grandparent := &Config{
		Profiles: map[string]*Profile{
			"grandparent-profile": {Name: "grandparent-profile", Provider: "github"},
		},
	}

	parent := &Config{
		Profiles: map[string]*Profile{
			"parent-profile": {Name: "parent-profile", Provider: "gitlab"},
		},
		ParentConfig: grandparent,
	}

	child := &Config{
		Profiles: map[string]*Profile{
			"child-profile": {Name: "child-profile", Provider: "gitea"},
		},
		ParentConfig: parent,
	}

	// Test profile found at each level
	profile := GetProfileFromChain(child, "child-profile")
	if profile == nil || profile.Provider != "gitea" {
		t.Error("Expected child-profile from child")
	}

	profile = GetProfileFromChain(child, "parent-profile")
	if profile == nil || profile.Provider != "gitlab" {
		t.Error("Expected parent-profile from parent")
	}

	profile = GetProfileFromChain(child, "grandparent-profile")
	if profile == nil || profile.Provider != "github" {
		t.Error("Expected grandparent-profile from grandparent")
	}

	// Test profile not found
	profile = GetProfileFromChain(child, "nonexistent")
	if profile != nil {
		t.Error("Expected nil for nonexistent profile")
	}

	// Test nil config
	profile = GetProfileFromChain(nil, "test")
	if profile != nil {
		t.Error("Expected nil for nil config")
	}

	// Test empty name
	profile = GetProfileFromChain(child, "")
	if profile != nil {
		t.Error("Expected nil for empty name")
	}
}

func TestGetProfileSource(t *testing.T) {
	grandparent := &Config{
		ConfigPath: "/home/user/workstation/.gz-git.yaml",
		Profiles: map[string]*Profile{
			"gp-profile": {Name: "gp-profile"},
		},
	}

	parent := &Config{
		ConfigPath:   "/home/user/devbox/.gz-git.yaml",
		ParentConfig: grandparent,
		Profiles: map[string]*Profile{
			"p-profile": {Name: "p-profile"},
		},
	}

	child := &Config{
		ConfigPath:   "/home/user/devbox/project/.gz-git.yaml",
		ParentConfig: parent,
		Profiles: map[string]*Profile{
			"c-profile": {Name: "c-profile"},
		},
	}

	// Test source at each level
	source := GetProfileSource(child, "c-profile")
	if source != "/home/user/devbox/project/.gz-git.yaml" {
		t.Errorf("Expected child config path, got %s", source)
	}

	source = GetProfileSource(child, "p-profile")
	if source != "/home/user/devbox/.gz-git.yaml" {
		t.Errorf("Expected parent config path, got %s", source)
	}

	source = GetProfileSource(child, "gp-profile")
	if source != "/home/user/workstation/.gz-git.yaml" {
		t.Errorf("Expected grandparent config path, got %s", source)
	}

	// Test profile not found
	source = GetProfileSource(child, "nonexistent")
	if source != "" {
		t.Errorf("Expected empty string for nonexistent profile, got %s", source)
	}
}

func TestGetParentChain(t *testing.T) {
	grandparent := &Config{ConfigPath: "/gp"}
	parent := &Config{ConfigPath: "/p", ParentConfig: grandparent}
	child := &Config{ConfigPath: "/c", ParentConfig: parent}

	chain := GetParentChain(child)
	if len(chain) != 3 {
		t.Errorf("Expected 3 configs in chain, got %d", len(chain))
	}

	if chain[0].ConfigPath != "/c" {
		t.Error("Expected child first in chain")
	}
	if chain[1].ConfigPath != "/p" {
		t.Error("Expected parent second in chain")
	}
	if chain[2].ConfigPath != "/gp" {
		t.Error("Expected grandparent third in chain")
	}

	// Test nil config
	chain = GetParentChain(nil)
	if chain != nil {
		t.Error("Expected nil for nil config")
	}
}

func TestMergeParentConfig(t *testing.T) {
	parent := &Config{
		Provider: "gitlab",
		BaseURL:  "https://gitlab.example.com",
		Defaults: &DefaultsConfig{
			Clone: &CloneDefaults{
				Proto: "ssh",
			},
			Sync: &SyncDefaults{
				Parallel: 10,
			},
		},
		Sync: &SyncConfig{
			Strategy:   "reset",
			MaxRetries: 3,
		},
	}

	child := &Config{
		Defaults: &DefaultsConfig{
			Sync: &SyncDefaults{
				Parallel: 5, // Override parent
			},
		},
		Sync: &SyncConfig{
			Strategy: "pull", // Override parent
		},
	}

	mergeParentConfig(child, parent)

	// Child overrides preserved
	if child.GetParallel() != 5 {
		t.Errorf("Expected parallel=5 (child override), got %d", child.GetParallel())
	}
	if child.Sync.Strategy != "pull" {
		t.Errorf("Expected sync.strategy=pull (child override), got %s", child.Sync.Strategy)
	}

	// Parent defaults applied
	if child.Provider != "gitlab" {
		t.Errorf("Expected provider=gitlab (from parent), got %s", child.Provider)
	}
	if child.BaseURL != "https://gitlab.example.com" {
		t.Errorf("Expected baseURL from parent, got %s", child.BaseURL)
	}
	if child.GetCloneProto() != "ssh" {
		t.Errorf("Expected cloneProto=ssh (from parent), got %s", child.GetCloneProto())
	}

	// Sync config partially merged
	if child.Sync.MaxRetries != 3 {
		t.Errorf("Expected sync.maxRetries=3 (from parent), got %d", child.Sync.MaxRetries)
	}
}

func TestLoadConfigRecursive_AbsoluteParentPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create parent config
	parentDir := filepath.Join(tmpDir, "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	parentConfig := `defaults:
  sync:
    parallel: 20
profiles:
  test-profile:
    name: test-profile
    provider: github
`
	if err := os.WriteFile(filepath.Join(parentDir, ".gz-git.yaml"), []byte(parentConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create child config with absolute parent path
	childDir := filepath.Join(tmpDir, "child")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatal(err)
	}
	childConfig := fmt.Sprintf(`parent: %s/.gz-git.yaml
profile: test-profile
`, parentDir)
	if err := os.WriteFile(filepath.Join(childDir, ".gz-git.yaml"), []byte(childConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load child config
	config, err := LoadConfigRecursive(childDir, ".gz-git.yaml")
	if err != nil {
		t.Fatalf("LoadConfigRecursive failed: %v", err)
	}

	// Verify parent loaded
	if config.ParentConfig == nil {
		t.Fatal("Expected ParentConfig to be set")
	}
	if config.GetParallel() != 20 {
		t.Errorf("Expected parallel=20 (from parent), got %d", config.GetParallel())
	}

	// Verify profile lookup
	profile := GetProfileFromChain(config, "test-profile")
	if profile == nil {
		t.Fatal("Expected to find test-profile")
	}
}
