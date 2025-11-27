package merge

import (
	"context"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-git/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

type mockExecutor struct {
	runFunc func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error)
}

func (m *mockExecutor) Run(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, repoPath, args...)
	}
	return &gitcmd.Result{Stdout: "", Stderr: "", ExitCode: 0}, nil
}

func TestConflictDetector_Detect(t *testing.T) {
	tests := []struct {
		name           string
		opts           DetectOptions
		mergeBase      string
		sourceChanges  string
		targetChanges  string
		wantConflicts  int
		wantDifficulty MergeDifficulty
	}{
		{
			name: "no conflicts",
			opts: DetectOptions{
				Source: "feature",
				Target: "main",
			},
			mergeBase:      "abc123",
			sourceChanges:  "A\tfile1.go\nM\tfile2.go\n",
			targetChanges:  "A\tfile3.go\nM\tfile4.go\n",
			wantConflicts:  0,
			wantDifficulty: DifficultyTrivial,
		},
		{
			name: "content conflict",
			opts: DetectOptions{
				Source: "feature",
				Target: "main",
			},
			mergeBase:      "abc123",
			sourceChanges:  "M\tfile1.go\n",
			targetChanges:  "M\tfile1.go\n",
			wantConflicts:  1,
			wantDifficulty: DifficultyMedium,
		},
		{
			name: "delete-modify conflict",
			opts: DetectOptions{
				Source: "feature",
				Target: "main",
			},
			mergeBase:      "abc123",
			sourceChanges:  "D\tfile1.go\n",
			targetChanges:  "M\tfile1.go\n",
			wantConflicts:  1,
			wantDifficulty: DifficultyMedium,
		},
		{
			name: "rename conflict",
			opts: DetectOptions{
				Source: "feature",
				Target: "main",
			},
			mergeBase:      "abc123",
			sourceChanges:  "R100\told.go\tnew.go\n",
			targetChanges:  "R100\told.go\tnew.go\n",
			wantConflicts:  1,
			wantDifficulty: DifficultyEasy,
		},
		{
			name: "multiple conflicts - hard",
			opts: DetectOptions{
				Source: "feature",
				Target: "main",
			},
			mergeBase:     "abc123",
			sourceChanges: "M\tfile1.go\nM\tfile2.go\nM\tfile3.go\nM\tfile4.go\nM\tfile5.go\nM\tfile6.go\n",
			targetChanges: "M\tfile1.go\nM\tfile2.go\nM\tfile3.go\nM\tfile4.go\nM\tfile5.go\nM\tfile6.go\n",
			wantConflicts: 6,
			wantDifficulty: DifficultyHard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					// Handle rev-parse (branch validation)
					if len(args) > 0 && args[0] == "rev-parse" {
						return &gitcmd.Result{Stdout: "abc123\n", Stderr: "", ExitCode: 0}, nil
					}

					// Handle merge-base
					if len(args) > 0 && args[0] == "merge-base" {
						if len(args) > 1 && args[1] == "--is-ancestor" {
							return &gitcmd.Result{Stdout: "", Stderr: "", ExitCode: 1}, nil
						}
						return &gitcmd.Result{Stdout: tt.mergeBase + "\n", Stderr: "", ExitCode: 0}, nil
					}

					// Handle diff --name-status
					if len(args) > 0 && args[0] == "diff" {
						diffRange := args[2]
						if strings.Contains(diffRange, tt.opts.Source) {
							return &gitcmd.Result{Stdout: tt.sourceChanges, Stderr: "", ExitCode: 0}, nil
						}
						return &gitcmd.Result{Stdout: tt.targetChanges, Stderr: "", ExitCode: 0}, nil
					}

					return &gitcmd.Result{Stdout: "", Stderr: "", ExitCode: 0}, nil
				},
			}

			detector := NewConflictDetector(executor)
			repo := &repository.Repository{Path: "/test/repo"}

			report, err := detector.Detect(context.Background(), repo, tt.opts)
			if err != nil {
				t.Fatalf("Detect() unexpected error = %v", err)
			}

			if report.TotalConflicts != tt.wantConflicts {
				t.Errorf("TotalConflicts = %d, want %d", report.TotalConflicts, tt.wantConflicts)
			}

			if report.Difficulty != tt.wantDifficulty {
				t.Errorf("Difficulty = %s, want %s", report.Difficulty, tt.wantDifficulty)
			}

			if report.Source != tt.opts.Source {
				t.Errorf("Source = %s, want %s", report.Source, tt.opts.Source)
			}

			if report.Target != tt.opts.Target {
				t.Errorf("Target = %s, want %s", report.Target, tt.opts.Target)
			}

			if report.MergeBase != tt.mergeBase {
				t.Errorf("MergeBase = %s, want %s", report.MergeBase, tt.mergeBase)
			}
		})
	}
}

