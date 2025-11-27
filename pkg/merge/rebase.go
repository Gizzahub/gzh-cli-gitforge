package merge

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-git/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

// RebaseManager handles rebase operations
type RebaseManager interface {
	// Rebase performs a rebase operation
	Rebase(ctx context.Context, repo *repository.Repository, opts RebaseOptions) (*RebaseResult, error)

	// Continue continues an in-progress rebase
	Continue(ctx context.Context, repo *repository.Repository) (*RebaseResult, error)

	// Skip skips the current commit in rebase
	Skip(ctx context.Context, repo *repository.Repository) (*RebaseResult, error)

	// Abort aborts an in-progress rebase
	Abort(ctx context.Context, repo *repository.Repository) error

	// Status checks the status of an in-progress rebase
	Status(ctx context.Context, repo *repository.Repository) (RebaseStatus, error)
}

type rebaseManager struct {
	executor GitExecutor
}

// NewRebaseManager creates a new rebase manager
func NewRebaseManager(executor GitExecutor) RebaseManager {
	return &rebaseManager{executor: executor}
}

// Rebase performs a rebase operation
func (r *rebaseManager) Rebase(ctx context.Context, repo *repository.Repository, opts RebaseOptions) (*RebaseResult, error) {
	// Validate options
	if opts.Branch == "" && opts.Onto == "" && opts.UpstreamName == "" {
		return nil, fmt.Errorf("branch, onto, or upstream is required")
	}

	// Check if rebase is already in progress
	status, err := r.Status(ctx, repo)
	if err != nil {
		return nil, err
	}
	if status == RebaseInProgress {
		return nil, ErrRebaseInProgress
	}

	// Check if working tree is clean
	if err := r.checkCleanWorkingTree(ctx, repo); err != nil {
		return nil, err
	}

	// Build rebase command
	args := r.buildRebaseArgs(opts)

	// Execute rebase
	result, err := r.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return r.handleRebaseError(ctx, repo, err)
	}

	// Check for conflicts
	if result.ExitCode != 0 {
		return r.handleRebaseConflict(ctx, repo, result)
	}

	// Parse rebase result
	return r.parseRebaseResult(result)
}

// Continue continues an in-progress rebase
func (r *rebaseManager) Continue(ctx context.Context, repo *repository.Repository) (*RebaseResult, error) {
	// Check if rebase is in progress
	status, err := r.Status(ctx, repo)
	if err != nil {
		return nil, err
	}
	if status != RebaseInProgress {
		return nil, ErrNoRebaseInProgress
	}

	// Execute rebase --continue
	result, err := r.executor.Run(ctx, repo.Path, "rebase", "--continue")
	if err != nil {
		return r.handleRebaseError(ctx, repo, err)
	}

	// Check for conflicts
	if result.ExitCode != 0 {
		return r.handleRebaseConflict(ctx, repo, result)
	}

	return r.parseRebaseResult(result)
}

// Skip skips the current commit in rebase
func (r *rebaseManager) Skip(ctx context.Context, repo *repository.Repository) (*RebaseResult, error) {
	// Check if rebase is in progress
	status, err := r.Status(ctx, repo)
	if err != nil {
		return nil, err
	}
	if status != RebaseInProgress {
		return nil, ErrNoRebaseInProgress
	}

	// Execute rebase --skip
	result, err := r.executor.Run(ctx, repo.Path, "rebase", "--skip")
	if err != nil {
		return r.handleRebaseError(ctx, repo, err)
	}

	// Check for conflicts
	if result.ExitCode != 0 {
		return r.handleRebaseConflict(ctx, repo, result)
	}

	return r.parseRebaseResult(result)
}

