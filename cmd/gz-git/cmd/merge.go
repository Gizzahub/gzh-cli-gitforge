package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// mergeCmd represents the merge command group
var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge conflict detection",
	Long: cliutil.QuickStartHelp(`  # Detect conflicts before merging
  gz-git merge detect feature/new-feature main`),
	Example: ``,
	Args:    cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(mergeCmd)
}
