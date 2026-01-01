package cmd

import (
	"github.com/spf13/cobra"
)

// mergeCmd represents the merge command group
var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge conflict detection",
	Long: `Detect potential merge conflicts before merging.

For basic merge operations, use git directly:
  git merge <branch>         # merge branch
  git merge --abort          # abort merge
  git rebase <branch>        # rebase`,
	Example: `  # Detect conflicts before merging
  gz-git merge detect feature/new-feature main`,
}

func init() {
	rootCmd.AddCommand(mergeCmd)
}
