// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitea"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/github"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitlab"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// ForgeCommandOptions holds options for forge sync command.
type ForgeCommandOptions struct {
	Provider        string
	Organization    string
	TargetPath      string
	Token           string
	BaseURL         string
	Strategy        string
	Parallel        int
	MaxRetries      int
	Resume          bool
	DryRun          bool
	StateFile       string
	IncludeArchived bool
	IncludeForks    bool
	IncludePrivate  bool
	UseSSH          bool
	CleanupOrphans  bool
	IsUser          bool
}

// newForgeCmd creates a command for syncing from git forges.
func (f CommandFactory) newForgeCmd() *cobra.Command {
	opts := &ForgeCommandOptions{
		Strategy:   "reset",
		Parallel:   4,
		MaxRetries: 3,
	}

	cmd := &cobra.Command{
		Use:   "forge",
		Short: "Sync repositories from a Git forge (GitHub, GitLab, Gitea)",
		Long: `Sync repositories from a Git forge provider.

Supports GitHub organizations, GitLab groups, and Gitea organizations.
Use --provider to specify the forge type.

Examples:
  # Sync from GitHub organization
  gz-git sync forge --provider github --org myorg --target ./repos --token $GITHUB_TOKEN

  # Sync from GitLab group
  gz-git sync forge --provider gitlab --org mygroup --target ./repos --token $GITLAB_TOKEN

  # Sync from self-hosted GitLab
  gz-git sync forge --provider gitlab --org mygroup --target ./repos --base-url https://gitlab.company.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.runForgeSync(cmd, opts)
		},
	}

	// Provider and target (required)
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "Git forge provider: github, gitlab, gitea [required]")
	cmd.Flags().StringVar(&opts.Organization, "org", "", "Organization/group name to sync [required]")
	cmd.Flags().StringVar(&opts.TargetPath, "target", "", "Target directory for cloned repositories [required]")
	cmd.Flags().BoolVar(&opts.IsUser, "user", false, "Treat --org as a user instead of organization")

	// Authentication
	cmd.Flags().StringVar(&opts.Token, "token", "", "API token for authentication")
	cmd.Flags().StringVar(&opts.BaseURL, "base-url", "", "Base URL for self-hosted instances")

	// Sync options
	cmd.Flags().StringVar(&opts.Strategy, "strategy", opts.Strategy, "Sync strategy (reset, pull, fetch)")
	cmd.Flags().IntVar(&opts.Parallel, "parallel", opts.Parallel, "Number of parallel workers")
	cmd.Flags().IntVar(&opts.MaxRetries, "max-retries", opts.MaxRetries, "Max retry attempts")
	cmd.Flags().BoolVar(&opts.Resume, "resume", false, "Resume from previous state")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show what would be done without executing")
	cmd.Flags().StringVar(&opts.StateFile, "state-file", "", "State file for resume support")

	// Filtering options
	cmd.Flags().BoolVar(&opts.IncludeArchived, "include-archived", false, "Include archived repositories")
	cmd.Flags().BoolVar(&opts.IncludeForks, "include-forks", false, "Include forked repositories")
	cmd.Flags().BoolVar(&opts.IncludePrivate, "include-private", true, "Include private repositories")
	cmd.Flags().BoolVar(&opts.UseSSH, "ssh", false, "Use SSH URLs for cloning")
	cmd.Flags().BoolVar(&opts.CleanupOrphans, "cleanup-orphans", false, "Delete directories not in organization")

	// Required flags
	_ = cmd.MarkFlagRequired("provider")
	_ = cmd.MarkFlagRequired("org")
	_ = cmd.MarkFlagRequired("target")

	return cmd
}

func (f CommandFactory) runForgeSync(cmd *cobra.Command, opts *ForgeCommandOptions) error {
	ctx := cmd.Context()

	// Create provider
	forgeProvider, err := createForgeProvider(opts)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Parse strategy
	strategy, err := reposync.ParseStrategy(opts.Strategy)
	if err != nil {
		return fmt.Errorf("invalid strategy: %w", err)
	}

	// Create ForgePlanner
	plannerConfig := reposync.ForgePlannerConfig{
		TargetPath:      opts.TargetPath,
		Organization:    opts.Organization,
		IsUser:          opts.IsUser,
		IncludeArchived: opts.IncludeArchived,
		IncludeForks:    opts.IncludeForks,
		IncludePrivate:  opts.IncludePrivate,
		UseSSH:          opts.UseSSH,
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

// createForgeProvider creates the appropriate provider based on options.
func createForgeProvider(opts *ForgeCommandOptions) (reposync.ForgeProvider, error) {
	switch opts.Provider {
	case "github":
		return github.NewProvider(opts.Token), nil

	case "gitlab":
		p, err := gitlab.NewProvider(opts.Token, opts.BaseURL)
		if err != nil {
			return nil, err
		}
		return forgeProviderAdapter{p}, nil

	case "gitea":
		return forgeProviderAdapter{gitea.NewProvider(opts.Token, opts.BaseURL)}, nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: github, gitlab, gitea)", opts.Provider)
	}
}

// forgeProviderAdapter adapts gitforge providers to ForgeProvider interface.
type forgeProviderAdapter struct {
	provider.Provider
}
