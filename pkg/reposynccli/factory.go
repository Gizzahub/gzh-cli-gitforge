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
		Long: `Git repository synchronization from Git forges (GitHub, GitLab, Gitea).

Commands:
  from-forge   - Sync repositories from Git forge API
  config       - Generate config from forge (for use with 'workspace' command)
  status       - Check repository health after sync

For local config-based operations, use 'gz-git workspace' command instead.

Sync Strategies:
  reset  - Hard reset to remote HEAD (default, ensures clean state)
  pull   - Pull with rebase (preserves local changes)
  fetch  - Fetch only (update refs without modifying working tree)

Examples:
  # Sync directly from GitLab organization
  gz-git sync from-forge --provider gitlab --org devbox --target ~/repos

  # Generate config from GitLab (then use with workspace)
  gz-git sync config generate --provider gitlab --org devbox -o .gz-git.yaml
  gz-git workspace sync

  # Check repository health
  gz-git sync status --target ~/repos`,
	}

	root.AddCommand(f.newFromForgeCmd())
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