func TestConflictDetector_Preview(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		target         string
		canFF          bool
		changes        string
		wantFilesAdd   int
		wantFilesChange int
		wantFilesDelete int
	}{
		{
			name:           "simple preview",
			source:         "feature",
			target:         "main",
			canFF:          false,
			changes:        "A\tfile1.go\nM\tfile2.go\nD\tfile3.go\n",
			wantFilesAdd:   1,
			wantFilesChange: 1,
			wantFilesDelete: 1,
		},
		{
			name:           "fast-forward possible",
			source:         "feature",
			target:         "main",
			canFF:          true,
			changes:        "A\tfile1.go\n",
			wantFilesAdd:   1,
			wantFilesChange: 0,
			wantFilesDelete: 0,
		},
		{
			name:           "with renames",
			source:         "feature",
			target:         "main",
			canFF:          false,
			changes:        "R100\told.go\tnew.go\n",
			wantFilesAdd:   0,
			wantFilesChange: 1,
			wantFilesDelete: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					// Handle rev-parse
					if len(args) > 0 && args[0] == "rev-parse" {
						return &gitcmd.Result{Stdout: "abc123\n", Stderr: "", ExitCode: 0}, nil
					}

					// Handle merge-base
					if len(args) > 0 && args[0] == "merge-base" {
						if len(args) > 1 && args[1] == "--is-ancestor" {
							exitCode := 1
							if tt.canFF {
								exitCode = 0
							}
							return &gitcmd.Result{Stdout: "", Stderr: "", ExitCode: exitCode}, nil
						}
						return &gitcmd.Result{Stdout: "abc123\n", Stderr: "", ExitCode: 0}, nil
					}

					// Handle diff
					if len(args) > 0 && args[0] == "diff" {
						return &gitcmd.Result{Stdout: tt.changes, Stderr: "", ExitCode: 0}, nil
					}

					return &gitcmd.Result{Stdout: "", Stderr: "", ExitCode: 0}, nil
				},
			}

			detector := NewConflictDetector(executor)
			repo := &repository.Repository{Path: "/test/repo"}

			preview, err := detector.Preview(context.Background(), repo, tt.source, tt.target)
			if err != nil {
				t.Fatalf("Preview() unexpected error = %v", err)
			}

			if preview.CanFastForward != tt.canFF {
				t.Errorf("CanFastForward = %v, want %v", preview.CanFastForward, tt.canFF)
			}

			if preview.FilesToAdd != tt.wantFilesAdd {
				t.Errorf("FilesToAdd = %d, want %d", preview.FilesToAdd, tt.wantFilesAdd)
			}

			if preview.FilesToChange != tt.wantFilesChange {
				t.Errorf("FilesToChange = %d, want %d", preview.FilesToChange, tt.wantFilesChange)
			}

			if preview.FilesToDelete != tt.wantFilesDelete {
				t.Errorf("FilesToDelete = %d, want %d", preview.FilesToDelete, tt.wantFilesDelete)
			}
		})
	}
}

