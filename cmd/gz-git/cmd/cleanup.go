// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// cleanupCmd represents the cleanup command group.
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up various Git resources",
	Long: cliutil.QuickStartHelp(`  # Clean up merged branches (dry-run)
  gz-git cleanup branch --merged

  # Clean up stale branches (>30 days)
  gz-git cleanup branch --stale

  # BULK: Clean up all repos in directory
  gz-git cleanup branch --merged --force .`),
	Example: ``,
	Args:    cobra.NoArgs,
	// Runnable help keeps NoArgs effective: a bare parent still prints this
	// help, and an unknown or retired subcommand is refused with an explicit
	// unknown-command error (exit 1) instead of being swallowed as help.
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}
