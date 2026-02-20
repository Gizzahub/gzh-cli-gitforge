package cmd

import (
	"bytes"
	"context"
	"encoding/json"
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
	switchFlags  BulkCommandFlags
	switchCreate bool
	switchForce  bool
)

// switchCmd represents the switch command
var switchCmd = &cobra.Command{
	Use:   "switch <branch> [directory]",
	Short: "Switch branches across multiple repositories",
	Long: cliutil.QuickStartHelp(`  # Switch all repos to develop branch
  gz-git switch develop

  # Create branch if it doesn't exist
  gz-git switch feature/new --create

  # Use custom parallelism
  gz-git switch main -j 10

  # Only include specific repos
  gz-git switch develop --include "gzh-cli-.*"

  # Force switch (discards uncommitted changes - DANGEROUS!)
  gz-git switch main --force`),
	Args: cobra.RangeArgs(1, 2),
	RunE: runSwitch,
}

func init() {
	rootCmd.AddCommand(switchCmd)

	// Common bulk operation flags (except watch/interval which don't apply to switch)
	switchCmd.Flags().IntVarP(&switchFlags.Depth, "scan-depth", "d", repository.DefaultBulkMaxDepth, "directory depth to scan for repositories")
	switchCmd.Flags().IntVarP(&switchFlags.Parallel, "parallel", "j", repository.DefaultBulkParallel, "number of parallel operations")
	switchCmd.Flags().BoolVarP(&switchFlags.DryRun, "dry-run", "n", false, "show what would be done without doing it")
	switchCmd.Flags().BoolVarP(&switchFlags.IncludeSubmodules, "recursive", "r", false, "recursively include nested repositories and submodules")
	switchCmd.Flags().StringVar(&switchFlags.Include, "include", "", "regex pattern to include repositories")
	switchCmd.Flags().StringVar(&switchFlags.Exclude, "exclude", "", "regex pattern to exclude repositories")
	switchCmd.Flags().StringVar(&switchFlags.Format, "format", "default", "output format: default, compact, json, llm")

	// Switch-specific flags (no -f shorthand for force to avoid conflict with --format)
	switchCmd.Flags().BoolVarP(&switchCreate, "create", "c", false, "create branch if it doesn't exist")
	switchCmd.Flags().BoolVar(&switchForce, "force", false, "force switch even with uncommitted changes (DANGEROUS!)")
}

