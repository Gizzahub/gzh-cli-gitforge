package cmd

import (
	"github.com/spf13/cobra"
)

// branchCmd represents the branch command group
var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Branch management commands",
	Long: `Manage Git branches, worktrees, and parallel development workflows.

This command provides subcommands for:
  - Creating and deleting branches
  - Managing worktrees for parallel development
  - Cleaning up merged and stale branches
  - Analyzing branch conflicts`,
	Example: `  # List all branches
  gzh-git branch list

  # Create a new branch
  gzh-git branch create feature/new-feature

  # Create branch with worktree
  gzh-git branch create feature/auth --worktree ./worktrees/auth

  # Clean up merged branches
  gzh-git branch cleanup --strategy merged`,
}

func init() {
	rootCmd.AddCommand(branchCmd)
}
