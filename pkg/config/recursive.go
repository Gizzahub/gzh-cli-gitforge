// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfigRecursive loads a config file and recursively loads all workspaces.
// This function works at ANY level (workstation, workspace, project, etc.)
//
// Parameters:
//   - path: The directory containing the config file
//   - configFile: The config file name (e.g., ".gz-git.yaml", ".gz-git-config.yaml")
//
// Returns:
//   - *Config: The loaded config with all workspaces recursively loaded
//   - error: Any error encountered during loading
//
// Example:
//
//	// Load workstation config
//	home, _ := os.UserHomeDir()
//	config, err := LoadConfigRecursive(home, ".gz-git-config.yaml")
//
//	// Load workspace config
//	config, err := LoadConfigRecursive("/home/user/mydevbox", ".gz-git.yaml")
func LoadConfigRecursive(path string, configFile string) (*Config, error) {
	// 1. Load this level's config file
	configPath := filepath.Join(path, configFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", configPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", configPath, err)
	}

	// 2. Recursively load workspaces
	for name, ws := range config.Workspaces {
		if ws == nil {
			continue
		}

		// Resolve workspace path (handle ~, relative paths)
		wsPath, err := resolvePath(path, ws.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve workspace '%s' path '%s' in %s: %w", name, ws.Path, configPath, err)
		}

		// Determine effective type
		effectiveType := ws.Type.Resolve(ws.Source != nil)

		switch effectiveType {
		case WorkspaceTypeConfig:
			// Workspace has a nested config file - recurse!
			nestedConfigFile := ".gz-git.yaml"
			nestedConfig, err := LoadConfigRecursive(wsPath, nestedConfigFile)
			if err != nil {
				// Config file not found is OK (use workspace settings only)
				continue
			}

			// Merge workspace overrides into loaded config
			mergeWorkspaceOverrides(nestedConfig, ws)

		case WorkspaceTypeGit:
			// Plain git repo - validate that it exists and is a git repo
			if !isGitRepo(wsPath) {
				// Git repo doesn't exist yet - this is OK for sync targets
				// It will be cloned when sync is run
				continue
			}

		case WorkspaceTypeForge:
			// Forge workspace - target directory may not exist yet
			// It will be created when sync from-forge is run
			// No validation needed here
		}

		// Recursively process nested workspaces
		for nestedName, nestedWs := range ws.Workspaces {
			if nestedWs == nil {
				continue
			}

			nestedPath, err := resolvePath(wsPath, nestedWs.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve nested workspace '%s/%s' path '%s': %w",
					name, nestedName, nestedWs.Path, err)
			}

			nestedType := nestedWs.Type.Resolve(nestedWs.Source != nil)
			if nestedType == WorkspaceTypeConfig {
				nestedConfigFile := ".gz-git.yaml"
				_, err := LoadConfigRecursive(nestedPath, nestedConfigFile)
				if err != nil {
					// Config not found is OK
					continue
				}
			}
		}
	}

	return &config, nil
}

// resolvePath resolves a path relative to a parent directory.
// Handles:
//   - Home-relative paths: ~/foo/bar → /home/user/foo/bar
//   - Absolute paths: /foo/bar → /foo/bar
//   - Relative paths: ./foo → /parent/path/foo
//   - Relative paths: foo → /parent/path/foo
func resolvePath(parentPath string, childPath string) (string, error) {
	// Handle home-relative paths
	if strings.HasPrefix(childPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, childPath[2:]), nil
	}

	// Handle absolute paths
	if filepath.IsAbs(childPath) {
		return childPath, nil
	}

	// Handle relative paths
	return filepath.Join(parentPath, childPath), nil
}

