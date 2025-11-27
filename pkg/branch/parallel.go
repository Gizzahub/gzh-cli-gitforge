package branch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gizzahub/gzh-cli-git/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

// ParallelWorkflow manages parallel development workflows.
type ParallelWorkflow interface {
	// GetActiveContexts returns all active worktree contexts.
	GetActiveContexts(ctx context.Context, repo *repository.Repository) ([]*WorkContext, error)

	// SwitchContext provides information for switching to a worktree.
	SwitchContext(ctx context.Context, repo *repository.Repository, path string) (*SwitchInfo, error)

	// DetectConflicts detects potential conflicts across worktrees.
	DetectConflicts(ctx context.Context, repo *repository.Repository) ([]*Conflict, error)

	// GetStatus gets status across all worktrees.
	GetStatus(ctx context.Context, repo *repository.Repository) (*ParallelStatus, error)
}

// WorkContext represents a development context (worktree).
type WorkContext struct {
	Path         string   // Worktree path
	Branch       string   // Current branch
	IsMain       bool     // Is main worktree
	HasChanges   bool     // Has uncommitted changes
	ModifiedFiles []string // List of modified files
}

// SwitchInfo provides information for context switching.
type SwitchInfo struct {
	FromPath   string // Current location
	ToPath     string // Target worktree path
	ToBranch   string // Target branch
	Command    string // Suggested command (cd)
	HasChanges bool   // Target has uncommitted changes
}

// Conflict represents a potential conflict across worktrees.
type Conflict struct {
	File      string   // File path
	Worktrees []string // Worktrees modifying this file
	Severity  ConflictSeverity
}

// ConflictSeverity indicates conflict severity.
type ConflictSeverity string

const (
	SeverityLow    ConflictSeverity = "low"    // Different files
	SeverityMedium ConflictSeverity = "medium" // Same directory
	SeverityHigh   ConflictSeverity = "high"   // Same file
)

// ParallelStatus represents status across all worktrees.
type ParallelStatus struct {
	TotalWorktrees  int            // Total number of worktrees
	ActiveWorktrees int            // Worktrees with changes
	Conflicts       int            // Number of conflicts
	Contexts        []*WorkContext // All contexts
}

// parallelWorkflow implements ParallelWorkflow.
type parallelWorkflow struct {
	executor        *gitcmd.Executor
	worktreeManager WorktreeManager
}

// NewParallelWorkflow creates a new ParallelWorkflow.
func NewParallelWorkflow() ParallelWorkflow {
	return &parallelWorkflow{
		executor:        gitcmd.NewExecutor(),
		worktreeManager: NewWorktreeManager(),
	}
}

// NewParallelWorkflowWithDeps creates a new ParallelWorkflow with custom dependencies.
func NewParallelWorkflowWithDeps(executor *gitcmd.Executor, worktreeManager WorktreeManager) ParallelWorkflow {
	return &parallelWorkflow{
		executor:        executor,
		worktreeManager: worktreeManager,
	}
}

