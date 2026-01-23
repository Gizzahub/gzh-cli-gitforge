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
	cloneStrategy     string // --update-strategy flag (skip, pull, reset, rebase, fetch)
	cloneUpdate       bool   // Deprecated: use --update-strategy instead
	cloneStructure    string
	cloneFile         string
	cloneSingleBranch bool
	cloneSubmodules   bool
	cloneURLs         []string // --url flag (repeatable)
	cloneConfig       string   // --config flag (YAML file path)
	cloneConfigStdin  bool     // --config-stdin flag (read YAML from stdin)
	cloneGroup        []string // --group flag (select specific groups)
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
	cloneCmd.Flags().StringVarP(&cloneStrategy, "update-strategy", "s", "", "existing repo handling: skip (default), pull, reset, rebase, fetch")
	cloneCmd.Flags().StringVar(&cloneStrategy, "strategy", "", "Deprecated: use --update-strategy")
	_ = cloneCmd.Flags().MarkDeprecated("strategy", "use --update-strategy instead")
	cloneCmd.Flags().BoolVar(&cloneUpdate, "update", false, "[DEPRECATED] use --update-strategy=pull instead")
	cloneCmd.Flags().StringVar(&cloneStructure, "structure", "flat", "directory structure: flat or user")
	cloneCmd.Flags().StringVar(&cloneFile, "file", "", "file containing repository URLs (one per line)")
	cloneCmd.Flags().BoolVar(&cloneSingleBranch, "single-branch", false, "clone only one branch")
	cloneCmd.Flags().BoolVar(&cloneSubmodules, "submodules", false, "initialize submodules in the clone")
	cloneCmd.Flags().StringVarP(&cloneConfig, "config", "c", "", "YAML config file for clone specifications")
	cloneCmd.Flags().BoolVar(&cloneConfigStdin, "config-stdin", false, "read YAML config from stdin")
	cloneCmd.Flags().StringArrayVarP(&cloneGroup, "group", "g", nil, "clone only specified groups (can be repeated)")
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

	// Resolve strategy from CLI flags
	strategy := resolveCloneStrategy(cloneStrategy, cloneUpdate, "", false)

	// Build options
	opts := repository.BulkCloneOptions{
		URLs:      urls,
		Directory: directory,
		Structure: structure,
		Strategy:  strategy,
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
		strategyStr := ""
		if strategy != repository.StrategySkip {
			strategyStr = fmt.Sprintf(", strategy: %s", strategy)
		}
		dirStr := ""
		if directory != "." {
			dirStr = fmt.Sprintf(" to %s", directory)
		}
		fmt.Printf("Cloning %d repositories%s (parallel: %d, structure: %s%s)%s\n",
			len(urls), dirStr, cloneFlags.Parallel, cloneStructure, strategyStr, suffix)
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
// Clone Config Types
// ============================================================================

// CloneConfig represents the YAML configuration for bulk clone.
// Supports two formats:
//  1. Flat format: global settings + repositories array
//  2. Grouped format: global settings + named groups with their own targets
type CloneConfig struct {
	// Global settings (can be overridden by CLI flags or group settings)
	Parallel  int    `yaml:"parallel,omitempty"`
	Strategy  string `yaml:"strategy,omitempty"`  // skip, pull, reset, rebase, fetch
	Structure string `yaml:"structure,omitempty"` // flat or user

	// Flat format: single target + repositories list
	Target       string          `yaml:"target,omitempty"`
	Repositories []CloneRepoSpec `yaml:"repositories,omitempty"`

	// Grouped format: named groups (parsed separately due to dynamic keys)
	Groups map[string]*CloneGroup `yaml:"-"`

	// Deprecated: Use Strategy instead
	Update bool `yaml:"update,omitempty"`
}

// CloneGroup represents a named group of repositories with its own target.
type CloneGroup struct {
	Target       string          `yaml:"target"`             // Required: target directory for this group
	Branch       string          `yaml:"branch,omitempty"`   // Default branch for all repos in group
	Depth        int             `yaml:"depth,omitempty"`    // Default depth for all repos in group
	Strategy     string          `yaml:"strategy,omitempty"` // Override global strategy
	Repositories []CloneRepoSpec `yaml:"repositories"`       // Repository list
	Hooks        *CloneHooks     `yaml:"hooks,omitempty"`    // Group-level hooks (applied to all repos in group)
}

// CloneHooks represents before/after hook commands for clone operations.
// Hooks are executed without shell interpretation for security (no pipes, redirects, etc.).
type CloneHooks struct {
	Before []string `yaml:"before,omitempty"` // Commands to run before clone/update
	After  []string `yaml:"after,omitempty"`  // Commands to run after clone/update
}

// CloneRepoSpec represents a single repository specification in YAML.
type CloneRepoSpec struct {
	URL    string      `yaml:"url"`              // Required
	Name   string      `yaml:"name,omitempty"`   // Optional: custom directory name (extracted from URL if empty)
	Path   string      `yaml:"path,omitempty"`   // Optional: subdirectory within target
	Branch string      `yaml:"branch,omitempty"` // Optional: branch to checkout
	Depth  int         `yaml:"depth,omitempty"`  // Optional: shallow clone depth
	Hooks  *CloneHooks `yaml:"hooks,omitempty"`  // Optional: repo-level hooks
}

// parseCloneConfig reads and parses YAML config from file or stdin.
// Detects format automatically: flat (has repositories) or grouped (has named groups).
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

	// First, unmarshal to detect format
	var rawMap map[string]interface{}
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	// Detect format: if 'repositories' key exists at top level, it's flat format
	_, hasRepositories := rawMap["repositories"]

	config := &CloneConfig{}

	if hasRepositories {
		// Flat format: unmarshal directly
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("parse flat config: %w", err)
		}
	} else {
		// Grouped format: parse global settings and groups separately
		if err := parseGroupedCloneConfig(data, rawMap, config); err != nil {
			return nil, err
		}
	}

	// Validate
	if err := validateCloneConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// parseGroupedCloneConfig parses grouped format where keys are group names.
