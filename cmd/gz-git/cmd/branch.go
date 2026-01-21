package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// branchCmd represents the branch command group
var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Branch management commands",
	Long: cliutil.QuickStartHelp(`  # List branches in current repo
  gz-git branch list

  # List all branches including remote
  gz-git branch list -a

  # BULK: List branches across multiple repos
  gz-git branch list .

  # Clean up branches
  gz-git cleanup branch --merged`),
	Example: ``,
	Args:    cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(branchCmd)
}
