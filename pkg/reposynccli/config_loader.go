// Copyright (c) 2025 Archmagece
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
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitea"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/github"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitlab"
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

type fileConfig struct {
	Strategy             string      `yaml:"strategy"`
	Parallel             int         `yaml:"parallel"`
	MaxRetries           *int        `yaml:"maxRetries"`
	Resume               bool        `yaml:"resume"`
	DryRun               bool        `yaml:"dryRun"`
	CleanupOrphans       bool        `yaml:"cleanupOrphans"`
	StrictBranchCheckout bool        `yaml:"strictBranchCheckout"` // default: false (lenient)
	Roots                []string    `yaml:"roots"`
	Repositories         []repoEntry `yaml:"repositories"`
}

type repoEntry struct {
	Name                 string            `yaml:"name"`
	Description          string            `yaml:"description"` // optional: human-readable description
	Provider             string            `yaml:"provider"`
	URL                  string            `yaml:"url"`
	AdditionalRemotes    map[string]string `yaml:"additionalRemotes"` // Additional git remotes (name: url)
	TargetPath           string            `yaml:"targetPath"`
	Branch               string            `yaml:"branch"`               // optional: branch to checkout after clone/update
	StrictBranchCheckout *bool             `yaml:"strictBranchCheckout"` // optional: override global setting (nil = use global)
	Strategy             string            `yaml:"strategy"`
	AssumePresent        bool              `yaml:"assumePresent"`
}

type gzhYamlConfig struct {
	Provider     string        `yaml:"provider"`
	SyncMode     gzhYamlMode   `yaml:"sync_mode"`
	Repositories []gzhYamlRepo `yaml:"repositories"`
}

type gzhYamlMode struct {
	CleanupOrphans bool `yaml:"cleanup_orphans"`
}

type gzhYamlRepo struct {
	Name     string `yaml:"name"`
	CloneURL string `yaml:"clone_url"`
}

// workspacesConfig represents the new hierarchical config format with workspaces
type workspacesConfig struct {
	Parent     string                     `yaml:"parent"`
	Profile    string                     `yaml:"profile"`
	Parallel   int                        `yaml:"parallel"`
	CloneProto string                     `yaml:"cloneProto"`
	SSHPort    int                        `yaml:"sshPort"`
	Workspaces map[string]*workspaceEntry `yaml:"workspaces"`
	Sync       *syncSettings              `yaml:"sync"`
	Profiles   map[string]*profileEntry   `yaml:"profiles"`
}

type workspaceEntry struct {
	Path       string        `yaml:"path"`
	Type       string        `yaml:"type"` // forge, git, config
	Profile    string        `yaml:"profile"`
	Source     *forgeSource  `yaml:"source"`
	Sync       *syncSettings `yaml:"sync"`
	Parallel   int           `yaml:"parallel"`
	CloneProto string        `yaml:"cloneProto"`
	SSHPort    int           `yaml:"sshPort"`
}

type forgeSource struct {
	Provider         string `yaml:"provider"` // gitlab, github, gitea
	Org              string `yaml:"org"`
	BaseURL          string `yaml:"baseURL"`
	Token            string `yaml:"token"`
	IncludeSubgroups bool   `yaml:"includeSubgroups"`
	SubgroupMode     string `yaml:"subgroupMode"`  // flat, nested
	FlatSeparator    string `yaml:"flatSeparator"` // separator for flat mode (default: "-")
}

type syncSettings struct {
	Strategy   string `yaml:"strategy"`
	MaxRetries int    `yaml:"maxRetries"`
}

type profileEntry struct {
	Name             string        `yaml:"name"`
	Provider         string        `yaml:"provider"`
	BaseURL          string        `yaml:"baseURL"`
	Token            string        `yaml:"token"`
	CloneProto       string        `yaml:"cloneProto"`
	SSHPort          int           `yaml:"sshPort"`
	Parallel         int           `yaml:"parallel"`
	IncludeSubgroups bool          `yaml:"includeSubgroups"`
	SubgroupMode     string        `yaml:"subgroupMode"`
	FlatSeparator    string        `yaml:"flatSeparator"` // separator for flat mode (default: "-")
	Sync             *syncSettings `yaml:"sync"`
}