func parseGroupedCloneConfig(data []byte, rawMap map[string]interface{}, config *CloneConfig) error {
	// Known global keys (not groups)
	globalKeys := map[string]bool{
		"parallel": true, "strategy": true, "structure": true,
		"target": true, "update": true,
	}

	// Extract global settings
	if v, ok := rawMap["parallel"].(int); ok {
		config.Parallel = v
	}
	if v, ok := rawMap["strategy"].(string); ok {
		config.Strategy = v
	}
	if v, ok := rawMap["structure"].(string); ok {
		config.Structure = v
	}

	// Parse groups (any key that's not a global key and has 'target' + 'repositories')
	config.Groups = make(map[string]*CloneGroup)

	for key, value := range rawMap {
		if globalKeys[key] {
			continue
		}

		// Check if this looks like a group (map with target and repositories)
		groupMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		// Must have repositories to be considered a group
		if _, hasRepos := groupMap["repositories"]; !hasRepos {
			continue
		}

		// Parse group
		group := &CloneGroup{}

		if v, ok := groupMap["target"].(string); ok {
			group.Target = v
		}
		if v, ok := groupMap["branch"].(string); ok {
			group.Branch = v
		}
		if v, ok := groupMap["depth"].(int); ok {
			group.Depth = v
		}
		if v, ok := groupMap["strategy"].(string); ok {
			group.Strategy = v
		}

		// Parse group-level hooks
		if hooksRaw, ok := groupMap["hooks"].(map[string]interface{}); ok {
			group.Hooks = parseCloneHooks(hooksRaw)
		}

		// Parse repositories
		if reposRaw, ok := groupMap["repositories"].([]interface{}); ok {
			for _, repoRaw := range reposRaw {
				repoMap, ok := repoRaw.(map[string]interface{})
				if !ok {
					continue
				}

				spec := CloneRepoSpec{}
				if v, ok := repoMap["url"].(string); ok {
					spec.URL = v
				}
				if v, ok := repoMap["name"].(string); ok {
					spec.Name = v
				}
				if v, ok := repoMap["path"].(string); ok {
					spec.Path = v
				}
				if v, ok := repoMap["branch"].(string); ok {
					spec.Branch = v
				}
				if v, ok := repoMap["depth"].(int); ok {
					spec.Depth = v
				}

				// Parse repo-level hooks
				if hooksRaw, ok := repoMap["hooks"].(map[string]interface{}); ok {
					spec.Hooks = parseCloneHooks(hooksRaw)
				}

				if spec.URL != "" {
					group.Repositories = append(group.Repositories, spec)
				}
			}
		}

		if len(group.Repositories) > 0 {
			config.Groups[key] = group
		}
	}

	return nil
}

