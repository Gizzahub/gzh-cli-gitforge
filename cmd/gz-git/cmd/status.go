package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-core/cli"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

var statusFlags BulkCommandFlags

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [directory]",
	Short: "Check status of multiple repositories",
	Long: `Scan for Git repositories and check their comprehensive health status in parallel.

This command recursively scans the specified directory (or current directory)
for Git repositories and performs comprehensive health checks including:
  - Fetches from all remotes with timeout detection
  - Analyzes divergence (ahead/behind/conflict)
  - Checks work tree status (dirty/clean)
  - Network error classification (timeout/unreachable/auth-failed)
  - Provides smart recommendations for next actions

By default:
  - Scans 1 directory level deep
  - Processes 5 repositories in parallel
  - Fetches from all remotes (30s timeout per fetch)
  - Shows only repositories with issues (use --verbose for all)

The command fetches to update remote tracking but does not modify your working tree.`,
	Example: `  # Check status of all repositories in current directory (1-level scan)
  gz-git status --scan-depth 1

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
  gz-git status --scan-depth 2 --watch --interval 30s ~/projects`,
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

	// Watch mode: continuously check at intervals
	if statusFlags.Watch {
		return runStatusWatch(ctx, client, directory, logger)
	}

	// One-time status check with diagnostic
	if shouldShowProgress(statusFlags.Format, quiet) {
		printScanningMessage(directory, statusFlags.Depth, statusFlags.Parallel, false)
	}

	result, err := runDiagnosticStatus(ctx, client, directory, logger)
	if err != nil {
		return fmt.Errorf("status check failed: %w", err)
	}

	// Display results (always output for JSON format, otherwise respect quiet flag)
	if statusFlags.Format == "json" || !quiet {
		displayDiagnosticResults(result)
	}

	return nil
}

func runDiagnosticStatus(ctx context.Context, client repository.Client, directory string, logger repository.Logger) (*reposync.HealthReport, error) {
	// Use BulkStatus to scan for repositories, then convert to diagnostic format
	bulkOpts := repository.BulkStatusOptions{
		Directory:         directory,
		Parallel:          statusFlags.Parallel,
		MaxDepth:          statusFlags.Depth,
		Verbose:           verbose,
		IncludeSubmodules: statusFlags.IncludeSubmodules,
		IncludePattern:    statusFlags.Include,
		ExcludePattern:    statusFlags.Exclude,
		Logger:            logger,
	}

	// Get basic status first to find all repositories
	bulkResult, err := client.BulkStatus(ctx, bulkOpts)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	// Convert repository results to RepoSpec instances for diagnostic
	var repoSpecs []reposync.RepoSpec
	for _, repoResult := range bulkResult.Repositories {
		// Reconstruct full path (RelativePath is relative to directory)
		fullPath := filepath.Join(directory, repoResult.RelativePath)

		repoSpecs = append(repoSpecs, reposync.RepoSpec{
			CloneURL:   "", // Not needed for local status check
			TargetPath: fullPath,
		})
	}

	// Handle empty result
	if len(repoSpecs) == 0 {
		return &reposync.HealthReport{
			Results:       []reposync.RepoHealth{},
			Summary:       reposync.HealthSummary{},
			TotalDuration: bulkResult.Duration,
			CheckedAt:     time.Now(),
		}, nil
	}

	// Run diagnostic health check
	executor := reposync.DiagnosticExecutor{
		Client: client,
		Logger: logger,
	}

	opts := reposync.DefaultDiagnosticOptions()
	opts.Parallel = statusFlags.Parallel
	opts.SkipFetch = false // Always fetch for comprehensive status

	return executor.CheckHealth(ctx, repoSpecs, opts)
}

func runStatusWatch(ctx context.Context, client repository.Client, directory string, logger repository.Logger) error {
	cfg := WatchConfig{
		Interval:      statusFlags.Interval,
		Format:        statusFlags.Format,
		Quiet:         quiet,
		OperationName: "status check",
		Directory:     directory,
		MaxDepth:      statusFlags.Depth,
		Parallel:      statusFlags.Parallel,
	}

	return RunBulkWatch(cfg, func() error {
		return executeStatusDiagnostic(ctx, client, directory, logger)
	})
}

func executeStatusDiagnostic(ctx context.Context, client repository.Client, directory string, logger repository.Logger) error {
	result, err := runDiagnosticStatus(ctx, client, directory, logger)
	if err != nil {
		return fmt.Errorf("diagnostic status failed: %w", err)
	}

	// Display results
	if !quiet {
		displayDiagnosticResults(result)
	}

	return nil
}

