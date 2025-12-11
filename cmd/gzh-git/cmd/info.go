package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info [path]",
	Short: "Show repository information",
	Long: `Display detailed information about a Git repository including:
  - Current branch
  - Remote URL
  - Upstream branch
  - Commits ahead/behind upstream

If no path is specified, the current directory is used.`,
	Example: `  # Show info for current directory
  gz-git info

  # Show info for specific repository
  gz-git info /path/to/repo`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func runInfo(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Determine repository path
	repoPath := "."
	if len(args) > 0 {
		repoPath = args[0]
	}

	// Get absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Create client
	client := repository.NewClient()

	// Check if it's a repository
	if !client.IsRepository(ctx, absPath) {
		return fmt.Errorf("not a git repository: %s", absPath)
	}

	// Open repository
	repo, err := client.Open(ctx, absPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Get info
	info, err := client.GetInfo(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Print info
	fmt.Printf("Repository: %s\n", repo.Path)
	fmt.Println()

	if info.Branch != "" {
		fmt.Printf("Branch:        %s\n", info.Branch)
	} else {
		fmt.Println("Branch:        (detached HEAD)")
	}

	if info.Commit != "" {
		fmt.Printf("Commit:        %s\n", info.Commit)
	}

	if info.Remote != "" {
		fmt.Printf("Remote:        %s\n", info.Remote)
	}

	if info.RemoteURL != "" {
		fmt.Printf("Remote URL:    %s\n", info.RemoteURL)
	}

	if info.Upstream != "" {
		fmt.Printf("Upstream:      %s\n", info.Upstream)

		if info.AheadBy > 0 || info.BehindBy > 0 {
			fmt.Printf("Ahead/Behind:  ")
			if info.AheadBy > 0 {
				fmt.Printf("+%d", info.AheadBy)
			}
			if info.BehindBy > 0 {
				if info.AheadBy > 0 {
					fmt.Printf(" / ")
				}
				fmt.Printf("-%d", info.BehindBy)
			}
			fmt.Println()
		}
	}

	// Get status
	status, err := client.GetStatus(ctx, repo)
	if err == nil {
		fmt.Println()
		if status.IsClean {
			fmt.Println("Status:        \x1b[32mclean\x1b[0m")
		} else {
			fmt.Println("Status:        \x1b[31mdirty\x1b[0m")

			totalChanges := len(status.ModifiedFiles) + len(status.StagedFiles) + len(status.UntrackedFiles)
			fmt.Printf("Changes:       %d files\n", totalChanges)

			if len(status.StagedFiles) > 0 {
				fmt.Printf("  Staged:      %d\n", len(status.StagedFiles))
			}
			if len(status.ModifiedFiles) > 0 {
				fmt.Printf("  Modified:    %d\n", len(status.ModifiedFiles))
			}
			if len(status.UntrackedFiles) > 0 {
				fmt.Printf("  Untracked:   %d\n", len(status.UntrackedFiles))
			}
			if len(status.ConflictFiles) > 0 {
				fmt.Printf("  Conflicts:   \x1b[31m%d\x1b[0m\n", len(status.ConflictFiles))
			}
		}
	}

	return nil
}
