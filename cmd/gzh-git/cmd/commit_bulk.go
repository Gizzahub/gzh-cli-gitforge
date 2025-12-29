package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

var (
	commitBulkFlags   BulkCommandFlags
	commitBulkMessage string
	commitBulkYes     bool
)

// bulkCmd represents the commit bulk command
var bulkCmd = &cobra.Command{
	Use:   "bulk [directory]",
	Short: "Batch commit changes across multiple repositories",
	Long: `Scan for Git repositories and commit uncommitted changes in parallel.

This command recursively scans the specified directory (or current directory)
for Git repositories with uncommitted changes and commits them in batch.

By default:
  - Scans 1 directory level deep
  - Processes 5 repositories in parallel
  - Shows preview and asks for confirmation
  - Auto-generates commit messages based on changed files

The workflow is:
  1. Scan repositories and identify dirty ones
  2. Show preview table with repositories and suggested messages
  3. Ask for confirmation (Y/n)
  4. Execute commits in parallel

Use --yes to skip confirmation, --dry-run to preview without committing.`,
	Example: `  # Commit all dirty repositories in current directory
  gz-git commit bulk -d 1

  # Commit with custom message for all
  gz-git commit bulk -m "chore: update dependencies" ~/projects

  # Dry run to see what would be committed
  gz-git commit bulk --dry-run ~/projects

  # Skip confirmation
  gz-git commit bulk --yes ~/workspace

  # Filter by pattern
  gz-git commit bulk --include "myproject.*" ~/workspace

  # Exclude pattern
  gz-git commit bulk --exclude "test.*" ~/projects

  # JSON output (for scripting)
  gz-git commit bulk --format json ~/projects

  # Process up to 2 levels deep with 10 parallel workers
  gz-git commit bulk -d 2 --parallel 10 ~/projects`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCommitBulk,
}

func init() {
	commitCmd.AddCommand(bulkCmd)

	// Common bulk operation flags
	addBulkFlags(bulkCmd, &commitBulkFlags)

	// Commit-specific flags
	bulkCmd.Flags().StringVarP(&commitBulkMessage, "message", "m", "", "common commit message for all repositories")
	bulkCmd.Flags().BoolVarP(&commitBulkYes, "yes", "y", false, "auto-approve without confirmation")
}

