// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// SpecLoader loads sync specifications from various sources.
type SpecLoader interface {
	Load(ctx context.Context, path string) (*ConfigData, error)
}

// ConfigData holds loaded configuration.
type ConfigData struct {
	Plan reposync.PlanRequest
	Run  reposync.RunOptions
}

// FileSpecLoader loads specs from YAML files.
type FileSpecLoader struct{}

// Load reads and parses a YAML config file.
func (l FileSpecLoader) Load(ctx context.Context, path string) (*ConfigData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var raw struct {
		// Meta information
		Version  int              `yaml:"version,omitempty"`
		Kind     string           `yaml:"kind,omitempty"`
		Metadata *config.Metadata `yaml:"metadata,omitempty"`

		// Sync settings
		Strategy       string   `yaml:"strategy"`
		Parallel       int      `yaml:"parallel"`
		MaxRetries     int      `yaml:"maxRetries"`
		CleanupOrphans bool     `yaml:"cleanupOrphans"`
		CloneProto     string   `yaml:"cloneProto"`
		SSHPort        int      `yaml:"sshPort"`
		Roots          []string `yaml:"roots"`
		Repositories   []struct {
			Name              string            `yaml:"name"`
			Description       string            `yaml:"description"` // optional: human-readable description
			URL               string            `yaml:"url"`
			AdditionalRemotes map[string]string `yaml:"additionalRemotes"` // Additional git remotes (name: url)
			Path              string            `yaml:"path"`
			Strategy          string            `yaml:"strategy"`
			CloneProto        string            `yaml:"cloneProto"`
			Enabled           *bool             `yaml:"enabled"`       // optional: if false, exclude from sync (default: true)
			AssumePresent     bool              `yaml:"assumePresent"` // if true, skip clone check
		} `yaml:"repositories"`
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	// Parse default strategy
	defaultStrategy := reposync.StrategyReset
	if raw.Strategy != "" {
		parsed, err := reposync.ParseStrategy(raw.Strategy)
		if err != nil {
			return nil, err
		}
		defaultStrategy = parsed
	}

	// Build repo specs
	repos := make([]reposync.RepoSpec, 0, len(raw.Repositories))
	for i, r := range raw.Repositories {
		// URL is always required
		if r.URL == "" {
			return nil, fmt.Errorf("repository[%d]: missing URL", i)
		}

		// Extract name from URL if not specified
		repoName := r.Name
		if repoName == "" {
			extracted, err := repository.ExtractRepoNameFromURL(r.URL)
			if err != nil {
				return nil, fmt.Errorf("repository[%d]: cannot extract name from URL %q: %w", i, r.URL, err)
			}
			repoName = extracted
		}

		// Default path to repo name if not specified
		path := r.Path
		if path == "" {
			path = repoName
		}

		spec := reposync.RepoSpec{
			Name:              repoName,
			Description:       r.Description,
			CloneURL:          r.URL,
			AdditionalRemotes: r.AdditionalRemotes,
			TargetPath:        path,
			Enabled:           r.Enabled,
			AssumePresent:     r.AssumePresent,
		}

		// Per-repo strategy override
		if r.Strategy != "" {
			parsed, err := reposync.ParseStrategy(r.Strategy)
			if err != nil {
				return nil, fmt.Errorf("repo %s: %w", repoName, err)
			}
			spec.Strategy = parsed
		}

		repos = append(repos, spec)
	}

	// Build result
	result := &ConfigData{
		Plan: reposync.PlanRequest{
			Input: reposync.PlanInput{
				Repos: repos,
			},
			Options: reposync.PlanOptions{
				Roots:           raw.Roots,
				DefaultStrategy: defaultStrategy,
				CleanupOrphans:  raw.CleanupOrphans,
			},
		},
		Run: reposync.RunOptions{
			Parallel:   raw.Parallel,
			MaxRetries: raw.MaxRetries,
		},
	}

	// Set defaults
	if result.Run.Parallel == 0 {
		result.Run.Parallel = 10 // Default: 10 parallel workers (industry standard)
	}
	if result.Run.MaxRetries == 0 {
		result.Run.MaxRetries = 3
	}

	return result, nil
}

// detectConfigFile searches for config files in the given directory.
// Priority: .gz-git.yaml > .gz-git.yml
// This is a wrapper around config.DetectConfigFile for backward compatibility.
func detectConfigFile(dir string) (string, error) {
	return config.DetectConfigFile(dir)
}