// validateCloneConfig validates the parsed YAML config.
func validateCloneConfig(config *CloneConfig) error {
	isFlat := len(config.Repositories) > 0
	isGrouped := len(config.Groups) > 0

	if !isFlat && !isGrouped {
		return fmt.Errorf("no repositories defined in config (need 'repositories' array or named groups)")
	}

	// Validate global structure
	if config.Structure != "" {
		if config.Structure != "flat" && config.Structure != "user" {
			return fmt.Errorf("invalid structure %q: must be 'flat' or 'user'", config.Structure)
		}
	}

	// Validate global strategy
	if err := validateStrategy(config.Strategy); err != nil {
		return err
	}

	if isFlat {
		// Validate flat format
		return validateFlatRepositories(config.Repositories, "")
	}

	// Validate grouped format
	for groupName, group := range config.Groups {
		if group.Target == "" {
			return fmt.Errorf("group %q: missing target directory", groupName)
		}

		if err := validateStrategy(group.Strategy); err != nil {
			return fmt.Errorf("group %q: %w", groupName, err)
		}

		if err := validateFlatRepositories(group.Repositories, groupName); err != nil {
			return err
		}
	}

	return nil
}

// validateStrategy validates a strategy value.
func validateStrategy(strategy string) error {
	if strategy == "" {
		return nil
	}
	validStrategies := map[string]bool{
		"skip": true, "pull": true, "reset": true, "rebase": true, "fetch": true,
	}
	if !validStrategies[strategy] {
		return fmt.Errorf("invalid strategy %q: must be one of 'skip', 'pull', 'reset', 'rebase', 'fetch'", strategy)
	}
	return nil
}

// validateFlatRepositories validates a list of repository specs.
func validateFlatRepositories(repos []CloneRepoSpec, groupName string) error {
	prefix := ""
	if groupName != "" {
		prefix = fmt.Sprintf("group %q: ", groupName)
	}

	if len(repos) == 0 {
		return fmt.Errorf("%sno repositories defined", prefix)
	}

	namesSeen := make(map[string]bool)
	for i, repo := range repos {
		if repo.URL == "" {
			return fmt.Errorf("%srepository[%d]: missing URL", prefix, i)
		}

		// Extract name from URL if not specified
		name := repo.Name
		if name == "" {
			extracted, err := repository.ExtractRepoNameFromURL(repo.URL)
			if err != nil {
				return fmt.Errorf("%srepository[%d]: cannot extract name from URL %q: %w", prefix, i, repo.URL, err)
			}
			name = extracted
		}

		// Include path in uniqueness check
		fullPath := name
		if repo.Path != "" {
			fullPath = filepath.Join(repo.Path, name)
		}

		if namesSeen[fullPath] {
			return fmt.Errorf("%srepository[%d]: duplicate path %q", prefix, i, fullPath)
		}
		namesSeen[fullPath] = true
	}

	return nil
}

