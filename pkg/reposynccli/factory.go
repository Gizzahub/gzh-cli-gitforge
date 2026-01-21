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

Sync Strategies (choose based on use case):
  reset  - Hard reset to remote HEAD (default, ensures clean state)
           Use for: Clean workspaces, CI/CD, fresh clones
  pull   - Pull with rebase (preserves local changes)
           Use for: Active development, preserve local commits
  fetch  - Fetch only (update refs without modifying working tree)
           Use for: Status checks, pre-merge verification
  skip   - Skip sync (clone only if missing)
           Use for: Submodules, archived repos

SSH Port Auto-Detection:
  GitLab API provides correct SSH URLs with ports in 'ssh_url_to_repo' field.
  No manual --ssh-port needed for GitLab (API provides it automatically).
  For other providers, use --ssh-port if non-standard (not 22).

Recommended Workflow:

  1. Initial Setup (one-time):
     gz-git config profile create work --provider gitlab
     gz-git config profile use work

  2. Generate Config (from forge):
     gz-git sync config generate --org myteam -o .gz-git.yaml

  3. Check Health (before sync):
     gz-git workspace status

  4. Sync Repositories:
     gz-git workspace sync

  5. Regular Updates:
     gz-git workspace sync --strategy pull

Config Management Workflows:

  • Scan Local Directory → Config:
    gz-git workspace scan ~/mydevbox -o .gz-git.yaml

  • Forge API → Config:
    gz-git sync config generate --provider gitlab --org team -o .gz-git.yaml

  • Use workspace command for local operations:
    gz-git workspace sync -c .gz-git.yaml

Examples:
  # Check repository health before sync
  gz-git sync status --target ~/repos

  # Sync directly from GitLab (SSH, auto-detect port)
  gz-git sync from-forge --provider gitlab --org devbox --target ~/repos

  # Sync with HTTPS clone instead of SSH
  gz-git sync from-forge --provider gitlab --org devbox --clone-proto https

  # Generate config from GitLab with subgroups (flat mode)
  gz-git sync config generate --provider gitlab --org parent \
    --include-subgroups --subgroup-mode flat -o .gz-git.yaml

  # Use workspace command for local config-based sync
  gz-git workspace sync

  # Override strategy for workspace sync
  gz-git workspace sync --strategy pull`,
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
