package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-core/cli"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	pushFlags       BulkCommandFlags
	pushForce       bool
	pushSetUpstream bool
	pushTags        bool
	pushRefspec     string
	pushRemotes     []string
	pushAllRemotes  bool
	pushIgnoreDirty bool
)

// pushCmd represents the push command for multi-repository operations
var pushCmd = &cobra.Command{
	Use:   "push [directory]",
	Short: "Push commits to multiple repositories in parallel",
	Long: `Scan for Git repositories and push local commits to remote in parallel.

This command recursively scans the specified directory (or current directory)
for Git repositories and pushes local commits to their remotes in parallel.

For single repository operations, use 'git push' directly.

By default:
  - Scans 1 directory level deep
  - Processes 5 repositories in parallel
  - Pushes to origin remote only
  - Skips repositories without remotes or upstreams

The command pushes your local commits to remote repositories.`,
	Example: `  # Push all repositories in current directory
  gz-git push

  # Push all repositories up to 2 levels deep
  gz-git push -d 2 ~/projects

  # Push with custom parallelism
  gz-git push --parallel 10 ~/workspace

  # Force push (use with caution!)
  gz-git push --force ~/projects

  # Push and set upstream for new branches
  gz-git push --set-upstream ~/projects

  # Push all tags
  gz-git push --tags ~/repos

  # Push with custom refspec (local:remote branch mapping)
  gz-git push --refspec develop:master ~/projects

  # Push to multiple remotes
  gz-git push --remote origin --remote backup ~/projects

  # Push to all configured remotes
  gz-git push --all-remotes ~/projects

  # Combine refspec with multiple remotes
  gz-git push --refspec develop:master --remote origin --remote backup ~/work

  # Dry run to see what would be pushed
  gz-git push --dry-run ~/projects

  # Filter by pattern
  gz-git push --include "myproject.*" ~/workspace

  # Exclude pattern
  gz-git push --exclude "test.*" ~/projects

  # Skip dirty status check (useful for CI/CD)
  gz-git push --ignore-dirty ~/projects

  # Compact output format
  gz-git push --format compact ~/projects

  # Continuously push at intervals (watch mode)
  gz-git push --scan-depth 2 --watch --interval 10m ~/projects

  # Watch with shorter interval
  gz-git push --watch --interval 5m ~/work`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)

	// Common bulk operation flags
	addBulkFlags(pushCmd, &pushFlags)

	// Push-specific flags (no -f shorthand for force, conflicts with --format)
	pushCmd.Flags().BoolVar(&pushForce, "force", false, "force push (use with caution!)")
	pushCmd.Flags().BoolVarP(&pushSetUpstream, "set-upstream", "u", false, "set upstream for new branches")
	pushCmd.Flags().BoolVarP(&pushTags, "tags", "t", false, "push all tags to remote")
	pushCmd.Flags().StringVar(&pushRefspec, "refspec", "", "custom refspec (e.g., 'develop:master' to push local develop to remote master)")
	pushCmd.Flags().StringSliceVar(&pushRemotes, "remote", []string{}, "remote(s) to push to (can be specified multiple times)")
	pushCmd.Flags().BoolVar(&pushAllRemotes, "all-remotes", false, "push to all configured remotes")
	pushCmd.Flags().BoolVar(&pushIgnoreDirty, "ignore-dirty", false, "skip dirty status check and warning (useful for CI/CD)")
}

