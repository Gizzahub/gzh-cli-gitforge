package merge

import (
	"context"
	"errors"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func TestRebaseManager_Rebase(t *testing.T) {
	tests := []struct {
		name           string
		opts           RebaseOptions
		cleanTree      bool
		inProgress     bool
		rebaseExitCode int
		rebaseOutput   string
		wantSuccess    bool
		wantError      error
	}{
		{
			name: "successful rebase",
			opts: RebaseOptions{
				Branch: "main",
			},
			cleanTree:      true,
			inProgress:     false,
			rebaseExitCode: 0,
			rebaseOutput:   "Successfully rebased and updated refs/heads/feature.",
			wantSuccess:    true,
		},
		{
			name: "rebase already in progress",
			opts: RebaseOptions{
				Branch: "main",
			},
			cleanTree:  true,
			inProgress: true,
			wantError:  ErrRebaseInProgress,
		},
		{
			name: "dirty working tree",
			opts: RebaseOptions{
				Branch: "main",
			},
			cleanTree: false,
			wantError: ErrDirtyWorkingTree,
		},
		{
			name: "rebase with conflicts",
			opts: RebaseOptions{
				Branch: "main",
			},
			cleanTree:      true,
			inProgress:     false,
			rebaseExitCode: 1,
			rebaseOutput:   "CONFLICT (content): Merge conflict in file.go",
			wantSuccess:    false,
		},
		{
			name:      "missing required options",
			opts:      RebaseOptions{},
			wantError: errors.New("branch, onto, or upstream is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					// Handle rev-parse --git-path
					if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-path" {
						return &gitcmd.Result{Stdout: ".git/rebase-merge\n", ExitCode: 0}, nil
					}

					// Handle test -d (check rebase in progress)
					if len(args) > 0 && args[0] == "test" {
						exitCode := 1
						if tt.inProgress {
							exitCode = 0
						}
						return &gitcmd.Result{ExitCode: exitCode}, nil
					}

					// Handle status --porcelain
					if len(args) > 0 && args[0] == "status" {
						output := ""
						if !tt.cleanTree {
							output = "M file.go\n"
						}
						return &gitcmd.Result{Stdout: output, ExitCode: 0}, nil
					}

					// Handle rebase
					if len(args) > 0 && args[0] == "rebase" {
						return &gitcmd.Result{
							Stdout:   tt.rebaseOutput,
							ExitCode: tt.rebaseExitCode,
						}, nil
					}

					// Handle rev-parse HEAD
					if len(args) > 0 && args[0] == "rev-parse" {
						return &gitcmd.Result{Stdout: "abc123\n", ExitCode: 0}, nil
					}

					return &gitcmd.Result{Stdout: "", ExitCode: 0}, nil
				},
			}

			manager := NewRebaseManager(executor)
			repo := &repository.Repository{Path: "/test/repo"}

			result, err := manager.Rebase(context.Background(), repo, tt.opts)

			if tt.wantError != nil {
				if err == nil {
					t.Errorf("Rebase() error = nil, want %v", tt.wantError)
					return
				}
				return
			}

			if err != nil {
				t.Fatalf("Rebase() unexpected error = %v", err)
			}

			if result.Success != tt.wantSuccess {
				t.Errorf("Success = %v, want %v", result.Success, tt.wantSuccess)
			}
		})
	}
}

