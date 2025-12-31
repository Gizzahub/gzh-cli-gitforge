package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initTestGitRepo creates a temporary git repository with initial commit for testing.
// Returns the resolved (real) path to avoid symlink issues on macOS.
func initTestGitRepo(t *testing.T, dir string) string {
	t.Helper()

	// Resolve symlinks (macOS /var -> /private/var)
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		realDir = dir
	}

	// Create a file first
	testFile := filepath.Join(realDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repository\n"), 0o644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "add", "."},
		{"git", "commit", "-m", "Initial commit"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = realDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Failed to run %v: %v\nOutput: %s", args, err, output)
		}
	}

	return realDir
}

// TestIntegration_Client_Open tests opening a repository.
func TestIntegration_Client_Open(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	client := NewClient()

	// Test opening a valid repository
	repo, err := client.Open(ctx, repoDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if repo == nil {
		t.Fatal("Open() returned nil repository")
	}

	if repo.Path != repoDir {
		t.Errorf("Open().Path = %q, want %q", repo.Path, repoDir)
	}
}

// TestIntegration_Client_Open_NotARepo tests opening a non-git directory.
func TestIntegration_Client_Open_NotARepo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	nonGitDir := t.TempDir()

	ctx := context.Background()
	client := NewClient()

	// Test opening a non-git directory
	_, err := client.Open(ctx, nonGitDir)
	if err == nil {
		t.Error("Open() should return error for non-git directory")
	}
}

