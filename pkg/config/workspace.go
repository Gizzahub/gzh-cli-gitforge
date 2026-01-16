// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// WorkstationConfigFileName is the workstation-wide config file
	WorkstationConfigFileName = ".gz-git-config.yaml"

	// WorkspaceConfigFileName is the workspace-level config file
	WorkspaceConfigFileName = ".gz-git-workspace.yaml"
)

// WorkstationConfig represents ~/.gz-git-config.yaml (workstation-wide settings).
//
// Example:
//
//	defaults:
//	  parallel: 5
//	  cloneProto: ssh
//	workspaces:
//	  ~/mydevbox:
//	    profile: opensource
//	    parallel: 10
//	  ~/mywork:
//	    profile: work
type WorkstationConfig struct {
	// Defaults apply to all workspaces
	Defaults map[string]interface{} `yaml:"defaults,omitempty"`

	// Workspaces maps workspace paths to their configs
	Workspaces map[string]WorkspaceMapping `yaml:"workspaces,omitempty"`
}

// WorkspaceMapping defines settings for a specific workspace.
type WorkspaceMapping struct {
	Profile  string `yaml:"profile,omitempty"`
	Parallel int    `yaml:"parallel,omitempty"`
}

// WorkspaceConfig represents .gz-git-workspace.yaml (workspace-level settings).
// This file applies to all projects within a workspace directory.
//
// Example:
//
//	profile: opensource
//	sync:
//	  strategy: reset
//	  parallel: 10
//	branch:
//	  defaultBranch: main
//	metadata:
//	  workspace: mydevbox
//	  type: development
type WorkspaceConfig struct {
	// Profile specifies which profile to use for this workspace
	Profile string `yaml:"profile,omitempty"`

	// Command-specific overrides
	Sync   *SyncConfig   `yaml:"sync,omitempty"`
	Branch *BranchConfig `yaml:"branch,omitempty"`
	Fetch  *FetchConfig  `yaml:"fetch,omitempty"`
	Pull   *PullConfig   `yaml:"pull,omitempty"`
	Push   *PushConfig   `yaml:"push,omitempty"`

	// Metadata is optional workspace information
	Metadata *WorkspaceMetadata `yaml:"metadata,omitempty"`
}

// WorkspaceMetadata holds optional workspace information.
type WorkspaceMetadata struct {
	Workspace string `yaml:"workspace,omitempty"`
	Type      string `yaml:"type,omitempty"` // development, production, personal, etc.
	Owner     string `yaml:"owner,omitempty"`
}

// FindWorkstationConfig finds the workstation config file.
// It looks for ~/.gz-git-config.yaml.
func FindWorkstationConfig() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, WorkstationConfigFileName)
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	}

	return "", nil // Not found (not an error)
}

// FindWorkspaceConfig walks up the directory tree to find .gz-git-workspace.yaml.
// It starts from the current working directory and stops at the home directory.
//
// This finds the NEAREST workspace config (closest to current directory).
func FindWorkspaceConfig() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Walk up directory tree
	dir := cwd
	for {
		configPath := filepath.Join(dir, WorkspaceConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		// Stop at home directory
		if dir == homeDir {
			break
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return "", nil // Not found (not an error)
}

// FindAllConfigs finds all config files in precedence order.
// Returns: workstation, workspace, project config paths.
func FindAllConfigs() (workstation, workspace, project string, err error) {
	// Find workstation config
	workstation, err = FindWorkstationConfig()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to find workstation config: %w", err)
	}

	// Find workspace config
	workspace, err = FindWorkspaceConfig()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to find workspace config: %w", err)
	}

	// Find project config (existing function)
	project, err = FindProjectConfig()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to find project config: %w", err)
	}

	return workstation, workspace, project, nil
}

// GetWorkspaceRoot returns the directory containing .gz-git-workspace.yaml.
// Returns empty string if no workspace config found.
func GetWorkspaceRoot() (string, error) {
	configPath, err := FindWorkspaceConfig()
	if err != nil {
		return "", err
	}

	if configPath == "" {
		return "", nil // No workspace config
	}

	return filepath.Dir(configPath), nil
}

// IsInWorkspace checks if the current directory is within a workspace.
func IsInWorkspace() (bool, error) {
	workspaceRoot, err := GetWorkspaceRoot()
	if err != nil {
		return false, err
	}

	return workspaceRoot != "", nil
}