// parseCloneHooks parses hooks from a raw map interface.
func parseCloneHooks(raw map[string]interface{}) *CloneHooks {
	hooks := &CloneHooks{}

	if before, ok := raw["before"].([]interface{}); ok {
		for _, b := range before {
			if s, ok := b.(string); ok && s != "" {
				hooks.Before = append(hooks.Before, s)
			}
		}
	}

	if after, ok := raw["after"].([]interface{}); ok {
		for _, a := range after {
			if s, ok := a.(string); ok && s != "" {
				hooks.After = append(hooks.After, s)
			}
		}
	}

	if len(hooks.Before) == 0 && len(hooks.After) == 0 {
		return nil
	}

	return hooks
}

// mergeHooks merges group-level and repo-level hooks.
// Repo hooks are appended after group hooks.
func mergeHooks(groupHooks, repoHooks *CloneHooks) *CloneHooks {
	if groupHooks == nil && repoHooks == nil {
		return nil
	}

	merged := &CloneHooks{}

	if groupHooks != nil {
		merged.Before = append(merged.Before, groupHooks.Before...)
		merged.After = append(merged.After, groupHooks.After...)
	}

	if repoHooks != nil {
		merged.Before = append(merged.Before, repoHooks.Before...)
		merged.After = append(merged.After, repoHooks.After...)
	}

	return merged
}

// executeHooks runs hook commands in the specified directory.
// Returns error if any hook fails (marks repo as failed per user decision).
// Uses direct exec without shell for security (no pipes, redirects, variables).
func executeHooks(ctx context.Context, hooks []string, workDir string, logger repository.Logger) error {
	if len(hooks) == 0 {
		return nil
	}

	// Validate working directory exists
	if _, err := os.Stat(workDir); err != nil {
		return fmt.Errorf("hook working directory does not exist: %s", workDir)
	}

	// Default timeout for hooks: 30 seconds
	hookTimeout := 30 * time.Second

	for _, hook := range hooks {
		args := parseHookCommand(hook)
		if len(args) == 0 {
			continue
		}

		// Create context with timeout
		hookCtx, cancel := context.WithTimeout(ctx, hookTimeout)

		cmd := exec.CommandContext(hookCtx, args[0], args[1:]...)
		cmd.Dir = workDir
		cmd.Env = os.Environ()

		output, err := cmd.CombinedOutput()
		cancel()

		if err != nil {
			return fmt.Errorf("hook %q failed: %w (output: %s)", hook, err, strings.TrimSpace(string(output)))
		}

		if logger != nil && len(output) > 0 {
			logger.Info("hook completed", "command", hook, "output", strings.TrimSpace(string(output)))
		}
	}

	return nil
}

