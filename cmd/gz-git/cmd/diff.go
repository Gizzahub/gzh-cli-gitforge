package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-core/cli"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	diffFlags          BulkCommandFlags
	diffStaged         bool
	diffIncludeUntrack bool
	diffContextLines   int
	diffMaxSize        int
	diffNoDiffContent  bool
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff [directory]",
	Short: "Show diffs across multiple repositories",
	Long: cliutil.QuickStartHelp(`  # Show diffs for all repositories in current directory
  gz-git diff --scan-depth 1

  # Show only staged changes
  gz-git diff --staged ~/projects

  # Include untracked files
  gz-git diff --include-untracked ~/projects

  # Summary only (no diff content)
  gz-git diff --no-content ~/projects

  # JSON output (for scripting/LLM)
  gz-git diff --format json ~/projects`),
	Example: ``,
	Args:    cobra.MaximumNArgs(1),
	RunE:    runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)

	// Common bulk operation flags
	addBulkFlags(diffCmd, &diffFlags)

	// Diff-specific flags
	diffCmd.Flags().BoolVar(&diffStaged, "staged", false, "show only staged changes (--cached)")
	diffCmd.Flags().BoolVar(&diffIncludeUntrack, "include-untracked", false, "include untracked files in output")
	diffCmd.Flags().IntVarP(&diffContextLines, "context", "U", 3, "number of context lines around changes")
	diffCmd.Flags().IntVar(&diffMaxSize, "max-size", 100, "max diff size per repository in KB")
	diffCmd.Flags().BoolVar(&diffNoDiffContent, "no-content", false, "show summary only, no diff content")
}

