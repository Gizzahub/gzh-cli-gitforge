package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	cloneFlags        BulkCommandFlags
	cloneBranch       string
	cloneDepth        int
	cloneUpdate       bool
	cloneStructure    string
	cloneFile         string
	cloneSingleBranch bool
	cloneSubmodules   bool
	cloneURLs         []string // --url flag (repeatable)
)

// cloneCmd represents the clone command
var cloneCmd = &cobra.Command{
	Use:   "clone [directory]",
	Short: "Clone multiple repositories in parallel",
	Long: `Clone one or more repositories from remote URLs in parallel.

This command is optimized for bulk cloning. For single repository cloning,
consider using 'gcl' alias which is simpler.

Supported URL formats:
  - HTTPS: https://github.com/user/repo.git
  - SSH: git@github.com:user/repo.git

Directory structure options:
  - flat: All repos in target directory (default)
          github.com/user/repo → ./repo/
  - user: Organized by user/org name
          github.com/user/repo → ./user/repo/

When --update is specified, existing repositories are pulled instead of skipped.`,
	Example: `  # Clone multiple repositories to current directory
  gz-git clone --url https://github.com/user/repo1.git --url https://github.com/user/repo2.git

  # Clone to specific directory
  gz-git clone ~/projects --url url1 --url url2

  # Clone from a file containing URLs (one per line)
  gz-git clone --file repos.txt
  gz-git clone ~/projects --file repos.txt

  # Clone with user directory structure
  gz-git clone --structure user --url url1 --url url2

  # Clone and update existing repos (pull if exists)
  gz-git clone --update --url url1 --url url2

  # Dry run to see what would be done
  gz-git clone --dry-run --url url1 --url url2

  # Clone with specific branch
  gz-git clone -b develop --url url1 --url url2

  # Clone in parallel with custom workers
  gz-git clone -j 10 --url url1 --url url2 --url url3

  # JSON output for scripting
  gz-git clone --format json --url url1 --url url2`,
	Args: cobra.MaximumNArgs(1),
	RunE: runClone,
}

func init() {
	rootCmd.AddCommand(cloneCmd)

	// Bulk operation flags
	addBulkFlags(cloneCmd, &cloneFlags)

	// Clone-specific flags
	cloneCmd.Flags().StringArrayVar(&cloneURLs, "url", nil, "repository URL to clone (can be repeated)")
	cloneCmd.Flags().StringVarP(&cloneBranch, "branch", "b", "", "checkout specific branch")
	cloneCmd.Flags().IntVar(&cloneDepth, "depth", 0, "create a shallow clone with truncated history")
	cloneCmd.Flags().BoolVar(&cloneUpdate, "update", false, "pull existing repositories instead of skipping")
	cloneCmd.Flags().StringVar(&cloneStructure, "structure", "flat", "directory structure: flat or user")
	cloneCmd.Flags().StringVar(&cloneFile, "file", "", "file containing repository URLs (one per line)")
	cloneCmd.Flags().BoolVar(&cloneSingleBranch, "single-branch", false, "clone only one branch")
	cloneCmd.Flags().BoolVar(&cloneSubmodules, "submodules", false, "initialize submodules in the clone")
}

