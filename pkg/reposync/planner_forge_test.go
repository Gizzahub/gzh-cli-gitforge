// Package reposync provides repository synchronization orchestration.
package reposync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// mockForgeProvider is a test implementation of ForgeProvider.
type mockForgeProvider struct {
	name     string
	orgRepos []*provider.Repository
	orgErr   error
	usrRepos []*provider.Repository
	usrErr   error
}

func (m *mockForgeProvider) Name() string { return m.name }

func (m *mockForgeProvider) ListOrganizationRepos(_ context.Context, _ string) ([]*provider.Repository, error) {
	return m.orgRepos, m.orgErr
}

func (m *mockForgeProvider) ListUserRepos(_ context.Context, _ string) ([]*provider.Repository, error) {
	return m.usrRepos, m.usrErr
}

func TestNewForgePlanner(t *testing.T) {
	p := &mockForgeProvider{name: "github"}
	cfg := ForgePlannerConfig{
		TargetPath:   "/tmp/repos",
		Organization: "myorg",
	}

	planner := NewForgePlanner(p, cfg)

	if planner == nil {
		t.Fatal("NewForgePlanner returned nil")
	}
	if planner.provider != p {
		t.Error("provider not set correctly")
	}
	if planner.config.TargetPath != cfg.TargetPath {
		t.Error("config not set correctly")
	}
}

func TestForgePlanner_Plan(t *testing.T) {
	t.Run("returns empty plan when no repos", func(t *testing.T) {
		p := &mockForgeProvider{name: "github", orgRepos: []*provider.Repository{}}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath:   "/tmp/repos",
			Organization: "myorg",
		})

		plan, err := planner.Plan(context.Background(), PlanRequest{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(plan.Actions) != 0 {
			t.Errorf("expected empty plan, got %d actions", len(plan.Actions))
		}
	})

	t.Run("creates clone actions for new repos", func(t *testing.T) {
		tmpDir := t.TempDir()
		repos := []*provider.Repository{
			{Name: "repo1", CloneURL: "https://github.com/org/repo1.git"},
			{Name: "repo2", CloneURL: "https://github.com/org/repo2.git"},
		}
		p := &mockForgeProvider{name: "github", orgRepos: repos}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath:     tmpDir,
			Organization:   "myorg",
			IncludePrivate: true,
		})

		plan, err := planner.Plan(context.Background(), PlanRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Actions) != 2 {
			t.Fatalf("expected 2 actions, got %d", len(plan.Actions))
		}
		for i, action := range plan.Actions {
			if action.Type != ActionClone {
				t.Errorf("action[%d]: expected clone, got %s", i, action.Type)
			}
		}
	})

	t.Run("creates update actions for existing repos", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create existing repo with .git directory
		repoDir := filepath.Join(tmpDir, "existing-repo")
		gitDir := filepath.Join(repoDir, ".git")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatal(err)
		}

		repos := []*provider.Repository{
			{Name: "existing-repo", CloneURL: "https://github.com/org/existing-repo.git"},
		}
		p := &mockForgeProvider{name: "github", orgRepos: repos}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath:     tmpDir,
			Organization:   "myorg",
			IncludePrivate: true,
		})

		plan, err := planner.Plan(context.Background(), PlanRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(plan.Actions))
		}
		if plan.Actions[0].Type != ActionUpdate {
			t.Errorf("expected update, got %s", plan.Actions[0].Type)
		}
	})

	t.Run("uses user repos when IsUser is true", func(t *testing.T) {
		tmpDir := t.TempDir()
		userRepos := []*provider.Repository{
			{Name: "user-repo", CloneURL: "https://github.com/user/user-repo.git"},
		}
		p := &mockForgeProvider{name: "github", usrRepos: userRepos}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath:     tmpDir,
			Organization:   "username",
			IsUser:         true,
			IncludePrivate: true,
		})

		plan, err := planner.Plan(context.Background(), PlanRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(plan.Actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(plan.Actions))
		}
		if plan.Actions[0].Repo.Name != "user-repo" {
			t.Errorf("expected user-repo, got %s", plan.Actions[0].Repo.Name)
		}
	})

	t.Run("uses default strategy when not specified", func(t *testing.T) {
		tmpDir := t.TempDir()
		repos := []*provider.Repository{
			{Name: "repo1", CloneURL: "https://github.com/org/repo1.git"},
		}
		p := &mockForgeProvider{name: "github", orgRepos: repos}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath:     tmpDir,
			Organization:   "myorg",
			IncludePrivate: true,
		})

		plan, err := planner.Plan(context.Background(), PlanRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.Actions[0].Strategy != StrategyReset {
			t.Errorf("expected reset strategy, got %s", plan.Actions[0].Strategy)
		}
	})

	t.Run("returns error when provider fails", func(t *testing.T) {
		p := &mockForgeProvider{
			name:   "github",
			orgErr: context.DeadlineExceeded,
		}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath:   "/tmp/repos",
			Organization: "myorg",
		})

		_, err := planner.Plan(context.Background(), PlanRequest{})

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestForgePlanner_filterRepos(t *testing.T) {
	tests := []struct {
		name     string
		repos    []*provider.Repository
		config   ForgePlannerConfig
		expected int
	}{
		{
			name: "filters archived repos by default",
			repos: []*provider.Repository{
				{Name: "active", Archived: false},
				{Name: "archived", Archived: true},
			},
			config:   ForgePlannerConfig{},
			expected: 1,
		},
		{
			name: "includes archived when configured",
			repos: []*provider.Repository{
				{Name: "active", Archived: false},
				{Name: "archived", Archived: true},
			},
			config:   ForgePlannerConfig{IncludeArchived: true},
			expected: 2,
		},
		{
			name: "filters forks by default",
			repos: []*provider.Repository{
				{Name: "original", Fork: false},
				{Name: "forked", Fork: true},
			},
			config:   ForgePlannerConfig{},
			expected: 1,
		},
		{
			name: "includes forks when configured",
			repos: []*provider.Repository{
				{Name: "original", Fork: false},
				{Name: "forked", Fork: true},
			},
			config:   ForgePlannerConfig{IncludeForks: true},
			expected: 2,
		},
		{
			name: "filters private repos by default",
			repos: []*provider.Repository{
				{Name: "public", Private: false},
				{Name: "private", Private: true},
			},
			config:   ForgePlannerConfig{},
			expected: 1,
		},
		{
			name: "includes private when configured",
			repos: []*provider.Repository{
				{Name: "public", Private: false},
				{Name: "private", Private: true},
			},
			config:   ForgePlannerConfig{IncludePrivate: true},
			expected: 2,
		},
		{
			name: "applies all filters together",
			repos: []*provider.Repository{
				{Name: "public-active", Private: false, Archived: false, Fork: false},
				{Name: "private", Private: true},
				{Name: "archived", Archived: true},
				{Name: "fork", Fork: true},
			},
			config:   ForgePlannerConfig{},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &mockForgeProvider{name: "github"}
			planner := NewForgePlanner(p, tt.config)

			filtered := planner.filterRepos(tt.repos)

			if len(filtered) != tt.expected {
				t.Errorf("expected %d repos, got %d", tt.expected, len(filtered))
			}
		})
	}
}

