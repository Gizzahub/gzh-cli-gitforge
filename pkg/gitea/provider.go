package gitea

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/ratelimit"
)

// Provider implements the provider.Provider interface for Gitea
type Provider struct {
	baseURL     string
	token       string
	httpClient  *http.Client
	rateLimiter *ratelimit.Limiter
}

// NewProvider creates a new Gitea provider
func NewProvider(token, baseURL string) *Provider {
	return &Provider{
		baseURL:     baseURL,
		token:       token,
		httpClient:  &http.Client{},
		rateLimiter: ratelimit.NewLimiter(1000), // Gitea default estimate
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "gitea"
}

// SetToken sets the authentication token
func (p *Provider) SetToken(token string) error {
	p.token = token
	return nil
}

// ValidateToken validates the current token
func (p *Provider) ValidateToken(ctx context.Context) (bool, error) {
	// TODO: Implement token validation
	return p.token != "", nil
}

// ListOrganizationRepos lists all repositories in a Gitea organization
func (p *Provider) ListOrganizationRepos(ctx context.Context, org string) ([]*provider.Repository, error) {
	// TODO: Implement using Gitea API
	// Using code.gitea.io/sdk/gitea package
	return nil, fmt.Errorf("gitea provider not implemented yet")
}

// ListUserRepos lists all repositories for a user
func (p *Provider) ListUserRepos(ctx context.Context, user string) ([]*provider.Repository, error) {
	// TODO: Implement using Gitea API
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

// GetRateLimit returns current rate limit status
func (p *Provider) GetRateLimit(ctx context.Context) (*provider.RateLimit, error) {
	remaining, limit, resetTime := p.rateLimiter.Status()
	return &provider.RateLimit{
		Limit:     limit,
		Remaining: remaining,
		Reset:     resetTime,
		Used:      limit - remaining,
	}, nil
}
