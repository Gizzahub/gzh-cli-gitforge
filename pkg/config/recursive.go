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

// LoadConfigRecursive loads a config file and recursively loads all children.
// This function works at ANY level (workstation, workspace, project, etc.)
//
// Parameters:
//   - path: The directory containing the config file
//   - configFile: The config file name (e.g., ".gz-git.yaml", ".work-config.yaml")
//
// Returns:
//   - *Config: The loaded config with all children recursively loaded
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

	// 2. Recursively load children
	for i := range config.Children {
		child := &config.Children[i]

		// Validate child type
		if !child.Type.IsValid() {
			return nil, fmt.Errorf("invalid child type '%s' in %s: must be 'config' or 'git'", child.Type, configPath)
		}

		// Resolve child path (handle ~, relative paths)
		childPath, err := resolvePath(path, child.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve child path '%s' in %s: %w", child.Path, configPath, err)
		}

		if child.Type == ChildTypeConfig {
			// Child has a config file - recurse!
			childConfigFile := child.ConfigFile
			if childConfigFile == "" {
				childConfigFile = child.Type.DefaultConfigFile() // ".gz-git.yaml"
			}

			// RECURSIVE CALL!
			childConfig, err := LoadConfigRecursive(childPath, childConfigFile)
			if err != nil {
				// Config file not found is OK (use inline overrides only)
				// This allows defining children without requiring config files
				continue
			}

			// Merge inline overrides into loaded config
			mergeInlineOverrides(childConfig, child)

			// Note: We could store childConfig in child.LoadedConfig for validation/debugging
			// For now, we just validate that the file loads successfully
		} else if child.Type == ChildTypeGit {
			// Plain git repo - validate that it exists and is a git repo
			if !isGitRepo(childPath) {
				return nil, fmt.Errorf("child path is not a git repo: %s (defined in %s)", childPath, configPath)
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

// mergeInlineOverrides applies inline overrides from ChildEntry to loaded Config.
// Inline overrides take precedence over the child's config file settings.
func mergeInlineOverrides(config *Config, entry *ChildEntry) {
	if entry.Profile != "" {
		config.Profile = entry.Profile
	}
	if entry.Parallel > 0 {
		config.Parallel = entry.Parallel
	}
	if entry.Sync != nil {
		if config.Sync == nil {
			config.Sync = &SyncConfig{}
		}
		mergeSyncConfig(config.Sync, entry.Sync)
	}
	if entry.Branch != nil {
		if config.Branch == nil {
			config.Branch = &BranchConfig{}
		}
		mergeBranchConfig(config.Branch, entry.Branch)
	}
	if entry.Fetch != nil {
		if config.Fetch == nil {
			config.Fetch = &FetchConfig{}
		}
		mergeFetchConfig(config.Fetch, entry.Fetch)
	}
	if entry.Pull != nil {
		if config.Pull == nil {
			config.Pull = &PullConfig{}
		}
		mergePullConfig(config.Pull, entry.Pull)
	}
	if entry.Push != nil {
		if config.Push == nil {
			config.Push = &PushConfig{}
		}
		mergePushConfig(config.Push, entry.Push)
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
	// For bool fields, we need to check if they were explicitly set
	// For now, we just copy the values (true overrides false)
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

// LoadChildren loads children based on discovery mode.
// This function is called after LoadConfigRecursive to optionally auto-discover children.
//
// Parameters:
//   - path: The directory to search for children
//   - config: The config to append discovered children to
//   - mode: The discovery mode (explicit, auto, hybrid)
//
// Returns:
//   - error: Any error encountered during discovery
//
// Example:
//
//	config, _ := LoadConfigRecursive("/home/user/mydevbox", ".gz-git.yaml")
//	err := LoadChildren("/home/user/mydevbox", config, HybridMode)
func LoadChildren(path string, config *Config, mode DiscoveryMode) error {
	// Use the default mode if not specified
	mode = mode.Default()

	switch mode {
	case ExplicitMode:
		// Already loaded by LoadConfigRecursive - nothing to do
		return nil

	case AutoMode:
		// Scan directory and replace config.Children with discovered repos
		config.Children = nil // Clear explicit children
		return autoDiscoverAndAppend(path, config)

	case HybridMode:
		// Use explicit children if defined, otherwise auto-discover
		if len(config.Children) > 0 {
			return nil // Use explicit
		}
		return autoDiscoverAndAppend(path, config)
	}

	return nil
}

// autoDiscoverAndAppend scans directory and appends discovered repos to config.Children.
func autoDiscoverAndAppend(path string, config *Config) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", path, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		childPath := filepath.Join(path, entry.Name())

		// Check if it has a config file
		if hasFile(childPath, ".gz-git.yaml") {
			config.Children = append(config.Children, ChildEntry{
				Path: childPath,
				Type: ChildTypeConfig,
			})
			continue
		}

		// Check if it's a git repo
		if isGitRepo(childPath) {
			config.Children = append(config.Children, ChildEntry{
				Path: childPath,
				Type: ChildTypeGit,
			})
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
