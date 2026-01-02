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

// BulkBranchListOptions configures bulk branch list operations.
type BulkBranchListOptions struct {
	// Directory is the root directory to scan for repositories
	Directory string

	// Parallel is the number of concurrent workers (default: 5)
	Parallel int

	// MaxDepth is the maximum directory depth to scan (default: 5)
	MaxDepth int

	// All includes remote branches (-a/--all)
	All bool

	// Merged shows only merged branches
	Merged bool

	// Unmerged shows only unmerged branches
	Unmerged bool

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

// BulkBranchListResult contains the results of a bulk branch list operation.
type BulkBranchListResult struct {
	// TotalScanned is the number of repositories found
	TotalScanned int

	// TotalProcessed is the number of repositories processed
	TotalProcessed int

	// Repositories contains individual repository results
	Repositories []RepositoryBranchListResult

	// Duration is the total operation time
	Duration time.Duration

	// Summary contains status counts
	Summary map[string]int

	// TotalBranchCount is the total number of branches across all repos
	TotalBranchCount int

	// TotalLocalCount is the total number of local branches
	TotalLocalCount int

	// TotalRemoteCount is the total number of remote branches
	TotalRemoteCount int
}

// BranchInfo represents basic branch information for bulk operations.
type BranchInfo struct {
	Name     string // Branch name
	SHA      string // Commit SHA
	IsHead   bool   // Currently checked out
	IsRemote bool   // Remote branch
	Upstream string // Upstream branch (if set)
	AheadBy  int    // Commits ahead of upstream
	BehindBy int    // Commits behind upstream
}

// RepositoryBranchListResult represents branch list for a single repository.
type RepositoryBranchListResult struct {
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

	// CurrentBranch is the currently checked out branch
	CurrentBranch string

	// Branches is the list of branches
	Branches []BranchInfo

	// LocalCount is the number of local branches
	LocalCount int

	// RemoteCount is the number of remote branches
	RemoteCount int
}

// GetStatus returns the status for summary calculation.
func (r RepositoryBranchListResult) GetStatus() string { return r.Status }

// Status constants for branch list operations.
const (
	StatusBranchesListed = "listed"
	StatusNoBranches     = "no-branches"
)

// BulkBranchList scans for repositories and lists their branches in parallel.
func (c *client) BulkBranchList(ctx context.Context, opts BulkBranchListOptions) (*BulkBranchListResult, error) {
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
		return &BulkBranchListResult{
			TotalScanned:   totalScanned,
			TotalProcessed: 0,
			Repositories:   []RepositoryBranchListResult{},
			Duration:       time.Since(startTime),
			Summary:        map[string]int{},
		}, nil
	}

	// Process repositories in parallel
	results, err := c.processBranchListRepositories(ctx, opts.Directory, filteredRepos, opts, common.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to process repositories: %w", err)
	}

	// Calculate summary and totals
	summary := calculateBranchListSummary(results)
	totalBranches, totalLocal, totalRemote := aggregateBranchCounts(results)

	return &BulkBranchListResult{
		TotalScanned:     totalScanned,
		TotalProcessed:   len(filteredRepos),
		Repositories:     results,
		Duration:         time.Since(startTime),
		Summary:          summary,
		TotalBranchCount: totalBranches,
		TotalLocalCount:  totalLocal,
		TotalRemoteCount: totalRemote,
	}, nil
}

