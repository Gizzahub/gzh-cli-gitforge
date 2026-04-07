// Copyright (c) 2026 Archmagece
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// BulkCleanOptions configures bulk git clean operations.
type BulkCleanOptions struct {
	// Directory is the root directory to scan for repositories
	Directory string

	// Parallel is the number of concurrent workers (default: 10)
	Parallel int

	// MaxDepth is the maximum directory depth to scan (default: 10)
	MaxDepth int

	// DryRun performs simulation without actual deletion (default: true)
	DryRun bool

	// RemoveDirectories also removes untracked directories (-d flag)
	RemoveDirectories bool

	// RemoveIgnored removes ignored files in addition to untracked (-x flag)
	RemoveIgnored bool

	// OnlyIgnored removes only ignored files, not untracked (-X flag)
	OnlyIgnored bool

	// ExcludePatterns are patterns to exclude from cleaning (-e flag)
	ExcludePatterns []string

	// IncludeSubmodules includes git submodules in the scan (default: false)
	IncludeSubmodules bool

	// IncludePattern is a regex pattern for repositories to include
	IncludePattern string

	// ExcludePattern is a regex pattern for repositories to exclude
	ExcludePattern string

	// Logger for operation feedback
	Logger Logger

	// ProgressCallback is called for each processed repository
	ProgressCallback func(current, total int, repo string)
}

// BulkCleanResult contains the results of a bulk clean operation.
type BulkCleanResult struct {
	// TotalScanned is the number of repositories found
	TotalScanned int

	// TotalProcessed is the number of repositories processed
	TotalProcessed int

	// Repositories contains individual repository results
	Repositories []RepositoryCleanResult

	// Duration is the total operation time
	Duration time.Duration

	// Summary contains status counts
	Summary map[string]int

	// TotalFiles is the total number of files removed/would-remove across all repos
	TotalFiles int
}

// RepositoryCleanResult represents the result for a single repository clean.
type RepositoryCleanResult struct {
	// Path is the repository path
	Path string

	// RelativePath is the path relative to scan root
	RelativePath string

	// Status is the operation status
	Status string

	// Message is a human-readable status message
	Message string

	// Error if the operation failed
	Error error

	// Duration is how long this repository took to process
	Duration time.Duration

	// Branch is the current branch name
	Branch string

	// FilesRemoved is the list of files removed or would be removed
	FilesRemoved []string

	// FilesCount is the number of files removed or would be removed
	FilesCount int
}

// GetStatus returns the status for summary calculation.
func (r RepositoryCleanResult) GetStatus() string { return r.Status }

// BulkClean scans for repositories and removes untracked/ignored files in parallel.
func (c *client) BulkClean(ctx context.Context, opts BulkCleanOptions) (*BulkCleanResult, error) {
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

	// Update opts with initialized values
	opts.Directory = common.Directory
	opts.Parallel = common.Parallel
	opts.MaxDepth = common.MaxDepth
	opts.Logger = common.Logger

	// Scan and filter repositories
	filteredRepos, totalScanned, err := c.scanAndFilterRepositories(ctx, common)
	if err != nil {
		return nil, err
	}

	// Handle empty result
	if len(filteredRepos) == 0 {
		return &BulkCleanResult{
			TotalScanned:   totalScanned,
			TotalProcessed: 0,
			Repositories:   []RepositoryCleanResult{},
			Duration:       time.Since(startTime),
			Summary:        map[string]int{},
		}, nil
	}

	// Process repositories in parallel
	results, err := c.processCleanRepositories(ctx, opts.Directory, filteredRepos, opts, common.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to process repositories: %w", err)
	}

	// Calculate summary and totals
	summary := calculateSummaryGeneric(results)
	totalFiles := 0
	for _, r := range results {
		totalFiles += r.FilesCount
	}

	return &BulkCleanResult{
		TotalScanned:   totalScanned,
		TotalProcessed: len(filteredRepos),
		Repositories:   results,
		Duration:       time.Since(startTime),
		Summary:        summary,
		TotalFiles:     totalFiles,
	}, nil
}

