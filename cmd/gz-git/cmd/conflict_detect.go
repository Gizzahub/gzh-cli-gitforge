package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/merge"
)

var (
	detectIncludeBinary bool
	detectBaseCommit    string
)

// detectCmd represents the conflict detect command.
var detectCmd = &cobra.Command{
	Use:   "detect <source> <target>",
	Short: "Detect potential merge conflicts",
	Long: cliutil.QuickStartHelp(`  # Detect conflicts between branches
  gz-git conflict detect feature/new-feature main

  # Include binary file conflicts
  gz-git conflict detect feature/new-feature main --include-binary

  # Detect with specific base commit
  gz-git conflict detect feature/new-feature main --base abc123`) + cliutil.ExitCodesConflictHelp(),
	Example: ``,
	Args:    cobra.ExactArgs(2),
	RunE:    runConflictDetect,
}

func init() {
	conflictCmd.AddCommand(detectCmd)

	detectCmd.Flags().BoolVar(&detectIncludeBinary, "include-binary", false, "include binary file conflicts")
	detectCmd.Flags().StringVar(&detectBaseCommit, "base", "", "base commit for three-way merge")
}

// runConflictDetect follows the grep-style exit convention, NOT the bulk
// convention: 0 = no conflict (clean), 1 = conflict found, 2 = execution error.
// This lets scripts branch on "is there a conflict?" via the exit code alone.
func runConflictDetect(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	source := args[0]
	target := args[1]

	repo, err := openCurrentRepo(ctx)
	if err != nil {
		return cliutil.NewExitError(2, err)
	}

	detector := merge.NewConflictDetector(gitcmd.NewExecutor())

	// Build options
	opts := merge.DetectOptions{
		Source:        source,
		Target:        target,
		BaseCommit:    detectBaseCommit,
		IncludeBinary: detectIncludeBinary,
	}

	if !quiet {
		fmt.Printf("Analyzing conflicts: %s → %s\n", source, target)
	}

	// Detect conflicts
	report, err := detector.Detect(ctx, repo, opts)
	if err != nil {
		return cliutil.NewExitError(2, fmt.Errorf("failed to detect conflicts: %w", err))
	}

	// Display report
	if !quiet {
		fmt.Println()
		if report.TotalConflicts > 0 {
			fmt.Printf("⚠ Found %d potential conflicts:\n\n", report.TotalConflicts)
			for _, conflict := range report.Conflicts {
				fmt.Printf("  %s: %s\n", conflict.ConflictType, conflict.FilePath)
				if verbose && conflict.Description != "" {
					fmt.Printf("     Reason: %s\n", conflict.Description)
				}
			}
			fmt.Println()
			fmt.Printf("Difficulty: %s\n", report.Difficulty)
			fmt.Printf("Auto-resolvable: %d/%d\n", report.CanAutoResolve, report.TotalConflicts)
		} else {
			fmt.Println("✓ No conflicts detected - merge should be clean!")
		}

		// Check fast-forward
		canFF, err := detector.CanFastForward(ctx, repo, source, target)
		if err == nil && canFF {
			fmt.Println()
			fmt.Println("Tip: This merge can be fast-forwarded")
		}
	}

	if report.TotalConflicts > 0 {
		return cliutil.NewExitError(1, fmt.Errorf("merge would result in conflicts"))
	}

	return nil
}
