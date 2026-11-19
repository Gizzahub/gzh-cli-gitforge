// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// ConfigData contains plan + run inputs loaded from a config file.
type ConfigData struct {
	Plan reposync.PlanRequest
	Run  reposync.RunOptions
}

// SpecLoader loads repository specs from a source (e.g., YAML file).
type SpecLoader interface {
	Load(ctx context.Context, path string) (ConfigData, error)
}

// FileSpecLoader loads configuration from a YAML file on disk.
type FileSpecLoader struct {
	// Optional defaults if the file omits values.
	DefaultStrategy reposync.Strategy
	DefaultParallel int
	DefaultRetries  int
}

// fileConfig is the reposync adapter's view of a flat repositories document:
// the shared neutral schema plus the parent/profile references that drive
// this loader's inheritance and stay outside the shared flat schema.
type fileConfig struct {
	// Flat repositories schema, decoded by config.ParseFlatRepositories.
	config.FlatRepositories `yaml:",inline"`

	// Parent config reference (for inheritance)
	Parent  string `yaml:"parent,omitempty"`  // path to parent config file
	Profile string `yaml:"profile,omitempty"` // profile name to use
}

type gzhYamlConfig struct {
	Provider     string        `yaml:"provider"`
	SyncMode     gzhYamlMode   `yaml:"sync_mode"` //nolint:tagliatelle // matches existing gzh.yaml format
	Repositories []gzhYamlRepo `yaml:"repositories"`
}

type gzhYamlMode struct {
	CleanupOrphans bool `yaml:"cleanup_orphans"` //nolint:tagliatelle // matches existing gzh.yaml format
}

type gzhYamlRepo struct {
	Name     string `yaml:"name"`
	CloneURL string `yaml:"clone_url"` //nolint:tagliatelle // matches existing gzh.yaml format
}

// Load implements SpecLoader.
func (l FileSpecLoader) Load(ctx context.Context, path string) (ConfigData, error) { //nolint:gocognit,gocyclo // multi-format config loading with parent inheritance — complexity is inherent
	if path == "" {
		return ConfigData{}, errors.New("config path is required")
	}

	configPath := cleanPath(path)

	raw, err := config.ReadConfigFile(configPath)
	if err != nil {
		return ConfigData{}, fmt.Errorf("read config: %w", err)
	}

	return l.loadData(ctx, raw, configPath)
}