// processBranchListRepositories processes repositories in parallel for branch list operations.
func (c *client) processBranchListRepositories(ctx context.Context, rootDir string, repos []string, opts BulkBranchListOptions, logger Logger) ([]RepositoryBranchListResult, error) {
	results := make([]RepositoryBranchListResult, len(repos))
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

			result := c.processBranchListRepository(gctx, rootDir, repoPath, opts, logger)

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

// processBranchListRepository processes a single repository branch list operation.
func (c *client) processBranchListRepository(ctx context.Context, rootDir, repoPath string, opts BulkBranchListOptions, logger Logger) RepositoryBranchListResult {
	startTime := time.Now()

	result := RepositoryBranchListResult{
		Path:         repoPath,
		RelativePath: getRelativePath(rootDir, repoPath),
		Duration:     0,
	}

	// Get repository info for current branch
	repo, err := c.Open(ctx, repoPath)
	if err != nil {
		result.Status = StatusError
		result.Message = "Failed to open repository"
		result.Error = err
		result.Duration = time.Since(startTime)
		return result
	}

	info, err := c.GetInfo(ctx, repo)
	if err == nil {
		result.CurrentBranch = info.Branch
	}

	// Build git branch command
	args := []string{"branch", "--list", "--verbose", "--verbose"}
	if opts.All {
		args = append(args, "--all")
	}
	if opts.Merged {
		args = append(args, "--merged")
	} else if opts.Unmerged {
		args = append(args, "--no-merged")
	}

	// Execute git branch command
	branchResult, err := c.executor.Run(ctx, repoPath, args...)
	if err != nil {
		result.Status = StatusError
		result.Message = "Failed to list branches"
		result.Error = err
		result.Duration = time.Since(startTime)
		return result
	}

	// Parse branch output
	branches := parseBranchListOutput(branchResult.Stdout)

	// Count local and remote branches
	localCount := 0
	remoteCount := 0
	for _, b := range branches {
		if b.IsRemote {
			remoteCount++
		} else {
			localCount++
		}
	}

	result.Branches = branches
	result.LocalCount = localCount
	result.RemoteCount = remoteCount

	if len(branches) == 0 {
		result.Status = StatusNoBranches
		result.Message = "No branches found"
	} else {
		result.Status = StatusBranchesListed
		result.Message = fmt.Sprintf("%d branch(es)", len(branches))
	}

	result.Duration = time.Since(startTime)
	return result
}

// parseBranchListOutput parses git branch -vv output.
func parseBranchListOutput(output string) []BranchInfo {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	branches := make([]BranchInfo, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		branch, ok := parseBranchLine(line)
		if ok {
			branches = append(branches, branch)
		}
	}

	return branches
}

// parseBranchLine parses a single line from git branch -vv output.
func parseBranchLine(line string) (BranchInfo, bool) {
	branch := BranchInfo{}

	// Check if this is the current branch (starts with *)
	if strings.HasPrefix(line, "*") {
		branch.IsHead = true
		line = strings.TrimPrefix(line, "*")
	}

	line = strings.TrimSpace(line)

	// Parse name, SHA, upstream, and message
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return branch, false
	}

	branch.Name = parts[0]
	branch.SHA = parts[1]

	// Check if remote branch
	if strings.HasPrefix(branch.Name, "remotes/") {
		branch.IsRemote = true
		branch.Name = strings.TrimPrefix(branch.Name, "remotes/")
	}

	// Parse upstream (if present, in brackets)
	if len(parts) > 2 && strings.HasPrefix(parts[2], "[") {
		// Find the complete bracket content
		bracketContent := ""
		for i := 2; i < len(parts); i++ {
			if bracketContent != "" {
				bracketContent += " "
			}
			bracketContent += parts[i]
			if strings.HasSuffix(parts[i], "]") {
				break
			}
		}

		// Remove brackets
		bracketContent = strings.Trim(bracketContent, "[]")

		// Parse upstream and ahead/behind info
		if colonIdx := strings.Index(bracketContent, ":"); colonIdx != -1 {
			branch.Upstream = strings.TrimSpace(bracketContent[:colonIdx])
			statusPart := bracketContent[colonIdx+1:]
			branch.AheadBy, branch.BehindBy = parseBranchAheadBehind(statusPart)
		} else {
			branch.Upstream = bracketContent
		}
	}

	return branch, true
}

// parseBranchAheadBehind parses "ahead 2, behind 3" or "ahead 2" or "behind 3" from git branch -vv output.
func parseBranchAheadBehind(status string) (ahead, behind int) {
	status = strings.TrimSpace(status)

	// Parse "ahead N"
	if strings.Contains(status, "ahead") {
		_, _ = fmt.Sscanf(extractNumberAfter(status, "ahead"), "%d", &ahead) //nolint:errcheck
	}

	// Parse "behind N"
	if strings.Contains(status, "behind") {
		_, _ = fmt.Sscanf(extractNumberAfter(status, "behind"), "%d", &behind) //nolint:errcheck
	}

	return ahead, behind
}

// extractNumberAfter extracts the number following a keyword.
func extractNumberAfter(s, keyword string) string {
	idx := strings.Index(s, keyword)
	if idx == -1 {
		return "0"
	}

	rest := strings.TrimSpace(s[idx+len(keyword):])

	var num strings.Builder
	for _, c := range rest {
		if c >= '0' && c <= '9' {
			num.WriteRune(c)
		} else if num.Len() > 0 {
			break
		}
	}

	if num.Len() == 0 {
		return "0"
	}
	return num.String()
}

// calculateBranchListSummary creates a summary of branch list results by status.
func calculateBranchListSummary(results []RepositoryBranchListResult) map[string]int {
	return calculateSummaryGeneric(results)
}

// aggregateBranchCounts calculates total branch counts across all repositories.
func aggregateBranchCounts(results []RepositoryBranchListResult) (total, local, remote int) {
	for _, r := range results {
		total += len(r.Branches)
		local += r.LocalCount
		remote += r.RemoteCount
	}
	return total, local, remote
}
