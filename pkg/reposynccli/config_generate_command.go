// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"github.com/spf13/cobra"
)

func (f CommandFactory) newConfigGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate config from Git forge (GitHub, GitLab, Gitea)",
		Long: `Generate a configuration file from a Git forge organization.

This command queries the forge API to list all repositories and creates
a YAML configuration file ready for use with 'sync from-config'.

Examples:
  # Generate config from GitLab
  gz-git sync config generate --provider gitlab --org devbox -o sync.yaml

  # Include subgroups with flat naming
  gz-git sync config generate --provider gitlab --org parent-group \
    --include-subgroups --subgroup-mode flat -o sync.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// TODO: Implement config generation from forge
	// Similar to from-forge but outputs YAML instead of executing sync

	return cmd
}