func TestRebaseManager_Continue(t *testing.T) {
	tests := []struct {
		name        string
		inProgress  bool
		exitCode    int
		output      string
		wantSuccess bool
		wantError   error
	}{
		{
			name:        "successful continue",
			inProgress:  true,
			exitCode:    0,
			output:      "Successfully rebased and updated refs/heads/feature.",
			wantSuccess: true,
		},
		{
			name:       "no rebase in progress",
			inProgress: false,
			wantError:  ErrNoRebaseInProgress,
		},
		{
			name:        "continue with more conflicts",
			inProgress:  true,
			exitCode:    1,
			output:      "CONFLICT (content): Merge conflict in file2.go",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					// Handle rev-parse --git-path
					if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-path" {
						return &gitcmd.Result{Stdout: ".git/rebase-merge\n", ExitCode: 0}, nil
					}

					// Handle test -d
					if len(args) > 0 && args[0] == "test" {
						exitCode := 1
						if tt.inProgress {
							exitCode = 0
						}
						return &gitcmd.Result{ExitCode: exitCode}, nil
					}

					// Handle rebase --continue
					if len(args) > 0 && args[0] == "rebase" && args[1] == "--continue" {
						return &gitcmd.Result{
							Stdout:   tt.output,
							ExitCode: tt.exitCode,
						}, nil
					}

					// Handle rev-parse HEAD
					if len(args) > 0 && args[0] == "rev-parse" {
						return &gitcmd.Result{Stdout: "abc123\n", ExitCode: 0}, nil
					}

					return &gitcmd.Result{Stdout: "", ExitCode: 0}, nil
				},
			}

			manager := NewRebaseManager(executor)
			repo := &repository.Repository{Path: "/test/repo"}

			result, err := manager.Continue(context.Background(), repo)

			if tt.wantError != nil {
				if err != tt.wantError {
					t.Errorf("Continue() error = %v, want %v", err, tt.wantError)
				}
				return
			}

			if err != nil {
				t.Fatalf("Continue() unexpected error = %v", err)
			}

			if result.Success != tt.wantSuccess {
				t.Errorf("Success = %v, want %v", result.Success, tt.wantSuccess)
			}
		})
	}
}

func TestRebaseManager_Skip(t *testing.T) {
	tests := []struct {
		name        string
		inProgress  bool
		exitCode    int
		output      string
		wantSuccess bool
		wantError   error
	}{
		{
			name:        "successful skip",
			inProgress:  true,
			exitCode:    0,
			output:      "Successfully rebased and updated refs/heads/feature.",
			wantSuccess: true,
		},
		{
			name:       "no rebase in progress",
			inProgress: false,
			wantError:  ErrNoRebaseInProgress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					// Handle rev-parse --git-path
					if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-path" {
						return &gitcmd.Result{Stdout: ".git/rebase-merge\n", ExitCode: 0}, nil
					}

					// Handle test -d
					if len(args) > 0 && args[0] == "test" {
						exitCode := 1
						if tt.inProgress {
							exitCode = 0
						}
						return &gitcmd.Result{ExitCode: exitCode}, nil
					}

					// Handle rebase --skip
					if len(args) > 0 && args[0] == "rebase" && args[1] == "--skip" {
						return &gitcmd.Result{
							Stdout:   tt.output,
							ExitCode: tt.exitCode,
						}, nil
					}

					// Handle rev-parse HEAD
					if len(args) > 0 && args[0] == "rev-parse" {
						return &gitcmd.Result{Stdout: "abc123\n", ExitCode: 0}, nil
					}

					return &gitcmd.Result{Stdout: "", ExitCode: 0}, nil
				},
			}

			manager := NewRebaseManager(executor)
			repo := &repository.Repository{Path: "/test/repo"}

			result, err := manager.Skip(context.Background(), repo)

			if tt.wantError != nil {
				if err != tt.wantError {
					t.Errorf("Skip() error = %v, want %v", err, tt.wantError)
				}
				return
			}

			if err != nil {
				t.Fatalf("Skip() unexpected error = %v", err)
			}

			if result.Success != tt.wantSuccess {
				t.Errorf("Success = %v, want %v", result.Success, tt.wantSuccess)
			}
		})
	}
}

func TestRebaseManager_Abort(t *testing.T) {
	tests := []struct {
		name       string
		inProgress bool
		exitCode   int
		wantError  error
	}{
		{
			name:       "successful abort",
			inProgress: true,
			exitCode:   0,
			wantError:  nil,
		},
		{
			name:       "no rebase in progress",
			inProgress: false,
			wantError:  ErrNoRebaseInProgress,
		},
		{
			name:       "abort failed",
			inProgress: true,
			exitCode:   1,
			wantError:  errors.New("rebase abort failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					// Handle rev-parse --git-path
					if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-path" {
						return &gitcmd.Result{Stdout: ".git/rebase-merge\n", ExitCode: 0}, nil
					}

					// Handle test -d
					if len(args) > 0 && args[0] == "test" {
						exitCode := 1
						if tt.inProgress {
							exitCode = 0
						}
						return &gitcmd.Result{ExitCode: exitCode}, nil
					}

					// Handle rebase --abort
					if len(args) > 0 && args[0] == "rebase" && args[1] == "--abort" {
						return &gitcmd.Result{
							Stderr:   "abort error",
							ExitCode: tt.exitCode,
						}, nil
					}

					return &gitcmd.Result{Stdout: "", ExitCode: 0}, nil
				},
			}

			manager := NewRebaseManager(executor)
			repo := &repository.Repository{Path: "/test/repo"}

			err := manager.Abort(context.Background(), repo)

			if tt.wantError != nil {
				if err == nil {
					t.Errorf("Abort() error = nil, want error")
					return
				}
				return
			}

			if err != nil {
				t.Errorf("Abort() unexpected error = %v", err)
			}
		})
	}
}

