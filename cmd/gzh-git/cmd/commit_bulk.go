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

	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

var (
	commitBulkFlags        BulkCommandFlags
	commitBulkMessage      string
	commitBulkYes          bool
	commitBulkEdit         bool
	commitBulkMessagesFile string
)

// bulkCmd represents the commit bulk command
var bulkCmd = &cobra.Command{
	Use:   "bulk [directory]",
	Short: "Batch commit changes across multiple repositories",
	Long: `Scan for Git repositories and commit uncommitted changes in parallel.

This command recursively scans the specified directory (or current directory)
for Git repositories with uncommitted changes and commits them in batch.

By default:
  - Scans 1 directory level deep
  - Processes 5 repositories in parallel
  - Shows preview only (use --yes to commit)
  - Auto-generates commit messages based on changed files

The workflow is:
  1. Scan repositories and identify dirty ones
  2. Show preview table with repositories and suggested messages
  3. Execute commits if --yes is specified

Use --yes to commit, --edit to modify messages in $EDITOR first.`,
	Example: `  # Commit all dirty repositories in current directory
  gz-git commit bulk -d 1

  # Commit with custom message for all
  gz-git commit bulk -m "chore: update dependencies" ~/projects

  # Dry run to see what would be committed
  gz-git commit bulk --dry-run ~/projects

  # Skip confirmation
  gz-git commit bulk --yes ~/workspace

  # Edit messages in editor before committing
  gz-git commit bulk -e ~/projects

  # Use messages from JSON file
  gz-git commit bulk --messages-file /tmp/messages.json ~/projects

  # Filter by pattern
  gz-git commit bulk --include "myproject.*" ~/workspace

  # Exclude pattern
  gz-git commit bulk --exclude "test.*" ~/projects

  # JSON output (for scripting)
  gz-git commit bulk --format json ~/projects

  # Process up to 2 levels deep with 10 parallel workers
  gz-git commit bulk -d 2 --parallel 10 ~/projects`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCommitBulk,
}

func init() {
	commitCmd.AddCommand(bulkCmd)

	// Common bulk operation flags
	addBulkFlags(bulkCmd, &commitBulkFlags)

	// Commit-specific flags
	bulkCmd.Flags().StringVarP(&commitBulkMessage, "message", "m", "", "common commit message for all repositories")
	bulkCmd.Flags().BoolVarP(&commitBulkYes, "yes", "y", false, "auto-approve without confirmation")
	bulkCmd.Flags().BoolVarP(&commitBulkEdit, "edit", "e", false, "edit messages in $EDITOR before committing")
	bulkCmd.Flags().StringVar(&commitBulkMessagesFile, "messages-file", "", "JSON file with custom messages per repository")
}

