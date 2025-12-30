package commit

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// SmartPush provides safe push operations.
type SmartPush interface {
	// Push performs a safe push with checks.
	Push(ctx context.Context, repo *repository.Repository, opts PushOptions) error

	// CanPush checks if push is safe.
	CanPush(ctx context.Context, repo *repository.Repository) (*PushCheck, error)
}

// PushOptions configures push behavior.
type PushOptions struct {
	Remote      string
	Remotes     []string // Multiple remotes for multi-push
	Branch      string
	Refspec     string // Custom refspec (e.g., "develop:master")
	Force       bool
	SetUpstream bool
	AllRemotes  bool // Push to all configured remotes
	DryRun      bool
	SkipChecks  bool // For emergency use
}

// PushCheck contains push safety check results.
type PushCheck struct {
	Safe            bool
	Issues          []PushIssue
	Recommendations []string
}

// PushIssue represents a push safety issue.
type PushIssue struct {
	Severity string // error, warning, info
	Message  string
	Blocker  bool // Blocks push if true
}

// smartPush implements SmartPush.
type smartPush struct {
	executor *gitcmd.Executor
}

// NewSmartPush creates a new SmartPush.
func NewSmartPush() SmartPush {
	return &smartPush{
		executor: gitcmd.NewExecutor(),
	}
}

// NewSmartPushWithExecutor creates a new SmartPush with a custom executor.
func NewSmartPushWithExecutor(executor *gitcmd.Executor) SmartPush {
	return &smartPush{
		executor: executor,
	}
}

// Protected branches that should not accept force pushes
var protectedBranches = map[string]bool{
	"main":    true,
	"master":  true,
	"develop": true,
	"release": true,
}

// Push performs a safe push with checks.
func (p *smartPush) Push(ctx context.Context, repo *repository.Repository, opts PushOptions) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	// Determine target remotes
	remotes, err := p.getTargetRemotes(ctx, repo, opts)
	if err != nil {
		return fmt.Errorf("failed to determine target remotes: %w", err)
	}

	// Set default branch if not using custom refspec
	if opts.Branch == "" && opts.Refspec == "" {
		// Get current branch
		currentBranch, err := p.getCurrentBranch(ctx, repo)
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		opts.Branch = currentBranch
	}

	// Run safety checks unless explicitly skipped
	if !opts.SkipChecks {
		check, err := p.CanPush(ctx, repo)
		if err != nil {
			return fmt.Errorf("safety check failed: %w", err)
		}

		// Check for blocking issues
		if !check.Safe {
			var blockers []string
			for _, issue := range check.Issues {
				if issue.Blocker {
					blockers = append(blockers, issue.Message)
				}
			}
			if len(blockers) > 0 {
				return &CommitError{
					Op:      "push",
					Cause:   ErrPushBlocked,
					Message: "push blocked by safety checks",
					Hints:   blockers,
				}
			}
		}
	}

	// Push to each remote
	var errors []string
	for _, remote := range remotes {
		if err := p.pushToRemote(ctx, repo, remote, opts); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", remote, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("push failed for some remotes: %s", strings.Join(errors, "; "))
	}

	return nil
}

// getTargetRemotes determines which remotes to push to based on options.
func (p *smartPush) getTargetRemotes(ctx context.Context, repo *repository.Repository, opts PushOptions) ([]string, error) {
	// Priority: Remotes array > AllRemotes > Single Remote
	if len(opts.Remotes) > 0 {
		return opts.Remotes, nil
	}

	if opts.AllRemotes {
		return p.getAllRemotes(ctx, repo)
	}

	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}
	return []string{remote}, nil
}