func (l FileSpecLoader) loadData(ctx context.Context, raw []byte, configPath string) (ConfigData, error) { //nolint:gocognit,gocyclo // multi-format config loading with parent inheritance — complexity is inherent
	// Determine config kind: explicit kind field > filename > content detection
	kind := detectConfigKind(raw, configPath)

	// Route to appropriate loader based on kind
	if kind == config.KindWorkspace {
		return l.loadWorkspacesConfig(ctx, raw, configPath)
	}

	if isGzhYaml(raw) {
		return l.loadGzhYaml(raw, configPath)
	}

	// Shared flat-schema decode; URL presence for this document's entries is
	// validated by the parser.
	flat, err := config.ParseFlatRepositories(raw)
	if err != nil {
		return ConfigData{}, err
	}

	// Parent/profile references drive reposync-specific inheritance and are
	// decoded separately from the shared flat schema.
	var refs struct {
		Parent  string `yaml:"parent"`
		Profile string `yaml:"profile"`
	}
	if err := yaml.Unmarshal(raw, &refs); err != nil {
		return ConfigData{}, fmt.Errorf("parse config: %w", err)
	}
	cfg := fileConfig{FlatRepositories: flat, Parent: refs.Parent, Profile: refs.Profile}

	// Load and merge parent config if specified
	if cfg.Parent != "" {
		parentPath := resolveParentPath(cfg.Parent, configPath)
		parentCfg, err := l.loadParentConfig(parentPath)
		if err != nil {
			// Log warning but continue - parent might not exist yet
			fmt.Fprintf(os.Stderr, "Warning: failed to load parent config %s: %v\n", parentPath, err)
		} else {
			// Merge: child overrides parent
			cfg = mergeFileConfigs(parentCfg, cfg)
		}
	}

	if len(cfg.Repositories) == 0 {
		return ConfigData{}, errors.New("config has no repositories")
	}

	if cfg.CleanupOrphans != nil && *cfg.CleanupOrphans && len(cfg.Roots) == 0 {
		return ConfigData{}, errors.New("cleanupOrphans enabled but no roots provided")
	}

	defaultStrategy := l.DefaultStrategy
	if defaultStrategy == "" {
		defaultStrategy = reposync.StrategyReset
	}

	defaultParallel := l.DefaultParallel
	if defaultParallel <= 0 {
		defaultParallel = repository.DefaultLocalParallel
	}

	defaultRetries := l.DefaultRetries
	if defaultRetries <= 0 {
		defaultRetries = 1
	}

	parsedStrategy, err := reposync.ParseStrategy(cfg.Strategy)
	if err != nil {
		return ConfigData{}, err
	}
	if cfg.Strategy == "" {
		parsedStrategy = defaultStrategy
	}

	plan := reposync.PlanRequest{
		Input: reposync.PlanInput{
			Repos: make([]reposync.RepoSpec, 0, len(cfg.Repositories)),
		},
		Options: reposync.PlanOptions{
			DefaultStrategy: parsedStrategy,
			CleanupOrphans:  cfg.CleanupOrphans != nil && *cfg.CleanupOrphans,
			Roots:           cleanRoots(cfg.Roots),
		},
	}

	seenTargets := make(map[string]struct{}, len(cfg.Repositories))

	for i, repo := range cfg.Repositories {
		// URL is always required. The shared parser already validated this
		// document's entries; the check also covers repositories merged in
		// from a parent config, which bypass the shared parser.
		if repo.URL == "" {
			return ConfigData{}, fmt.Errorf("repository[%d]: missing URL", i)
		}

		// Extract name from URL if not specified
		repoName := repo.Name
		if repoName == "" {
			extracted, err := repository.ExtractRepoNameFromURL(repo.URL)
			if err != nil {
				return ConfigData{}, fmt.Errorf("repository[%d]: cannot extract name from URL %q: %w", i, repo.URL, err)
			}
			repoName = extracted
		}

		// Default path to repo name if not specified
		targetPath := repo.Path
		if targetPath == "" {
			targetPath = repoName
		}

		repoStrategy := parsedStrategy
		if repo.Strategy != "" {
			repoStrategy, err = reposync.ParseStrategy(repo.Strategy)
			if err != nil {
				return ConfigData{}, fmt.Errorf("repository %s: %w", repoName, err)
			}
		}

		targetPath = cleanPath(targetPath)
		if _, exists := seenTargets[targetPath]; exists {
			return ConfigData{}, fmt.Errorf("duplicate path detected: %s", targetPath)
		}
		seenTargets[targetPath] = struct{}{}

		// Determine strictBranchCheckout: use per-repo override if set, otherwise global
		strictBranchCheckout := cfg.StrictBranchCheckout
		if repo.StrictBranchCheckout != nil {
			strictBranchCheckout = *repo.StrictBranchCheckout
		}

		plan.Input.Repos = append(plan.Input.Repos, reposync.RepoSpec{
			Name:                 repoName,
			Description:          repo.Description,
			Provider:             repo.Provider,
			CloneURL:             repo.URL,
			AdditionalRemotes:    repo.AdditionalRemotes,
			TargetPath:           targetPath,
			Branch:               string(repo.Branch),
			StrictBranchCheckout: strictBranchCheckout,
			Strategy:             repoStrategy,
			Enabled:              repo.Enabled,
			AssumePresent:        repo.AssumePresent,
		})
	}

	run := reposync.RunOptions{
		MaxRetries: defaultRetries,
		Resume:     cfg.Resume,
		DryRun:     cfg.DryRun,
	}
	if cfg.Parallel != nil {
		run.Parallel = *cfg.Parallel
	}
	if cfg.MaxRetries != nil {
		run.MaxRetries = *cfg.MaxRetries
	}

	if run.Parallel <= 0 {
		run.Parallel = defaultParallel
	}
	if run.MaxRetries < 0 {
		return ConfigData{}, fmt.Errorf("maxRetries must be >= 0 (got %d)", run.MaxRetries)
	}

	return ConfigData{
		Plan: plan,
		Run:  run,
	}, nil
}