func TestConflictDetector_CanFastForward(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		target     string
		isAncestor bool
		want       bool
	}{
		{
			name:       "can fast-forward",
			source:     "feature",
			target:     "main",
			isAncestor: true,
			want:       true,
		},
		{
			name:       "cannot fast-forward",
			source:     "feature",
			target:     "main",
			isAncestor: false,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					if len(args) > 0 && args[0] == "merge-base" && args[1] == "--is-ancestor" {
						exitCode := 1
						if tt.isAncestor {
							exitCode = 0
						}
						return &gitcmd.Result{Stdout: "", Stderr: "", ExitCode: exitCode}, nil
					}
					return &gitcmd.Result{Stdout: "", Stderr: "", ExitCode: 0}, nil
				},
			}

			detector := NewConflictDetector(executor)
			repo := &repository.Repository{Path: "/test/repo"}

			got, err := detector.CanFastForward(context.Background(), repo, tt.source, tt.target)
			if err != nil {
				t.Fatalf("CanFastForward() unexpected error = %v", err)
			}

			if got != tt.want {
				t.Errorf("CanFastForward() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConflictDetector_ValidateBranch(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		exitCode  int
		wantError error
	}{
		{
			name:      "valid branch",
			branch:    "main",
			exitCode:  0,
			wantError: nil,
		},
		{
			name:      "invalid branch",
			branch:    "nonexistent",
			exitCode:  1,
			wantError: ErrBranchNotFound,
		},
		{
			name:      "empty branch",
			branch:    "",
			exitCode:  0,
			wantError: ErrInvalidBranch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					return &gitcmd.Result{Stdout: "abc123\n", Stderr: "", ExitCode: tt.exitCode}, nil
				},
			}

			detector := &conflictDetector{executor: executor}
			repo := &repository.Repository{Path: "/test/repo"}

			err := detector.validateBranch(context.Background(), repo, tt.branch)
			if err != tt.wantError {
				t.Errorf("validateBranch() error = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func TestConflictDetector_FindMergeBase(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		target    string
		output    string
		exitCode  int
		want      string
		wantError bool
	}{
		{
			name:      "valid merge base",
			source:    "feature",
			target:    "main",
			output:    "abc123def456\n",
			exitCode:  0,
			want:      "abc123def456",
			wantError: false,
		},
		{
			name:      "no merge base",
			source:    "feature",
			target:    "main",
			output:    "",
			exitCode:  1,
			want:      "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					return &gitcmd.Result{
						Stdout:   tt.output,
						Stderr:   "",
						ExitCode: tt.exitCode,
					}, nil
				},
			}

			detector := &conflictDetector{executor: executor}
			repo := &repository.Repository{Path: "/test/repo"}

			got, err := detector.findMergeBase(context.Background(), repo, tt.source, tt.target)
			if (err != nil) != tt.wantError {
				t.Errorf("findMergeBase() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if got != tt.want {
				t.Errorf("findMergeBase() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestConflictDetector_GetChangedFiles(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantCount int
		wantFirst *FileChange
	}{
		{
			name:      "basic changes",
			output:    "A\tfile1.go\nM\tfile2.go\nD\tfile3.go\n",
			wantCount: 3,
			wantFirst: &FileChange{Path: "file1.go", ChangeType: ChangeAdded},
		},
		{
			name:      "with rename",
			output:    "R100\told.go\tnew.go\n",
			wantCount: 1,
			wantFirst: &FileChange{Path: "new.go", OldPath: "old.go", ChangeType: ChangeRenamed},
		},
		{
			name:      "empty output",
			output:    "",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					return &gitcmd.Result{Stdout: tt.output, Stderr: "", ExitCode: 0}, nil
				},
			}

			detector := &conflictDetector{executor: executor}
			repo := &repository.Repository{Path: "/test/repo"}

			changes, err := detector.getChangedFiles(context.Background(), repo, "base", "head")
			if err != nil {
				t.Fatalf("getChangedFiles() unexpected error = %v", err)
			}

			if len(changes) != tt.wantCount {
				t.Errorf("len(changes) = %d, want %d", len(changes), tt.wantCount)
			}

			if tt.wantFirst != nil && len(changes) > 0 {
				if changes[0].Path != tt.wantFirst.Path {
					t.Errorf("changes[0].Path = %s, want %s", changes[0].Path, tt.wantFirst.Path)
				}
				if changes[0].ChangeType != tt.wantFirst.ChangeType {
					t.Errorf("changes[0].ChangeType = %s, want %s", changes[0].ChangeType, tt.wantFirst.ChangeType)
				}
				if tt.wantFirst.OldPath != "" && changes[0].OldPath != tt.wantFirst.OldPath {
					t.Errorf("changes[0].OldPath = %s, want %s", changes[0].OldPath, tt.wantFirst.OldPath)
				}
			}
		})
	}
}

