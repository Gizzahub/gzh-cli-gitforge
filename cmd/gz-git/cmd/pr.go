// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Create and inspect pull requests / merge requests",
	Long: cliutil.QuickStartHelp(`  # Create PRs from the current branch of every scanned repo
  gz-git pr create

  # Dry-run a workspace
  gz-git pr create -n ~/work`),
	Args: cobra.NoArgs,
	// Runnable help keeps NoArgs effective: a bare parent still prints this
	// help, and an unknown or retired subcommand is refused with an explicit
	// unknown-command error (exit 1) instead of being swallowed as help.
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(prCmd)
}
