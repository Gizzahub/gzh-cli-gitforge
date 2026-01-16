// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitea"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/github"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitlab"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// ConfigMergeOptions holds options for config merge command.
type ConfigMergeOptions struct {
	Provider         string
	Organization     string
	TargetPath       string
	Token            string
	BaseURL          string
	Into             string // Existing config file to merge into
	Mode             string // append | update | overwrite
	CloneProto       string
	SSHPort          int
	IncludeArchived  bool
	IncludeForks     bool
	IncludePrivate   bool
	IsUser           bool
	IncludeSubgroups bool
	SubgroupMode     string
}

func (f CommandFactory) newConfigMergeCmd() *cobra.Command {
	opts := &ConfigMergeOptions{
		Mode:         "append",
		CloneProto:   "ssh",
		SubgroupMode: "flat",
	}

	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge repositories from forge into existing config",
		Long: `Merge repositories from a Git forge into an existing configuration file.

This command queries the forge API and adds new repositories to the config
file without duplicating existing entries (by default).

Merge Modes:
  append    - Add new repos only (default, no duplicates)
  update    - Update existing repos, add new ones
  overwrite - Replace entire config with forge repos

Examples:
  # Merge another org into existing config (append mode)
  gz-git sync config merge --provider gitlab --org another-group \
    --into sync.yaml --token $GITLAB_TOKEN

  # Update mode: update existing + add new
  gz-git sync config merge --provider gitlab --org devbox \
    --into sync.yaml --mode update --token $GITLAB_TOKEN

  # Overwrite mode: replace entire config
  gz-git sync config merge --provider github --org myorg \
    --into sync.yaml --mode overwrite --token $GITHUB_TOKEN

  # Include subgroups
  gz-git sync config merge --provider gitlab --org parent-group \
    --into sync.yaml --include-subgroups --subgroup-mode nested`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.runConfigMerge(cmd, opts)
		},
	}

	// Provider and target (required)
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "Git forge provider: github, gitlab, gitea [required]")
	cmd.Flags().StringVar(&opts.Organization, "org", "", "Organization/group name [required]")
	cmd.Flags().StringVar(&opts.TargetPath, "target", "", "Target directory for repositories [required]")
	cmd.Flags().BoolVar(&opts.IsUser, "user", false, "Treat --org as a user instead of organization")

	// Authentication
	cmd.Flags().StringVar(&opts.Token, "token", "", "API token for authentication")
	cmd.Flags().StringVar(&opts.BaseURL, "base-url", "", "Base URL for self-hosted instances (API endpoint)")

	// Clone options
	cmd.Flags().StringVar(&opts.CloneProto, "clone-proto", opts.CloneProto, "Clone protocol: ssh, https")
	cmd.Flags().IntVar(&opts.SSHPort, "ssh-port", opts.SSHPort, "Custom SSH port (0 = default 22)")

	// Merge options
	cmd.Flags().StringVar(&opts.Into, "into", "", "Existing config file to merge into [required]")
	cmd.Flags().StringVar(&opts.Mode, "mode", opts.Mode, "Merge mode: append | update | overwrite")

	// Filters
	cmd.Flags().BoolVar(&opts.IncludeArchived, "include-archived", false, "Include archived repositories")
	cmd.Flags().BoolVar(&opts.IncludeForks, "include-forks", false, "Include forked repositories")
	cmd.Flags().BoolVar(&opts.IncludePrivate, "include-private", true, "Include private repositories")

	// GitLab subgroups
	cmd.Flags().BoolVar(&opts.IncludeSubgroups, "include-subgroups", false, "Include subgroups (GitLab only)")
	cmd.Flags().StringVar(&opts.SubgroupMode, "subgroup-mode", opts.SubgroupMode, "Subgroup mode: flat | nested")

	// Mark required
	cmd.MarkFlagRequired("provider")
	cmd.MarkFlagRequired("org")
	cmd.MarkFlagRequired("target")
	cmd.MarkFlagRequired("into")

	return cmd
}

