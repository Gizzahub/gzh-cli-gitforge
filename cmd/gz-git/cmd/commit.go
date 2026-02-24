package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	commitFlags        BulkCommandFlags
	commitAll          string   // --all: common message for all repos
	commitMessages     []string // -m, --message: per-repo messages
	commitYes          bool
	commitEdit         bool
	commitMessagesFile string
)

// commitCmd represents the commit command
var commitCmd = &cobra.Command{
	Use:   "commit [directory]",
	Short: "Commit changes across multiple repositories",
	Long: cliutil.QuickStartHelp(`  # Commit with per-repository messages (most common usage)
  gz-git commit -m "repo1:feat: add feature" -m "repo2:fix: bug fix"

  # Commit with same message for all repositories
  gz-git commit --all "chore: update dependencies"

  # Interactive mode: edit messages in editor before committing
  gz-git commit -e

  # Scan and show dirty repos (preview only)
  gz-git commit

  # Skip confirmation
  gz-git commit --yes`),
	Args: cobra.MaximumNArgs(1),
	RunE: runCommit,
}

func init() {
	rootCmd.AddCommand(commitCmd)

	// Common bulk operation flags
	addBulkFlags(commitCmd, &commitFlags)

	// Commit-specific flags
	commitCmd.Flags().StringArrayVarP(&commitMessages, "message", "m", []string{}, "per-repository message in format 'repo:message' (can be repeated)")
	commitCmd.Flags().StringVar(&commitAll, "all", "", "common commit message for all repositories")
	commitCmd.Flags().BoolVarP(&commitYes, "yes", "y", false, "auto-approve without confirmation")
	commitCmd.Flags().BoolVarP(&commitEdit, "edit", "e", false, "edit messages in $EDITOR before committing")
	commitCmd.Flags().StringVar(&commitMessagesFile, "file", "", "JSON file with custom messages per repository")
}

