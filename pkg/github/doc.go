// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package github implements the provider interface for GitHub.
//
// This package provides GitHub-specific API integration for repository
// operations including listing, cloning, and organization management.
//
// # Features
//
//   - Repository listing (org and user)
//   - Token validation
//   - Rate limit management
//   - Pagination handling
//   - GitHub Enterprise Server (custom base URL)
//
// # Usage
//
//	// baseURL="" targets github.com; set it for a GitHub Enterprise Server.
//	provider, err := github.NewProvider(token, baseURL)
//	if err != nil {
//		return err
//	}
//	repos, err := provider.ListOrganizationRepos(ctx, "myorg")
package github
