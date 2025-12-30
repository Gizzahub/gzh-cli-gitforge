package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	pullStrategy string
	pullPrune    bool
	pullTags     bool
	pullStash    bool
	pullDryRun   bool
	pullRebase   bool
	pullFFOnly   bool
)

// pullCmd represents the pull command (single repository, git-style)
var pullCmd = &cobra.Command{
	Use:   "pull [remote] [branch]",
	Short: "Pull updates from remote repository",
	Long: `Pull updates from a remote repository and integrate with current branch.

This command works like 'git pull' - it fetches from remote and integrates
changes into the current branch. It operates on the current directory's Git repository.

Arguments:
  remote   - Remote name (default: origin)
  branch   - Branch to pull from (default: upstream tracking branch)

Pull strategies:
  --rebase    - Rebase current branch on top of fetched changes
  --ff-only   - Only fast-forward, fail if not possible
  (default)   - Merge fetched changes

For bulk operations across multiple repositories, use 'pull-bulk' instead.`,
	Example: `  # Pull from origin (current branch's upstream)
  gz-git pull

  # Pull from specific remote
  gz-git pull upstream

  # Pull specific branch
  gz-git pull origin main

  # Pull with rebase
  gz-git pull --rebase origin main

  # Pull with fast-forward only
  gz-git pull --ff-only origin

  # Pull and prune deleted remote branches
  gz-git pull --prune origin

  # Pull and fetch all tags
  gz-git pull --tags origin

  # Automatically stash local changes before pull
  gz-git pull --stash origin

  # Dry run - show what would be pulled
  gz-git pull --dry-run origin`,
	Args: cobra.MaximumNArgs(2),
	RunE: runPull,
}

func init() {
	rootCmd.AddCommand(pullCmd)

	// Pull flags
	pullCmd.Flags().BoolVarP(&pullRebase, "rebase", "r", false, "rebase current branch on top of upstream")
	pullCmd.Flags().BoolVar(&pullFFOnly, "ff-only", false, "only fast-forward, fail if not possible")
	pullCmd.Flags().BoolVarP(&pullPrune, "prune", "p", false, "prune remote-tracking branches that no longer exist")
	pullCmd.Flags().BoolVarP(&pullTags, "tags", "t", false, "fetch all tags from remote")
	pullCmd.Flags().BoolVar(&pullStash, "stash", false, "automatically stash local changes before pull")
	pullCmd.Flags().BoolVarP(&pullDryRun, "dry-run", "n", false, "dry run - show what would be pulled")
}

func runPull(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	startTime := time.Now()

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Parse arguments
	remote, branch := parsePullArgs(args)

	// Check if current directory is a git repository
	client := repository.NewClient()
	if !client.IsRepository(ctx, cwd) {
		return fmt.Errorf("not a git repository")
	}

	// Build git pull command
	executor := gitcmd.NewExecutor()

	// Show what we're about to do
	if !quiet {
		displayPullIntent(remote, branch)
	}

	// Stash local changes if requested
	stashed := false
	if pullStash {
		hasChanges, err := hasUncommittedChanges(ctx, executor, cwd)
		if err != nil {
			return fmt.Errorf("failed to check for changes: %w", err)
		}
		if hasChanges {
			if err := stashChanges(ctx, executor, cwd); err != nil {
				return fmt.Errorf("failed to stash changes: %w", err)
			}
			stashed = true
			if !quiet {
				fmt.Println("  Stashed local changes")
			}
		}
	}

	// Execute pull
	if err := executeSinglePull(ctx, executor, cwd, remote, branch); err != nil {
		// Try to restore stash on error
		if stashed {
			_ = unstashChanges(ctx, executor, cwd)
		}
		return err
	}

	// Restore stashed changes
	if stashed {
		if err := unstashChanges(ctx, executor, cwd); err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to restore stashed changes: %v\n", err)
				fmt.Fprintln(os.Stderr, "  Use 'git stash pop' to restore manually")
			}
		} else if !quiet {
			fmt.Println("  Restored stashed changes")
		}
	}

	// Show success
	duration := time.Since(startTime)
	if !quiet {
		displayPullSuccess(remote, branch, duration)
	}

	return nil
}

// parsePullArgs parses command arguments into remote and branch
func parsePullArgs(args []string) (remote, branch string) {
	remote = "origin" // default remote

	switch len(args) {
	case 0:
		return remote, ""
	case 1:
		return args[0], ""
	case 2:
		return args[0], args[1]
	}

	return remote, ""
}

// hasUncommittedChanges checks if there are uncommitted changes
func hasUncommittedChanges(ctx context.Context, executor *gitcmd.Executor, repoPath string) (bool, error) {
	result, err := executor.Run(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		return false, err
	}

	return len(result.Stdout) > 0, nil
}

// stashChanges stashes local changes
func stashChanges(ctx context.Context, executor *gitcmd.Executor, repoPath string) error {
	result, err := executor.Run(ctx, repoPath, "stash", "push", "-m", "gz-git pull auto-stash")
	if err != nil {
		return err
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git stash failed: %s", result.Stderr)
	}

	return nil
}

// unstashChanges restores stashed changes
func unstashChanges(ctx context.Context, executor *gitcmd.Executor, repoPath string) error {
	result, err := executor.Run(ctx, repoPath, "stash", "pop")
	if err != nil {
		return err
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git stash pop failed: %s", result.Stderr)
	}

	return nil
}

// executeSinglePull performs pull from a single remote
func executeSinglePull(ctx context.Context, executor *gitcmd.Executor, repoPath, remote, branch string) error {
	args := []string{"pull"}

	if pullDryRun {
		args = append(args, "--dry-run")
	}

	if pullRebase {
		args = append(args, "--rebase")
	} else if pullFFOnly {
		args = append(args, "--ff-only")
	}

	if pullPrune {
		args = append(args, "--prune")
	}

	if pullTags {
		args = append(args, "--tags")
	}

	args = append(args, remote)

	if branch != "" {
		args = append(args, branch)
	}

	result, err := executor.Run(ctx, repoPath, args...)
	if err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git pull failed: %s", result.Stderr)
	}

	return nil
}

// displayPullIntent shows what the pull command will do
func displayPullIntent(remote, branch string) {
	var action string
	if pullDryRun {
		action = "Would pull"
	} else {
		action = "Pulling"
	}

	target := remote
	if branch != "" {
		target += "/" + branch
	}

	strategy := ""
	if pullRebase {
		strategy = " (rebase)"
	} else if pullFFOnly {
		strategy = " (ff-only)"
	}

	flags := []string{}
	if pullPrune {
		flags = append(flags, "--prune")
	}
	if pullTags {
		flags = append(flags, "--tags")
	}
	if pullStash {
		flags = append(flags, "--stash")
	}

	flagStr := ""
	if len(flags) > 0 {
		flagStr = " " + joinStrings(flags, " ")
	}

	fmt.Printf("%s from %s%s%s\n", action, target, strategy, flagStr)
}

// displayPullSuccess shows pull completion message
func displayPullSuccess(remote, branch string, duration time.Duration) {
	if pullDryRun {
		fmt.Println("✓ Dry run complete (no changes made)")
		return
	}

	target := remote
	if branch != "" {
		target += "/" + branch
	}

	fmt.Printf("✓ Pulled from %s (%s)\n", target, duration.Round(time.Millisecond))
}
