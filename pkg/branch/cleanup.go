// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package branch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// CleanupService analyzes and cleans up branches.
type CleanupService interface {
	// Analyze analyzes branches for cleanup.
	Analyze(ctx context.Context, repo *repository.Repository, opts AnalyzeOptions) (*CleanupReport, error)

	// Execute performs cleanup based on report.
	Execute(ctx context.Context, repo *repository.Repository, report *CleanupReport, opts ExecuteOptions) error
}

// cleanupService implements CleanupService.
type cleanupService struct {
	executor      *gitcmd.Executor
	branchManager BranchManager
}

// NewCleanupService creates a new CleanupService.
func NewCleanupService() CleanupService {
	return &cleanupService{
		executor:      gitcmd.NewExecutor(),
		branchManager: NewManager(),
	}
}

// NewCleanupServiceWithDeps creates a new CleanupService with custom dependencies.
func NewCleanupServiceWithDeps(executor *gitcmd.Executor, branchManager BranchManager) CleanupService {
	return &cleanupService{
		executor:      executor,
		branchManager: branchManager,
	}
}

// Analyze analyzes branches for cleanup.
func (c *cleanupService) Analyze(ctx context.Context, repo *repository.Repository, opts AnalyzeOptions) (*CleanupReport, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	// Set defaults
	if opts.StaleThreshold == 0 {
		opts.StaleThreshold = 30 * 24 * time.Hour // 30 days
	}

	if opts.BaseBranch == "" {
		// Try to detect base branch
		baseBranch, err := c.detectBaseBranch(ctx, repo)
		if err == nil {
			opts.BaseBranch = baseBranch
		} else {
			opts.BaseBranch = "main" // Default fallback
		}
	}

	// Get all branches
	branches, err := c.branchManager.List(ctx, repo, ListOptions{
		All: opts.IncludeRemote,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	report := &CleanupReport{
		Merged:    make([]*Branch, 0),
		Stale:     make([]*Branch, 0),
		Orphaned:  make([]*Branch, 0),
		Protected: make([]*Branch, 0),
		Total:     len(branches),
	}

	// Analyze each branch
	for _, branch := range branches {
		// Skip current branch
		if branch.IsHead {
			continue
		}

		// Check if protected
		if c.isProtectedBranch(branch.Name, opts.Exclude) {
			report.Protected = append(report.Protected, branch)
			continue
		}

		// Check if merged
		if opts.IncludeMerged {
			if merged, err := c.isBranchMerged(ctx, repo, branch.Name, opts.BaseBranch); err == nil && merged {
				report.Merged = append(report.Merged, branch)
				continue
			}
		}

		// Check if stale
		if opts.IncludeStale {
			if stale, err := c.isBranchStale(ctx, repo, branch.Name, opts.StaleThreshold); err == nil && stale {
				report.Stale = append(report.Stale, branch)
				continue
			}
		}

		// Check if orphaned (remote tracking branch with no remote)
		if branch.IsRemote {
			if orphaned, err := c.isBranchOrphaned(ctx, repo, branch.Name); err == nil && orphaned {
				report.Orphaned = append(report.Orphaned, branch)
			}
		}
	}

	return report, nil
}

// Execute performs cleanup based on report.
func (c *cleanupService) Execute(ctx context.Context, repo *repository.Repository, report *CleanupReport, opts ExecuteOptions) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	if report == nil {
		return fmt.Errorf("cleanup report cannot be nil")
	}

	// Collect all branches to delete
	toDelete := make([]*Branch, 0)
	toDelete = append(toDelete, report.Merged...)
	toDelete = append(toDelete, report.Stale...)
	toDelete = append(toDelete, report.Orphaned...)

	// Filter out excluded branches
	if len(opts.Exclude) > 0 {
		filtered := make([]*Branch, 0)
		for _, branch := range toDelete {
			if !c.isProtectedBranch(branch.Name, opts.Exclude) {
				filtered = append(filtered, branch)
			}
		}
		toDelete = filtered
	}

	// Dry run - just return
	if opts.DryRun {
		return nil
	}

	// Delete branches
	for _, branch := range toDelete {
		deleteOpts := DeleteOptions{
			Name:    branch.Name,
			Force:   opts.Force,
			Remote:  opts.Remote && branch.IsRemote,
			Confirm: opts.Confirm,
		}

		if err := c.branchManager.Delete(ctx, repo, deleteOpts); err != nil {
			// Log error but continue with other branches
			// In a real implementation, we'd use a logger here
			continue
		}
	}

	return nil
}

// detectBaseBranch detects the main/master branch.
func (c *cleanupService) detectBaseBranch(ctx context.Context, repo *repository.Repository) (string, error) {
	// Try common base branches in order
	candidates := []string{"main", "master", "develop", "development"}

	for _, branch := range candidates {
		exists, err := c.branchManager.Exists(ctx, repo, branch)
		if err == nil && exists {
			return branch, nil
		}
	}

	return "", fmt.Errorf("could not detect base branch")
}

// isBranchMerged checks if a branch is fully merged into base.
func (c *cleanupService) isBranchMerged(ctx context.Context, repo *repository.Repository, branch, base string) (bool, error) {
	// Run git branch --merged base
	result, err := c.executor.Run(ctx, repo.Path, "branch", "--merged", base)
	if err != nil {
		return false, err
	}

	// Parse output
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		if line == branch {
			return true, nil
		}
	}

	return false, nil
}

