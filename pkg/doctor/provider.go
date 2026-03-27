// Copyright (c) 2026 Archmagece
// SPDX-License-Identifier: MIT

package doctor

import (
	"fmt"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitea"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/github"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitlab"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

// createProvider creates a forge provider for token validation and rate limit checks.
func createProvider(providerName, token, baseURL string) (provider.ProviderWithAuth, error) {
	switch providerName {
	case "github":
		return github.NewProvider(token), nil
	case "gitlab":
		p, err := gitlab.NewProviderWithOptions(gitlab.ProviderOptions{
			Token:   token,
			BaseURL: baseURL,
		})
		if err != nil {
			return nil, err
		}
		return p, nil
	case "gitea":
		p, err := gitea.NewProviderWithOptions(gitea.ProviderOptions{
			Token:   token,
			BaseURL: baseURL,
		})
		if err != nil {
			return nil, err
		}
		return p, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}
