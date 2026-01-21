package cmd

import (
	"github.com/spf13/cobra"
)

// cleanupCmd represents the cleanup command group
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up various Git resources",
	Long: `Clean up Git resources across single or multiple repositories.

This command provides subcommands for cleaning up:
  - branch: merged, stale, or gone branches

All cleanup commands support bulk mode for multi-repository operations.`,
	Example: `  # Clean up merged branches (dry-run)
  gz-git cleanup branch --merged

  # Clean up stale branches (no activity for 30 days)
  gz-git cleanup branch --stale

  # BULK: Clean up all repos in directory
  gz-git cleanup branch --merged --force .`,
	Args: cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}
