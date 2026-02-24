package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-core/cli"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	fetchFlags      BulkCommandFlags
	fetchAllRemotes bool
	fetchPrune      bool
	fetchTags       bool
)

// fetchCmd represents the fetch command for multi-repository operations
var fetchCmd = &cobra.Command{
	Use:   "fetch [directory]",
	Short: "Fetch updates from multiple repositories in parallel",
	Long: cliutil.QuickStartHelp(`  # Fetch all repositories in current directory
  gz-git fetch

  # Fetch all repositories up to 2 levels deep
  gz-git fetch -d 2 .

  # Fetch from origin only (default is all remotes)
  gz-git fetch --all-remotes=false ~/workspace

  # Fetch and prune deleted remote branches
  gz-git fetch --prune ~/projects

  # Fetch all tags
  gz-git fetch --tags ~/repos

  # Filter by pattern
  gz-git fetch --include "myproject.*" ~/workspace`),
	Args: cobra.MaximumNArgs(1),
	RunE: runFetch,
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	// Common bulk operation flags
	addBulkFlags(fetchCmd, &fetchFlags)

	// Fetch-specific flags
	fetchCmd.Flags().BoolVar(&fetchAllRemotes, "all-remotes", true, "fetch from all remotes (default: true, use --no-all-remotes for origin only)")
	fetchCmd.Flags().BoolVarP(&fetchPrune, "prune", "p", false, "prune remote-tracking branches that no longer exist")
	fetchCmd.Flags().BoolVarP(&fetchTags, "tags", "t", false, "fetch all tags from remote")
}

