package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

var statusPath string

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [path]",
	Short: "Show the working tree status",
	Long: `Display the status of the working tree including:
  - Modified files (unstaged changes)
  - Staged files (ready to commit)
  - Untracked files (not tracked by Git)
  - Deleted files
  - Renamed files
  - Conflict files (merge conflicts)

If no path is specified, the current directory is used.`,
	Example: `  # Show status of current directory
  gzh-git status

  # Show status of specific repository
  gzh-git status /path/to/repo

  # Quiet output (only show if dirty)
  gzh-git status -q`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Local flags
	statusCmd.Flags().StringVarP(&statusPath, "path", "p", ".", "path to repository")
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Determine repository path
	repoPath := statusPath
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

	// Get status
	status, err := client.GetStatus(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	// Check for special repository states
	rebaseInProgress := repository.IsRebaseInProgress(absPath)
	mergeInProgress := repository.IsMergeInProgress(absPath)

	// Get repository info
	info, err := client.GetInfo(ctx, repo)
	if err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to get repository info: %v\n", err)
		}
		// Continue without info
	}

	// Output
	if quiet {
		// Quiet mode: only show if dirty
		if !status.IsClean {
			os.Exit(1)
		}
		return nil
	}

	// Show special states first (rebase/merge in progress)
	if rebaseInProgress {
		fmt.Println("\x1b[33m↻ Rebase in progress\x1b[0m")
		fmt.Println("  (use \"git rebase --continue\" to continue)")
		fmt.Println("  (use \"git rebase --abort\" to abort)")
		fmt.Println()
	}

	if mergeInProgress {
		fmt.Println("\x1b[33m⇄ Merge in progress\x1b[0m")
		fmt.Println("  (fix conflicts and run \"git commit\")")
		fmt.Println("  (use \"git merge --abort\" to abort)")
		fmt.Println()
	}

	// Print repository info
	if info != nil && info.Branch != "" {
		fmt.Printf("On branch %s\n", info.Branch)

		if info.Upstream != "" {
			fmt.Printf("Your branch is tracking '%s'\n", info.Upstream)

			if info.AheadBy > 0 && info.BehindBy > 0 {
				fmt.Printf("  and is %d commits ahead, %d commits behind\n", info.AheadBy, info.BehindBy)
			} else if info.AheadBy > 0 {
				fmt.Printf("  and is %d commits ahead\n", info.AheadBy)
			} else if info.BehindBy > 0 {
				fmt.Printf("  and is %d commits behind\n", info.BehindBy)
			} else {
				fmt.Println("  and is up to date")
			}
		}
		fmt.Println()
	}

	// Print status
	if status.IsClean {
		fmt.Println("Working tree is clean")
		return nil
	}

	// Staged files
	if len(status.StagedFiles) > 0 {
		fmt.Println("Changes to be committed:")
		fmt.Println("  (use \"git restore --staged <file>...\" to unstage)")
		fmt.Println()
		for _, file := range status.StagedFiles {
			fmt.Printf("  \x1b[32mmodified:   %s\x1b[0m\n", file)
		}
		fmt.Println()
	}

	// Modified files (unstaged)
	if len(status.ModifiedFiles) > 0 {
		fmt.Println("Changes not staged for commit:")
		fmt.Println("  (use \"git add <file>...\" to update what will be committed)")
		fmt.Println()
		for _, file := range status.ModifiedFiles {
			fmt.Printf("  \x1b[31mmodified:   %s\x1b[0m\n", file)
		}
		fmt.Println()
	}

	// Deleted files
	if len(status.DeletedFiles) > 0 {
		fmt.Println("Deleted files:")
		for _, file := range status.DeletedFiles {
			fmt.Printf("  \x1b[31mdeleted:    %s\x1b[0m\n", file)
		}
		fmt.Println()
	}

	// Renamed files
	if len(status.RenamedFiles) > 0 {
		fmt.Println("Renamed files:")
		for _, file := range status.RenamedFiles {
			fmt.Printf("  \x1b[33mrenamed:    %s -> %s\x1b[0m\n", file.OldPath, file.NewPath)
		}
		fmt.Println()
	}

	// Untracked files
	if len(status.UntrackedFiles) > 0 {
		fmt.Println("Untracked files:")
		fmt.Println("  (use \"git add <file>...\" to include in what will be committed)")
		fmt.Println()
		for _, file := range status.UntrackedFiles {
			fmt.Printf("  \x1b[31m%s\x1b[0m\n", file)
		}
		fmt.Println()
	}

	// Conflict files
	if len(status.ConflictFiles) > 0 {
		fmt.Printf("\x1b[31m⚡ Unresolved conflicts (%d file(s)):\x1b[0m\n", len(status.ConflictFiles))
		fmt.Println("  (fix conflicts and run \"git add <file>...\")")
		fmt.Println()
		for _, file := range status.ConflictFiles {
			fmt.Printf("  \x1b[31mboth modified:   %s\x1b[0m\n", file)
		}
		fmt.Println()
	}

	return nil
}
