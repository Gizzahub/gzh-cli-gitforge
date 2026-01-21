package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/branch"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// cleanupBranchBulkFlags holds bulk-specific flags
var cleanupBranchBulkFlags BulkCommandFlags

var (
	cleanupBranchMerged     bool
	cleanupBranchStale      bool
	cleanupBranchGone       bool
	cleanupBranchStaleDays  int
	cleanupBranchDryRun     bool
	cleanupBranchForce      bool
	cleanupBranchRemote     bool
	cleanupBranchProtect    string
	cleanupBranchBaseBranch string
)

// cleanupBranchCmd represents the cleanup branch command
var cleanupBranchCmd = &cobra.Command{
	Use:   "branch [directory]",
	Short: "Clean up merged, stale, or gone branches",
	Long: cliutil.QuickStartHelp(`  # Preview merged branches in current repo
  gz-git cleanup branch --merged

  # Preview stale branches (no activity for 30 days)
  gz-git cleanup branch --stale

  # Actually delete merged branches
  gz-git cleanup branch --merged --force

  # BULK MODE: Clean up merged branches across all repos
  gz-git cleanup branch --merged --force .

  # Protect additional branches
  gz-git cleanup branch --merged --protect "staging,qa" --force`),
	Example: ``,
	RunE:    runCleanupBranch,
}

func init() {
	cleanupCmd.AddCommand(cleanupBranchCmd)

	// Cleanup-specific flags
	cleanupBranchCmd.Flags().BoolVar(&cleanupBranchMerged, "merged", false, "clean up fully merged branches")
	cleanupBranchCmd.Flags().BoolVar(&cleanupBranchStale, "stale", false, "clean up stale branches (no recent activity)")
	cleanupBranchCmd.Flags().BoolVar(&cleanupBranchGone, "gone", false, "clean up gone branches (remote deleted)")
	cleanupBranchCmd.Flags().IntVar(&cleanupBranchStaleDays, "stale-days", 30, "days threshold for stale branches")
	cleanupBranchCmd.Flags().BoolVarP(&cleanupBranchDryRun, "dry-run", "n", true, "preview changes without deleting (default: true)")
	cleanupBranchCmd.Flags().BoolVar(&cleanupBranchForce, "force", false, "actually delete branches (disables dry-run)")
	cleanupBranchCmd.Flags().BoolVarP(&cleanupBranchRemote, "remote", "r", false, "also delete remote branches")
	cleanupBranchCmd.Flags().StringVar(&cleanupBranchProtect, "protect", "", "additional branches to protect (comma-separated)")
	cleanupBranchCmd.Flags().StringVar(&cleanupBranchBaseBranch, "base", "", "base branch for merge detection (default: auto-detect)")

	// Bulk operation flags (manually added to avoid dry-run conflict)
	cleanupBranchCmd.Flags().IntVarP(&cleanupBranchBulkFlags.Depth, "scan-depth", "d", repository.DefaultBulkMaxDepth, "directory depth to scan for repositories")
	cleanupBranchCmd.Flags().IntVarP(&cleanupBranchBulkFlags.Parallel, "parallel", "j", repository.DefaultBulkParallel, "number of parallel operations")
	cleanupBranchCmd.Flags().BoolVar(&cleanupBranchBulkFlags.IncludeSubmodules, "recursive", false, "recursively include nested repositories and submodules")
	cleanupBranchCmd.Flags().StringVar(&cleanupBranchBulkFlags.Include, "include", "", "regex pattern to include repositories")
	cleanupBranchCmd.Flags().StringVar(&cleanupBranchBulkFlags.Exclude, "exclude", "", "regex pattern to exclude repositories")
}

func runCleanupBranch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Require at least one cleanup type
	if !cleanupBranchMerged && !cleanupBranchStale && !cleanupBranchGone {
		return fmt.Errorf("specify at least one cleanup type: --merged, --stale, or --gone")
	}

	// Force flag disables dry-run
	if cleanupBranchForce {
		cleanupBranchDryRun = false
	}

	// Build exclude list
	excludePatterns := []string{}
	if cleanupBranchProtect != "" {
		for _, p := range strings.Split(cleanupBranchProtect, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				excludePatterns = append(excludePatterns, p)
			}
		}
	}

	// If directory argument provided, run in bulk mode
	if len(args) > 0 {
		return runBulkCleanupBranch(ctx, args[0], excludePatterns)
	}

	// Single repository mode
	return runSingleRepoCleanupBranch(ctx, excludePatterns)
}

