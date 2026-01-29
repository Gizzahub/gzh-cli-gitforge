package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-core/cli"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	pullFlags    BulkCommandFlags
	pullStrategy string
	pullPrune    bool
	pullTags     bool
	pullStash    bool
)

// pullCmd represents the pull command for multi-repository operations
var pullCmd = &cobra.Command{
	Use:   "pull [directory]",
	Short: "Pull updates from multiple repositories in parallel",
	Long: cliutil.QuickStartHelp(`  # Pull all repositories in current directory
  gz-git pull

  # Pull all repositories up to 2 levels deep
  gz-git pull -d 2 ~/projects

  # Pull with custom parallelism
  gz-git pull --parallel 10 ~/workspace

  # Pull with rebase strategy
  gz-git pull --merge-strategy rebase ~/projects

  # Pull with fast-forward only strategy
  gz-git pull --merge-strategy ff-only ~/repos

  # Pull and prune deleted remote branches
  gz-git pull --prune ~/projects

  # Automatically stash local changes before pull
  gz-git pull --stash ~/projects

  # Filter by pattern
  gz-git pull --include "myproject.*" ~/workspace`),
	Args: cobra.MaximumNArgs(1),
	RunE: runPull,
}

func init() {
	rootCmd.AddCommand(pullCmd)

	// Common bulk operation flags
	addBulkFlags(pullCmd, &pullFlags)

	// Pull-specific flags
	pullCmd.Flags().StringVarP(&pullStrategy, "merge-strategy", "s", "merge", "merge strategy: merge, rebase, ff-only")
	pullCmd.Flags().BoolVarP(&pullPrune, "prune", "p", false, "prune remote-tracking branches that no longer exist")
	pullCmd.Flags().BoolVarP(&pullTags, "tags", "t", false, "fetch all tags from remote")
	pullCmd.Flags().BoolVar(&pullStash, "stash", false, "automatically stash local changes before pull")
}