func runCommit(cmd *cobra.Command, args []string) error {
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

	// Validate and parse directory
	directory, err := validateBulkDirectory(args)
	if err != nil {
		return err
	}

	// Validate depth
	if err := validateBulkDepth(cmd, commitFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(commitFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkCommitOptions{
		Directory:         directory,
		Parallel:          commitFlags.Parallel,
		MaxDepth:          commitFlags.Depth,
		DryRun:            commitFlags.DryRun,
		Message:           commitAll,
		Yes:               commitYes,
		Verbose:           verbose,
		IncludeSubmodules: commitFlags.IncludeSubmodules,
		IncludePattern:    commitFlags.Include,
		ExcludePattern:    commitFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Analyzing", commitFlags.Format, quiet),
	}

	// Load messages from file if provided
	var customMessages map[string]string
	if commitMessagesFile != "" {
		var err error
		customMessages, err = loadMessagesFile(commitMessagesFile)
		if err != nil {
			return fmt.Errorf("failed to load messages file: %w", err)
		}
	}

	// Parse -m/--message flag (repo:message format)
	if len(commitMessages) > 0 {
		if customMessages == nil {
			customMessages = make(map[string]string)
		}
		for _, msg := range commitMessages {
			repo, message, err := parseRepoMessage(msg)
			if err != nil {
				return fmt.Errorf("invalid --message format: %w", err)
			}
			customMessages[repo] = message
		}
	}

	// If custom messages provided, set up MessageGenerator
	if customMessages != nil {
		opts.MessageGenerator = func(ctx context.Context, repoPath string, files []string) (string, error) {
			// Extract relative path from full path
			relPath, err := filepath.Rel(directory, repoPath)
			if err != nil {
				relPath = filepath.Base(repoPath)
			}
			if relPath == "." {
				relPath = filepath.Base(directory)
			}

			// Try to find custom message by various path formats
			if msg, ok := customMessages[relPath]; ok {
				return msg, nil
			}
			if msg, ok := customMessages[filepath.Base(relPath)]; ok {
				return msg, nil
			}
			if msg, ok := customMessages[repoPath]; ok {
				return msg, nil
			}

			// No custom message found, return empty to use default
			return "", nil
		}
	}

	// If --yes not specified and not using editor, this is preview only
	// Set DryRun before first BulkCommit call to avoid accidental commits
	previewOnly := !opts.Yes && !opts.DryRun && !commitEdit
	if previewOnly {
		opts.DryRun = true
	}

	// Scanning phase
	if shouldShowProgress(commitFlags.Format, quiet) {
		printScanningMessage(directory, commitFlags.Depth, commitFlags.Parallel, commitFlags.DryRun)
	}

	// Execute bulk commit (analysis phase if DryRun, otherwise commits)
	result, err := client.BulkCommit(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk commit failed: %w", err)
	}

	// Display scan completion message
	if shouldShowProgress(commitFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// Apply custom messages from file or CLI
	if customMessages != nil {
		applyCustomMessages(result, customMessages)
	}

	// If -e flag is set, open editor for message editing then commit
	if commitEdit && result.TotalDirty > 0 {
		editedMessages, err := editMessagesInEditor(result)
		if err != nil {
			return fmt.Errorf("editor failed: %w", err)
		}
		if editedMessages == nil {
			fmt.Println("Cancelled (empty file).")
			return nil
		}
		applyCustomMessages(result, editedMessages)

		// Execute commits with edited messages
		opts.DryRun = false
		opts.Yes = true
		opts.MessageGenerator = func(ctx context.Context, repoPath string, files []string) (string, error) {
			for _, repo := range result.Repositories {
				if repo.Path == repoPath {
					if repo.Message != "" {
						return repo.Message, nil
					}
					return repo.SuggestedMessage, nil
				}
			}
			return "", nil
		}
		result, err = client.BulkCommit(ctx, opts)
		if err != nil {
			return fmt.Errorf("bulk commit failed: %w", err)
		}
	}

	// Show hint for preview mode
	if previewOnly && result.TotalDirty > 0 && shouldShowProgress(commitFlags.Format, quiet) {
		fmt.Println("Hint: Use --yes (-y) to commit, or --edit (-e) to edit messages first")
	}

	// Display results
	if commitFlags.Format == "json" || !quiet {
		displayCommitResults(result)
	}

	return nil
}

// parseRepoMessage parses "repo:message" format
func parseRepoMessage(input string) (repo, message string, err error) {
	idx := strings.Index(input, ":")
	if idx == -1 {
		return "", "", fmt.Errorf("expected format 'repo:message', got %q", input)
	}
	repo = strings.TrimSpace(input[:idx])
	message = strings.TrimSpace(input[idx+1:])
	if repo == "" || message == "" {
		return "", "", fmt.Errorf("repo and message cannot be empty in %q", input)
	}
	return repo, message, nil
}

// loadMessagesFile loads commit messages from a JSON file
func loadMessagesFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	return messages, nil
}

// applyCustomMessages applies custom messages to repository results
func applyCustomMessages(result *repository.BulkCommitResult, messages map[string]string) {
	for i := range result.Repositories {
		repo := &result.Repositories[i]
		// Try to match by relative path or full path
		if msg, ok := messages[repo.RelativePath]; ok {
			repo.Message = msg
		} else if msg, ok := messages[filepath.Base(repo.RelativePath)]; ok {
			repo.Message = msg
		} else if msg, ok := messages[repo.Path]; ok {
			repo.Message = msg
		}
	}
}

// editMessagesInEditor opens an editor for bulk message editing
// Returns nil if the user cancelled (empty file)
func editMessagesInEditor(result *repository.BulkCommitResult) (map[string]string, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "gz-git-commit-*.txt")
	if err != nil {
		return nil, fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write template
	var content strings.Builder
	content.WriteString("# Bulk Commit Messages\n")
	content.WriteString("# Edit messages below. Lines starting with # are ignored.\n")
	content.WriteString("# Format: repository: commit message\n")
	content.WriteString("# Save and close to proceed. Delete all lines to cancel.\n")
	content.WriteString("#\n")

	for _, repo := range result.Repositories {
		if repo.Status != "dirty" && repo.Status != "would-commit" {
			continue
		}
		msg := repo.SuggestedMessage
		if repo.Message != "" {
			msg = repo.Message
		}
		content.WriteString(fmt.Sprintf("%s: %s\n", repo.RelativePath, msg))
	}

	if _, err := tmpFile.WriteString(content.String()); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("cannot write temp file: %w", err)
	}
	tmpFile.Close()

	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Try common editors in order of preference
		for _, candidate := range []string{"vim", "vi", "nano", "notepad"} {
			if _, err := exec.LookPath(candidate); err == nil {
				editor = candidate
				break
			}
		}
	}
	if editor == "" {
		return nil, fmt.Errorf("no editor found: set EDITOR or VISUAL environment variable, or install vim/nano")
	}

	// Verify editor exists (in case $EDITOR is set but invalid)
	editorPath, err := exec.LookPath(editor)
	if err != nil {
		return nil, fmt.Errorf("editor '%s' not found: %w (set EDITOR or VISUAL to a valid editor)", editor, err)
	}

	// Open editor
	cmd := exec.Command(editorPath, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Check for specific error types
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("editor exited with code %d: consider using --file instead", exitErr.ExitCode())
		}
		return nil, fmt.Errorf("failed to run editor '%s': %w", editor, err)
	}

	// Read edited content
	editedData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read edited file: %w", err)
	}

	// Parse edited content
	messages := make(map[string]string)
	lines := strings.Split(string(editedData), "\n")
	hasContent := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse "repo: message" format
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}

		repo := strings.TrimSpace(line[:idx])
		msg := strings.TrimSpace(line[idx+1:])

		if repo != "" && msg != "" {
			messages[repo] = msg
			hasContent = true
		}
	}

	if !hasContent {
		return nil, nil // Cancelled
	}

	return messages, nil
}