// detectConfigKind determines the config type using priority:
// 1. Explicit "kind" field in YAML
// 2. Content detection (workspaces/profiles keys → workspace)
// 3. Default to repositories.
func detectConfigKind(raw []byte, _ string) config.ConfigKind {
	var meta struct {
		Kind config.ConfigKind `yaml:"kind"`
	}
	if err := yaml.Unmarshal(raw, &meta); err == nil && meta.Kind.IsValid() {
		return meta.Kind
	}

	// Content-based detection
	if hasWorkspaceKeys(raw) {
		return config.KindWorkspace
	}

	return config.KindRepositories
}

// hasWorkspaceKeys checks if the YAML content has workspace-specific keys.
func hasWorkspaceKeys(raw []byte) bool {
	var root map[string]any
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return false
	}

	// Check for workspaces key (new hierarchical config)
	if _, ok := root["workspaces"]; ok {
		return true
	}
	// Check for profiles key (inline profiles)
	if _, ok := root["profiles"]; ok {
		return true
	}
	return false
}

func isGzhYaml(raw []byte) bool {
	var root map[string]any
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return false
	}

	if _, ok := root["sync_mode"]; ok {
		return true
	}
	if _, ok := root["organization"]; ok {
		return true
	}
	if _, ok := root["generated_at"]; ok {
		return true
	}

	repos, ok := root["repositories"].([]any)
	if !ok {
		return false
	}
	for _, entry := range repos {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := m["clone_url"]; ok {
			return true
		}
	}
	return false
}

func (l FileSpecLoader) loadGzhYaml(raw []byte, path string) (ConfigData, error) {
	var cfg gzhYamlConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return ConfigData{}, fmt.Errorf("parse gzh.yaml: %w", err)
	}
	if len(cfg.Repositories) == 0 {
		return ConfigData{}, errors.New("gzh.yaml has no repositories")
	}

	defaultStrategy := l.DefaultStrategy
	if defaultStrategy == "" {
		defaultStrategy = reposync.StrategyReset
	}

	defaultParallel := l.DefaultParallel
	if defaultParallel <= 0 {
		defaultParallel = repository.DefaultLocalParallel
	}

	defaultRetries := l.DefaultRetries
	if defaultRetries <= 0 {
		defaultRetries = 1
	}

	root := cleanPath(filepath.Dir(path))

	plan := reposync.PlanRequest{
		Input: reposync.PlanInput{
			Repos: make([]reposync.RepoSpec, 0, len(cfg.Repositories)),
		},
		Options: reposync.PlanOptions{
			DefaultStrategy: defaultStrategy,
			CleanupOrphans:  cfg.SyncMode.CleanupOrphans,
			Roots:           []string{root},
		},
	}

	seenTargets := make(map[string]struct{}, len(cfg.Repositories))
	for _, repo := range cfg.Repositories {
		if repo.Name == "" || repo.CloneURL == "" {
			return ConfigData{}, errors.New("gzh.yaml repository entry is missing required fields (name/clone_url)")
		}

		targetPath := cleanPath(filepath.Join(root, repo.Name))
		if _, exists := seenTargets[targetPath]; exists {
			return ConfigData{}, fmt.Errorf("duplicate path detected: %s", targetPath)
		}
		seenTargets[targetPath] = struct{}{}

		plan.Input.Repos = append(plan.Input.Repos, reposync.RepoSpec{
			Name:       repo.Name,
			Provider:   cfg.Provider,
			CloneURL:   repo.CloneURL,
			TargetPath: targetPath,
			Strategy:   defaultStrategy,
		})
	}

	run := reposync.RunOptions{
		Parallel:   defaultParallel,
		MaxRetries: defaultRetries,
	}

	return ConfigData{
		Plan: plan,
		Run:  run,
	}, nil
}

func cleanPath(path string) string {
	if path == "" {
		return path
	}
	expanded := os.ExpandEnv(path)
	if expanded == "~" || strings.HasPrefix(expanded, "~/") || strings.HasPrefix(expanded, "~\\") {
		if home, err := os.UserHomeDir(); err == nil {
			rest := strings.TrimPrefix(expanded[1:], "/")
			rest = strings.TrimPrefix(rest, "\\")
			expanded = filepath.Join(home, rest)
		}
	}
	return filepath.Clean(expanded)
}

func cleanRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		if root == "" {
			continue
		}
		out = append(out, cleanPath(root))
	}
	return out
}

// detectConfigFile searches upward from the given directory to $HOME for a
// .gz-git config file (priority: .gz-git.yaml > .gz-git.yml > .gz-git.json).
// This is a thin wrapper around config.DetectConfigFile.
func detectConfigFile(dir string) (string, error) {
	return config.DetectConfigFile(dir)
}

// resolveParentPath resolves a parent config path relative to the current config.
// Supports: absolute paths, ~/relative paths, and relative paths.
func resolveParentPath(parentRef, configPath string) string {
	if parentRef == "" {
		return ""
	}

	// Expand environment variables
	parentRef = os.ExpandEnv(parentRef)

	// Handle ~ (home directory)
	if parentRef == "~" || strings.HasPrefix(parentRef, "~/") || strings.HasPrefix(parentRef, "~\\") {
		if home, err := os.UserHomeDir(); err == nil {
			rest := strings.TrimPrefix(parentRef[1:], "/")
			rest = strings.TrimPrefix(rest, "\\")
			parentRef = filepath.Join(home, rest)
		}
	}

	// If absolute, use as-is
	if filepath.IsAbs(parentRef) {
		return filepath.Clean(parentRef)
	}

	// Relative path: resolve relative to config file's directory
	configDir := filepath.Dir(configPath)
	return filepath.Clean(filepath.Join(configDir, parentRef))
}

// loadParentConfig loads a parent config file and returns its parsed fileConfig.
func (l FileSpecLoader) loadParentConfig(parentPath string) (fileConfig, error) {
	raw, err := config.ReadConfigFile(parentPath)
	if err != nil {
		return fileConfig{}, fmt.Errorf("read parent config %s: %w", parentPath, err)
	}

	var cfg fileConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return fileConfig{}, fmt.Errorf("parse parent config %s: %w", parentPath, err)
	}

	// Recursively load parent's parent if specified
	if cfg.Parent != "" {
		grandparentPath := resolveParentPath(cfg.Parent, parentPath)
		grandparentCfg, err := l.loadParentConfig(grandparentPath)
		if err != nil {
			// Log warning but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to load grandparent config %s: %v\n", grandparentPath, err)
		} else {
			// Merge grandparent into parent (parent overrides grandparent)
			cfg = mergeFileConfigs(grandparentCfg, cfg)
		}
	}

	return cfg, nil
}

// mergeFileConfigs merges parent config into child config.
// Child values override parent values (child takes precedence).
func mergeFileConfigs(parent, child fileConfig) fileConfig {
	result := parent // Start with parent as base

	// Child meta fields override if set
	if child.Version != 0 {
		result.Version = child.Version
	}
	if child.Kind != "" {
		result.Kind = child.Kind
	}
	if child.Metadata != nil {
		result.Metadata = child.Metadata
	}

	// Parent reference is NOT inherited (child's parent is its own declaration)
	result.Parent = child.Parent
	if child.Profile != "" {
		result.Profile = child.Profile
	}

	// Sync settings: child overrides if set
	if child.Strategy != "" {
		result.Strategy = child.Strategy
	}
	// An explicit child zero keeps the parent value, matching the legacy
	// int semantics this merge was written against.
	if child.Parallel != nil && *child.Parallel != 0 {
		result.Parallel = child.Parallel
	}
	if child.MaxRetries != nil {
		result.MaxRetries = child.MaxRetries
	}
	if child.Resume {
		result.Resume = child.Resume
	}
	if child.DryRun {
		result.DryRun = child.DryRun
	}
	if child.CleanupOrphans != nil && *child.CleanupOrphans {
		result.CleanupOrphans = child.CleanupOrphans
	}
	if child.StrictBranchCheckout {
		result.StrictBranchCheckout = child.StrictBranchCheckout
	}

	// Roots: child replaces if set (not merged)
	if len(child.Roots) > 0 {
		result.Roots = child.Roots
	}

	// Repositories: child replaces entirely (not merged)
	// This is intentional: child config defines its own repo list
	if len(child.Repositories) > 0 {
		result.Repositories = child.Repositories
	}

	return result
}
