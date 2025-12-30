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
	fetchAllRemotes bool
	fetchPrune      bool
	fetchTags       bool
	fetchDryRun     bool
	fetchDepth      int
)

// fetchCmd represents the fetch command (single repository, git-style)
var fetchCmd = &cobra.Command{
	Use:   "fetch [remote] [refspec]",
	Short: "Fetch updates from remote repository",
	Long: `Fetch updates from a remote repository.

This command works like 'git fetch' and downloads objects and refs from a remote.
It operates on the current directory's Git repository.

Arguments:
  remote   - Remote name (default: origin)
  refspec  - Branch or refspec to fetch (optional)

This command is safe to run as it only updates remote-tracking branches
and does not modify your working tree or local branches.

For bulk operations across multiple repositories, use 'fetch-bulk' instead.`,
	Example: `  # Fetch from origin
  gz-git fetch

  # Fetch from specific remote
  gz-git fetch upstream

  # Fetch specific branch
  gz-git fetch origin main

  # Fetch and prune deleted remote branches
  gz-git fetch --prune origin

  # Fetch all tags
  gz-git fetch --tags origin

  # Fetch from all configured remotes
  gz-git fetch --all-remotes

  # Dry run - show what would be fetched
  gz-git fetch --dry-run origin

  # Shallow fetch with depth limit
  gz-git fetch --depth 1 origin`,
	Args: cobra.MaximumNArgs(2),
	RunE: runFetch,
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	// Fetch flags
	fetchCmd.Flags().BoolVar(&fetchAllRemotes, "all-remotes", false, "fetch from all configured remotes")
	fetchCmd.Flags().BoolVarP(&fetchPrune, "prune", "p", false, "prune remote-tracking branches that no longer exist")
	fetchCmd.Flags().BoolVarP(&fetchTags, "tags", "t", false, "fetch all tags from remote")
	fetchCmd.Flags().BoolVarP(&fetchDryRun, "dry-run", "n", false, "dry run - show what would be fetched")
	fetchCmd.Flags().IntVar(&fetchDepth, "depth", 0, "limit fetching to specified depth")
}

func runFetch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	startTime := time.Now()

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Parse arguments
	remote, refspec := parseFetchArgs(args)

	// Check if current directory is a git repository
	client := repository.NewClient()
	if !client.IsRepository(ctx, cwd) {
		return fmt.Errorf("not a git repository")
	}

	// Build git fetch command
	executor := gitcmd.NewExecutor()

	// Show what we're about to do
	if !quiet {
		displayFetchIntent(remote, refspec)
	}

	// Determine remotes to fetch from
	remotes := []string{remote}
	if fetchAllRemotes {
		var err error
		remotes, err = getAllRemotes(ctx, executor, cwd)
		if err != nil {
			return fmt.Errorf("failed to get remotes: %w", err)
		}
	}

	// Execute fetch for each remote
	for _, r := range remotes {
		if err := executeSingleFetch(ctx, executor, cwd, r, refspec); err != nil {
			return err
		}
	}

	// Show success
	duration := time.Since(startTime)
	if !quiet {
		displayFetchSuccess(remote, remotes, duration)
	}

	return nil
}

// parseFetchArgs parses command arguments into remote and refspec
func parseFetchArgs(args []string) (remote, refspec string) {
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

// getAllRemotes retrieves all configured remotes
func getAllRemotes(ctx context.Context, executor *gitcmd.Executor, repoPath string) ([]string, error) {
	result, err := executor.Run(ctx, repoPath, "remote")
	if err != nil {
		return nil, err
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to list remotes: %s", result.Stderr)
	}

	var remotes []string
	for _, line := range splitLines(result.Stdout) {
		if line != "" {
			remotes = append(remotes, line)
		}
	}

	if len(remotes) == 0 {
		return nil, fmt.Errorf("no remotes configured")
	}

	return remotes, nil
}

// executeSingleFetch performs fetch from a single remote
func executeSingleFetch(ctx context.Context, executor *gitcmd.Executor, repoPath, remote, refspec string) error {
	args := []string{"fetch"}

	if fetchDryRun {
		args = append(args, "--dry-run")
	}

	if fetchPrune {
		args = append(args, "--prune")
	}

	if fetchTags {
		args = append(args, "--tags")
	}

	if fetchDepth > 0 {
		args = append(args, fmt.Sprintf("--depth=%d", fetchDepth))
	}

	args = append(args, remote)

	if refspec != "" {
		args = append(args, refspec)
	}

	result, err := executor.Run(ctx, repoPath, args...)
	if err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git fetch failed: %s", result.Stderr)
	}

	return nil
}

// displayFetchIntent shows what the fetch command will do
func displayFetchIntent(remote, refspec string) {
	var action string
	if fetchDryRun {
		action = "Would fetch"
	} else {
		action = "Fetching"
	}

	target := remote
	if fetchAllRemotes {
		target = "(all remotes)"
	}
	if refspec != "" {
		target += " " + refspec
	}

	flags := []string{}
	if fetchPrune {
		flags = append(flags, "--prune")
	}
	if fetchTags {
		flags = append(flags, "--tags")
	}

	flagStr := ""
	if len(flags) > 0 {
		flagStr = " " + joinStrings(flags, " ")
	}

	fmt.Printf("%s from %s%s\n", action, target, flagStr)
}

// displayFetchSuccess shows fetch completion message
func displayFetchSuccess(remote string, remotes []string, duration time.Duration) {
	if fetchDryRun {
		fmt.Println("✓ Dry run complete (no changes made)")
		return
	}

	target := remote
	if len(remotes) > 1 {
		target = fmt.Sprintf("%d remotes", len(remotes))
	}

	fmt.Printf("✓ Fetched from %s (%s)\n", target, duration.Round(time.Millisecond))
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
