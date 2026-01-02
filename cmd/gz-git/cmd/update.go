package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-core/cli"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	updateFlags   BulkCommandFlags
	updateNoFetch bool
)

// updateCmd represents the update command for multi-repository operations
var updateCmd = &cobra.Command{
	Use:   "update [directory]",
	Short: "Update multiple repositories in parallel",
	Long: `Scan for Git repositories and update them from remote in parallel.

This command recursively scans the specified directory (or current directory)
for Git repositories and updates them using git pull --rebase.

For single repository operations, use 'git pull' directly.

By default:
  - Scans 1 directory level deep
  - Processes 5 repositories in parallel
  - Uses rebase strategy (git pull --rebase)
  - Skips repositories with uncommitted changes
  - Skips repositories without upstream branch

The command is safe: it will NOT modify repositories with local changes.`,
	Example: `  # Update all repositories in current directory
  gz-git update

  # Update all repositories up to 2 levels deep
  gz-git update -d 2 .

  # Update with custom parallelism
  gz-git update --parallel 10 ~/projects

  # Dry run to see what would be updated
  gz-git update --dry-run ~/projects

  # Skip fetching (only update already fetched repos)
  gz-git update --no-fetch ~/workspace

  # Filter by pattern
  gz-git update --include "myproject.*" ~/workspace

  # Exclude pattern
  gz-git update --exclude "test.*" ~/projects

  # Compact output format
  gz-git update --format compact ~/projects

  # Continuously update at intervals (watch mode)
  gz-git update --scan-depth 2 --watch --interval 5m ~/projects

  # Watch with shorter interval
  gz-git update --watch --interval 1m ~/work`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)

	// Common bulk operation flags
	addBulkFlags(updateCmd, &updateFlags)

	// Update-specific flags
	updateCmd.Flags().BoolVar(&updateNoFetch, "no-fetch", false, "skip fetching from remote (only update already fetched repos)")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate and parse directory
	directory, err := validateBulkDirectory(args)
	if err != nil {
		return err
	}

	// Validate depth
	if err := validateBulkDepth(cmd, updateFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(updateFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkUpdateOptions{
		Directory:         directory,
		Parallel:          updateFlags.Parallel,
		MaxDepth:          updateFlags.Depth,
		DryRun:            updateFlags.DryRun,
		Verbose:           verbose,
		NoFetch:           updateNoFetch,
		IncludeSubmodules: updateFlags.IncludeSubmodules,
		IncludePattern:    updateFlags.Include,
		ExcludePattern:    updateFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Updating", updateFlags.Format, quiet),
	}

	// Watch mode: continuously update at intervals
	if updateFlags.Watch {
		return runUpdateWatch(ctx, client, opts)
	}

	// One-time update
	if shouldShowProgress(updateFlags.Format, quiet) {
		printScanningMessage(directory, updateFlags.Depth, updateFlags.Parallel, updateFlags.DryRun)
	}

	result, err := client.BulkUpdate(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk update failed: %w", err)
	}

	// Display scan completion message
	if shouldShowProgress(updateFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// Display results (always output for JSON format, otherwise respect quiet flag)
	if updateFlags.Format == "json" || !quiet {
		displayUpdateResults(result)
	}

	return nil
}

func runUpdateWatch(ctx context.Context, client repository.Client, opts repository.BulkUpdateOptions) error {
	cfg := WatchConfig{
		Interval:      updateFlags.Interval,
		Format:        updateFlags.Format,
		Quiet:         quiet,
		OperationName: "update",
		Directory:     opts.Directory,
		MaxDepth:      opts.MaxDepth,
		Parallel:      opts.Parallel,
	}

	return RunBulkWatch(cfg, func() error {
		return executeUpdate(ctx, client, opts)
	})
}

func executeUpdate(ctx context.Context, client repository.Client, opts repository.BulkUpdateOptions) error {
	result, err := client.BulkUpdate(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk update failed: %w", err)
	}

	// Display results
	if !quiet {
		displayUpdateResults(result)
	}

	return nil
}

func displayUpdateResults(result *repository.BulkUpdateResult) {
	// JSON output mode
	if updateFlags.Format == "json" {
		displayUpdateResultsJSON(result)
		return
	}

	// LLM output mode
	if updateFlags.Format == "llm" {
		displayUpdateResultsLLM(result)
		return
	}

	fmt.Println()
	fmt.Println("=== Update Results ===")
	fmt.Printf("Total scanned:   %d repositories\n", result.TotalScanned)
	fmt.Printf("Total processed: %d repositories\n", result.TotalProcessed)
	fmt.Printf("Duration:        %s\n", result.Duration.Round(100_000_000)) // Round to 0.1s
	fmt.Println()

	// Display summary
	if len(result.Summary) > 0 {
		fmt.Println("Summary by status:")
		for status, count := range result.Summary {
			icon := getBulkStatusIconSimple(status)
			fmt.Printf("  %s %-15s %d\n", icon, status+":", count)
		}
		fmt.Println()
	}

	// Display individual results if not compact
	if updateFlags.Format != "compact" && len(result.Repositories) > 0 {
		fmt.Println("Repository details:")
		for _, repo := range result.Repositories {
			displayUpdateRepositoryResult(repo)
		}
	}

	// Display only errors/warnings in compact mode
	if updateFlags.Format == "compact" {
		hasIssues := false
		for _, repo := range result.Repositories {
			if repo.Status == "error" || repo.Status == "dirty" || repo.Status == "conflict" {
				if !hasIssues {
					fmt.Println("Issues found:")
					hasIssues = true
				}
				displayUpdateRepositoryResult(repo)
			}
		}
		if !hasIssues {
			fmt.Println("✓ All repositories updated successfully")
		}
	}
}

func displayUpdateRepositoryResult(repo repository.RepositoryUpdateResult) {
	// Determine icon based on actual result
	icon := getBulkStatusIcon(repo.Status, repo.CommitsBehind)

	// Build compact one-line format: icon path (branch) status duration
	parts := []string{icon}

	// Path with branch
	pathPart := repo.RelativePath
	if repo.Branch != "" {
		pathPart += fmt.Sprintf(" (%s)", repo.Branch)
	}
	parts = append(parts, fmt.Sprintf("%-50s", pathPart))

	// Show status compactly
	statusStr := ""
	switch repo.Status {
	case "success", "pulled", "updated":
		if repo.CommitsBehind > 0 && repo.CommitsAhead > 0 {
			statusStr = fmt.Sprintf("%d↓ %d↑ updated", repo.CommitsBehind, repo.CommitsAhead)
		} else if repo.CommitsBehind > 0 {
			statusStr = fmt.Sprintf("%d↓ updated", repo.CommitsBehind)
		} else if repo.CommitsAhead > 0 {
			statusStr = fmt.Sprintf("up-to-date %d↑", repo.CommitsAhead)
		} else {
			statusStr = "up-to-date"
		}
	case "up-to-date":
		if repo.CommitsAhead > 0 {
			statusStr = fmt.Sprintf("up-to-date %d↑", repo.CommitsAhead)
		} else {
			statusStr = "up-to-date"
		}
	case "error":
		statusStr = "failed"
	case "dirty":
		statusStr = "has changes"
	case "no-remote":
		statusStr = "no remote"
	case "no-upstream":
		statusStr = "no upstream"
	case "would-update":
		if repo.CommitsBehind > 0 {
			statusStr = fmt.Sprintf("would update %d↓", repo.CommitsBehind)
		} else {
			statusStr = "would update"
		}
	case "skipped":
		if repo.HasUncommittedChanges {
			statusStr = "skipped (dirty)"
		} else {
			statusStr = "skipped"
		}
	case "conflict":
		statusStr = "conflict"
	case "rebase-in-progress":
		statusStr = "rebase in progress"
	case "merge-in-progress":
		statusStr = "merge in progress"
	default:
		statusStr = repo.Status
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

// UpdateJSONOutput represents the JSON output structure for update command
type UpdateJSONOutput struct {
	TotalScanned   int                          `json:"total_scanned"`
	TotalProcessed int                          `json:"total_processed"`
	DurationMs     int64                        `json:"duration_ms"`
	Summary        map[string]int               `json:"summary"`
	Repositories   []UpdateRepositoryJSONOutput `json:"repositories"`
}

// UpdateRepositoryJSONOutput represents a single repository in JSON output
type UpdateRepositoryJSONOutput struct {
	Path                  string `json:"path"`
	Branch                string `json:"branch,omitempty"`
	Status                string `json:"status"`
	Message               string `json:"message,omitempty"`
	CommitsAhead          int    `json:"commits_ahead,omitempty"`
	CommitsBehind         int    `json:"commits_behind,omitempty"`
	HasUncommittedChanges bool   `json:"has_uncommitted_changes,omitempty"`
	DurationMs            int64  `json:"duration_ms,omitempty"`
	Error                 string `json:"error,omitempty"`
}

func displayUpdateResultsJSON(result *repository.BulkUpdateResult) {
	output := UpdateJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]UpdateRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := UpdateRepositoryJSONOutput{
			Path:                  repo.RelativePath,
			Branch:                repo.Branch,
			Status:                repo.Status,
			Message:               repo.Message,
			CommitsAhead:          repo.CommitsAhead,
			CommitsBehind:         repo.CommitsBehind,
			HasUncommittedChanges: repo.HasUncommittedChanges,
			DurationMs:            repo.Duration.Milliseconds(),
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

func displayUpdateResultsLLM(result *repository.BulkUpdateResult) {
	output := UpdateJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]UpdateRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := UpdateRepositoryJSONOutput{
			Path:                  repo.RelativePath,
			Branch:                repo.Branch,
			Status:                repo.Status,
			Message:               repo.Message,
			CommitsAhead:          repo.CommitsAhead,
			CommitsBehind:         repo.CommitsBehind,
			HasUncommittedChanges: repo.HasUncommittedChanges,
			DurationMs:            repo.Duration.Milliseconds(),
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
