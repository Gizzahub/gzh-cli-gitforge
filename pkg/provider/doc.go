// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package provider defines the interface for Git forge providers.
//
// This package contains the common interface and types used by all
// Git forge implementations (GitHub, GitLab, Gitea).
//
// # Interface
//
// The Provider interface defines methods for:
//   - Repository listing (organization and user)
//   - Single repository retrieval
//   - Organization listing
//   - Token validation
//   - Rate limit checking
//
// # Types
//
//   - Repository: Common repository representation
//   - Organization: Common organization representation
//   - RateLimit: Rate limit status
//
// # Implementations
//
// See the github, gitlab, and gitea packages for concrete implementations.
package provider