// Abort aborts an in-progress rebase
func (r *rebaseManager) Abort(ctx context.Context, repo *repository.Repository) error {
	// Check if rebase is in progress
	status, err := r.Status(ctx, repo)
	if err != nil {
		return err
	}
	if status != RebaseInProgress {
		return ErrNoRebaseInProgress
	}

	// Execute rebase --abort
	result, err := r.executor.Run(ctx, repo.Path, "rebase", "--abort")
	if err != nil {
		return fmt.Errorf("failed to abort rebase: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("rebase abort failed: %s", result.Stderr)
	}

	return nil
}

// Status checks the status of an in-progress rebase
func (r *rebaseManager) Status(ctx context.Context, repo *repository.Repository) (RebaseStatus, error) {
	// Check for rebase directory
	result, err := r.executor.Run(ctx, repo.Path, "rev-parse", "--git-path", "rebase-merge")
	if err != nil {
		return "", err
	}

	rebasePath := strings.TrimSpace(result.Stdout)

	// Check if rebase directory exists using git
	checkResult, err := r.executor.Run(ctx, repo.Path, "test", "-d", rebasePath)
	if err == nil && checkResult.ExitCode == 0 {
		return RebaseInProgress, nil
	}

	return RebaseComplete, nil
}

// checkCleanWorkingTree verifies no uncommitted changes exist
func (r *rebaseManager) checkCleanWorkingTree(ctx context.Context, repo *repository.Repository) error {
	result, err := r.executor.Run(ctx, repo.Path, "status", "--porcelain")
	if err != nil {
		return err
	}

	if strings.TrimSpace(result.Stdout) != "" {
		return ErrDirtyWorkingTree
	}

	return nil
}

// buildRebaseArgs constructs git rebase command arguments
func (r *rebaseManager) buildRebaseArgs(opts RebaseOptions) []string {
	args := []string{"rebase"}

	// Interactive mode
	if opts.Interactive {
		args = append(args, "-i")
	}

	// Auto-squash
	if opts.AutoSquash {
		args = append(args, "--autosquash")
	}

	// Preserve merges
	if opts.PreserveMerges {
		args = append(args, "--preserve-merges")
	}

	// Onto option
	if opts.Onto != "" {
		args = append(args, "--onto", opts.Onto)
	}

	// Upstream/branch
	if opts.UpstreamName != "" {
		args = append(args, opts.UpstreamName)
	} else if opts.Branch != "" {
		args = append(args, opts.Branch)
	}

	return args
}

// handleRebaseError handles rebase execution errors
func (r *rebaseManager) handleRebaseError(ctx context.Context, repo *repository.Repository, err error) (*RebaseResult, error) {
	return &RebaseResult{
		Success: false,
		Status:  RebaseAborted,
		Message: fmt.Sprintf("rebase failed: %v", err),
	}, err
}

// handleRebaseConflict handles rebase conflicts
func (r *rebaseManager) handleRebaseConflict(ctx context.Context, repo *repository.Repository, result *gitcmd.Result) (*RebaseResult, error) {
	// Count conflicts
	conflictsFound := strings.Count(result.Stdout, "CONFLICT")
	if conflictsFound == 0 {
		conflictsFound = strings.Count(result.Stderr, "CONFLICT")
	}

	// Get current commit
	commitResult, _ := r.executor.Run(ctx, repo.Path, "rev-parse", "HEAD")
	currentCommit := strings.TrimSpace(commitResult.Stdout)

	return &RebaseResult{
		Success:        false,
		ConflictsFound: conflictsFound,
		CurrentCommit:  currentCommit,
		Status:         RebaseConflict,
		Message:        "rebase conflicts detected",
	}, nil
}

// parseRebaseResult parses successful rebase output
func (r *rebaseManager) parseRebaseResult(result *gitcmd.Result) (*RebaseResult, error) {
	output := result.Stdout + result.Stderr

	// Count rebased commits
	commitsRebased := strings.Count(output, "Rebasing (")
	if commitsRebased == 0 {
		// Alternative patterns
		if strings.Contains(output, "Successfully rebased") {
			commitsRebased = 1
		}
	}

	return &RebaseResult{
		Success:        true,
		CommitsRebased: commitsRebased,
		Status:         RebaseComplete,
		Message:        "rebase completed successfully",
	}, nil
}
