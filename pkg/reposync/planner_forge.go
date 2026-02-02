// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// CloneProto is the clone protocol: ssh, https
	CloneProto string

	// SSHPort is the custom SSH port (0 = default 22)
	SSHPort int

	// IncludeSubgroups includes subgroups (GitLab only)
	IncludeSubgroups bool

	// SubgroupMode is flat (dash-separated) or nested (directories)
	SubgroupMode string

	// FlatSeparator is the separator for flat mode (default: "-")
	// Examples: "-", "_", ".", "" (empty = no separator)
	// Invalid characters: / \ : * ? " < > |
	FlatSeparator string

	// Auth contains authentication settings for clone operations
	Auth AuthConfig

	// Metadata filters
	FilterLanguages     []string  // Filter by language (lowercase)
	FilterMinStars      int       // Minimum star count
	FilterMaxStars      int       // Maximum star count (0 = unlimited)
	FilterLastPushAfter time.Time // Only include repos pushed after this time
}

// ForgePlanner produces a Plan by querying a gitforge Provider.
type ForgePlanner struct {
	provider ForgeProvider
	config   ForgePlannerConfig
}

// NewForgePlanner creates a new ForgePlanner with the given provider and config.
func NewForgePlanner(provider ForgeProvider, config ForgePlannerConfig) *ForgePlanner {
	// Validate flat separator for filesystem safety
	if config.SubgroupMode == "flat" && config.FlatSeparator != "" {
		if !isValidFlatSeparator(config.FlatSeparator) {
			// Fall back to default separator for safety
			config.FlatSeparator = "-"
		}
	}

	return &ForgePlanner{
		provider: provider,
		config:   config,
	}
}

// isValidFlatSeparator checks if the separator is safe for filesystem use.
func isValidFlatSeparator(sep string) bool {
	// Invalid characters that could cause filesystem issues
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, invalid := range invalidChars {
		if strings.Contains(sep, invalid) {
			return false
		}
	}
	return true
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

		// Language filter
		if len(p.config.FilterLanguages) > 0 {
			repoLang := strings.ToLower(repo.Language)
			if repoLang == "" || !containsStringSlice(p.config.FilterLanguages, repoLang) {
				continue
			}
		}

		// Stars filter (minimum)
		if p.config.FilterMinStars > 0 && repo.Stars < p.config.FilterMinStars {
			continue
		}

		// Stars filter (maximum, 0 = unlimited)
		if p.config.FilterMaxStars > 0 && repo.Stars > p.config.FilterMaxStars {
			continue
		}

		// Activity filter (last push)
		if !p.config.FilterLastPushAfter.IsZero() && repo.PushedAt.Before(p.config.FilterLastPushAfter) {
			continue
		}

		filtered = append(filtered, repo)
	}

	return filtered
}

// containsStringSlice checks if slice contains the target string.
func containsStringSlice(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// toRepoSpec converts a provider.Repository to a RepoSpec.
func (p *ForgePlanner) toRepoSpec(repo *provider.Repository) RepoSpec {
	// Select clone URL based on protocol
	cloneURL := repo.CloneURL // default: HTTPS
	if p.config.CloneProto == "ssh" && repo.SSHURL != "" {
		cloneURL = repo.SSHURL
	}

	// Build target path based on subgroup mode
	targetPath := p.buildTargetPath(repo)

	// Copy auth config with provider name for URL injection
	auth := p.config.Auth
	auth.Provider = p.provider.Name()
	auth.SSHPort = p.config.SSHPort

	return RepoSpec{
		Name:       repo.Name,
		Provider:   p.provider.Name(),
		CloneURL:   cloneURL,
		TargetPath: targetPath,
		Auth:       auth,
	}
}

// buildTargetPath constructs the target path based on subgroup mode.
func (p *ForgePlanner) buildTargetPath(repo *provider.Repository) string {
	basePath := p.config.TargetPath

	// Use FullName for subgroup handling (e.g., "parent-group/subgroup/repo")
	projectPath := repo.FullName

	// If not including subgroups or mode is empty, use simple name
	if !p.config.IncludeSubgroups || p.config.SubgroupMode == "" {
		return filepath.Join(basePath, repo.Name)
	}

	// Strip organization prefix from projectPath
	// Example: "notes/repo" -> "repo", "notes/subgroup/repo" -> "subgroup/repo"
	subPath := p.stripOrgPrefix(projectPath)

	switch p.config.SubgroupMode {
	case "flat":
		// Replace / with configured separator for flat structure
		// Default separator: "-"
		// Example: "subgroup/repo" -> "subgroup-repo" (or "subgroup_repo" with "_")
		separator := p.config.FlatSeparator
		if separator == "" {
			separator = "-" // default for backward compatibility
		}
		flat := strings.ReplaceAll(subPath, "/", separator)
		return filepath.Join(basePath, flat)

	case "nested":
		// Keep directory structure
		// Example: "subgroup/repo" -> "subgroup/repo"
		return filepath.Join(basePath, subPath)

	default:
		// Default to repo name only
		return filepath.Join(basePath, repo.Name)
	}
}

// stripOrgPrefix removes the organization prefix from FullName.
// Example: "org/repo" -> "repo", "org/sub/repo" -> "sub/repo"
func (p *ForgePlanner) stripOrgPrefix(fullName string) string {
	// Split by / and remove first component (organization)
	parts := strings.Split(fullName, "/")
	if len(parts) <= 1 {
		return fullName // No organization prefix
	}

	// If organization matches, strip it
	if parts[0] == p.config.Organization {
		return strings.Join(parts[1:], "/")
	}

	// Fallback: strip first component anyway (might be parent group)
	return strings.Join(parts[1:], "/")
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