// Load implements SpecLoader.
func (l FileSpecLoader) Load(_ context.Context, path string) (ConfigData, error) {
	if path == "" {
		return ConfigData{}, errors.New("config path is required")
	}

	configPath := cleanPath(path)

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return ConfigData{}, fmt.Errorf("read config: %w", err)
	}

	// Check for workspaces format first (new hierarchical config)
	if isWorkspacesConfig(raw) {
		return l.loadWorkspacesConfig(raw, configPath)
	}

	if isGzhYaml(raw) {
		return l.loadGzhYaml(raw, configPath)
	}

	var cfg fileConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return ConfigData{}, fmt.Errorf("parse config: %w", err)
	}

	if len(cfg.Repositories) == 0 {
		return ConfigData{}, errors.New("config has no repositories")
	}

	if cfg.CleanupOrphans && len(cfg.Roots) == 0 {
		return ConfigData{}, errors.New("cleanupOrphans enabled but no roots provided")
	}

	defaultStrategy := l.DefaultStrategy
	if defaultStrategy == "" {
		defaultStrategy = reposync.StrategyReset
	}

	defaultParallel := l.DefaultParallel
	if defaultParallel <= 0 {
		defaultParallel = 10
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
			CleanupOrphans:  cfg.CleanupOrphans,
			Roots:           cleanRoots(cfg.Roots),
		},
	}

	seenTargets := make(map[string]struct{}, len(cfg.Repositories))

	for _, repo := range cfg.Repositories {
		// Default path to repo name if not specified
		targetPath := repo.TargetPath
		if targetPath == "" {
			targetPath = repo.Name
		}

		if repo.Name == "" || repo.URL == "" {
			return ConfigData{}, fmt.Errorf("repository entry is missing required fields (name/url)")
		}

		repoStrategy := parsedStrategy
		if repo.Strategy != "" {
			repoStrategy, err = reposync.ParseStrategy(repo.Strategy)
			if err != nil {
				return ConfigData{}, fmt.Errorf("repository %s: %w", repo.Name, err)
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
			Name:                 repo.Name,
			Description:          repo.Description,
			Provider:             repo.Provider,
			CloneURL:             repo.URL,
			AdditionalRemotes:    repo.AdditionalRemotes,
			TargetPath:           targetPath,
			Branch:               repo.Branch,
			StrictBranchCheckout: strictBranchCheckout,
			Strategy:             repoStrategy,
			AssumePresent:        repo.AssumePresent,
		})
	}

	run := reposync.RunOptions{
		Parallel:   cfg.Parallel,
		MaxRetries: defaultRetries,
		Resume:     cfg.Resume,
		DryRun:     cfg.DryRun,
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

func isWorkspacesConfig(raw []byte) bool {
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
		defaultParallel = 10
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

// loadWorkspacesConfig handles the new hierarchical config format with workspaces.
// Supports three workspace types:
//   - type: forge  → fetch repos from forge API (GitHub/GitLab/Gitea)
//   - type: config → recursively load nested .gz-git.yaml
//   - type: git    → scan directory for git repositories (default)
func (l FileSpecLoader) loadWorkspacesConfig(raw []byte, configPath string) (ConfigData, error) {
	var cfg workspacesConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return ConfigData{}, fmt.Errorf("parse workspaces config: %w", err)
	}

	if len(cfg.Workspaces) == 0 {
		return ConfigData{}, errors.New("config has no workspaces defined")
	}

	// Load and merge parent config profiles (if parent is specified)
	if err := mergeProfilesIntoConfig(&cfg, configPath); err != nil {
		// Log warning but continue - parent profiles are optional
		fmt.Fprintf(os.Stderr, "Warning: failed to merge parent profiles: %v\n", err)
	}

	// Build profile lookup map (inline profiles + inherited from parent)
	profiles := cfg.Profiles

	// Resolve active profile settings
	var activeProfile *profileEntry
	if cfg.Profile != "" && profiles != nil {
		activeProfile = profiles[cfg.Profile]
	}

	// Default settings
	defaultStrategy := l.DefaultStrategy
	if defaultStrategy == "" {
		defaultStrategy = reposync.StrategyPull
	}
	if cfg.Sync != nil && cfg.Sync.Strategy != "" {
		if parsed, err := reposync.ParseStrategy(cfg.Sync.Strategy); err == nil {
			defaultStrategy = parsed
		}
	}

	defaultParallel := l.DefaultParallel
	if defaultParallel <= 0 {
		defaultParallel = 10
	}
	if cfg.Parallel > 0 {
		defaultParallel = cfg.Parallel
	}

	defaultRetries := l.DefaultRetries
	if defaultRetries <= 0 {
		defaultRetries = 1
	}
	if cfg.Sync != nil && cfg.Sync.MaxRetries > 0 {
		defaultRetries = cfg.Sync.MaxRetries
	}

	defaultCloneProto := cfg.CloneProto
	if defaultCloneProto == "" {
		defaultCloneProto = "ssh"
	}

	defaultSSHPort := cfg.SSHPort

	plan := reposync.PlanRequest{
		Input: reposync.PlanInput{
			Repos: make([]reposync.RepoSpec, 0),
		},
		Options: reposync.PlanOptions{
			DefaultStrategy: defaultStrategy,
		},
	}

	var roots []string
	seenTargets := make(map[string]struct{})
	configDir := filepath.Dir(configPath)

	// Process each workspace
	for name, ws := range cfg.Workspaces {
		if ws == nil {
			continue
		}

		// Resolve workspace path (relative to config file location)
		wsPath := ws.Path
		if wsPath == "" {
			wsPath = "."
		}
		if wsPath == "." {
			wsPath = configDir
		} else if !filepath.IsAbs(wsPath) && !strings.HasPrefix(wsPath, "~") {
			wsPath = filepath.Join(configDir, wsPath)
		}
		wsPath = cleanPath(wsPath)
		roots = append(roots, wsPath)

		// Determine workspace strategy
		wsStrategy := defaultStrategy
		if ws.Sync != nil && ws.Sync.Strategy != "" {
			if parsed, err := reposync.ParseStrategy(ws.Sync.Strategy); err == nil {
				wsStrategy = parsed
			}
		}

		// Resolve workspace profile (inherit from parent or use own)
		wsProfile := activeProfile
		if ws.Profile != "" && profiles != nil {
			if p, ok := profiles[ws.Profile]; ok {
				wsProfile = p
			}
		}

		// Determine workspace type
		wsType := ws.Type
		if wsType == "" {
			if ws.Source != nil {
				wsType = "forge"
			} else {
				wsType = "git"
			}
		}

		var repos []reposync.RepoSpec
		var err error

		switch wsType {
		case "forge":
			repos, err = l.loadForgeWorkspace(name, ws, wsProfile, wsPath, defaultCloneProto, defaultSSHPort)
			if err != nil {
				// Log warning but continue with other workspaces
				fmt.Fprintf(os.Stderr, "Warning: workspace '%s' forge fetch failed: %v\n", name, err)
				continue
			}

		case "config":
			repos, err = l.loadConfigWorkspace(wsPath)
			if err != nil {
				// Config workspace might not exist yet
				continue
			}

		default: // "git" or unspecified
			repos, err = scanGitRepos(wsPath, name)
			if err != nil {
				// Workspace might not exist yet
				continue
			}
		}

		for _, repo := range repos {
			if _, exists := seenTargets[repo.TargetPath]; exists {
				continue
			}
			seenTargets[repo.TargetPath] = struct{}{}

			if repo.Strategy == "" {
				repo.Strategy = wsStrategy
			}
			plan.Input.Repos = append(plan.Input.Repos, repo)
		}
	}

	if len(plan.Input.Repos) == 0 {
		return ConfigData{}, errors.New("no git repositories found in workspaces")
	}

	plan.Options.Roots = roots

	run := reposync.RunOptions{
		Parallel:   defaultParallel,
		MaxRetries: defaultRetries,
	}

	return ConfigData{
		Plan: plan,
		Run:  run,
	}, nil
}

// loadForgeWorkspace fetches repos from a forge (GitHub/GitLab/Gitea).
func (l FileSpecLoader) loadForgeWorkspace(
	name string,
	ws *workspaceEntry,
	profile *profileEntry,
	targetPath string,
	defaultCloneProto string,
	defaultSSHPort int,
) ([]reposync.RepoSpec, error) {
	if ws.Source == nil {
		return nil, errors.New("forge workspace requires source configuration")
	}

	src := ws.Source

	// Resolve provider settings (workspace source > profile > defaults)
	provider := src.Provider
	if provider == "" && profile != nil {
		provider = profile.Provider
	}
	if provider == "" {
		return nil, errors.New("forge workspace requires provider")
	}

	org := src.Org
	if org == "" {
		return nil, errors.New("forge workspace requires org")
	}

	baseURL := src.BaseURL
	if baseURL == "" && profile != nil {
		baseURL = profile.BaseURL
	}

	token := expandEnvVar(src.Token)
	if token == "" && profile != nil {
		token = expandEnvVar(profile.Token)
	}

	cloneProto := ws.CloneProto
	if cloneProto == "" && profile != nil {
		cloneProto = profile.CloneProto
	}
	if cloneProto == "" {
		cloneProto = defaultCloneProto
	}

	sshPort := ws.SSHPort
	if sshPort == 0 && profile != nil {
		sshPort = profile.SSHPort
	}
	if sshPort == 0 {
		sshPort = defaultSSHPort
	}

	includeSubgroups := src.IncludeSubgroups
	if !includeSubgroups && profile != nil {
		includeSubgroups = profile.IncludeSubgroups
	}

	subgroupMode := src.SubgroupMode
	if subgroupMode == "" && profile != nil {
		subgroupMode = profile.SubgroupMode
	}
	if subgroupMode == "" {
		subgroupMode = "flat"
	}

	flatSeparator := src.FlatSeparator
	if flatSeparator == "" && profile != nil {
		flatSeparator = profile.FlatSeparator
	}
	// Default separator is "-" (handled in buildTargetPath)

	// Create ForgePlanner and fetch repos
	forgeProvider, err := createForgeProvider(provider, token, baseURL, sshPort)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	plannerConfig := reposync.ForgePlannerConfig{
		TargetPath:       targetPath,
		Organization:     org,
		CloneProto:       cloneProto,
		SSHPort:          sshPort,
		IncludeSubgroups: includeSubgroups,
		SubgroupMode:     subgroupMode,
		FlatSeparator:    flatSeparator,
		IncludePrivate:   true,
	}

	planner := reposync.NewForgePlanner(forgeProvider, plannerConfig)

	// Generate plan to get repo list
	ctx := context.Background()
	planResult, err := planner.Plan(ctx, reposync.PlanRequest{})
	if err != nil {
		return nil, fmt.Errorf("fetch repos: %w", err)
	}

	// Extract repos from plan actions
	repos := make([]reposync.RepoSpec, 0, len(planResult.Actions))
	for _, action := range planResult.Actions {
		repos = append(repos, action.Repo)
	}

	return repos, nil
}

// loadConfigWorkspace recursively loads a nested config file.
func (l FileSpecLoader) loadConfigWorkspace(wsPath string) ([]reposync.RepoSpec, error) {
	// Try .gz-git.yaml first, then .gz-git.yml
	candidates := []string{".gz-git.yaml", ".gz-git.yml"}

	for _, name := range candidates {
		configPath := filepath.Join(wsPath, name)
		if _, err := os.Stat(configPath); err == nil {
			// Recursively load the nested config
			data, err := l.Load(context.Background(), configPath)
			if err != nil {
				return nil, err
			}
			return data.Plan.Input.Repos, nil
		}
	}

	// No config file found, fall back to scanning for git repos
	return scanGitRepos(wsPath, filepath.Base(wsPath))
}

// createForgeProvider creates a forge provider based on provider type.
func createForgeProvider(provider, token, baseURL string, sshPort int) (reposync.ForgeProvider, error) {
	switch provider {
	case "github":
		return github.NewProvider(token), nil

	case "gitlab":
		p, err := gitlab.NewProviderWithOptions(gitlab.ProviderOptions{
			Token:   token,
			BaseURL: baseURL,
			SSHPort: sshPort,
		})
		if err != nil {
			return nil, err
		}
		return forgeProviderAdapter{p}, nil

	case "gitea":
		return forgeProviderAdapter{gitea.NewProvider(token, baseURL)}, nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: github, gitlab, gitea)", provider)
	}
}