func TestForgePlanner_toRepoSpec(t *testing.T) {
	t.Run("uses HTTPS URL by default", func(t *testing.T) {
		p := &mockForgeProvider{name: "github"}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath: "/tmp/repos",
		})
		repo := &provider.Repository{
			Name:     "myrepo",
			CloneURL: "https://github.com/org/myrepo.git",
			SSHURL:   "git@github.com:org/myrepo.git",
		}

		spec := planner.toRepoSpec(repo)

		if spec.CloneURL != repo.CloneURL {
			t.Errorf("expected HTTPS URL %s, got %s", repo.CloneURL, spec.CloneURL)
		}
	})

	t.Run("uses SSH URL when configured", func(t *testing.T) {
		p := &mockForgeProvider{name: "github"}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath: "/tmp/repos",
			UseSSH:     true,
		})
		repo := &provider.Repository{
			Name:     "myrepo",
			CloneURL: "https://github.com/org/myrepo.git",
			SSHURL:   "git@github.com:org/myrepo.git",
		}

		spec := planner.toRepoSpec(repo)

		if spec.CloneURL != repo.SSHURL {
			t.Errorf("expected SSH URL %s, got %s", repo.SSHURL, spec.CloneURL)
		}
	})

	t.Run("falls back to HTTPS when SSH URL is empty", func(t *testing.T) {
		p := &mockForgeProvider{name: "github"}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath: "/tmp/repos",
			UseSSH:     true,
		})
		repo := &provider.Repository{
			Name:     "myrepo",
			CloneURL: "https://github.com/org/myrepo.git",
			SSHURL:   "",
		}

		spec := planner.toRepoSpec(repo)

		if spec.CloneURL != repo.CloneURL {
			t.Errorf("expected HTTPS fallback %s, got %s", repo.CloneURL, spec.CloneURL)
		}
	})

	t.Run("sets correct target path", func(t *testing.T) {
		p := &mockForgeProvider{name: "gitlab"}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath: "/home/user/repos",
		})
		repo := &provider.Repository{Name: "myproject"}

		spec := planner.toRepoSpec(repo)

		expected := filepath.Join("/home/user/repos", "myproject")
		if spec.TargetPath != expected {
			t.Errorf("expected target path %s, got %s", expected, spec.TargetPath)
		}
	})

	t.Run("sets provider name", func(t *testing.T) {
		p := &mockForgeProvider{name: "gitea"}
		planner := NewForgePlanner(p, ForgePlannerConfig{TargetPath: "/tmp"})
		repo := &provider.Repository{Name: "myrepo"}

		spec := planner.toRepoSpec(repo)

		if spec.Provider != "gitea" {
			t.Errorf("expected provider gitea, got %s", spec.Provider)
		}
	})
}

