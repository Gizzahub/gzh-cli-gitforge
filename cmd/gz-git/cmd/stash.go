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
	stashBulkFlags      BulkCommandFlags
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

	// Bulk flags for each subcommand
	addStashBulkFlags(stashSaveCmd)
	addStashBulkFlags(stashListCmd)
	addStashBulkFlags(stashPopCmd)
}

func addStashBulkFlags(cmd *cobra.Command) {
	cmd.Flags().IntVarP(&stashBulkFlags.Depth, "scan-depth", "d", repository.DefaultBulkMaxDepth, "directory depth to scan")
	cmd.Flags().IntVarP(&stashBulkFlags.Parallel, "parallel", "j", repository.DefaultBulkParallel, "number of parallel operations")
	cmd.Flags().BoolVarP(&stashBulkFlags.DryRun, "dry-run", "n", false, "show what would be done")
	cmd.Flags().StringVar(&stashBulkFlags.Include, "include", "", "regex pattern to include repositories")
	cmd.Flags().StringVar(&stashBulkFlags.Exclude, "exclude", "", "regex pattern to exclude repositories")
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
		Parallel:         stashBulkFlags.Parallel,
		MaxDepth:         stashBulkFlags.Depth,
		DryRun:           stashBulkFlags.DryRun,
		Operation:        "save",
		Message:          stashMessage,
		IncludeUntracked: stashIncludeUntrack,
		IncludePattern:   stashBulkFlags.Include,
		ExcludePattern:   stashBulkFlags.Exclude,
		Logger:           repository.NewNoopLogger(),
	}

	if !quiet {
		modeStr := ""
		if stashBulkFlags.DryRun {
			modeStr = "[DRY-RUN] "
		}
		fmt.Printf("%sScanning for repositories in %s...\n", modeStr, directory)
	}

	result, err := client.BulkStash(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk stash save failed: %w", err)
	}

	printBulkStashResult(result, "save", stashBulkFlags.DryRun)
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
		Parallel:       stashBulkFlags.Parallel,
		MaxDepth:       stashBulkFlags.Depth,
		Operation:      "list",
		IncludePattern: stashBulkFlags.Include,
		ExcludePattern: stashBulkFlags.Exclude,
		Logger:         repository.NewNoopLogger(),
	}

	if !quiet {
		fmt.Printf("Scanning for repositories in %s...\n", directory)
	}

	result, err := client.BulkStash(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk stash list failed: %w", err)
	}

	printBulkStashResult(result, "list", false)
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
		Parallel:       stashBulkFlags.Parallel,
		MaxDepth:       stashBulkFlags.Depth,
		DryRun:         stashBulkFlags.DryRun,
		Operation:      "pop",
		IncludePattern: stashBulkFlags.Include,
		ExcludePattern: stashBulkFlags.Exclude,
		Logger:         repository.NewNoopLogger(),
	}

	if !quiet {
		modeStr := ""
		if stashBulkFlags.DryRun {
			modeStr = "[DRY-RUN] "
		}
		fmt.Printf("%sScanning for repositories in %s...\n", modeStr, directory)
	}

	result, err := client.BulkStash(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk stash pop failed: %w", err)
	}

	printBulkStashResult(result, "pop", stashBulkFlags.DryRun)
	return nil
}

func printBulkStashResult(result *repository.BulkStashResult, operation string, dryRun bool) {
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
