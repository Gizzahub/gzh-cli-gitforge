package gitea

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// Provider implements the provider.Provider interface for Gitea
type Provider struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewProvider creates a new Gitea provider
func NewProvider(token, baseURL string) *Provider {
	return &Provider{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "gitea"
}

// ListOrganizationRepos lists all repositories in a Gitea organization
func (p *Provider) ListOrganizationRepos(ctx context.Context, org string) ([]*provider.Repository, error) {
	// TODO: Implement using Gitea API
	// Using code.gitea.io/sdk/gitea package
	return nil, fmt.Errorf("gitea provider not implemented yet")
}

// GetRepository gets a single repository from Gitea
func (p *Provider) GetRepository(ctx context.Context, owner, repo string) (*provider.Repository, error) {
	// TODO: Implement using Gitea API
	return nil, fmt.Errorf("gitea provider not implemented yet")
}

// ListOrganizations lists organizations the authenticated user belongs to
func (p *Provider) ListOrganizations(ctx context.Context) ([]*provider.Organization, error) {
	// TODO: Implement using Gitea API
	return nil, fmt.Errorf("gitea provider not implemented yet")
}