// GetActiveContexts returns all active worktree contexts.
func (p *parallelWorkflow) GetActiveContexts(ctx context.Context, repo *repository.Repository) ([]*WorkContext, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	// Get all worktrees
	worktrees, err := p.worktreeManager.List(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Build contexts
	contexts := make([]*WorkContext, 0, len(worktrees))
	for _, wt := range worktrees {
		context, err := p.buildWorkContext(ctx, wt)
		if err != nil {
			// Log error but continue with other worktrees
			continue
		}
		contexts = append(contexts, context)
	}

	return contexts, nil
}

// SwitchContext provides information for switching to a worktree.
func (p *parallelWorkflow) SwitchContext(ctx context.Context, repo *repository.Repository, path string) (*SwitchInfo, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	if path == "" {
		return nil, fmt.Errorf("worktree path is required")
	}

	// Get target worktree
	targetWt, err := p.worktreeManager.Get(ctx, repo, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get current directory
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = ""
	}

	// Check if target has changes
	hasChanges, err := p.hasUncommittedChanges(ctx, path)
	if err != nil {
		hasChanges = false
	}

	// Build switch info
	info := &SwitchInfo{
		FromPath:   currentDir,
		ToPath:     targetWt.Path,
		ToBranch:   targetWt.Branch,
		Command:    fmt.Sprintf("cd %s", targetWt.Path),
		HasChanges: hasChanges,
	}

	return info, nil
}

// DetectConflicts detects potential conflicts across worktrees.
func (p *parallelWorkflow) DetectConflicts(ctx context.Context, repo *repository.Repository) ([]*Conflict, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	// Get all contexts
	contexts, err := p.GetActiveContexts(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get contexts: %w", err)
	}

	// Build file -> worktrees map
	fileWorktrees := make(map[string][]string)
	for _, context := range contexts {
		if !context.HasChanges {
			continue
		}

		for _, file := range context.ModifiedFiles {
			fileWorktrees[file] = append(fileWorktrees[file], context.Path)
		}
	}

	// Find conflicts
	conflicts := make([]*Conflict, 0)
	for file, worktrees := range fileWorktrees {
		if len(worktrees) > 1 {
			conflict := &Conflict{
				File:      file,
				Worktrees: worktrees,
				Severity:  p.determineConflictSeverity(file, worktrees),
			}
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts, nil
}

// GetStatus gets status across all worktrees.
func (p *parallelWorkflow) GetStatus(ctx context.Context, repo *repository.Repository) (*ParallelStatus, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	// Get all contexts
	contexts, err := p.GetActiveContexts(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get contexts: %w", err)
	}

	// Detect conflicts
	conflicts, err := p.DetectConflicts(ctx, repo)
	if err != nil {
		// Continue without conflict detection
		conflicts = []*Conflict{}
	}

	// Count active worktrees
	activeCount := 0
	for _, context := range contexts {
		if context.HasChanges {
			activeCount++
		}
	}

	status := &ParallelStatus{
		TotalWorktrees:  len(contexts),
		ActiveWorktrees: activeCount,
		Conflicts:       len(conflicts),
		Contexts:        contexts,
	}

	return status, nil
}

// buildWorkContext builds a WorkContext from a Worktree.
func (p *parallelWorkflow) buildWorkContext(ctx context.Context, wt *Worktree) (*WorkContext, error) {
	// Check for uncommitted changes
	hasChanges, err := p.hasUncommittedChanges(ctx, wt.Path)
	if err != nil {
		hasChanges = false
	}

	// Get modified files if there are changes
	var modifiedFiles []string
	if hasChanges {
		modifiedFiles, err = p.getModifiedFiles(ctx, wt.Path)
		if err != nil {
			modifiedFiles = []string{}
		}
	}

	context := &WorkContext{
		Path:          wt.Path,
		Branch:        wt.Branch,
		IsMain:        wt.IsMain,
		HasChanges:    hasChanges,
		ModifiedFiles: modifiedFiles,
	}

	return context, nil
}

// hasUncommittedChanges checks if a worktree has uncommitted changes.
func (p *parallelWorkflow) hasUncommittedChanges(ctx context.Context, path string) (bool, error) {
	result, err := p.executor.Run(ctx, path, "status", "--porcelain")
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(result.Stdout) != "", nil
}

// getModifiedFiles gets list of modified files in a worktree.
func (p *parallelWorkflow) getModifiedFiles(ctx context.Context, path string) ([]string, error) {
	result, err := p.executor.Run(ctx, path, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	files := make([]string, 0)
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Format: "XY filename" where XY is status code
		if len(line) > 3 {
			filename := strings.TrimSpace(line[3:])
			files = append(files, filename)
		}
	}

	return files, nil
}

// determineConflictSeverity determines conflict severity based on file paths.
func (p *parallelWorkflow) determineConflictSeverity(file string, worktrees []string) ConflictSeverity {
	// If same file modified in multiple worktrees, high severity
	if len(worktrees) > 1 {
		return SeverityHigh
	}

	// Check if files in same directory
	dir := filepath.Dir(file)
	for _, wt := range worktrees {
		wtDir := filepath.Dir(wt)
		if dir == wtDir {
			return SeverityMedium
		}
	}

	return SeverityLow
}

// HasConflicts checks if there are any conflicts.
func (s *ParallelStatus) HasConflicts() bool {
	return s.Conflicts > 0
}

// IsActive checks if any worktree has uncommitted changes.
func (s *ParallelStatus) IsActive() bool {
	return s.ActiveWorktrees > 0
}

// GetMainContext returns the main worktree context.
func (s *ParallelStatus) GetMainContext() *WorkContext {
	for _, ctx := range s.Contexts {
		if ctx.IsMain {
			return ctx
		}
	}
	return nil
}

// GetActiveContexts returns contexts with uncommitted changes.
func (s *ParallelStatus) GetActiveContexts() []*WorkContext {
	active := make([]*WorkContext, 0)
	for _, ctx := range s.Contexts {
		if ctx.HasChanges {
			active = append(active, ctx)
		}
	}
	return active
}