func runCommitBulk(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-sigChan
		if !quiet {
			fmt.Println("\nInterrupted, cancelling...")
		}
		cancel()
	}()

	// Validate and parse directory
	directory, err := validateBulkDirectory(args)
	if err != nil {
		return err
	}

	// Validate depth
	if err := validateBulkDepth(cmd, commitBulkFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(commitBulkFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkCommitOptions{
		Directory:         directory,
		Parallel:          commitBulkFlags.Parallel,
		MaxDepth:          commitBulkFlags.Depth,
		DryRun:            commitBulkFlags.DryRun,
		Message:           commitBulkMessage,
		Yes:               commitBulkYes,
		Verbose:           verbose,
		IncludeSubmodules: commitBulkFlags.IncludeSubmodules,
		IncludePattern:    commitBulkFlags.Include,
		ExcludePattern:    commitBulkFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Analyzing", commitBulkFlags.Format, quiet),
	}

	// Scanning phase
	if shouldShowProgress(commitBulkFlags.Format, quiet) {
		fmt.Printf("Scanning for repositories in %s (depth: %d)...\n", directory, commitBulkFlags.Depth)
	}

	// Execute bulk commit
	result, err := client.BulkCommit(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk commit failed: %w", err)
	}

	// Display scan completion message
	if shouldShowProgress(commitBulkFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// For non-dry-run, interactive confirmation
	if !opts.DryRun && !opts.Yes && result.TotalDirty > 0 && commitBulkFlags.Format != "json" {
		// Show preview
		displayCommitPreview(result)

		// Ask for confirmation
		confirmed, err := askConfirmation()
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled.")
			return nil
		}

		// Re-run with actual commits
		opts.Yes = true
		opts.DryRun = false
		result, err = client.BulkCommit(ctx, opts)
		if err != nil {
			return fmt.Errorf("bulk commit failed: %w", err)
		}
	}

	// Display results
	if commitBulkFlags.Format == "json" || !quiet {
		displayCommitResults(result)
	}

	return nil
}

func displayCommitPreview(result *repository.BulkCommitResult) {
	fmt.Println()
	fmt.Println("=== Repositories to Commit ===")
	fmt.Printf("Found %d repositories with uncommitted changes\n\n", result.TotalDirty)

	// Table header
	fmt.Printf("%-40s %-15s %-6s %s\n", "Repository", "Branch", "Files", "Suggested Message")
	fmt.Println(strings.Repeat("-", 100))

	for _, repo := range result.Repositories {
		if repo.Status != "dirty" && repo.Status != "would-commit" {
			continue
		}

		// Truncate path if too long
		path := repo.RelativePath
		if len(path) > 38 {
			path = "..." + path[len(path)-35:]
		}

		// Truncate message if too long
		msg := repo.SuggestedMessage
		if len(msg) > 40 {
			msg = msg[:37] + "..."
		}

		fmt.Printf("%-40s %-15s %6d %s\n",
			path,
			repo.Branch,
			repo.FilesChanged,
			msg,
		)
	}
	fmt.Println()
}

func askConfirmation() (bool, error) {
	fmt.Print("Proceed with commit? [Y/n] ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes", nil
}

func displayCommitResults(result *repository.BulkCommitResult) {
	// JSON output mode
	if commitBulkFlags.Format == "json" {
		displayCommitResultsJSON(result)
		return
	}

	fmt.Println()
	fmt.Println("=== Bulk Commit Results ===")
	fmt.Printf("Total scanned:   %d repositories\n", result.TotalScanned)
	fmt.Printf("Total dirty:     %d repositories\n", result.TotalDirty)
	fmt.Printf("Total committed: %d repositories\n", result.TotalCommitted)
	fmt.Printf("Total skipped:   %d repositories\n", result.TotalSkipped)
	fmt.Printf("Total failed:    %d repositories\n", result.TotalFailed)
	fmt.Printf("Duration:        %s\n", result.Duration.Round(100_000_000)) // Round to 0.1s
	fmt.Println()

	// Display summary
	if len(result.Summary) > 0 {
		fmt.Println("Summary by status:")
		for status, count := range result.Summary {
			icon := getCommitStatusIcon(status)
			fmt.Printf("  %s %-15s %d\n", icon, status+":", count)
		}
		fmt.Println()
	}

	// Display individual results if not compact
	if commitBulkFlags.Format != "compact" && len(result.Repositories) > 0 {
		fmt.Println("Repository details:")
		for _, repo := range result.Repositories {
			displayCommitRepositoryResult(repo)
		}
	}

	// Display only errors/committed in compact mode
	if commitBulkFlags.Format == "compact" {
		hasIssues := false
		for _, repo := range result.Repositories {
			if repo.Status == "error" || repo.Status == "success" {
				if !hasIssues && repo.Status == "error" {
					fmt.Println("Issues found:")
					hasIssues = true
				}
				displayCommitRepositoryResult(repo)
			}
		}
		if !hasIssues && result.TotalCommitted > 0 {
			fmt.Printf("✓ Successfully committed %d repositories\n", result.TotalCommitted)
		} else if result.TotalDirty == 0 {
			fmt.Println("✓ All repositories are clean")
		}
	}
}

func displayCommitRepositoryResult(repo repository.RepositoryCommitResult) {
	icon := getCommitStatusIcon(repo.Status)

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
	case "success":
		if repo.CommitHash != "" {
			statusStr = fmt.Sprintf("committed [%s]", repo.CommitHash)
		} else {
			statusStr = "committed"
		}
	case "clean":
		statusStr = "clean"
	case "dirty", "would-commit":
		statusStr = fmt.Sprintf("%d files changed", repo.FilesChanged)
	case "error":
		statusStr = "failed"
	case "skipped":
		statusStr = "skipped"
	default:
		statusStr = repo.Status
	}

	parts = append(parts, fmt.Sprintf("%-25s", statusStr))

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

	// Show error details if present
	if repo.Error != nil && verbose {
		fmt.Printf("    Error: %v\n", repo.Error)
	}
}

func getCommitStatusIcon(status string) string {
	switch status {
	case "success":
		return "✓"
	case "clean":
		return "="
	case "dirty", "would-commit":
		return "⚠"
	case "error":
		return "✗"
	case "skipped":
		return "⊘"
	default:
		return "•"
	}
}

// CommitJSONOutput represents the JSON output structure for commit bulk command
type CommitJSONOutput struct {
	TotalScanned   int                          `json:"total_scanned"`
	TotalDirty     int                          `json:"total_dirty"`
	TotalCommitted int                          `json:"total_committed"`
	TotalSkipped   int                          `json:"total_skipped"`
	TotalFailed    int                          `json:"total_failed"`
	DurationMs     int64                        `json:"duration_ms"`
	Summary        map[string]int               `json:"summary"`
	Repositories   []CommitRepositoryJSONOutput `json:"repositories"`
}

// CommitRepositoryJSONOutput represents a single repository in JSON output
type CommitRepositoryJSONOutput struct {
	Path             string   `json:"path"`
	Branch           string   `json:"branch,omitempty"`
	Status           string   `json:"status"`
	CommitHash       string   `json:"commit_hash,omitempty"`
	Message          string   `json:"message,omitempty"`
	SuggestedMessage string   `json:"suggested_message,omitempty"`
	FilesChanged     int      `json:"files_changed,omitempty"`
	Additions        int      `json:"additions,omitempty"`
	Deletions        int      `json:"deletions,omitempty"`
	ChangedFiles     []string `json:"changed_files,omitempty"`
	DurationMs       int64    `json:"duration_ms,omitempty"`
	Error            string   `json:"error,omitempty"`
}

func displayCommitResultsJSON(result *repository.BulkCommitResult) {
	output := CommitJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalDirty:     result.TotalDirty,
		TotalCommitted: result.TotalCommitted,
		TotalSkipped:   result.TotalSkipped,
		TotalFailed:    result.TotalFailed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]CommitRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := CommitRepositoryJSONOutput{
			Path:             repo.RelativePath,
			Branch:           repo.Branch,
			Status:           repo.Status,
			CommitHash:       repo.CommitHash,
			Message:          repo.Message,
			SuggestedMessage: repo.SuggestedMessage,
			FilesChanged:     repo.FilesChanged,
			Additions:        repo.Additions,
			Deletions:        repo.Deletions,
			ChangedFiles:     repo.ChangedFiles,
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
