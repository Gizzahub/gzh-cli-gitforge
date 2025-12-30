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
	pushBulkFlags       BulkCommandFlags
	pushBulkForce       bool
	pushBulkSetUpstream bool
	pushBulkTags        bool
	pushBulkRefspec     string
	pushBulkRemotes     []string
	pushBulkAllRemotes  bool
)

// pushBulkCmd represents the push-bulk command
var pushBulkCmd = &cobra.Command{
	Use:   "push-bulk [directory]",
	Short: "Push commits to remote repositories in bulk",
	Long: `Scan for Git repositories and push local commits to remote in parallel.

This command recursively scans the specified directory (or current directory)
for Git repositories and pushes local commits to their remotes in parallel.

By default:
  - Scans 1 directory level deep
  - Processes 5 repositories in parallel
  - Pushes to origin remote only
  - Skips repositories without remotes or upstreams

The command pushes your local commits to remote repositories.`,
	Example: `  # Push all repositories in current directory (1-level scan)
  gz-git push-bulk --scan-depth 1

  # Push all repositories up to 2 levels deep
  gz-git push-bulk -d 2 ~/projects

  # Push with custom parallelism
  gz-git push-bulk --parallel 10 ~/workspace

  # Force push (use with caution!)
  gz-git push-bulk --force ~/projects

  # Push and set upstream for new branches
  gz-git push-bulk --set-upstream ~/projects

  # Push all tags
  gz-git push-bulk --tags ~/repos

  # Push with custom refspec (local:remote branch mapping)
  gz-git push-bulk --refspec develop:master ~/projects

  # Push to multiple remotes
  gz-git push-bulk --remote origin --remote backup ~/projects

  # Push to all configured remotes
  gz-git push-bulk --all-remotes ~/projects

  # Combine refspec with multiple remotes
  gz-git push-bulk --refspec develop:master --remote origin --remote backup ~/work

  # Dry run to see what would be pushed
  gz-git push-bulk --dry-run ~/projects

  # Filter by pattern
  gz-git push-bulk --include "myproject.*" ~/workspace

  # Exclude pattern
  gz-git push-bulk --exclude "test.*" ~/projects

  # Compact output format
  gz-git push-bulk --format compact ~/projects

  # Continuously push at intervals (watch mode)
  gz-git push-bulk --scan-depth 2 --watch --interval 10m ~/projects

  # Watch with shorter interval
  gz-git push-bulk --watch --interval 5m ~/work`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPushBulk,
}

func init() {
	rootCmd.AddCommand(pushBulkCmd)

	// Common bulk operation flags
	addBulkFlags(pushBulkCmd, &pushBulkFlags)

	// Push-specific flags (no -f shorthand for force, conflicts with --format)
	pushBulkCmd.Flags().BoolVar(&pushBulkForce, "force", false, "force push (use with caution!)")
	pushBulkCmd.Flags().BoolVarP(&pushBulkSetUpstream, "set-upstream", "u", false, "set upstream for new branches")
	pushBulkCmd.Flags().BoolVarP(&pushBulkTags, "tags", "t", false, "push all tags to remote")
	pushBulkCmd.Flags().StringVar(&pushBulkRefspec, "refspec", "", "custom refspec (e.g., 'develop:master' to push local develop to remote master)")
	pushBulkCmd.Flags().StringSliceVar(&pushBulkRemotes, "remote", []string{}, "remote(s) to push to (can be specified multiple times)")
	pushBulkCmd.Flags().BoolVar(&pushBulkAllRemotes, "all-remotes", false, "push to all configured remotes")
}

