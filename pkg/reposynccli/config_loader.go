package reposynccli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

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
	Strategy       string      `yaml:"strategy"`
	Parallel       int         `yaml:"parallel"`
	MaxRetries     *int        `yaml:"maxRetries"`
	Resume         bool        `yaml:"resume"`
	DryRun         bool        `yaml:"dryRun"`
	CleanupOrphans bool        `yaml:"cleanupOrphans"`
	Roots          []string    `yaml:"roots"`
	Repositories   []repoEntry `yaml:"repositories"`
}

type repoEntry struct {
	Name          string `yaml:"name"`
	Provider      string `yaml:"provider"`
	URL           string `yaml:"url"`
	TargetPath    string `yaml:"targetPath"`
	Strategy      string `yaml:"strategy"`
	AssumePresent bool   `yaml:"assumePresent"`
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
		defaultParallel = 4
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
		if repo.Name == "" || repo.URL == "" || repo.TargetPath == "" {
			return ConfigData{}, fmt.Errorf("repository entry is missing required fields (name/url/targetPath)")
		}

		repoStrategy := parsedStrategy
		if repo.Strategy != "" {
			repoStrategy, err = reposync.ParseStrategy(repo.Strategy)
			if err != nil {
				return ConfigData{}, fmt.Errorf("repository %s: %w", repo.Name, err)
			}
		}

		targetPath := cleanPath(repo.TargetPath)
		if _, exists := seenTargets[targetPath]; exists {
			return ConfigData{}, fmt.Errorf("duplicate targetPath detected: %s", targetPath)
		}
		seenTargets[targetPath] = struct{}{}

		plan.Input.Repos = append(plan.Input.Repos, reposync.RepoSpec{
			Name:          repo.Name,
			Provider:      repo.Provider,
			CloneURL:      repo.URL,
			TargetPath:    targetPath,
			Strategy:      repoStrategy,
			AssumePresent: repo.AssumePresent,
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
		defaultParallel = 4
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
			return ConfigData{}, fmt.Errorf("duplicate targetPath detected: %s", targetPath)
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
