package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	commitFlags        BulkCommandFlags
	commitMessage      string
	commitMessages     []string // NEW: Multiple repo:message pairs
	commitYes          bool
	commitEdit         bool
	commitMessagesFile string
)

// commitCmd represents the commit command (now with bulk as default)
var commitCmd = &cobra.Command{
	Use:   "commit [directory]",
	Short: "Commit changes across multiple repositories",
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

Use --yes to commit, --edit to modify messages in $EDITOR first.

Subcommands:
  auto      - Single repository auto-commit
  validate  - Validate commit message format
  template  - Manage commit message templates`,
	Example: `  # Commit all dirty repositories in current directory
  gz-git commit -d 1

  # Commit with custom message for all
  gz-git commit -m "chore: update dependencies"

  # Commit with per-repository messages (NEW!)
  gz-git commit --messages "repo1:feat: add feature" --messages "repo2:fix: bug fix"

  # Dry run to see what would be committed
  gz-git commit --dry-run

  # Skip confirmation
  gz-git commit --yes

  # Edit messages in editor before committing
  gz-git commit -e

  # Use messages from JSON file
  gz-git commit --messages-file /tmp/messages.json

  # JSON output (for scripting)
  gz-git commit --format json

  # Subcommands (for single repo):
  gz-git commit auto              # Auto-commit current repo
  gz-git commit validate "msg"    # Validate message format`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCommit,
}

func init() {
	rootCmd.AddCommand(commitCmd)

	// Common bulk operation flags
	addBulkFlags(commitCmd, &commitFlags)

	// Commit-specific flags
	commitCmd.Flags().StringVarP(&commitMessage, "message", "m", "", "common commit message for all repositories")
	commitCmd.Flags().StringArrayVar(&commitMessages, "messages", []string{}, "per-repository messages in format 'repo:message'")
	commitCmd.Flags().BoolVarP(&commitYes, "yes", "y", false, "auto-approve without confirmation")
	commitCmd.Flags().BoolVarP(&commitEdit, "edit", "e", false, "edit messages in $EDITOR before committing")
	commitCmd.Flags().StringVar(&commitMessagesFile, "messages-file", "", "JSON file with custom messages per repository")
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
		Message:           commitMessage,
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

	// NEW: Parse --messages flag (repo:message format)
	if len(commitMessages) > 0 {
		if customMessages == nil {
			customMessages = make(map[string]string)
		}
		for _, msg := range commitMessages {
			repo, message, err := parseRepoMessage(msg)
			if err != nil {
				return fmt.Errorf("invalid --messages format: %w", err)
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
		displayCommitResults(result, commitFlags.Format)
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

// displayCommitResults is a wrapper to call commit_bulk's display function with format parameter
func displayCommitResults(result *repository.BulkCommitResult, format string) {
	// Temporarily set the format for commit_bulk's display function
	oldFormat := commitBulkFlags.Format
	commitBulkFlags.Format = format
	defer func() { commitBulkFlags.Format = oldFormat }()

	// Call the existing display function from commit_bulk.go
	displayCommitBulkResults(result)
}
