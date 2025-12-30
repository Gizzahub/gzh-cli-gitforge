package merge

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

type mockConflictDetector struct {
	detectFunc         func(ctx context.Context, repo *repository.Repository, opts DetectOptions) (*ConflictReport, error)
	canFastForwardFunc func(ctx context.Context, repo *repository.Repository, source, target string) (bool, error)
}

func (m *mockConflictDetector) Detect(ctx context.Context, repo *repository.Repository, opts DetectOptions) (*ConflictReport, error) {
	if m.detectFunc != nil {
		return m.detectFunc(ctx, repo, opts)
	}
	return &ConflictReport{TotalConflicts: 0}, nil
}

func (m *mockConflictDetector) Preview(ctx context.Context, repo *repository.Repository, source, target string) (*MergePreview, error) {
	return nil, nil
}

func (m *mockConflictDetector) CanFastForward(ctx context.Context, repo *repository.Repository, source, target string) (bool, error) {
	if m.canFastForwardFunc != nil {
		return m.canFastForwardFunc(ctx, repo, source, target)
	}
	return false, nil
}

func TestMergeManager_Merge(t *testing.T) {
	tests := []struct {
		name          string
		opts          MergeOptions
		cleanTree     bool
		canFF         bool
		upToDate      bool
		mergeExitCode int
		mergeOutput   string
		wantSuccess   bool
		wantError     error
	}{
		{
			name: "successful fast-forward merge",
			opts: MergeOptions{
				Source:           "feature",
				Target:           "main",
				Strategy:         StrategyFastForward,
				AllowFastForward: true,
			},
			cleanTree:     true,
			canFF:         true,
			upToDate:      false,
			mergeExitCode: 0,
			mergeOutput:   "Fast-forward\n 1 file changed, 10 insertions(+), 5 deletions(-)",
			wantSuccess:   true,
		},
		{
			name: "already up to date",
			opts: MergeOptions{
				Source: "feature",
				Target: "main",
			},
			cleanTree:   true,
			canFF:       false,
			upToDate:    true,
			wantSuccess: true,
		},
		{
			name: "dirty working tree",
			opts: MergeOptions{
				Source: "feature",
				Target: "main",
			},
			cleanTree: false,
			wantError: ErrDirtyWorkingTree,
		},
		{
			name: "merge with conflicts",
			opts: MergeOptions{
				Source: "feature",
				Target: "main",
			},
			cleanTree:     true,
			canFF:         false,
			upToDate:      false,
			mergeExitCode: 1,
			mergeOutput:   "CONFLICT",
			wantSuccess:   false,
			wantError:     ErrMergeConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					// Handle rev-parse
					if len(args) > 0 && args[0] == "rev-parse" {
						if len(args) > 1 && args[1] == "--verify" {
							return &gitcmd.Result{Stdout: "abc123\n", ExitCode: 0}, nil
						}
						// For isAlreadyUpToDate check - return different hashes
						if len(args) > 1 {
							if args[1] == tt.opts.Source {
								return &gitcmd.Result{Stdout: "source-hash\n", ExitCode: 0}, nil
							}
							if args[1] == tt.opts.Target {
								return &gitcmd.Result{Stdout: "target-hash\n", ExitCode: 0}, nil
							}
						}
						return &gitcmd.Result{Stdout: "abc123\n", ExitCode: 0}, nil
					}

					// Handle status --porcelain
					if len(args) > 0 && args[0] == "status" {
						output := ""
						if !tt.cleanTree {
							output = "M file.go\n"
						}
						return &gitcmd.Result{Stdout: output, ExitCode: 0}, nil
					}

					// Handle merge
					if len(args) > 0 && args[0] == "merge" {
						return &gitcmd.Result{
							Stdout:   tt.mergeOutput,
							ExitCode: tt.mergeExitCode,
						}, nil
					}

					return &gitcmd.Result{Stdout: "", ExitCode: 0}, nil
				},
			}

			detector := &mockConflictDetector{
				canFastForwardFunc: func(ctx context.Context, repo *repository.Repository, source, target string) (bool, error) {
					return tt.canFF, nil
				},
				detectFunc: func(ctx context.Context, repo *repository.Repository, opts DetectOptions) (*ConflictReport, error) {
					var conflicts []*Conflict
					if tt.mergeExitCode != 0 {
						conflicts = []*Conflict{
							{FilePath: "file.go", ConflictType: ConflictContent},
						}
					}
					return &ConflictReport{
						TotalConflicts: len(conflicts),
						Conflicts:      conflicts,
					}, nil
				},
			}

			// Mock isAlreadyUpToDate check
			originalExecutor := executor
			if tt.upToDate {
				executor = &mockExecutor{
					runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
						if len(args) > 0 && args[0] == "rev-parse" {
							return &gitcmd.Result{Stdout: "same-hash\n", ExitCode: 0}, nil
						}
						return originalExecutor.runFunc(ctx, repoPath, args...)
					},
				}
			}

			manager := NewMergeManager(executor, detector)
			repo := &repository.Repository{Path: "/test/repo"}

			result, err := manager.Merge(context.Background(), repo, tt.opts)

			if tt.wantError != nil {
				if !errors.Is(err, tt.wantError) {
					t.Errorf("Merge() error = %v, want %v", err, tt.wantError)
				}
				return
			}

			if err != nil {
				t.Fatalf("Merge() unexpected error = %v", err)
			}

			if result.Success != tt.wantSuccess {
				t.Errorf("Success = %v, want %v", result.Success, tt.wantSuccess)
			}
		})
	}
}

