package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-core/cli"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	pullBulkFlags    BulkCommandFlags
	pullBulkStrategy string
	pullBulkPrune    bool
	pullBulkTags     bool
	pullBulkStash    bool
)

// pullBulkCmd represents the pull-bulk command
var pullBulkCmd = &cobra.Command{
	Use:   "pull-bulk [directory]",
	Short: "Pull updates from remote repositories in bulk",
	Long: `Scan for Git repositories and pull updates from remote in parallel.

This command recursively scans the specified directory (or current directory)
for Git repositories and pulls updates (fetch + merge/rebase) from their remotes
in parallel.

By default:
  - Scans 1 directory level deep
  - Processes 5 repositories in parallel
  - Uses merge strategy (can use rebase or ff-only)
  - Skips repositories without remotes or upstreams

The command updates your working tree with changes from the remote.`,
	Example: `  # Pull all repositories in current directory (1-level scan)
  gz-git pull-bulk --scan-depth 1

  # Pull all repositories up to 2 levels deep
  gz-git pull-bulk -d 2 ~/projects

  # Pull with custom parallelism
  gz-git pull-bulk --parallel 10 ~/workspace

  # Pull with rebase strategy
  gz-git pull-bulk --strategy rebase ~/projects

  # Pull with fast-forward only strategy
  gz-git pull-bulk --strategy ff-only ~/repos

  # Pull and prune deleted remote branches
  gz-git pull-bulk --prune ~/projects

  # Dry run to see what would be pulled
  gz-git pull-bulk --dry-run ~/projects

  # Automatically stash local changes before pull
  gz-git pull-bulk --stash ~/projects

  # Filter by pattern
  gz-git pull-bulk --include "myproject.*" ~/workspace

  # Exclude pattern
  gz-git pull-bulk --exclude "test.*" ~/projects

  # Compact output format
  gz-git pull-bulk --format compact ~/projects

  # Continuously pull at intervals (watch mode)
  gz-git pull-bulk --scan-depth 2 --watch --interval 10m ~/projects

  # Watch with shorter interval
  gz-git pull-bulk --watch --interval 5m ~/work`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPullBulk,
}

func init() {
	rootCmd.AddCommand(pullBulkCmd)

	// Common bulk operation flags
	addBulkFlags(pullBulkCmd, &pullBulkFlags)

	// Pull-specific flags
	pullBulkCmd.Flags().StringVarP(&pullBulkStrategy, "strategy", "s", "merge", "pull strategy: merge, rebase, ff-only")
	pullBulkCmd.Flags().BoolVarP(&pullBulkPrune, "prune", "p", false, "prune remote-tracking branches that no longer exist")
	pullBulkCmd.Flags().BoolVarP(&pullBulkTags, "tags", "t", false, "fetch all tags from remote")
	pullBulkCmd.Flags().BoolVar(&pullBulkStash, "stash", false, "automatically stash local changes before pull")
}

