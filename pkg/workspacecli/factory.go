// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// DefaultConfigFile is the default workspace config filename.
const DefaultConfigFile = ".gz-git.yaml"

// CommandFactory builds workspace CLI commands.
type CommandFactory struct {
	Orchestrator reposync.Runner
}

// NewRootCmd returns the workspace root command.
func (f CommandFactory) NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "workspace",
		Short: "Manage local workspace with config-based repository sync",
		Long: `Manage local workspace with config-based repository synchronization.

Workspace commands handle local config file operations for managing
multiple git repositories as a workspace.

Commands:
  init      - Create empty config file
  scan      - Scan directory for git repos and generate config
  sync      - Clone/update repositories based on config
  status    - Check workspace health
  add       - Add repository to config
  validate  - Validate config file

Config File:
  Default: .gz-git.yaml (in current directory)
  Custom:  Use -c/--config flag to specify different file

Examples:
  # Initialize new workspace
  gz-git workspace init

  # Scan existing repos and create config
  gz-git workspace scan ~/mydevbox -o .gz-git.yaml

  # Sync repositories
  gz-git workspace sync

  # Check workspace status
  gz-git workspace status

  # Use custom config file
  gz-git workspace sync -c myworkspace.yaml`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(f.newInitCmd())
	root.AddCommand(f.newScanCmd())
	root.AddCommand(f.newSyncCmd())
	root.AddCommand(f.newStatusCmd())
	root.AddCommand(f.newAddCmd())
	root.AddCommand(f.newValidateCmd())

	return root
}

func (f CommandFactory) orchestrator() (reposync.Runner, error) {
	if f.Orchestrator == nil {
		return nil, fmt.Errorf("orchestrator not configured")
	}
	return f.Orchestrator, nil
}