func (f CommandFactory) runConfigMerge(cmd *cobra.Command, opts *ConfigMergeOptions) error {
	ctx := cmd.Context()

	// Validate mode
	if opts.Mode != "append" && opts.Mode != "update" && opts.Mode != "overwrite" {
		return fmt.Errorf("invalid mode: %s (must be append, update, or overwrite)", opts.Mode)
	}

	// Validate clone protocol
	if opts.CloneProto != "ssh" && opts.CloneProto != "https" {
		return fmt.Errorf("invalid --clone-proto: %s (must be ssh or https)", opts.CloneProto)
	}

	// Load existing config
	existingConfig, err := loadExistingConfig(opts.Into)
	if err != nil {
		return fmt.Errorf("failed to load existing config: %w", err)
	}

	// Create provider
	forgeProvider, err := createConfigMergeProvider(opts)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Fetch repositories from forge
	repos, err := fetchRepositoriesForMerge(ctx, forgeProvider, opts)
	if err != nil {
		return fmt.Errorf("failed to fetch repositories: %w", err)
	}

	// Merge based on mode
	var mergedConfig map[string]interface{}
	var addedCount, updatedCount, skippedCount int

	switch opts.Mode {
	case "overwrite":
		mergedConfig = generateConfigYAML(repos, &ConfigGenerateOptions{
			Strategy:   "reset",
			Parallel:   4,
			MaxRetries: 3,
			CloneProto: opts.CloneProto,
			SSHPort:    opts.SSHPort,
		})
		addedCount = len(repos)
		fmt.Fprintf(cmd.OutOrStdout(), "Mode: overwrite (replaced entire config)\n")

	case "append":
		mergedConfig, addedCount, skippedCount = mergeAppend(existingConfig, repos, opts)
		fmt.Fprintf(cmd.OutOrStdout(), "Mode: append (add new repos only)\n")

	case "update":
		mergedConfig, addedCount, updatedCount, skippedCount = mergeUpdate(existingConfig, repos, opts)
		fmt.Fprintf(cmd.OutOrStdout(), "Mode: update (update existing + add new)\n")
	}

	// Write merged config
	data, err := yaml.Marshal(mergedConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(opts.Into, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Print summary
	fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Merged into: %s\n", opts.Into)
	if opts.Mode == "overwrite" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Total repositories: %d\n", addedCount)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "  Added: %d\n", addedCount)
		if updatedCount > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Updated: %d\n", updatedCount)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Skipped (duplicates): %d\n", skippedCount)
	}

	return nil
}

func createConfigMergeProvider(opts *ConfigMergeOptions) (provider.Provider, error) {
	switch opts.Provider {
	case "github":
		return github.NewProvider(opts.Token), nil

	case "gitlab":
		p, err := gitlab.NewProviderWithOptions(gitlab.ProviderOptions{
			Token:   opts.Token,
			BaseURL: opts.BaseURL,
			SSHPort: opts.SSHPort,
		})
		if err != nil {
			return nil, err
		}
		return p, nil

	case "gitea":
		if opts.BaseURL == "" {
			return nil, fmt.Errorf("gitea requires --base-url")
		}
		return gitea.NewProvider(opts.Token, opts.BaseURL), nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", opts.Provider)
	}
}

func fetchRepositoriesForMerge(ctx context.Context, forgeProvider provider.Provider, opts *ConfigMergeOptions) ([]*provider.Repository, error) {
	var repos []*provider.Repository
	var err error

	if opts.IsUser {
		repos, err = forgeProvider.ListUserRepos(ctx, opts.Organization)
	} else {
		repos, err = forgeProvider.ListOrganizationRepos(ctx, opts.Organization)
	}

	if err != nil {
		return nil, err
	}

	// Filter
	filtered := make([]*provider.Repository, 0, len(repos))
	for _, repo := range repos {
		if repo.Archived && !opts.IncludeArchived {
			continue
		}
		if repo.Fork && !opts.IncludeForks {
			continue
		}
		if repo.Private && !opts.IncludePrivate {
			continue
		}

		// Filter subgroups (GitLab)
		if !opts.IncludeSubgroups && opts.Provider == "gitlab" {
			if repo.FullName != fmt.Sprintf("%s/%s", opts.Organization, repo.Name) {
				continue
			}
		}

		filtered = append(filtered, repo)
	}

	return filtered, nil
}

