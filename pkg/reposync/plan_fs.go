package reposync

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FSPlanner inspects the filesystem to decide clone/update/delete actions.
type FSPlanner struct{}

var errEmptyRepos = errors.New("no repositories provided")

// Plan implements Planner by checking target paths and optional orphan cleanup.
func (FSPlanner) Plan(ctx context.Context, req PlanRequest) (Plan, error) {
	if len(req.Input.Repos) == 0 {
		return Plan{}, errEmptyRepos
	}

	defaultStrategy := req.Options.DefaultStrategy
	if defaultStrategy == "" {
		defaultStrategy = StrategyReset
	}

	actions := make([]Action, 0, len(req.Input.Repos))
	desired := make(map[string]RepoSpec, len(req.Input.Repos))

	for _, repo := range req.Input.Repos {
		select {
		case <-ctx.Done():
			return Plan{}, ctx.Err()
		default:
		}

		repo.Strategy = effectiveStrategy(repo.Strategy, defaultStrategy)
		repo.TargetPath = filepath.Clean(repo.TargetPath)

		desired[repo.TargetPath] = repo
		action := planForRepo(repo)
		actions = append(actions, action)
	}

	if req.Options.CleanupOrphans && len(req.Options.Roots) > 0 {
		actions = append(actions, findOrphans(ctx, req.Options.Roots, desired)...)
	}

	return Plan{Actions: actions}, nil
}

func planForRepo(repo RepoSpec) Action {
	info, err := os.Stat(repo.TargetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Action{
				Repo:      repo,
				Type:      ActionClone,
				Strategy:  repo.Strategy,
				Reason:    "target missing",
				PlannedBy: "fs",
			}
		}
		return Action{
			Repo:      repo,
			Type:      ActionClone,
			Strategy:  repo.Strategy,
			Reason:    fmt.Sprintf("stat error: %v", err),
			PlannedBy: "fs",
		}
	}

	if !info.IsDir() {
		return Action{
			Repo:      repo,
			Type:      ActionClone,
			Strategy:  repo.Strategy,
			Reason:    "target exists but is not a directory",
			PlannedBy: "fs",
		}
	}

	if isGitRepo(repo.TargetPath) {
		return Action{
			Repo:      repo,
			Type:      ActionUpdate,
			Strategy:  repo.Strategy,
			Reason:    "git repository detected",
			PlannedBy: "fs",
		}
	}

	// Non-git directory: plan a clone with clone strategy to replace.
	return Action{
		Repo:      repo,
		Type:      ActionClone,
		Strategy:  repo.Strategy,
		Reason:    "non-git directory, will replace with fresh clone",
		PlannedBy: "fs",
	}
}

func findOrphans(ctx context.Context, roots []string, desired map[string]RepoSpec) []Action {
	var actions []Action
	protected := buildProtectedDirs(desired)

	for _, root := range roots {
		select {
		case <-ctx.Done():
			return actions
		default:
		}

		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == ".git" || strings.HasPrefix(name, ".") {
				continue
			}
			path := filepath.Clean(filepath.Join(root, name))
			if _, ok := desired[path]; ok {
				continue
			}
			if _, ok := protected[path]; ok {
				continue
			}

			actions = append(actions, Action{
				Repo: RepoSpec{
					Name:       name,
					TargetPath: path,
				},
				Type:      ActionDelete,
				Reason:    "orphan directory",
				PlannedBy: "fs",
			})
		}
	}

	return actions
}

func buildProtectedDirs(desired map[string]RepoSpec) map[string]struct{} {
	protected := make(map[string]struct{}, len(desired))

	for targetPath := range desired {
		dir := filepath.Clean(filepath.Dir(targetPath))
		for dir != "." && dir != string(filepath.Separator) && dir != targetPath {
			protected[dir] = struct{}{}

			parent := filepath.Clean(filepath.Dir(dir))
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	return protected
}

func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		return true
	}
	return false
}

func effectiveStrategy(repo Strategy, fallback Strategy) Strategy {
	if repo != "" {
		return repo
	}
	return fallback
}
