package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-core/cli"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	cleanFlags       BulkCommandFlags
	cleanForce       bool
	cleanDirs        bool
	cleanIgnored     bool
	cleanOnlyIgnored bool
	cleanExclude     []string
)

var cleanCmd = &cobra.Command{
	Use:   "clean [directory]",
	Short: "Remove untracked files from multiple repositories in parallel",
	Long: cliutil.QuickStartHelp(`  # Preview what would be cleaned (dry-run by default)
  gz-git clean

  # Actually delete untracked files
  gz-git clean --force

  # Also remove untracked directories
  gz-git clean --force --dirs

  # Remove ignored files too
  gz-git clean --force -x

  # Remove only ignored files (keep untracked)
  gz-git clean --force -X

  # Exclude specific patterns
  gz-git clean --force -e "*.log" -e "tmp/"

  # Filter repositories
  gz-git clean --force --include "myproject.*"

  # JSON output
  gz-git clean --format json`),
	Args: cobra.MaximumNArgs(1),
	RunE: runClean,
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	// Common bulk operation flags (without dry-run since we use --force instead)
	cleanCmd.Flags().IntVarP(&cleanFlags.Depth, "scan-depth", "d", repository.DefaultBulkMaxDepth, "directory depth to scan for repositories")
	cleanCmd.Flags().IntVarP(&cleanFlags.Parallel, "parallel", "j", repository.DefaultBulkParallel, "number of parallel operations")
	cleanCmd.Flags().BoolVarP(&cleanFlags.IncludeSubmodules, "recursive", "r", false, "recursively include nested repositories and submodules")
	cleanCmd.Flags().StringVar(&cleanFlags.Include, "include", "", "regex pattern to include repositories")
	cleanCmd.Flags().StringVar(&cleanFlags.Exclude, "exclude", "", "regex pattern to exclude repositories")
	cleanCmd.Flags().StringVar(&cleanFlags.Format, "format", "default", "output format: default, compact, json, llm")
	cleanCmd.Flags().BoolVar(&cleanFlags.Watch, "watch", false, "continuously run at intervals")
	cleanCmd.Flags().DurationVar(&cleanFlags.Interval, "interval", 5*time.Minute, "interval when watching")

	// Clean-specific flags
	cleanCmd.Flags().BoolVar(&cleanForce, "force", false, "actually delete files (without this flag, operates in dry-run mode)")
	cleanCmd.Flags().BoolVar(&cleanDirs, "dirs", false, "also remove untracked directories")
	cleanCmd.Flags().BoolVarP(&cleanIgnored, "ignored", "x", false, "also remove ignored files")
	cleanCmd.Flags().BoolVarP(&cleanOnlyIgnored, "only-ignored", "X", false, "remove only ignored files (not untracked)")
	cleanCmd.Flags().StringSliceVarP(&cleanExclude, "exclude-pattern", "e", nil, "exclude pattern for git clean (can be repeated)")
}