func TestRebaseManager_Status(t *testing.T) {
	tests := []struct {
		name       string
		inProgress bool
		want       RebaseStatus
	}{
		{"rebase in progress", true, RebaseInProgress},
		{"no rebase", false, RebaseComplete},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					// Handle rev-parse --git-path
					if len(args) > 1 && args[0] == "rev-parse" && args[1] == "--git-path" {
						return &gitcmd.Result{Stdout: ".git/rebase-merge\n", ExitCode: 0}, nil
					}

					// Handle test -d
					if len(args) > 0 && args[0] == "test" {
						exitCode := 1
						if tt.inProgress {
							exitCode = 0
						}
						return &gitcmd.Result{ExitCode: exitCode}, nil
					}

					return &gitcmd.Result{Stdout: "", ExitCode: 0}, nil
				},
			}

			manager := NewRebaseManager(executor)
			repo := &repository.Repository{Path: "/test/repo"}

			got, err := manager.Status(context.Background(), repo)
			if err != nil {
				t.Fatalf("Status() unexpected error = %v", err)
			}

			if got != tt.want {
				t.Errorf("Status() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRebaseManager_BuildRebaseArgs(t *testing.T) {
	tests := []struct {
		name         string
		opts         RebaseOptions
		wantContains []string
	}{
		{
			name: "basic rebase",
			opts: RebaseOptions{
				Branch: "main",
			},
			wantContains: []string{"rebase", "main"},
		},
		{
			name: "interactive rebase",
			opts: RebaseOptions{
				Branch:      "main",
				Interactive: true,
			},
			wantContains: []string{"rebase", "-i", "main"},
		},
		{
			name: "auto-squash",
			opts: RebaseOptions{
				Branch:     "main",
				AutoSquash: true,
			},
			wantContains: []string{"rebase", "--autosquash", "main"},
		},
		{
			name: "preserve merges",
			opts: RebaseOptions{
				Branch:         "main",
				PreserveMerges: true,
			},
			wantContains: []string{"rebase", "--preserve-merges", "main"},
		},
		{
			name: "onto option",
			opts: RebaseOptions{
				Branch: "main",
				Onto:   "develop",
			},
			wantContains: []string{"rebase", "--onto", "develop", "main"},
		},
		{
			name: "upstream name",
			opts: RebaseOptions{
				UpstreamName: "origin/main",
			},
			wantContains: []string{"rebase", "origin/main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &rebaseManager{}
			args := manager.buildRebaseArgs(tt.opts)

			for _, want := range tt.wantContains {
				found := false
				for _, arg := range args {
					if arg == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildRebaseArgs() missing %q in %v", want, args)
				}
			}
		})
	}
}

func TestRebaseManager_ParseRebaseResult(t *testing.T) {
	tests := []struct {
		name        string
		result      *gitcmd.Result
		wantSuccess bool
		wantCommits int
	}{
		{
			name: "successful rebase",
			result: &gitcmd.Result{
				Stdout: "Successfully rebased and updated refs/heads/feature.",
			},
			wantSuccess: true,
			wantCommits: 1,
		},
		{
			name: "rebase with multiple commits",
			result: &gitcmd.Result{
				Stdout: "Rebasing (1/3)\nRebasing (2/3)\nRebasing (3/3)",
			},
			wantSuccess: true,
			wantCommits: 3,
		},
		{
			name: "empty output",
			result: &gitcmd.Result{
				Stdout: "",
			},
			wantSuccess: true,
			wantCommits: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &rebaseManager{}
			result, err := manager.parseRebaseResult(tt.result)
			if err != nil {
				t.Fatalf("parseRebaseResult() unexpected error = %v", err)
			}

			if result.Success != tt.wantSuccess {
				t.Errorf("Success = %v, want %v", result.Success, tt.wantSuccess)
			}

			if result.CommitsRebased != tt.wantCommits {
				t.Errorf("CommitsRebased = %d, want %d", result.CommitsRebased, tt.wantCommits)
			}
		})
	}
}
