package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// Provider implements the provider.Provider interface for GitHub
type Provider struct {
	client *github.Client
}

// NewProvider creates a new GitHub provider
func NewProvider(token string) *Provider {
	var client *github.Client

	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(context.Background(), ts)
		client = github.NewClient(tc)
	} else {
		client = github.NewClient(nil)
	}

	return &Provider{client: client}
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

func convertGitHubRepo(repo *github.Repository) *provider.Repository {
	return &provider.Repository{
		Name:          repo.GetName(),
		FullName:      repo.GetFullName(),
		CloneURL:      repo.GetCloneURL(),
		SSHURL:        repo.GetSSHURL(),
		Description:   repo.GetDescription(),
		DefaultBranch: repo.GetDefaultBranch(),
		Private:       repo.GetPrivate(),
		Archived:      repo.GetArchived(),
		Fork:          repo.GetFork(),
	}
}
