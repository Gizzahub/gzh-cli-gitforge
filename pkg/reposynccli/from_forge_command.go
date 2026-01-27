// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitea"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/github"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitlab"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// FromForgeOptions holds options for from-forge sync command.
type FromForgeOptions struct {
	Provider         string
	Organization     string
	TargetPath       string
	Token            string
	BaseURL          string // API endpoint (http/https only)
	CloneProto       string // Clone protocol: ssh, https (default: ssh)
	SSHPort          int    // Custom SSH port (0 = default 22)
	SSHKeyPath       string // SSH private key file path
	SSHKeyContent    string // SSH private key content (use env var)
	Strategy         string
	Parallel         int
	MaxRetries       int
	Resume           bool
	DryRun           bool
	StateFile        string
	IncludeArchived  bool
	IncludeForks     bool
	IncludePrivate   bool
	CleanupOrphans   bool
	IsUser           bool
	IncludeSubgroups bool   // GitLab: include subgroups
	SubgroupMode     string // GitLab: flat | nested
}

// newFromForgeCmd creates a command for syncing from git forges.
func (f CommandFactory) newFromForgeCmd() *cobra.Command {
	opts := &FromForgeOptions{
		Strategy:     "reset",
		Parallel:     repository.DefaultBulkParallel,
		MaxRetries:   3,
		CloneProto:   "ssh",  // Default to SSH
		SubgroupMode: "flat", // Default to flat
	}

	cmd := &cobra.Command{
		Use:     "from",
		Aliases: []string{"from-forge"},
		Short:   "Sync repositories from a Git forge (GitHub, GitLab, Gitea)",
		Long: cliutil.QuickStartHelp(`  # Sync from GitHub organization (default: SSH clone)
  gz-git forge from --provider github --org myorg --path ./repos --token $GITHUB_TOKEN

  # Sync from GitLab group with HTTPS clone
  gz-git forge from --provider gitlab --org mygroup --path ./repos \
    --token $GITLAB_TOKEN --clone-proto https

  # Sync from self-hosted GitLab with custom SSH port
  gz-git forge from --provider gitlab --org mygroup --path ./repos \
    --base-url https://gitlab.company.com --token $GITLAB_TOKEN \
    --clone-proto ssh --ssh-port 2224

  # Sync GitLab with subgroups (flat mode)
  gz-git forge from --provider gitlab --org parent-group --path ./repos \
    --include-subgroups --subgroup-mode flat

  # Sync from Gitea
  gz-git forge from --provider gitea --org myorg --path ./repos \
    --base-url https://gitea.company.com --token $GITEA_TOKEN`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.runFromForge(cmd, opts)
		},
	}

	// Provider and path (required)
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "Git forge provider: github, gitlab, gitea [required]")
	cmd.Flags().StringVar(&opts.Organization, "org", "", "Organization/group name to sync [required]")
	cmd.Flags().StringVar(&opts.TargetPath, "path", "", "Directory for cloned repositories [required]")
	cmd.Flags().BoolVar(&opts.IsUser, "user", false, "Treat --org as a user instead of organization")

	// Authentication
	cmd.Flags().StringVar(&opts.Token, "token", "", "API token for authentication")
	cmd.Flags().StringVar(&opts.BaseURL, "base-url", "", "Base URL for self-hosted instances (API endpoint)")

	// Clone options
	cmd.Flags().StringVar(&opts.CloneProto, "clone-proto", opts.CloneProto, "Clone protocol: ssh, https")
	cmd.Flags().IntVar(&opts.SSHPort, "ssh-port", 0, "Custom SSH port (0 = default 22)")
	cmd.Flags().StringVar(&opts.SSHKeyPath, "ssh-key", "", "SSH private key file path")
	cmd.Flags().StringVar(&opts.SSHKeyContent, "ssh-key-content", "", "SSH private key content (use env var for security)")

	// Sync options
	cmd.Flags().StringVar(&opts.Strategy, "sync-strategy", opts.Strategy, "Sync strategy (reset, pull, fetch)")
	cmd.Flags().IntVar(&opts.Parallel, "parallel", opts.Parallel, "Number of parallel workers")
	cmd.Flags().IntVar(&opts.MaxRetries, "max-retries", opts.MaxRetries, "Max retry attempts")
	cmd.Flags().BoolVar(&opts.Resume, "resume", false, "Resume from previous state")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show what would be done without executing")
	cmd.Flags().StringVar(&opts.StateFile, "state-file", "", "State file for resume support")

	// Filtering options
	cmd.Flags().BoolVar(&opts.IncludeArchived, "include-archived", false, "Include archived repositories")
	cmd.Flags().BoolVar(&opts.IncludeForks, "include-forks", false, "Include forked repositories")
	cmd.Flags().BoolVar(&opts.IncludePrivate, "include-private", true, "Include private repositories")
	cmd.Flags().BoolVar(&opts.CleanupOrphans, "cleanup-orphans", false, "Delete directories not in organization")

	// GitLab specific
	cmd.Flags().BoolVar(&opts.IncludeSubgroups, "include-subgroups", false, "Include subgroups (GitLab only)")
	cmd.Flags().StringVar(&opts.SubgroupMode, "subgroup-mode", opts.SubgroupMode, "Subgroup mode: flat (dash-separated) or nested (directories)")

	// Required flags
	_ = cmd.MarkFlagRequired("provider")
	_ = cmd.MarkFlagRequired("org")
	_ = cmd.MarkFlagRequired("path")

	return cmd
}