// TestIntegration_Client_IsRepository tests repository detection.
func TestIntegration_Client_IsRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())
	nonGitDir := t.TempDir()

	ctx := context.Background()
	client := NewClient()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "valid git repository",
			path: repoDir,
			want: true,
		},
		{
			name: "non-git directory",
			path: nonGitDir,
			want: false,
		},
		{
			name: "non-existent path",
			path: "/nonexistent/path/12345",
			want: false,
		},
		{
			name: "empty path",
			path: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.IsRepository(ctx, tt.path)
			if got != tt.want {
				t.Errorf("IsRepository() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIntegration_Client_GetInfo tests retrieving repository info.
func TestIntegration_Client_GetInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	client := NewClient()

	repo, err := client.Open(ctx, repoDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	info, err := client.GetInfo(ctx, repo)
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}

	// Branch should be main or master
	if info.Branch != "main" && info.Branch != "master" {
		t.Errorf("GetInfo().Branch = %q, want 'main' or 'master'", info.Branch)
	}
}

// TestIntegration_Client_GetStatus tests retrieving repository status.
func TestIntegration_Client_GetStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	client := NewClient()

	repo, err := client.Open(ctx, repoDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// Clean status
	status, err := client.GetStatus(ctx, repo)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if !status.IsClean {
		t.Error("GetStatus().IsClean should be true for clean repo")
	}

	// Create an untracked file
	testFile := filepath.Join(repoDir, "untracked.txt")
	if err := os.WriteFile(testFile, []byte("untracked content"), 0o644); err != nil {
		t.Fatalf("Failed to create untracked file: %v", err)
	}

	// Dirty status
	status, err = client.GetStatus(ctx, repo)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.IsClean {
		t.Error("GetStatus().IsClean should be false with untracked file")
	}

	if len(status.UntrackedFiles) != 1 {
		t.Errorf("GetStatus().UntrackedFiles = %v, want 1 file", status.UntrackedFiles)
	}
}

// TestIntegration_Client_GetStatus_Modified tests detecting modified files.
func TestIntegration_Client_GetStatus_Modified(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	client := NewClient()

	repo, err := client.Open(ctx, repoDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// Modify existing file
	readmeFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Modified\n"), 0o644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	status, err := client.GetStatus(ctx, repo)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.IsClean {
		t.Error("GetStatus().IsClean should be false with modified file")
	}

	// Debug output
	t.Logf("ModifiedFiles: %v", status.ModifiedFiles)
	t.Logf("StagedFiles: %v", status.StagedFiles)
	t.Logf("UntrackedFiles: %v", status.UntrackedFiles)

	// Modified files can appear in worktree (unstaged) or index (staged)
	// Either is acceptable for this test
	totalChanges := len(status.ModifiedFiles) + len(status.StagedFiles)
	if totalChanges == 0 {
		t.Error("GetStatus() should detect modified file (in ModifiedFiles or StagedFiles)")
	}
}

// TestIntegration_Client_GetStatus_Staged tests detecting staged files.
func TestIntegration_Client_GetStatus_Staged(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	client := NewClient()

	repo, err := client.Open(ctx, repoDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// Create and stage a new file
	testFile := filepath.Join(repoDir, "staged.txt")
	if err := os.WriteFile(testFile, []byte("staged content"), 0o644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	cmd := exec.Command("git", "add", "staged.txt")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to stage file: %v", err)
	}

	status, err := client.GetStatus(ctx, repo)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.IsClean {
		t.Error("GetStatus().IsClean should be false with staged file")
	}

	if len(status.StagedFiles) != 1 {
		t.Errorf("GetStatus().StagedFiles = %v, want 1 file", status.StagedFiles)
	}
}

// TestIntegration_CloneOrUpdate_Validation tests input validation.
func TestIntegration_CloneOrUpdate_Validation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := NewClient()

	tests := []struct {
		name    string
		opts    CloneOrUpdateOptions
		wantErr bool
	}{
		{
			name:    "empty URL",
			opts:    CloneOrUpdateOptions{URL: "", Destination: "/tmp/test"},
			wantErr: true,
		},
		{
			name:    "empty Destination",
			opts:    CloneOrUpdateOptions{URL: "https://example.com/repo.git", Destination: ""},
			wantErr: true,
		},
		{
			name:    "invalid strategy",
			opts:    CloneOrUpdateOptions{URL: "https://example.com/repo.git", Destination: "/tmp/test", Strategy: "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.CloneOrUpdate(ctx, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("CloneOrUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestIntegration_CloneOrUpdate_SkipStrategy tests skip strategy for existing repos.
func TestIntegration_CloneOrUpdate_SkipStrategy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())

	ctx := context.Background()
	client := NewClient()

	result, err := client.CloneOrUpdate(ctx, CloneOrUpdateOptions{
		URL:         "https://github.com/test/repo.git", // URL doesn't matter for skip
		Destination: repoDir,
		Strategy:    StrategySkip,
	})
	if err != nil {
		t.Fatalf("CloneOrUpdate() error = %v", err)
	}

	if result.Action != "skipped" {
		t.Errorf("CloneOrUpdate().Action = %q, want 'skipped'", result.Action)
	}

	if result.StrategyUsed != StrategySkip {
		t.Errorf("CloneOrUpdate().StrategyUsed = %q, want StrategySkip", result.StrategyUsed)
	}

	if !result.Success {
		t.Error("CloneOrUpdate().Success should be true")
	}
}

// TestIntegration_CloneOrUpdate_NonGitDirectory tests error for non-git directory.
func TestIntegration_CloneOrUpdate_NonGitDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	nonGitDir := t.TempDir()

	ctx := context.Background()
	client := NewClient()

	// Without Force, should error
	_, err := client.CloneOrUpdate(ctx, CloneOrUpdateOptions{
		URL:         "https://github.com/test/repo.git",
		Destination: nonGitDir,
		Strategy:    StrategyRebase,
	})
	if err == nil {
		t.Error("CloneOrUpdate() should return error for non-git directory")
	}
}

// TestExtractRepoNameFromURL tests URL parsing.
func TestExtractRepoNameFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name:    "HTTPS URL with .git suffix",
			url:     "https://github.com/user/repo.git",
			want:    "repo",
			wantErr: false,
		},
		{
			name:    "HTTPS URL without .git suffix",
			url:     "https://github.com/user/repo",
			want:    "repo",
			wantErr: false,
		},
		{
			name:    "SSH URL with .git suffix",
			url:     "git@github.com:user/repo.git",
			want:    "repo",
			wantErr: false,
		},
		{
			name:    "SSH URL without .git suffix",
			url:     "git@github.com:user/repo",
			want:    "repo",
			wantErr: false,
		},
		{
			name:    "SSH URL with ssh:// prefix",
			url:     "ssh://git@github.com/user/repo.git",
			want:    "repo",
			wantErr: false,
		},
		{
			name:    "repo name with dashes",
			url:     "https://github.com/user/my-repo-name.git",
			want:    "my-repo-name",
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractRepoNameFromURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractRepoNameFromURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractRepoNameFromURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCheckTargetDirectory tests directory check helper.
func TestCheckTargetDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoDir := initTestGitRepo(t, t.TempDir())
	nonGitDir := t.TempDir()

	tests := []struct {
		name       string
		path       string
		wantExists bool
		wantIsGit  bool
		wantErr    bool
	}{
		{
			name:       "existing git repo",
			path:       repoDir,
			wantExists: true,
			wantIsGit:  true,
			wantErr:    false,
		},
		{
			name:       "existing non-git directory",
			path:       nonGitDir,
			wantExists: true,
			wantIsGit:  false,
			wantErr:    false,
		},
		{
			name:       "non-existent path",
			path:       "/nonexistent/path/12345",
			wantExists: false,
			wantIsGit:  false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, isGit, err := checkTargetDirectory(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkTargetDirectory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if exists != tt.wantExists {
				t.Errorf("checkTargetDirectory() exists = %v, want %v", exists, tt.wantExists)
			}
			if isGit != tt.wantIsGit {
				t.Errorf("checkTargetDirectory() isGit = %v, want %v", isGit, tt.wantIsGit)
			}
		})
	}
}

// TestIsValidUpdateStrategy tests strategy validation.
func TestIsValidUpdateStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy UpdateStrategy
		want     bool
	}{
		{"rebase", StrategyRebase, true},
		{"reset", StrategyReset, true},
		{"clone", StrategyClone, true},
		{"skip", StrategySkip, true},
		{"pull", StrategyPull, true},
		{"fetch", StrategyFetch, true},
		{"invalid", UpdateStrategy("invalid"), false},
		{"empty", UpdateStrategy(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidUpdateStrategy(tt.strategy)
			if got != tt.want {
				t.Errorf("isValidUpdateStrategy(%q) = %v, want %v", tt.strategy, got, tt.want)
			}
		})
	}
}