func TestMergeManager_ValidateStrategy(t *testing.T) {
	tests := []struct {
		name      string
		opts      MergeOptions
		wantError error
	}{
		{
			name: "valid options",
			opts: MergeOptions{
				Source:   "feature",
				Target:   "main",
				Strategy: StrategyRecursive,
			},
			wantError: nil,
		},
		{
			name: "missing source",
			opts: MergeOptions{
				Target: "main",
			},
			wantError: errors.New("source branch is required"),
		},
		{
			name: "missing target",
			opts: MergeOptions{
				Source: "feature",
			},
			wantError: errors.New("target branch is required"),
		},
		{
			name: "invalid strategy",
			opts: MergeOptions{
				Source:   "feature",
				Target:   "main",
				Strategy: "invalid",
			},
			wantError: ErrInvalidStrategy,
		},
		{
			name: "octopus with single source",
			opts: MergeOptions{
				Source:   "feature",
				Target:   "main",
				Strategy: StrategyOctopus,
			},
			wantError: errors.New("octopus strategy requires multiple source branches"),
		},
		{
			name: "octopus with multiple sources",
			opts: MergeOptions{
				Source:   "feature1 feature2",
				Target:   "main",
				Strategy: StrategyOctopus,
			},
			wantError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					return &gitcmd.Result{Stdout: "abc123\n", ExitCode: 0}, nil
				},
			}

			detector := &mockConflictDetector{}
			manager := NewMergeManager(executor, detector)
			repo := &repository.Repository{Path: "/test/repo"}

			err := manager.ValidateStrategy(context.Background(), repo, tt.opts)

			if tt.wantError != nil {
				if err == nil {
					t.Errorf("ValidateStrategy() error = nil, want %v", tt.wantError)
					return
				}
				if !strings.Contains(err.Error(), tt.wantError.Error()) {
					t.Errorf("ValidateStrategy() error = %v, want %v", err, tt.wantError)
				}
			} else if err != nil {
				t.Errorf("ValidateStrategy() unexpected error = %v", err)
			}
		})
	}
}