// runSingleRepoCleanupBranch performs cleanup on a single repository
func runSingleRepoCleanupBranch(ctx context.Context, excludePatterns []string) error {
	// Get repository path
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Create client
	client := repository.NewClient()

	// Check if it's a repository
	if !client.IsRepository(ctx, absPath) {
		return fmt.Errorf("not a git repository: %s", absPath)
	}

	// Open repository
	repo, err := client.Open(ctx, absPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Create cleanup service
	svc := branch.NewCleanupService()

	// Analyze branches
	analyzeOpts := branch.AnalyzeOptions{
		IncludeMerged:  cleanupBranchMerged,
		IncludeStale:   cleanupBranchStale,
		StaleThreshold: time.Duration(cleanupBranchStaleDays) * 24 * time.Hour,
		IncludeRemote:  cleanupBranchRemote,
		Exclude:        excludePatterns,
		BaseBranch:     cleanupBranchBaseBranch,
	}

	if !quiet {
		fmt.Println("Analyzing branches...")
	}

	report, err := svc.Analyze(ctx, repo, analyzeOpts)
	if err != nil {
		return fmt.Errorf("failed to analyze branches: %w", err)
	}

	// Display report
	if !quiet {
		printCleanupBranchReport(report, cleanupBranchDryRun)
	}

	// If no branches to clean up, exit
	if report.IsEmpty() {
		if !quiet {
			fmt.Println("\n✓ No branches to clean up")
		}
		return nil
	}

	// Execute cleanup if not dry-run
	if !cleanupBranchDryRun {
		executeOpts := branch.ExecuteOptions{
			DryRun:  false,
			Force:   true, // We already confirmed with --force flag
			Remote:  cleanupBranchRemote,
			Confirm: true, // Skip confirmation
			Exclude: excludePatterns,
		}

		if err := svc.Execute(ctx, repo, report, executeOpts); err != nil {
			return fmt.Errorf("failed to execute cleanup: %w", err)
		}

		if !quiet {
			fmt.Printf("\n✓ Deleted %d branch(es)\n", report.CountBranches())
		}
	} else {
		if !quiet {
			fmt.Println("\nDry-run mode: use --force to actually delete branches")
		}
	}

	return nil
}

// runBulkCleanupBranch performs cleanup across multiple repositories
func runBulkCleanupBranch(ctx context.Context, directory string, excludePatterns []string) error {
	client := repository.NewClient()

	opts := repository.BulkCleanupOptions{
		Directory:         directory,
		Parallel:          cleanupBranchBulkFlags.Parallel,
		MaxDepth:          cleanupBranchBulkFlags.Depth,
		DryRun:            cleanupBranchDryRun,
		IncludeMerged:     cleanupBranchMerged,
		IncludeStale:      cleanupBranchStale,
		IncludeGone:       cleanupBranchGone,
		StaleThreshold:    time.Duration(cleanupBranchStaleDays) * 24 * time.Hour,
		BaseBranch:        cleanupBranchBaseBranch,
		DeleteRemote:      cleanupBranchRemote,
		ProtectPatterns:   excludePatterns,
		IncludeSubmodules: cleanupBranchBulkFlags.IncludeSubmodules,
		IncludePattern:    cleanupBranchBulkFlags.Include,
		ExcludePattern:    cleanupBranchBulkFlags.Exclude,
		Logger:            repository.NewNoopLogger(),
	}

	if !quiet {
		modeStr := "[DRY-RUN]"
		if !cleanupBranchDryRun {
			modeStr = "[EXECUTE]"
		}
		fmt.Printf("%s Scanning for repositories in %s...\n", modeStr, directory)
	}

	result, err := client.BulkCleanup(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk cleanup failed: %w", err)
	}

	// Print results
	printBulkCleanupBranchResult(result, cleanupBranchDryRun)

	return nil
}

// printBulkCleanupBranchResult displays bulk cleanup results
func printBulkCleanupBranchResult(result *repository.BulkCleanupResult, dryRun bool) {
	modeStr := "[DRY-RUN]"
	if !dryRun {
		modeStr = "[EXECUTE]"
	}

	fmt.Printf("\n%s Bulk Branch Cleanup Report\n", modeStr)
	fmt.Println(strings.Repeat("─", 60))

	// Group by status
	cleanedUp := 0
	wouldCleanup := 0
	nothingToDo := 0
	errors := 0

	for _, repo := range result.Repositories {
		switch repo.Status {
		case repository.StatusCleanedUp:
			cleanedUp++
			if verbose {
				fmt.Printf("✓ %s: %s\n", repo.RelativePath, repo.Message)
			}
		case repository.StatusWouldCleanup:
			wouldCleanup++
			if !quiet {
				branchList := ""
				if len(repo.DeletedBranches) > 0 {
					branchList = fmt.Sprintf(" [%s]", strings.Join(repo.DeletedBranches, ", "))
				}
				fmt.Printf("→ %s: %s%s\n", repo.RelativePath, repo.Message, branchList)
			}
		case repository.StatusNothingToDo:
			nothingToDo++
		case repository.StatusError:
			errors++
			if !quiet {
				fmt.Printf("✗ %s: %s\n", repo.RelativePath, repo.Message)
			}
		}
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Repositories: %d scanned, %d processed\n", result.TotalScanned, result.TotalProcessed)
	fmt.Printf("Branches: %d analyzed\n", result.TotalBranchesAnalyzed)

	if dryRun {
		fmt.Printf("Would clean up: %d repo(s), Nothing to do: %d, Errors: %d\n", wouldCleanup, nothingToDo, errors)
		fmt.Printf("\nDry-run mode: use --force to actually delete branches\n")
	} else {
		fmt.Printf("Cleaned up: %d repo(s), Deleted: %d branch(es), Errors: %d\n", cleanedUp, result.TotalBranchesDeleted, errors)
	}

	fmt.Printf("Duration: %s\n", result.Duration.Round(time.Millisecond))
}

// printCleanupBranchReport displays the cleanup analysis report.
func printCleanupBranchReport(report *branch.CleanupReport, dryRun bool) {
	modeStr := "[DRY-RUN]"
	if !dryRun {
		modeStr = "[EXECUTE]"
	}

	fmt.Printf("\n%s Branch Cleanup Report\n", modeStr)
	fmt.Println(strings.Repeat("─", 50))

	// Merged branches
	if len(report.Merged) > 0 {
		fmt.Printf("\n📦 Merged branches (%d):\n", len(report.Merged))
		for _, b := range report.Merged {
			fmt.Printf("   • %s\n", b.Name)
		}
	}

	// Stale branches
	if len(report.Stale) > 0 {
		fmt.Printf("\n⏰ Stale branches (%d):\n", len(report.Stale))
		for _, b := range report.Stale {
			ageStr := ""
			if b.UpdatedAt != nil {
				age := time.Since(*b.UpdatedAt)
				ageStr = fmt.Sprintf(" (%.0f days old)", age.Hours()/24)
			}
			fmt.Printf("   • %s%s\n", b.Name, ageStr)
		}
	}

	// Orphaned branches
	if len(report.Orphaned) > 0 {
		fmt.Printf("\n👻 Gone branches (%d):\n", len(report.Orphaned))
		for _, b := range report.Orphaned {
			fmt.Printf("   • %s\n", b.Name)
		}
	}

	// Protected branches (info only)
	if len(report.Protected) > 0 {
		fmt.Printf("\n🔒 Protected branches (%d) - will not be deleted:\n", len(report.Protected))
		for _, b := range report.Protected {
			fmt.Printf("   • %s\n", b.Name)
		}
	}

	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Total: %d branch(es) to clean up (analyzed %d)\n", report.CountBranches(), report.Total)
}