// expandEnvVar expands ${VAR} syntax in a string.
func expandEnvVar(s string) string {
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		varName := s[2 : len(s)-1]
		return os.Getenv(varName)
	}
	return os.ExpandEnv(s)
}

// scanGitRepos scans a directory for git repositories (depth 1).
func scanGitRepos(dir string, workspaceName string) ([]reposync.RepoSpec, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var repos []reposync.RepoSpec
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		repoPath := filepath.Join(dir, entry.Name())
		gitDir := filepath.Join(repoPath, ".git")

		// Check if it's a git repository
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			// Get remote URL if available
			remoteURL := getGitRemoteURL(repoPath)

			repos = append(repos, reposync.RepoSpec{
				Name:       entry.Name(),
				TargetPath: repoPath,
				CloneURL:   remoteURL,
			})
		}
	}

	return repos, nil
}

// getGitRemoteURL gets the origin remote URL from a git repository.
func getGitRemoteURL(repoPath string) string {
	configPath := filepath.Join(repoPath, ".git", "config")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	// Simple parsing to find [remote "origin"] url
	lines := strings.Split(string(data), "\n")
	inOrigin := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == `[remote "origin"]` {
			inOrigin = true
			continue
		}
		if inOrigin {
			if strings.HasPrefix(line, "[") {
				break
			}
			if strings.HasPrefix(line, "url = ") {
				return strings.TrimPrefix(line, "url = ")
			}
		}
	}
	return ""
}

