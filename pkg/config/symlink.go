// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultConfigFileName is the default name for config files.
const DefaultConfigFileName = ".gz-git.yaml"

// CreateConfigSymlink creates a symlink from the workspace config location to a source config file.
//
// Parameters:
//   - srcPath: The source config file path. Supports:
//   - Absolute paths: /path/to/config.yaml
//   - Home-relative: ~/configs/myconfig.yaml
//   - Relative to parent config: ./configs/myconfig.yaml
//   - targetDir: The workspace directory where the symlink will be created as .gz-git.yaml
//   - parentConfigDir: The directory containing the parent config (for resolving relative paths)
//
// The symlink is created at {targetDir}/.gz-git.yaml → {resolved srcPath}
//
// If a file already exists at the symlink location:
//   - If it's a symlink: it will be removed and recreated
//   - If it's a regular file: returns an error (use force=true to override)
func CreateConfigSymlink(srcPath, targetDir, parentConfigDir string) error {
	if srcPath == "" {
		return nil // No symlink requested
	}

	// Resolve source path (parentConfigDir is the base, srcPath is the relative/absolute path)
	resolvedSrc, err := resolvePath(parentConfigDir, srcPath)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}

	// Verify source exists
	if _, err := os.Stat(resolvedSrc); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config link source does not exist: %s", resolvedSrc)
		}
		return fmt.Errorf("check source path: %w", err)
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	// Symlink destination
	linkPath := filepath.Join(targetDir, DefaultConfigFileName)

	// Check if something already exists at link path
	fi, err := os.Lstat(linkPath)
	if err == nil {
		// Something exists
		if fi.Mode()&os.ModeSymlink != 0 {
			// It's a symlink - remove it
			if err := os.Remove(linkPath); err != nil {
				return fmt.Errorf("remove existing symlink: %w", err)
			}
		} else {
			// It's a regular file - don't overwrite without force
			return fmt.Errorf("config file already exists at %s (not a symlink); remove it or use configLink on a fresh directory", linkPath)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check link path: %w", err)
	}

	// Create the symlink
	if err := os.Symlink(resolvedSrc, linkPath); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	return nil
}

// CreateConfigSymlinkForce creates a symlink, removing any existing file (not just symlinks).
// Use with caution - this will delete existing config files.
func CreateConfigSymlinkForce(srcPath, targetDir, parentConfigDir string) error {
	if srcPath == "" {
		return nil
	}

	// Resolve source path (parentConfigDir is the base, srcPath is the relative/absolute path)
	resolvedSrc, err := resolvePath(parentConfigDir, srcPath)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}

	// Verify source exists
	if _, err := os.Stat(resolvedSrc); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config link source does not exist: %s", resolvedSrc)
		}
		return fmt.Errorf("check source path: %w", err)
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	// Symlink destination
	linkPath := filepath.Join(targetDir, DefaultConfigFileName)

	// Remove anything that exists
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing file: %w", err)
	}

	// Create the symlink
	if err := os.Symlink(resolvedSrc, linkPath); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	return nil
}

// IsConfigSymlink checks if the config file at the given path is a symlink.
func IsConfigSymlink(configPath string) (bool, error) {
	fi, err := os.Lstat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return fi.Mode()&os.ModeSymlink != 0, nil
}

// GetSymlinkTarget returns the target of a symlink, or empty string if not a symlink.
func GetSymlinkTarget(linkPath string) (string, error) {
	isSymlink, err := IsConfigSymlink(linkPath)
	if err != nil {
		return "", err
	}
	if !isSymlink {
		return "", nil
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		return "", fmt.Errorf("read symlink: %w", err)
	}

	return target, nil
}