// parseHookCommand splits a hook command string into executable and arguments.
// Supports simple quoting but NOT shell features (pipes, redirects, variables).
// This is intentional for security - use scripts for complex commands.
func parseHookCommand(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}

	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range cmd {
		switch {
		case inQuote:
			if r == quoteChar {
				inQuote = false
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '\'':
			inQuote = true
			quoteChar = r
		case r == ' ' || r == '\t':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// buildCloneOptionsFromConfig builds BulkCloneOptions from YAML config.
// CLI flags take precedence over YAML config.
func buildCloneOptionsFromConfig(
	config *CloneConfig,
	directory string,
	flags BulkCommandFlags,
	branch string,
	depth int,
	strategy string,
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

	// Resolve strategy: CLI flag > YAML config > deprecated update field
	strategyVal := resolveCloneStrategy(strategy, update, config.Strategy, config.Update)

	// Build BulkCloneOptions with custom per-repo settings
	opts := repository.BulkCloneOptions{
		Directory: targetDir,
		Structure: repository.DirectoryStructure(structureVal),
		Strategy:  strategyVal,
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

// resolveCloneStrategy resolves the effective strategy with precedence:
// CLI --update-strategy > CLI --update > YAML strategy > YAML update > default (skip)
func resolveCloneStrategy(cliStrategy string, cliUpdate bool, yamlStrategy string, yamlUpdate bool) repository.UpdateStrategy {
	// CLI --update-strategy takes highest precedence
	if cliStrategy != "" {
		return repository.UpdateStrategy(cliStrategy)
	}
	// CLI --update (deprecated) maps to pull
	if cliUpdate {
		fmt.Fprintln(os.Stderr, "Warning: --update is deprecated, use --update-strategy=pull instead")
		return repository.StrategyPull
	}
	// YAML strategy
	if yamlStrategy != "" {
		return repository.UpdateStrategy(yamlStrategy)
	}
	// YAML update (deprecated) maps to pull
	if yamlUpdate {
		fmt.Fprintln(os.Stderr, "Warning: 'update: true' in config is deprecated, use 'strategy: pull' instead")
		return repository.StrategyPull
	}
	// Default: skip
	return repository.StrategySkip
}

// runCloneFromConfig executes clone operation based on YAML config.
// Supports both flat format (repositories array) and grouped format (named groups).
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

	// Determine if flat or grouped format
	isGrouped := len(config.Groups) > 0

	if isGrouped {
		return runCloneFromGroupedConfig(ctx, config, directory)
	}

	return runCloneFromFlatConfig(ctx, config, directory)
}

// runCloneFromFlatConfig handles flat format (repositories array).
func runCloneFromFlatConfig(ctx context.Context, config *CloneConfig, directory string) error {
	client := repository.NewClient()
	logger := createBulkLogger(verbose)

	baseOpts := buildCloneOptionsFromConfig(
		config,
		directory,
		cloneFlags,
		cloneBranch,
		cloneDepth,
		cloneStrategy,
		cloneUpdate,
		cloneStructure,
		logger,
	)

	totalRepos := len(config.Repositories)

	if shouldShowProgress(cloneFlags.Format, quiet) {
		suffix := ""
		if cloneFlags.DryRun {
			suffix = " [DRY-RUN]"
		}
		strategyStr := ""
		if baseOpts.Strategy != repository.StrategySkip {
			strategyStr = fmt.Sprintf(", strategy: %s", baseOpts.Strategy)
		}
		dirStr := ""
		if baseOpts.Directory != "." {
			dirStr = fmt.Sprintf(" to %s", baseOpts.Directory)
		}
		fmt.Printf("Cloning %d repositories%s (parallel: %d, structure: %s%s)%s\n",
			totalRepos, dirStr, baseOpts.Parallel, baseOpts.Structure, strategyStr, suffix)
	}

	results := cloneRepositoriesParallel(ctx, client, config.Repositories, baseOpts, "", nil)
	displayCloneResults(results)

	return nil
}

// runCloneFromGroupedConfig handles grouped format (named groups with targets).
func runCloneFromGroupedConfig(ctx context.Context, config *CloneConfig, directory string) error {
	client := repository.NewClient()
	logger := createBulkLogger(verbose)

	// Filter groups if --group flag is specified
	groupsToClone := config.Groups
	if len(cloneGroup) > 0 {
		groupsToClone = make(map[string]*CloneGroup)
		for _, name := range cloneGroup {
			if group, ok := config.Groups[name]; ok {
				groupsToClone[name] = group
			} else {
				return fmt.Errorf("group %q not found in config", name)
			}
		}
	}

	// Count total repositories
	totalRepos := 0
	for _, group := range groupsToClone {
		totalRepos += len(group.Repositories)
	}

	if shouldShowProgress(cloneFlags.Format, quiet) {
		suffix := ""
		if cloneFlags.DryRun {
			suffix = " [DRY-RUN]"
		}
		fmt.Printf("Cloning %d repositories from %d groups%s\n",
			totalRepos, len(groupsToClone), suffix)
	}

	// Clone each group
	allResults := &repository.BulkCloneResult{
		TotalRequested: totalRepos,
		Summary:        make(map[string]int),
		Repositories:   make([]repository.RepositoryCloneResult, 0, totalRepos),
	}

	startTime := time.Now()

	for groupName, group := range groupsToClone {
		// Resolve target directory (relative to CLI directory argument)
		targetDir := group.Target
		if directory != "." && !filepath.IsAbs(targetDir) {
			targetDir = filepath.Join(directory, targetDir)
		}

		// Build options for this group
		groupOpts := buildGroupCloneOptions(config, group, targetDir, logger)

		if shouldShowProgress(cloneFlags.Format, quiet) {
			fmt.Printf("\n[%s] → %s (%d repos)\n", groupName, targetDir, len(group.Repositories))
		}

		// Clone repositories in this group
		groupResults := cloneRepositoriesParallel(ctx, client, group.Repositories, groupOpts, groupName, group.Hooks)

		// Merge results
		for _, r := range groupResults.Repositories {
			allResults.Repositories = append(allResults.Repositories, r)
			allResults.Summary[r.Status]++

			switch r.Status {
			case "cloned":
				allResults.TotalCloned++
			case "updated", "pulled", "rebased":
				allResults.TotalUpdated++
			case "skipped":
				allResults.TotalSkipped++
			case "error":
				allResults.TotalFailed++
			}
		}
	}

	allResults.Duration = time.Since(startTime)
	displayCloneResults(allResults)

	return nil
}

// buildGroupCloneOptions builds clone options for a specific group.
func buildGroupCloneOptions(config *CloneConfig, group *CloneGroup, targetDir string, logger repository.Logger) repository.BulkCloneOptions {
	// Resolve parallel: CLI > global config > default
	parallel := cloneFlags.Parallel
	if parallel == 0 && config.Parallel > 0 {
		parallel = config.Parallel
	}
	if parallel == 0 {
		parallel = repository.DefaultBulkParallel
	}

	// Resolve strategy: CLI > group > global > default
	strategy := resolveCloneStrategy(cloneStrategy, cloneUpdate, group.Strategy, false)
	if strategy == repository.StrategySkip && config.Strategy != "" {
		strategy = repository.UpdateStrategy(config.Strategy)
	}

	// Resolve structure
	structureVal := cloneStructure
	if structureVal == "flat" && config.Structure != "" {
		structureVal = config.Structure
	}

	// Resolve branch: CLI > group > empty
	branch := cloneBranch
	if branch == "" && group.Branch != "" {
		branch = group.Branch
	}

	// Resolve depth: CLI > group > 0
	depth := cloneDepth
	if depth == 0 && group.Depth > 0 {
		depth = group.Depth
	}

	return repository.BulkCloneOptions{
		Directory: targetDir,
		Structure: repository.DirectoryStructure(structureVal),
		Strategy:  strategy,
		Branch:    branch,
		Depth:     depth,
		Parallel:  parallel,
		DryRun:    cloneFlags.DryRun,
		Verbose:   verbose,
		Logger:    logger,
	}
}

// cloneRepositoriesParallel clones repositories in parallel and returns results.
func cloneRepositoriesParallel(
	ctx context.Context,
	client repository.Client,
	repos []CloneRepoSpec,
	opts repository.BulkCloneOptions,
	groupName string,
	groupHooks *CloneHooks,
) *repository.BulkCloneResult {
	totalRepos := len(repos)

	results := &repository.BulkCloneResult{
		TotalRequested: totalRepos,
		Summary:        make(map[string]int),
		Repositories:   make([]repository.RepositoryCloneResult, 0, totalRepos),
	}

	startTime := time.Now()

	type workItem struct {
		index    int
		repoSpec CloneRepoSpec
		repoName string
	}

	workChan := make(chan workItem, totalRepos)
	resultsChan := make(chan repository.RepositoryCloneResult, totalRepos)

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < opts.Parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workChan {
				// Show progress
				if shouldShowProgress(cloneFlags.Format, quiet) {
					prefix := ""
					if groupName != "" {
						prefix = fmt.Sprintf("[%s] ", groupName)
					}
					fmt.Printf("%s[%d/%d] Cloning %s...\n", prefix, work.index+1, totalRepos, work.repoName)
				}

				result := cloneSingleRepository(ctx, client, work.repoSpec, work.repoName, opts, groupHooks)
				resultsChan <- result
			}
		}()
	}

	// Send work items
	go func() {
		for i, repoSpec := range repos {
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

		if results.Summary == nil {
			results.Summary = make(map[string]int)
		}
		results.Summary[result.Status]++
	}

	results.Duration = time.Since(startTime)
	return results
}

// cloneSingleRepository clones a single repository with custom settings.
func cloneSingleRepository(
	ctx context.Context,
	client repository.Client,
	spec CloneRepoSpec,
	repoName string,
	baseOpts repository.BulkCloneOptions,
	groupHooks *CloneHooks,
) repository.RepositoryCloneResult {
	startTime := time.Now()

	// Build destination path
	// If spec.Path is set, use it as subdirectory within target
	// Otherwise use repoName directly under target
	var destination string
	var relativePath string
	if spec.Path != "" {
		destination = filepath.Join(baseOpts.Directory, spec.Path)
		relativePath = spec.Path
	} else {
		destination = filepath.Join(baseOpts.Directory, repoName)
		relativePath = repoName
	}

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

	// Determine if we should update existing repos (any strategy except skip)
	shouldUpdate := baseOpts.Strategy != "" && baseOpts.Strategy != repository.StrategySkip

	// Dry run mode
	if baseOpts.DryRun {
		status := "would-clone"
		if exists && isGitRepo && shouldUpdate {
			status = "would-update"
		} else if exists && isGitRepo {
			status = "skipped"
		}

		return repository.RepositoryCloneResult{
			URL:          spec.URL,
			Path:         destination,
			RelativePath: relativePath,
			Branch:       branch,
			Status:       status,
			Duration:     time.Since(startTime),
		}
	}

	// Skip if exists and not updating
	if exists && isGitRepo && !shouldUpdate {
		return repository.RepositoryCloneResult{
			URL:          spec.URL,
			Path:         destination,
			RelativePath: relativePath,
			Branch:       branch,
			Status:       "skipped",
			Duration:     time.Since(startTime),
		}
	}

	// Merge group and repo hooks
	mergedHooks := mergeHooks(groupHooks, spec.Hooks)

	// Execute before hooks (in parent directory)
	if mergedHooks != nil && len(mergedHooks.Before) > 0 {
		parentDir := baseOpts.Directory
		if parentDir == "" {
			parentDir = "."
		}
		// Ensure parent directory exists
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			return repository.RepositoryCloneResult{
				URL:          spec.URL,
				Path:         destination,
				RelativePath: relativePath,
				Branch:       branch,
				Status:       "error",
				Error:        fmt.Errorf("create parent directory for before hooks: %w", err),
				Duration:     time.Since(startTime),
			}
		}

		if err := executeHooks(ctx, mergedHooks.Before, parentDir, baseOpts.Logger); err != nil {
			return repository.RepositoryCloneResult{
				URL:          spec.URL,
				Path:         destination,
				RelativePath: relativePath,
				Branch:       branch,
				Status:       "error",
				Error:        fmt.Errorf("before hook: %w", err),
				Duration:     time.Since(startTime),
			}
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

	if exists && isGitRepo && shouldUpdate {
		// Update existing repository based on strategy
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
		RelativePath: relativePath,
		Branch:       branch,
		Status:       status,
		Duration:     time.Since(startTime),
	}

	if err != nil {
		result.Error = err
	}

	// Execute after hooks (in cloned repo directory) only if clone/update succeeded
	if result.Status != "error" && mergedHooks != nil && len(mergedHooks.After) > 0 {
		if hookErr := executeHooks(ctx, mergedHooks.After, destination, baseOpts.Logger); hookErr != nil {
			result.Status = "error"
			result.Error = fmt.Errorf("after hook: %w", hookErr)
		}
	}

	result.Duration = time.Since(startTime)
	return result
}