// loadParentProfiles recursively loads parent configs and merges profiles.
// Returns merged profiles map where child profiles override parent profiles.
// Uses visited set to prevent circular references.
func loadParentProfiles(configPath string, visited map[string]bool) (map[string]*profileEntry, error) {
	absPath, err := filepath.Abs(cleanPath(configPath))
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	// Check for circular reference
	if visited[absPath] {
		return nil, fmt.Errorf("circular parent reference detected: %s", absPath)
	}
	visited[absPath] = true

	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read parent config %s: %w", absPath, err)
	}

	var cfg workspacesConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse parent config %s: %w", absPath, err)
	}

	// Start with parent's profiles (recursively load if parent has its own parent)
	profiles := make(map[string]*profileEntry)
	if cfg.Parent != "" {
		parentPath := cfg.Parent
		// Resolve relative path based on current config's directory
		if !filepath.IsAbs(parentPath) && !strings.HasPrefix(parentPath, "~") {
			parentPath = filepath.Join(filepath.Dir(absPath), parentPath)
		}
		parentProfiles, err := loadParentProfiles(parentPath, visited)
		if err != nil {
			// Log warning but continue - parent might not exist
			fmt.Fprintf(os.Stderr, "Warning: failed to load parent config: %v\n", err)
		} else {
			// Copy parent profiles
			for k, v := range parentProfiles {
				profiles[k] = v
			}
		}
	}

	// Merge current config's profiles (overrides parent)
	for k, v := range cfg.Profiles {
		profiles[k] = v
	}

	return profiles, nil
}

