// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BulkCommitOptions configures bulk commit operations.
type BulkCommitOptions struct {
	// Directory is the root directory to scan for repositories
	Directory string

	// Parallel is the number of concurrent workers (default: 5)
	Parallel int

	// MaxDepth is the maximum directory depth to scan (default: 1)
	MaxDepth int

	// DryRun shows what would be committed without actually committing
	DryRun bool

	// Message is a common message for all repositories (overrides auto-generation)
	Message string

	// Yes auto-approves without confirmation
	Yes bool

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

	// MessageGenerator generates commit messages for repositories
	// If nil, a simple default message is generated
	MessageGenerator func(ctx context.Context, repoPath string, files []string) (string, error)
}

// BulkCommitResult contains the results of a bulk commit operation.
type BulkCommitResult struct {
	// TotalScanned is the number of repositories found
	TotalScanned int

	// TotalDirty is the number of repositories with uncommitted changes
	TotalDirty int

	// TotalCommitted is the number of repositories successfully committed
	TotalCommitted int

	// TotalFailed is the number of repositories that failed to commit
	TotalFailed int

	// TotalSkipped is the number of repositories skipped (clean or excluded)
	TotalSkipped int

	// Repositories contains individual repository results
	Repositories []RepositoryCommitResult

	// Duration is the total operation time
	Duration time.Duration

	// Summary contains status counts
	Summary map[string]int
}

// RepositoryCommitResult contains the result for a single repository commit.
type RepositoryCommitResult struct {
	// Path is the repository path
	Path string

	// RelativePath is the path relative to scan root
	RelativePath string

	// Branch is the current branch
	Branch string

	// Status is the operation status (success, skipped, error, would-commit)
	Status string

	// CommitHash is the commit hash if successful
	CommitHash string

	// Message is the commit message used
	Message string

	// SuggestedMessage is the auto-generated message (for preview)
	SuggestedMessage string

	// FilesChanged is the number of files changed
	FilesChanged int

	// Additions is the number of lines added
	Additions int

	// Deletions is the number of lines deleted
	Deletions int

	// ChangedFiles is the list of changed files
	ChangedFiles []string

	// Error if the operation failed
	Error error

	// Duration is the operation time for this repository
	Duration time.Duration
}

// GetStatus returns the status for summary calculation.
func (r RepositoryCommitResult) GetStatus() string { return r.Status }

// BulkCommit scans for repositories and commits changes in parallel.
func (c *client) BulkCommit(ctx context.Context, opts BulkCommitOptions) (*BulkCommitResult, error) {
	startTime := time.Now()

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

	result := &BulkCommitResult{
		TotalScanned: totalScanned,
		Repositories: make([]RepositoryCommitResult, 0, len(filteredRepos)),
		Summary:      make(map[string]int),
	}

	if len(filteredRepos) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Phase 1: Collect status for all dirty repositories
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

			repoResult := c.analyzeRepositoryForCommit(ctx, common.Directory, path, opts)

			mu.Lock()
			result.Repositories = append(result.Repositories, repoResult)
			mu.Unlock()
		}(i, repoPath)
	}
	wg.Wait()

	// Count dirty repositories
	for _, repo := range result.Repositories {
		if repo.Status == "dirty" || repo.Status == "would-commit" {
			result.TotalDirty++
		}
	}

	// If dry-run, we're done with analysis
	if opts.DryRun {
		for i := range result.Repositories {
			if result.Repositories[i].Status == "dirty" {
				result.Repositories[i].Status = "would-commit"
			}
		}
		result.Duration = time.Since(startTime)
		c.updateCommitSummary(result)
		return result, nil
	}

	// Phase 2: Commit dirty repositories (if not dry-run)
	for i, repoPath := range filteredRepos {
		// Find the corresponding result
		var repoResult *RepositoryCommitResult
		for j := range result.Repositories {
			if result.Repositories[j].Path == repoPath {
				repoResult = &result.Repositories[j]
				break
			}
		}

		if repoResult == nil || repoResult.Status != "dirty" {
			continue
		}

		wg.Add(1)
		go func(idx int, path string, res *RepositoryCommitResult) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			commitStart := time.Now()

			// Determine message
			message := opts.Message
			if message == "" {
				message = res.SuggestedMessage
			}
			if message == "" {
				message = fmt.Sprintf("chore: update %d files", res.FilesChanged)
			}

			// Execute commit
			hash, err := c.executeCommit(ctx, path, message)
			if err != nil {
				mu.Lock()
				res.Status = "error"
				res.Error = err
				result.TotalFailed++
				mu.Unlock()
			} else {
				mu.Lock()
				res.Status = "success"
				res.CommitHash = hash
				res.Message = message
				result.TotalCommitted++
				mu.Unlock()
			}

			res.Duration = time.Since(commitStart)
		}(i, repoPath, repoResult)
	}
	wg.Wait()

	// Calculate skipped
	result.TotalSkipped = result.TotalScanned - result.TotalDirty

	result.Duration = time.Since(startTime)
	c.updateCommitSummary(result)

	return result, nil
}