func TestConflictDetector_ParseChangeType(t *testing.T) {
	tests := []struct {
		status string
		want   ChangeType
	}{
		{"A", ChangeAdded},
		{"M", ChangeModified},
		{"D", ChangeDeleted},
		{"R100", ChangeRenamed},
		{"C", ChangeCopied},
		{"X", ChangeModified}, // Unknown defaults to modified
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			detector := &conflictDetector{}
			got := detector.parseChangeType(tt.status)
			if got != tt.want {
				t.Errorf("parseChangeType(%s) = %s, want %s", tt.status, got, tt.want)
			}
		})
	}
}

func TestConflictDetector_CalculateDifficulty(t *testing.T) {
	tests := []struct {
		name           string
		totalConflicts int
		canAutoResolve int
		want           MergeDifficulty
	}{
		{"no conflicts", 0, 0, DifficultyTrivial},
		{"all auto-resolvable", 3, 3, DifficultyEasy},
		{"few conflicts", 3, 1, DifficultyMedium},
		{"many conflicts", 10, 2, DifficultyHard},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := &conflictDetector{}
			got := detector.calculateDifficulty(tt.totalConflicts, tt.canAutoResolve)
			if got != tt.want {
				t.Errorf("calculateDifficulty() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestConflictDetector_AnalyzeConflict(t *testing.T) {
	tests := []struct {
		name         string
		sourceChange *FileChange
		targetChange *FileChange
		wantType     ConflictType
		wantSeverity ConflictSeverity
		wantAuto     bool
		wantNil      bool
	}{
		{
			name:         "both modified",
			sourceChange: &FileChange{ChangeType: ChangeModified},
			targetChange: &FileChange{ChangeType: ChangeModified},
			wantType:     ConflictContent,
			wantSeverity: SeverityMedium,
			wantAuto:     false,
			wantNil:      false,
		},
		{
			name:         "deleted vs modified",
			sourceChange: &FileChange{ChangeType: ChangeDeleted},
			targetChange: &FileChange{ChangeType: ChangeModified},
			wantType:     ConflictDelete,
			wantSeverity: SeverityHigh,
			wantAuto:     false,
			wantNil:      false,
		},
		{
			name:         "both renamed",
			sourceChange: &FileChange{ChangeType: ChangeRenamed, OldPath: "old.go"},
			targetChange: &FileChange{ChangeType: ChangeRenamed, OldPath: "old.go"},
			wantType:     ConflictRename,
			wantSeverity: SeverityLow,
			wantAuto:     true,
			wantNil:      false,
		},
		{
			name:         "no conflict",
			sourceChange: &FileChange{ChangeType: ChangeAdded},
			targetChange: &FileChange{ChangeType: ChangeAdded},
			wantNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := &conflictDetector{}
			conflict := detector.analyzeConflict("test.go", tt.sourceChange, tt.targetChange)

			if tt.wantNil {
				if conflict != nil {
					t.Errorf("analyzeConflict() = %+v, want nil", conflict)
				}
				return
			}

			if conflict == nil {
				t.Fatal("analyzeConflict() = nil, want conflict")
			}

			if conflict.ConflictType != tt.wantType {
				t.Errorf("ConflictType = %s, want %s", conflict.ConflictType, tt.wantType)
			}

			if conflict.Severity != tt.wantSeverity {
				t.Errorf("Severity = %s, want %s", conflict.Severity, tt.wantSeverity)
			}

			if conflict.AutoResolvable != tt.wantAuto {
				t.Errorf("AutoResolvable = %v, want %v", conflict.AutoResolvable, tt.wantAuto)
			}
		})
	}
}
