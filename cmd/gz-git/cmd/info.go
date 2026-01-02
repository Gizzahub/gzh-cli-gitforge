package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
	"github.com/spf13/cobra"
)

var infoVerbose bool

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info [path]",
	Short: "Display repository information",
	Long: `Display information about the current Git repository.

Shows basic repository details like current branch, status, and remote info.
Use --verbose for additional details.`,
	Example: `  # Show info for current directory
  gz-git info

  # Show info for specific repository
  gz-git info /path/to/repo

  # Verbose output with more details
  gz-git info --verbose`,
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().BoolVarP(&infoVerbose, "verbose", "v", false, "show verbose output")
}

func runInfo(cmd *cobra.Command, args []string) error {
	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

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

	// Create client and open repository
	client := repository.NewClient()

	repo, err := client.Open(ctx, absPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Get repository info
	info, err := client.GetInfo(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Get repository status for clean/dirty
	status, err := client.GetStatus(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to get repository status: %w", err)
	}

	// Display info
	displayInfo(absPath, info, status, infoVerbose)

	return nil
}

func displayInfo(path string, info *repository.Info, status *repository.Status, verbose bool) {
	fmt.Printf("Repository: %s\n", path)
	fmt.Printf("Branch: %s\n", info.Branch)

	// Status line
	if status.IsClean {
		fmt.Println("Status: clean")
	} else {
		fmt.Println("Status: dirty")
	}

	if verbose {
		fmt.Printf("Commit: %s\n", info.Commit)

		if info.Remote != "" {
			fmt.Printf("Remote: %s\n", info.Remote)
		}
		if info.RemoteURL != "" {
			fmt.Printf("Remote URL: %s\n", info.RemoteURL)
		}
		if info.Upstream != "" {
			fmt.Printf("Upstream: %s\n", info.Upstream)
		}

		// Ahead/behind info
		if info.AheadBy > 0 || info.BehindBy > 0 {
			fmt.Printf("Ahead: %d, Behind: %d\n", info.AheadBy, info.BehindBy)
		}

		// File counts if dirty
		if !status.IsClean {
			if len(status.StagedFiles) > 0 {
				fmt.Printf("Staged: %d files\n", len(status.StagedFiles))
			}
			if len(status.ModifiedFiles) > 0 {
				fmt.Printf("Modified: %d files\n", len(status.ModifiedFiles))
			}
			if len(status.UntrackedFiles) > 0 {
				fmt.Printf("Untracked: %d files\n", len(status.UntrackedFiles))
			}
		}
	}
}
