// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// conflictCmd represents the conflict detection command group.
var conflictCmd = &cobra.Command{
	Use:   "conflict",
	Short: "Pre-merge conflict detection",
	Long: cliutil.QuickStartHelp(`  # Detect conflicts before merging
  gz-git conflict detect feature/new-feature main`),
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
	rootCmd.AddCommand(conflictCmd)
}