func displayCommitResults(result *repository.BulkCommitResult) {
	// JSON output mode
	if commitFlags.Format == "json" {
		displayCommitResultsJSON(result)
		return
	}

	fmt.Println()
	fmt.Println("=== Bulk Commit Results ===")
	fmt.Printf("Total scanned:   %d repositories\n", result.TotalScanned)
	fmt.Printf("Total dirty:     %d repositories\n", result.TotalDirty)
	fmt.Printf("Total committed: %d repositories\n", result.TotalCommitted)
	fmt.Printf("Total skipped:   %d repositories\n", result.TotalSkipped)
	fmt.Printf("Total failed:    %d repositories\n", result.TotalFailed)
	fmt.Printf("Duration:        %s\n", result.Duration.Round(100_000_000)) // Round to 0.1s
	fmt.Println()

	// Display summary
	if len(result.Summary) > 0 {
		fmt.Println("Summary by status:")
		for status, count := range result.Summary {
			icon := getCommitStatusIcon(status)
			fmt.Printf("  %s %-15s %d\n", icon, status+":", count)
		}
		fmt.Println()
	}

	// Display individual results if not compact
	if commitFlags.Format != "compact" && len(result.Repositories) > 0 {
		fmt.Println("Repository details:")
		for _, repo := range result.Repositories {
			displayCommitRepositoryResult(repo)
		}
	}

	// Display only errors/committed in compact mode
	if commitFlags.Format == "compact" {
		hasIssues := false
		for _, repo := range result.Repositories {
			if repo.Status == "error" || repo.Status == "success" {
				if !hasIssues && repo.Status == "error" {
					fmt.Println("Issues found:")
					hasIssues = true
				}
				displayCommitRepositoryResult(repo)
			}
		}
		if !hasIssues && result.TotalCommitted > 0 {
			fmt.Printf("✓ Successfully committed %d repositories\n", result.TotalCommitted)
		} else if result.TotalDirty == 0 {
			fmt.Println("✓ All repositories are clean")
		}
	}
}

