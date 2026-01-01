// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package tag

import "time"

// Tag represents a Git tag.
type Tag struct {
	// Name is the tag name (e.g., "v1.0.0")
	Name string

	// Ref is the full tag reference (e.g., "refs/tags/v1.0.0")
	Ref string

	// SHA is the commit SHA the tag points to
	SHA string

	// Message is the tag message (for annotated tags)
	Message string

	// Tagger is the person who created the tag
	Tagger string

	// Date is when the tag was created
	Date time.Time

	// IsAnnotated indicates if this is an annotated tag
	IsAnnotated bool

	// IsRemote indicates if this tag exists on remote
	IsRemote bool
}

// CreateOptions configures tag creation.
type CreateOptions struct {
	// Name is the tag name (required)
	Name string

	// Message is the tag message (creates annotated tag if set)
	Message string

	// Sign creates a GPG-signed tag
	Sign bool

	// Force overwrites existing tag
	Force bool

	// Ref is the commit to tag (default: HEAD)
	Ref string
}

// ListOptions configures tag listing.
type ListOptions struct {
	// Pattern filters tags by name pattern
	Pattern string

	// Sort defines sort order: "version" (semver), "date", "name"
	Sort string

	// Limit limits the number of tags returned
	Limit int

	// IncludeRemote includes remote-only tags
	IncludeRemote bool
}

// PushOptions configures tag push.
type PushOptions struct {
	// All pushes all tags
	All bool

	// Name is the specific tag to push
	Name string

	// Remote is the remote to push to (default: origin)
	Remote string

	// Force forces push (overwrites remote)
	Force bool
}

// DeleteOptions configures tag deletion.
type DeleteOptions struct {
	// Name is the tag name to delete
	Name string

	// Remote also deletes from remote
	Remote bool
}