// mergeWorkspaceOverrides applies workspace overrides to loaded Config.
// Workspace overrides take precedence over the nested config file settings.
func mergeWorkspaceOverrides(config *Config, ws *Workspace) {
	if ws.Profile != "" {
		config.Profile = ws.Profile
	}
	if ws.Parallel > 0 {
		config.Parallel = ws.Parallel
	}
	if ws.CloneProto != "" {
		config.CloneProto = ws.CloneProto
	}
	if ws.SSHPort > 0 {
		config.SSHPort = ws.SSHPort
	}
	if ws.Sync != nil {
		if config.Sync == nil {
			config.Sync = &SyncConfig{}
		}
		mergeSyncConfig(config.Sync, ws.Sync)
	}
	if ws.Branch != nil {
		if config.Branch == nil {
			config.Branch = &BranchConfig{}
		}
		mergeBranchConfig(config.Branch, ws.Branch)
	}
	if ws.Fetch != nil {
		if config.Fetch == nil {
			config.Fetch = &FetchConfig{}
		}
		mergeFetchConfig(config.Fetch, ws.Fetch)
	}
	if ws.Pull != nil {
		if config.Pull == nil {
			config.Pull = &PullConfig{}
		}
		mergePullConfig(config.Pull, ws.Pull)
	}
	if ws.Push != nil {
		if config.Push == nil {
			config.Push = &PushConfig{}
		}
		mergePushConfig(config.Push, ws.Push)
	}
}

// mergeSyncConfig merges override into target (override takes precedence).
func mergeSyncConfig(target *SyncConfig, override *SyncConfig) {
	if override.Strategy != "" {
		target.Strategy = override.Strategy
	}
	if override.MaxRetries > 0 {
		target.MaxRetries = override.MaxRetries
	}
	if override.Timeout != "" {
		target.Timeout = override.Timeout
	}
}

// mergeBranchConfig merges override into target (override takes precedence).
func mergeBranchConfig(target *BranchConfig, override *BranchConfig) {
	if override.DefaultBranch != "" {
		target.DefaultBranch = override.DefaultBranch
	}
	if len(override.ProtectedBranches) > 0 {
		target.ProtectedBranches = override.ProtectedBranches
	}
}

// mergeFetchConfig merges override into target (override takes precedence).
func mergeFetchConfig(target *FetchConfig, override *FetchConfig) {
	if override.AllRemotes {
		target.AllRemotes = override.AllRemotes
	}
	if override.Prune {
		target.Prune = override.Prune
	}
}

// mergePullConfig merges override into target (override takes precedence).
func mergePullConfig(target *PullConfig, override *PullConfig) {
	if override.Rebase {
		target.Rebase = override.Rebase
	}
	if override.FFOnly {
		target.FFOnly = override.FFOnly
	}
}

// mergePushConfig merges override into target (override takes precedence).
func mergePushConfig(target *PushConfig, override *PushConfig) {
	if override.SetUpstream {
		target.SetUpstream = override.SetUpstream
	}
}

// isGitRepo checks if a directory is a git repository.
func isGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// LoadWorkspaces loads workspaces based on discovery mode.
// This function is called after LoadConfigRecursive to optionally auto-discover workspaces.
//
// Parameters:
//   - path: The directory to search for workspaces
//   - config: The config to append discovered workspaces to
//   - mode: The discovery mode (explicit, auto, hybrid)
//
// Returns:
//   - error: Any error encountered during discovery
//
// Example:
//
//	config, _ := LoadConfigRecursive("/home/user/mydevbox", ".gz-git.yaml")
//	err := LoadWorkspaces("/home/user/mydevbox", config, HybridMode)
func LoadWorkspaces(path string, config *Config, mode DiscoveryMode) error {
	// Use the default mode if not specified
	mode = mode.Default()

	switch mode {
	case ExplicitMode:
		// Already loaded by LoadConfigRecursive - nothing to do
		return nil

	case AutoMode:
		// Scan directory and replace config.Workspaces with discovered repos
		config.Workspaces = nil // Clear explicit workspaces
		return autoDiscoverWorkspaces(path, config)

	case HybridMode:
		// Use explicit workspaces if defined, otherwise auto-discover
		if len(config.Workspaces) > 0 {
			return nil // Use explicit
		}
		return autoDiscoverWorkspaces(path, config)
	}

	return nil
}

