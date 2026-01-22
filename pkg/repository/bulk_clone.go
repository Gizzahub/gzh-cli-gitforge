// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DirectoryStructure defines how clone destinations are organized.
type DirectoryStructure string

const (
	// StructureFlat clones all repos directly into target directory.
	// Example: github.com/user/repo → ./repo/
	StructureFlat DirectoryStructure = "flat"

	// StructureUser organizes repos by user/org name.
	// Example: github.com/user/repo → ./user/repo/
	StructureUser DirectoryStructure = "user"
)

// BulkCloneOptions configures the bulk clone operation.
type BulkCloneOptions struct {
	// URLs is the list of repository URLs to clone.
	URLs []string

	// Directory is the base directory for cloning (default: current directory).
	Directory string

	// Structure determines directory organization (flat or user).
	Structure DirectoryStructure

	// Strategy determines how to handle existing repositories.
	// Values: "skip" (default), "pull", "reset", "rebase", "fetch"
	// This field takes precedence over Update if both are set.
	Strategy UpdateStrategy

	// Update pulls existing repositories instead of skipping.
	// Deprecated: Use Strategy instead. Will be removed in a future version.
	// When Update=true and Strategy is empty, it maps to Strategy="pull".
	Update bool

	// Branch is the branch to checkout after cloning.
	Branch string

	// Depth limits clone depth (0 = full clone).
	Depth int

	// Parallel is the number of concurrent operations.
	Parallel int

	// DryRun shows what would be done without doing it.
	DryRun bool

	// Verbose enables detailed output.
	Verbose bool

	// Logger for operation feedback.
	Logger Logger

	// ProgressCallback is called during bulk operations.
	ProgressCallback func(current, total int, repo string)
}

// resolveCloneStrategy resolves the effective strategy from Strategy and deprecated Update fields.
// Strategy field takes precedence. If Strategy is empty and Update is true, returns StrategyPull.
// Default is StrategySkip.
func resolveCloneStrategy(strategy UpdateStrategy, update bool) UpdateStrategy {
	// Strategy field takes precedence
	if strategy != "" {
		return strategy
	}
	// Backward compatibility: Update=true maps to pull
	if update {
		return StrategyPull
	}
	// Default: skip existing repos
	return StrategySkip
}

// BulkCloneResult contains the result of a bulk clone operation.
type BulkCloneResult struct {
	// TotalRequested is the number of URLs requested.
	TotalRequested int

	// TotalCloned is the number successfully cloned.
	TotalCloned int

	// TotalUpdated is the number successfully updated (when --update is used).
	TotalUpdated int

	// TotalSkipped is the number skipped (already exists, no --update).
	TotalSkipped int

	// TotalFailed is the number that failed.
	TotalFailed int

	// Duration is the total operation time.
	Duration time.Duration

	// Repositories contains individual results.
	Repositories []RepositoryCloneResult

	// Summary contains counts by status.
	Summary map[string]int
}

// RepositoryCloneResult represents the result for a single repository.
type RepositoryCloneResult struct {
	// URL is the repository URL.
	URL string

	// Path is the local path where it was cloned.
	Path string

	// RelativePath is the path relative to the base directory.
	RelativePath string

	// Status is the operation status.
	Status string // "cloned", "updated", "skipped", "error", "would-clone", "would-update"

	// Branch is the checked out branch.
	Branch string

	// Duration is how long the operation took.
	Duration time.Duration

	// Error is set if the operation failed.
	Error error
}

// GetStatus implements statusGetter for generic summary calculation.
func (r RepositoryCloneResult) GetStatus() string {
	return r.Status
}

// BulkClone clones multiple repositories in parallel.
func (c *client) BulkClone(ctx context.Context, opts BulkCloneOptions) (*BulkCloneResult, error) {
	startTime := time.Now()

	// Set defaults
	if opts.Parallel <= 0 {
		opts.Parallel = DefaultBulkParallel
	}
	if opts.Directory == "" {
		opts.Directory = "."
	}
	if opts.Structure == "" {
		opts.Structure = StructureFlat
	}

	logger := opts.Logger
	if logger == nil {
		logger = &noopLogger{}
	}

	result := &BulkCloneResult{
		TotalRequested: len(opts.URLs),
		Repositories:   make([]RepositoryCloneResult, 0, len(opts.URLs)),
	}

	if len(opts.URLs) == 0 {
		result.Duration = time.Since(startTime)
		result.Summary = make(map[string]int)
		return result, nil
	}

	// Create work channel and results channel
	type workItem struct {
		index int
		url   string
	}

	workChan := make(chan workItem, len(opts.URLs))
	resultsChan := make(chan RepositoryCloneResult, len(opts.URLs))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < opts.Parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workChan {
				select {
				case <-ctx.Done():
					resultsChan <- RepositoryCloneResult{
						URL:    work.url,
						Status: "skipped",
						Error:  ctx.Err(),
					}
					return
				default:
					res := c.cloneSingleRepo(ctx, work.url, opts, logger)
					resultsChan <- res
				}
			}
		}()
	}

	// Send work
	for i, url := range opts.URLs {
		if opts.ProgressCallback != nil {
			opts.ProgressCallback(i+1, len(opts.URLs), url)
		}
		workChan <- workItem{index: i, url: url}
	}
	close(workChan)

	// Collect results in background
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Gather results
	for res := range resultsChan {
		result.Repositories = append(result.Repositories, res)

		switch res.Status {
		case "cloned", "would-clone":
			result.TotalCloned++
		case "updated", "would-update":
			result.TotalUpdated++
		case "skipped":
			result.TotalSkipped++
		case "error":
			result.TotalFailed++
		}
	}

	result.Duration = time.Since(startTime)
	result.Summary = calculateSummaryGeneric(result.Repositories)

	return result, nil
}