func runCommitBulk(cmd *cobra.Command, args []string) error {
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
	if err := validateBulkDepth(cmd, commitBulkFlags.Depth); err != nil {
		return err
	}

	// Validate format
	if err := validateBulkFormat(commitBulkFlags.Format); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	// Build options
	opts := repository.BulkCommitOptions{
		Directory:         directory,
		Parallel:          commitBulkFlags.Parallel,
		MaxDepth:          commitBulkFlags.Depth,
		DryRun:            commitBulkFlags.DryRun,
		Message:           commitBulkMessage,
		Yes:               commitBulkYes,
		Verbose:           verbose,
		IncludeSubmodules: commitBulkFlags.IncludeSubmodules,
		IncludePattern:    commitBulkFlags.Include,
		ExcludePattern:    commitBulkFlags.Exclude,
		Logger:            logger,
		ProgressCallback:  createProgressCallback("Analyzing", commitBulkFlags.Format, quiet),
	}

	// Load messages from file if provided
	var customMessages map[string]string
	if commitBulkMessagesFile != "" {
		var err error
		customMessages, err = loadMessagesFile(commitBulkMessagesFile)
		if err != nil {
			return fmt.Errorf("failed to load messages file: %w", err)
		}
	}

	// Scanning phase
	if shouldShowProgress(commitBulkFlags.Format, quiet) {
		fmt.Printf("Scanning for repositories in %s (depth: %d)...\n", directory, commitBulkFlags.Depth)
	}

	// Execute bulk commit (analysis phase)
	result, err := client.BulkCommit(ctx, opts)
	if err != nil {
		return fmt.Errorf("bulk commit failed: %w", err)
	}

	// Display scan completion message
	if shouldShowProgress(commitBulkFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// Apply custom messages from file
	if customMessages != nil {
		applyCustomMessages(result, customMessages)
	}

	// If -e flag is set, open editor for message editing
	if commitBulkEdit && result.TotalDirty > 0 && !opts.DryRun {
		editedMessages, err := editMessagesInEditor(result)
		if err != nil {
			return fmt.Errorf("editor failed: %w", err)
		}
		if editedMessages == nil {
			fmt.Println("Cancelled (empty file).")
			return nil
		}
		applyCustomMessages(result, editedMessages)
		// After editor, proceed to commit
		opts.Yes = true
	}

	// If --yes not specified and not using editor, treat as dry-run (preview only)
	if !opts.Yes && !opts.DryRun && result.TotalDirty > 0 {
		opts.DryRun = true
		if shouldShowProgress(commitBulkFlags.Format, quiet) {
			fmt.Println("Hint: Use --yes (-y) to commit, or --edit (-e) to edit messages first")
		}
	}

	// Execute commits if --yes is set
	if opts.Yes && !opts.DryRun && result.TotalDirty > 0 {
		// Pass the custom messages via MessageGenerator
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

	// Display results
	if commitBulkFlags.Format == "json" || !quiet {
		displayCommitResults(result)
	}

	return nil
}

func displayCommitPreview(result *repository.BulkCommitResult) {
	fmt.Println()
	fmt.Println("=== Bulk Commit Preview ===")
	fmt.Printf("Found %d dirty repositories\n\n", result.TotalDirty)

	// Table header
	fmt.Printf(" # | %-30s | %-10s | Files | %-10s | %s\n", "Repository", "Branch", "+/-", "Message (suggested)")
	fmt.Println(strings.Repeat("-", 105))

	// Calculate totals
	totalFiles := 0
	totalAdditions := 0
	totalDeletions := 0
	rowNum := 0

	for _, repo := range result.Repositories {
		if repo.Status != "dirty" && repo.Status != "would-commit" {
			continue
		}
		rowNum++

		// Truncate path if too long
		path := repo.RelativePath
		if len(path) > 28 {
			path = "..." + path[len(path)-25:]
		}

		// Truncate branch if too long
		branch := repo.Branch
		if len(branch) > 10 {
			branch = branch[:7] + "..."
		}

		// Format +/-
		plusMinus := fmt.Sprintf("+%d/-%d", repo.Additions, repo.Deletions)

		// Truncate message if too long
		msg := repo.SuggestedMessage
		if repo.Message != "" {
			msg = repo.Message
		}
		if len(msg) > 35 {
			msg = msg[:32] + "..."
		}

		fmt.Printf("%2d | %-30s | %-10s | %5d | %-10s | %s\n",
			rowNum,
			path,
			branch,
			repo.FilesChanged,
			plusMinus,
			msg,
		)

		totalFiles += repo.FilesChanged
		totalAdditions += repo.Additions
		totalDeletions += repo.Deletions
	}

	fmt.Println(strings.Repeat("-", 105))
	fmt.Printf("Total: %d repositories, %d files, +%d/-%d lines\n\n", result.TotalDirty, totalFiles, totalAdditions, totalDeletions)
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

	// Get editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vim"
	}

	// Open editor
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("editor failed: %w", err)
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
	if commitBulkFlags.Format == "json" {
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
	if commitBulkFlags.Format != "compact" && len(result.Repositories) > 0 {
		fmt.Println("Repository details:")
		for _, repo := range result.Repositories {
			displayCommitRepositoryResult(repo)
		}
	}

	// Display only errors/committed in compact mode
	if commitBulkFlags.Format == "compact" {
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

// CommitJSONOutput represents the JSON output structure for commit bulk command
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

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}