func runPullBulk(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate and parse directory
	directory, err := validateBulkDirectory(args)
	if err != nil {
		return err
	}

	// Validate depth
	if err := validateBulkDepth(cmd, pullBulkFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(pullBulkFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkPullOptions{
		Directory:         directory,
		Parallel:          pullBulkFlags.Parallel,
		MaxDepth:          pullBulkFlags.Depth,
		DryRun:            pullBulkFlags.DryRun,
		Verbose:           verbose,
		Strategy:          pullBulkStrategy,
		Prune:             pullBulkPrune,
		Tags:              pullBulkTags,
		Stash:             pullBulkStash,
		IncludeSubmodules: pullBulkFlags.IncludeSubmodules,
		IncludePattern:    pullBulkFlags.Include,
		ExcludePattern:    pullBulkFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Pulling", pullBulkFlags.Format, quiet),
	}

	// Watch mode: continuously pull at intervals
	if pullBulkFlags.Watch {
		return runPullBulkWatch(ctx, client, opts)
	}

	// One-time pull
	if shouldShowProgress(pullBulkFlags.Format, quiet) {
		fmt.Printf("Scanning for repositories in %s (depth: %d)...\n", directory, pullBulkFlags.Depth)
	}

	result, err := client.BulkPull(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk pull failed: %w", err)
	}

	// Display scan completion message
	if shouldShowProgress(pullBulkFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// Display results (always output for JSON format, otherwise respect quiet flag)
	if pullBulkFlags.Format == "json" || !quiet {
		displayPullBulkResults(result)
	}

	return nil
}

func runPullBulkWatch(ctx context.Context, client repository.Client, opts repository.BulkPullOptions) error {
	if !quiet {
		fmt.Printf("Starting watch mode: pulling every %s\n", pullBulkFlags.Interval)
		fmt.Printf("Scanning for repositories in %s (depth: %d)...\n", opts.Directory, opts.MaxDepth)
		fmt.Println("Press Ctrl+C to stop...")
		fmt.Println()
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create ticker for periodic pulling
	ticker := time.NewTicker(pullBulkFlags.Interval)
	defer ticker.Stop()

	// Perform initial pull immediately
	if err := executePullBulk(ctx, client, opts); err != nil {
		return err
	}

	// Watch loop
	for {
		select {
		case <-sigChan:
			if !quiet {
				fmt.Println("\nStopping watch...")
			}
			return nil

		case <-ticker.C:
			if shouldShowProgress(pullBulkFlags.Format, quiet) {
				fmt.Printf("\n[%s] Running scheduled pull...\n", time.Now().Format("15:04:05"))
			}
			if err := executePullBulk(ctx, client, opts); err != nil {
				if !quiet {
					fmt.Fprintf(os.Stderr, "Pull error: %v\n", err)
				}
				// Continue watching even on error
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func executePullBulk(ctx context.Context, client repository.Client, opts repository.BulkPullOptions) error {
	result, err := client.BulkPull(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk pull failed: %w", err)
	}

	// Display results
	if !quiet {
		displayPullBulkResults(result)
	}

	return nil
}

func displayPullBulkResults(result *repository.BulkPullResult) {
	// JSON output mode
	if pullBulkFlags.Format == "json" {
		displayPullBulkResultsJSON(result)
		return
	}

	// LLM output mode
	if pullBulkFlags.Format == "llm" {
		displayPullBulkResultsLLM(result)
		return
	}

	fmt.Println()
	fmt.Println("=== Bulk Pull Results ===")
	fmt.Printf("Total scanned:   %d repositories\n", result.TotalScanned)
	fmt.Printf("Total processed: %d repositories\n", result.TotalProcessed)
	fmt.Printf("Duration:        %s\n", result.Duration.Round(100_000_000)) // Round to 0.1s
	fmt.Println()

	// Display summary
	if len(result.Summary) > 0 {
		fmt.Println("Summary by status:")
		for status, count := range result.Summary {
			icon := getPullStatusIcon(status)
			fmt.Printf("  %s %-15s %d\n", icon, status+":", count)
		}
		fmt.Println()
	}

	// Display individual results if not compact
	if pullBulkFlags.Format != "compact" && len(result.Repositories) > 0 {
		fmt.Println("Repository details:")
		for _, repo := range result.Repositories {
			displayPullBulkRepositoryResult(repo)
		}
	}

	// Display only errors/warnings in compact mode
	if pullBulkFlags.Format == "compact" {
		hasIssues := false
		for _, repo := range result.Repositories {
			if repo.Status == "error" || repo.Status == "no-remote" || repo.Status == "no-upstream" ||
				repo.Status == "conflict" || repo.Status == "rebase-in-progress" || repo.Status == "merge-in-progress" ||
				repo.Status == "dirty" {
				if !hasIssues {
					fmt.Println("Issues found:")
					hasIssues = true
				}
				displayPullBulkRepositoryResult(repo)
			}
		}
		if !hasIssues {
			fmt.Println("✓ All repositories pulled successfully")
		}
	}
}

func displayPullBulkRepositoryResult(repo repository.RepositoryPullResult) {
	// Determine icon based on actual result, not just status
	// ✓ = changes pulled, = = no changes (up-to-date)
	icon := getPullStatusIconWithContext(repo.Status, repo.CommitsBehind)

	// Build compact one-line format: icon path (branch) status duration
	parts := []string{icon}

	// Path with branch
	pathPart := repo.RelativePath
	if repo.Branch != "" {
		pathPart += fmt.Sprintf(" (%s)", repo.Branch)
	}
	parts = append(parts, fmt.Sprintf("%-50s", pathPart))

	// Show status compactly
	// Status Display Guidelines:
	//   - Changes occurred: "N↓ pulled" with ✓ icon
	//   - No changes: "up-to-date" with = icon
	statusStr := ""
	switch repo.Status {
	case "success", "pulled":
		if repo.CommitsBehind > 0 {
			statusStr = fmt.Sprintf("%d↓ pulled", repo.CommitsBehind)
		} else {
			// No changes pulled - display as up-to-date for consistency
			statusStr = "up-to-date"
		}
	case "up-to-date":
		if repo.CommitsAhead > 0 {
			statusStr = fmt.Sprintf("up-to-date %d↑", repo.CommitsAhead)
		} else {
			statusStr = "up-to-date"
		}
	case "would-pull":
		if repo.CommitsBehind > 0 {
			statusStr = fmt.Sprintf("would pull %d↓", repo.CommitsBehind)
		} else {
			statusStr = "would pull"
		}
	case "error":
		statusStr = "failed"
	case "no-remote":
		statusStr = "no remote"
	case "no-upstream":
		statusStr = "no upstream"
	case "conflict":
		statusStr = "CONFLICT"
	case "rebase-in-progress":
		statusStr = "REBASE"
	case "merge-in-progress":
		statusStr = "MERGE"
	case "dirty":
		statusStr = "dirty"
	case "skipped":
		statusStr = "skipped"
	default:
		statusStr = repo.Status
	}

	// Add stash indicator
	if repo.Stashed {
		statusStr += " [stash]"
	}

	parts = append(parts, fmt.Sprintf("%-18s", statusStr))

	// Duration
	if repo.Duration > 0 {
		parts = append(parts, fmt.Sprintf("%6s", repo.Duration.Round(10_000_000)))
	}

	// Build output line safely
	line := "  " + parts[0] + " " + parts[1] + " " + parts[2]
	if len(parts) > 3 {
		line += " " + parts[3]
	}
	fmt.Println(line)

	// Show fix hint for no-upstream status
	if repo.Status == "no-upstream" {
		fmt.Print(FormatUpstreamFixHint(repo.Branch, repo.Remote))
	}

	// Show error details if present
	if repo.Error != nil && verbose {
		fmt.Printf("    Error: %v\n", repo.Error)
	}
}

// PullBulkJSONOutput represents the JSON output structure for pull-bulk command
type PullBulkJSONOutput struct {
	TotalScanned   int                            `json:"total_scanned"`
	TotalProcessed int                            `json:"total_processed"`
	DurationMs     int64                          `json:"duration_ms"`
	Summary        map[string]int                 `json:"summary"`
	Repositories   []PullBulkRepositoryJSONOutput `json:"repositories"`
}

// PullBulkRepositoryJSONOutput represents a single repository in JSON output
type PullBulkRepositoryJSONOutput struct {
	Path          string `json:"path"`
	Branch        string `json:"branch,omitempty"`
	Status        string `json:"status"`
	CommitsAhead  int    `json:"commits_ahead,omitempty"`
	CommitsBehind int    `json:"commits_behind,omitempty"`
	Stashed       bool   `json:"stashed,omitempty"`
	DurationMs    int64  `json:"duration_ms,omitempty"`
	Error         string `json:"error,omitempty"`
}

func displayPullBulkResultsJSON(result *repository.BulkPullResult) {
	output := PullBulkJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]PullBulkRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := PullBulkRepositoryJSONOutput{
			Path:          repo.RelativePath,
			Branch:        repo.Branch,
			Status:        repo.Status,
			CommitsAhead:  repo.CommitsAhead,
			CommitsBehind: repo.CommitsBehind,
			Stashed:       repo.Stashed,
			DurationMs:    repo.Duration.Milliseconds(),
		}
		if repo.Error != nil {
			repoOutput.Error = repo.Error.Error()
		}
		output.Repositories = append(output.Repositories, repoOutput)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

func displayPullBulkResultsLLM(result *repository.BulkPullResult) {
	output := PullBulkJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]PullBulkRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := PullBulkRepositoryJSONOutput{
			Path:          repo.RelativePath,
			Branch:        repo.Branch,
			Status:        repo.Status,
			CommitsAhead:  repo.CommitsAhead,
			CommitsBehind: repo.CommitsBehind,
			Stashed:       repo.Stashed,
			DurationMs:    repo.Duration.Milliseconds(),
		}
		if repo.Error != nil {
			repoOutput.Error = repo.Error.Error()
		}
		output.Repositories = append(output.Repositories, repoOutput)
	}

	var buf bytes.Buffer
	out := cli.NewOutput().SetWriter(&buf).SetFormat("llm")
	if err := out.Print(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding LLM format: %v\n", err)
		return
	}
	fmt.Print(buf.String())
}

// getPullStatusIconWithContext returns the appropriate icon based on status and actual changes.
// Icons: ✓ (changes pulled), = (no changes), ✗ (error), ⚠ (warning), ⊘ (skipped)
func getPullStatusIconWithContext(status string, commitsBehind int) string {
	switch status {
	case "success", "pulled":
		// Only show ✓ if actual changes were pulled
		if commitsBehind > 0 {
			return "✓"
		}
		return "=" // No changes = up-to-date
	case "up-to-date":
		return "="
	case "error":
		return "✗"
	case "conflict":
		return "⚡"
	case "rebase-in-progress":
		return "↻"
	case "merge-in-progress":
		return "⇄"
	case "dirty":
		return "⚠"
	case "skipped":
		return "⊘"
	case "would-pull":
		return "→"
	case "no-remote":
		return "⚠"
	case "no-upstream":
		return "⚠"
	default:
		return "•"
	}
}

// getPullStatusIcon returns the icon for a status.
func getPullStatusIcon(status string) string {
	return getPullStatusIconWithContext(status, 0)
}
