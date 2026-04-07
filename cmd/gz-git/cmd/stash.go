package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/stash"
)

var (
	stashMessage        string
	stashIncludeUntrack bool
	stashSaveBulkFlags  BulkCommandFlags
	stashListBulkFlags  BulkCommandFlags
	stashPopBulkFlags   BulkCommandFlags
)

// stashCmd represents the stash command group
var stashCmd = &cobra.Command{
	Use:   "stash",
	Short: "Stash management commands",
	Long: cliutil.QuickStartHelp(`  # Stash changes in current repo
  gz-git stash save -m "WIP: feature"

  # Stash all dirty repos (Bulk)
  gz-git stash save . -m "WIP: bulk stash"

  # List stashes
  gz-git stash list

  # Pop latest stash
  gz-git stash pop`),
	Example: ``,
	Args:    cobra.NoArgs,
}

// stashSaveCmd saves changes to stash
var stashSaveCmd = &cobra.Command{
	Use:   "save [directory]",
	Short: "Save changes to stash",
	Long: cliutil.QuickStartHelp(`  # Stash changes with message
  gz-git stash save -m "WIP: refactoring"

  # Include untracked files
  gz-git stash save -u -m "WIP: new files"

  # BULK: Stash all dirty repos
  gz-git stash save . -m "WIP: before branch switch"`),
	Example: ``,
	RunE:    runStashSave,
}

// stashListCmd lists stash entries
var stashListCmd = &cobra.Command{
	Use:   "list [directory]",
	Short: "List stash entries",
	Long: cliutil.QuickStartHelp(`  # List stashes in current repo
  gz-git stash list

  # BULK: List stashes across all repos
  gz-git stash list .`),
	Example: ``,
	RunE:    runStashList,
}

// stashPopCmd pops latest stash
var stashPopCmd = &cobra.Command{
	Use:   "pop [directory]",
	Short: "Pop latest stash",
	Long: cliutil.QuickStartHelp(`  # Pop latest stash
  gz-git stash pop

  # BULK: Pop stashes in all repos
  gz-git stash pop .

  # BULK: Dry-run to preview
  gz-git stash pop . -n`),
	Example: ``,
	RunE:    runStashPop,
}

func init() {
	rootCmd.AddCommand(stashCmd)
	stashCmd.AddCommand(stashSaveCmd)
	stashCmd.AddCommand(stashListCmd)
	stashCmd.AddCommand(stashPopCmd)

	// Save flags
	stashSaveCmd.Flags().StringVarP(&stashMessage, "message", "m", "", "stash message")
	stashSaveCmd.Flags().BoolVarP(&stashIncludeUntrack, "include-untracked", "u", false, "include untracked files")

	// Bulk flags for each subcommand (using shared addBulkFlags)
	addBulkFlags(stashSaveCmd, &stashSaveBulkFlags)
	addBulkFlags(stashListCmd, &stashListBulkFlags)
	addBulkFlags(stashPopCmd, &stashPopBulkFlags)
}

func runStashSave(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Bulk mode
	if len(args) > 0 {
		return runBulkStashSave(ctx, args[0])
	}

	// Single repo mode
	return runSingleStashSave(ctx)
}

func runSingleStashSave(ctx context.Context) error {
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	client := repository.NewClient()
	if !client.IsRepository(ctx, absPath) {
		return fmt.Errorf("not a git repository: %s", absPath)
	}

	repo, err := client.Open(ctx, absPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	mgr := stash.NewManager()
	opts := stash.SaveOptions{
		Message:          stashMessage,
		IncludeUntracked: stashIncludeUntrack,
	}

	if err := mgr.Save(ctx, repo, opts); err != nil {
		return fmt.Errorf("failed to save stash: %w", err)
	}

	if !quiet {
		msg := "Changes stashed"
		if stashMessage != "" {
			msg = fmt.Sprintf("Stashed: %s", stashMessage)
		}
		fmt.Printf("✓ %s\n", msg)
	}

	return nil
}

func runBulkStashSave(ctx context.Context, directory string) error {
	client := repository.NewClient()

	opts := repository.BulkStashOptions{
		Directory:        directory,
		Parallel:         stashSaveBulkFlags.Parallel,
		MaxDepth:         stashSaveBulkFlags.Depth,
		DryRun:           stashSaveBulkFlags.DryRun,
		Operation:        "save",
		Message:          stashMessage,
		IncludeUntracked: stashIncludeUntrack,
		IncludePattern:   stashSaveBulkFlags.Include,
		ExcludePattern:   stashSaveBulkFlags.Exclude,
		Logger:           repository.NewNoopLogger(),
	}

	if shouldShowProgress(stashSaveBulkFlags.Format, quiet) {
		printScanningMessage(directory, stashSaveBulkFlags.Depth, stashSaveBulkFlags.Parallel, stashSaveBulkFlags.DryRun)
	}

	result, err := client.BulkStash(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk stash save failed: %w", err)
	}

	printBulkStashResult(result, "save", stashSaveBulkFlags.DryRun, stashSaveBulkFlags.Format)
	return nil
}

func runStashList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Bulk mode
	if len(args) > 0 {
		return runBulkStashList(ctx, args[0])
	}

	// Single repo mode
	return runSingleStashList(ctx)
}

