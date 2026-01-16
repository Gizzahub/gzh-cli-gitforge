// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

//go:build integration

package reposync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	repo "github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func TestDiagnosticExecutor_Integration(t *testing.T) {
	// Create a temporary directory with a git repository
	tempRepo := testutil.TempGitRepoWithCommit(t)
	defer os.RemoveAll(filepath.Dir(tempRepo))

	ctx := context.Background()
	executor := DiagnosticExecutor{
		Client: repo.NewClient(),
	}

	repos := []RepoSpec{
		{
			Name:       "test-repo",
			TargetPath: tempRepo,
			CloneURL:   "https://github.com/example/test.git",
		},
	}

	opts := DiagnosticOptions{
		SkipFetch:              true, // Skip fetch to avoid network dependency
		Parallel:               1,
		CheckWorkTree:          true,
		IncludeRecommendations: true,
	}

	report, err := executor.CheckHealth(ctx, repos, opts)
	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}

	if report == nil {
		t.Fatal("CheckHealth() returned nil report")
	}

	if len(report.Results) != 1 {
		t.Errorf("CheckHealth() returned %d results, want 1", len(report.Results))
	}

	health := report.Results[0]

	// Should be able to open the repository
	if health.Error != nil {
		t.Errorf("health check failed: %v", health.Error)
	}

	// Should have a current branch
	if health.CurrentBranch == "" {
		t.Error("CurrentBranch is empty")
	}

	// Should detect clean working tree (freshly created repo)
	if health.WorkTreeStatus != WorkTreeClean {
		t.Errorf("WorkTreeStatus = %v, want %v", health.WorkTreeStatus, WorkTreeClean)
	}

	// Should generate recommendation
	if health.Recommendation == "" {
		t.Error("Recommendation is empty")
	}

	t.Logf("Health check result: %+v", health)
}

func TestDiagnosticExecutor_WithModifiedFiles(t *testing.T) {
	tempRepo := testutil.TempGitRepoWithCommit(t)
	defer os.RemoveAll(filepath.Dir(tempRepo))

	// Modify the existing README file
	readmeFile := filepath.Join(tempRepo, "README.md")
	if err := os.WriteFile(readmeFile, []byte("modified content"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	ctx := context.Background()
	executor := DiagnosticExecutor{
		Client: repo.NewClient(),
	}

	repos := []RepoSpec{
		{
			Name:       "test-repo-dirty",
			TargetPath: tempRepo,
		},
	}

	opts := DiagnosticOptions{
		SkipFetch:              true,
		Parallel:               1,
		CheckWorkTree:          true,
		IncludeRecommendations: true,
	}

	report, err := executor.CheckHealth(ctx, repos, opts)
	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}

	health := report.Results[0]

	// Should detect dirty working tree
	if health.WorkTreeStatus != WorkTreeDirty {
		t.Errorf("WorkTreeStatus = %v, want %v", health.WorkTreeStatus, WorkTreeDirty)
	}

	// Should count modified files
	if health.ModifiedFiles == 0 {
		t.Error("ModifiedFiles = 0, want > 0")
	}

	// Should still be healthy or warning (not error, since we're not behind)
	if health.HealthStatus == HealthError {
		t.Errorf("HealthStatus = %v, expected healthy or warning", health.HealthStatus)
	}

	t.Logf("Dirty repo health: %+v", health)
}

func TestDiagnosticExecutor_Parallel(t *testing.T) {
	// Create multiple temporary repositories
	repo1 := testutil.TempGitRepoWithCommit(t)
	defer os.RemoveAll(filepath.Dir(repo1))

	repo2 := testutil.TempGitRepoWithCommit(t)
	defer os.RemoveAll(filepath.Dir(repo2))

	repo3 := testutil.TempGitRepoWithCommit(t)
	defer os.RemoveAll(filepath.Dir(repo3))

	ctx := context.Background()
	executor := DiagnosticExecutor{
		Client: repo.NewClient(),
	}

	repos := []RepoSpec{
		{Name: "repo1", TargetPath: repo1},
		{Name: "repo2", TargetPath: repo2},
		{Name: "repo3", TargetPath: repo3},
	}

	opts := DiagnosticOptions{
		SkipFetch:              true,
		Parallel:               3, // Check all in parallel
		CheckWorkTree:          true,
		IncludeRecommendations: true,
	}

	start := time.Now()
	report, err := executor.CheckHealth(ctx, repos, opts)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}

	if len(report.Results) != 3 {
		t.Errorf("CheckHealth() returned %d results, want 3", len(report.Results))
	}

	// Verify all repositories were checked
	for i, health := range report.Results {
		if health.Error != nil {
			t.Errorf("repo %d health check failed: %v", i, health.Error)
		}
		if health.CurrentBranch == "" {
			t.Errorf("repo %d has empty CurrentBranch", i)
		}
	}

	// Parallel execution should be faster than sequential
	// (though hard to guarantee in tests, just log for visibility)
	t.Logf("Parallel health check duration: %v", duration)
	t.Logf("Summary: %+v", report.Summary)
}

func TestCalculateSummary(t *testing.T) {
	results := []RepoHealth{
		{HealthStatus: HealthHealthy},
		{HealthStatus: HealthHealthy},
		{HealthStatus: HealthWarning},
		{HealthStatus: HealthError},
		{HealthStatus: HealthUnreachable},
	}

	summary := calculateSummary(results)

	if summary.Total != 5 {
		t.Errorf("Total = %d, want 5", summary.Total)
	}
	if summary.Healthy != 2 {
		t.Errorf("Healthy = %d, want 2", summary.Healthy)
	}
	if summary.Warning != 1 {
		t.Errorf("Warning = %d, want 1", summary.Warning)
	}
	if summary.Error != 1 {
		t.Errorf("Error = %d, want 1", summary.Error)
	}
	if summary.Unreachable != 1 {
		t.Errorf("Unreachable = %d, want 1", summary.Unreachable)
	}
}
