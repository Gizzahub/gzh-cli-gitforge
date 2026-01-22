// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
)

func (f CommandFactory) newGenerateCmd() *cobra.Command {
	opts := &reposynccli.ConfigGenerateOptions{
		Strategy:     "reset",
		Parallel:     4,
		MaxRetries:   3,
		CloneProto:   "ssh",
		SubgroupMode: "flat",
		Output:       "sync.yaml",
	}

	cmd := &cobra.Command{
		Use:   "generate-config",
		Short: "Generate workspace config from Git forge",
		Long: cliutil.QuickStartHelp(`  # Generate config from GitLab
  gz-git workspace generate-config --provider gitlab --org devbox -o .gz-git.yaml \
    --token $GITLAB_TOKEN --path ~/repos

  # Include subgroups with flat naming
  gz-git workspace generate-config --provider gitlab --org parent-group \
    --include-subgroups --subgroup-mode flat -o .gz-git.yaml \
    --token $GITLAB_TOKEN --path ~/repos

  # Generate from GitHub
  gz-git workspace generate-config --provider github --org myorg \
    --clone-proto ssh -o .gz-git.yaml \
    --token $GITHUB_TOKEN --path ~/repos`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return reposynccli.RunConfigGenerate(cmd, opts)
		},
	}

	// Provider and path (required)
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "Git forge provider: github, gitlab, gitea [required]")
	cmd.Flags().StringVar(&opts.Organization, "org", "", "Organization/group name [required]")
	cmd.Flags().StringVar(&opts.Path, "path", "", "Directory for cloned repositories [required]")
	cmd.Flags().StringVar(&opts.Path, "target", "", "Deprecated: use --path")
	_ = cmd.Flags().MarkDeprecated("target", "use --path instead")
	cmd.Flags().BoolVar(&opts.IsUser, "user", false, "Treat --org as a user instead of organization")

	// Authentication
	cmd.Flags().StringVar(&opts.Token, "token", "", "API token for authentication")
	cmd.Flags().StringVar(&opts.BaseURL, "base-url", "", "Base URL for self-hosted instances (API endpoint)")

	// Clone options
	cmd.Flags().StringVar(&opts.CloneProto, "clone-proto", opts.CloneProto, "Clone protocol: ssh, https")
	cmd.Flags().IntVar(&opts.SSHPort, "ssh-port", opts.SSHPort, "Custom SSH port (0 = default 22)")

	// Strategy and execution
	cmd.Flags().StringVar(&opts.Strategy, "sync-strategy", opts.Strategy, "Sync strategy (reset, pull, fetch)")
	cmd.Flags().StringVar(&opts.Strategy, "strategy", opts.Strategy, "Deprecated: use --sync-strategy")
	_ = cmd.Flags().MarkDeprecated("strategy", "use --sync-strategy instead")
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
	_ = cmd.MarkFlagRequired("provider")
	_ = cmd.MarkFlagRequired("org")
	_ = cmd.MarkFlagRequired("path")

	return cmd
}
