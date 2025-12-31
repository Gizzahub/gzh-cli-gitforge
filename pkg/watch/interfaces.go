// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package watch

import (
	"context"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// Watcher monitors Git repositories for changes.
type Watcher interface {
	// Start begins monitoring the specified repository paths.
	// It returns immediately and sends events to the event channel.
	Start(ctx context.Context, paths []string) error

	// Events returns the channel for receiving watch events.
	Events() <-chan Event

	// Errors returns the channel for receiving errors.
	Errors() <-chan error

	// Stop stops the watcher and closes all channels.
	Stop() error
}

// Event represents a change detected in a repository.
type Event struct {
	// Path is the repository path where the change occurred.
	Path string

	// Type is the type of change detected.
	Type EventType

	// Timestamp is when the event was detected.
	Timestamp time.Time

	// Status is the repository status at the time of detection.
	Status *repository.Status

	// Files are the specific files that changed (if available).
	Files []string
}

// EventType represents the type of change detected.
type EventType string

const (
	// EventTypeModified indicates files were modified (unstaged).
	EventTypeModified EventType = "modified"

	// EventTypeStaged indicates files were staged.
	EventTypeStaged EventType = "staged"

	// EventTypeUntracked indicates new untracked files appeared.
	EventTypeUntracked EventType = "untracked"

	// EventTypeDeleted indicates files were deleted.
	EventTypeDeleted EventType = "deleted"

	// EventTypeCommit indicates a new commit was created.
	EventTypeCommit EventType = "commit"

	// EventTypeBranch indicates branch changed.
	EventTypeBranch EventType = "branch"

	// EventTypeClean indicates the repository became clean.
	EventTypeClean EventType = "clean"
)

// String returns the string representation of the event type.
func (e EventType) String() string {
	return string(e)
}

// WatchOptions configures the watcher behavior.
type WatchOptions struct {
	// Interval is the polling interval for checking changes.
	// If not specified, defaults to 2 seconds.
	Interval time.Duration

	// IncludeClean indicates whether to send events when repository becomes clean.
	IncludeClean bool

	// DebounceDuration is the minimum time between events for the same path.
	// This prevents duplicate events in rapid succession.
	DebounceDuration time.Duration

	// Logger is the logger to use for watch operations.
	Logger Logger
}

// Logger defines the logging interface for the watch package.
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}
