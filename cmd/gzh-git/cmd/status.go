package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

var statusFlags BulkCommandFlags

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [directory]",
	Short: "Check status of multiple repositories",
	Long: `Scan for Git repositories and check their status in parallel.

This command recursively scans the specified directory (or current directory)
for Git repositories and checks their working tree status in parallel.

By default:
  - Scans 1 directory level deep
  - Processes 5 repositories in parallel
  - Shows repositories with uncommitted changes, ahead/behind status

The command is read-only and will not modify your repositories.`,
	Example: `  # Check status of all repositories in current directory (1-depth scan)
  gz-git status -d 1

  # Check status of all repositories up to 2 levels deep
  gz-git status -d 2 ~/projects

  # Check with custom parallelism
  gz-git status --parallel 10 ~/workspace

  # Show all repositories (including clean ones)
  gz-git status --verbose ~/projects

  # Filter by pattern
  gz-git status --include "myproject.*" ~/workspace

  # Exclude pattern
  gz-git status --exclude "test.*" ~/projects

  # Compact output format
  gz-git status --format compact ~/projects

  # Continuously check at intervals (watch mode)
  gz-git status -d 2 --watch --interval 30s ~/projects`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Common bulk operation flags
	addBulkFlags(statusCmd, &statusFlags)
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate and parse directory
	directory, err := validateBulkDirectory(args)
	if err != nil {
		return err
	}

	// Validate depth
	if err := validateBulkDepth(cmd, statusFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(statusFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkStatusOptions{
		Directory:         directory,
		Parallel:          statusFlags.Parallel,
		MaxDepth:          statusFlags.Depth,
		Verbose:           verbose,
		IncludeSubmodules: statusFlags.IncludeSubmodules,
		IncludePattern:    statusFlags.Include,
		ExcludePattern:    statusFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Checking status", statusFlags.Format, quiet),
	}

	// Watch mode: continuously check at intervals
	if statusFlags.Watch {
		return runStatusWatch(ctx, client, opts)
	}

	// One-time status check
	if shouldShowProgress(statusFlags.Format, quiet) {
		fmt.Printf("Scanning for repositories in %s (depth: %d)...\n", directory, statusFlags.Depth)
	}

	result, err := client.BulkStatus(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk status failed: %w", err)
	}

	// Display scan completion message
	if shouldShowProgress(statusFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// Display results (always output for JSON format, otherwise respect quiet flag)
	if statusFlags.Format == "json" || !quiet {
		displayStatusResults(result)
	}

	return nil
}

func runStatusWatch(ctx context.Context, client repository.Client, opts repository.BulkStatusOptions) error {
	if !quiet {
		fmt.Printf("Starting watch mode: checking every %s\n", statusFlags.Interval)
		fmt.Printf("Scanning for repositories in %s (depth: %d)...\n", opts.Directory, opts.MaxDepth)
		fmt.Println("Press Ctrl+C to stop...")
		fmt.Println()
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create ticker for periodic checking
	ticker := time.NewTicker(statusFlags.Interval)
	defer ticker.Stop()

	// Perform initial check immediately
	if err := executeStatus(ctx, client, opts); err != nil {
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
			if !quiet && statusFlags.Format != "compact" {
				fmt.Printf("\n[%s] Running scheduled status check...\n", time.Now().Format("15:04:05"))
			}
			if err := executeStatus(ctx, client, opts); err != nil {
				if !quiet {
					fmt.Fprintf(os.Stderr, "Status check error: %v\n", err)
				}
				// Continue watching even on error
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func executeStatus(ctx context.Context, client repository.Client, opts repository.BulkStatusOptions) error {
	result, err := client.BulkStatus(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk status failed: %w", err)
	}

	// Display results
	if !quiet {
		displayStatusResults(result)
	}

	return nil
}

func displayStatusResults(result *repository.BulkStatusResult) {
	// JSON output mode
	if statusFlags.Format == "json" {
		displayStatusResultsJSON(result)
		return
	}

	fmt.Println()
	fmt.Println("=== Bulk Status Results ===")
	fmt.Printf("Total scanned:   %d repositories\n", result.TotalScanned)
	fmt.Printf("Total processed: %d repositories\n", result.TotalProcessed)
	fmt.Printf("Duration:        %s\n", result.Duration.Round(100_000_000)) // Round to 0.1s
	fmt.Println()

	// Display summary
	if len(result.Summary) > 0 {
		fmt.Println("Summary by status:")
		for status, count := range result.Summary {
			icon := getStatusIconForStatus(status)
			fmt.Printf("  %s %-15s %d\n", icon, status+":", count)
		}
		fmt.Println()
	}

	// Display individual results if not compact
	if statusFlags.Format != "compact" && len(result.Repositories) > 0 {
		fmt.Println("Repository details:")
		for _, repo := range result.Repositories {
			displayStatusRepositoryResult(repo)
		}
	}

	// Display only dirty/issues in compact mode or when not verbose
	if statusFlags.Format == "compact" || !verbose {
		hasIssues := false
		for _, repo := range result.Repositories {
			if repo.Status != "clean" {
				if !hasIssues {
					fmt.Println("Repositories with changes:")
					hasIssues = true
				}
				displayStatusRepositoryResult(repo)
			}
		}
		if !hasIssues {
			fmt.Println("✓ All repositories are clean")
		}
	}
}

// StatusJSONOutput represents the JSON output structure for status command
type StatusJSONOutput struct {
	TotalScanned   int                          `json:"total_scanned"`
	TotalProcessed int                          `json:"total_processed"`
	DurationMs     int64                        `json:"duration_ms"`
	Summary        map[string]int               `json:"summary"`
	Repositories   []StatusRepositoryJSONOutput `json:"repositories"`
}

// StatusRepositoryJSONOutput represents a single repository in JSON output
type StatusRepositoryJSONOutput struct {
	Path             string   `json:"path"`
	Branch           string   `json:"branch,omitempty"`
	Status           string   `json:"status"`
	UncommittedFiles int      `json:"uncommitted_files,omitempty"`
	UntrackedFiles   int      `json:"untracked_files,omitempty"`
	CommitsAhead     int      `json:"commits_ahead,omitempty"`
	CommitsBehind    int      `json:"commits_behind,omitempty"`
	ConflictFiles    []string `json:"conflict_files,omitempty"`
	DurationMs       int64    `json:"duration_ms,omitempty"`
	Error            string   `json:"error,omitempty"`
}

func displayStatusResultsJSON(result *repository.BulkStatusResult) {
	output := StatusJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]StatusRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := StatusRepositoryJSONOutput{
			Path:             repo.RelativePath,
			Branch:           repo.Branch,
			Status:           repo.Status,
			UncommittedFiles: repo.UncommittedFiles,
			UntrackedFiles:   repo.UntrackedFiles,
			CommitsAhead:     repo.CommitsAhead,
			CommitsBehind:    repo.CommitsBehind,
			ConflictFiles:    repo.ConflictFiles,
			DurationMs:       repo.Duration.Milliseconds(),
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

func displayStatusRepositoryResult(repo repository.RepositoryStatusResult) {
	icon := getStatusIconForStatus(repo.Status)

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
	case "clean":
		if repo.CommitsAhead > 0 && repo.CommitsBehind > 0 {
			statusStr = fmt.Sprintf("clean %d↓ %d↑", repo.CommitsBehind, repo.CommitsAhead)
		} else if repo.CommitsAhead > 0 {
			statusStr = fmt.Sprintf("clean %d↑", repo.CommitsAhead)
		} else if repo.CommitsBehind > 0 {
			statusStr = fmt.Sprintf("clean %d↓", repo.CommitsBehind)
		} else {
			statusStr = "clean"
		}
	case "dirty":
		details := []string{}
		if repo.UncommittedFiles > 0 {
			details = append(details, fmt.Sprintf("%d uncommitted", repo.UncommittedFiles))
		}
		if repo.UntrackedFiles > 0 {
			details = append(details, fmt.Sprintf("%d untracked", repo.UntrackedFiles))
		}
		if len(details) > 0 {
			statusStr = "dirty: " + details[0]
			if len(details) > 1 {
				statusStr += ", " + details[1]
			}
		} else {
			statusStr = "dirty"
		}
	case "conflict":
		statusStr = fmt.Sprintf("CONFLICT: %d files", len(repo.ConflictFiles))
	case "rebase-in-progress":
		statusStr = "REBASE"
	case "merge-in-progress":
		statusStr = "MERGE"
	case "no-remote":
		statusStr = "no remote"
	case "no-upstream":
		statusStr = "no upstream"
	case "error":
		statusStr = "error"
	default:
		statusStr = repo.Status
	}

	parts = append(parts, fmt.Sprintf("%-30s", statusStr))

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
	if repo.Status == "no-upstream" && repo.Branch != "" {
		fmt.Printf("    → Fix: git branch --set-upstream-to=origin/%s %s\n", repo.Branch, repo.Branch)
	}

	// Show error details if present
	if repo.Error != nil && verbose {
		fmt.Printf("    Error: %v\n", repo.Error)
	}
}

func getStatusIconForStatus(status string) string {
	switch status {
	case "clean":
		return "✓"
	case "dirty":
		return "⚠"
	case "conflict":
		return "⚡"
	case "rebase-in-progress":
		return "↻"
	case "merge-in-progress":
		return "⇄"
	case "error":
		return "✗"
	case "no-remote":
		return "⚠"
	case "no-upstream":
		return "⚠"
	default:
		return "•"
	}
}
