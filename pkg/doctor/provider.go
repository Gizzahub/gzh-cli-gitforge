// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package doctor

import (
	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
)

// createProvider creates a forge provider for token validation and rate limit
// checks. It delegates to reposynccli.NewForgeProviderWithAuth — the single
// owner of provider-construction logic — so doctor's provider wiring can never
// drift from the sync paths (the original bug: doctor omitted the GitLab SSH
// port that from_forge set). SSH port is irrelevant to doctor's API-only token
// and rate-limit checks, so the default (0) is passed.
func createProvider(providerName, token, baseURL string) (provider.ProviderWithAuth, error) {
	return reposynccli.NewForgeProviderWithAuth(providerName, token, baseURL, 0)
}
