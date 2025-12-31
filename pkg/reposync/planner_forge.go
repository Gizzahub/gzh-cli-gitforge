// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// ForgeProvider defines the minimal interface required from gitforge providers.
// This allows git-sync to work with any gitforge provider without importing
// the entire gitforge package directly in the core reposync package.
type ForgeProvider interface {
	Name() string
	ListOrganizationRepos(ctx context.Context, org string) ([]*provider.Repository, error)
	ListUserRepos(ctx context.Context, user string) ([]*provider.Repository, error)
}

// ForgePlannerConfig configures the ForgePlanner behavior.
type ForgePlannerConfig struct {
	// TargetPath is the base directory for cloning repositories
	TargetPath string

	// Organization is the org/group to sync from
	Organization string

	// IsUser indicates if Organization is actually a user (for ListUserRepos)
	IsUser bool

	// IncludeArchived includes archived repositories
	IncludeArchived bool

	// IncludeForks includes forked repositories
	IncludeForks bool

	// IncludePrivate includes private repositories
	IncludePrivate bool

	// UseSSH uses SSH URLs instead of HTTPS for cloning
	UseSSH bool
}

// ForgePlanner produces a Plan by querying a gitforge Provider.
type ForgePlanner struct {
	provider ForgeProvider
	config   ForgePlannerConfig
}

// NewForgePlanner creates a new ForgePlanner with the given provider and config.
func NewForgePlanner(provider ForgeProvider, config ForgePlannerConfig) *ForgePlanner {
	return &ForgePlanner{
		provider: provider,
		config:   config,
	}
}

// Plan implements the Planner interface.
// It queries the provider for repositories and creates a plan based on local state.
func (p *ForgePlanner) Plan(ctx context.Context, req PlanRequest) (Plan, error) {
	// Fetch repositories from the provider
	var repos []*provider.Repository
	var err error

	if p.config.IsUser {
		repos, err = p.provider.ListUserRepos(ctx, p.config.Organization)
	} else {
		repos, err = p.provider.ListOrganizationRepos(ctx, p.config.Organization)
	}

	if err != nil {
		return Plan{}, fmt.Errorf("failed to list repositories: %w", err)
	}

	// Filter repositories based on config
	filteredRepos := p.filterRepos(repos)

	if len(filteredRepos) == 0 {
		return Plan{}, nil
	}

	// Convert to RepoSpecs and determine actions
	defaultStrategy := req.Options.DefaultStrategy
	if defaultStrategy == "" {
		defaultStrategy = StrategyReset
	}

	actions := make([]Action, 0, len(filteredRepos))

	for _, repo := range filteredRepos {
		repoSpec := p.toRepoSpec(repo)

		// Check if repository exists locally
		targetPath := repoSpec.TargetPath
		gitDir := filepath.Join(targetPath, ".git")

		var actionType ActionType
		var reason string

		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			actionType = ActionClone
			reason = "repository not present locally"
		} else {
			actionType = ActionUpdate
			reason = "repository exists, will update"
		}

		strategy := repoSpec.Strategy
		if strategy == "" {
			strategy = defaultStrategy
		}

		actions = append(actions, Action{
			Repo:      repoSpec,
			Type:      actionType,
			Strategy:  strategy,
			Reason:    reason,
			PlannedBy: "forge:" + p.provider.Name(),
		})
	}

	// Handle orphan cleanup if enabled
	if req.Options.CleanupOrphans && len(req.Options.Roots) > 0 {
		orphanActions := p.planOrphanCleanup(filteredRepos, req.Options.Roots)
		actions = append(actions, orphanActions...)
	}

	return Plan{Actions: actions}, nil
}

// filterRepos filters repositories based on configuration.
func (p *ForgePlanner) filterRepos(repos []*provider.Repository) []*provider.Repository {
	filtered := make([]*provider.Repository, 0, len(repos))

	for _, repo := range repos {
		// Skip archived repos unless configured to include
		if repo.Archived && !p.config.IncludeArchived {
			continue
		}

		// Skip forks unless configured to include
		if repo.Fork && !p.config.IncludeForks {
			continue
		}

		// Skip private repos unless configured to include
		if repo.Private && !p.config.IncludePrivate {
			continue
		}

		filtered = append(filtered, repo)
	}

	return filtered
}

// toRepoSpec converts a provider.Repository to a RepoSpec.
func (p *ForgePlanner) toRepoSpec(repo *provider.Repository) RepoSpec {
	cloneURL := repo.CloneURL
	if p.config.UseSSH && repo.SSHURL != "" {
		cloneURL = repo.SSHURL
	}

	targetPath := filepath.Join(p.config.TargetPath, repo.Name)

	return RepoSpec{
		Name:       repo.Name,
		Provider:   p.provider.Name(),
		CloneURL:   cloneURL,
		TargetPath: targetPath,
	}
}

// planOrphanCleanup identifies directories that should be deleted.
func (p *ForgePlanner) planOrphanCleanup(repos []*provider.Repository, roots []string) []Action {
	// Build a set of expected repository names
	expectedNames := make(map[string]struct{}, len(repos))
	for _, repo := range repos {
		expectedNames[repo.Name] = struct{}{}
	}

	var deleteActions []Action

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue // Skip if we can't read the directory
		}

		for _, entry := range entries {
			// Skip non-directories and dot-directories
			if !entry.IsDir() || entry.Name()[0] == '.' {
				continue
			}

			// Check if this directory is an expected repository
			if _, expected := expectedNames[entry.Name()]; !expected {
				dirPath := filepath.Join(root, entry.Name())

				// Verify it's a git repository before marking for deletion
				gitDir := filepath.Join(dirPath, ".git")
				if _, err := os.Stat(gitDir); err == nil {
					deleteActions = append(deleteActions, Action{
						Repo: RepoSpec{
							Name:       entry.Name(),
							TargetPath: dirPath,
						},
						Type:      ActionDelete,
						Reason:    "orphan: not in organization repository list",
						PlannedBy: "forge:" + p.provider.Name(),
					})
				}
			}
		}
	}

	return deleteActions
}

// Describe returns a description of the planner configuration.
func (p *ForgePlanner) Describe(req PlanRequest) string {
	strategy := req.Options.DefaultStrategy
	if strategy == "" {
		strategy = StrategyReset
	}

	targetType := "organization"
	if p.config.IsUser {
		targetType = "user"
	}

	return fmt.Sprintf("forge plan for %s %s/%s (target=%s, strategy=%s)",
		p.provider.Name(), targetType, p.config.Organization, p.config.TargetPath, strategy)
}
