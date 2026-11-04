// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// historyCmd represents the history command group.
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "History analysis commands",
	Long: cliutil.QuickStartHelp(`  # Show commit statistics
  gz-git history stats

  # List top contributors
  gz-git history contributors --top 10

  # View file history
  gz-git history file src/main.go`),
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
	rootCmd.AddCommand(historyCmd)
}
