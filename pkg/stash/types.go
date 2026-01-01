// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package stash

import "time"

// Stash represents a single stash entry.
type Stash struct {
	// Index is the stash index (0 = most recent)
	Index int

	// Ref is the stash reference (e.g., "stash@{0}")
	Ref string

	// Message is the stash message
	Message string

	// Branch is the branch where the stash was created
	Branch string

	// SHA is the commit SHA of the stash
	SHA string

	// Date is when the stash was created
	Date time.Time
}

// SaveOptions configures stash save operation.
type SaveOptions struct {
	// Message is the stash message (optional)
	Message string

	// IncludeUntracked includes untracked files
	IncludeUntracked bool

	// KeepIndex keeps staged changes in the index
	KeepIndex bool

	// All includes ignored files
	All bool
}

// PopOptions configures stash pop/apply operation.
type PopOptions struct {
	// Index is the stash index to pop (default: 0)
	Index int

	// Apply applies without removing from stash list
	Apply bool
}

// DropOptions configures stash drop operation.
type DropOptions struct {
	// Index is the stash index to drop
	Index int
}

// ListOptions configures stash list operation.
type ListOptions struct {
	// Limit limits the number of stashes to return
	Limit int
}