func runPushBulk(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate and parse directory
	directory, err := validateBulkDirectory(args)
	if err != nil {
		return err
	}

	// Validate depth
	if err := validateBulkDepth(cmd, pushBulkFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(pushBulkFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkPushOptions{
		Directory:         directory,
		Parallel:          pushBulkFlags.Parallel,
		MaxDepth:          pushBulkFlags.Depth,
		DryRun:            pushBulkFlags.DryRun,
		Verbose:           verbose,
		Force:             pushBulkForce,
		SetUpstream:       pushBulkSetUpstream,
		Tags:              pushBulkTags,
		Refspec:           pushBulkRefspec,
		Remotes:           pushBulkRemotes,
		AllRemotes:        pushBulkAllRemotes,
		IncludeSubmodules: pushBulkFlags.IncludeSubmodules,
		IncludePattern:    pushBulkFlags.Include,
		ExcludePattern:    pushBulkFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Pushing", pushBulkFlags.Format, quiet),
	}

	// Watch mode: continuously push at intervals
	if pushBulkFlags.Watch {
		return runPushBulkWatch(ctx, client, opts)
	}

	// One-time push
	if shouldShowProgress(pushBulkFlags.Format, quiet) {
		fmt.Printf("Scanning for repositories in %s (depth: %d)...\n", directory, pushBulkFlags.Depth)
	}

	result, err := client.BulkPush(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk push failed: %w", err)
	}

	// Display scan completion message
	if shouldShowProgress(pushBulkFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// Display results (always output for JSON format, otherwise respect quiet flag)
	if pushBulkFlags.Format == "json" || !quiet {
		displayPushBulkResults(result)
	}

	return nil
}

func runPushBulkWatch(ctx context.Context, client repository.Client, opts repository.BulkPushOptions) error {
	if !quiet {
		fmt.Printf("Starting watch mode: pushing every %s\n", pushBulkFlags.Interval)
		fmt.Printf("Scanning for repositories in %s (depth: %d)...\n", opts.Directory, opts.MaxDepth)
		fmt.Println("Press Ctrl+C to stop...")
		fmt.Println()
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create ticker for periodic pushing
	ticker := time.NewTicker(pushBulkFlags.Interval)
	defer ticker.Stop()

	// Perform initial push immediately
	if err := executePushBulk(ctx, client, opts); err != nil {
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
			if shouldShowProgress(pushBulkFlags.Format, quiet) {
				fmt.Printf("\n[%s] Running scheduled push...\n", time.Now().Format("15:04:05"))
			}
			if err := executePushBulk(ctx, client, opts); err != nil {
				if !quiet {
					fmt.Fprintf(os.Stderr, "Push error: %v\n", err)
				}
				// Continue watching even on error
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func executePushBulk(ctx context.Context, client repository.Client, opts repository.BulkPushOptions) error {
	result, err := client.BulkPush(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk push failed: %w", err)
	}

	// Display results
	if !quiet {
		displayPushBulkResults(result)
	}

	return nil
}

func displayPushBulkResults(result *repository.BulkPushResult) {
	// JSON output mode
	if pushBulkFlags.Format == "json" {
		displayPushBulkResultsJSON(result)
		return
	}

	// LLM output mode
	if pushBulkFlags.Format == "llm" {
		displayPushBulkResultsLLM(result)
		return
	}

	fmt.Println()
	fmt.Println("=== Bulk Push Results ===")
	fmt.Printf("Total scanned:   %d repositories\n", result.TotalScanned)
	fmt.Printf("Total processed: %d repositories\n", result.TotalProcessed)
	fmt.Printf("Duration:        %s\n", result.Duration.Round(100_000_000)) // Round to 0.1s
	fmt.Println()

	// Display summary
	if len(result.Summary) > 0 {
		fmt.Println("Summary by status:")
		for status, count := range result.Summary {
			icon := getPushStatusIcon(status)
			fmt.Printf("  %s %-15s %d\n", icon, status+":", count)
		}
		fmt.Println()
	}

	// Display individual results if not compact
	if pushBulkFlags.Format != "compact" && len(result.Repositories) > 0 {
		fmt.Println("Repository details:")
		for _, repo := range result.Repositories {
			displayPushBulkRepositoryResult(repo)
		}
	}

	// Display only errors/warnings in compact mode
	if pushBulkFlags.Format == "compact" {
		hasIssues := false
		for _, repo := range result.Repositories {
			if repo.Status == "error" || repo.Status == "no-remote" || repo.Status == "no-upstream" ||
				repo.Status == "conflict" || repo.Status == "rebase-in-progress" || repo.Status == "merge-in-progress" {
				if !hasIssues {
					fmt.Println("Issues found:")
					hasIssues = true
				}
				displayPushBulkRepositoryResult(repo)
			}
		}
		if !hasIssues {
			fmt.Println("✓ All repositories pushed successfully")
		}
	}
}

func displayPushBulkRepositoryResult(repo repository.RepositoryPushResult) {
	// Determine icon based on actual result, not just status
	// ✓ = changes pushed, = = no changes (up-to-date)
	icon := getPushStatusIconWithContext(repo.Status, repo.PushedCommits)

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
	//   - Changes occurred: "N↑ pushed" with ✓ icon
	//   - No changes: "up-to-date" with = icon
	statusStr := ""
	switch repo.Status {
	case "success", "pushed":
		if repo.PushedCommits > 0 {
			statusStr = fmt.Sprintf("%d↑ pushed", repo.PushedCommits)
		} else {
			// No changes pushed - display as up-to-date for consistency
			statusStr = "up-to-date"
		}
	case "nothing-to-push", "up-to-date":
		statusStr = "up-to-date"
	case "would-push":
		if repo.CommitsAhead > 0 {
			statusStr = fmt.Sprintf("would push %d↑", repo.CommitsAhead)
		} else {
			statusStr = "would push"
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
	case "skipped":
		statusStr = "skipped"
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

// PushBulkJSONOutput represents the JSON output structure for push-bulk command
type PushBulkJSONOutput struct {
	TotalScanned   int                            `json:"total_scanned"`
	TotalProcessed int                            `json:"total_processed"`
	DurationMs     int64                          `json:"duration_ms"`
	Summary        map[string]int                 `json:"summary"`
	Repositories   []PushBulkRepositoryJSONOutput `json:"repositories"`
}

// PushBulkRepositoryJSONOutput represents a single repository in JSON output
type PushBulkRepositoryJSONOutput struct {
	Path          string `json:"path"`
	Branch        string `json:"branch,omitempty"`
	Status        string `json:"status"`
	CommitsAhead  int    `json:"commits_ahead,omitempty"`
	PushedCommits int    `json:"pushed_commits,omitempty"`
	DurationMs    int64  `json:"duration_ms,omitempty"`
	Error         string `json:"error,omitempty"`
}

func displayPushBulkResultsJSON(result *repository.BulkPushResult) {
	output := PushBulkJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]PushBulkRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := PushBulkRepositoryJSONOutput{
			Path:          repo.RelativePath,
			Branch:        repo.Branch,
			Status:        repo.Status,
			CommitsAhead:  repo.CommitsAhead,
			PushedCommits: repo.PushedCommits,
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

func displayPushBulkResultsLLM(result *repository.BulkPushResult) {
	output := PushBulkJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]PushBulkRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := PushBulkRepositoryJSONOutput{
			Path:          repo.RelativePath,
			Branch:        repo.Branch,
			Status:        repo.Status,
			CommitsAhead:  repo.CommitsAhead,
			PushedCommits: repo.PushedCommits,
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