// TestGetValidStrategies tests valid strategies list.
func TestGetValidStrategies(t *testing.T) {
	result := getValidStrategies()
	if result == "" {
		t.Error("getValidStrategies() should not return empty string")
	}
}

// TestNewNoopProgress tests NoopProgress creation.
func TestNewNoopProgress(t *testing.T) {
	progress := NewNoopProgress()
	if progress == nil {
		t.Fatal("NewNoopProgress() returned nil")
	}

	// These should not panic
	progress.Start(100)
	progress.Update(50)
	progress.Done()
}

// TestWriterLogger tests WriterLogger functionality.
func TestWriterLogger(t *testing.T) {
	// Test with nil writer (should not panic)
	nilLogger := &WriterLogger{w: nil}
	nilLogger.Debug("test")
	nilLogger.Info("test")
	nilLogger.Warn("test")
	nilLogger.Error("test")

	// Test with actual writer
	var buf testBuffer
	logger := NewWriterLogger(&buf)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	if output == "" {
		t.Error("WriterLogger should write output")
	}
}

// testBuffer is a simple buffer for testing WriterLogger.
type testBuffer struct {
	data []byte
}

func (b *testBuffer) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *testBuffer) String() string {
	return string(b.data)
}

// TestFormatValue tests value formatting helper.
func TestFormatValue(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"string", "test"},
		{"int", 42},
		{"int64", int64(42)},
		{"uint", uint(42)},
		{"float64", 3.14},
		{"bool", true},
		{"struct", struct{ X int }{X: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.value)
			if result == "" {
				t.Error("formatValue() should not return empty string")
			}
		})
	}
}
