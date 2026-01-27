// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// ConfigDirName is the config directory name under XDG_CONFIG_HOME
	ConfigDirName = "gz-git"

	// ProfilesDirName is the subdirectory for profile files
	ProfilesDirName = "profiles"

	// StateDirName is the subdirectory for runtime state
	StateDirName = "state"

	// GlobalConfigFileName is the base name for the main config file
	GlobalConfigFileName = "config"

	// ProjectConfigFileName is the base name for the project-specific config file
	ProjectConfigFileName = ".gz-git"

	// ActiveProfileFileName stores the active profile name
	ActiveProfileFileName = "active-profile.txt"

	// DefaultProfileName is the default profile
	DefaultProfileName = "default"
)

var (
	// supportedExtensions is the list of config file extensions to check
	supportedExtensions = []string{".yaml", ".yml", ".json"}
)

// Paths provides access to all config file locations.
type Paths struct {
	// ConfigDir is the root config directory (~/.config/gz-git)
	ConfigDir string

	// ProfilesDir is the profiles directory (~/.config/gz-git/profiles)
	ProfilesDir string

	// StateDir is the state directory (~/.config/gz-git/state)
	StateDir string

	// GlobalConfigFile is the global config file path
	GlobalConfigFile string

	// ActiveProfileFile tracks the active profile
	ActiveProfileFile string
}

// findFile checks for a file with multiple extensions and returns the first one found.
// It prioritizes the extensions in the order they are provided.
func findFile(basePath string, extensions []string) string {
	for _, ext := range extensions {
		path := basePath + ext
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// NewPaths creates a Paths instance with standard locations.
// It uses XDG_CONFIG_HOME if set, otherwise falls back to ~/.config.
func NewPaths() (*Paths, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user config directory: %w", err)
	}

	configDir := filepath.Join(configHome, ConfigDirName)

	return &Paths{
		ConfigDir:         configDir,
		ProfilesDir:       filepath.Join(configDir, ProfilesDirName),
		StateDir:          filepath.Join(configDir, StateDirName),
		GlobalConfigFile:  findFile(filepath.Join(configDir, GlobalConfigFileName), supportedExtensions),
		ActiveProfileFile: filepath.Join(configDir, StateDirName, ActiveProfileFileName),
	}, nil
}

// ProfilePath returns the path to a specific profile file, checking for supported extensions.
func (p *Paths) ProfilePath(name string) string {
	basePath := filepath.Join(p.ProfilesDir, name)
	return findFile(basePath, supportedExtensions)
}

// EnsureDirectories creates all necessary directories with correct permissions.
// Directories are created with 0700 (user access only).
func (p *Paths) EnsureDirectories() error {
	dirs := []string{
		p.ConfigDir,
		p.ProfilesDir,
		p.StateDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// Exists checks if the config directory exists.
func (p *Paths) Exists() bool {
	info, err := os.Stat(p.ConfigDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ProfileExists checks if a profile file exists.
func (p *Paths) ProfileExists(name string) bool {
	return p.ProfilePath(name) != ""
}

// ListProfiles returns all available profile names.
func (p *Paths) ListProfiles() ([]string, error) {
	entries, err := os.ReadDir(p.ProfilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	var profiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		for _, supportedExt := range supportedExtensions {
			if ext == supportedExt {
				profiles = append(profiles, name[:len(name)-len(ext)])
				break
			}
		}
	}

	return profiles, nil
}

// FindProjectConfig walks up the directory tree to find .gz-git config file.
// It starts from the current working directory and stops at the home directory.
func FindProjectConfig() (string, error) {
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
		basePath := filepath.Join(dir, ProjectConfigFileName)
		if configPath := findFile(basePath, supportedExtensions); configPath != "" {
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

// GetActiveProfile reads the active profile name from state file.
// Returns empty string if not set.
func (p *Paths) GetActiveProfile() (string, error) {
	data, err := os.ReadFile(p.ActiveProfileFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No active profile set
		}
		return "", fmt.Errorf("failed to read active profile file: %w", err)
	}

	profile := string(data)
	// Trim whitespace/newlines
	profile = filepath.Clean(profile)

	return profile, nil
}

// SetActiveProfile writes the active profile name to state file.
func (p *Paths) SetActiveProfile(name string) error {
	// Ensure state directory exists
	if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write profile name
	if err := os.WriteFile(p.ActiveProfileFile, []byte(name), 0o600); err != nil {
		return fmt.Errorf("failed to write active profile file: %w", err)
	}

	return nil
}

// DetectConfigFile searches for config files in the given directory.
// It checks for supported extensions in their defined order.
// Returns the full path to the found config file, or an error if not found.
func DetectConfigFile(dir string) (string, error) {
	path, _ := DetectConfigFileWithKind(dir)
	if path == "" {
		return "", fmt.Errorf("config file not found in %s (tried %s with extensions %v)",
			dir, ProjectConfigFileName, supportedExtensions)
	}
	return path, nil
}

// ConfigFileInfo holds detected config file information.
type ConfigFileInfo struct {
	Path string
	Kind ConfigKind
}

// DetectConfigFileWithKind searches for config files and returns path.
// It checks for supported extensions in their defined order.
func DetectConfigFileWithKind(dir string) (string, ConfigKind) {
	basePath := filepath.Join(dir, ProjectConfigFileName)
	if path := findFile(basePath, supportedExtensions); path != "" {
		// Kind is not determined by filename; caller should read file content
		return path, ""
	}
	return "", ""
}

// DetectAllConfigFiles finds all config files in a directory.
// Returns list of found config file paths.
func DetectAllConfigFiles(dir string) []string {
	var result []string
	basePath := filepath.Join(dir, ProjectConfigFileName)
	for _, ext := range supportedExtensions {
		path := basePath + ext
		if _, err := os.Stat(path); err == nil {
			result = append(result, path)
		}
_	}
	return result
}
