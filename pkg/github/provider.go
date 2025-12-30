package github

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/ratelimit"
)

// Provider implements the provider.Provider interface for GitHub
type Provider struct {
	client      *github.Client
	token       string
	rateLimiter *ratelimit.Limiter
	mu          sync.RWMutex
}

// NewProvider creates a new GitHub provider
func NewProvider(token string) *Provider {
	p := &Provider{
		token:       token,
		rateLimiter: ratelimit.NewLimiter(5000), // GitHub default
	}
	p.initClient(token)
	return p
}

func (p *Provider) initClient(token string) {
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(context.Background(), ts)
		p.client = github.NewClient(tc)
	} else {
		p.client = github.NewClient(nil)
	}
}

// SetToken sets the authentication token
func (p *Provider) SetToken(token string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.token = token
	p.initClient(token)
	return nil
}

// ValidateToken validates the current token
func (p *Provider) ValidateToken(ctx context.Context) (bool, error) {
	if p.token == "" {
		return false, nil
	}
	_, _, err := p.client.Users.Get(ctx, "")
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "github"
}

// ListOrganizationRepos lists all repositories in a GitHub organization
func (p *Provider) ListOrganizationRepos(ctx context.Context, org string) ([]*provider.Repository, error) {
	var allRepos []*provider.Repository

	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		repos, resp, err := p.client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list repos for org %s: %w", org, err)
		}

		for _, repo := range repos {
			allRepos = append(allRepos, convertGitHubRepo(repo))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// GetRepository gets a single repository from GitHub
func (p *Provider) GetRepository(ctx context.Context, owner, repo string) (*provider.Repository, error) {
	ghRepo, _, err := p.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo %s/%s: %w", owner, repo, err)
	}

	return convertGitHubRepo(ghRepo), nil
}

// ListOrganizations lists organizations the authenticated user belongs to
func (p *Provider) ListOrganizations(ctx context.Context) ([]*provider.Organization, error) {
	var allOrgs []*provider.Organization

	opts := &github.ListOptions{PerPage: 100}

	for {
		orgs, resp, err := p.client.Organizations.List(ctx, "", opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list organizations: %w", err)
		}

		for _, org := range orgs {
			allOrgs = append(allOrgs, &provider.Organization{
				Name:        org.GetLogin(),
				Description: org.GetDescription(),
				URL:         org.GetHTMLURL(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allOrgs, nil
}

// ListUserRepos lists all repositories for a user
func (p *Provider) ListUserRepos(ctx context.Context, user string) ([]*provider.Repository, error) {
	var allRepos []*provider.Repository

	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Type:        "all",
	}

	for {
		repos, resp, err := p.client.Repositories.List(ctx, user, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list repos for user %s: %w", user, err)
		}

		for _, repo := range repos {
			allRepos = append(allRepos, convertGitHubRepo(repo))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// GetRateLimit returns current rate limit status
func (p *Provider) GetRateLimit(ctx context.Context) (*provider.RateLimit, error) {
	limits, _, err := p.client.RateLimit.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rate limit: %w", err)
	}

	core := limits.Core
	return &provider.RateLimit{
		Limit:     core.Limit,
		Remaining: core.Remaining,
		Reset:     core.Reset.Time,
		Used:      core.Limit - core.Remaining,
	}, nil
}

func convertGitHubRepo(repo *github.Repository) *provider.Repository {
	return &provider.Repository{
		Name:          repo.GetName(),
		FullName:      repo.GetFullName(),
		CloneURL:      repo.GetCloneURL(),
		SSHURL:        repo.GetSSHURL(),
		HTMLURL:       repo.GetHTMLURL(),
		Description:   repo.GetDescription(),
		DefaultBranch: repo.GetDefaultBranch(),
		Private:       repo.GetPrivate(),
		Archived:      repo.GetArchived(),
		Fork:          repo.GetFork(),
		Disabled:      repo.GetDisabled(),
		Language:      repo.GetLanguage(),
		Size:          repo.GetSize(),
		Topics:        repo.Topics,
		Visibility:    repo.GetVisibility(),
		CreatedAt:     repo.GetCreatedAt().Time,
		UpdatedAt:     repo.GetUpdatedAt().Time,
		PushedAt:      repo.GetPushedAt().Time,
	}
}
