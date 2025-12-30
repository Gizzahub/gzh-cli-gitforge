package cmd

import (
	"github.com/spf13/cobra"
)

// mergeCmd represents the merge command group
var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge and rebase operations",
	Long: `Perform Git merge and rebase operations with conflict detection.

This command provides subcommands for:
  - Merging branches with various strategies
  - Detecting potential conflicts before merge
  - Aborting in-progress merges
  - Rebasing branches with conflict handling`,
	Example: `  # Merge a branch
  gz-git merge do feature/new-feature

  # Detect conflicts before merging
  gz-git merge detect feature/new-feature main

  # Abort an in-progress merge
  gz-git merge abort

  # Rebase current branch
  gz-git merge rebase main`,
}

func init() {
	rootCmd.AddCommand(mergeCmd)
}
