// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

const (
	// ConfigDirName is the config directory name under XDG_CONFIG_HOME.
	ConfigDirName = "gz-git"

	// ProfilesDirName is the subdirectory for profile files.
	ProfilesDirName = "profiles"

	// StateDirName is the subdirectory for runtime state.
	StateDirName = "state"

	// GlobalConfigFileName is the base name for the main config file.
	GlobalConfigFileName = "config"

	// ProjectConfigFileName is the base name for the project-specific config file.
	ProjectConfigFileName = ".gz-git"

	// ActiveProfileFileName stores the active profile name.
	ActiveProfileFileName = "active-profile.txt"

	// DefaultProfileName is the default profile.
	DefaultProfileName = "default"
)

// supportedExtensions is the list of config file extensions to check.
var supportedExtensions = []string{".yaml", ".yml", ".json"}

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
		if slices.Contains(supportedExtensions, ext) {
			profiles = append(profiles, name[:len(name)-len(ext)])
		}
	}

	return profiles, nil
}

// projectConfigNames returns the concrete config filenames probed per directory,
// in priority order (.gz-git.yaml, .gz-git.yml, .gz-git.json).
func projectConfigNames() []string {
	names := make([]string, 0, len(supportedExtensions))
	for _, ext := range supportedExtensions {
		names = append(names, ProjectConfigFileName+ext)
	}
	return names
}

// findConfigUpward is the single config-discovery algorithm shared by
// FindProjectConfig, DetectConfigFile and FindConfigRecursive. Starting at
// startDir it ascends the parent chain, returning the first directory that
// contains one of names together with the full path to that file. The ascent is
// bounded by $HOME: it never rises above the user's home directory, which keeps
// discovery from leaking into other users' or system directories. For start
// directories outside $HOME it stops at the filesystem root. Returns
// ("", "", nil) when nothing matches — absence is not an error here; callers that
// require a config translate the empty result into their own error.
func findConfigUpward(startDir string, names []string) (dir, path string, err error) {
	// $HOME is the ascent ceiling when it can be determined. If it can't (e.g.
	// HOME unset in a minimal container), fall back to no ceiling and stop at the
	// filesystem root rather than failing discovery outright — an empty homeDir
	// never equals a real directory, so the loop simply walks to the root.
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		homeDir = ""
	}

	// Resolve to an absolute path first: a relative start like "." has
	// filepath.Dir(".") == ".", which would trap the ascent in a single
	// directory and silently defeat the upward search.
	current, err := filepath.Abs(startDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve %q: %w", startDir, err)
	}

	for {
		for _, name := range names {
			candidate := filepath.Join(current, name)
			if _, statErr := os.Stat(candidate); statErr == nil {
				return current, candidate, nil
			}
		}

		if homeDir != "" && current == homeDir {
			break // never ascend above $HOME
		}
		parent := filepath.Dir(current)
		if parent == current {
			break // reached filesystem root
		}
		current = parent
	}

	return "", "", nil
}

// FindProjectConfig walks up from the current working directory to $HOME and
// returns the path to the nearest .gz-git config file, or "" if none exists.
func FindProjectConfig() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	_, path, err := findConfigUpward(cwd, projectConfigNames())
	if err != nil {
		return "", err
	}
	return path, nil // "" when not found (not an error)
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

// DetectConfigFile searches upward from the given directory to $HOME for a
// .gz-git config file and returns the full path to the nearest one. Searching
// upward (rather than the given directory alone) lets commands run from a
// workspace subdirectory resolve the workspace's existing config instead of
// re-initializing a second one in the wrong place. Returns an error when no
// config exists between dir and $HOME.
func DetectConfigFile(dir string) (string, error) {
	_, path, err := findConfigUpward(dir, projectConfigNames())
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("config file not found in %s or any parent up to $HOME (tried %s with extensions %v)",
			dir, ProjectConfigFileName, supportedExtensions)
	}
	return path, nil
}

// ConfigFileInfo holds detected config file information.
type ConfigFileInfo struct {
	Path string
	Kind ConfigKind
}

// DetectConfigFileWithKind probes a single directory (no parent walk, unlike
// DetectConfigFile) for a .gz-git config file, checking supported extensions in
// their defined order. Use it when the caller specifically wants "is there a
// config in exactly this directory".
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
	}
	return result
}
