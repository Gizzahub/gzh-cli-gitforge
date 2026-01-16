// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BulkDiffOptions configures bulk diff operations.
type BulkDiffOptions struct {
	// Directory is the root directory to scan for repositories
	Directory string

	// Parallel is the number of concurrent workers (default: 10)
	Parallel int

	// MaxDepth is the maximum directory depth to scan (default: 1)
	MaxDepth int

	// Staged shows only staged changes (git diff --cached)
	Staged bool

	// IncludeUntracked includes untracked files in the output
	IncludeUntracked bool

	// ContextLines is the number of context lines around changes (default: 3)
	ContextLines int

	// MaxDiffSize limits the diff size per repository in bytes (default: 100KB)
	MaxDiffSize int

	// IncludePattern is a regex pattern to include repositories
	IncludePattern string

	// ExcludePattern is a regex pattern to exclude repositories
	ExcludePattern string

	// IncludeSubmodules includes git submodules in the scan
	IncludeSubmodules bool

	// Verbose enables detailed logging
	Verbose bool

	// Logger for progress logging
	Logger Logger

	// ProgressCallback is called for each repository processed
	ProgressCallback func(current, total int, repo string)
}

// BulkDiffResult contains the results of a bulk diff operation.
type BulkDiffResult struct {
	// TotalScanned is the number of repositories found
	TotalScanned int

	// TotalWithChanges is the number of repositories with changes
	TotalWithChanges int

	// TotalClean is the number of repositories without changes
	TotalClean int

	// Repositories contains individual repository results
	Repositories []RepositoryDiffResult

	// Duration is the total operation time
	Duration time.Duration

	// Summary contains status counts
	Summary map[string]int
}

// RepositoryDiffResult contains the diff result for a single repository.
type RepositoryDiffResult struct {
	// Path is the repository path
	Path string

	// RelativePath is the path relative to scan root
	RelativePath string

	// Branch is the current branch
	Branch string

	// Status is the operation status (has-changes, clean, error)
	Status string

	// DiffContent is the actual diff output
	DiffContent string

	// DiffSummary is a short summary of changes
	DiffSummary string

	// FilesChanged is the number of files changed
	FilesChanged int

	// Additions is the number of lines added
	Additions int

	// Deletions is the number of lines deleted
	Deletions int

	// ChangedFiles is the list of changed files with their status
	ChangedFiles []ChangedFile

	// UntrackedFiles is the list of untracked files
	UntrackedFiles []string

	// Truncated indicates if the diff was truncated due to size limits
	Truncated bool

	// Error if the operation failed
	Error error

	// Duration is the operation time for this repository
	Duration time.Duration
}

// GetStatus returns the status for summary calculation.
func (r RepositoryDiffResult) GetStatus() string { return r.Status }

// ChangedFile represents a changed file with its status.
type ChangedFile struct {
	// Path is the file path
	Path string

	// Status is the change status (M=modified, A=added, D=deleted, R=renamed, etc.)
	Status string

	// OldPath is the old path for renamed files
	OldPath string
}

// BulkDiff scans for repositories and gets diffs in parallel.
func (c *client) BulkDiff(ctx context.Context, opts BulkDiffOptions) (*BulkDiffResult, error) {
	startTime := time.Now()

	// Set defaults
	if opts.ContextLines == 0 {
		opts.ContextLines = 3
	}
	if opts.MaxDiffSize == 0 {
		opts.MaxDiffSize = 100 * 1024 // 100KB default
	}

	// Initialize common settings
	common, err := initializeBulkOperation(
		opts.Directory,
		opts.Parallel,
		opts.MaxDepth,
		opts.IncludeSubmodules,
		opts.IncludePattern,
		opts.ExcludePattern,
		opts.Logger,
	)
	if err != nil {
		return nil, err
	}

	// Scan and filter repositories
	filteredRepos, totalScanned, err := c.scanAndFilterRepositories(ctx, common)
	if err != nil {
		return nil, err
	}

	result := &BulkDiffResult{
		TotalScanned: totalScanned,
		Repositories: make([]RepositoryDiffResult, 0, len(filteredRepos)),
		Summary:      make(map[string]int),
	}

	if len(filteredRepos) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Process repositories in parallel
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, common.Parallel)

	for i, repoPath := range filteredRepos {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if opts.ProgressCallback != nil {
				opts.ProgressCallback(idx+1, len(filteredRepos), path)
			}

			repoResult := c.getRepositoryDiff(ctx, common.Directory, path, opts)

			mu.Lock()
			result.Repositories = append(result.Repositories, repoResult)
			switch repoResult.Status {
			case "has-changes":
				result.TotalWithChanges++
			case "clean":
				result.TotalClean++
			}
			mu.Unlock()
		}(i, repoPath)
	}
	wg.Wait()

	result.Duration = time.Since(startTime)
	c.updateDiffSummary(result)

	return result, nil
}

