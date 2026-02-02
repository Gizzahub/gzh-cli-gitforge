// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"time"
)

// Repository represents a repository from any Git platform.
type Repository struct {
	Name          string
	FullName      string
	CloneURL      string
	SSHURL        string
	HTMLURL       string
	Description   string
	DefaultBranch string
	Private       bool
	Archived      bool
	Fork          bool
	Disabled      bool
	Language      string
	Size          int
	Stars         int
	Topics        []string
	Visibility    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	PushedAt      time.Time
}

// Organization represents an organization or group from any Git platform.
type Organization struct {
	Name        string
	Description string
	URL         string
}

// SyncOptions configures repository synchronization.
type SyncOptions struct {
	TargetPath      string
	Parallel        int
	IncludeArchived bool
	IncludeForks    bool
	IncludePrivate  bool
	DryRun          bool
}

// SyncResult represents the result of syncing a single repository.
type SyncResult struct {
	Repository *Repository
	Action     SyncAction
	Error      error
}

// SyncAction represents what action was taken during sync.
type SyncAction string

const (
	ActionCloned  SyncAction = "cloned"
	ActionUpdated SyncAction = "updated"
	ActionSkipped SyncAction = "skipped"
	ActionFailed  SyncAction = "failed"
)

// RateLimit represents API rate limit information.
type RateLimit struct {
	Limit     int
	Remaining int
	Reset     time.Time
	Used      int
}

// ListOptions common pagination options.
type ListOptions struct {
	Page    int
	PerPage int
}

// Provider defines the interface for Git platform providers.
type Provider interface {
	// Name returns the provider name (github, gitlab, gitea)
	Name() string

	// ListOrganizationRepos lists all repositories in an organization/group
	ListOrganizationRepos(ctx context.Context, org string) ([]*Repository, error)

	// ListUserRepos lists all repositories for a user
	ListUserRepos(ctx context.Context, user string) ([]*Repository, error)

	// GetRepository gets a single repository
	GetRepository(ctx context.Context, owner, repo string) (*Repository, error)

	// ListOrganizations lists organizations the authenticated user belongs to
	ListOrganizations(ctx context.Context) ([]*Organization, error)

	// GetRateLimit returns current rate limit status
	GetRateLimit(ctx context.Context) (*RateLimit, error)
}

// ProviderWithAuth extends Provider with authentication capabilities.
type ProviderWithAuth interface {
	Provider

	// SetToken sets the authentication token
	SetToken(token string) error

	// ValidateToken validates the current token
	ValidateToken(ctx context.Context) (bool, error)
}

// Syncer handles repository synchronization operations.
type Syncer interface {
	// SyncOrganization syncs all repositories from an organization
	SyncOrganization(ctx context.Context, provider Provider, org string, opts SyncOptions) ([]SyncResult, error)
}
