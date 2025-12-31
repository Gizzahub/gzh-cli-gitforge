package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/commit"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	autoTemplate string
	autoScope    string
	autoType     string
	autoDryRun   bool
	autoEdit     bool
)

// autoCmd represents the commit auto command
var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Automatically generate and create a commit",
	Long: `Analyze staged changes and automatically generate a commit message
following conventional commit format.

This command will:
1. Analyze staged changes (git diff --cached)
2. Infer commit type (feat, fix, docs, etc.)
3. Detect scope from changed files
4. Generate a descriptive message
5. Validate the message
6. Create the commit (unless --dry-run)`,
	Example: `  # Auto-commit staged changes
  git add .
  gz-git commit auto

  # Preview message without committing
  gz-git commit auto --dry-run

  # Override detected type and scope
  gz-git commit auto --type feat --scope auth

  # Use different template
  gz-git commit auto --template semantic`,
	RunE: runCommitAuto,
}

func init() {
	commitCmd.AddCommand(autoCmd)

	autoCmd.Flags().StringVar(&autoTemplate, "template", "conventional", "template to use: conventional, semantic")
	autoCmd.Flags().StringVar(&autoScope, "scope", "", "override detected scope")
	autoCmd.Flags().StringVar(&autoType, "type", "", "override detected type")
	autoCmd.Flags().BoolVar(&autoDryRun, "dry-run", false, "show message without committing")
	autoCmd.Flags().BoolVar(&autoEdit, "edit", false, "open editor to edit message before committing")
}

// ValidCommitTemplates contains the list of valid templates for commit auto command
var ValidCommitTemplates = []string{"conventional", "semantic"}

// validateCommitTemplate validates the template flag for commit auto command
func validateCommitTemplate(template string) error {
	for _, valid := range ValidCommitTemplates {
		if template == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid template %q: must be one of: conventional, semantic", template)
}

func runCommitAuto(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate template
	if err := validateCommitTemplate(autoTemplate); err != nil {
		return err
	}

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

	// Load template
	templateMgr := commit.NewTemplateManager()
	tmpl, err := templateMgr.Load(ctx, autoTemplate)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}

	// Create generator
	gen := commit.NewGenerator()

	// Generate commit message
	message, err := gen.Generate(ctx, repo, commit.GenerateOptions{
		Template:  tmpl,
		MaxLength: 72,
	})
	if err != nil {
		if err == commit.ErrNoChanges {
			return fmt.Errorf("no changes staged for commit\nUse 'git add <file>...' to stage changes")
		}
		return fmt.Errorf("failed to generate commit message: %w", err)
	}

	// Validate the message
	validator := commit.NewValidator()
	result, err := validator.Validate(ctx, message, tmpl)
	if err != nil {
		return fmt.Errorf("failed to validate message: %w", err)
	}

	// Show generated message
	if !quiet {
		fmt.Printf("\nGenerated Commit Message:\n\n")
		fmt.Printf("  %s\n\n", message)

		if result.Warnings != nil && len(result.Warnings) > 0 {
			fmt.Println("⚠ Warnings:")
			for _, warning := range result.Warnings {
				fmt.Printf("  - %s\n", warning.Message)
				if warning.Suggestion != "" {
					fmt.Printf("    Suggestion: %s\n", warning.Suggestion)
				}
			}
			fmt.Println()
		}
	}

	// Check for validation errors
	if !result.Valid {
		fmt.Fprintln(os.Stderr, "\n✗ Validation failed:")
		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "  - %s", err.Message)
			if err.Line > 0 {
				fmt.Fprintf(os.Stderr, " (line %d)", err.Line)
			}
			fmt.Fprintln(os.Stderr)
		}
		return fmt.Errorf("commit message validation failed")
	}

	// Dry run mode
	if autoDryRun {
		if !quiet {
			fmt.Println("✓ Dry run mode - no commit created")
		}
		return nil
	}

	// Edit mode - open editor to modify the message
	if autoEdit {
		editedMessage, err := editMessageInEditor(ctx, message)
		if err != nil {
			return fmt.Errorf("failed to edit message: %w", err)
		}
		if editedMessage == "" {
			return fmt.Errorf("aborting commit due to empty message")
		}
		message = editedMessage
		if !quiet {
			fmt.Printf("\nEdited Commit Message:\n\n  %s\n\n", message)
		}
	}

	// Create commit using git command
	if !quiet {
		fmt.Println("Creating commit...")
	}

	// Execute git commit directly using os/exec
	gitCmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	gitCmd.Dir = repo.Path

	output, err := gitCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create commit: %w\nOutput: %s", err, string(output))
	}

	if !quiet {
		fmt.Println("\n✓ Commit created successfully!")
		fmt.Println(string(output))
	}

	return nil
}

// editMessageInEditor opens the user's preferred editor to edit the commit message.
// Returns the edited message or an error.
func editMessageInEditor(ctx context.Context, initialMessage string) (string, error) {
	// Find editor from environment variables
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi" // Default fallback
	}

	// Create temporary file for editing
	tmpFile, err := os.CreateTemp("", "gz-git-commit-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write initial message to temp file with instructions
	content := initialMessage + "\n\n# Edit the commit message above.\n# Lines starting with '#' will be ignored.\n# An empty message aborts the commit.\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Open editor
	editorCmd := exec.CommandContext(ctx, editor, tmpPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}

	// Read edited content
	editedContent, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read edited file: %w", err)
	}

	// Remove comment lines and trim
	lines := strings.Split(string(editedContent), "\n")
	var messageLines []string
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			messageLines = append(messageLines, line)
		}
	}

	return strings.TrimSpace(strings.Join(messageLines, "\n")), nil
}