// getRepositoryDiff gets the diff for a single repository.
//
//nolint:gocognit // TODO: Refactor diff logic into smaller functions
func (c *client) getRepositoryDiff(ctx context.Context, rootDir, repoPath string, opts BulkDiffOptions) RepositoryDiffResult {
	startTime := time.Now()

	relPath, err := filepath.Rel(rootDir, repoPath)
	if err != nil {
		relPath = repoPath
	}
	if relPath == "." {
		relPath = filepath.Base(rootDir)
	}

	result := RepositoryDiffResult{
		Path:           repoPath,
		RelativePath:   relPath,
		Status:         "clean",
		ChangedFiles:   []ChangedFile{},
		UntrackedFiles: []string{},
	}

	// Get branch
	branchResult, err := c.executor.Run(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		result.Branch = strings.TrimSpace(branchResult.Stdout)
	}

	// Get status to identify changed files
	statusResult, err := c.executor.Run(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		result.Status = "error"
		result.Error = err
		result.Duration = time.Since(startTime)
		return result
	}

	// Parse status output
	lines := strings.Split(statusResult.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) < 3 {
			continue
		}

		statusCode := line[:2]
		filePath := strings.TrimLeft(line[2:], " \t")

		// Handle renamed files (R oldpath -> newpath)
		if strings.Contains(filePath, " -> ") {
			parts := strings.Split(filePath, " -> ")
			if len(parts) == 2 {
				result.ChangedFiles = append(result.ChangedFiles, ChangedFile{
					Path:    parts[1],
					OldPath: parts[0],
					Status:  "R",
				})
				continue
			}
		}

		// Untracked files
		if statusCode == "??" {
			result.UntrackedFiles = append(result.UntrackedFiles, filePath)
			continue
		}

		// Parse status code
		status := parseGitStatus(statusCode)
		result.ChangedFiles = append(result.ChangedFiles, ChangedFile{
			Path:   filePath,
			Status: status,
		})
	}

	result.FilesChanged = len(result.ChangedFiles)

	// If no changes, mark as clean
	if result.FilesChanged == 0 && len(result.UntrackedFiles) == 0 {
		result.Status = "clean"
		result.Duration = time.Since(startTime)
		return result
	}

	result.Status = "has-changes"

	// Build diff command
	diffArgs := []string{"diff"}
	if opts.Staged {
		diffArgs = append(diffArgs, "--cached")
	}
	diffArgs = append(diffArgs, fmt.Sprintf("--unified=%d", opts.ContextLines))

	// Get diff content
	diffResult, err := c.executor.Run(ctx, repoPath, diffArgs...)
	if err != nil {
		result.Error = fmt.Errorf("failed to get diff: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	result.DiffContent = diffResult.Stdout

	// Truncate if too large
	if len(result.DiffContent) > opts.MaxDiffSize {
		result.DiffContent = result.DiffContent[:opts.MaxDiffSize]
		result.Truncated = true
	}

	// Get diff stats
	statsArgs := []string{"diff", "--stat"}
	if opts.Staged {
		statsArgs = append(statsArgs, "--cached")
	}
	statsResult, err := c.executor.Run(ctx, repoPath, statsArgs...)
	if err == nil {
		result.Additions, result.Deletions = parseDiffStats(statsResult.Stdout)
		result.DiffSummary = extractDiffSummaryLine(statsResult.Stdout)
	}

	// Also get unstaged diff if not in staged-only mode
	if !opts.Staged && !result.Truncated {
		unstagedResult, err := c.executor.Run(ctx, repoPath, "diff")
		if err == nil && unstagedResult.Stdout != "" {
			if result.DiffContent != "" {
				result.DiffContent += "\n"
			}
			unstagedDiff := unstagedResult.Stdout
			remainingSize := opts.MaxDiffSize - len(result.DiffContent)
			if remainingSize <= 0 {
				result.Truncated = true
			} else if len(unstagedDiff) > remainingSize {
				unstagedDiff = unstagedDiff[:remainingSize]
				result.Truncated = true
				result.DiffContent += unstagedDiff
			} else {
				result.DiffContent += unstagedDiff
			}
		}
	}

	// Include untracked file contents if requested
	if opts.IncludeUntracked && len(result.UntrackedFiles) > 0 {
		for _, file := range result.UntrackedFiles {
			if result.Truncated {
				break
			}
			// Show untracked file as new file diff
			catResult, err := c.executor.Run(ctx, repoPath, "show", ":"+file)
			if err != nil {
				// File not in index, use cat-like approach
				continue
			}
			untrackedDiff := fmt.Sprintf("\n--- /dev/null\n+++ b/%s\n@@ -0,0 +1 @@\n+%s\n", file, catResult.Stdout)
			if len(result.DiffContent)+len(untrackedDiff) > opts.MaxDiffSize {
				result.Truncated = true
				break
			}
			result.DiffContent += untrackedDiff
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// parseGitStatus converts git status codes to readable status.
func parseGitStatus(code string) string {
	// First character is index status, second is worktree status
	indexStatus := code[0]
	worktreeStatus := code[1]

	// Prioritize index status for staged files
	switch indexStatus {
	case 'M':
		return "M" // Modified
	case 'A':
		return "A" // Added
	case 'D':
		return "D" // Deleted
	case 'R':
		return "R" // Renamed
	case 'C':
		return "C" // Copied
	}

	// Fall back to worktree status
	switch worktreeStatus {
	case 'M':
		return "M"
	case 'D':
		return "D"
	}

	return "?" // Unknown
}

// extractDiffSummaryLine extracts the summary line from git diff --stat output.
func extractDiffSummaryLine(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "changed") {
			return line
		}
	}
	return ""
}

// updateDiffSummary updates the summary counts.
func (c *client) updateDiffSummary(result *BulkDiffResult) {
	for _, repo := range result.Repositories {
		result.Summary[repo.Status]++
	}
}
