// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposync

import "context"

// StateStore persists run state for resume/audit.
type StateStore interface {
	Save(ctx context.Context, state RunState) error
	Load(ctx context.Context) (RunState, error)
}

// RunState captures progress for resuming operations.
type RunState struct {
	Items []RunStateItem
}

// RunStateItem tracks per-repo status.
type RunStateItem struct {
	Repo    RepoSpec
	Status  RunStatus
	Message string
}

// RunStatus represents the last known state of a repository.
type RunStatus string

// RunStatus values track a repository's execution state across a sync run.
const (
	RunStatusPending RunStatus = "pending"
	RunStatusRunning RunStatus = "running"
	RunStatusDone    RunStatus = "done"
	RunStatusFailed  RunStatus = "failed"
)

// InMemoryStateStore is a lightweight StateStore useful for dry-runs and tests.
type InMemoryStateStore struct {
	state RunState
}

// NewInMemoryStateStore creates a new in-memory state store.
func NewInMemoryStateStore() *InMemoryStateStore {
	return &InMemoryStateStore{
		state: RunState{},
	}
}

// Save stores state in memory.
func (s *InMemoryStateStore) Save(_ context.Context, state RunState) error {
	s.state = state
	return nil
}

// Load returns state in memory.
func (s *InMemoryStateStore) Load(_ context.Context) (RunState, error) {
	return s.state, nil
}
