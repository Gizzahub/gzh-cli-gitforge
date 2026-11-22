// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	commitFlags    BulkCommandFlags
	commitAll      string   // --all: common message for all repos
	commitMessages []string // -m, --message: per-repo messages
	commitYes      bool
	commitEdit     bool
	commitJSON     string // --json: inline JSON messages
	commitYAML     string // --yaml: inline YAML messages

	commitAllowConflicted bool // --allow-conflicted: commit repos with unmerged paths

	commitStage       string // --stage: all|tracked|staged
	commitFile        string // --file: JSON/YAML message map from a file or "-"
	commitAutoMessage bool   // --auto-message: generate messages missing from the map
)

// commitCmd represents the commit command.
var commitCmd = &cobra.Command{
	Use:   "commit [directory]",
	Short: "Commit changes across multiple repositories",
	Long: cliutil.QuickStartHelp(`  # Commit with per-repository messages (most common usage). Every dirty
  # repository needs a message; add --auto-message to generate the rest.
  gz-git commit -m "repo1:feat: add feature" -m "repo2:fix: bug fix"

  # Commit with same message for all repositories
  gz-git commit --all "chore: update dependencies"

  # Commit using inline JSON (Best for LLM pipelines and scripts)
  gz-git commit --json '{"repo1":"feat: add feature", "repo2":"fix: bug fix"}'

  # Commit using piped JSON or YAML (Best for workflow integration)
  cat messages.yaml | gz-git commit --yes

  # Read the message map from a file ("." is the scan directory itself)
  gz-git commit --file messages.yaml --yes

  # Commit only what is already staged, in the listed repositories
  gz-git commit --select listed --stage staged --file messages.json --yes

  # Interactive mode: edit messages in editor before committing
  gz-git commit -e

  # Scan and show dirty repos (preview only)
  gz-git commit

  # Skip confirmation
  gz-git commit --yes`) + cliutil.ExitCodesBulkHelp(),
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
	commitCmd.Flags().StringVar(&commitJSON, "json", "", `inline JSON with per-repo messages (e.g., '{"repo":"message"}')`)
	commitCmd.Flags().StringVar(&commitYAML, "yaml", "", `inline YAML with per-repo messages`)
	commitCmd.Flags().BoolVar(&commitAllowConflicted, "allow-conflicted", false, "commit repositories that still have unmerged paths (writes conflict markers into history)")
	commitCmd.Flags().StringVar(&commitStage, "stage", string(repository.StageAll), "what each commit records: all (git add -A), tracked (git add -u) or staged (the index as it is)")
	commitCmd.Flags().StringVar(&commitFile, "file", "", `read the per-repo message map (JSON or YAML) from a file, or "-" for stdin`)
	commitCmd.Flags().BoolVar(&commitAutoMessage, "auto-message", false, "with a message map, generate a message for repositories the map leaves out instead of refusing")
}

