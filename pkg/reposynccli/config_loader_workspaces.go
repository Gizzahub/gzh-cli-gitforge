// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/internal/safefs"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// workspacesConfig is the hierarchical workspaces document. It decodes into
// the canonical pkg/config schema that `workspace sync` reads (ISSUE-391), so
// a field is declared once; only the legacy top-level connection defaults,
// which config.Config keeps under defaults.*, are read here.
type workspacesConfig struct {
	config.Config `yaml:",inline"`

	Parallel   int    `yaml:"parallel"`
	CloneProto string `yaml:"cloneProto"`
	SSHPort    int    `yaml:"sshPort"`
}

// configChainKey carries the config directories of the current nesting path,
// including the directory of the config being loaded, so this loader and
// sync share one cycle rule (ISSUE-104).
type configChainKey struct{}

// configChain returns the config directories of the current nesting path.
// The stored chain includes the directory of the config being loaded; at
// the top level that is configDir.
func configChain(ctx context.Context, configDir string) config.ConfigChain {
	chain, ok := ctx.Value(configChainKey{}).(config.ConfigChain)
	if !ok {
		return config.ConfigChain{configDir}
	}
	return chain
}

// loadWorkspacesConfig handles the new hierarchical config format with workspaces.
// Supports three workspace types:
//   - type: forge  → fetch repos from forge API (GitHub/GitLab/Gitea)
//   - type: config → recursively load nested .gz-git.yaml
//   - type: git    → scan directory for git repositories (default)
func (l FileSpecLoader) loadWorkspacesConfig(ctx context.Context, raw []byte, configPath string) (ConfigData, error) { //nolint:gocognit,gocyclo // complex multi-workspace loading logic — extracting helpers would lose clarity
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
	var activeProfile *config.Profile
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
		defaultParallel = repository.DefaultLocalParallel
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
	// Absolute, like the nested config targets loadConfigWorkspace returns, so
	// seenTargets compares one path form whatever path the caller passed.
	configDir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return ConfigData{}, fmt.Errorf("resolve config directory of %s: %w", configPath, err)
	}

	// Process each workspace
	for name, ws := range cfg.Workspaces {
		if ws == nil {
			continue
		}
		// Classify with the rule `workspace sync` uses (ISSUE-104), so an
		// untyped workspace without a url and with sync.recursive is a config
		// workspace here too, and a contradictory entry fails both commands.
		recursive := ws.Sync != nil && ws.Sync.Recursive
		wsType, err := config.ClassifyWorkspace(ws.Type, ws.Source != nil, ws.URL, recursive)
		if err != nil {
			return ConfigData{}, fmt.Errorf("workspace '%s' in %s: %w", name, configPath, err)
		}
		// `workspace sync` plans a config workspace only with sync.recursive
		// (config.GetConfigWorkspaces); skip the rest before they add a root,
		// so status neither lists their repositories nor treats them as
		// orphans (ISSUE-101).
		if wsType == config.WorkspaceTypeConfig && !config.SyncsConfigWorkspace(wsType, recursive) {
			continue
		}

		// Resolve workspace path (relative to config file location)
		// A missing path defaults to the workspace name (compact config
		// convention), matching `workspace sync`; the former "." default made
		// a pathless config workspace reload this same file (ISSUE-102).
		wsPath, err := config.ResolvePath(configDir, config.WorkspacePathOrName(ws.Path, name))
		if err != nil {
			return ConfigData{}, fmt.Errorf("workspace '%s' in %s: %w", name, configPath, err)
		}
		// No environment expansion: sync leaves "$VAR" literal, so status
		// only cleans the resolved path (ISSUE-391).
		wsPath = filepath.Clean(wsPath)
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

		var repos []reposync.RepoSpec

		switch wsType {
		case config.WorkspaceTypeForge:
			repos, err = l.loadForgeWorkspace(ctx, ws, wsProfile, wsPath, defaultCloneProto, defaultSSHPort)
			if err != nil {
				// Log warning but continue with other workspaces
				fmt.Fprintf(os.Stderr, "Warning: workspace '%s' forge fetch failed: %v\n", name, err)
				continue
			}

		case config.WorkspaceTypeConfig:
			// A cycle or over-deep nesting refuses the whole tree, as sync
			// does (ISSUE-104); siblings get independent chains.
			next, err := configChain(ctx, configDir).Enter(wsPath)
			if err != nil {
				return ConfigData{}, fmt.Errorf("workspace '%s': %w", name, err)
			}
			repos, err = l.loadConfigWorkspace(context.WithValue(ctx, configChainKey{}, next), wsPath)
			if config.IsConfigTreeError(err) {
				return ConfigData{}, err
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: workspace '%s' skipped: %v\n", name, err)
				continue
			}

		case config.WorkspaceTypeGit:
			repos, err = loadGitWorkspace(name, ws, wsPath, configDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: workspace '%s' skipped: %v\n", name, err)
				continue
			}

		default:
			// No sync planner takes an unknown type either.
			fmt.Fprintf(os.Stderr, "Warning: workspace '%s' skipped: unknown type %q\n", name, wsType)
			continue
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
func (l FileSpecLoader) loadForgeWorkspace( //nolint:gocyclo // complex provider resolution logic — adding helpers would obscure the fallback chain
	ctx context.Context,
	ws *config.Workspace,
	profile *config.Profile,
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
	forgeProvider, err := CreateForgeProviderRaw(provider, token, baseURL, sshPort)
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
//
// Relative repository paths in the nested config are rebased on wsPath, the
// nested config's directory, where `workspace sync` places them (ISSUE-96).
func (l FileSpecLoader) loadConfigWorkspace(ctx context.Context, wsPath string) ([]reposync.RepoSpec, error) {
	// An absolute wsPath makes the nested config's own workspace paths absolute,
	// so only its relative repository entries are rebased below.
	absWSPath, err := filepath.Abs(wsPath)
	if err != nil {
		return nil, fmt.Errorf("resolve nested config workspace %s: %w", wsPath, err)
	}
	wsPath = absWSPath

	// Try .gz-git.yaml first, then .gz-git.yml
	candidates := []string{".gz-git.yaml", ".gz-git.yml"}
	root, err := safefs.OpenRoot(wsPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()

	for _, name := range candidates {
		if _, err := root.Stat(name); err == nil {
			// This config is selected by workspace discovery, so read it through
			// the workspace root. Explicit l.Load calls retain their existing
			// caller-selected path contract.
			raw, err := root.ReadFile(name)
			if err != nil {
				return nil, fmt.Errorf("read nested config %s: %w", filepath.Join(wsPath, name), err)
			}
			if err := config.RejectRemovedKeys(raw, filepath.Join(wsPath, name)); err != nil {
				return nil, err
			}
			data, err := l.loadData(ctx, raw, filepath.Join(wsPath, name))
			if err != nil {
				return nil, err
			}
			repos := data.Plan.Input.Repos
			for i := range repos {
				if repos[i].TargetPath != "" && !filepath.IsAbs(repos[i].TargetPath) {
					repos[i].TargetPath = filepath.Join(wsPath, repos[i].TargetPath)
				}
			}
			return repos, nil
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat nested config %s: %w", filepath.Join(wsPath, name), err)
		}
	}

	// No config file found, fall back to scanning for git repos
	return scanGitRepos(wsPath)
}

// expandEnvVar expands ${VAR} syntax in a string.
func expandEnvVar(s string) string {
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		varName := s[2 : len(s)-1]
		return os.Getenv(varName)
	}
	return os.ExpandEnv(s)
}

// loadParentProfiles recursively loads parent configs and merges profiles.
// Returns merged profiles map where child profiles override parent profiles.
// Uses visited set to prevent circular references.
func loadParentProfiles(configPath string, visited map[string]bool) (map[string]*config.Profile, error) {
	absPath, err := filepath.Abs(filepath.Clean(configPath))
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	// Check for circular reference
	if visited[absPath] {
		return nil, fmt.Errorf("circular parent reference detected: %s", absPath)
	}
	visited[absPath] = true

	raw, err := config.ReadConfigFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read parent config %s: %w", absPath, err)
	}

	var cfg workspacesConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse parent config %s: %w", absPath, err)
	}

	// Start with parent's profiles (recursively load if parent has its own parent)
	profiles := make(map[string]*config.Profile)
	if cfg.Parent != "" {
		// The parent rule shared with the config loader (config.ResolvePath).
		parentPath, err := config.ResolvePath(filepath.Dir(absPath), cfg.Parent)
		if err != nil {
			return nil, fmt.Errorf("resolve parent path %q in %s: %w", cfg.Parent, absPath, err)
		}
		parentProfiles, err := loadParentProfiles(parentPath, visited)
		if err != nil {
			// Log warning but continue - parent might not exist
			fmt.Fprintf(os.Stderr, "Warning: failed to load parent config: %v\n", err)
		} else {
			// Copy parent profiles
			maps.Copy(profiles, parentProfiles)
		}
	}

	// Merge current config's profiles (overrides parent)
	maps.Copy(profiles, cfg.Profiles)

	return profiles, nil
}

// mergeProfilesIntoConfig loads parent profiles and merges them into the current config.
// Child profiles override parent profiles.
func mergeProfilesIntoConfig(cfg *workspacesConfig, configPath string) error {
	if cfg.Parent == "" {
		return nil // No parent to merge
	}

	// The parent rule shared with the config loader (config.ResolvePath).
	parentPath, err := config.ResolvePath(filepath.Dir(configPath), cfg.Parent)
	if err != nil {
		return fmt.Errorf("resolve parent path %q: %w", cfg.Parent, err)
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
		cfg.Profiles = make(map[string]*config.Profile)
	}

	for k, v := range parentProfiles {
		if _, exists := cfg.Profiles[k]; !exists {
			cfg.Profiles[k] = v
		}
	}

	return nil
}