// autoDiscoverWorkspaces scans directory and appends discovered repos to config.Workspaces.
func autoDiscoverWorkspaces(path string, config *Config) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", path, err)
	}

	if config.Workspaces == nil {
		config.Workspaces = make(map[string]*Workspace)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip hidden directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		childPath := filepath.Join(path, entry.Name())

		// Check if it has a config file
		if hasFile(childPath, ".gz-git.yaml") {
			config.Workspaces[entry.Name()] = &Workspace{
				Path: childPath,
				Type: WorkspaceTypeConfig,
			}
			continue
		}

		// Check if it's a git repo
		if isGitRepo(childPath) {
			config.Workspaces[entry.Name()] = &Workspace{
				Path: childPath,
				Type: WorkspaceTypeGit,
			}
		}
	}

	return nil
}

// hasFile checks if a file exists in a directory.
func hasFile(dir string, fileName string) bool {
	filePath := filepath.Join(dir, fileName)
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// FindConfigRecursive walks up the directory tree to find a config file.
// This is used to find the nearest config file for the current directory.
//
// Parameters:
//   - startPath: The directory to start searching from
//   - configFile: The config file name to search for (e.g., ".gz-git.yaml")
//
// Returns:
//   - string: The directory containing the config file
//   - error: Error if config file not found
//
// Example:
//
//	// Find nearest .gz-git.yaml
//	configDir, err := FindConfigRecursive("/home/user/mydevbox/project", ".gz-git.yaml")
//	// Returns: "/home/user/mydevbox" if .gz-git.yaml exists there
func FindConfigRecursive(startPath string, configFile string) (string, error) {
	currentPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Walk up the directory tree
	for {
		configPath := filepath.Join(currentPath, configFile)
		if _, err := os.Stat(configPath); err == nil {
			return currentPath, nil
		}

		// Move to parent directory
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached root
			break
		}
		currentPath = parentPath
	}

	return "", fmt.Errorf("config file %s not found in %s or any parent directory", configFile, startPath)
}

// GetWorkspaceByName returns a workspace by name from the config.
func GetWorkspaceByName(config *Config, name string) *Workspace {
	if config == nil || config.Workspaces == nil {
		return nil
	}
	return config.Workspaces[name]
}

// GetAllWorkspaces returns all workspaces as a slice with their names.
func GetAllWorkspaces(config *Config) []struct {
	Name      string
	Workspace *Workspace
} {
	if config == nil || config.Workspaces == nil {
		return nil
	}

	result := make([]struct {
		Name      string
		Workspace *Workspace
	}, 0, len(config.Workspaces))

	for name, ws := range config.Workspaces {
		result = append(result, struct {
			Name      string
			Workspace *Workspace
		}{Name: name, Workspace: ws})
	}

	return result
}

// GetForgeWorkspaces returns only workspaces that sync from a forge.
func GetForgeWorkspaces(config *Config) map[string]*Workspace {
	result := make(map[string]*Workspace)

	if config == nil || config.Workspaces == nil {
		return result
	}

	for name, ws := range config.Workspaces {
		if ws.Source != nil || ws.Type == WorkspaceTypeForge {
			result[name] = ws
		}
	}

	return result
}

// GetProfileByName returns a profile by name from the config.
// Lookup order: inline (config.Profiles) → external (~/.config/gz-git/profiles/)
// Returns nil if profile not found in either location.
func GetProfileByName(config *Config, name string) *Profile {
	if config == nil || name == "" {
		return nil
	}

	// 1. Check inline profiles first (higher priority)
	if config.Profiles != nil {
		if profile, ok := config.Profiles[name]; ok {
			return profile
		}
	}

	// 2. External profiles would be loaded by manager.go
	// This function only handles inline profiles
	// For external profile loading, use Manager.GetProfile()
	return nil
}

// GetAllProfiles returns all inline profiles from the config.
func GetAllProfiles(config *Config) map[string]*Profile {
	if config == nil || config.Profiles == nil {
		return make(map[string]*Profile)
	}
	return config.Profiles
}

// HasInlineProfile checks if a profile exists in the inline profiles.
func HasInlineProfile(config *Config, name string) bool {
	if config == nil || config.Profiles == nil {
		return false
	}
	_, ok := config.Profiles[name]
	return ok
}