func runCommit(cmd *cobra.Command, args []string) error {
	// SIGINT/SIGTERM cancels the shared context so in-flight commits stop
	// gracefully (extracted into bulk_common's withInterruptCancel).
	ctx, cancel := withInterruptCancel(context.Background())
	defer cancel()

	config, err := prepareCommitRun(cmd, args)
	if err != nil {
		return err
	}
	client := repository.NewClient()

	// Scanning phase
	if shouldShowProgress(commitFlags.Format, quiet) {
		printScanningMessage(config.directory, commitFlags.Depth, commitFlags.Parallel, commitFlags.DryRun)
	}

	// An explicit message map must name real repositories. Verify that on a
	// dry run before any commit is made.
	if config.customMessages != nil && !config.options.DryRun {
		dryOpts := config.options
		dryOpts.DryRun = true
		dryOpts.ProgressCallback = nil
		preview, previewErr := client.BulkCommit(ctx, dryOpts)
		if previewErr != nil {
			return fmt.Errorf("bulk commit failed: %w", previewErr)
		}
		if err := checkCommitMessages(config, preview); err != nil {
			return err
		}
	}

	// Execute bulk commit (analysis phase if DryRun, otherwise commits)
	result, err := client.BulkCommit(ctx, config.options)
	if err != nil {
		return fmt.Errorf("bulk commit failed: %w", err)
	}
	if config.customMessages != nil && config.options.DryRun {
		if err := checkCommitMessages(config, result); err != nil {
			return err
		}
	}

	// Display scan completion message
	if shouldShowProgress(commitFlags.Format, quiet) && result.TotalScanned == 0 {
		fmt.Printf("Scan complete: no repositories found\n")
	}

	// Apply custom messages from file or CLI
	if config.customMessages != nil {
		applyCustomMessages(result, config.customMessages)
	}

	// If -e flag is set, open editor for message editing then commit
	result, canceled, err := commitEditorResult(ctx, client, result, config.options)
	if err != nil {
		return err
	}
	if canceled {
		return nil
	}

	// Show hint for preview mode
	if config.previewOnly && result.TotalDirty > 0 && shouldShowProgress(commitFlags.Format, quiet) {
		fmt.Println("Hint: Use --yes (-y) to commit, or --edit (-e) to edit messages first")
	}

	// Display results
	if commitFlags.Format == "json" || !quiet {
		displayCommitResults(result)
	}

	// Conflicted repositories were deliberately left uncommitted. They must
	// reach the exit code, otherwise an unattended caller that only checks $?
	// records the refusal as a clean success.
	if result.TotalConflicted > 0 {
		fmt.Fprintf(os.Stderr,
			"\n%d repository(ies) skipped: unresolved merge conflicts. Resolve them and re-run, or pass --allow-conflicted.\n",
			result.TotalConflicted)
	}

	failed := result.TotalFailed + result.TotalConflicted
	return errPartialFailure(failed, max(failed, commitAttempted(result)))
}

// commitAttempted counts the repositories the run had something to do in:
// every one that was not clean, including those whose analysis failed.
func commitAttempted(result *repository.BulkCommitResult) int {
	n := 0
	for _, repo := range result.Repositories {
		if repo.Status != "clean" {
			n++
		}
	}
	return n
}

type commitRunConfig struct {
	directory      string
	keyRoot        string
	options        repository.BulkCommitOptions
	customMessages map[string]string
	previewOnly    bool
	// requireCoverage refuses a message map that leaves out a repository
	// about to be committed.
	requireCoverage bool
}