func runSingleStashList(ctx context.Context) error {
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	client := repository.NewClient()
	if !client.IsRepository(ctx, absPath) {
		return fmt.Errorf("not a git repository: %s", absPath)
	}

	repo, err := client.Open(ctx, absPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	mgr := stash.NewManager()
	stashes, err := mgr.List(ctx, repo, stash.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list stashes: %w", err)
	}

	if len(stashes) == 0 {
		if !quiet {
			fmt.Println("No stashes")
		}
		return nil
	}

	if !quiet {
		fmt.Printf("Stashes (%d):\n\n", len(stashes))
		for _, s := range stashes {
			msg := s.Message
			if msg == "" {
				msg = "(no message)"
			}
			fmt.Printf("  %s: %s\n", s.Ref, msg)
			if verbose && s.Branch != "" {
				fmt.Printf("         on branch: %s, created: %s\n", s.Branch, s.Date.Format("2006-01-02 15:04"))
			}
		}
	}

	return nil
}

func runBulkStashList(ctx context.Context, directory string) error {
	client := repository.NewClient()

	opts := repository.BulkStashOptions{
		Directory:      directory,
		Parallel:       stashListBulkFlags.Parallel,
		MaxDepth:       stashListBulkFlags.Depth,
		Operation:      "list",
		IncludePattern: stashListBulkFlags.Include,
		ExcludePattern: stashListBulkFlags.Exclude,
		Logger:         repository.NewNoopLogger(),
	}

	if shouldShowProgress(stashListBulkFlags.Format, quiet) {
		printScanningMessage(directory, stashListBulkFlags.Depth, stashListBulkFlags.Parallel, false)
	}

	result, err := client.BulkStash(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk stash list failed: %w", err)
	}

	printBulkStashResult(result, "list", false, stashListBulkFlags.Format)
	return nil
}

func runStashPop(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Bulk mode
	if len(args) > 0 {
		return runBulkStashPop(ctx, args[0])
	}

	// Single repo mode
	return runSingleStashPop(ctx)
}

func runSingleStashPop(ctx context.Context) error {
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	client := repository.NewClient()
	if !client.IsRepository(ctx, absPath) {
		return fmt.Errorf("not a git repository: %s", absPath)
	}

	repo, err := client.Open(ctx, absPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	mgr := stash.NewManager()
	if err := mgr.Pop(ctx, repo, stash.PopOptions{}); err != nil {
		return fmt.Errorf("failed to pop stash: %w", err)
	}

	if !quiet {
		fmt.Println("✓ Stash popped")
	}

	return nil
}

func runBulkStashPop(ctx context.Context, directory string) error {
	client := repository.NewClient()

	opts := repository.BulkStashOptions{
		Directory:      directory,
		Parallel:       stashPopBulkFlags.Parallel,
		MaxDepth:       stashPopBulkFlags.Depth,
		DryRun:         stashPopBulkFlags.DryRun,
		Operation:      "pop",
		IncludePattern: stashPopBulkFlags.Include,
		ExcludePattern: stashPopBulkFlags.Exclude,
		Logger:         repository.NewNoopLogger(),
	}

	if shouldShowProgress(stashPopBulkFlags.Format, quiet) {
		printScanningMessage(directory, stashPopBulkFlags.Depth, stashPopBulkFlags.Parallel, stashPopBulkFlags.DryRun)
	}

	result, err := client.BulkStash(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk stash pop failed: %w", err)
	}

	printBulkStashResult(result, "pop", stashPopBulkFlags.DryRun, stashPopBulkFlags.Format)
	return nil
}

func printBulkStashResult(result *repository.BulkStashResult, operation string, dryRun bool, format string) {
	// JSON or LLM output mode
	if format == "json" || format == "llm" {
		displayStashResultsStructured(result, operation, format)
		return
	}

	modeStr := ""
	if dryRun {
		modeStr = "[DRY-RUN] "
	}

	fmt.Printf("\n%sBulk Stash %s Report\n", modeStr, strings.Title(operation))
	fmt.Println(strings.Repeat("─", 50))

	// Show repos with stashes or changes
	for _, repo := range result.Repositories {
		switch repo.Status {
		case repository.StatusStashed, repository.StatusWouldStash:
			icon := "✓"
			if dryRun {
				icon = "→"
			}
			fmt.Printf("%s %s: %s\n", icon, repo.RelativePath, repo.Message)

		case repository.StatusPopped, repository.StatusWouldPop:
			icon := "✓"
			if dryRun {
				icon = "→"
			}
			fmt.Printf("%s %s: %s\n", icon, repo.RelativePath, repo.Message)

		case repository.StatusHasStash:
			fmt.Printf("📦 %s: %s\n", repo.RelativePath, repo.Message)

		case repository.StatusError:
			fmt.Printf("✗ %s: %s\n", repo.RelativePath, repo.Message)
		}
	}

	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Repositories: %d scanned, %d processed\n", result.TotalScanned, result.TotalProcessed)
	fmt.Printf("Total stashes: %d\n", result.TotalStashCount)
	fmt.Printf("Duration: %s\n", result.Duration.Round(time.Millisecond))
}

// StashJSONOutput represents the JSON output structure for stash command
type StashJSONOutput struct {
	Operation      string                      `json:"operation"`
	TotalScanned   int                         `json:"total_scanned"`
	TotalProcessed int                         `json:"total_processed"`
	TotalStashes   int                         `json:"total_stashes"`
	DurationMs     int64                       `json:"duration_ms"`
	Repositories   []StashRepositoryJSONOutput `json:"repositories"`
}

// StashRepositoryJSONOutput represents a single repository in JSON output
type StashRepositoryJSONOutput struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func displayStashResultsStructured(result *repository.BulkStashResult, operation string, format string) {
	output := StashJSONOutput{
		Operation:      operation,
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		TotalStashes:   result.TotalStashCount,
		DurationMs:     result.Duration.Milliseconds(),
		Repositories:   make([]StashRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		output.Repositories = append(output.Repositories, StashRepositoryJSONOutput{
			Path:    repo.RelativePath,
			Status:  repo.Status,
			Message: repo.Message,
		})
	}

	writeBulkOutput(format, output)
}