// mergeProfilesIntoConfig loads parent profiles and merges them into the current config.
// Child profiles override parent profiles.
func mergeProfilesIntoConfig(cfg *workspacesConfig, configPath string) error {
	if cfg.Parent == "" {
		return nil // No parent to merge
	}

	parentPath := cfg.Parent
	configDir := filepath.Dir(configPath)

	// Resolve relative path
	if !filepath.IsAbs(parentPath) && !strings.HasPrefix(parentPath, "~") {
		parentPath = filepath.Join(configDir, parentPath)
	}

	visited := make(map[string]bool)
	// Mark current config as visited
	if absPath, err := filepath.Abs(configPath); err == nil {
		visited[absPath] = true
	}

	parentProfiles, err := loadParentProfiles(parentPath, visited)
	if err != nil {
		return fmt.Errorf("load parent profiles: %w", err)
	}

	// Merge: parent profiles first, then child profiles override
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]*profileEntry)
	}

	for k, v := range parentProfiles {
		if _, exists := cfg.Profiles[k]; !exists {
			cfg.Profiles[k] = v
		}
	}

	return nil
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

// detectConfigFile searches for config files in the given directory.
// Priority: .gz-git.yaml > .gz-git.yml (current directory only, no parent scan)
// This is a wrapper around config.DetectConfigFile for backward compatibility.
func detectConfigFile(dir string) (string, error) {
	return config.DetectConfigFile(dir)
}