// isBranchStale checks if a branch has no recent activity.
func (c *cleanupService) isBranchStale(ctx context.Context, repo *repository.Repository, branch string, threshold time.Duration) (bool, error) {
	// Get last commit date
	result, err := c.executor.Run(ctx, repo.Path, "log", "-1", "--format=%ct", branch)
	if err != nil {
		return false, err
	}

	// Parse timestamp
	var timestamp int64
	if _, err := fmt.Sscanf(strings.TrimSpace(result.Stdout), "%d", &timestamp); err != nil {
		return false, err
	}

	// Check if older than threshold
	lastCommit := time.Unix(timestamp, 0)
	age := time.Since(lastCommit)

	return age > threshold, nil
}

// isBranchOrphaned checks if a remote tracking branch has no remote.
func (c *cleanupService) isBranchOrphaned(ctx context.Context, repo *repository.Repository, branch string) (bool, error) {
	// Remote branches should start with "remotes/"
	if !strings.HasPrefix(branch, "remotes/") {
		return false, nil
	}

	// Extract remote name (e.g., "remotes/origin/feature" -> "origin")
	parts := strings.SplitN(strings.TrimPrefix(branch, "remotes/"), "/", 2)
	if len(parts) < 2 {
		return false, nil
	}

	remoteName := parts[0]

	// Check if remote exists
	result, err := c.executor.Run(ctx, repo.Path, "remote")
	if err != nil {
		return false, err
	}

	// Parse remotes
	remotes := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	for _, remote := range remotes {
		if strings.TrimSpace(remote) == remoteName {
			return false, nil // Remote exists, not orphaned
		}
	}

	return true, nil // Remote doesn't exist, orphaned
}

// isProtectedBranch checks if a branch is protected.
func (c *cleanupService) isProtectedBranch(branch string, additionalPatterns []string) bool {
	// Check built-in protected branches
	if IsProtected(branch) {
		return true
	}

	// Check additional patterns
	for _, pattern := range additionalPatterns {
		if matchPattern(branch, pattern) {
			return true
		}
	}

	return false
}

// CountBranches returns the total number of branches in the report.
func (r *CleanupReport) CountBranches() int {
	return len(r.Merged) + len(r.Stale) + len(r.Orphaned)
}

// IsEmpty checks if the report has no branches to clean up.
func (r *CleanupReport) IsEmpty() bool {
	return r.CountBranches() == 0
}

// GetAllBranches returns all branches eligible for cleanup.
func (r *CleanupReport) GetAllBranches() []*Branch {
	all := make([]*Branch, 0)
	all = append(all, r.Merged...)
	all = append(all, r.Stale...)
	all = append(all, r.Orphaned...)
	return all
}