func runClone(cmd *cobra.Command, args []string) error {
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

	// Parse directory from positional argument (consistent with other bulk commands)
	directory := "."
	if len(args) > 0 {
		directory = args[0]
		// Validate directory exists (if not current dir)
		if directory != "." {
			if _, err := os.Stat(directory); os.IsNotExist(err) {
				return fmt.Errorf("directory does not exist: %s", directory)
			}
		}
	}

	// Collect URLs from --url flag and --file
	urls, err := collectCloneURLs(cloneURLs, cloneFile)
	if err != nil {
		return err
	}

	if len(urls) == 0 {
		return fmt.Errorf("no repository URLs provided. Use --url or --file")
	}

	// Validate structure
	structure := repository.DirectoryStructure(cloneStructure)
	if structure != repository.StructureFlat && structure != repository.StructureUser {
		return fmt.Errorf("invalid structure %q: must be 'flat' or 'user'", cloneStructure)
	}

	// Validate format
	if err := validateBulkFormat(cloneFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkCloneOptions{
		URLs:      urls,
		Directory: directory,
		Structure: structure,
		Update:    cloneUpdate,
		Branch:    cloneBranch,
		Depth:     cloneDepth,
		Parallel:  cloneFlags.Parallel,
		DryRun:    cloneFlags.DryRun,
		Verbose:   verbose,
		Logger:    logger,
		ProgressCallback: func(current, total int, url string) {
			if shouldShowProgress(cloneFlags.Format, quiet) {
				repoName, _ := repository.ExtractRepoNameFromURL(url)
				if repoName == "" {
					repoName = "repository"
				}
				fmt.Printf("[%d/%d] Cloning %s...\n", current, total, repoName)
			}
		},
	}

	// Show scanning message
	if shouldShowProgress(cloneFlags.Format, quiet) {
		suffix := ""
		if cloneFlags.DryRun {
			suffix = " [DRY-RUN]"
		}
		updateStr := ""
		if cloneUpdate {
			updateStr = ", update existing"
		}
		dirStr := ""
		if directory != "." {
			dirStr = fmt.Sprintf(" to %s", directory)
		}
		fmt.Printf("Cloning %d repositories%s (parallel: %d, structure: %s%s)%s\n",
			len(urls), dirStr, cloneFlags.Parallel, cloneStructure, updateStr, suffix)
	}

	// Execute bulk clone
	result, err := client.BulkClone(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk clone failed: %w", err)
	}

	// Display results
	displayCloneResults(result)

	return nil
}

// collectCloneURLs collects URLs from --url flags and --file.
func collectCloneURLs(urlFlags []string, filePath string) ([]string, error) {
	urls := make([]string, 0)

	// Add URLs from --url flags
	for _, url := range urlFlags {
		url = strings.TrimSpace(url)
		if url != "" && !strings.HasPrefix(url, "#") {
			urls = append(urls, url)
		}
	}

	// Add URLs from file
	if filePath != "" {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("cannot open file %s: %w", filePath, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// Skip empty lines and comments
			if line != "" && !strings.HasPrefix(line, "#") {
				urls = append(urls, line)
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
		}
	}

	return urls, nil
}

func displayCloneResults(result *repository.BulkCloneResult) {
	// JSON output mode
	if cloneFlags.Format == "json" {
		displayCloneResultsJSON(result)
		return
	}

	fmt.Println()
	fmt.Println("=== Bulk Clone Results ===")
	fmt.Printf("Total requested: %d repositories\n", result.TotalRequested)
	fmt.Printf("Total cloned:    %d repositories\n", result.TotalCloned)
	fmt.Printf("Total updated:   %d repositories\n", result.TotalUpdated)
	fmt.Printf("Total skipped:   %d repositories\n", result.TotalSkipped)
	fmt.Printf("Total failed:    %d repositories\n", result.TotalFailed)
	fmt.Printf("Duration:        %s\n", result.Duration.Round(100_000_000))
	fmt.Println()

	// Display summary
	if len(result.Summary) > 0 {
		fmt.Println("Summary by status:")
		for status, count := range result.Summary {
			icon := getCloneStatusIcon(status)
			fmt.Printf("  %s %-15s %d\n", icon, status+":", count)
		}
		fmt.Println()
	}

	// Display individual results if not compact
	if cloneFlags.Format != "compact" && len(result.Repositories) > 0 {
		fmt.Println("Repository details:")
		for _, repo := range result.Repositories {
			displayCloneRepositoryResult(repo)
		}
	}
}

func displayCloneRepositoryResult(repo repository.RepositoryCloneResult) {
	icon := getCloneStatusIcon(repo.Status)

	// Build output line
	pathPart := repo.RelativePath
	if repo.Branch != "" {
		pathPart += fmt.Sprintf(" (%s)", repo.Branch)
	}

	statusStr := repo.Status
	if repo.Error != nil && verbose {
		statusStr = fmt.Sprintf("%s: %v", repo.Status, repo.Error)
	}

	durationStr := ""
	if repo.Duration > 0 {
		durationStr = fmt.Sprintf(" %s", repo.Duration.Round(10_000_000))
	}

	fmt.Printf("  %s %-50s %-15s%s\n", icon, pathPart, statusStr, durationStr)
}

func getCloneStatusIcon(status string) string {
	switch status {
	case "cloned":
		return "✓"
	case "updated", "pulled", "rebased":
		return "↓"
	case "skipped":
		return "⊘"
	case "would-clone", "would-update":
		return "→"
	case "error":
		return "✗"
	default:
		return "•"
	}
}

// CloneJSONOutput represents the JSON output structure for clone command.
type CloneJSONOutput struct {
	TotalRequested int                       `json:"total_requested"`
	TotalCloned    int                       `json:"total_cloned"`
	TotalUpdated   int                       `json:"total_updated"`
	TotalSkipped   int                       `json:"total_skipped"`
	TotalFailed    int                       `json:"total_failed"`
	DurationMs     int64                     `json:"duration_ms"`
	Summary        map[string]int            `json:"summary"`
	Repositories   []CloneRepositoryJSONItem `json:"repositories"`
}

// CloneRepositoryJSONItem represents a single repository in JSON output.
type CloneRepositoryJSONItem struct {
	URL        string `json:"url"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	Branch     string `json:"branch,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
}

func displayCloneResultsJSON(result *repository.BulkCloneResult) {
	output := CloneJSONOutput{
		TotalRequested: result.TotalRequested,
		TotalCloned:    result.TotalCloned,
		TotalUpdated:   result.TotalUpdated,
		TotalSkipped:   result.TotalSkipped,
		TotalFailed:    result.TotalFailed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]CloneRepositoryJSONItem, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		item := CloneRepositoryJSONItem{
			URL:        repo.URL,
			Path:       repo.RelativePath,
			Status:     repo.Status,
			Branch:     repo.Branch,
			DurationMs: repo.Duration.Milliseconds(),
		}
		if repo.Error != nil {
			item.Error = repo.Error.Error()
		}
		output.Repositories = append(output.Repositories, item)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

