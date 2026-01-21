package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
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
	cloneConfig       string   // --config flag (YAML file path)
	cloneConfigStdin  bool     // --config-stdin flag (read YAML from stdin)
)

// cloneCmd represents the clone command
var cloneCmd = &cobra.Command{
	Use:   "clone [directory]",
	Short: "Clone multiple repositories in parallel",
	Long: `Clone one or more repositories from remote URLs in parallel.

` + cliutil.QuickStartHelp(`  # 1. Clone multiple URLs to current directory
  gz-git clone --url https://github.com/user/repo1.git --url https://github.com/user/repo2.git

  # 2. Clone from a file containing URLs
  gz-git clone --file repos.txt

  # 3. Clone with user directory structure (user/repo)
  gz-git clone --structure user --url ...

See 'gcl' alias for single repository cloning.`),
	Example: "",
	Args:    cobra.MaximumNArgs(1),
	RunE:    runClone,
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
	cloneCmd.Flags().StringVarP(&cloneConfig, "config", "c", "", "YAML config file for clone specifications")
	cloneCmd.Flags().BoolVar(&cloneConfigStdin, "config-stdin", false, "read YAML config from stdin")
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

	// Check for YAML config mode
	if cloneConfig != "" || cloneConfigStdin {
		return runCloneFromConfig(ctx, directory)
	}

	// Collect URLs from --url flag and --file
	urls, err := collectCloneURLs(cloneURLs, cloneFile)
	if err != nil {
		return err
	}

	if len(urls) == 0 {
		return fmt.Errorf("no repository URLs provided. Use --url, --file, or --config")
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

// ============================================================================
// YAML Config Support
// ============================================================================

// CloneConfig represents the YAML configuration for bulk clone with custom names.
type CloneConfig struct {
	// Target directory for all repositories (optional, can be overridden by CLI arg)
	Target string `yaml:"target,omitempty"`

	// Global settings (can be overridden by CLI flags)
	Parallel  int    `yaml:"parallel,omitempty"`
	Structure string `yaml:"structure,omitempty"` // "flat" or "user"
	Update    bool   `yaml:"update,omitempty"`

	// Repository list
	Repositories []CloneRepoSpec `yaml:"repositories"`
}

// CloneRepoSpec represents a single repository specification in YAML.
type CloneRepoSpec struct {
	URL    string `yaml:"url"`              // Required
	Name   string `yaml:"name,omitempty"`   // Optional: custom directory name
	Branch string `yaml:"branch,omitempty"` // Optional: branch to checkout
	Depth  int    `yaml:"depth,omitempty"`  // Optional: shallow clone depth
}

// parseCloneConfig reads and parses YAML config from file or stdin.
func parseCloneConfig(configPath string, useStdin bool) (*CloneConfig, error) {
	var data []byte
	var err error

	if useStdin {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("read config file %s: %w", configPath, err)
		}
	}

	var config CloneConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	// Validate
	if err := validateCloneConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validateCloneConfig validates the parsed YAML config.
func validateCloneConfig(config *CloneConfig) error {
	if len(config.Repositories) == 0 {
		return fmt.Errorf("no repositories defined in config")
	}

	// Validate structure
	if config.Structure != "" {
		if config.Structure != "flat" && config.Structure != "user" {
			return fmt.Errorf("invalid structure %q: must be 'flat' or 'user'", config.Structure)
		}
	}

	// Check for duplicate names
	namesSeen := make(map[string]bool)
	for i, repo := range config.Repositories {
		if repo.URL == "" {
			return fmt.Errorf("repository[%d]: missing URL", i)
		}

		// Extract name from URL if not specified
		name := repo.Name
		if name == "" {
			extracted, err := repository.ExtractRepoNameFromURL(repo.URL)
			if err != nil {
				return fmt.Errorf("repository[%d]: cannot extract name from URL %q: %w", i, repo.URL, err)
			}
			name = extracted
		}

		if namesSeen[name] {
			return fmt.Errorf("repository[%d]: duplicate name %q", i, name)
		}
		namesSeen[name] = true
	}

	return nil
}

// buildCloneOptionsFromConfig builds BulkCloneOptions from YAML config.
// CLI flags take precedence over YAML config.
func buildCloneOptionsFromConfig(
	config *CloneConfig,
	directory string,
	flags BulkCommandFlags,
	branch string,
	depth int,
	update bool,
	structure string,
	logger repository.Logger,
) repository.BulkCloneOptions {
	// Use CLI argument if provided, otherwise YAML config, otherwise default
	targetDir := directory
	if targetDir == "." && config.Target != "" {
		targetDir = config.Target
	}

	// Use CLI flags if set, otherwise YAML config, otherwise defaults
	parallel := flags.Parallel
	if parallel == 0 && config.Parallel > 0 {
		parallel = config.Parallel
	}
	if parallel == 0 {
		parallel = repository.DefaultBulkParallel
	}

	structureVal := structure
	if structureVal == "flat" && config.Structure != "" {
		structureVal = config.Structure
	}

	updateVal := update || config.Update

	// Build BulkCloneOptions with custom per-repo settings
	opts := repository.BulkCloneOptions{
		Directory: targetDir,
		Structure: repository.DirectoryStructure(structureVal),
		Update:    updateVal,
		Branch:    branch,
		Depth:     depth,
		Parallel:  parallel,
		DryRun:    flags.DryRun,
		Verbose:   verbose,
		Logger:    logger,
		ProgressCallback: func(current, total int, url string) {
			if shouldShowProgress(flags.Format, quiet) {
				repoName, _ := repository.ExtractRepoNameFromURL(url)
				if repoName == "" {
					repoName = "repository"
				}
				fmt.Printf("[%d/%d] Cloning %s...\n", current, total, repoName)
			}
		},
	}

	return opts
}

// runCloneFromConfig executes clone operation based on YAML config.
func runCloneFromConfig(ctx context.Context, directory string) error {
	// Parse YAML config
	config, err := parseCloneConfig(cloneConfig, cloneConfigStdin)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Validate format
	if err := validateBulkFormat(cloneFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build base options from config
	baseOpts := buildCloneOptionsFromConfig(
		config,
		directory,
		cloneFlags,
		cloneBranch,
		cloneDepth,
		cloneUpdate,
		cloneStructure,
		logger,
	)

	// Clone each repository with custom settings
	totalRepos := len(config.Repositories)

	if shouldShowProgress(cloneFlags.Format, quiet) {
		suffix := ""
		if cloneFlags.DryRun {
			suffix = " [DRY-RUN]"
		}
		updateStr := ""
		if baseOpts.Update {
			updateStr = ", update existing"
		}
		dirStr := ""
		if baseOpts.Directory != "." {
			dirStr = fmt.Sprintf(" to %s", baseOpts.Directory)
		}
		fmt.Printf("Cloning %d repositories%s (parallel: %d, structure: %s%s)%s\n",
			totalRepos, dirStr, baseOpts.Parallel, baseOpts.Structure, updateStr, suffix)
	}

	// Build custom clone operations with parallel execution
	results := &repository.BulkCloneResult{
		TotalRequested: totalRepos,
		Summary:        make(map[string]int),
		Repositories:   make([]repository.RepositoryCloneResult, 0, totalRepos),
	}

	startTime := time.Now()

	// Create work channel and results channel
	type workItem struct {
		index    int
		repoSpec CloneRepoSpec
		repoName string
	}

	workChan := make(chan workItem, totalRepos)
	resultsChan := make(chan repository.RepositoryCloneResult, totalRepos)

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < baseOpts.Parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workChan {
				// Show progress
				if baseOpts.ProgressCallback != nil {
					baseOpts.ProgressCallback(work.index+1, totalRepos, work.repoSpec.URL)
				}

				// Clone single repository
				result := cloneSingleRepository(
					ctx,
					client,
					work.repoSpec,
					work.repoName,
					baseOpts,
				)

				resultsChan <- result
			}
		}()
	}

	// Send work items
	go func() {
		for i, repoSpec := range config.Repositories {
			// Determine repository name
			repoName := repoSpec.Name
			if repoName == "" {
				extracted, _ := repository.ExtractRepoNameFromURL(repoSpec.URL)
				repoName = extracted
			}

			workChan <- workItem{
				index:    i,
				repoSpec: repoSpec,
				repoName: repoName,
			}
		}
		close(workChan)
	}()

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for result := range resultsChan {
		results.Repositories = append(results.Repositories, result)

		// Update counters
		switch result.Status {
		case "cloned":
			results.TotalCloned++
		case "updated", "pulled", "rebased":
			results.TotalUpdated++
		case "skipped":
			results.TotalSkipped++
		case "error":
			results.TotalFailed++
		}

		// Update summary
		if results.Summary == nil {
			results.Summary = make(map[string]int)
		}
		results.Summary[result.Status]++
	}

	results.Duration = time.Since(startTime)

	// Display results
	displayCloneResults(results)

	return nil
}

// cloneSingleRepository clones a single repository with custom settings.
func cloneSingleRepository(
	ctx context.Context,
	client repository.Client,
	spec CloneRepoSpec,
	repoName string,
	baseOpts repository.BulkCloneOptions,
) repository.RepositoryCloneResult {
	startTime := time.Now()

	// Build destination path
	destination := filepath.Join(baseOpts.Directory, repoName)

	// Determine branch
	branch := spec.Branch
	if branch == "" && baseOpts.Branch != "" {
		branch = baseOpts.Branch
	}

	// Determine depth
	depth := spec.Depth
	if depth == 0 && baseOpts.Depth > 0 {
		depth = baseOpts.Depth
	}

	// Check if target already exists
	exists, isGitRepo := false, false
	if fi, err := os.Stat(destination); err == nil {
		exists = true
		if fi.IsDir() {
			gitDir := filepath.Join(destination, ".git")
			if _, err := os.Stat(gitDir); err == nil {
				isGitRepo = true
			}
		}
	}

	// Dry run mode
	if baseOpts.DryRun {
		status := "would-clone"
		if exists && isGitRepo && baseOpts.Update {
			status = "would-update"
		} else if exists && isGitRepo {
			status = "skipped"
		}

		return repository.RepositoryCloneResult{
			URL:          spec.URL,
			Path:         destination,
			RelativePath: repoName,
			Branch:       branch,
			Status:       status,
			Duration:     time.Since(startTime),
		}
	}

	// Skip if exists and not updating
	if exists && isGitRepo && !baseOpts.Update {
		return repository.RepositoryCloneResult{
			URL:          spec.URL,
			Path:         destination,
			RelativePath: repoName,
			Branch:       branch,
			Status:       "skipped",
			Duration:     time.Since(startTime),
		}
	}

	// Build clone options
	cloneOpts := repository.CloneOptions{
		URL:          spec.URL,
		Destination:  destination,
		Branch:       branch,
		Depth:        depth,
		SingleBranch: cloneSingleBranch,
		Recursive:    cloneSubmodules,
	}

	// Clone or update
	var err error
	var status string

	if exists && isGitRepo && baseOpts.Update {
		// Update existing repository using git pull
		pullCmd := exec.CommandContext(ctx, "git", "-C", destination, "pull", "--rebase")
		output, pullErr := pullCmd.CombinedOutput()

		if pullErr != nil {
			err = fmt.Errorf("git pull failed: %w (output: %s)", pullErr, string(output))
			status = "error"
		} else {
			// Check if there were any changes
			if strings.Contains(string(output), "Already up to date") {
				status = "up-to-date"
			} else {
				status = "updated"
			}
		}
	} else {
		// Clone new repository
		_, err = client.Clone(ctx, cloneOpts)
		if err != nil {
			status = "error"
		} else {
			status = "cloned"
		}
	}

	result := repository.RepositoryCloneResult{
		URL:          spec.URL,
		Path:         destination,
		RelativePath: repoName,
		Branch:       branch,
		Status:       status,
		Duration:     time.Since(startTime),
	}

	if err != nil {
		result.Error = err
	}

	return result
}
