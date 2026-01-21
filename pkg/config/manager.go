// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Manager handles CRUD operations for profiles and configurations.
type Manager struct {
	paths     *Paths
	validator *Validator
}

// NewManager creates a new configuration manager.
func NewManager() (*Manager, error) {
	paths, err := NewPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize paths: %w", err)
	}

	return &Manager{
		paths:     paths,
		validator: NewValidator(),
	}, nil
}

// Initialize creates the config directory structure with default profile.
func (m *Manager) Initialize() error {
	// Create directories
	if err := m.paths.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Create default profile if it doesn't exist
	if !m.paths.ProfileExists(DefaultProfileName) {
		defaultProfile := &Profile{
			Name:       DefaultProfileName,
			Parallel:   10,
			CloneProto: "ssh",
		}

		if err := m.SaveProfile(defaultProfile); err != nil {
			return fmt.Errorf("failed to create default profile: %w", err)
		}
	}

	// Create global config if it doesn't exist
	if _, err := os.Stat(m.paths.GlobalConfigFile); os.IsNotExist(err) {
		globalConfig := &GlobalConfig{
			ActiveProfile: DefaultProfileName,
			Defaults: map[string]interface{}{
				"parallel":   10,
				"cloneProto": "ssh",
			},
		}

		if err := m.SaveGlobalConfig(globalConfig); err != nil {
			return fmt.Errorf("failed to create global config: %w", err)
		}
	}

	return nil
}

// CreateProfile creates a new profile.
func (m *Manager) CreateProfile(profile *Profile) error {
	if profile == nil {
		return fmt.Errorf("profile is nil")
	}

	// Validate profile
	if err := m.validator.ValidateProfile(profile); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if profile already exists
	if m.paths.ProfileExists(profile.Name) {
		return fmt.Errorf("profile '%s' already exists", profile.Name)
	}

	// Save profile
	return m.SaveProfile(profile)
}

// SaveProfile saves a profile to disk.
func (m *Manager) SaveProfile(profile *Profile) error {
	if profile == nil {
		return fmt.Errorf("profile is nil")
	}

	// Validate profile
	if err := m.validator.ValidateProfile(profile); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Ensure profiles directory exists
	if err := os.MkdirAll(m.paths.ProfilesDir, 0o700); err != nil {
		return fmt.Errorf("failed to create profiles directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	// Write to file with restricted permissions (user read/write only)
	profilePath := m.paths.ProfilePath(profile.Name)
	if err := os.WriteFile(profilePath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write profile file: %w", err)
	}

	return nil
}

// LoadProfile loads a profile from disk.
func (m *Manager) LoadProfile(name string) (*Profile, error) {
	if name == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	profilePath := m.paths.ProfilePath(name)

	// Check if profile exists
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}

	// Read profile file
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	// Unmarshal YAML
	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile file: %w", err)
	}

	// Expand environment variables
	if err := m.validator.ExpandEnvVarsInProfile(&profile); err != nil {
		return nil, fmt.Errorf("failed to expand environment variables: %w", err)
	}

	// Validate profile
	if err := m.validator.ValidateProfile(&profile); err != nil {
		return nil, fmt.Errorf("profile validation failed: %w", err)
	}

	return &profile, nil
}

// DeleteProfile deletes a profile.
func (m *Manager) DeleteProfile(name string) error {
	if name == "" {
		return fmt.Errorf("profile name is required")
	}

	// Prevent deletion of default profile
	if name == DefaultProfileName {
		return fmt.Errorf("cannot delete default profile")
	}

	profilePath := m.paths.ProfilePath(name)

	// Check if profile exists
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return fmt.Errorf("profile '%s' not found", name)
	}

	// Delete file
	if err := os.Remove(profilePath); err != nil {
		return fmt.Errorf("failed to delete profile file: %w", err)
	}

	// If this was the active profile, reset to default
	activeProfile, _ := m.paths.GetActiveProfile()
	if activeProfile == name {
		if err := m.paths.SetActiveProfile(DefaultProfileName); err != nil {
			// Log warning but don't fail
			fmt.Fprintf(os.Stderr, "Warning: failed to reset active profile to default: %v\n", err)
		}
	}

	return nil
}

// ListProfiles returns all available profile names.
func (m *Manager) ListProfiles() ([]string, error) {
	return m.paths.ListProfiles()
}