// analyzeRepositoryForCommit analyzes a repository for potential commit.
func (c *client) analyzeRepositoryForCommit(ctx context.Context, rootDir, repoPath string, opts BulkCommitOptions) RepositoryCommitResult {
	startTime := time.Now()

	relPath, err := filepath.Rel(rootDir, repoPath)
	if err != nil {
		relPath = repoPath
	}
	if relPath == "." {
		relPath = filepath.Base(rootDir)
	}

	result := RepositoryCommitResult{
		Path:         repoPath,
		RelativePath: relPath,
		Status:       "clean",
		ChangedFiles: []string{},
	}

	// Get branch
	branchResult, err := c.executor.Run(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		result.Branch = strings.TrimSpace(branchResult.Stdout)
	}

	// Get status (staged + unstaged)
	statusResult, err := c.executor.Run(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		result.Status = "error"
		result.Error = err
		result.Duration = time.Since(startTime)
		return result
	}

	// Parse status
	lines := strings.Split(strings.TrimSpace(statusResult.Stdout), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if len(line) >= 3 {
			file := strings.TrimSpace(line[3:])
			result.ChangedFiles = append(result.ChangedFiles, file)
		}
	}

	result.FilesChanged = len(result.ChangedFiles)

	if result.FilesChanged == 0 {
		result.Status = "clean"
		result.Duration = time.Since(startTime)
		return result
	}

	result.Status = "dirty"

	// Get diff stats
	diffResult, err := c.executor.Run(ctx, repoPath, "diff", "--stat", "--cached")
	if err == nil {
		result.Additions, result.Deletions = parseDiffStats(diffResult.Stdout)
	}

	// Also check unstaged changes
	diffUnstagedResult, err := c.executor.Run(ctx, repoPath, "diff", "--stat")
	if err == nil {
		additions, deletions := parseDiffStats(diffUnstagedResult.Stdout)
		result.Additions += additions
		result.Deletions += deletions
	}

	// Generate suggested message
	if opts.MessageGenerator != nil {
		msg, err := opts.MessageGenerator(ctx, repoPath, result.ChangedFiles)
		if err == nil {
			result.SuggestedMessage = msg
		}
	}

	if result.SuggestedMessage == "" {
		result.SuggestedMessage = c.generateSimpleCommitMessage(result.ChangedFiles)
	}

	result.Duration = time.Since(startTime)
	return result
}

// executeCommit executes git add and commit.
func (c *client) executeCommit(ctx context.Context, repoPath, message string) (string, error) {
	// Stage all changes
	_, err := c.executor.Run(ctx, repoPath, "add", "-A")
	if err != nil {
		return "", fmt.Errorf("failed to stage changes: %w", err)
	}

	// Create commit
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w\nOutput: %s", err, string(output))
	}

	// Get commit hash
	hashResult, err := c.executor.Run(ctx, repoPath, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", nil // Commit succeeded but couldn't get hash
	}

	return strings.TrimSpace(hashResult.Stdout), nil
}

// generateSimpleCommitMessage generates a simple commit message from file changes.
func (c *client) generateSimpleCommitMessage(files []string) string {
	if len(files) == 0 {
		return "chore: update files"
	}

	// Analyze files to infer type
	testFiles := 0
	docFiles := 0
	configFiles := 0

	for _, file := range files {
		lower := strings.ToLower(file)
		switch {
		case strings.Contains(lower, "test"):
			testFiles++
		case strings.HasSuffix(lower, ".md"), strings.Contains(lower, "readme"):
			docFiles++
		case strings.HasSuffix(lower, ".yaml"), strings.HasSuffix(lower, ".yml"),
			strings.HasSuffix(lower, ".json"), strings.HasSuffix(lower, ".toml"):
			configFiles++
		}
	}

	total := len(files)

	// Determine type
	commitType := "chore"
	if testFiles > total/2 {
		commitType = "test"
	} else if docFiles > total/2 {
		commitType = "docs"
	}

	// Infer scope from common directory
	scope := inferScopeFromFiles(files)

	// Generate description
	var desc string
	if len(files) == 1 {
		desc = fmt.Sprintf("update %s", filepath.Base(files[0]))
	} else {
		desc = fmt.Sprintf("update %d files", len(files))
	}

	if scope != "" {
		return fmt.Sprintf("%s(%s): %s", commitType, scope, desc)
	}
	return fmt.Sprintf("%s: %s", commitType, desc)
}

// inferScopeFromFiles infers a scope from file paths.
func inferScopeFromFiles(files []string) string {
	if len(files) == 0 {
		return ""
	}

	// Count first-level directories
	dirCount := make(map[string]int)
	for _, file := range files {
		parts := strings.Split(filepath.Dir(file), string(filepath.Separator))
		if len(parts) > 0 && parts[0] != "." && parts[0] != "" {
			dirCount[parts[0]]++
		}
	}

	// Find most common directory
	maxCount := 0
	scope := ""
	for dir, count := range dirCount {
		if count > maxCount {
			maxCount = count
			scope = dir
		}
	}

	// Clean up scope
	scope = strings.TrimPrefix(scope, "pkg/")
	scope = strings.TrimPrefix(scope, "internal/")
	scope = strings.TrimPrefix(scope, "cmd/")

	return scope
}

// parseDiffStats parses git diff --stat output.
func parseDiffStats(output string) (additions, deletions int) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "changed") {
			parts := strings.Split(line, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.Contains(part, "insertion") {
					_, _ = fmt.Sscanf(part, "%d", &additions) // Best effort parse
				}
				if strings.Contains(part, "deletion") {
					_, _ = fmt.Sscanf(part, "%d", &deletions) // Best effort parse
				}
			}
		}
	}
	return
}

// updateCommitSummary updates the summary counts.
func (c *client) updateCommitSummary(result *BulkCommitResult) {
	for _, repo := range result.Repositories {
		result.Summary[repo.Status]++
	}
}