func runPull(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load config with profile support
	effective, _ := LoadEffectiveConfig(cmd, nil)
	if effective != nil {
		// Apply config if flag not explicitly set
		if !cmd.Flags().Changed("parallel") && effective.Parallel > 0 {
			pullFlags.Parallel = effective.Parallel
		}
		// Apply pull strategy from config (rebase or ff-only)
		if !cmd.Flags().Changed("merge-strategy") && !cmd.Flags().Changed("strategy") {
			if effective.Pull.FFOnly {
				pullStrategy = "ff-only"
			} else if effective.Pull.Rebase {
				pullStrategy = "rebase"
			}
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
	if err := validateBulkDepth(cmd, pullFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(pullFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkPullOptions{
		Directory:         directory,
		Parallel:          pullFlags.Parallel,
		MaxDepth:          pullFlags.Depth,
		DryRun:            pullFlags.DryRun,
		Verbose:           verbose,
		Strategy:          pullStrategy,
		Prune:             pullPrune,
		Tags:              pullTags,
		Stash:             pullStash,
		IncludeSubmodules: pullFlags.IncludeSubmodules,
		IncludePattern:    pullFlags.Include,
		ExcludePattern:    pullFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Pulling", pullFlags.Format, quiet),
	}

	// Watch mode: continuously pull at intervals
	if pullFlags.Watch {
		return runPullWatch(ctx, client, opts)
	}

	// One-time pull
	if shouldShowProgress(pullFlags.Format, quiet) {
		printScanningMessage(directory, pullFlags.Depth, pullFlags.Parallel, pullFlags.DryRun)
	}

	result, err := client.BulkPull(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk pull failed: %w", err)
	}

	// Display scan completion message
	if shouldShowProgress(pullFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// Display results (always output for JSON format, otherwise respect quiet flag)
	if pullFlags.Format == "json" || !quiet {
		displayPullResults(result)
	}

	return nil
}

func runPullWatch(ctx context.Context, client repository.Client, opts repository.BulkPullOptions) error {
	cfg := WatchConfig{
		Interval:      pullFlags.Interval,
		Format:        pullFlags.Format,
		Quiet:         quiet,
		OperationName: "pull",
		Directory:     opts.Directory,
		MaxDepth:      opts.MaxDepth,
		Parallel:      opts.Parallel,
	}

	return RunBulkWatch(cfg, func() error {
		return executePull(ctx, client, opts)
	})
}

func executePull(ctx context.Context, client repository.Client, opts repository.BulkPullOptions) error {
	result, err := client.BulkPull(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk pull failed: %w", err)
	}

	// Display results
	if !quiet {
		displayPullResults(result)
	}

	return nil
}

func displayPullResults(result *repository.BulkPullResult) {
	// JSON output mode
	if pullFlags.Format == "json" {
		displayPullResultsJSON(result)
		return
	}

	// LLM output mode
	if pullFlags.Format == "llm" {
		displayPullResultsLLM(result)
		return
	}

	fmt.Println()
	fmt.Println("=== Pull Results ===")
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
	if pullFlags.Format != "compact" && len(result.Repositories) > 0 {
		fmt.Println("Repository details:")
		for _, repo := range result.Repositories {
			displayPullRepositoryResult(repo)
		}
	}

	// Display only errors/warnings in compact mode
	if pullFlags.Format == "compact" {
		hasIssues := false
		for _, repo := range result.Repositories {
			if repo.Status == "error" || repo.Status == "no-remote" || repo.Status == "no-upstream" ||
				repo.Status == "conflict" || repo.Status == "rebase-in-progress" || repo.Status == "merge-in-progress" ||
				repo.Status == "dirty" {
				if !hasIssues {
					fmt.Println("Issues found:")
					hasIssues = true
				}
				displayPullRepositoryResult(repo)
			}
		}
		if !hasIssues {
			fmt.Println("✓ All repositories pulled successfully")
		}
	}

	// Display dirty repositories warning
	dirtyCount := countPullDirtyRepositories(result.Repositories)
	if dirtyCount > 0 {
		fmt.Println()
		fmt.Printf("⚠ Warning: %d repository(ies) have uncommitted changes\n", dirtyCount)
	}

	// Display authentication errors summary
	authErrors := getPullAuthRequiredRepositories(result.Repositories)
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

// getPullAuthRequiredRepositories returns paths of repositories that failed due to authentication.
func getPullAuthRequiredRepositories(repos []repository.RepositoryPullResult) []string {
	var paths []string
	for _, repo := range repos {
		if repo.Status == repository.StatusAuthRequired {
			paths = append(paths, repo.RelativePath)
		}
	}
	return paths
}

func displayPullRepositoryResult(repo repository.RepositoryPullResult) {
	// Determine icon based on actual result, not just status
	// ✓ = changes pulled, = = no changes (up-to-date)
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

// countPullDirtyRepositories counts repositories with uncommitted or untracked files.
func countPullDirtyRepositories(repos []repository.RepositoryPullResult) int {
	count := 0
	for _, repo := range repos {
		if repo.UncommittedFiles > 0 || repo.UntrackedFiles > 0 {
			count++
		}
	}
	return count
}

// PullJSONOutput represents the JSON output structure for pull command
type PullJSONOutput struct {
	TotalScanned   int                        `json:"total_scanned"`
	TotalProcessed int                        `json:"total_processed"`
	DurationMs     int64                      `json:"duration_ms"`
	Summary        map[string]int             `json:"summary"`
	Repositories   []PullRepositoryJSONOutput `json:"repositories"`
}

// PullRepositoryJSONOutput represents a single repository in JSON output
type PullRepositoryJSONOutput struct {
	Path             string `json:"path"`
	Branch           string `json:"branch,omitempty"`
	Status           string `json:"status"`
	CommitsAhead     int    `json:"commits_ahead,omitempty"`
	CommitsBehind    int    `json:"commits_behind,omitempty"`
	UncommittedFiles int    `json:"uncommitted_files,omitempty"`
	UntrackedFiles   int    `json:"untracked_files,omitempty"`
	Stashed          bool   `json:"stashed,omitempty"`
	DurationMs       int64  `json:"duration_ms,omitempty"`
	Error            string `json:"error,omitempty"`
}

func displayPullResultsJSON(result *repository.BulkPullResult) {
	output := PullJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]PullRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := PullRepositoryJSONOutput{
			Path:             repo.RelativePath,
			Branch:           repo.Branch,
			Status:           repo.Status,
			CommitsAhead:     repo.CommitsAhead,
			CommitsBehind:    repo.CommitsBehind,
			UncommittedFiles: repo.UncommittedFiles,
			UntrackedFiles:   repo.UntrackedFiles,
			Stashed:          repo.Stashed,
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

func displayPullResultsLLM(result *repository.BulkPullResult) {
	output := PullJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]PullRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := PullRepositoryJSONOutput{
			Path:             repo.RelativePath,
			Branch:           repo.Branch,
			Status:           repo.Status,
			CommitsAhead:     repo.CommitsAhead,
			CommitsBehind:    repo.CommitsBehind,
			UncommittedFiles: repo.UncommittedFiles,
			UntrackedFiles:   repo.UntrackedFiles,
			Stashed:          repo.Stashed,
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