func runPush(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load config with profile support
	effective, _ := LoadEffectiveConfig(cmd, nil)
	if effective != nil {
		// Apply config if flag not explicitly set
		if !cmd.Flags().Changed("parallel") && effective.Parallel > 0 {
			pushFlags.Parallel = effective.Parallel
		}
		if !cmd.Flags().Changed("set-upstream") {
			pushSetUpstream = effective.Push.SetUpstream
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
	if err := validateBulkDepth(cmd, pushFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(pushFlags.Format); err != nil {
		return err
	}

	// Validate refspec if provided
	if pushRefspec != "" {
		if _, err := repository.ValidateRefspec(pushRefspec); err != nil {
			return fmt.Errorf("invalid refspec: %w", err)
		}
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkPushOptions{
		Directory:         directory,
		Parallel:          pushFlags.Parallel,
		MaxDepth:          pushFlags.Depth,
		DryRun:            pushFlags.DryRun,
		Verbose:           verbose,
		Force:             pushForce,
		SetUpstream:       pushSetUpstream,
		Tags:              pushTags,
		Refspec:           pushRefspec,
		Remotes:           pushRemotes,
		AllRemotes:        pushAllRemotes,
		IgnoreDirty:       pushIgnoreDirty,
		IncludeSubmodules: pushFlags.IncludeSubmodules,
		IncludePattern:    pushFlags.Include,
		ExcludePattern:    pushFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Pushing", pushFlags.Format, quiet),
	}

	// Watch mode: continuously push at intervals
	if pushFlags.Watch {
		return runPushWatch(ctx, client, opts)
	}

	// One-time push
	if shouldShowProgress(pushFlags.Format, quiet) {
		printScanningMessage(directory, pushFlags.Depth, pushFlags.Parallel, pushFlags.DryRun)
	}

	result, err := client.BulkPush(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk push failed: %w", err)
	}

	// Display scan completion message
	if shouldShowProgress(pushFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// Display results (always output for JSON format, otherwise respect quiet flag)
	if pushFlags.Format == "json" || !quiet {
		displayPushResults(result)
	}

	return nil
}

func runPushWatch(ctx context.Context, client repository.Client, opts repository.BulkPushOptions) error {
	cfg := WatchConfig{
		Interval:      pushFlags.Interval,
		Format:        pushFlags.Format,
		Quiet:         quiet,
		OperationName: "push",
		Directory:     opts.Directory,
		MaxDepth:      opts.MaxDepth,
		Parallel:      opts.Parallel,
	}

	return RunBulkWatch(cfg, func() error {
		return executePush(ctx, client, opts)
	})
}

func executePush(ctx context.Context, client repository.Client, opts repository.BulkPushOptions) error {
	result, err := client.BulkPush(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk push failed: %w", err)
	}

	// Display results
	if !quiet {
		displayPushResults(result)
	}

	return nil
}

func displayPushResults(result *repository.BulkPushResult) {
	// JSON output mode
	if pushFlags.Format == "json" {
		displayPushResultsJSON(result)
		return
	}

	// LLM output mode
	if pushFlags.Format == "llm" {
		displayPushResultsLLM(result)
		return
	}

	fmt.Println()
	fmt.Println("=== Push Results ===")
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
	if pushFlags.Format != "compact" && len(result.Repositories) > 0 {
		fmt.Println("Repository details:")
		for _, repo := range result.Repositories {
			displayPushRepositoryResult(repo)
		}
	}

	// Display only errors/warnings in compact mode
	if pushFlags.Format == "compact" {
		hasIssues := false
		for _, repo := range result.Repositories {
			if repo.Status == "error" || repo.Status == "no-remote" || repo.Status == "no-upstream" ||
				repo.Status == "conflict" || repo.Status == "rebase-in-progress" || repo.Status == "merge-in-progress" {
				if !hasIssues {
					fmt.Println("Issues found:")
					hasIssues = true
				}
				displayPushRepositoryResult(repo)
			}
		}
		if !hasIssues {
			fmt.Println("✓ All repositories pushed successfully")
		}
	}

	// Display dirty repositories warning
	dirtyCount := countDirtyRepositories(result.Repositories)
	if dirtyCount > 0 {
		fmt.Println()
		fmt.Printf("⚠ Warning: %d repository(ies) have uncommitted changes\n", dirtyCount)
	}
}

// countDirtyRepositories counts repositories with uncommitted or untracked files.
func countDirtyRepositories(repos []repository.RepositoryPushResult) int {
	count := 0
	for _, repo := range repos {
		if repo.UncommittedFiles > 0 || repo.UntrackedFiles > 0 {
			count++
		}
	}
	return count
}

func displayPushRepositoryResult(repo repository.RepositoryPushResult) {
	// Determine icon based on actual result, not just status
	// ✓ = changes pushed, = = no changes (up-to-date)
	// ⚠ = dirty (has uncommitted/untracked files)
	icon := getBulkStatusIcon(repo.Status, repo.PushedCommits)

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

	// Show error details - always show for refspec errors, otherwise only in verbose mode
	if repo.Error != nil {
		errMsg := repo.Error.Error()
		isRefspecError := strings.Contains(errMsg, "not found in repository") || strings.Contains(errMsg, "does not exist")

		// Always show refspec source branch errors (critical for user understanding)
		// Show other errors only in verbose mode
		if isRefspecError || verbose {
			fmt.Printf("    ⚠  %v\n", repo.Error)
		}
	}
}

// PushJSONOutput represents the JSON output structure for push command
type PushJSONOutput struct {
	TotalScanned   int                        `json:"total_scanned"`
	TotalProcessed int                        `json:"total_processed"`
	DurationMs     int64                      `json:"duration_ms"`
	Summary        map[string]int             `json:"summary"`
	Repositories   []PushRepositoryJSONOutput `json:"repositories"`
}

// PushRepositoryJSONOutput represents a single repository in JSON output
type PushRepositoryJSONOutput struct {
	Path             string `json:"path"`
	Branch           string `json:"branch,omitempty"`
	Status           string `json:"status"`
	CommitsAhead     int    `json:"commits_ahead,omitempty"`
	PushedCommits    int    `json:"pushed_commits,omitempty"`
	UncommittedFiles int    `json:"uncommitted_files,omitempty"`
	UntrackedFiles   int    `json:"untracked_files,omitempty"`
	DurationMs       int64  `json:"duration_ms,omitempty"`
	Error            string `json:"error,omitempty"`
}

func displayPushResultsJSON(result *repository.BulkPushResult) {
	output := PushJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]PushRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := PushRepositoryJSONOutput{
			Path:             repo.RelativePath,
			Branch:           repo.Branch,
			Status:           repo.Status,
			CommitsAhead:     repo.CommitsAhead,
			PushedCommits:    repo.PushedCommits,
			UncommittedFiles: repo.UncommittedFiles,
			UntrackedFiles:   repo.UntrackedFiles,
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

func displayPushResultsLLM(result *repository.BulkPushResult) {
	output := PushJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]PushRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := PushRepositoryJSONOutput{
			Path:             repo.RelativePath,
			Branch:           repo.Branch,
			Status:           repo.Status,
			CommitsAhead:     repo.CommitsAhead,
			PushedCommits:    repo.PushedCommits,
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
