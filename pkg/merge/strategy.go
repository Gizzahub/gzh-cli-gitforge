package merge

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-git/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

// MergeManager handles merge operations with different strategies
type MergeManager interface {
	// Merge performs a merge operation
	Merge(ctx context.Context, repo *repository.Repository, opts MergeOptions) (*MergeResult, error)

	// ValidateStrategy checks if a strategy is valid for the merge
	ValidateStrategy(ctx context.Context, repo *repository.Repository, opts MergeOptions) error

	// CanMerge checks if merge is possible without conflicts
	CanMerge(ctx context.Context, repo *repository.Repository, source, target string) (bool, error)

	// AbortMerge aborts an in-progress merge
	AbortMerge(ctx context.Context, repo *repository.Repository) error
}

type mergeManager struct {
	executor GitExecutor
	detector ConflictDetector
}

// NewMergeManager creates a new merge manager
func NewMergeManager(executor GitExecutor, detector ConflictDetector) MergeManager {
	return &mergeManager{
		executor: executor,
		detector: detector,
	}
}

// Merge performs a merge operation
func (m *mergeManager) Merge(ctx context.Context, repo *repository.Repository, opts MergeOptions) (*MergeResult, error) {
	// Validate options
	if err := m.ValidateStrategy(ctx, repo, opts); err != nil {
		return nil, err
	}

	// Check if working tree is clean
	if err := m.checkCleanWorkingTree(ctx, repo); err != nil {
		return nil, err
	}

	// Check if already up to date
	canFF, err := m.detector.CanFastForward(ctx, repo, opts.Source, opts.Target)
	if err != nil {
		return nil, err
	}

	// If target is already at source commit
	isUpToDate, err := m.isAlreadyUpToDate(ctx, repo, opts.Source, opts.Target)
	if err != nil {
		return nil, err
	}
	if isUpToDate {
		return &MergeResult{
			Success: true,
			Message: "already up to date",
		}, nil
	}

	// Build merge command
	args := m.buildMergeArgs(opts, canFF)

	// Execute merge
	result, err := m.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return m.handleMergeError(ctx, repo, opts, err)
	}

	// Check for conflicts
	if result.ExitCode != 0 {
		return m.handleMergeConflict(ctx, repo, opts)
	}

	// Parse merge result
	return m.parseMergeResult(ctx, repo, opts, result)
}

// ValidateStrategy checks if a strategy is valid for the merge
func (m *mergeManager) ValidateStrategy(ctx context.Context, repo *repository.Repository, opts MergeOptions) error {
	if opts.Source == "" {
		return fmt.Errorf("source branch is required")
	}

	if opts.Target == "" {
		return fmt.Errorf("target branch is required")
	}

	// Check if branches exist
	if result, err := m.executor.Run(ctx, repo.Path, "rev-parse", "--verify", opts.Source); err != nil || result.ExitCode != 0 {
		return ErrBranchNotFound
	}

	// Validate strategy
	switch opts.Strategy {
	case StrategyFastForward, StrategyRecursive, StrategyOurs, StrategyTheirs, StrategyOctopus:
		// Valid strategies
	case "":
		// Default strategy is fine
	default:
		return ErrInvalidStrategy
	}

	// Octopus requires multiple sources
	if opts.Strategy == StrategyOctopus {
		if !strings.Contains(opts.Source, " ") {
			return fmt.Errorf("octopus strategy requires multiple source branches")
		}
	}

	return nil
}

// CanMerge checks if merge is possible without conflicts
func (m *mergeManager) CanMerge(ctx context.Context, repo *repository.Repository, source, target string) (bool, error) {
	report, err := m.detector.Detect(ctx, repo, DetectOptions{
		Source: source,
		Target: target,
	})
	if err != nil {
		return false, err
	}

	return report.TotalConflicts == 0, nil
}