func prepareCommitRun(cmd *cobra.Command, args []string) (commitRunConfig, error) {
	directory, err := validateBulkDirectory(args)
	if err != nil {
		return commitRunConfig{}, err
	}
	if err := resolveBulkDepth(cmd, directory, &commitFlags.Depth); err != nil {
		return commitRunConfig{}, err
	}
	if err := validateBulkFormat(commitFlags.Format); err != nil {
		return commitRunConfig{}, err
	}
	// -e commits once the editor closes, so it cannot honor --dry-run.
	if commitEdit && commitFlags.DryRun {
		return commitRunConfig{}, fmt.Errorf("--edit cannot be combined with --dry-run: --edit commits the edited messages; run without --edit to preview")
	}

	stage, err := repository.ParseCommitStage(commitStage)
	if err != nil {
		return commitRunConfig{}, fmt.Errorf("--stage: %w", err)
	}

	customMessages, err := loadCommitMessages()
	if err != nil {
		return commitRunConfig{}, err
	}
	if commitAutoMessage && customMessages == nil {
		return commitRunConfig{}, fmt.Errorf("--auto-message fills gaps in a message map; give one with --file, --json, --yaml, -m or stdin")
	}

	repoSelection, err := resolveBulkSelection(directory, &commitFlags)
	if err != nil {
		return commitRunConfig{}, err
	}
	opts := repository.BulkCommitOptions{
		Directory:         directory,
		Parallel:          commitFlags.Parallel,
		MaxDepth:          commitFlags.Depth,
		DryRun:            commitFlags.DryRun,
		Message:           commitAll,
		Yes:               commitYes,
		Verbose:           verbose,
		IncludeSubmodules: commitFlags.IncludeSubmodules,
		AllowConflicted:   commitAllowConflicted,
		Stage:             stage,
		IncludePattern:    commitFlags.Include,
		ExcludePattern:    resolveScanExclude(directory, commitFlags.Exclude),
		Selection:         repoSelection,
		Logger:            createBulkLogger(verbose),
		ProgressCallback:  createProgressCallback("Analyzing", commitFlags.Format, quiet),
	}
	// Message keys are matched against paths relative to the scan root, which
	// filepath.Rel cannot compute from a relative directory and the absolute
	// repository paths the scan reports.
	keyRoot, err := filepath.Abs(directory)
	if err != nil {
		return commitRunConfig{}, fmt.Errorf("resolve %s: %w", directory, err)
	}
	if customMessages != nil {
		opts.MessageGenerator = customCommitMessageGenerator(keyRoot, customMessages)
	}

	previewOnly := !opts.Yes && !opts.DryRun && !commitEdit
	// With -e the first pass only collects the messages to edit; the commit
	// happens in commitEditorResult once the editor closes with content.
	if previewOnly || commitEdit {
		opts.DryRun = true
	}
	return commitRunConfig{
		directory:      directory,
		keyRoot:        keyRoot,
		options:        opts,
		customMessages: customMessages,
		previewOnly:    previewOnly,
		// --all gives every repository a message, and -e shows each generated
		// message to the operator before committing; neither leaves a gap.
		requireCoverage: !commitAutoMessage && commitAll == "" && !commitEdit,
	}, nil
}

// checkCommitMessages verifies an explicit message map against a dry-run
// result: every key must address a repository with changes and, unless
// coverage is waived, every repository about to be committed needs a message.
func checkCommitMessages(config commitRunConfig, preview *repository.BulkCommitResult) error {
	if err := validateCommitMessageKeys(config.keyRoot, config.customMessages, preview); err != nil {
		return err
	}
	if !config.requireCoverage {
		return nil
	}
	return validateCommitMessageCoverage(config.keyRoot, config.customMessages, preview)
}