func TestMergeManager_CanMerge(t *testing.T) {
	tests := []struct {
		name      string
		conflicts int
		want      bool
	}{
		{"no conflicts", 0, true},
		{"has conflicts", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{}
			detector := &mockConflictDetector{
				detectFunc: func(ctx context.Context, repo *repository.Repository, opts DetectOptions) (*ConflictReport, error) {
					return &ConflictReport{TotalConflicts: tt.conflicts}, nil
				},
			}

			manager := NewMergeManager(executor, detector)
			repo := &repository.Repository{Path: "/test/repo"}

			got, err := manager.CanMerge(context.Background(), repo, "feature", "main")
			if err != nil {
				t.Fatalf("CanMerge() unexpected error = %v", err)
			}

			if got != tt.want {
				t.Errorf("CanMerge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeManager_AbortMerge(t *testing.T) {
	tests := []struct {
		name      string
		exitCode  int
		wantError bool
	}{
		{"successful abort", 0, false},
		{"abort failed", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					return &gitcmd.Result{
						Stdout:   "",
						Stderr:   "abort failed",
						ExitCode: tt.exitCode,
					}, nil
				},
			}

			detector := &mockConflictDetector{}
			manager := NewMergeManager(executor, detector)
			repo := &repository.Repository{Path: "/test/repo"}

			err := manager.AbortMerge(context.Background(), repo)

			if (err != nil) != tt.wantError {
				t.Errorf("AbortMerge() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestMergeManager_BuildMergeArgs(t *testing.T) {
	tests := []struct {
		name            string
		opts            MergeOptions
		canFastForward  bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "fast-forward strategy",
			opts: MergeOptions{
				Source:   "feature",
				Strategy: StrategyFastForward,
			},
			canFastForward: true,
			wantContains:   []string{"merge", "--ff-only", "feature"},
		},
		{
			name: "recursive strategy",
			opts: MergeOptions{
				Source:   "feature",
				Strategy: StrategyRecursive,
			},
			canFastForward: false,
			wantContains:   []string{"merge", "--strategy=recursive", "feature"},
		},
		{
			name: "no-ff merge",
			opts: MergeOptions{
				Source:           "feature",
				AllowFastForward: false,
			},
			canFastForward: true,
			wantContains:   []string{"merge", "--no-ff", "feature"},
		},
		{
			name: "squash merge",
			opts: MergeOptions{
				Source: "feature",
				Squash: true,
			},
			canFastForward: false,
			wantContains:   []string{"merge", "--squash", "feature"},
		},
		{
			name: "no-commit merge",
			opts: MergeOptions{
				Source:   "feature",
				NoCommit: true,
			},
			canFastForward: false,
			wantContains:   []string{"merge", "--no-commit", "feature"},
		},
		{
			name: "with commit message",
			opts: MergeOptions{
				Source:        "feature",
				CommitMessage: "Merge feature branch",
			},
			canFastForward: false,
			wantContains:   []string{"merge", "-m", "Merge feature branch", "feature"},
		},
		{
			name: "ours strategy",
			opts: MergeOptions{
				Source:   "feature",
				Strategy: StrategyOurs,
			},
			canFastForward: false,
			wantContains:   []string{"merge", "--strategy=ours", "feature"},
		},
		{
			name: "theirs strategy",
			opts: MergeOptions{
				Source:   "feature",
				Strategy: StrategyTheirs,
			},
			canFastForward: false,
			wantContains:   []string{"merge", "--strategy-option=theirs", "feature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &mergeManager{}
			args := manager.buildMergeArgs(tt.opts, tt.canFastForward)

			for _, want := range tt.wantContains {
				found := false
				for _, arg := range args {
					if arg == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildMergeArgs() missing %q in %v", want, args)
				}
			}

			for _, notWant := range tt.wantNotContains {
				for _, arg := range args {
					if arg == notWant {
						t.Errorf("buildMergeArgs() should not contain %q in %v", notWant, args)
					}
				}
			}
		})
	}
}

func TestMergeManager_ParseStats(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		wantFiles     int
		wantAdditions int
		wantDeletions int
	}{
		{
			name:          "standard output",
			output:        "Merge made by the 'recursive' strategy.\n 3 files changed, 10 insertions(+), 5 deletions(-)",
			wantFiles:     3,
			wantAdditions: 10,
			wantDeletions: 5,
		},
		{
			name:          "single file",
			output:        " 1 files changed, 5 insertions(+), 2 deletions(-)",
			wantFiles:     1,
			wantAdditions: 5,
			wantDeletions: 2,
		},
		{
			name:          "no deletions",
			output:        " 2 files changed, 15 insertions(+), 0 deletions(-)",
			wantFiles:     2,
			wantAdditions: 15,
			wantDeletions: 0,
		},
		{
			name:          "empty output",
			output:        "",
			wantFiles:     0,
			wantAdditions: 0,
			wantDeletions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &mergeManager{}
			files, additions, deletions := manager.parseStats(tt.output)

			if files != tt.wantFiles {
				t.Errorf("parseStats() files = %d, want %d", files, tt.wantFiles)
			}

			if additions != tt.wantAdditions {
				t.Errorf("parseStats() additions = %d, want %d", additions, tt.wantAdditions)
			}

			if deletions != tt.wantDeletions {
				t.Errorf("parseStats() deletions = %d, want %d", deletions, tt.wantDeletions)
			}
		})
	}
}

func TestMergeManager_CheckCleanWorkingTree(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantError error
	}{
		{"clean tree", "", nil},
		{"dirty tree", "M file.go\n", ErrDirtyWorkingTree},
		{"untracked files", "?? new.go\n", ErrDirtyWorkingTree},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					return &gitcmd.Result{Stdout: tt.output, ExitCode: 0}, nil
				},
			}

			manager := &mergeManager{executor: executor}
			repo := &repository.Repository{Path: "/test/repo"}

			err := manager.checkCleanWorkingTree(context.Background(), repo)

			if err != tt.wantError {
				t.Errorf("checkCleanWorkingTree() error = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func TestMergeManager_IsAlreadyUpToDate(t *testing.T) {
	tests := []struct {
		name       string
		sourceHash string
		targetHash string
		want       bool
	}{
		{"same hash", "abc123", "abc123", true},
		{"different hash", "abc123", "def456", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					if len(args) > 1 {
						if args[1] == "feature" {
							return &gitcmd.Result{Stdout: tt.sourceHash + "\n", ExitCode: 0}, nil
						}
						if args[1] == "main" {
							return &gitcmd.Result{Stdout: tt.targetHash + "\n", ExitCode: 0}, nil
						}
					}
					return &gitcmd.Result{Stdout: "", ExitCode: 0}, nil
				},
			}

			manager := &mergeManager{executor: executor}
			repo := &repository.Repository{Path: "/test/repo"}

			got, err := manager.isAlreadyUpToDate(context.Background(), repo, "feature", "main")
			if err != nil {
				t.Fatalf("isAlreadyUpToDate() unexpected error = %v", err)
			}

			if got != tt.want {
				t.Errorf("isAlreadyUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}