// getAllRemotes retrieves all configured remotes for the repository.
func (p *smartPush) getAllRemotes(ctx context.Context, repo *repository.Repository) ([]string, error) {
	result, err := p.executor.Run(ctx, repo.Path, "remote")
	if err != nil {
		return nil, fmt.Errorf("failed to get remotes: %w", err)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to get remotes: %s", result.Stderr)
	}

	remotes := []string{}
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if remote := strings.TrimSpace(line); remote != "" {
			remotes = append(remotes, remote)
		}
	}

	if len(remotes) == 0 {
		return nil, fmt.Errorf("no remotes configured")
	}

	return remotes, nil
}

// pushToRemote performs the actual push to a single remote.
func (p *smartPush) pushToRemote(ctx context.Context, repo *repository.Repository, remote string, opts PushOptions) error {
	// Build push command
	args := []string{"push"}

	if opts.DryRun {
		args = append(args, "--dry-run")
	}

	if opts.Force {
		// Additional check for force push to protected branches
		branchName := opts.Branch
		if opts.Refspec != "" {
			// Extract local branch from refspec (format: local:remote)
			parts := strings.Split(opts.Refspec, ":")
			if len(parts) > 0 {
				branchName = parts[0]
			}
		}
		if protectedBranches[branchName] && !opts.SkipChecks {
			return &CommitError{
				Op:      "push",
				Cause:   ErrPushBlocked,
				Message: fmt.Sprintf("force push to protected branch '%s' is not allowed", branchName),
				Hints:   []string{"use --skip-checks flag only if absolutely necessary"},
			}
		}
		args = append(args, "--force-with-lease")
	}

	if opts.SetUpstream {
		args = append(args, "--set-upstream")
	}

	args = append(args, remote)

	// Add refspec or branch
	if opts.Refspec != "" {
		args = append(args, opts.Refspec)
	} else {
		args = append(args, opts.Branch)
	}

	// Execute push
	result, err := p.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git push failed: %s", result.Stderr)
	}

	return nil
}

