package cmd

import (
	"github.com/spf13/cobra"
)

// branchCmd represents the branch command group
var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Branch management commands",
	Long: `Quick Start:
  # List branches in current repo
  gz-git branch list

  # List all branches including remote
  gz-git branch list -a

  # BULK: List branches across multiple repos
  gz-git branch list .

  # Clean up branches
  gz-git cleanup branch --merged`,
	Example: ``,
	Args:    cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(branchCmd)
}
