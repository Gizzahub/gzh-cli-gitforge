package gitlab

import (
	"context"
	"fmt"

	"github.com/xanzy/go-gitlab"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// Provider implements the provider.Provider interface for GitLab
type Provider struct {
	client *gitlab.Client
}

// NewProvider creates a new GitLab provider
func NewProvider(token, baseURL string) (*Provider, error) {
	var client *gitlab.Client
	var err error

	if baseURL != "" {
		client, err = gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
	} else {
		client, err = gitlab.NewClient(token)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	return &Provider{client: client}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "gitlab"
}

// ListOrganizationRepos lists all repositories in a GitLab group
func (p *Provider) ListOrganizationRepos(ctx context.Context, group string) ([]*provider.Repository, error) {
	var allRepos []*provider.Repository

	opts := &gitlab.ListGroupProjectsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
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

func convertGitLabProject(project *gitlab.Project) *provider.Repository {
	return &provider.Repository{
		Name:          project.Path,
		FullName:      project.PathWithNamespace,
		CloneURL:      project.HTTPURLToRepo,
		SSHURL:        project.SSHURLToRepo,
		Description:   project.Description,
		DefaultBranch: project.DefaultBranch,
		Private:       project.Visibility != gitlab.PublicVisibility,
		Archived:      project.Archived,
		Fork:          project.ForkedFromProject != nil,
	}
}
