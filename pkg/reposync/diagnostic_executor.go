// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	repo "github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// DiagnosticExecutor performs health checks on repositories.
type DiagnosticExecutor struct {
	Client repo.Client
	Logger repo.Logger
}

// CheckHealth performs health diagnostics on multiple repositories.
func (e DiagnosticExecutor) CheckHealth(ctx context.Context, repos []RepoSpec, opts DiagnosticOptions) (*HealthReport, error) {
	startTime := time.Now()

	if opts.FetchTimeout == 0 {
		opts = DefaultDiagnosticOptions()
	}

	client := e.Client
	if client == nil {
		client = repo.NewClient()
	}
	logger := e.Logger
	if logger == nil {
		logger = nopGitLogger{}
	}

	parallel := opts.Parallel
	if parallel <= 0 {
		parallel = repo.DefaultForgeParallel
	}

	jobs := make(chan RepoSpec, len(repos))
	results := make(chan RepoHealth, len(repos))

	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for descriptor := range jobs {
			if ctx.Err() != nil {
				return
			}
			// Notify progress start
			if opts.Progress != nil {
				opts.Progress.OnRepoStart(descriptor)
			}

			health := e.checkOne(ctx, client, logger, descriptor, opts)

			// Notify progress complete
			if opts.Progress != nil {
				opts.Progress.OnRepoComplete(health)
			}

			results <- health
		}
	}

	wg.Add(parallel)
	for i := 0; i < parallel; i++ {
		go worker()
	}

	go func() {
		for _, r := range repos {
			jobs <- r
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var healthResults []RepoHealth
	for health := range results {
		healthResults = append(healthResults, health)
	}

	report := &HealthReport{
		Results:       healthResults,
		Summary:       calculateSummary(healthResults),
		TotalDuration: time.Since(startTime),
		CheckedAt:     time.Now(),
	}

	return report, nil
}

func (e DiagnosticExecutor) checkOne(ctx context.Context, client repo.Client, logger repo.Logger, descriptor RepoSpec, opts DiagnosticOptions) RepoHealth {
	startTime := time.Now()

	health := RepoHealth{
		Repo:         descriptor,
		HealthStatus: HealthUnreachable,
	}

	// Open repository
	r, err := client.Open(ctx, descriptor.TargetPath)
	if err != nil {
		health.Error = fmt.Errorf("failed to open repository: %w", err)
		health.Duration = time.Since(startTime)
		return health
	}

	// Get current branch and upstream
	info, err := client.GetInfo(ctx, r)
	if err != nil {
		health.Error = fmt.Errorf("failed to get repository info: %w", err)
		health.Duration = time.Since(startTime)
		return health
	}

	health.CurrentBranch = info.Branch
	health.UpstreamBranch = info.Upstream

	// Fetch remotes (unless skipped)
	if !opts.SkipFetch {
		fetchStart := time.Now()
		networkStatus, fetchErr := e.fetchWithTimeout(ctx, r, opts.FetchTimeout)
		health.FetchDuration = time.Since(fetchStart)
		health.NetworkStatus = networkStatus

		if fetchErr != nil {
			logger.Warn("remote fetch failed", "repo", descriptor.TargetPath, "error", fetchErr)
			if networkStatus == NetworkTimeout || networkStatus == NetworkUnreachable {
				health.HealthStatus = HealthUnreachable
				health.Error = fetchErr
				health.Recommendation = "Check network connection and remote URL"
				health.Duration = time.Since(startTime)
				return health
			}
			// Continue even if fetch fails (use stale data)
		}
	} else {
		health.NetworkStatus = NetworkOK // Assume OK if not checked
	}

	// Re-check info after fetch
	info, err = client.GetInfo(ctx, r)
	if err != nil {
		health.Error = fmt.Errorf("failed to refresh repository info: %w", err)
		health.Duration = time.Since(startTime)
		return health
	}

	health.AheadBy = info.AheadBy
	health.BehindBy = info.BehindBy

	// Check working tree status
	if opts.CheckWorkTree {
		status, err := client.GetStatus(ctx, r)
		if err != nil {
			logger.Warn("failed to check working tree", "repo", descriptor.TargetPath, "error", err)
			health.WorkTreeStatus = WorkTreeClean // Assume clean on error
		} else {
			health.ModifiedFiles = len(status.ModifiedFiles) + len(status.StagedFiles)
			health.UntrackedFiles = len(status.UntrackedFiles)
			health.ConflictFiles = len(status.ConflictFiles)

			if len(status.ConflictFiles) > 0 {
				health.WorkTreeStatus = WorkTreeConflict
			} else if len(status.ModifiedFiles)+len(status.StagedFiles) > 0 {
				health.WorkTreeStatus = WorkTreeDirty
			} else {
				health.WorkTreeStatus = WorkTreeClean
			}
		}
	}

	// Classify divergence
	health.DivergenceType = classifyDivergence(health)

	// Determine overall health status
	health.HealthStatus = classifyHealth(health)

	// Generate recommendation
	if opts.IncludeRecommendations {
		health.Recommendation = generateRecommendation(health)
	}

	health.Duration = time.Since(startTime)
	return health
}

func (e DiagnosticExecutor) fetchWithTimeout(ctx context.Context, r *repo.Repository, timeout time.Duration) (NetworkStatus, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute git fetch --all --prune with timeout
	executor := gitcmd.NewExecutor(gitcmd.WithTimeout(timeout))
	result, err := executor.Run(fetchCtx, r.Path, "fetch", "--all", "--prune")
	if err != nil {
		// Check for timeout
		if fetchCtx.Err() == context.DeadlineExceeded {
			return NetworkTimeout, fmt.Errorf("fetch timeout after %v", timeout)
		}

		// Check for network errors in stderr
		stderrLower := strings.ToLower(result.Stderr)
		if strings.Contains(stderrLower, "could not resolve host") ||
			strings.Contains(stderrLower, "connection refused") ||
			strings.Contains(stderrLower, "network is unreachable") {
			return NetworkUnreachable, fmt.Errorf("remote unreachable: %w", err)
		}

		// Check for authentication errors
		if strings.Contains(stderrLower, "authentication failed") ||
			strings.Contains(stderrLower, "permission denied") ||
			strings.Contains(stderrLower, "could not read from remote") {
			return NetworkAuthFailed, fmt.Errorf("authentication failed: %w", err)
		}

		// Unknown error
		return NetworkUnreachable, fmt.Errorf("fetch failed: %w", err)
	}

	return NetworkOK, nil
}

func classifyDivergence(health RepoHealth) DivergenceType {
	if health.UpstreamBranch == "" {
		return DivergenceNoUpstream
	}

	if health.ConflictFiles > 0 {
		return DivergenceConflict
	}

	ahead := health.AheadBy
	behind := health.BehindBy

	if ahead == 0 && behind == 0 {
		return DivergenceNone
	}

	if ahead > 0 && behind > 0 {
		return DivergenceDiverged
	}

	if behind > 0 {
		return DivergenceFastForward
	}

	if ahead > 0 {
		return DivergenceAhead
	}

	return DivergenceNone
}

func classifyHealth(health RepoHealth) HealthStatus {
	// Unreachable if network failed
	if health.NetworkStatus == NetworkTimeout || health.NetworkStatus == NetworkUnreachable {
		return HealthUnreachable
	}

	// Error if conflicts exist
	if health.WorkTreeStatus == WorkTreeConflict {
		return HealthError
	}

	// Error if dirty + behind (potential merge conflicts)
	if health.WorkTreeStatus == WorkTreeDirty && health.BehindBy > 0 {
		return HealthError
	}

	// Warning if diverged or behind
	if health.DivergenceType == DivergenceDiverged || health.DivergenceType == DivergenceFastForward {
		return HealthWarning
	}

	// Warning if ahead (unpushed commits)
	if health.DivergenceType == DivergenceAhead {
		return HealthWarning
	}

	// Warning if dirty working tree (uncommitted changes need attention)
	if health.WorkTreeStatus == WorkTreeDirty {
		return HealthWarning
	}

	// Healthy if up-to-date and clean
	return HealthHealthy
}

func generateRecommendation(health RepoHealth) string {
	switch health.HealthStatus {
	case HealthUnreachable:
		return "Check network connection and verify remote URL is accessible"

	case HealthError:
		if health.WorkTreeStatus == WorkTreeConflict {
			return "Resolve merge conflicts, then commit or reset"
		}
		if health.WorkTreeStatus == WorkTreeDirty && health.BehindBy > 0 {
			return fmt.Sprintf("Commit or stash %d modified files, then pull %d commits from upstream", health.ModifiedFiles, health.BehindBy)
		}
		return "Manual intervention required"

	case HealthWarning:
		// Check divergence-related warnings first
		switch health.DivergenceType {
		case DivergenceFastForward:
			return fmt.Sprintf("Pull %d commits from upstream (fast-forward): gz-git workspace sync --strategy pull", health.BehindBy)
		case DivergenceDiverged:
			return fmt.Sprintf("Diverged: %d ahead, %d behind. Use 'git pull --rebase' or 'git merge' to reconcile", health.AheadBy, health.BehindBy)
		case DivergenceAhead:
			return fmt.Sprintf("Push %d local commits to upstream: gz-git push", health.AheadBy)
		case DivergenceNoUpstream:
			return "No upstream branch configured. Set upstream with: git branch --set-upstream-to=origin/" + health.CurrentBranch
		}
		// Check dirty working tree (when divergence is none)
		if health.WorkTreeStatus == WorkTreeDirty {
			if health.UntrackedFiles > 0 && health.ModifiedFiles > 0 {
				return fmt.Sprintf("Uncommitted changes: %d modified, %d untracked files. Commit or stash before syncing", health.ModifiedFiles, health.UntrackedFiles)
			} else if health.ModifiedFiles > 0 {
				return fmt.Sprintf("Uncommitted changes: %d modified files. Commit or stash before syncing", health.ModifiedFiles)
			}
			return "Uncommitted changes detected. Commit or stash before syncing"
		}

	case HealthHealthy:
		return "No action needed, repository is up-to-date"
	}

	return ""
}

func calculateSummary(results []RepoHealth) HealthSummary {
	var summary HealthSummary
	summary.Total = len(results)

	for _, r := range results {
		switch r.HealthStatus {
		case HealthHealthy:
			summary.Healthy++
		case HealthWarning:
			summary.Warning++
		case HealthError:
			summary.Error++
		case HealthUnreachable:
			summary.Unreachable++
		}
	}

	return summary
}
