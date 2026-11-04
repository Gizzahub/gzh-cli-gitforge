// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// newConfigCmd creates the config command for forge-based config generation.
func (f CommandFactory) newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Generate config from Git forge",
		Long: cliutil.QuickStartHelp(`  # Generate config from GitLab
  gz-git forge config generate --provider gitlab --org devbox -o .gz-git.yaml

  # Then use with workspace
  gz-git workspace sync`),
		Args: cobra.NoArgs,
		// Runnable help keeps NoArgs effective: a bare parent still prints
		// this help, and an unknown or retired subcommand is refused with an
		// explicit unknown-command error (exit 1) instead of being swallowed
		// as help.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(f.newConfigGenerateCmd())

	return cmd
}
