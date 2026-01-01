package cmd

import (
	"github.com/spf13/cobra"
)

// branchCmd represents the branch command group
var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Branch management commands",
	Long: `Manage Git branches across single or multiple repositories.

This command provides subcommands for:
  - Cleaning up merged, stale, or gone branches (with bulk support)

For basic branch operations (create, delete, list), use git directly:
  git checkout -b <name>     # create branch
  git branch -d <name>       # delete branch
  git branch -a              # list branches`,
	Example: `  # Clean up merged branches (dry-run)
  gz-git branch cleanup --merged --dry-run

  # BULK: Clean up all repos
  gz-git branch cleanup --merged ..`,
}

func init() {
	rootCmd.AddCommand(branchCmd)
}