// processCleanRepositories processes repositories in parallel for cleaning.
func (c *client) processCleanRepositories(ctx context.Context, rootDir string, repos []string, opts BulkCleanOptions, logger Logger) ([]RepositoryCleanResult, error) {
	results := make([]RepositoryCleanResult, len(repos))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.Parallel)

	for i, repoPath := range repos {
		i, repoPath := i, repoPath

		g.Go(func() error {
			if opts.ProgressCallback != nil {
				opts.ProgressCallback(i+1, len(repos), repoPath)
			}

			result := c.processCleanRepository(gctx, rootDir, repoPath, opts, logger)
			results[i] = result

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// processCleanRepository processes a single repository clean operation.
func (c *client) processCleanRepository(ctx context.Context, rootDir, repoPath string, opts BulkCleanOptions, logger Logger) RepositoryCleanResult {
	startTime := time.Now()

	result := RepositoryCleanResult{
		Path:         repoPath,
		RelativePath: getRelativePath(rootDir, repoPath),
		FilesRemoved: []string{},
	}

	// Open repository
	repo, err := c.Open(ctx, repoPath)
	if err != nil {
		result.Status = StatusError
		result.Message = "Failed to open repository"
		result.Error = err
		result.Duration = time.Since(startTime)
		return result
	}

	// Get repository info for branch name
	info, err := c.GetInfo(ctx, repo)
	if err == nil {
		result.Branch = info.Branch
	}

	// Build git clean arguments
	args := []string{"clean"}
	if opts.DryRun {
		args = append(args, "-n")
	} else {
		args = append(args, "-f")
	}

	if opts.RemoveDirectories {
		args = append(args, "-d")
	}

	if opts.OnlyIgnored {
		args = append(args, "-X")
	} else if opts.RemoveIgnored {
		args = append(args, "-x")
	}

	for _, pattern := range opts.ExcludePatterns {
		args = append(args, "-e", pattern)
	}

	// Execute git clean with forced English locale for parseable output
	cleanEnv := []string{"LC_ALL=C"}
	cleanResult, err := c.executor.RunWithEnv(ctx, repoPath, cleanEnv, args...)
	if err != nil {
		result.Status = StatusError
		result.Message = "Failed to run git clean"
		result.Error = err
		result.Duration = time.Since(startTime)
		return result
	}

	if cleanResult.ExitCode != 0 {
		result.Status = StatusError
		result.Message = "git clean failed"
		result.Error = fmt.Errorf("exit code %d: %s", cleanResult.ExitCode, cleanResult.Stderr)
		result.Duration = time.Since(startTime)
		return result
	}

	// Parse output: "Would remove ..." or "Removing ..."
	files := parseCleanOutput(cleanResult.Stdout)
	result.FilesRemoved = files
	result.FilesCount = len(files)

	// Determine status
	switch {
	case len(files) == 0:
		result.Status = StatusNothingToClean
		result.Message = "Nothing to clean"
	case opts.DryRun:
		result.Status = StatusWouldClean
		result.Message = fmt.Sprintf("Would remove %d file(s)", len(files))
	default:
		result.Status = StatusCleaned
		result.Message = fmt.Sprintf("Removed %d file(s)", len(files))
	}

	result.Duration = time.Since(startTime)

	if len(files) > 0 {
		logger.Info("repository cleaned",
			"path", result.RelativePath,
			"files", result.FilesCount,
			"dry-run", opts.DryRun)
	}

	return result
}

// parseCleanOutput extracts file names from git clean output.
// Lines are formatted as "Would remove X" (dry-run) or "Removing X" (force).
func parseCleanOutput(output string) []string {
	var files []string
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// git clean outputs "Would remove X" or "Removing X"
		if after, ok := strings.CutPrefix(line, "Would remove "); ok {
			files = append(files, strings.TrimSpace(after))
		} else if after, ok := strings.CutPrefix(line, "Removing "); ok {
			files = append(files, strings.TrimSpace(after))
		}
	}
	return files
}