func runDiff(cmd *cobra.Command, args []string) error {
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
	if err := validateBulkDepth(cmd, diffFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(diffFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkDiffOptions{
		Directory:         directory,
		Parallel:          diffFlags.Parallel,
		MaxDepth:          diffFlags.Depth,
		Staged:            diffStaged,
		IncludeUntracked:  diffIncludeUntrack,
		ContextLines:      diffContextLines,
		MaxDiffSize:       diffMaxSize * 1024, // Convert KB to bytes
		Verbose:           verbose,
		IncludeSubmodules: diffFlags.IncludeSubmodules,
		IncludePattern:    diffFlags.Include,
		ExcludePattern:    diffFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Scanning", diffFlags.Format, quiet),
	}

	// Scanning phase
	if shouldShowProgress(diffFlags.Format, quiet) {
		printScanningMessage(directory, diffFlags.Depth, diffFlags.Parallel, false)
	}

	// Execute bulk diff
	result, err := client.BulkDiff(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk diff failed: %w", err)
	}

	// Display results
	if diffFlags.Format == "json" || !quiet {
		displayDiffResults(result)
	}

	return nil
}

func displayDiffResults(result *repository.BulkDiffResult) {
	// JSON output mode
	if diffFlags.Format == "json" {
		displayDiffResultsJSON(result)
		return
	}

	// LLM output mode
	if diffFlags.Format == "llm" {
		displayDiffResultsLLM(result)
		return
	}

	// Compact mode: unchanged
	if diffFlags.Format == "compact" {
		displayDiffResultsCompact(result)
		return
	}

	if verbose {
		// Verbose: full diff output (old default behavior)
		displayDiffResultsDefault(result)
	} else {
		// Default: summary line + changed repos brief (no diff content)
		fmt.Println()
		durationStr := result.Duration.Round(100 * time.Millisecond).String()
		fmt.Printf("Diff %d repos  [⚠%d changed  ✓%d clean]  %s\n",
			result.TotalScanned, result.TotalWithChanges, result.TotalClean, durationStr)

		for _, repo := range result.Repositories {
			if repo.Status == "clean" {
				continue
			}
			icon := getDiffStatusIcon(repo.Status)
			changes := fmt.Sprintf("+%d/-%d", repo.Additions, repo.Deletions)
			fmt.Printf("  %s %-40s (%s)  %d files  %-10s %s\n",
				icon, repo.RelativePath, repo.Branch, repo.FilesChanged, changes, repo.DiffSummary)
		}

		if result.TotalWithChanges == 0 {
			fmt.Println("✓ All repositories are clean")
		}
	}
}

func displayDiffResultsDefault(result *repository.BulkDiffResult) {
	fmt.Println()
	fmt.Println("=== Bulk Diff Results ===")
	fmt.Printf("Total scanned:     %d repositories\n", result.TotalScanned)
	fmt.Printf("With changes:      %d repositories\n", result.TotalWithChanges)
	fmt.Printf("Clean:             %d repositories\n", result.TotalClean)
	fmt.Printf("Duration:          %s\n", result.Duration.Round(100_000_000))
	fmt.Println()

	// Show each repository's diff
	for _, repo := range result.Repositories {
		if repo.Status == "clean" && !verbose {
			continue
		}

		displayDiffRepositoryResult(repo)
	}

	if result.TotalWithChanges == 0 {
		fmt.Println("✓ All repositories are clean")
	}
}

func displayDiffResultsCompact(result *repository.BulkDiffResult) {
	fmt.Println()
	fmt.Println("=== Bulk Diff Summary ===")
	fmt.Printf("Total: %d | With changes: %d | Clean: %d\n",
		result.TotalScanned, result.TotalWithChanges, result.TotalClean)
	fmt.Println()

	if result.TotalWithChanges == 0 {
		fmt.Println("✓ All repositories are clean")
		return
	}

	fmt.Printf("%-40s %-12s %-8s %-8s %s\n", "Repository", "Branch", "Files", "+/-", "Summary")
	fmt.Println(strings.Repeat("-", 90))

	for _, repo := range result.Repositories {
		if repo.Status == "clean" {
			continue
		}

		icon := getDiffStatusIcon(repo.Status)
		path := repo.RelativePath
		if len(path) > 38 {
			path = "..." + path[len(path)-35:]
		}

		changes := fmt.Sprintf("+%d/-%d", repo.Additions, repo.Deletions)
		summary := repo.DiffSummary
		if len(summary) > 30 {
			summary = summary[:27] + "..."
		}
		if repo.Truncated {
			summary += " [truncated]"
		}

		fmt.Printf("%s %-38s %-12s %-8d %-8s %s\n",
			icon, path, repo.Branch, repo.FilesChanged, changes, summary)
	}
}

func displayDiffRepositoryResult(repo repository.RepositoryDiffResult) {
	icon := getDiffStatusIcon(repo.Status)

	// Header
	fmt.Printf("\n%s === %s (%s) ===\n", icon, repo.RelativePath, repo.Branch)

	if repo.Error != nil {
		fmt.Printf("  Error: %v\n", repo.Error)
		return
	}

	if repo.Status == "clean" {
		fmt.Println("  No changes")
		return
	}

	// Summary line
	if repo.DiffSummary != "" {
		fmt.Printf("  %s\n", repo.DiffSummary)
	}

	// Changed files list
	if len(repo.ChangedFiles) > 0 {
		fmt.Println("  Changed files:")
		for _, file := range repo.ChangedFiles {
			statusIcon := getFileStatusIcon(file.Status)
			if file.OldPath != "" {
				fmt.Printf("    %s %s → %s\n", statusIcon, file.OldPath, file.Path)
			} else {
				fmt.Printf("    %s %s\n", statusIcon, file.Path)
			}
		}
	}

	// Untracked files
	if len(repo.UntrackedFiles) > 0 {
		fmt.Println("  Untracked files:")
		for _, file := range repo.UntrackedFiles {
			fmt.Printf("    ? %s\n", file)
		}
	}

	// Diff content (unless --no-content)
	if !diffNoDiffContent && repo.DiffContent != "" {
		fmt.Println()
		fmt.Println("  --- Diff ---")
		// Indent diff content
		lines := strings.Split(repo.DiffContent, "\n")
		for _, line := range lines {
			if line == "" {
				fmt.Println()
			} else {
				fmt.Printf("  %s\n", line)
			}
		}
		if repo.Truncated {
			fmt.Println("  ... [truncated due to size limit] ...")
		}
	}
}

func getDiffStatusIcon(status string) string {
	switch status {
	case "has-changes":
		return "⚠"
	case "clean":
		return "✓"
	case "error":
		return "✗"
	default:
		return "•"
	}
}

func getFileStatusIcon(status string) string {
	switch status {
	case "M":
		return "M" // Modified
	case "A":
		return "A" // Added
	case "D":
		return "D" // Deleted
	case "R":
		return "R" // Renamed
	case "C":
		return "C" // Copied
	default:
		return "?"
	}
}

// DiffJSONOutput represents the JSON output structure for diff command
type DiffJSONOutput struct {
	TotalScanned     int                        `json:"total_scanned"`
	TotalWithChanges int                        `json:"total_with_changes"`
	TotalClean       int                        `json:"total_clean"`
	DurationMs       int64                      `json:"duration_ms"`
	Summary          map[string]int             `json:"summary"`
	Repositories     []DiffRepositoryJSONOutput `json:"repositories"`
}

// DiffRepositoryJSONOutput represents a single repository in JSON output
type DiffRepositoryJSONOutput struct {
	Path           string                  `json:"path"`
	Branch         string                  `json:"branch,omitempty"`
	Status         string                  `json:"status"`
	FilesChanged   int                     `json:"files_changed,omitempty"`
	Additions      int                     `json:"additions,omitempty"`
	Deletions      int                     `json:"deletions,omitempty"`
	DiffSummary    string                  `json:"diff_summary,omitempty"`
	DiffContent    string                  `json:"diff_content,omitempty"`
	ChangedFiles   []ChangedFileJSONOutput `json:"changed_files,omitempty"`
	UntrackedFiles []string                `json:"untracked_files,omitempty"`
	Truncated      bool                    `json:"truncated,omitempty"`
	DurationMs     int64                   `json:"duration_ms,omitempty"`
	Error          string                  `json:"error,omitempty"`
}

// ChangedFileJSONOutput represents a changed file in JSON output
type ChangedFileJSONOutput struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	OldPath string `json:"old_path,omitempty"`
}

func displayDiffResultsJSON(result *repository.BulkDiffResult) {
	output := DiffJSONOutput{
		TotalScanned:     result.TotalScanned,
		TotalWithChanges: result.TotalWithChanges,
		TotalClean:       result.TotalClean,
		DurationMs:       result.Duration.Milliseconds(),
		Summary:          result.Summary,
		Repositories:     make([]DiffRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := DiffRepositoryJSONOutput{
			Path:           repo.RelativePath,
			Branch:         repo.Branch,
			Status:         repo.Status,
			FilesChanged:   repo.FilesChanged,
			Additions:      repo.Additions,
			Deletions:      repo.Deletions,
			DiffSummary:    repo.DiffSummary,
			UntrackedFiles: repo.UntrackedFiles,
			Truncated:      repo.Truncated,
			DurationMs:     repo.Duration.Milliseconds(),
		}

		// Include diff content unless --no-content
		if !diffNoDiffContent {
			repoOutput.DiffContent = repo.DiffContent
		}

		// Convert changed files
		for _, file := range repo.ChangedFiles {
			repoOutput.ChangedFiles = append(repoOutput.ChangedFiles, ChangedFileJSONOutput{
				Path:    file.Path,
				Status:  file.Status,
				OldPath: file.OldPath,
			})
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

func displayDiffResultsLLM(result *repository.BulkDiffResult) {
	output := DiffJSONOutput{
		TotalScanned:     result.TotalScanned,
		TotalWithChanges: result.TotalWithChanges,
		TotalClean:       result.TotalClean,
		DurationMs:       result.Duration.Milliseconds(),
		Summary:          result.Summary,
		Repositories:     make([]DiffRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := DiffRepositoryJSONOutput{
			Path:           repo.RelativePath,
			Branch:         repo.Branch,
			Status:         repo.Status,
			FilesChanged:   repo.FilesChanged,
			Additions:      repo.Additions,
			Deletions:      repo.Deletions,
			DiffSummary:    repo.DiffSummary,
			UntrackedFiles: repo.UntrackedFiles,
			Truncated:      repo.Truncated,
			DurationMs:     repo.Duration.Milliseconds(),
		}

		// Include diff content unless --no-content
		if !diffNoDiffContent {
			repoOutput.DiffContent = repo.DiffContent
		}

		// Convert changed files
		for _, file := range repo.ChangedFiles {
			repoOutput.ChangedFiles = append(repoOutput.ChangedFiles, ChangedFileJSONOutput{
				Path:    file.Path,
				Status:  file.Status,
				OldPath: file.OldPath,
			})
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
