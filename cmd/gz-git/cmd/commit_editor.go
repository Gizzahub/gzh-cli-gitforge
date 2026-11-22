// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func commitEditorResult(ctx context.Context, client repository.Client, result *repository.BulkCommitResult, opts repository.BulkCommitOptions) (*repository.BulkCommitResult, bool, error) {
	if !commitEdit || result.TotalDirty == 0 {
		return result, false, nil
	}
	// --all is the message BulkCommit would use for every repository, so the
	// editor starts from it; the commit below then takes the edited messages
	// instead of reapplying it.
	if opts.Message != "" {
		for i := range result.Repositories {
			result.Repositories[i].Message = opts.Message
		}
		opts.Message = ""
	}
	editedMessages, err := editMessagesInEditor(result)
	if err != nil {
		return nil, false, fmt.Errorf("editor failed: %w", err)
	}
	if editedMessages == nil {
		fmt.Println("Canceled (empty file).")
		return result, true, nil
	}
	applyCustomMessages(result, editedMessages)
	opts.DryRun = false
	opts.Yes = true
	opts.MessageGenerator = commitResultMessageGenerator(result)
	result, err = client.BulkCommit(ctx, opts)
	if err != nil {
		return nil, false, fmt.Errorf("bulk commit failed: %w", err)
	}
	return result, false, nil
}

func commitResultMessageGenerator(result *repository.BulkCommitResult) func(context.Context, string, []string) (string, error) {
	return func(_ context.Context, repoPath string, _ []string) (string, error) {
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
}

// editMessagesInEditor opens an editor for bulk message editing.
// Returns nil if the user canceled (empty file).
func editMessagesInEditor(result *repository.BulkCommitResult) (messages map[string]string, err error) { //nolint:gocyclo // editor parsing has independent validation branches.
	// Create temp file
	tmpFile, err := os.CreateTemp("", "gz-git-commit-*.txt")
	if err != nil {
		return nil, fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove temporary commit message file: %w", removeErr))
		}
	}()

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
		fmt.Fprintf(&content, "%s: %s\n", repo.RelativePath, msg)
	}

	if _, err := tmpFile.WriteString(content.String()); err != nil {
		closeErr := tmpFile.Close()
		if closeErr != nil {
			return nil, errors.Join(fmt.Errorf("cannot write temp file: %w", err), fmt.Errorf("close temp file: %w", closeErr))
		}
		return nil, fmt.Errorf("cannot write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("cannot close temp file: %w", err)
	}

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
	cmd := exec.CommandContext(context.Background(), editorPath, tmpPath) // #nosec G204,G702 -- editorPath is resolved with LookPath and invoked as argv, never through a shell.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Check for specific error types
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("editor exited with code %d: consider using --json instead", exitErr.ExitCode())
		}
		return nil, fmt.Errorf("failed to run editor '%s': %w", editor, err)
	}

	// Read edited content
	editedData, err := os.ReadFile(tmpPath) // #nosec G304 -- tmpPath is created by this command for the editor.
	if err != nil {
		return nil, fmt.Errorf("cannot read edited file: %w", err)
	}

	// Parse edited content
	messages = make(map[string]string)
	lines := strings.Split(string(editedData), "\n")
	hasContent := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse "repo: message" format
		before, after, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		repo := strings.TrimSpace(before)
		msg := strings.TrimSpace(after)

		if repo != "" && msg != "" {
			messages[repo] = msg
			hasContent = true
		}
	}

	if !hasContent {
		return nil, nil //nolint:nilnil // cancellation is represented by an empty result and no error.
	}

	return messages, nil
}
