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
}

func init() {
	rootCmd.AddCommand(conflictCmd)
}