// AbortMerge aborts an in-progress merge
func (m *mergeManager) AbortMerge(ctx context.Context, repo *repository.Repository) error {
	result, err := m.executor.Run(ctx, repo.Path, "merge", "--abort")
	if err != nil {
		return fmt.Errorf("failed to abort merge: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("merge abort failed: %s", result.Stderr)
	}

	return nil
}

// checkCleanWorkingTree verifies no uncommitted changes exist
func (m *mergeManager) checkCleanWorkingTree(ctx context.Context, repo *repository.Repository) error {
	result, err := m.executor.Run(ctx, repo.Path, "status", "--porcelain")
	if err != nil {
		return err
	}

	if strings.TrimSpace(result.Stdout) != "" {
		return ErrDirtyWorkingTree
	}

	return nil
}

// isAlreadyUpToDate checks if target is already at source
func (m *mergeManager) isAlreadyUpToDate(ctx context.Context, repo *repository.Repository, source, target string) (bool, error) {
	// Get commit hashes
	sourceResult, err := m.executor.Run(ctx, repo.Path, "rev-parse", source)
	if err != nil {
		return false, err
	}

	targetResult, err := m.executor.Run(ctx, repo.Path, "rev-parse", target)
	if err != nil {
		return false, err
	}

	sourceHash := strings.TrimSpace(sourceResult.Stdout)
	targetHash := strings.TrimSpace(targetResult.Stdout)

	return sourceHash == targetHash, nil
}

// buildMergeArgs constructs git merge command arguments
func (m *mergeManager) buildMergeArgs(opts MergeOptions, canFastForward bool) []string {
	args := []string{"merge"}

	// Strategy options
	if opts.Strategy != "" {
		switch opts.Strategy {
		case StrategyFastForward:
			args = append(args, "--ff-only")
		case StrategyRecursive:
			args = append(args, "--strategy=recursive")
		case StrategyOurs:
			args = append(args, "--strategy=ours")
		case StrategyTheirs:
			args = append(args, "--strategy-option=theirs")
		case StrategyOctopus:
			args = append(args, "--strategy=octopus")
		}
	}

	// Fast-forward option
	if !opts.AllowFastForward && canFastForward {
		args = append(args, "--no-ff")
	}

	// No commit option
	if opts.NoCommit {
		args = append(args, "--no-commit")
	}

	// Squash option
	if opts.Squash {
		args = append(args, "--squash")
	}

	// Commit message
	if opts.CommitMessage != "" {
		args = append(args, "-m", opts.CommitMessage)
	}

	// Source branch(es)
	args = append(args, strings.Fields(opts.Source)...)

	return args
}

// handleMergeError handles merge execution errors
func (m *mergeManager) handleMergeError(ctx context.Context, repo *repository.Repository, opts MergeOptions, err error) (*MergeResult, error) {
	return &MergeResult{
		Success: false,
		Message: fmt.Sprintf("merge failed: %v", err),
	}, err
}

// handleMergeConflict handles merge conflicts
func (m *mergeManager) handleMergeConflict(ctx context.Context, repo *repository.Repository, opts MergeOptions) (*MergeResult, error) {
	// Get conflict information
	report, err := m.detector.Detect(ctx, repo, DetectOptions{
		Source: opts.Source,
		Target: opts.Target,
	})
	if err != nil {
		return nil, err
	}

	return &MergeResult{
		Success:   false,
		Strategy:  opts.Strategy,
		Conflicts: report.Conflicts,
		Message:   fmt.Sprintf("merge conflicts detected: %d conflicts", len(report.Conflicts)),
	}, ErrMergeConflict
}

// parseMergeResult parses successful merge output
func (m *mergeManager) parseMergeResult(ctx context.Context, repo *repository.Repository, opts MergeOptions, result *gitcmd.Result) (*MergeResult, error) {
	// Get merge commit hash
	commitResult, err := m.executor.Run(ctx, repo.Path, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	commitHash := strings.TrimSpace(commitResult.Stdout)

	// Parse stats from output
	filesChanged, additions, deletions := m.parseStats(result.Stdout)

	return &MergeResult{
		Success:      true,
		Strategy:     opts.Strategy,
		CommitHash:   commitHash,
		FilesChanged: filesChanged,
		Additions:    additions,
		Deletions:    deletions,
		Message:      "merge completed successfully",
	}, nil
}

// parseStats extracts statistics from git merge output
func (m *mergeManager) parseStats(output string) (files, additions, deletions int) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for lines like: "3 files changed, 10 insertions(+), 5 deletions(-)"
		if strings.Contains(line, "file") && strings.Contains(line, "changed") {
			var f, a, d int
			fmt.Sscanf(line, "%d files changed, %d insertions(+), %d deletions(-)", &f, &a, &d)
			if f > 0 {
				files = f
			}
			if a > 0 {
				additions = a
			}
			if d > 0 {
				deletions = d
			}
		}
	}
	return
}