func TestForgePlanner_planOrphanCleanup(t *testing.T) {
	t.Run("identifies orphan directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create expected repo
		expectedRepo := filepath.Join(tmpDir, "expected-repo", ".git")
		if err := os.MkdirAll(expectedRepo, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create orphan repo
		orphanRepo := filepath.Join(tmpDir, "orphan-repo", ".git")
		if err := os.MkdirAll(orphanRepo, 0o755); err != nil {
			t.Fatal(err)
		}

		repos := []*provider.Repository{{Name: "expected-repo"}}
		p := &mockForgeProvider{name: "github"}
		planner := NewForgePlanner(p, ForgePlannerConfig{TargetPath: tmpDir})

		actions := planner.planOrphanCleanup(repos, []string{tmpDir})

		if len(actions) != 1 {
			t.Fatalf("expected 1 orphan action, got %d", len(actions))
		}
		if actions[0].Type != ActionDelete {
			t.Errorf("expected delete action, got %s", actions[0].Type)
		}
		if actions[0].Repo.Name != "orphan-repo" {
			t.Errorf("expected orphan-repo, got %s", actions[0].Repo.Name)
		}
	})

	t.Run("skips non-git directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create regular directory (not a git repo)
		regularDir := filepath.Join(tmpDir, "not-a-repo")
		if err := os.MkdirAll(regularDir, 0o755); err != nil {
			t.Fatal(err)
		}

		p := &mockForgeProvider{name: "github"}
		planner := NewForgePlanner(p, ForgePlannerConfig{TargetPath: tmpDir})

		actions := planner.planOrphanCleanup([]*provider.Repository{}, []string{tmpDir})

		if len(actions) != 0 {
			t.Errorf("expected 0 actions for non-git dir, got %d", len(actions))
		}
	})

	t.Run("skips dot directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create hidden directory with git
		hiddenDir := filepath.Join(tmpDir, ".hidden-repo", ".git")
		if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
			t.Fatal(err)
		}

		p := &mockForgeProvider{name: "github"}
		planner := NewForgePlanner(p, ForgePlannerConfig{TargetPath: tmpDir})

		actions := planner.planOrphanCleanup([]*provider.Repository{}, []string{tmpDir})

		if len(actions) != 0 {
			t.Errorf("expected 0 actions for hidden dir, got %d", len(actions))
		}
	})
}

func TestForgePlanner_Describe(t *testing.T) {
	t.Run("describes organization plan", func(t *testing.T) {
		p := &mockForgeProvider{name: "github"}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath:   "/tmp/repos",
			Organization: "myorg",
		})

		desc := planner.Describe(PlanRequest{})

		if desc == "" {
			t.Error("expected non-empty description")
		}
		if !contains(desc, "github") {
			t.Error("description should mention provider")
		}
		if !contains(desc, "organization") {
			t.Error("description should mention organization")
		}
		if !contains(desc, "myorg") {
			t.Error("description should mention org name")
		}
	})

	t.Run("describes user plan", func(t *testing.T) {
		p := &mockForgeProvider{name: "gitlab"}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath:   "/tmp/repos",
			Organization: "username",
			IsUser:       true,
		})

		desc := planner.Describe(PlanRequest{})

		if !contains(desc, "user") {
			t.Error("description should mention user")
		}
	})

	t.Run("includes custom strategy", func(t *testing.T) {
		p := &mockForgeProvider{name: "github"}
		planner := NewForgePlanner(p, ForgePlannerConfig{
			TargetPath:   "/tmp/repos",
			Organization: "myorg",
		})

		desc := planner.Describe(PlanRequest{
			Options: PlanOptions{DefaultStrategy: StrategyPull},
		})

		if !contains(desc, "pull") {
			t.Error("description should mention custom strategy")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
