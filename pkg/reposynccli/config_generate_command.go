// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitea"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/github"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitlab"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// ConfigGenerateOptions holds options for config generate command.
type ConfigGenerateOptions struct {
	Provider         string
	Organization     string
	TargetPath       string
	Token            string
	BaseURL          string
	CloneProto       string
	SSHPort          int
	Strategy         string
	Parallel         int
	MaxRetries       int
	Output           string
	IncludeArchived  bool
	IncludeForks     bool
	IncludePrivate   bool
	IsUser           bool
	IncludeSubgroups bool
	SubgroupMode     string
}

func (f CommandFactory) newConfigGenerateCmd() *cobra.Command {
	opts := &ConfigGenerateOptions{
		Strategy:     "reset",
		Parallel:     4,
		MaxRetries:   3,
		CloneProto:   "ssh",
		SubgroupMode: "flat",
		Output:       "sync.yaml",
	}

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate config from Git forge (GitHub, GitLab, Gitea)",
		Long: cliutil.QuickStartHelp(`  # Generate config from GitLab
  gz-git sync config generate --provider gitlab --org devbox -o sync.yaml \
    --token $GITLAB_TOKEN --target ~/repos

  # Include subgroups with flat naming
  gz-git sync config generate --provider gitlab --org parent-group \
    --include-subgroups --subgroup-mode flat -o sync.yaml \
    --token $GITLAB_TOKEN --target ~/repos

  # Generate from GitHub with HTTPS clone
  gz-git sync config generate --provider github --org myorg \
    --clone-proto https -o sync.yaml \
    --token $GITHUB_TOKEN --target ~/repos

  # Self-hosted GitLab with custom SSH port
  gz-git sync config generate --provider gitlab --org mygroup \
    --base-url https://gitlab.company.com --ssh-port 2224 -o sync.yaml \
    --token $GITLAB_TOKEN --target ~/repos`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.runConfigGenerate(cmd, opts)
		},
	}

	// Provider and target (required)
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "Git forge provider: github, gitlab, gitea [required]")
	cmd.Flags().StringVar(&opts.Organization, "org", "", "Organization/group name [required]")
	cmd.Flags().StringVar(&opts.TargetPath, "target", "", "Target directory for cloned repositories [required]")
	cmd.Flags().BoolVar(&opts.IsUser, "user", false, "Treat --org as a user instead of organization")

	// Authentication
	cmd.Flags().StringVar(&opts.Token, "token", "", "API token for authentication")
	cmd.Flags().StringVar(&opts.BaseURL, "base-url", "", "Base URL for self-hosted instances (API endpoint)")

	// Clone options
	cmd.Flags().StringVar(&opts.CloneProto, "clone-proto", opts.CloneProto, "Clone protocol: ssh, https")
	cmd.Flags().IntVar(&opts.SSHPort, "ssh-port", opts.SSHPort, "Custom SSH port (0 = default 22)")

	// Strategy and execution
	cmd.Flags().StringVar(&opts.Strategy, "strategy", opts.Strategy, "Sync strategy (reset, pull, fetch)")
	cmd.Flags().IntVar(&opts.Parallel, "parallel", opts.Parallel, "Number of parallel workers")
	cmd.Flags().IntVar(&opts.MaxRetries, "max-retries", opts.MaxRetries, "Max retry attempts")

	// Output
	cmd.Flags().StringVarP(&opts.Output, "output", "o", opts.Output, "Output file path")

	// Filters
	cmd.Flags().BoolVar(&opts.IncludeArchived, "include-archived", false, "Include archived repositories")
	cmd.Flags().BoolVar(&opts.IncludeForks, "include-forks", false, "Include forked repositories")
	cmd.Flags().BoolVar(&opts.IncludePrivate, "include-private", true, "Include private repositories")

	// GitLab subgroups
	cmd.Flags().BoolVar(&opts.IncludeSubgroups, "include-subgroups", false, "Include subgroups (GitLab only)")
	cmd.Flags().StringVar(&opts.SubgroupMode, "subgroup-mode", opts.SubgroupMode, "Subgroup mode: flat (dash-separated) or nested (directories)")

	// Mark required
	cmd.MarkFlagRequired("provider")
	cmd.MarkFlagRequired("org")
	cmd.MarkFlagRequired("target")

	return cmd
}

