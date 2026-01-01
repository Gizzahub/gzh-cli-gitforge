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
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// cleanupBulkFlags holds bulk-specific flags
var cleanupBulkFlags BulkCommandFlags

var (
	cleanupMerged     bool
	cleanupStale      bool
	cleanupGone       bool
	cleanupStaleDays  int
	cleanupDryRun     bool
	cleanupForce      bool
	cleanupRemote     bool
	cleanupProtect    string
	cleanupBaseBranch string
)

// cleanupCmd represents the branch cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup [directory]",
	Short: "Clean up merged, stale, or gone branches",
	Long: `Analyze and clean up branches that are no longer needed.

This command identifies branches that can be safely deleted:
  - Merged branches: fully merged into the base branch
  - Stale branches: no commits for a specified number of days
  - Gone branches: remote tracking branches where the remote branch was deleted

MODES:
  - Single repo: Run without directory argument in a git repository
  - Bulk mode: Provide a directory to scan for multiple repositories

By default, runs in dry-run mode to preview what would be deleted.
Use --force to actually delete branches.

Protected branches (main, master, develop, release/*, hotfix/*) are never deleted
unless explicitly overridden.`,
	Example: `  # Preview merged branches in current repo
  gz-git branch cleanup --merged

  # Preview stale branches (no activity for 30 days)
  gz-git branch cleanup --stale

  # Preview all cleanup-eligible branches
  gz-git branch cleanup --merged --stale --gone

  # Actually delete merged branches
  gz-git branch cleanup --merged --force

  # BULK MODE: Clean up merged branches across all repos
  gz-git branch cleanup . --merged --force

  # BULK MODE: Preview cleanup with parallel workers
  gz-git branch cleanup ~/projects --merged --stale -j 10

  # Protect additional branches
  gz-git branch cleanup --merged --protect "staging,qa" --force`,
	RunE: runBranchCleanup,
}

func init() {
	branchCmd.AddCommand(cleanupCmd)

	// Cleanup-specific flags
	cleanupCmd.Flags().BoolVar(&cleanupMerged, "merged", false, "clean up fully merged branches")
	cleanupCmd.Flags().BoolVar(&cleanupStale, "stale", false, "clean up stale branches (no recent activity)")
	cleanupCmd.Flags().BoolVar(&cleanupGone, "gone", false, "clean up gone branches (remote deleted)")
	cleanupCmd.Flags().IntVar(&cleanupStaleDays, "stale-days", 30, "days threshold for stale branches")
	cleanupCmd.Flags().BoolVarP(&cleanupDryRun, "dry-run", "n", true, "preview changes without deleting (default: true)")
	cleanupCmd.Flags().BoolVarP(&cleanupForce, "force", "f", false, "actually delete branches (disables dry-run)")
	cleanupCmd.Flags().BoolVarP(&cleanupRemote, "remote", "r", false, "also delete remote branches")
	cleanupCmd.Flags().StringVar(&cleanupProtect, "protect", "", "additional branches to protect (comma-separated)")
	cleanupCmd.Flags().StringVar(&cleanupBaseBranch, "base", "", "base branch for merge detection (default: auto-detect)")

	// Bulk operation flags (manually added to avoid dry-run conflict)
	cleanupCmd.Flags().IntVarP(&cleanupBulkFlags.Depth, "scan-depth", "d", repository.DefaultBulkMaxDepth, "directory depth to scan for repositories")
	cleanupCmd.Flags().IntVarP(&cleanupBulkFlags.Parallel, "parallel", "j", repository.DefaultBulkParallel, "number of parallel operations")
	cleanupCmd.Flags().BoolVar(&cleanupBulkFlags.IncludeSubmodules, "recursive", false, "recursively include nested repositories and submodules")
	cleanupCmd.Flags().StringVar(&cleanupBulkFlags.Include, "include", "", "regex pattern to include repositories")
	cleanupCmd.Flags().StringVar(&cleanupBulkFlags.Exclude, "exclude", "", "regex pattern to exclude repositories")
}

func runBranchCleanup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Require at least one cleanup type
	if !cleanupMerged && !cleanupStale && !cleanupGone {
		return fmt.Errorf("specify at least one cleanup type: --merged, --stale, or --gone")
	}

	// Force flag disables dry-run
	if cleanupForce {
		cleanupDryRun = false
	}

	// Build exclude list
	excludePatterns := []string{}
	if cleanupProtect != "" {
		for _, p := range strings.Split(cleanupProtect, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				excludePatterns = append(excludePatterns, p)
			}
		}
	}

	// If directory argument provided, run in bulk mode
	if len(args) > 0 {
		return runBulkCleanup(ctx, args[0], excludePatterns)
	}

	// Single repository mode
	return runSingleRepoCleanup(ctx, excludePatterns)
}

// runSingleRepoCleanup performs cleanup on a single repository
func runSingleRepoCleanup(ctx context.Context, excludePatterns []string) error {
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
		IncludeMerged:  cleanupMerged,
		IncludeStale:   cleanupStale,
		StaleThreshold: time.Duration(cleanupStaleDays) * 24 * time.Hour,
		IncludeRemote:  cleanupRemote,
		Exclude:        excludePatterns,
		BaseBranch:     cleanupBaseBranch,
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
		printCleanupReport(report, cleanupDryRun)
	}

	// If no branches to clean up, exit
	if report.IsEmpty() {
		if !quiet {
			fmt.Println("\n✓ No branches to clean up")
		}
		return nil
	}

	// Execute cleanup if not dry-run
	if !cleanupDryRun {
		executeOpts := branch.ExecuteOptions{
			DryRun:  false,
			Force:   true, // We already confirmed with --force flag
			Remote:  cleanupRemote,
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

// runBulkCleanup performs cleanup across multiple repositories
func runBulkCleanup(ctx context.Context, directory string, excludePatterns []string) error {
	client := repository.NewClient()

	opts := repository.BulkCleanupOptions{
		Directory:         directory,
		Parallel:          cleanupBulkFlags.Parallel,
		MaxDepth:          cleanupBulkFlags.Depth,
		DryRun:            cleanupDryRun,
		IncludeMerged:     cleanupMerged,
		IncludeStale:      cleanupStale,
		IncludeGone:       cleanupGone,
		StaleThreshold:    time.Duration(cleanupStaleDays) * 24 * time.Hour,
		BaseBranch:        cleanupBaseBranch,
		DeleteRemote:      cleanupRemote,
		ProtectPatterns:   excludePatterns,
		IncludeSubmodules: cleanupBulkFlags.IncludeSubmodules,
		IncludePattern:    cleanupBulkFlags.Include,
		ExcludePattern:    cleanupBulkFlags.Exclude,
		Logger:            repository.NewNoopLogger(),
	}

	if !quiet {
		modeStr := "[DRY-RUN]"
		if !cleanupDryRun {
			modeStr = "[EXECUTE]"
		}
		fmt.Printf("%s Scanning for repositories in %s...\n", modeStr, directory)
	}

	result, err := client.BulkCleanup(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk cleanup failed: %w", err)
	}

	// Print results
	printBulkCleanupResult(result, cleanupDryRun)

	return nil
}

// printBulkCleanupResult displays bulk cleanup results
func printBulkCleanupResult(result *repository.BulkCleanupResult, dryRun bool) {
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

// printCleanupReport displays the cleanup analysis report.
func printCleanupReport(report *branch.CleanupReport, dryRun bool) {
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