func runSwitch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get branch name (required)
	branch := args[0]

	// Get directory (optional, defaults to current)
	directory := "."
	if len(args) > 1 {
		directory = args[1]
	}

	// Validate directory exists
	if _, err := os.Stat(directory); err != nil {
		return fmt.Errorf("directory does not exist: %s", directory)
	}

	// Validate depth
	if err := validateBulkDepth(cmd, switchFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(switchFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkSwitchOptions{
		Directory:         directory,
		Branch:            branch,
		Parallel:          switchFlags.Parallel,
		MaxDepth:          switchFlags.Depth,
		DryRun:            switchFlags.DryRun,
		Verbose:           verbose,
		Create:            switchCreate,
		Force:             switchForce,
		IncludeSubmodules: switchFlags.IncludeSubmodules,
		IncludePattern:    switchFlags.Include,
		ExcludePattern:    switchFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Switching", switchFlags.Format, quiet),
	}

	// Print header
	if !quiet {
		printScanningMessage(directory, switchFlags.Depth, switchFlags.Parallel, switchFlags.DryRun)
	}

	// Execute bulk switch
	result, err := client.BulkSwitch(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk switch failed: %w", err)
	}

	// Display results (always output for JSON format, otherwise respect quiet flag)
	if switchFlags.Format == "json" || !quiet {
		displaySwitchResults(result, switchFlags.Format)
	}

	// Return error if there were any failures
	if result.Summary[repository.StatusError] > 0 {
		return fmt.Errorf("%d repositories failed to switch", result.Summary[repository.StatusError])
	}

	return nil
}

// displaySwitchResults displays the results of a bulk switch operation
func displaySwitchResults(result *repository.BulkSwitchResult, format string) {
	// JSON output mode
	if format == "json" {
		displaySwitchResultsJSON(result)
		return
	}

	// LLM output mode
	if format == "llm" {
		displaySwitchResultsLLM(result)
		return
	}

	// Compact mode: unchanged
	if format == "compact" {
		fmt.Println()
		fmt.Printf("Target branch: %s\n", result.TargetBranch)
		fmt.Printf("Scanned: %d repositories\n", result.TotalScanned)
		fmt.Printf("Processed: %d repositories\n", result.TotalProcessed)
		fmt.Println()
		for _, repo := range result.Repositories {
			if repo.Status != repository.StatusSwitched &&
				repo.Status != repository.StatusAlreadyOnBranch &&
				repo.Status != repository.StatusBranchCreated {
				displaySwitchRepoResult(repo)
			}
		}
		fmt.Println()
		displaySwitchSummary(result)
		fmt.Printf("Duration: %s\n", result.Duration.Round(time.Millisecond))
		return
	}

	fmt.Println()

	if verbose {
		// Verbose: full detailed output (old default behavior)
		fmt.Printf("Target branch: %s\n", result.TargetBranch)
		fmt.Printf("Scanned: %d repositories\n", result.TotalScanned)
		fmt.Printf("Processed: %d repositories\n", result.TotalProcessed)
		fmt.Println()
		for _, repo := range result.Repositories {
			displaySwitchRepoResult(repo)
		}
		fmt.Println()
		displaySwitchSummary(result)
		fmt.Printf("Duration: %s\n", result.Duration.Round(time.Millisecond))
	} else {
		// Default: one-line summary + failures only
		type switchIcon struct {
			icon  string
			label string
		}
		switchIcons := map[string]switchIcon{
			repository.StatusSwitched:        {"+", "switched"},
			repository.StatusBranchCreated:   {"+", "created"},
			repository.StatusAlreadyOnBranch: {"=", "already"},
			repository.StatusWouldSwitch:     {"~", "would-switch"},
			repository.StatusDirty:           {"!", "dirty"},
			repository.StatusBranchNotFound:  {"?", "not-found"},
			repository.StatusError:           {"✗", "error"},
		}
		switchOrder := []string{
			repository.StatusSwitched, repository.StatusBranchCreated,
			repository.StatusAlreadyOnBranch, repository.StatusWouldSwitch,
			repository.StatusDirty, repository.StatusBranchNotFound, repository.StatusError,
		}

		var parts []string
		for _, key := range switchOrder {
			count, ok := result.Summary[key]
			if !ok || count == 0 {
				continue
			}
			info := switchIcons[key]
			parts = append(parts, fmt.Sprintf("%s%d %s", info.icon, count, info.label))
		}
		bracket := ""
		if len(parts) > 0 {
			bracket = "  [" + strings.Join(parts, "  ") + "]"
		}
		durationStr := result.Duration.Round(time.Millisecond).String()
		fmt.Printf("Switched → %s: %d repos%s  %s\n", result.TargetBranch, result.TotalProcessed, bracket, durationStr)

		// Show failures only
		for _, repo := range result.Repositories {
			if repo.Status == repository.StatusError ||
				repo.Status == repository.StatusDirty ||
				repo.Status == repository.StatusBranchNotFound {
				displaySwitchRepoResult(repo)
			}
		}
	}
}

// displaySwitchRepoResult displays a single repository switch result
func displaySwitchRepoResult(repo repository.RepositorySwitchResult) {
	var icon string
	switch repo.Status {
	case repository.StatusSwitched:
		icon = "+"
	case repository.StatusBranchCreated:
		icon = "+"
	case repository.StatusAlreadyOnBranch:
		icon = "="
	case repository.StatusWouldSwitch:
		icon = "~"
	case repository.StatusDirty:
		icon = "!"
	case repository.StatusBranchNotFound:
		icon = "?"
	case repository.StatusRebaseInProgress, repository.StatusMergeInProgress:
		icon = "!"
	default:
		icon = "x"
	}

	fmt.Printf("[%s] %-40s %s\n", icon, repo.RelativePath, repo.Message)
}

// displaySwitchSummary displays the summary of bulk switch results
func displaySwitchSummary(result *repository.BulkSwitchResult) {
	fmt.Print("Summary: ")

	parts := []string{}

	if count := result.Summary[repository.StatusSwitched]; count > 0 {
		parts = append(parts, fmt.Sprintf("%d switched", count))
	}
	if count := result.Summary[repository.StatusBranchCreated]; count > 0 {
		parts = append(parts, fmt.Sprintf("%d created", count))
	}
	if count := result.Summary[repository.StatusAlreadyOnBranch]; count > 0 {
		parts = append(parts, fmt.Sprintf("%d already", count))
	}
	if count := result.Summary[repository.StatusWouldSwitch]; count > 0 {
		parts = append(parts, fmt.Sprintf("%d would-switch", count))
	}
	if count := result.Summary[repository.StatusDirty]; count > 0 {
		parts = append(parts, fmt.Sprintf("%d dirty", count))
	}
	if count := result.Summary[repository.StatusBranchNotFound]; count > 0 {
		parts = append(parts, fmt.Sprintf("%d not-found", count))
	}
	if count := result.Summary[repository.StatusError]; count > 0 {
		parts = append(parts, fmt.Sprintf("%d errors", count))
	}

	if len(parts) == 0 {
		fmt.Println("no changes")
	} else {
		for i, part := range parts {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(part)
		}
		fmt.Println()
	}
}

// SwitchJSONOutput represents the JSON output structure for switch command
type SwitchJSONOutput struct {
	TargetBranch   string                       `json:"target_branch"`
	TotalScanned   int                          `json:"total_scanned"`
	TotalProcessed int                          `json:"total_processed"`
	DurationMs     int64                        `json:"duration_ms"`
	Summary        map[string]int               `json:"summary"`
	Repositories   []SwitchRepositoryJSONOutput `json:"repositories"`
}

// SwitchRepositoryJSONOutput represents a single repository in JSON output
type SwitchRepositoryJSONOutput struct {
	Path           string `json:"path"`
	Status         string `json:"status"`
	PreviousBranch string `json:"previous_branch,omitempty"`
	CurrentBranch  string `json:"current_branch,omitempty"`
	Message        string `json:"message,omitempty"`
	DurationMs     int64  `json:"duration_ms,omitempty"`
}

func displaySwitchResultsJSON(result *repository.BulkSwitchResult) {
	// Convert summary keys to strings
	summary := make(map[string]int)
	for k, v := range result.Summary {
		summary[string(k)] = v
	}

	output := SwitchJSONOutput{
		TargetBranch:   result.TargetBranch,
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        summary,
		Repositories:   make([]SwitchRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := SwitchRepositoryJSONOutput{
			Path:           repo.RelativePath,
			Status:         string(repo.Status),
			PreviousBranch: repo.PreviousBranch,
			CurrentBranch:  repo.CurrentBranch,
			Message:        repo.Message,
			DurationMs:     repo.Duration.Milliseconds(),
		}
		output.Repositories = append(output.Repositories, repoOutput)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

func displaySwitchResultsLLM(result *repository.BulkSwitchResult) {
	// Convert summary keys to strings
	summary := make(map[string]int)
	for k, v := range result.Summary {
		summary[string(k)] = v
	}

	output := SwitchJSONOutput{
		TargetBranch:   result.TargetBranch,
		TotalScanned:   result.TotalScanned,
		TotalProcessed: result.TotalProcessed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        summary,
		Repositories:   make([]SwitchRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := SwitchRepositoryJSONOutput{
			Path:           repo.RelativePath,
			Status:         string(repo.Status),
			PreviousBranch: repo.PreviousBranch,
			CurrentBranch:  repo.CurrentBranch,
			Message:        repo.Message,
			DurationMs:     repo.Duration.Milliseconds(),
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