// CanPush checks if push is safe.
func (p *smartPush) CanPush(ctx context.Context, repo *repository.Repository) (*PushCheck, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	check := &PushCheck{
		Safe:            true,
		Issues:          []PushIssue{},
		Recommendations: []string{},
	}

	// Get current branch
	currentBranch, err := p.getCurrentBranch(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if there are uncommitted changes
	hasUncommitted, err := p.hasUncommittedChanges(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}

	if hasUncommitted {
		check.Safe = false
		check.Issues = append(check.Issues, PushIssue{
			Severity: "error",
			Message:  "repository has uncommitted changes",
			Blocker:  true,
		})
		check.Recommendations = append(check.Recommendations, "commit or stash changes before pushing")
	}

	// Check if branch has upstream
	hasUpstream, err := p.hasUpstream(ctx, repo, currentBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to check upstream: %w", err)
	}

	if !hasUpstream {
		check.Issues = append(check.Issues, PushIssue{
			Severity: "warning",
			Message:  fmt.Sprintf("branch '%s' has no upstream", currentBranch),
			Blocker:  false,
		})
		check.Recommendations = append(check.Recommendations, "use --set-upstream flag to set upstream")
	}

	// Check if branch is ahead/behind remote
	if hasUpstream {
		ahead, behind, err := p.getAheadBehind(ctx, repo, currentBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to get ahead/behind count: %w", err)
		}

		if behind > 0 {
			check.Safe = false
			check.Issues = append(check.Issues, PushIssue{
				Severity: "error",
				Message:  fmt.Sprintf("branch is %d commit(s) behind remote", behind),
				Blocker:  true,
			})
			check.Recommendations = append(check.Recommendations, "pull remote changes before pushing")
		}

		if ahead == 0 {
			check.Issues = append(check.Issues, PushIssue{
				Severity: "info",
				Message:  "branch is up-to-date with remote",
				Blocker:  false,
			})
		} else {
			check.Issues = append(check.Issues, PushIssue{
				Severity: "info",
				Message:  fmt.Sprintf("branch is %d commit(s) ahead of remote", ahead),
				Blocker:  false,
			})
		}
	}

	// Check if pushing to protected branch
	if protectedBranches[currentBranch] {
		check.Issues = append(check.Issues, PushIssue{
			Severity: "warning",
			Message:  fmt.Sprintf("pushing to protected branch '%s'", currentBranch),
			Blocker:  false,
		})
		check.Recommendations = append(check.Recommendations, "ensure you have proper authorization")
	}

	return check, nil
}

// getCurrentBranch gets the current branch name.
func (p *smartPush) getCurrentBranch(ctx context.Context, repo *repository.Repository) (string, error) {
	result, err := p.executor.Run(ctx, repo.Path, "branch", "--show-current")
	if err != nil {
		return "", err
	}

	branch := strings.TrimSpace(result.Stdout)
	if branch == "" {
		return "", fmt.Errorf("not on any branch (detached HEAD)")
	}

	return branch, nil
}

// hasUncommittedChanges checks if there are uncommitted changes.
func (p *smartPush) hasUncommittedChanges(ctx context.Context, repo *repository.Repository) (bool, error) {
	result, err := p.executor.Run(ctx, repo.Path, "status", "--porcelain")
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(result.Stdout) != "", nil
}

// hasUpstream checks if the branch has an upstream.
func (p *smartPush) hasUpstream(ctx context.Context, repo *repository.Repository, branch string) (bool, error) {
	result, err := p.executor.Run(ctx, repo.Path, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil || result.ExitCode != 0 {
		// No upstream configured
		return false, nil
	}

	return strings.TrimSpace(result.Stdout) != "", nil
}

// getAheadBehind gets the number of commits ahead/behind remote.
func (p *smartPush) getAheadBehind(ctx context.Context, repo *repository.Repository, branch string) (ahead, behind int, err error) {
	result, err := p.executor.Run(ctx, repo.Path, "rev-list", "--left-right", "--count", branch+"..."+branch+"@{upstream}")
	if err != nil || result.ExitCode != 0 {
		return 0, 0, fmt.Errorf("failed to get ahead/behind count")
	}

	// Output format: "ahead\tbehind"
	parts := strings.Fields(strings.TrimSpace(result.Stdout))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %s", result.Stdout)
	}

	fmt.Sscanf(parts[0], "%d", &ahead)
	fmt.Sscanf(parts[1], "%d", &behind)

	return ahead, behind, nil
}

// FormatPushCheck formats a PushCheck as a human-readable string.
func FormatPushCheck(check *PushCheck) string {
	if check == nil {
		return ""
	}

	var sb strings.Builder

	if check.Safe {
		sb.WriteString("✓ Safe to push\n")
	} else {
		sb.WriteString("✗ Push blocked\n")
	}

	// Format issues by severity
	errors := []PushIssue{}
	warnings := []PushIssue{}
	infos := []PushIssue{}

	for _, issue := range check.Issues {
		switch issue.Severity {
		case "error":
			errors = append(errors, issue)
		case "warning":
			warnings = append(warnings, issue)
		case "info":
			infos = append(infos, issue)
		}
	}

	if len(errors) > 0 {
		sb.WriteString("\nErrors:\n")
		for _, err := range errors {
			sb.WriteString(fmt.Sprintf("  ✗ %s\n", err.Message))
		}
	}

	if len(warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, warn := range warnings {
			sb.WriteString(fmt.Sprintf("  ⚠ %s\n", warn.Message))
		}
	}

	if len(infos) > 0 {
		sb.WriteString("\nInfo:\n")
		for _, info := range infos {
			sb.WriteString(fmt.Sprintf("  ℹ %s\n", info.Message))
		}
	}

	if len(check.Recommendations) > 0 {
		sb.WriteString("\nRecommendations:\n")
		for _, rec := range check.Recommendations {
			sb.WriteString(fmt.Sprintf("  → %s\n", rec))
		}
	}

	return sb.String()
}