func displayStatusResults(result *repository.BulkStatusResult) {
	// JSON output mode
	if statusFlags.Format == "json" {
		displayStatusResultsJSON(result)
		return
	}

	// LLM output mode
	if statusFlags.Format == "llm" {
		displayStatusResultsLLM(result)
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
			icon := getBulkStatusIconSimple(status)
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

func displayStatusResultsLLM(result *repository.BulkStatusResult) {
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

	var buf bytes.Buffer
	out := cli.NewOutput().SetWriter(&buf).SetFormat("llm")
	if err := out.Print(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding LLM format: %v\n", err)
		return
	}
	fmt.Print(buf.String())
}

func displayStatusRepositoryResult(repo repository.RepositoryStatusResult) {
	icon := getBulkStatusIconSimple(repo.Status)

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
	if repo.Status == "no-upstream" {
		fmt.Print(FormatUpstreamFixHint(repo.Branch, repo.Remote))
	}

	// Show error details if present
	if repo.Error != nil && verbose {
		fmt.Printf("    Error: %v\n", repo.Error)
	}
}

// FormatUpstreamFixHint returns a formatted fix hint for no-upstream status.
// Returns empty string if branch is empty.
// If remote is empty, defaults to "origin".
func FormatUpstreamFixHint(branch, remote string) string {
	if branch == "" {
		return ""
	}
	if remote == "" {
		remote = "origin"
	}
	return fmt.Sprintf("    → Fix: git branch --set-upstream-to=%s/%s %s\n", remote, branch, branch)
}

func displayDiagnosticResults(report *reposync.HealthReport) {
	// JSON output mode
	if statusFlags.Format == "json" {
		displayDiagnosticResultsJSON(report)
		return
	}

	// Compact output mode
	if statusFlags.Format == "compact" {
		displayDiagnosticResultsCompact(report)
		return
	}

	// Default detailed output
	fmt.Println()
	fmt.Println("=== Repository Health Status ===")
	fmt.Printf("Total repositories: %d\n", len(report.Results))
	fmt.Printf("Duration:           %s\n", report.TotalDuration.Round(100_000_000)) // Round to 0.1s
	fmt.Println()

	// Display summary by health status
	if report.Summary.Total > 0 {
		fmt.Println("Summary by health:")
		if report.Summary.Healthy > 0 {
			icon := getHealthIcon(reposync.HealthHealthy)
			fmt.Printf("  %s %-15s %d\n", icon, "healthy:", report.Summary.Healthy)
		}
		if report.Summary.Warning > 0 {
			icon := getHealthIcon(reposync.HealthWarning)
			fmt.Printf("  %s %-15s %d\n", icon, "warning:", report.Summary.Warning)
		}
		if report.Summary.Error > 0 {
			icon := getHealthIcon(reposync.HealthError)
			fmt.Printf("  %s %-15s %d\n", icon, "error:", report.Summary.Error)
		}
		if report.Summary.Unreachable > 0 {
			icon := getHealthIcon(reposync.HealthUnreachable)
			fmt.Printf("  %s %-15s %d\n", icon, "unreachable:", report.Summary.Unreachable)
		}
		fmt.Println()
	}

	// Display individual repository details
	if len(report.Results) > 0 {
		if verbose {
			fmt.Println("Repository details:")
			for _, repo := range report.Results {
				displayHealthRepositoryResult(repo)
			}
		} else {
			// Show only repos with issues
			hasIssues := false
			for _, repo := range report.Results {
				if repo.HealthStatus != reposync.HealthHealthy {
					if !hasIssues {
						fmt.Println("Repositories with issues:")
						hasIssues = true
					}
					displayHealthRepositoryResult(repo)
				}
			}
			if !hasIssues {
				fmt.Println("✓ All repositories are healthy")
			}
		}
	}
}

func displayDiagnosticResultsJSON(report *reposync.HealthReport) {
	type RepoHealthJSON struct {
		Path            string `json:"path"`
		HealthStatus    string `json:"health_status"`
		NetworkStatus   string `json:"network_status,omitempty"`
		DivergenceType  string `json:"divergence_type,omitempty"`
		Branch          string `json:"branch,omitempty"`
		Upstream        string `json:"upstream,omitempty"`
		AheadBy         int    `json:"ahead_by,omitempty"`
		BehindBy        int    `json:"behind_by,omitempty"`
		ModifiedFiles   int    `json:"modified_files,omitempty"`
		UntrackedFiles  int    `json:"untracked_files,omitempty"`
		ConflictFiles   int    `json:"conflict_files,omitempty"`
		Recommendation  string `json:"recommendation,omitempty"`
		Error           string `json:"error,omitempty"`
		DurationMs      int64  `json:"duration_ms"`
		FetchDurationMs int64  `json:"fetch_duration_ms,omitempty"`
	}

	output := struct {
		TotalRepositories int   `json:"total_repositories"`
		DurationMs        int64 `json:"duration_ms"`
		Summary           struct {
			Healthy     int `json:"healthy"`
			Warning     int `json:"warning"`
			Error       int `json:"error"`
			Unreachable int `json:"unreachable"`
			Total       int `json:"total"`
		} `json:"summary"`
		Repositories []RepoHealthJSON `json:"repositories"`
	}{
		TotalRepositories: len(report.Results),
		DurationMs:        report.TotalDuration.Milliseconds(),
		Repositories:      make([]RepoHealthJSON, 0, len(report.Results)),
	}

	// Convert summary
	output.Summary.Healthy = report.Summary.Healthy
	output.Summary.Warning = report.Summary.Warning
	output.Summary.Error = report.Summary.Error
	output.Summary.Unreachable = report.Summary.Unreachable
	output.Summary.Total = report.Summary.Total

	// Convert repositories
	for _, repo := range report.Results {
		repoJSON := RepoHealthJSON{
			Path:            repo.Repo.TargetPath,
			HealthStatus:    string(repo.HealthStatus),
			NetworkStatus:   string(repo.NetworkStatus),
			DivergenceType:  string(repo.DivergenceType),
			Branch:          repo.CurrentBranch,
			Upstream:        repo.UpstreamBranch,
			AheadBy:         repo.AheadBy,
			BehindBy:        repo.BehindBy,
			ModifiedFiles:   repo.ModifiedFiles,
			UntrackedFiles:  repo.UntrackedFiles,
			ConflictFiles:   repo.ConflictFiles,
			Recommendation:  repo.Recommendation,
			DurationMs:      repo.Duration.Milliseconds(),
			FetchDurationMs: repo.FetchDuration.Milliseconds(),
		}
		if repo.Error != nil {
			repoJSON.Error = repo.Error.Error()
		}
		output.Repositories = append(output.Repositories, repoJSON)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

func displayDiagnosticResultsCompact(report *reposync.HealthReport) {
	for _, repo := range report.Results {
		if repo.HealthStatus != reposync.HealthHealthy {
			icon := getHealthIcon(repo.HealthStatus)
			fmt.Printf("%s %s (%s) - %s\n",
				icon,
				filepath.Base(repo.Repo.TargetPath),
				repo.CurrentBranch,
				repo.Recommendation,
			)
		}
	}
}

func displayHealthRepositoryResult(health reposync.RepoHealth) {
	icon := getHealthIcon(health.HealthStatus)
	repoName := filepath.Base(health.Repo.TargetPath)

	// Build status line
	statusParts := []string{icon, repoName}
	if health.CurrentBranch != "" {
		statusParts = append(statusParts, fmt.Sprintf("(%s)", health.CurrentBranch))
	}

	// Add divergence info
	if health.DivergenceType != reposync.DivergenceNone {
		divergenceStr := ""
		switch health.DivergenceType {
		case reposync.DivergenceFastForward:
			divergenceStr = fmt.Sprintf("↓%d", health.BehindBy)
		case reposync.DivergenceAhead:
			divergenceStr = fmt.Sprintf("↑%d", health.AheadBy)
		case reposync.DivergenceDiverged:
			divergenceStr = fmt.Sprintf("↓%d ↑%d", health.BehindBy, health.AheadBy)
		case reposync.DivergenceConflict:
			divergenceStr = fmt.Sprintf("CONFLICT: %d files", health.ConflictFiles)
		case reposync.DivergenceNoUpstream:
			divergenceStr = "no upstream"
		}
		if divergenceStr != "" {
			statusParts = append(statusParts, divergenceStr)
		}
	}

	// Add work tree status
	if health.WorkTreeStatus == reposync.WorkTreeDirty {
		dirtyDetails := []string{}
		if health.ModifiedFiles > 0 {
			dirtyDetails = append(dirtyDetails, fmt.Sprintf("%d modified", health.ModifiedFiles))
		}
		if health.UntrackedFiles > 0 {
			dirtyDetails = append(dirtyDetails, fmt.Sprintf("%d untracked", health.UntrackedFiles))
		}
		if len(dirtyDetails) > 0 {
			statusParts = append(statusParts, fmt.Sprintf("[%s]", dirtyDetails[0]))
		}
	}

	// Print status line
	fmt.Printf("  %s\n", statusParts[0]+" "+statusParts[1])
	for i := 2; i < len(statusParts); i++ {
		fmt.Printf("     %s\n", statusParts[i])
	}

	// Print recommendation
	if health.Recommendation != "" {
		fmt.Printf("     → %s\n", health.Recommendation)
	}

	// Print error if present and verbose
	if health.Error != nil && verbose {
		fmt.Printf("     ⚠  Error: %v\n", health.Error)
	}

	// Print timing if verbose
	if verbose {
		fmt.Printf("     ⏱  Check: %s, Fetch: %s\n",
			health.Duration.Round(10*time.Millisecond),
			health.FetchDuration.Round(10*time.Millisecond),
		)
	}

	fmt.Println()
}

func getHealthIcon(status reposync.HealthStatus) string {
	switch status {
	case reposync.HealthHealthy:
		return "✓"
	case reposync.HealthWarning:
		return "⚠"
	case reposync.HealthError:
		return "✗"
	case reposync.HealthUnreachable:
		return "⊘"
	default:
		return "?"
	}
}