// ProfileExists checks if a profile exists.
func (m *Manager) ProfileExists(name string) bool {
	return m.paths.ProfileExists(name)
}

// GetActiveProfile returns the active profile name.
func (m *Manager) GetActiveProfile() (string, error) {
	profile, err := m.paths.GetActiveProfile()
	if err != nil {
		return "", err
	}

	// Default to "default" if not set
	if profile == "" {
		profile = DefaultProfileName
	}

	return profile, nil
}

// SetActiveProfile sets the active profile.
func (m *Manager) SetActiveProfile(name string) error {
	if name == "" {
		return fmt.Errorf("profile name is required")
	}

	// Check if profile exists
	if !m.paths.ProfileExists(name) {
		return fmt.Errorf("profile '%s' not found", name)
	}

	return m.paths.SetActiveProfile(name)
}

// SaveGlobalConfig saves the global configuration.
func (m *Manager) SaveGlobalConfig(config *GlobalConfig) error {
	if config == nil {
		return fmt.Errorf("global config is nil")
	}

	// Validate global config
	if err := m.validator.ValidateGlobalConfig(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Ensure config directory exists
	if err := os.MkdirAll(m.paths.ConfigDir, 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal global config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(m.paths.GlobalConfigFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write global config file: %w", err)
	}

	return nil
}

// LoadGlobalConfig loads the global configuration.
func (m *Manager) LoadGlobalConfig() (*GlobalConfig, error) {
	// Check if config file exists
	if _, err := os.Stat(m.paths.GlobalConfigFile); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return &GlobalConfig{
			ActiveProfile: DefaultProfileName,
			Defaults: map[string]interface{}{
				"parallel":   10,
				"cloneProto": "ssh",
			},
		}, nil
	}

	// Read config file
	data, err := os.ReadFile(m.paths.GlobalConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read global config file: %w", err)
	}

	// Unmarshal YAML
	var config GlobalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse global config file: %w", err)
	}

	// Expand environment variables
	if err := m.validator.ExpandEnvVarsInGlobalConfig(&config); err != nil {
		return nil, fmt.Errorf("failed to expand environment variables: %w", err)
	}

	// Validate config
	if err := m.validator.ValidateGlobalConfig(&config); err != nil {
		return nil, fmt.Errorf("global config validation failed: %w", err)
	}

	return &config, nil
}

// LoadProjectConfig loads project-specific configuration.
func (m *Manager) LoadProjectConfig() (*ProjectConfig, error) {
	// Find project config file
	configPath, err := FindProjectConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to find project config: %w", err)
	}

	// No project config found (not an error)
	if configPath == "" {
		return nil, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project config file: %w", err)
	}

	// Unmarshal YAML
	var config ProjectConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse project config file: %w", err)
	}

	// Validate config
	if err := m.validator.ValidateProjectConfig(&config); err != nil {
		return nil, fmt.Errorf("project config validation failed: %w", err)
	}

	return &config, nil
}

// ================================================================================
// Recursive Hierarchical Configuration (NEW!)
// ================================================================================

// LoadConfigRecursiveFromPath loads a recursive config from the specified path.
// This is a wrapper around LoadConfigRecursive with manager validation.
func (m *Manager) LoadConfigRecursiveFromPath(path string, configFile string) (*Config, error) {
	// Use the standalone LoadConfigRecursive function
	config, err := LoadConfigRecursive(path, configFile)
	if err != nil {
		return nil, err
	}

	// Validate the loaded config
	if err := m.validator.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// SaveConfig saves a recursive config to the specified path.
func (m *Manager) SaveConfig(path string, configFile string, config *Config) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate config
	if err := m.validator.ValidateConfig(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file
	configPath := fmt.Sprintf("%s/%s", path, configFile)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadWorkstationConfig loads the workstation-level config (~/.gz-git-config.yaml).
func (m *Manager) FindNearestConfig(configFile string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	return FindConfigRecursive(cwd, configFile)
}

// SaveProjectConfig saves project configuration to current directory.
func (m *Manager) SaveProjectConfig(config *ProjectConfig) error {
	if config == nil {
		return fmt.Errorf("project config is nil")
	}

	// Validate config
	if err := m.validator.ValidateProjectConfig(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}

	// Write to current directory
	configPath := fmt.Sprintf("%s/%s", cwd, ProjectConfigFileName)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write project config file: %w", err)
	}

	return nil
}