func (f CommandFactory) runConfigGenerate(cmd *cobra.Command, opts *ConfigGenerateOptions) error {
	ctx := cmd.Context()

	// Validate clone protocol
	if opts.CloneProto != "ssh" && opts.CloneProto != "https" {
		return fmt.Errorf("invalid --clone-proto: %s (must be ssh or https)", opts.CloneProto)
	}

	// Validate subgroup mode
	if opts.SubgroupMode != "flat" && opts.SubgroupMode != "nested" {
		return fmt.Errorf("invalid --subgroup-mode: %s (must be flat or nested)", opts.SubgroupMode)
	}

	// Create provider
	forgeProvider, err := createConfigGenerateProvider(opts)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Fetch repositories from forge
	repos, err := fetchRepositoriesFromForge(ctx, forgeProvider, opts)
	if err != nil {
		return fmt.Errorf("failed to fetch repositories: %w", err)
	}

	// Generate config
	config := generateConfigYAML(repos, opts)

	// Write to file
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(opts.Output, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Generated configuration: %s (%d repositories)\n", opts.Output, len(repos))
	fmt.Fprintf(cmd.OutOrStdout(), "\nReview the file and run:\n  gz-git sync from-config -c %s\n", opts.Output)

	return nil
}

func createConfigGenerateProvider(opts *ConfigGenerateOptions) (provider.Provider, error) {
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
		return nil, fmt.Errorf("unsupported provider: %s (must be github, gitlab, or gitea)", opts.Provider)
	}
}

func fetchRepositoriesFromForge(ctx context.Context, forgeProvider provider.Provider, opts *ConfigGenerateOptions) ([]*provider.Repository, error) {
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

	// Filter based on options
	filtered := make([]*provider.Repository, 0, len(repos))
	for _, repo := range repos {
		// Filter archived
		if repo.Archived && !opts.IncludeArchived {
			continue
		}
		// Filter forks
		if repo.Fork && !opts.IncludeForks {
			continue
		}
		// Filter private (include by default)
		if repo.Private && !opts.IncludePrivate {
			continue
		}

		// Filter subgroups if needed (GitLab only)
		if !opts.IncludeSubgroups && opts.Provider == "gitlab" {
			// Keep only repos directly in the org (no slashes in FullName after org/)
			if repo.FullName != fmt.Sprintf("%s/%s", opts.Organization, repo.Name) {
				continue
			}
		}

		filtered = append(filtered, repo)
	}

	return filtered, nil
}

func generateConfigYAML(repos []*provider.Repository, opts *ConfigGenerateOptions) map[string]interface{} {
	repoEntries := make([]map[string]interface{}, 0, len(repos))

	for _, repo := range repos {
		targetPath := buildTargetPath(repo, opts)

		entry := map[string]interface{}{
			"name":       repo.Name,
			"url":        repo.CloneURL,
			"targetPath": targetPath,
		}

		repoEntries = append(repoEntries, entry)
	}

	config := map[string]interface{}{
		"# Generated":  time.Now().Format(time.RFC3339),
		"strategy":     opts.Strategy,
		"parallel":     opts.Parallel,
		"maxRetries":   opts.MaxRetries,
		"cloneProto":   opts.CloneProto,
		"sshPort":      opts.SSHPort,
		"repositories": repoEntries,
	}

	return config
}

func buildTargetPath(repo *provider.Repository, opts *ConfigGenerateOptions) string {
	basePath := opts.TargetPath
	projectPath := repo.FullName

	if !opts.IncludeSubgroups || opts.SubgroupMode == "" {
		return fmt.Sprintf("%s/%s", basePath, repo.Name)
	}

	switch opts.SubgroupMode {
	case "flat":
		// Replace slashes with dashes: parent-group/subgroup/repo → parent-group-subgroup-repo
		flat := projectPath
		for i := 0; i < len(flat); i++ {
			if flat[i] == '/' {
				flat = flat[:i] + "-" + flat[i+1:]
			}
		}
		return fmt.Sprintf("%s/%s", basePath, flat)
	case "nested":
		// Keep directory structure: parent-group/subgroup/repo
		return fmt.Sprintf("%s/%s", basePath, projectPath)
	default:
		return fmt.Sprintf("%s/%s", basePath, repo.Name)
	}
}
