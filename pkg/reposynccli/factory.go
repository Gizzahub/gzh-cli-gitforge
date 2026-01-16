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
		Long: `Git repository synchronization from various sources.

Sync Sources:
  from-forge   - Sync from Git forge (GitHub, GitLab, Gitea) API
  from-config  - Sync from YAML configuration file
  config       - Manage configuration files (generate, merge, validate)
  status       - Check repository health and sync status

Sync Strategies:
  reset  - Hard reset to remote HEAD (default, ensures clean state)
  pull   - Pull with rebase (preserves local changes)
  fetch  - Fetch only (update refs without modifying working tree)

Examples:
  # Check repository health before sync
  gz-git sync status -c sync.yaml

  # Sync directly from GitLab organization
  gz-git sync from-forge --provider gitlab --org devbox --target ~/repos

  # Generate config from GitLab
  gz-git sync config generate --provider gitlab --org devbox -o sync.yaml

  # Sync from config file
  gz-git sync from-config -c sync.yaml

  # Merge another org into existing config
  gz-git sync config merge --provider gitlab --org another-group --into sync.yaml`,
	}

	// New command structure (Option A)
	root.AddCommand(f.newFromForgeCmd())
	root.AddCommand(f.newFromConfigCmd())
	root.AddCommand(f.newConfigCmd())
	root.AddCommand(f.newStatusCmd())
	root.AddCommand(f.newSetupCmd())

	return root
}

func (f CommandFactory) orchestrator() (reposync.Runner, error) {
	if f.Orchestrator == nil {
		return nil, fmt.Errorf("orchestrator not configured")
	}
	return f.Orchestrator, nil
}