func loadExistingConfig(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

func mergeAppend(existingConfig map[string]interface{}, newRepos []*provider.Repository, opts *ConfigMergeOptions) (map[string]interface{}, int, int) {
	// Get existing repositories
	existingRepos, ok := existingConfig["repositories"].([]interface{})
	if !ok {
		existingRepos = []interface{}{}
	}

	// Build set of existing URLs for deduplication
	existingURLs := make(map[string]bool)
	for _, repo := range existingRepos {
		repoMap, ok := repo.(map[string]interface{})
		if !ok {
			continue
		}
		if url, ok := repoMap["url"].(string); ok && url != "" {
			existingURLs[url] = true
		}
	}

	// Add new repos (skip duplicates)
	addedCount := 0
	skippedCount := 0

	for _, repo := range newRepos {
		if existingURLs[repo.CloneURL] {
			skippedCount++
			continue
		}

		targetPath := buildMergeTargetPath(repo, opts)
		entry := map[string]interface{}{
			"name":       repo.Name,
			"url":        repo.CloneURL,
			"targetPath": targetPath,
		}

		existingRepos = append(existingRepos, entry)
		addedCount++
	}

	existingConfig["repositories"] = existingRepos
	return existingConfig, addedCount, skippedCount
}

func mergeUpdate(existingConfig map[string]interface{}, newRepos []*provider.Repository, opts *ConfigMergeOptions) (map[string]interface{}, int, int, int) {
	existingRepos, ok := existingConfig["repositories"].([]interface{})
	if !ok {
		existingRepos = []interface{}{}
	}

	// Build map of existing repos by URL
	existingByURL := make(map[string]int)
	for i, repo := range existingRepos {
		repoMap, ok := repo.(map[string]interface{})
		if !ok {
			continue
		}
		if url, ok := repoMap["url"].(string); ok && url != "" {
			existingByURL[url] = i
		}
	}

	addedCount := 0
	updatedCount := 0
	skippedCount := 0

	for _, repo := range newRepos {
		if idx, exists := existingByURL[repo.CloneURL]; exists {
			// Update existing
			targetPath := buildMergeTargetPath(repo, opts)
			repoMap := existingRepos[idx].(map[string]interface{})
			repoMap["name"] = repo.Name
			repoMap["url"] = repo.CloneURL
			repoMap["targetPath"] = targetPath
			updatedCount++
		} else {
			// Add new
			targetPath := buildMergeTargetPath(repo, opts)
			entry := map[string]interface{}{
				"name":       repo.Name,
				"url":        repo.CloneURL,
				"targetPath": targetPath,
			}
			existingRepos = append(existingRepos, entry)
			addedCount++
		}
	}

	existingConfig["repositories"] = existingRepos
	return existingConfig, addedCount, updatedCount, skippedCount
}

func buildMergeTargetPath(repo *provider.Repository, opts *ConfigMergeOptions) string {
	basePath := opts.TargetPath
	projectPath := repo.FullName

	if !opts.IncludeSubgroups || opts.SubgroupMode == "" {
		return fmt.Sprintf("%s/%s", basePath, repo.Name)
	}

	switch opts.SubgroupMode {
	case "flat":
		flat := projectPath
		for i := 0; i < len(flat); i++ {
			if flat[i] == '/' {
				flat = flat[:i] + "-" + flat[i+1:]
			}
		}
		return fmt.Sprintf("%s/%s", basePath, flat)
	case "nested":
		return fmt.Sprintf("%s/%s", basePath, projectPath)
	default:
		return fmt.Sprintf("%s/%s", basePath, repo.Name)
	}
}
