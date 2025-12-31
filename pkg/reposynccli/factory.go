// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// CommandFactory builds a Cobra command tree that can be embedded into other CLIs.
type CommandFactory struct {
	Use   string
	Short string

	Orchestrator reposync.Runner
	SpecLoader   SpecLoader

	Version   string
	Commit    string
	BuildDate string
}

// NewRootCmd returns a root command suitable for standalone binary usage.
func (f CommandFactory) NewRootCmd() *cobra.Command {
	use := f.Use
	if use == "" {
		use = "git-sync"
	}

	short := f.Short
	if short == "" {
		short = "Git repository synchronization"
	}

	root := &cobra.Command{
		Use:           use,
		Short:         short,
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `Git repository synchronization for filesystem and forge providers.

Two sync modes available:

  forge  - Sync all repositories from a Git forge (GitHub, GitLab, Gitea)
           Fetches repository list from the provider API and syncs locally.

  run    - Sync repositories from a YAML configuration file.
           Useful for managing a specific set of repositories.

Sync Strategies:
  reset  - Hard reset to remote HEAD (default, ensures clean state)
  pull   - Pull with rebase (preserves local changes)
  fetch  - Fetch only (update refs without modifying working tree)

Examples:
  # Sync all repos from a GitHub organization
  gz-git sync forge --provider github --org myorg --target ./repos --token $GITHUB_TOKEN

  # Sync from self-hosted GitLab
  gz-git sync forge --provider gitlab --org mygroup --target ./repos --base-url https://gitlab.company.com

  # Sync repos defined in a config file
  gz-git sync run -c sync-config.yaml

  # Preview sync without making changes
  gz-git sync run -c sync-config.yaml --dry-run`,
	}

	root.AddCommand(f.newRunCmd())
	root.AddCommand(f.newForgeCmd())

	return root
}

func (f CommandFactory) orchestrator() (reposync.Runner, error) {
	if f.Orchestrator == nil {
		return nil, fmt.Errorf("orchestrator not configured")
	}
	return f.Orchestrator, nil
}
