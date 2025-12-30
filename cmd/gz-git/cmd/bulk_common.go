package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// BulkCommandFlags holds common flags for bulk operations (fetch, pull, push)
type BulkCommandFlags struct {
	Depth             int
	Parallel          int
	DryRun            bool
	IncludeSubmodules bool
	Include           string
	Exclude           string
	Format            string
	Watch             bool
	Interval          time.Duration
}

// addBulkFlags registers common bulk operation flags to a command
func addBulkFlags(cmd *cobra.Command, flags *BulkCommandFlags) {
	cmd.Flags().IntVarP(&flags.Depth, "scan-depth", "d", repository.DefaultBulkMaxDepth, "directory depth to scan for repositories")
	cmd.Flags().IntVarP(&flags.Parallel, "parallel", "j", repository.DefaultBulkParallel, "number of parallel operations")
	cmd.Flags().BoolVarP(&flags.DryRun, "dry-run", "n", false, "show what would be done without doing it")
	cmd.Flags().BoolVarP(&flags.IncludeSubmodules, "recursive", "r", false, "recursively include nested repositories and submodules")
	cmd.Flags().StringVar(&flags.Include, "include", "", "regex pattern to include repositories")
	cmd.Flags().StringVar(&flags.Exclude, "exclude", "", "regex pattern to exclude repositories")
	cmd.Flags().StringVarP(&flags.Format, "format", "f", "default", "output format: default, compact, json, llm")
	cmd.Flags().BoolVar(&flags.Watch, "watch", false, "continuously run at intervals")
	cmd.Flags().DurationVar(&flags.Interval, "interval", 5*time.Minute, "interval when watching")
}

// validateBulkDirectory parses and validates the directory argument
// Returns the directory path (defaults to ".") or an error
func validateBulkDirectory(args []string) (string, error) {
	directory := "."
	if len(args) > 0 {
		directory = args[0]
	}

	// Validate directory exists
	if _, err := os.Stat(directory); err != nil {
		return "", fmt.Errorf("directory does not exist: %s", directory)
	}

	return directory, nil
}

// validateBulkDepth validates the scan-depth flag
// Returns an error if depth is explicitly set to 0
func validateBulkDepth(cmd *cobra.Command, depth int) error {
	if cmd.Flags().Changed("scan-depth") && depth == 0 {
		return fmt.Errorf("scan-depth must be at least 1 (use --scan-depth 1 to scan current directory and immediate subdirectories)")
	}
	return nil
}

// ValidBulkFormats contains the list of valid output formats
var ValidBulkFormats = []string{"default", "compact", "json", "llm"}

// validateBulkFormat validates the format flag
// Returns an error if the format is not supported
func validateBulkFormat(format string) error {
	for _, valid := range ValidBulkFormats {
		if format == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid format %q: must be one of: default, compact, json, llm", format)
}

// createBulkLogger creates a logger for bulk operations
// Returns a logger if verbose mode is enabled, nil otherwise
func createBulkLogger(verbose bool) repository.Logger {
	if verbose {
		return repository.NewWriterLogger(os.Stdout)
	}
	return nil
}

// createProgressCallback creates a progress callback function for bulk operations
// The callback is used to display progress during bulk operations
func createProgressCallback(operationName string, format string, quiet bool) func(int, int, string) {
	return func(current, total int, repo string) {
		if !quiet && format != "compact" && format != "json" {
			fmt.Printf("[%d/%d] %s %s...\n", current, total, operationName, repo)
		}
	}
}

// shouldShowProgress returns true if progress messages should be displayed
func shouldShowProgress(format string, quiet bool) bool {
	return !quiet && format != "json"
}

// getPushStatusIconWithContext returns the appropriate icon based on status and actual changes.
// Icons: ✓ (changes pushed), = (no changes), ✗ (error), ⚠ (warning), ⊘ (skipped)
func getPushStatusIconWithContext(status string, pushedCommits int) string {
	switch status {
	case "success", "pushed":
		// Only show ✓ if actual changes were pushed
		if pushedCommits > 0 {
			return "✓"
		}
		return "=" // No changes = up-to-date
	case "nothing-to-push", "up-to-date":
		return "="
	case "error":
		return "✗"
	case "conflict":
		return "⚡"
	case "rebase-in-progress":
		return "↻"
	case "merge-in-progress":
		return "⇄"
	case "skipped":
		return "⊘"
	case "would-push":
		return "→"
	case "no-remote":
		return "⚠"
	case "no-upstream":
		return "⚠"
	default:
		return "•"
	}
}

// getPushStatusIcon returns the icon for a status (deprecated: use getPushStatusIconWithContext).
func getPushStatusIcon(status string) string {
	return getPushStatusIconWithContext(status, 0)
}
