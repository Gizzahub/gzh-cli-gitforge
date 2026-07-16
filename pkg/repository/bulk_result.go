// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package repository

// BulkRepoResult is the shared surface for bulk operation per-repo results.
// Command renderers consume this interface so display/JSON logic stays in one place.
type BulkRepoResult interface {
	GetStatus() string
	GetPath() string
	GetMessage() string
	GetError() error
}

func bulkResultPath(relative, absolute string) string {
	if relative != "" {
		return relative
	}
	return absolute
}

// GetPath returns RelativePath when set, otherwise Path.
func (r RepositoryFetchResult) GetPath() string { return bulkResultPath(r.RelativePath, r.Path) }

// GetMessage returns the human-readable status message.
func (r RepositoryFetchResult) GetMessage() string { return r.Message }

// GetError returns the per-repo error, if any.
func (r RepositoryFetchResult) GetError() error { return r.Error }

// GetPath returns RelativePath when set, otherwise Path.
func (r RepositoryPullResult) GetPath() string { return bulkResultPath(r.RelativePath, r.Path) }

// GetMessage returns the human-readable status message.
func (r RepositoryPullResult) GetMessage() string { return r.Message }

// GetError returns the per-repo error, if any.
func (r RepositoryPullResult) GetError() error { return r.Error }

// GetPath returns RelativePath when set, otherwise Path.
func (r RepositoryPushResult) GetPath() string { return bulkResultPath(r.RelativePath, r.Path) }

// GetMessage returns the human-readable status message.
func (r RepositoryPushResult) GetMessage() string { return r.Message }

// GetError returns the per-repo error, if any.
func (r RepositoryPushResult) GetError() error { return r.Error }

// GetPath returns RelativePath when set, otherwise Path.
func (r RepositoryUpdateResult) GetPath() string { return bulkResultPath(r.RelativePath, r.Path) }

// GetMessage returns the human-readable status message.
func (r RepositoryUpdateResult) GetMessage() string { return r.Message }

// GetError returns the per-repo error, if any.
func (r RepositoryUpdateResult) GetError() error { return r.Error }

// GetPath returns RelativePath when set, otherwise Path.
func (r RepositoryStatusResult) GetPath() string { return bulkResultPath(r.RelativePath, r.Path) }

// GetMessage returns the human-readable status message.
func (r RepositoryStatusResult) GetMessage() string { return r.Message }

// GetError returns the per-repo error, if any.
func (r RepositoryStatusResult) GetError() error { return r.Error }

// GetPath returns RelativePath when set, otherwise Path.
func (r RepositorySwitchResult) GetPath() string { return bulkResultPath(r.RelativePath, r.Path) }

// GetMessage returns the human-readable status message.
func (r RepositorySwitchResult) GetMessage() string { return r.Message }

// GetError returns the per-repo error, if any.
func (r RepositorySwitchResult) GetError() error { return r.Error }
