// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// BulkTagOptions configures bulk tag operations.
type BulkTagOptions struct {
	// Directory is the root directory to scan for repositories
	Directory string

	// Parallel is the number of concurrent workers (default: 5)
	Parallel int

	// MaxDepth is the maximum directory depth to scan (default: 5)
	MaxDepth int

	// DryRun performs simulation without actual changes
	DryRun bool

	// Operation is the tag operation: "create", "list", "push", "status"
	Operation string

	// TagName is the tag name (for create operation)
	TagName string

	// Message is the tag message (creates annotated tag)
	Message string

	// Force overwrites existing tags
	Force bool

	// PushAll pushes all tags (for push operation)
	PushAll bool

	// IncludeSubmodules includes git submodules in the scan
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

// BulkTagResult contains the results of a bulk tag operation.
type BulkTagResult struct {
	// TotalScanned is the number of repositories found
	TotalScanned int

	// TotalProcessed is the number of repositories processed
	TotalProcessed int

	// Repositories contains individual repository results
	Repositories []RepositoryTagResult

	// Duration is the total operation time
	Duration time.Duration

	// Summary contains status counts
	Summary map[string]int

	// TotalTagCount is the total number of tags affected/found
	TotalTagCount int
}

// RepositoryTagResult represents the result for a single repository tag operation.
type RepositoryTagResult struct {
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

	// TagCount is the number of tags in the repository
	TagCount int

	// LatestTag is the latest tag name
	LatestTag string

	// CreatedTag is the tag that was created
	CreatedTag string
}

// GetStatus returns the status for summary calculation.
func (r RepositoryTagResult) GetStatus() string { return r.Status }

// Status constants for tag operations.
const (
	StatusTagCreated     = "tag-created"
	StatusTagPushed      = "tag-pushed"
	StatusTagExists      = "tag-exists"
	StatusWouldCreateTag = "would-create"
	StatusWouldPushTag   = "would-push"
	StatusNoTags         = "no-tags"
	StatusHasTags        = "has-tags"
)

// BulkTag scans for repositories and performs tag operations in parallel.
func (c *client) BulkTag(ctx context.Context, opts BulkTagOptions) (*BulkTagResult, error) {
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
		return &BulkTagResult{
			TotalScanned:   totalScanned,
			TotalProcessed: 0,
			Repositories:   []RepositoryTagResult{},
			Duration:       time.Since(startTime),
			Summary:        map[string]int{},
		}, nil
	}

	// Process repositories in parallel
	results, err := c.processTagRepositories(ctx, opts.Directory, filteredRepos, opts, common.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to process repositories: %w", err)
	}

	// Calculate summary and totals
	summary := calculateTagSummary(results)
	totalTagCount := 0
	for _, r := range results {
		totalTagCount += r.TagCount
	}

	return &BulkTagResult{
		TotalScanned:   totalScanned,
		TotalProcessed: len(filteredRepos),
		Repositories:   results,
		Duration:       time.Since(startTime),
		Summary:        summary,
		TotalTagCount:  totalTagCount,
	}, nil
}