// cloneSingleRepo clones or updates a single repository.
func (c *client) cloneSingleRepo(ctx context.Context, url string, opts BulkCloneOptions, logger Logger) RepositoryCloneResult {
	startTime := time.Now()

	// Determine destination path
	destination, err := c.resolveCloneDestination(url, opts.Directory, opts.Structure)
	if err != nil {
		return RepositoryCloneResult{
			URL:      url,
			Status:   "error",
			Duration: time.Since(startTime),
			Error:    err,
		}
	}

	relPath, _ := filepath.Rel(opts.Directory, destination)
	if relPath == "" {
		relPath = filepath.Base(destination)
	}

	// Determine strategy: Strategy field takes precedence over deprecated Update field
	strategy := resolveCloneStrategy(opts.Strategy, opts.Update)

	// Dry run mode
	if opts.DryRun {
		exists, isGitRepo, _ := checkTargetDirectory(destination)
		if exists && isGitRepo {
			if strategy != StrategySkip {
				return RepositoryCloneResult{
					URL:          url,
					Path:         destination,
					RelativePath: relPath,
					Status:       "would-update",
					Duration:     time.Since(startTime),
				}
			}
			return RepositoryCloneResult{
				URL:          url,
				Path:         destination,
				RelativePath: relPath,
				Status:       "skipped",
				Duration:     time.Since(startTime),
			}
		}
		return RepositoryCloneResult{
			URL:          url,
			Path:         destination,
			RelativePath: relPath,
			Status:       "would-clone",
			Duration:     time.Since(startTime),
		}
	}

	// Use CloneOrUpdate for the actual operation
	cloneOpts := CloneOrUpdateOptions{
		URL:         url,
		Destination: destination,
		Strategy:    strategy,
		Branch:      opts.Branch,
		Depth:       opts.Depth,
		Logger:      logger,
	}

	cloneResult, err := c.CloneOrUpdate(ctx, cloneOpts)
	if err != nil {
		return RepositoryCloneResult{
			URL:          url,
			Path:         destination,
			RelativePath: relPath,
			Status:       "error",
			Duration:     time.Since(startTime),
			Error:        err,
		}
	}

	// Map CloneOrUpdate action to our status
	status := cloneResult.Action
	branch := ""
	if cloneResult.Repository != nil {
		info, _ := c.GetInfo(ctx, cloneResult.Repository)
		if info != nil {
			branch = info.Branch
		}
	}

	return RepositoryCloneResult{
		URL:          url,
		Path:         destination,
		RelativePath: relPath,
		Status:       status,
		Branch:       branch,
		Duration:     time.Since(startTime),
	}
}

// resolveCloneDestination determines the destination path based on URL and structure.
func (c *client) resolveCloneDestination(url, baseDir string, structure DirectoryStructure) (string, error) {
	repoName, err := ExtractRepoNameFromURL(url)
	if err != nil {
		return "", err
	}

	switch structure {
	case StructureUser:
		// Extract user/org from URL
		userName := extractUserFromURL(url)
		if userName != "" {
			return filepath.Join(baseDir, userName, repoName), nil
		}
		// Fallback to flat if can't extract user
		return filepath.Join(baseDir, repoName), nil

	case StructureFlat:
		fallthrough
	default:
		return filepath.Join(baseDir, repoName), nil
	}
}

// extractUserFromURL extracts the user/organization name from a Git URL.
func extractUserFromURL(url string) string {
	url = strings.TrimSpace(url)

	// Remove .git suffix
	if strings.HasSuffix(url, ".git") {
		url = strings.TrimSuffix(url, ".git")
	}

	var pathPart string

	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		// https://github.com/user/repo → extract user/repo part
		parts := strings.Split(url, "/")
		if len(parts) >= 5 {
			// [https:, , github.com, user, repo]
			pathPart = parts[len(parts)-2] + "/" + parts[len(parts)-1]
		}
	} else if strings.Contains(url, "@") && strings.Contains(url, ":") {
		// git@github.com:user/repo
		colonIdx := strings.LastIndex(url, ":")
		if colonIdx > 0 && colonIdx < len(url)-1 {
			pathPart = url[colonIdx+1:]
		}
	}

	if pathPart == "" {
		return ""
	}

	// Split path to get user
	parts := strings.Split(pathPart, "/")
	if len(parts) >= 2 {
		return parts[0]
	}

	return ""
}