func runFetch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load config with profile support
	effective, _ := LoadEffectiveConfig(cmd, nil)
	if effective != nil {
		// Apply config if flag not explicitly set
		if !cmd.Flags().Changed("parallel") && effective.Parallel > 0 {
			fetchFlags.Parallel = effective.Parallel
		}
		if !cmd.Flags().Changed("all-remotes") {
			fetchAllRemotes = effective.Fetch.AllRemotes
		}
		if !cmd.Flags().Changed("prune") {
			fetchPrune = effective.Fetch.Prune
		}
		if verbose {
			PrintConfigSources(cmd, effective)
		}
	}

	// Validate and parse directory
	directory, err := validateBulkDirectory(args)
	if err != nil {
		return err
	}

	// Validate depth
	if err := validateBulkDepth(cmd, fetchFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(fetchFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkFetchOptions{
		Directory:         directory,
		Parallel:          fetchFlags.Parallel,
		MaxDepth:          fetchFlags.Depth,
		DryRun:            fetchFlags.DryRun,
		Verbose:           verbose,
		AllRemotes:        fetchAllRemotes,
		Prune:             fetchPrune,
		Tags:              fetchTags,
		IncludeSubmodules: fetchFlags.IncludeSubmodules,
		IncludePattern:    fetchFlags.Include,
		ExcludePattern:    fetchFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Fetching", fetchFlags.Format, quiet),
	}

	// Watch mode: continuously fetch at intervals
	if fetchFlags.Watch {
		return runFetchWatch(ctx, client, opts)
	}

	// One-time fetch
	if shouldShowProgress(fetchFlags.Format, quiet) {
		printScanningMessage(directory, fetchFlags.Depth, fetchFlags.Parallel, fetchFlags.DryRun)
	}

	result, err := client.BulkFetch(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk fetch failed: %w", err)
	}

	// Display scan completion message
	if shouldShowProgress(fetchFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// Display results (always output for JSON format, otherwise respect quiet flag)
	if fetchFlags.Format == "json" || !quiet {
		displayFetchResults(result)
	}

	return nil
}

func runFetchWatch(ctx context.Context, client repository.Client, opts repository.BulkFetchOptions) error {
	cfg := WatchConfig{
		Interval:      fetchFlags.Interval,
		Format:        fetchFlags.Format,
		Quiet:         quiet,
		OperationName: "fetch",
		Directory:     opts.Directory,
		MaxDepth:      opts.MaxDepth,
		Parallel:      opts.Parallel,
	}

	return RunBulkWatch(cfg, func() error {
		return executeFetch(ctx, client, opts)
	})
}

func executeFetch(ctx context.Context, client repository.Client, opts repository.BulkFetchOptions) error {
	result, err := client.BulkFetch(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk fetch failed: %w", err)
	}

	// Display results
	if !quiet {
		displayFetchResults(result)
	}

	return nil
}

func displayFetchResults(result *repository.BulkFetchResult) {
	// JSON output mode
	if fetchFlags.Format == "json" {
		displayFetchResultsJSON(result)
		return
	}

	// LLM output mode
	if fetchFlags.Format == "llm" {
		displayFetchResultsLLM(result)
		return
	}

	// Compact mode: unchanged
	if fetchFlags.Format == "compact" {
		fmt.Println()
		fmt.Println("=== Fetch Results ===")
		fmt.Printf("Total scanned:   %d repositories\n", result.TotalScanned)
		fmt.Printf("Total processed: %d repositories\n", result.TotalProcessed)
		fmt.Printf("Duration:        %s\n", result.Duration.Round(100_000_000))
		fmt.Println()
		if len(result.Summary) > 0 {
			fmt.Println("Summary by status:")
			for status, count := range result.Summary {
				icon := getBulkStatusIconSimple(status)
				fmt.Printf("  %s %-15s %d\n", icon, status+":", count)
			}
			fmt.Println()
		}
		hasIssues := false
		for _, repo := range result.Repositories {
			if repo.Status == "error" || repo.Status == "no-remote" {
				if !hasIssues {
					fmt.Println("Issues found:")
					hasIssues = true
				}
				displayFetchRepositoryResult(repo)
			}
		}
		if !hasIssues {
			fmt.Println("✓ All repositories fetched successfully")
		}
	} else if verbose {
		// Verbose: full detailed output (old default behavior)
		fmt.Println()
		fmt.Println("=== Fetch Results ===")
		fmt.Printf("Total scanned:   %d repositories\n", result.TotalScanned)
		fmt.Printf("Total processed: %d repositories\n", result.TotalProcessed)
		fmt.Printf("Duration:        %s\n", result.Duration.Round(100_000_000))
		fmt.Println()
		if len(result.Summary) > 0 {
			fmt.Println("Summary by status:")
			for status, count := range result.Summary {
				icon := getBulkStatusIconSimple(status)
				fmt.Printf("  %s %-15s %d\n", icon, status+":", count)
			}
			fmt.Println()
		}
		if len(result.Repositories) > 0 {
			fmt.Println("Repository details:")
			for _, repo := range result.Repositories {
				displayFetchRepositoryResult(repo)
			}
		}
	} else {
		// Default: summary line + issues only
		WriteSummaryLine(os.Stdout, "Fetched", result.TotalProcessed, result.Summary, result.Duration)
		for _, repo := range result.Repositories {
			if repo.Status == "error" || repo.Status == "no-remote" || repo.Status == "no-upstream" || repo.Status == "auth-required" {
				displayFetchRepositoryResult(repo)
			}
		}
	}

	// Always show dirty warning and auth errors
	dirtyCount := countFetchDirtyRepositories(result.Repositories)
	if dirtyCount > 0 {
		fmt.Println()
		fmt.Printf("⚠ Warning: %d repository(ies) have uncommitted changes\n", dirtyCount)
	}

	authErrors := getFetchAuthRequiredRepositories(result.Repositories)
	if len(authErrors) > 0 {
		fmt.Println()
		fmt.Printf("🔐 Authentication required for %d repository(ies):\n", len(authErrors))
		for _, path := range authErrors {
			fmt.Printf("   • %s\n", path)
		}
		fmt.Println()
		fmt.Println("💡 To fix: Configure credential helper or switch to SSH")
		fmt.Println("   git config --global credential.helper cache")
	}
}

// getFetchAuthRequiredRepositories returns paths of repositories that failed due to authentication.
func getFetchAuthRequiredRepositories(repos []repository.RepositoryFetchResult) []string {
	var paths []string
	for _, repo := range repos {
		if repo.Status == repository.StatusAuthRequired {
			paths = append(paths, repo.RelativePath)
		}
	}
	return paths
}

func displayFetchRepositoryResult(repo repository.RepositoryFetchResult) {
	// Determine icon based on actual result, not just status
	// ✓ = changes fetched, = = no changes (up-to-date)
	// ⚠ = dirty (has uncommitted/untracked files)
	icon := getBulkStatusIcon(repo.Status, repo.CommitsBehind)

	// Override icon if repo is dirty (uncommitted or untracked files)
	isDirty := repo.UncommittedFiles > 0 || repo.UntrackedFiles > 0
	if isDirty && repo.Status != "error" && repo.Status != "conflict" {
		icon = "⚠"
	}

	// Build compact one-line format: icon path (branch) status duration [dirty]
	parts := []string{icon}

	// Path with branch
	pathPart := repo.RelativePath
	if repo.Branch != "" {
		pathPart += fmt.Sprintf(" (%s)", repo.Branch)
	}
	parts = append(parts, fmt.Sprintf("%-50s", pathPart))

	// Show status compactly
	// Status Display Guidelines:
	//   - Changes occurred: "N↓ fetched" with ✓ icon
	//   - No changes: "up-to-date" with = icon
	statusStr := ""
	switch repo.Status {
	case "success", "fetched", "updated":
		if repo.CommitsBehind > 0 && repo.CommitsAhead > 0 {
			statusStr = fmt.Sprintf("%d↓ %d↑ fetched", repo.CommitsBehind, repo.CommitsAhead)
		} else if repo.CommitsBehind > 0 {
			statusStr = fmt.Sprintf("%d↓ fetched", repo.CommitsBehind)
		} else if repo.CommitsAhead > 0 {
			statusStr = fmt.Sprintf("up-to-date %d↑", repo.CommitsAhead)
		} else {
			// No changes fetched - display as up-to-date for consistency
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
	case "no-remote":
		statusStr = "no remote"
	case "no-upstream":
		statusStr = "no upstream"
	case "would-fetch":
		if repo.CommitsBehind > 0 {
			statusStr = fmt.Sprintf("would fetch %d↓", repo.CommitsBehind)
		} else {
			statusStr = "would fetch"
		}
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

	// Add dirty status annotation
	if isDirty {
		dirtyInfo := fmt.Sprintf("[dirty: %d uncommitted, %d untracked]", repo.UncommittedFiles, repo.UntrackedFiles)
		line += " " + dirtyInfo
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

// countFetchDirtyRepositories counts repositories with uncommitted or untracked files.
func countFetchDirtyRepositories(repos []repository.RepositoryFetchResult) int {
	count := 0
	for _, repo := range repos {
		if repo.UncommittedFiles > 0 || repo.UntrackedFiles > 0 {
			count++
		}
	}
	return count
}

// FetchJSONOutput represents the JSON output structure for fetch command
type FetchJSONOutput struct {
	TotalScanned   int                         `json:"total_scanned"`
	TotalProcessed int                         `json:"total_processed"`
	DurationMs     int64                       `json:"duration_ms"`
	Summary        map[string]int              `json:"summary"`
	Repositories   []FetchRepositoryJSONOutput `json:"repositories"`
}

// FetchRepositoryJSONOutput represents a single repository in JSON output
type FetchRepositoryJSONOutput struct {
	Path             string `json:"path"`
	Branch           string `json:"branch,omitempty"`
	Status           string `json:"status"`
	CommitsAhead     int    `json:"commits_ahead,omitempty"`
	CommitsBehind    int    `json:"commits_behind,omitempty"`
	UncommittedFiles int    `json:"uncommitted_files,omitempty"`
	UntrackedFiles   int    `json:"untracked_files,omitempty"`
	DurationMs       int64  `json:"duration_ms,omitempty"`
	Error            string `json:"error,omitempty"`
}

func displayFetchResultsJSON(result *repository.BulkFetchResult) {
	output := FetchJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]FetchRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := FetchRepositoryJSONOutput{
			Path:             repo.RelativePath,
			Branch:           repo.Branch,
			Status:           repo.Status,
			CommitsAhead:     repo.CommitsAhead,
			CommitsBehind:    repo.CommitsBehind,
			UncommittedFiles: repo.UncommittedFiles,
			UntrackedFiles:   repo.UntrackedFiles,
			DurationMs:       repo.Duration.Milliseconds(),
		}
		if repo.Error != nil {
			repoOutput.Error = repo.Error.Error()
		}
		output.Repositories = append(output.Repositories, repoOutput)
	}

	if err := cliutil.WriteJSON(os.Stdout, output, verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

func displayFetchResultsLLM(result *repository.BulkFetchResult) {
	output := FetchJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]FetchRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := FetchRepositoryJSONOutput{
			Path:             repo.RelativePath,
			Branch:           repo.Branch,
			Status:           repo.Status,
			CommitsAhead:     repo.CommitsAhead,
			CommitsBehind:    repo.CommitsBehind,
			UncommittedFiles: repo.UncommittedFiles,
			UntrackedFiles:   repo.UntrackedFiles,
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