func (f CommandFactory) runFromForge(cmd *cobra.Command, opts *FromForgeOptions) error {
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
	forgeProvider, err := createFromForgeProvider(opts)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Parse strategy
	strategy, err := reposync.ParseStrategy(opts.Strategy)
	if err != nil {
		return fmt.Errorf("invalid strategy: %w", err)
	}

	// Create ForgePlanner with authentication config
	plannerConfig := reposync.ForgePlannerConfig{
		TargetPath:       opts.TargetPath,
		Organization:     opts.Organization,
		IsUser:           opts.IsUser,
		IncludeArchived:  opts.IncludeArchived,
		IncludeForks:     opts.IncludeForks,
		IncludePrivate:   opts.IncludePrivate,
		CloneProto:       opts.CloneProto,
		SSHPort:          opts.SSHPort,
		IncludeSubgroups: opts.IncludeSubgroups,
		SubgroupMode:     opts.SubgroupMode,
		Auth: reposync.AuthConfig{
			Token:         opts.Token,
			Provider:      opts.Provider,
			SSHKeyPath:    opts.SSHKeyPath,
			SSHKeyContent: opts.SSHKeyContent,
			SSHPort:       opts.SSHPort,
		},
	}

	planner := reposync.NewForgePlanner(forgeProvider, plannerConfig)

	// Create orchestrator with ForgePlanner
	executor := reposync.GitExecutor{}
	orchestrator := reposync.NewOrchestrator(planner, executor, nil)

	// Build plan request
	planReq := reposync.PlanRequest{
		Options: reposync.PlanOptions{
			DefaultStrategy: strategy,
			CleanupOrphans:  opts.CleanupOrphans,
		},
	}

	if opts.CleanupOrphans {
		planReq.Options.Roots = []string{opts.TargetPath}
	}

	// Build run options
	runOpts := reposync.RunOptions{
		Parallel:   opts.Parallel,
		MaxRetries: opts.MaxRetries,
		Resume:     opts.Resume,
		DryRun:     opts.DryRun,
	}

	if opts.Resume && opts.StateFile == "" {
		return fmt.Errorf("resume requested but no --state-file provided")
	}

	// Progress and state
	progress := ConsoleProgressSink{Out: cmd.OutOrStdout()}

	var state reposync.StateStore
	if opts.StateFile != "" {
		state = reposync.NewFileStateStore(opts.StateFile)
	}

	// Run
	result, err := orchestrator.Run(ctx, reposync.RunRequest{
		PlanRequest: planReq,
		RunOptions:  runOpts,
		Progress:    progress,
		State:       state,
	})
	if err != nil {
		return fmt.Errorf("forge sync failed: %w", err)
	}

	// Print summary
	fmt.Fprintf(cmd.OutOrStdout(), "\nSync completed: %d succeeded, %d failed, %d skipped\n",
		len(result.Succeeded), len(result.Failed), len(result.Skipped))

	return nil
}

// createFromForgeProvider creates the appropriate provider based on options.
func createFromForgeProvider(opts *FromForgeOptions) (reposync.ForgeProvider, error) {
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
		return forgeProviderAdapter{p}, nil

	case "gitea":
		p, err := gitea.NewProvider(opts.Token, opts.BaseURL)
		if err != nil {
			return nil, err
		}
		return forgeProviderAdapter{p}, nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: github, gitlab, gitea)", opts.Provider)
	}
}

// forgeProviderAdapter adapts gitforge providers to ForgeProvider interface.
type forgeProviderAdapter struct {
	provider.Provider
}

// CreateForgeProviderRaw creates a provider from raw strings.
// This is a helper for other packages (like workspacecli) to avoid duplicating provider creation logic.
func CreateForgeProviderRaw(providerName, token, baseURL string, sshPort int) (reposync.ForgeProvider, error) {
	opts := &FromForgeOptions{
		Provider: providerName,
		Token:    token,
		BaseURL:  baseURL,
		SSHPort:  sshPort,
	}
	return createFromForgeProvider(opts)
}

// CreateProviderFromSource creates a forge provider from config types with profile fallback.
// This handles the common pattern of extracting provider settings from ForgeSource,
// falling back to Workspace settings, then to profile settings from the config chain.
func CreateProviderFromSource(src *config.ForgeSource, ws *config.Workspace, cfg *config.Config) (reposync.ForgeProvider, error) {
	// Extract values from source
	token := src.Token
	baseURL := src.BaseURL
	sshPort := ws.SSHPort
	providerName := src.Provider

	// Fallback to profile values if not set in source
	if ws.Profile != "" && cfg != nil {
		profile := config.GetProfileFromChain(cfg, ws.Profile)
		if profile != nil {
			if token == "" {
				token = profile.Token
			}
			if baseURL == "" {
				baseURL = profile.BaseURL
			}
			if sshPort == 0 {
				sshPort = profile.SSHPort
			}
			if providerName == "" {
				providerName = profile.Provider
			}
		}
	}

	return CreateForgeProviderRaw(providerName, token, baseURL, sshPort)
}