// processTagRepositories processes repositories in parallel for tag operations.
func (c *client) processTagRepositories(ctx context.Context, rootDir string, repos []string, opts BulkTagOptions, logger Logger) ([]RepositoryTagResult, error) {
	results := make([]RepositoryTagResult, len(repos))
	var mu sync.Mutex

	// Create error group with concurrency limit
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.Parallel)

	for i, repoPath := range repos {
		i, repoPath := i, repoPath // capture loop variables

		g.Go(func() error {
			// Call progress callback
			if opts.ProgressCallback != nil {
				opts.ProgressCallback(i+1, len(repos), repoPath)
			}

			result := c.processTagRepository(gctx, rootDir, repoPath, opts, logger)

			mu.Lock()
			results[i] = result
			mu.Unlock()

			return nil // Don't fail entire operation on single repo error
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// processTagRepository processes a single repository tag operation.
func (c *client) processTagRepository(ctx context.Context, rootDir, repoPath string, opts BulkTagOptions, logger Logger) RepositoryTagResult {
	startTime := time.Now()

	result := RepositoryTagResult{
		Path:         repoPath,
		RelativePath: getRelativePath(rootDir, repoPath),
		Duration:     0,
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

	// Get repository info
	info, err := c.GetInfo(ctx, repo)
	if err == nil {
		result.Branch = info.Branch
	}

	// Get tag count
	tagResult, _ := c.executor.Run(ctx, repoPath, "tag", "-l")
	if tagResult.ExitCode == 0 {
		lines := strings.Split(strings.TrimSpace(tagResult.Stdout), "\n")
		if lines[0] != "" {
			result.TagCount = len(lines)
		}
	}

	// Get latest tag
	latestResult, _ := c.executor.Run(ctx, repoPath, "describe", "--tags", "--abbrev=0")
	if latestResult.ExitCode == 0 {
		result.LatestTag = strings.TrimSpace(latestResult.Stdout)
	}

	switch opts.Operation {
	case "create":
		result = c.processTagCreate(ctx, repoPath, opts, result, logger)
	case "push":
		result = c.processTagPush(ctx, repoPath, opts, result, logger)
	case "list", "status":
		result = c.processTagList(ctx, repoPath, opts, result, logger)
	default:
		result.Status = StatusError
		result.Message = fmt.Sprintf("Unknown operation: %s", opts.Operation)
	}

	result.Duration = time.Since(startTime)
	return result
}

// processTagCreate handles tag create operation.
func (c *client) processTagCreate(ctx context.Context, repoPath string, opts BulkTagOptions, result RepositoryTagResult, logger Logger) RepositoryTagResult {
	if opts.TagName == "" {
		result.Status = StatusError
		result.Message = "Tag name is required"
		return result
	}

	// Check if tag already exists
	checkResult, _ := c.executor.Run(ctx, repoPath, "rev-parse", "--verify", "refs/tags/"+opts.TagName)
	if checkResult.ExitCode == 0 && !opts.Force {
		result.Status = StatusTagExists
		result.Message = fmt.Sprintf("Tag %s already exists", opts.TagName)
		return result
	}

	// Dry run - just report
	if opts.DryRun {
		result.Status = StatusWouldCreateTag
		result.Message = fmt.Sprintf("Would create tag %s", opts.TagName)
		result.CreatedTag = opts.TagName
		return result
	}

	// Execute tag create
	args := []string{"tag"}
	if opts.Message != "" {
		args = append(args, "-a", "-m", opts.Message)
	}
	if opts.Force {
		args = append(args, "-f")
	}
	args = append(args, opts.TagName)

	createResult, err := c.executor.Run(ctx, repoPath, args...)
	if err != nil || createResult.ExitCode != 0 {
		result.Status = StatusError
		result.Message = "Failed to create tag"
		if createResult.Stderr != "" {
			result.Error = fmt.Errorf("%s", createResult.Stderr)
		}
		return result
	}

	result.Status = StatusTagCreated
	result.Message = fmt.Sprintf("Created tag %s", opts.TagName)
	result.CreatedTag = opts.TagName
	result.TagCount++

	logger.Info("tag created", "path", result.RelativePath, "tag", opts.TagName)
	return result
}

// processTagPush handles tag push operation.
func (c *client) processTagPush(ctx context.Context, repoPath string, opts BulkTagOptions, result RepositoryTagResult, logger Logger) RepositoryTagResult {
	if result.TagCount == 0 {
		result.Status = StatusNoTags
		result.Message = "No tags to push"
		return result
	}

	// Dry run - just report
	if opts.DryRun {
		result.Status = StatusWouldPushTag
		result.Message = fmt.Sprintf("Would push %d tag(s)", result.TagCount)
		return result
	}

	// Execute tag push
	args := []string{"push", "origin"}
	if opts.PushAll {
		args = append(args, "--tags")
	} else if opts.TagName != "" {
		args = append(args, opts.TagName)
	} else {
		args = append(args, "--tags")
	}

	pushResult, err := c.executor.Run(ctx, repoPath, args...)
	if err != nil || pushResult.ExitCode != 0 {
		result.Status = StatusError
		result.Message = "Failed to push tags"
		if pushResult.Stderr != "" {
			result.Error = fmt.Errorf("%s", pushResult.Stderr)
		}
		return result
	}

	result.Status = StatusTagPushed
	result.Message = "Tags pushed"

	logger.Info("tags pushed", "path", result.RelativePath)
	return result
}

// processTagList handles tag list/status operation.
func (c *client) processTagList(ctx context.Context, repoPath string, opts BulkTagOptions, result RepositoryTagResult, logger Logger) RepositoryTagResult {
	if result.TagCount == 0 {
		result.Status = StatusNoTags
		result.Message = "No tags"
		return result
	}

	result.Status = StatusHasTags
	result.Message = fmt.Sprintf("%d tag(s), latest: %s", result.TagCount, result.LatestTag)
	return result
}

// calculateTagSummary creates a summary of tag results by status.
func calculateTagSummary(results []RepositoryTagResult) map[string]int {
	return calculateSummaryGeneric(results)
}
