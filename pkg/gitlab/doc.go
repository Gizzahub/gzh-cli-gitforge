// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package gitlab implements the provider interface for GitLab.
//
// This package provides GitLab-specific API integration for repository
// operations including listing, cloning, and group management.
//
// # Features
//
//   - Repository listing (group and user)
//   - Subgroup support (flat and nested modes)
//   - Custom SSH port configuration
//   - Self-hosted instance support
//   - Token validation
//
// # Usage
//
//	provider, err := gitlab.NewProvider(token, "https://gitlab.example.com")
//	repos, err := provider.ListOrganizationRepos(ctx, "mygroup")
package gitlab
