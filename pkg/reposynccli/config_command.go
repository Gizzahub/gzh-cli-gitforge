// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"github.com/spf13/cobra"
)

// newConfigCmd creates the root config management command.
func (f CommandFactory) newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage sync configuration files",
		Long: `Manage sync configuration files for repository synchronization.

Subcommands:
  init      - Create a sample configuration file
  generate  - Generate config from Git forge (GitHub, GitLab, Gitea)
  merge     - Merge repositories from forge into existing config
  validate  - Validate configuration file format

Examples:
  # Create sample config
  gz-git sync config init

  # Generate config from GitLab
  gz-git sync config generate --provider gitlab --org devbox -o sync.yaml

  # Merge another org into existing config
  gz-git sync config merge --provider gitlab --org another-group --into sync.yaml

  # Validate config file
  gz-git sync config validate -c sync.yaml`,
	}

	cmd.AddCommand(f.newConfigInitCmd())
	cmd.AddCommand(f.newConfigGenerateCmd())
	cmd.AddCommand(f.newConfigMergeCmd())
	cmd.AddCommand(f.newConfigValidateCmd())

	return cmd
}