func displayCommitRepositoryResult(repo repository.RepositoryCommitResult) {
	icon := getCommitStatusIcon(repo.Status)

	// Build compact one-line format: icon path (branch) status duration
	parts := []string{icon}

	// Path with branch
	pathPart := repo.RelativePath
	if repo.Branch != "" {
		pathPart += fmt.Sprintf(" (%s)", repo.Branch)
	}
	parts = append(parts, fmt.Sprintf("%-50s", pathPart))

	// Show status compactly
	statusStr := ""
	switch repo.Status {
	case "success":
		if repo.CommitHash != "" {
			statusStr = fmt.Sprintf("committed [%s]", repo.CommitHash)
		} else {
			statusStr = "committed"
		}
	case "clean":
		statusStr = "clean"
	case "dirty", "would-commit":
		statusStr = fmt.Sprintf("%d files changed", repo.FilesChanged)
	case "error":
		statusStr = "failed"
	case "skipped":
		statusStr = "skipped"
	default:
		statusStr = repo.Status
	}

	parts = append(parts, fmt.Sprintf("%-25s", statusStr))

	// Duration
	if repo.Duration > 0 {
		parts = append(parts, fmt.Sprintf("%6s", repo.Duration.Round(10_000_000)))
	}

	// Build output line safely
	line := "  " + parts[0] + " " + parts[1] + " " + parts[2]
	if len(parts) > 3 {
		line += " " + parts[3]
	}
	fmt.Println(line)

	// Show error details if present
	if repo.Error != nil && verbose {
		fmt.Printf("    Error: %v\n", repo.Error)
	}
}

func getCommitStatusIcon(status string) string {
	switch status {
	case "success":
		return "✓"
	case "clean":
		return "="
	case "dirty", "would-commit":
		return "⚠"
	case "error":
		return "✗"
	case "skipped":
		return "⊘"
	default:
		return "•"
	}
}

// CommitJSONOutput represents the JSON output structure for commit command
type CommitJSONOutput struct {
	TotalScanned   int                          `json:"total_scanned"`
	TotalDirty     int                          `json:"total_dirty"`
	TotalCommitted int                          `json:"total_committed"`
	TotalSkipped   int                          `json:"total_skipped"`
	TotalFailed    int                          `json:"total_failed"`
	DurationMs     int64                        `json:"duration_ms"`
	Summary        map[string]int               `json:"summary"`
	Repositories   []CommitRepositoryJSONOutput `json:"repositories"`
}

// CommitRepositoryJSONOutput represents a single repository in JSON output
type CommitRepositoryJSONOutput struct {
	Path             string   `json:"path"`
	Branch           string   `json:"branch,omitempty"`
	Status           string   `json:"status"`
	CommitHash       string   `json:"commit_hash,omitempty"`
	Message          string   `json:"message,omitempty"`
	SuggestedMessage string   `json:"suggested_message,omitempty"`
	FilesChanged     int      `json:"files_changed,omitempty"`
	Additions        int      `json:"additions,omitempty"`
	Deletions        int      `json:"deletions,omitempty"`
	ChangedFiles     []string `json:"changed_files,omitempty"`
	DurationMs       int64    `json:"duration_ms,omitempty"`
	Error            string   `json:"error,omitempty"`
}

func displayCommitResultsJSON(result *repository.BulkCommitResult) {
	output := CommitJSONOutput{
		TotalScanned:   result.TotalScanned,
		TotalDirty:     result.TotalDirty,
		TotalCommitted: result.TotalCommitted,
		TotalSkipped:   result.TotalSkipped,
		TotalFailed:    result.TotalFailed,
		DurationMs:     result.Duration.Milliseconds(),
		Summary:        result.Summary,
		Repositories:   make([]CommitRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := CommitRepositoryJSONOutput{
			Path:             repo.RelativePath,
			Branch:           repo.Branch,
			Status:           repo.Status,
			CommitHash:       repo.CommitHash,
			Message:          repo.Message,
			SuggestedMessage: repo.SuggestedMessage,
			FilesChanged:     repo.FilesChanged,
			Additions:        repo.Additions,
			Deletions:        repo.Deletions,
			ChangedFiles:     repo.ChangedFiles,
			DurationMs:       repo.Duration.Milliseconds(),
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
