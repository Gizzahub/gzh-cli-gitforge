package cmd

import (
	"github.com/spf13/cobra"
)

// branchCmd represents the branch command group
var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Branch management commands",
	Long: `Manage Git branches across single or multiple repositories.

For basic branch operations (create, delete, list), use git directly:
  git checkout -b <name>     # create branch
  git branch -d <name>       # delete branch
  git branch -a              # list branches

For branch cleanup, use: gz-git cleanup branch`,
	Example: `  # List branches
  git branch -a

  # Clean up branches (new location)
  gz-git cleanup branch --merged`,
}

func init() {
	rootCmd.AddCommand(branchCmd)
}
