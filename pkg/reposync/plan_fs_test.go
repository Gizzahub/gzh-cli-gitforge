package reposync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFSPlanner_Plan_EmptyRepos(t *testing.T) {
	_, err := (FSPlanner{}).Plan(context.Background(), PlanRequest{})
	if !errors.Is(err, errEmptyRepos) {
		t.Fatalf("expected errEmptyRepos, got %v", err)
	}
}

func TestFSPlanner_Plan_DefaultStrategyAndCleanPath(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "repo", "..", "repo")

	plan, err := (FSPlanner{}).Plan(context.Background(), PlanRequest{
		Input: PlanInput{
			Repos: []RepoSpec{
				{
					Name:       "repo",
					TargetPath: target,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan.Actions))
	}
	action := plan.Actions[0]
	if action.Strategy != StrategyReset {
		t.Fatalf("expected default strategy %q, got %q", StrategyReset, action.Strategy)
	}
	if action.Repo.TargetPath != filepath.Clean(target) {
		t.Fatalf("expected cleaned path %q, got %q", filepath.Clean(target), action.Repo.TargetPath)
	}
}

func TestFSPlanner_Plan_RepoStrategyOverridesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "repo")

	plan, err := (FSPlanner{}).Plan(context.Background(), PlanRequest{
		Input: PlanInput{
			Repos: []RepoSpec{
				{
					Name:       "repo",
					TargetPath: target,
					Strategy:   StrategyFetch,
				},
			},
		},
		Options: PlanOptions{
			DefaultStrategy: StrategyPull,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan.Actions))
	}
	if plan.Actions[0].Strategy != StrategyFetch {
		t.Fatalf("expected repo strategy %q, got %q", StrategyFetch, plan.Actions[0].Strategy)
	}
}

func TestFSPlanner_Plan_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (FSPlanner{}).Plan(ctx, PlanRequest{
		Input: PlanInput{
			Repos: []RepoSpec{
				{
					Name:       "repo",
					TargetPath: filepath.Join(t.TempDir(), "repo"),
				},
			},
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestPlanForRepo(t *testing.T) {
	t.Run("missing path plans clone", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "missing")
		action := planForRepo(RepoSpec{
			Name:       "repo",
			TargetPath: target,
			Strategy:   StrategyReset,
		})
		if action.Type != ActionClone {
			t.Fatalf("expected clone action, got %q", action.Type)
		}
	})

	t.Run("file path plans clone", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "file.txt")
		if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		action := planForRepo(RepoSpec{
			Name:       "repo",
			TargetPath: target,
			Strategy:   StrategyReset,
		})
		if action.Type != ActionClone {
			t.Fatalf("expected clone action, got %q", action.Type)
		}
		if action.Reason != "target exists but is not a directory" {
			t.Fatalf("unexpected reason: %q", action.Reason)
		}
	})

	t.Run("git directory plans update", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "repo")
		if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		action := planForRepo(RepoSpec{
			Name:       "repo",
			TargetPath: target,
			Strategy:   StrategyReset,
		})
		if action.Type != ActionUpdate {
			t.Fatalf("expected update action, got %q", action.Type)
		}
	})

	t.Run("non-git directory plans clone", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "repo")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		action := planForRepo(RepoSpec{
			Name:       "repo",
			TargetPath: target,
			Strategy:   StrategyReset,
		})
		if action.Type != ActionClone {
			t.Fatalf("expected clone action, got %q", action.Type)
		}
		if action.Reason != "non-git directory, will replace with fresh clone" {
			t.Fatalf("unexpected reason: %q", action.Reason)
		}
	})
}

func TestFindOrphans(t *testing.T) {
	root := t.TempDir()
	desiredPath := filepath.Join(root, "team", "repo1")
	if err := os.MkdirAll(desiredPath, 0o755); err != nil {
		t.Fatalf("mkdir desired repo: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "orphan"), 0o755); err != nil {
		t.Fatalf("mkdir orphan: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".hidden"), 0o755); err != nil {
		t.Fatalf("mkdir hidden: %v", err)
	}

	desired := map[string]RepoSpec{
		desiredPath: {
			Name:       "repo1",
			TargetPath: desiredPath,
		},
	}

	actions := findOrphans(context.Background(), []string{root}, desired)
	if len(actions) != 1 {
		t.Fatalf("expected 1 orphan action, got %d", len(actions))
	}
	if actions[0].Type != ActionDelete {
		t.Fatalf("expected delete action, got %q", actions[0].Type)
	}
	if actions[0].Repo.TargetPath != filepath.Join(root, "orphan") {
		t.Fatalf("unexpected orphan target: %q", actions[0].Repo.TargetPath)
	}
}
