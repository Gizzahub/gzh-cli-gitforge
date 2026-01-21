// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"github.com/spf13/cobra"
)

// newConfigCmd creates the config command for forge-based config generation.
func (f CommandFactory) newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Generate config from Git forge",
		Long: `Generate configuration files from Git forge (GitHub, GitLab, Gitea).

The generated config can be used with 'gz-git workspace sync'.

For local config management (init, scan, validate), use 'gz-git workspace' instead.

Examples:
  # Generate config from GitLab
  gz-git sync config generate --provider gitlab --org devbox -o .gz-git.yaml

  # Then use with workspace
  gz-git workspace sync

  # Merge another org into existing config
  gz-git sync config merge --provider gitlab --org another-group --into sync.yaml

  # Validate config file
  gz-git sync config validate -c sync.yaml`,
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(f.newConfigGenerateCmd())

	return cmd
}
