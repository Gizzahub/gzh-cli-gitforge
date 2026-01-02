package cmd

import (
	"github.com/spf13/cobra"
)

// branchCmd represents the branch command group
var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Branch management commands",
	Long: `Manage Git branches across single or multiple repositories.

Subcommands:
  list    - List branches (single or bulk mode)
  cleanup - Clean up merged/stale/gone branches (via gz-git cleanup branch)

For basic branch operations (create, delete), use git directly:
  git checkout -b <name>     # create branch
  git branch -d <name>       # delete branch`,
	Example: `  # List branches in current repo
  gz-git branch list

  # List all branches including remote
  gz-git branch list -a

  # BULK: List branches across multiple repos
  gz-git branch list .

  # Clean up branches
  gz-git cleanup branch --merged`,
}

func init() {
	rootCmd.AddCommand(branchCmd)
}