func displayCommitResults(result *repository.BulkCommitResult) {
	// JSON or LLM output mode
	if commitFlags.Format == "json" || commitFlags.Format == "llm" {
		displayCommitResultsStructured(result, commitFlags.Format)
		return
	}

	fmt.Println()
	fmt.Println("=== Bulk Commit Results ===")
	fmt.Printf("Total scanned:   %d repositories\n", result.TotalScanned)
	fmt.Printf("Total dirty:     %d repositories\n", result.TotalDirty)
	fmt.Printf("Total committed: %d repositories\n", result.TotalCommitted)
	fmt.Printf("Total skipped:   %d repositories\n", result.TotalSkipped)
	if result.TotalConflicted > 0 {
		fmt.Printf("Total conflicted: %d repositories (not committed)\n", result.TotalConflicted)
	}
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
			if repo.Status == "error" || repo.Status == "success" || repo.Status == "conflicted" {
				if !hasIssues && repo.Status != "success" {
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
	case "conflicted":
		statusStr = fmt.Sprintf("CONFLICT: %d unmerged", len(repo.ConflictedFiles))
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

	for _, dir := range repo.NestedRepos {
		fmt.Printf("    left out nested repository: %s/ (commit it on its own)\n", dir)
	}

	// Conflicted repositories always list their unmerged paths, regardless of
	// --verbose: this is a refusal the user has to act on, and burying it was
	// exactly how the silent conflict-commit went unnoticed.
	if repo.Status == "conflicted" {
		for _, file := range repo.ConflictedFiles {
			fmt.Printf("    unmerged: %s\n", file)
		}
		fmt.Println("    → resolve the merge, then re-run (or pass --allow-conflicted)")
		return
	}

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
	case "conflicted":
		return "⊗"
	case "error":
		return "✗"
	case "skipped":
		return "⊘"
	default:
		return "•"
	}
}

// CommitJSONOutput represents the JSON output structure for commit command.
type CommitJSONOutput struct {
	TotalScanned    int                          `json:"total_scanned"`
	TotalDirty      int                          `json:"total_dirty"`
	TotalCommitted  int                          `json:"total_committed"`
	TotalSkipped    int                          `json:"total_skipped"`
	TotalConflicted int                          `json:"total_conflicted,omitempty"`
	TotalFailed     int                          `json:"total_failed"`
	DurationMs      int64                        `json:"duration_ms"`
	Summary         map[string]int               `json:"summary"`
	Repositories    []CommitRepositoryJSONOutput `json:"repositories"`
}

// CommitRepositoryJSONOutput represents a single repository in JSON output.
type CommitRepositoryJSONOutput struct {
	Path                  string   `json:"path"`
	Branch                string   `json:"branch,omitempty"`
	Status                string   `json:"status"`
	CommitHash            string   `json:"commit_hash,omitempty"`
	Message               string   `json:"message,omitempty"`
	SuggestedMessage      string   `json:"suggested_message,omitempty"`
	FilesChanged          int      `json:"files_changed,omitempty"`
	TrackedFilesChanged   int      `json:"tracked_files_changed,omitempty"`
	UntrackedFilesChanged int      `json:"untracked_files_changed,omitempty"`
	StagedFilesChanged    int      `json:"staged_files_changed,omitempty"`
	Additions             int      `json:"additions,omitempty"`
	Deletions             int      `json:"deletions,omitempty"`
	ChangedFiles          []string `json:"changed_files,omitempty"`
	ConflictedFiles       []string `json:"conflicted_files,omitempty"`
	NestedRepos           []string `json:"nested_repos,omitempty"`
	DurationMs            int64    `json:"duration_ms,omitempty"`
	Error                 string   `json:"error,omitempty"`
}

func displayCommitResultsStructured(result *repository.BulkCommitResult, format string) {
	output := CommitJSONOutput{
		TotalScanned:    result.TotalScanned,
		TotalDirty:      result.TotalDirty,
		TotalCommitted:  result.TotalCommitted,
		TotalSkipped:    result.TotalSkipped,
		TotalConflicted: result.TotalConflicted,
		TotalFailed:     result.TotalFailed,
		DurationMs:      result.Duration.Milliseconds(),
		Summary:         result.Summary,
		Repositories:    make([]CommitRepositoryJSONOutput, 0, len(result.Repositories)),
	}

	for _, repo := range result.Repositories {
		repoOutput := CommitRepositoryJSONOutput{
			Path:                  repo.RelativePath,
			Branch:                repo.Branch,
			Status:                repo.Status,
			CommitHash:            repo.CommitHash,
			Message:               repo.Message,
			SuggestedMessage:      repo.SuggestedMessage,
			FilesChanged:          repo.FilesChanged,
			TrackedFilesChanged:   repo.TrackedFilesChanged,
			UntrackedFilesChanged: repo.UntrackedFilesChanged,
			StagedFilesChanged:    repo.StagedFilesChanged,
			Additions:             repo.Additions,
			Deletions:             repo.Deletions,
			ChangedFiles:          repo.ChangedFiles,
			ConflictedFiles:       repo.ConflictedFiles,
			NestedRepos:           repo.NestedRepos,
			DurationMs:            repo.Duration.Milliseconds(),
		}
		if repo.Error != nil {
			repoOutput.Error = repo.Error.Error()
		}
		output.Repositories = append(output.Repositories, repoOutput)
	}

	writeBulkOutput(format, output)
}
