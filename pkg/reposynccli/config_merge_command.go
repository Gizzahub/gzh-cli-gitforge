// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"github.com/spf13/cobra"
)

func (f CommandFactory) newConfigMergeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge repositories from forge into existing config",
		Long: `Merge repositories from a Git forge into an existing configuration file.

This command queries the forge API and adds new repositories to the config
file without duplicating existing entries.

Examples:
  # Merge another org into existing config
  gz-git sync config merge --provider gitlab --org another-group --into sync.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// TODO: Implement config merging

	return cmd
}
