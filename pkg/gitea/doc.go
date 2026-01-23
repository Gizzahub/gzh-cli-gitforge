// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package gitea implements the provider interface for Gitea.
//
// This package provides Gitea-specific API integration for repository
// operations including listing, cloning, and organization management.
//
// # Features
//
//   - Repository listing (org and user)
//   - Token validation
//   - Self-hosted instance support
//   - Pagination handling
//
// # Usage
//
//	provider, err := gitea.NewProvider(token, "https://gitea.example.com")
//	repos, err := provider.ListOrganizationRepos(ctx, "myorg")
package gitea
