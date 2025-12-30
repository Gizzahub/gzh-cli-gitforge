package gitlab

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xanzy/go-gitlab"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/ratelimit"
)

// Provider implements the provider.Provider interface for GitLab
type Provider struct {
	client      *gitlab.Client
	token       string
	baseURL     string
	rateLimiter *ratelimit.Limiter
	mu          sync.RWMutex
}

// NewProvider creates a new GitLab provider
func NewProvider(token, baseURL string) (*Provider, error) {
	p := &Provider{
		token:       token,
		baseURL:     baseURL,
		rateLimiter: ratelimit.NewLimiter(2000), // GitLab default
	}

	if err := p.initClient(); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Provider) initClient() error {
	var client *gitlab.Client
	var err error

	if p.baseURL != "" {
		client, err = gitlab.NewClient(p.token, gitlab.WithBaseURL(p.baseURL))
	} else {
		client, err = gitlab.NewClient(p.token)
	}

	if err != nil {
		return fmt.Errorf("failed to create GitLab client: %w", err)
	}

	p.client = client
	return nil
}

// SetToken sets the authentication token
func (p *Provider) SetToken(token string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.token = token
	return p.initClient()
}

// ValidateToken validates the current token
func (p *Provider) ValidateToken(ctx context.Context) (bool, error) {
	if p.token == "" {
		return false, nil
	}
	_, _, err := p.client.Users.CurrentUser(gitlab.WithContext(ctx))
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "gitlab"
}

// ListOrganizationRepos lists all repositories in a GitLab group
func (p *Provider) ListOrganizationRepos(ctx context.Context, group string) ([]*provider.Repository, error) {
	var allRepos []*provider.Repository

	opts := &gitlab.ListGroupProjectsOptions{
		ListOptions:      gitlab.ListOptions{PerPage: 100},
		IncludeSubGroups: gitlab.Ptr(true),
	}

	for {
		projects, resp, err := p.client.Groups.ListGroupProjects(group, opts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("failed to list projects for group %s: %w", group, err)
		}

		for _, project := range projects {
			allRepos = append(allRepos, convertGitLabProject(project))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// GetRepository gets a single repository from GitLab
func (p *Provider) GetRepository(ctx context.Context, owner, repo string) (*provider.Repository, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	project, _, err := p.client.Projects.GetProject(projectPath, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectPath, err)
	}

	return convertGitLabProject(project), nil
}

// ListOrganizations lists groups the authenticated user belongs to
func (p *Provider) ListOrganizations(ctx context.Context) ([]*provider.Organization, error) {
	var allOrgs []*provider.Organization

	opts := &gitlab.ListGroupsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}

	for {
		groups, resp, err := p.client.Groups.ListGroups(opts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("failed to list groups: %w", err)
		}

		for _, group := range groups {
			allOrgs = append(allOrgs, &provider.Organization{
				Name:        group.Path,
				Description: group.Description,
				URL:         group.WebURL,
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

	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}

	for {
		projects, resp, err := p.client.Projects.ListUserProjects(user, opts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("failed to list projects for user %s: %w", user, err)
		}

		for _, project := range projects {
			allRepos = append(allRepos, convertGitLabProject(project))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// GetRateLimit returns current rate limit status
// GitLab doesn't have a dedicated rate limit API, so we return estimated values
func (p *Provider) GetRateLimit(ctx context.Context) (*provider.RateLimit, error) {
	remaining, limit, resetTime := p.rateLimiter.Status()
	return &provider.RateLimit{
		Limit:     limit,
		Remaining: remaining,
		Reset:     resetTime,
		Used:      limit - remaining,
	}, nil
}

func convertGitLabProject(project *gitlab.Project) *provider.Repository {
	var createdAt, updatedAt, pushedAt time.Time
	if project.CreatedAt != nil {
		createdAt = *project.CreatedAt
	}
	if project.LastActivityAt != nil {
		updatedAt = *project.LastActivityAt
		pushedAt = *project.LastActivityAt
	}

	return &provider.Repository{
		Name:          project.Path,
		FullName:      project.PathWithNamespace,
		CloneURL:      project.HTTPURLToRepo,
		SSHURL:        project.SSHURLToRepo,
		HTMLURL:       project.WebURL,
		Description:   project.Description,
		DefaultBranch: project.DefaultBranch,
		Private:       project.Visibility != gitlab.PublicVisibility,
		Archived:      project.Archived,
		Fork:          project.ForkedFromProject != nil,
		Disabled:      false,
		Language:      "",
		Size:          0,
		Topics:        project.Topics,
		Visibility:    string(project.Visibility),
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		PushedAt:      pushedAt,
	}
}
