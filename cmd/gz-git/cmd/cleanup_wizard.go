// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/wizard"
)

// cleanupWizardCmd represents the cleanup wizard command
var cleanupWizardCmd = &cobra.Command{
	Use:   "wizard [directory]",
	Short: "Interactive wizard for branch cleanup",
	Long: `Interactive wizard for cleaning up branches across repositories.

This wizard guides you through:
  1. Selecting cleanup types (merged, stale, gone branches)
  2. Scanning repositories
  3. Reviewing and selecting branches to delete
  4. Executing the cleanup

The wizard provides a safe, interactive way to clean up branches
with clear visibility into what will be deleted.

Examples:
  # Start wizard in current directory
  gz-git cleanup wizard

  # Start wizard in specific directory
  gz-git cleanup wizard ~/projects

  # The wizard will interactively ask about:
  # - Which types of branches to clean (merged, stale, gone)
  # - Stale threshold (days without activity)
  # - Whether to include remote branches
  # - Which specific branches to delete`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCleanupWizard,
}

func init() {
	cleanupCmd.AddCommand(cleanupWizardCmd)
}

func runCleanupWizard(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Get directory from args or use current
	directory := "."
	if len(args) > 0 {
		directory = args[0]
	}

	// Run the wizard
	w := wizard.NewBranchCleanupWizard(directory)
	result, err := w.Run(ctx)
	if err != nil {
		return fmt.Errorf("wizard failed: %w", err)
	}

	// Print final message
	if result.BranchesDeleted > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nCleanup completed successfully!\n")
	}

	return nil
}
