// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	GitHub GitHubConfig `yaml:"github"`
	GitLab GitLabConfig `yaml:"gitlab"`
	Gitea  GiteaConfig  `yaml:"gitea"`
	Sync   SyncConfig   `yaml:"sync"`
}

// GitHubConfig holds GitHub-specific configuration.
type GitHubConfig struct {
	Token   string `yaml:"token"`
	BaseURL string `yaml:"base_url"` // For GitHub Enterprise
}

// GitLabConfig holds GitLab-specific configuration.
type GitLabConfig struct {
	Token   string `yaml:"token"`
	BaseURL string `yaml:"base_url"`
}

// GiteaConfig holds Gitea-specific configuration.
type GiteaConfig struct {
	Token   string `yaml:"token"`
	BaseURL string `yaml:"base_url"`
}

// SyncConfig holds sync operation defaults.
type SyncConfig struct {
	TargetPath      string `yaml:"target_path"`
	Parallel        int    `yaml:"parallel"`
	IncludeArchived bool   `yaml:"include_archived"`
	IncludeForks    bool   `yaml:"include_forks"`
	IncludePrivate  bool   `yaml:"include_private"`
}

// DefaultConfig returns a config with default values.
func DefaultConfig() *Config {
	return &Config{
		Sync: SyncConfig{
			TargetPath:      ".",
			Parallel:        4,
			IncludeArchived: false,
			IncludeForks:    false,
			IncludePrivate:  true,
		},
	}
}

// Load loads configuration from file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables
	cfg.applyEnvOverrides()

	return cfg, nil
}

// LoadDefault loads configuration from default locations.
func LoadDefault() (*Config, error) {
	// Try locations in order
	locations := []string{
		"forge.yaml",
		".forge.yaml",
		filepath.Join(os.Getenv("HOME"), ".config", "gzh-forge", "config.yaml"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return Load(loc)
		}
	}

	// Return default config if no file found
	cfg := DefaultConfig()
	cfg.applyEnvOverrides()
	return cfg, nil
}

func (c *Config) applyEnvOverrides() {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		c.GitHub.Token = token
	}
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		c.GitLab.Token = token
	}
	if token := os.Getenv("GITEA_TOKEN"); token != "" {
		c.Gitea.Token = token
	}
}
