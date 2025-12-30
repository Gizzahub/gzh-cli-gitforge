package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sync/errgroup"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// Syncer handles repository synchronization operations
type Syncer struct {
	// gitClient could be gzh-cli-gitforge client in the future
}

// NewSyncer creates a new Syncer
func NewSyncer() *Syncer {
	return &Syncer{}
}

// SyncOrganization syncs all repositories from an organization
func (s *Syncer) SyncOrganization(ctx context.Context, p provider.Provider, org string, opts provider.SyncOptions) ([]provider.SyncResult, error) {
	repos, err := p.ListOrganizationRepos(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to list repos: %w", err)
	}

	// Filter repos based on options
	var filteredRepos []*provider.Repository
	for _, repo := range repos {
		if repo.Archived && !opts.IncludeArchived {
			continue
		}
		if repo.Fork && !opts.IncludeForks {
			continue
		}
		if repo.Private && !opts.IncludePrivate {
			continue
		}
		filteredRepos = append(filteredRepos, repo)
	}

	if opts.DryRun {
		var results []provider.SyncResult
		for _, repo := range filteredRepos {
			repoPath := filepath.Join(opts.TargetPath, repo.Name)
			action := provider.ActionCloned
			if _, err := os.Stat(repoPath); err == nil {
				action = provider.ActionUpdated
			}
			results = append(results, provider.SyncResult{
				Repository: repo,
				Action:     action,
			})
		}
		return results, nil
	}

	// Ensure target directory exists
	if err := os.MkdirAll(opts.TargetPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Sync repositories in parallel
	results := make([]provider.SyncResult, len(filteredRepos))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.Parallel)

	for i, repo := range filteredRepos {
		i, repo := i, repo
		g.Go(func() error {
			result := s.syncRepo(ctx, repo, opts.TargetPath)
			results[i] = result
			return nil // Don't fail fast, collect all results
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}

	return results, nil
}

func (s *Syncer) syncRepo(ctx context.Context, repo *provider.Repository, targetPath string) provider.SyncResult {
	repoPath := filepath.Join(targetPath, repo.Name)

	// Check if repo already exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		// Repository exists, update it
		if err := s.updateRepo(ctx, repoPath); err != nil {
			return provider.SyncResult{
				Repository: repo,
				Action:     provider.ActionFailed,
				Error:      err,
			}
		}
		return provider.SyncResult{
			Repository: repo,
			Action:     provider.ActionUpdated,
		}
	}

	// Clone new repository
	if err := s.cloneRepo(ctx, repo.CloneURL, repoPath); err != nil {
		return provider.SyncResult{
			Repository: repo,
			Action:     provider.ActionFailed,
			Error:      err,
		}
	}

	return provider.SyncResult{
		Repository: repo,
		Action:     provider.ActionCloned,
	}
}

func (s *Syncer) cloneRepo(ctx context.Context, url, path string) error {
	// TODO: Use gzh-cli-gitforge library for cloning
	// For now, this is a placeholder
	return fmt.Errorf("clone not implemented: would clone %s to %s", url, path)
}

func (s *Syncer) updateRepo(ctx context.Context, path string) error {
	// TODO: Use gzh-cli-gitforge library for updating
	// For now, this is a placeholder
	return fmt.Errorf("update not implemented: would update %s", path)
}