func runClean(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load config with profile support
	effective, _ := LoadEffectiveConfig(cmd, nil)
	if effective != nil {
		if !cmd.Flags().Changed("parallel") && effective.Parallel > 0 {
			cleanFlags.Parallel = effective.Parallel
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
	if err := validateBulkDepth(cmd, cleanFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(cleanFlags.Format); err != nil {
		return err
	}

	// Validate mutually exclusive flags
	if cleanIgnored && cleanOnlyIgnored {
		return fmt.Errorf("--ignored (-x) and --only-ignored (-X) are mutually exclusive")
	}

	// DryRun is default, --force disables it
	dryRun := !cleanForce

	// Create client
	client := repository.NewClient()

	// Create logger
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkCleanOptions{
		Directory:         directory,
		Parallel:          cleanFlags.Parallel,
		MaxDepth:          cleanFlags.Depth,
		DryRun:            dryRun,
		RemoveDirectories: cleanDirs,
		RemoveIgnored:     cleanIgnored,
		OnlyIgnored:       cleanOnlyIgnored,
		ExcludePatterns:   cleanExclude,
		IncludeSubmodules: cleanFlags.IncludeSubmodules,
		IncludePattern:    cleanFlags.Include,
		ExcludePattern:    cleanFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Cleaning", cleanFlags.Format, quiet),
	}

	// Watch mode
	if cleanFlags.Watch {
		return runCleanWatch(ctx, client, opts)
	}

	// One-time clean
	if shouldShowProgress(cleanFlags.Format, quiet) {
		printScanningMessage(directory, cleanFlags.Depth, cleanFlags.Parallel, dryRun)
	}

	result, err := client.BulkClean(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk clean failed: %w", err)
	}

	if shouldShowProgress(cleanFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	if cleanFlags.Format == "json" || !quiet {
		displayCleanResults(result, dryRun)
	}

	return nil
}

func runCleanWatch(ctx context.Context, client repository.Client, opts repository.BulkCleanOptions) error {
	cfg := WatchConfig{
		Interval:      cleanFlags.Interval,
		Format:        cleanFlags.Format,
		Quiet:         quiet,
		OperationName: "clean",
		Directory:     opts.Directory,
		MaxDepth:      opts.MaxDepth,
		Parallel:      opts.Parallel,
	}

	return RunBulkWatch(cfg, func() error {
		result, err := client.BulkClean(ctx, opts)
		if err != nil {
			return fmt.Errorf("bulk clean failed: %w", err)
		}
		if !quiet {
			displayCleanResults(result, opts.DryRun)
		}
		return nil
	})
}

func displayCleanResults(result *repository.BulkCleanResult, dryRun bool) {
	switch cleanFlags.Format {
	case "json":
		displayCleanResultsJSON(result)
		return
	case "llm":
		displayCleanResultsLLM(result)
		return
	}

	if verbose {
		displayCleanResultsVerbose(result, dryRun)
	} else {
		displayCleanResultsDefault(result, dryRun)
	}
}

func displayCleanResultsDefault(result *repository.BulkCleanResult, dryRun bool) {
	verb := "Cleaned"
	if dryRun {
		verb = "[DRY-RUN] Cleaned"
	}

	WriteSummaryLine(os.Stdout, verb, result.TotalProcessed, result.Summary, result.Duration)

	// Show repos with files to clean or errors
	for _, repo := range result.Repositories {
		if repo.Status == repository.StatusError ||
			repo.Status == repository.StatusWouldClean ||
			repo.Status == repository.StatusCleaned {
			displayCleanRepositoryResult(repo)
		}
	}

	if dryRun && result.TotalFiles > 0 {
		fmt.Printf("\nUse --force to actually delete files.\n")
	}
}

func displayCleanResultsVerbose(result *repository.BulkCleanResult, dryRun bool) {
	modeStr := "[DRY-RUN]"
	if !dryRun {
		modeStr = "[EXECUTE]"
	}

	fmt.Printf("\n%s Bulk Clean Report\n", modeStr)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Total scanned:   %d repositories\n", result.TotalScanned)
	fmt.Printf("Total processed: %d repositories\n", result.TotalProcessed)
	fmt.Printf("Duration:        %s\n", result.Duration.Round(100_000_000))
	fmt.Println()

	if len(result.Summary) > 0 {
		fmt.Println("Summary by status:")
		for status, count := range result.Summary {
			icon := getBulkStatusIconSimple(status)
			fmt.Printf("  %s %-20s %d\n", icon, status+":", count)
		}
		fmt.Println()
	}

	for _, repo := range result.Repositories {
		displayCleanRepositoryResult(repo)
	}

	fmt.Println(strings.Repeat("-", 60))
	if dryRun && result.TotalFiles > 0 {
		fmt.Printf("Would remove %d file(s) total. Use --force to actually delete.\n", result.TotalFiles)
	} else if !dryRun {
		fmt.Printf("Removed %d file(s) total.\n", result.TotalFiles)
	}
}

func displayCleanRepositoryResult(repo repository.RepositoryCleanResult) {
	icon := getBulkStatusIconSimple(repo.Status)

	pathPart := repo.RelativePath
	if repo.Branch != "" {
		pathPart += fmt.Sprintf(" (%s)", repo.Branch)
	}

	statusStr := repo.Status
	switch repo.Status {
	case repository.StatusCleaned:
		statusStr = fmt.Sprintf("removed %d file(s)", repo.FilesCount)
	case repository.StatusWouldClean:
		statusStr = fmt.Sprintf("would remove %d file(s)", repo.FilesCount)
	case repository.StatusNothingToClean:
		statusStr = "nothing to clean"
	case repository.StatusError:
		statusStr = "failed"
	}

	fmt.Printf("  %s %-50s %-25s", icon, pathPart, statusStr)
	if repo.Duration > 0 {
		fmt.Printf(" %6s", repo.Duration.Round(10_000_000))
	}
	fmt.Println()

	// Show file list in verbose mode
	if verbose && len(repo.FilesRemoved) > 0 {
		for _, f := range repo.FilesRemoved {
			fmt.Printf("      %s\n", f)
		}
	}

	if repo.Error != nil && (repo.Status == repository.StatusError || verbose) {
		fmt.Printf("    Error: %v\n", repo.Error)
	}
}

// CleanJSONOutput represents the JSON output structure for clean command
type CleanJSONOutput struct {
	TotalScanned   int                         `json:"total_scanned"`
	TotalProcessed int                         `json:"total_processed"`
	TotalFiles     int                         `json:"total_files"`
	DurationMs     int64                       `json:"duration_ms"`
	Summary        map[string]int              `json:"summary"`
	Repositories   []CleanRepositoryJSONOutput `json:"repositories"`
}

// CleanRepositoryJSONOutput represents a single repository in JSON output
type CleanRepositoryJSONOutput struct {
	Path         string   `json:"path"`
	Branch       string   `json:"branch,omitempty"`
	Status       string   `json:"status"`
	FilesCount   int      `json:"files_count,omitempty"`
	FilesRemoved []string `json:"files_removed,omitempty"`
	DurationMs   int64    `json:"duration_ms,omitempty"`
	Error        string   `json:"error,omitempty"`
}

func displayCleanResultsJSON(result *repository.BulkCleanResult) {
	output := CleanJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		TotalFiles:     result.TotalFiles,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]CleanRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := CleanRepositoryJSONOutput{
			Path:         repo.RelativePath,
			Branch:       repo.Branch,
			Status:       repo.Status,
			FilesCount:   repo.FilesCount,
			FilesRemoved: repo.FilesRemoved,
			DurationMs:   repo.Duration.Milliseconds(),
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

func displayCleanResultsLLM(result *repository.BulkCleanResult) {
	output := CleanJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		TotalFiles:     result.TotalFiles,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]CleanRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := CleanRepositoryJSONOutput{
			Path:         repo.RelativePath,
			Branch:       repo.Branch,
			Status:       repo.Status,
			FilesCount:   repo.FilesCount,
			FilesRemoved: repo.FilesRemoved,
			DurationMs:   repo.Duration.Milliseconds(),
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
