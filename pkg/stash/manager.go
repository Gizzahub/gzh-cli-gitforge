// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package stash

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// Manager defines stash operations.
type Manager interface {
	// Save creates a new stash entry.
	Save(ctx context.Context, repo *repository.Repository, opts SaveOptions) error

	// List returns all stash entries.
	List(ctx context.Context, repo *repository.Repository, opts ListOptions) ([]*Stash, error)

	// Pop applies and removes a stash entry.
	Pop(ctx context.Context, repo *repository.Repository, opts PopOptions) error

	// Apply applies a stash entry without removing it.
	Apply(ctx context.Context, repo *repository.Repository, opts PopOptions) error

	// Drop removes a stash entry.
	Drop(ctx context.Context, repo *repository.Repository, opts DropOptions) error

	// Clear removes all stash entries.
	Clear(ctx context.Context, repo *repository.Repository) error

	// Count returns the number of stash entries.
	Count(ctx context.Context, repo *repository.Repository) (int, error)
}

// manager implements Manager.
type manager struct {
	executor *gitcmd.Executor
}

// NewManager creates a new stash manager.
func NewManager() Manager {
	return &manager{
		executor: gitcmd.NewExecutor(),
	}
}

// Save creates a new stash entry.
func (m *manager) Save(ctx context.Context, repo *repository.Repository, opts SaveOptions) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	args := []string{"stash", "push"}

	if opts.Message != "" {
		args = append(args, "-m", opts.Message)
	}

	if opts.IncludeUntracked {
		args = append(args, "--include-untracked")
	}

	if opts.KeepIndex {
		args = append(args, "--keep-index")
	}

	if opts.All {
		args = append(args, "--all")
	}

	result, err := m.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return fmt.Errorf("failed to save stash: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git stash failed: %s", result.Stderr)
	}

	return nil
}

// List returns all stash entries.
func (m *manager) List(ctx context.Context, repo *repository.Repository, opts ListOptions) ([]*Stash, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}

	// Use custom format to parse stash entries
	// Format: index|ref|sha|message|date
	args := []string{"stash", "list", "--format=%gd|%H|%s|%ct"}

	result, err := m.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list stashes: %w", err)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git stash list failed: %s", result.Stderr)
	}

	var stashes []*Stash
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")

	for i, line := range lines {
		if line == "" {
			continue
		}

		if opts.Limit > 0 && i >= opts.Limit {
			break
		}

		stash, err := parseStashLine(line, i)
		if err != nil {
			continue // Skip malformed lines
		}
		stashes = append(stashes, stash)
	}

	return stashes, nil
}

// Pop applies and removes a stash entry.
func (m *manager) Pop(ctx context.Context, repo *repository.Repository, opts PopOptions) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	args := []string{"stash", "pop"}
	if opts.Index > 0 {
		args = append(args, fmt.Sprintf("stash@{%d}", opts.Index))
	}

	result, err := m.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return fmt.Errorf("failed to pop stash: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git stash pop failed: %s", result.Stderr)
	}

	return nil
}

// Apply applies a stash entry without removing it.
func (m *manager) Apply(ctx context.Context, repo *repository.Repository, opts PopOptions) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	args := []string{"stash", "apply"}
	if opts.Index > 0 {
		args = append(args, fmt.Sprintf("stash@{%d}", opts.Index))
	}

	result, err := m.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return fmt.Errorf("failed to apply stash: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git stash apply failed: %s", result.Stderr)
	}

	return nil
}

// Drop removes a stash entry.
func (m *manager) Drop(ctx context.Context, repo *repository.Repository, opts DropOptions) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	args := []string{"stash", "drop", fmt.Sprintf("stash@{%d}", opts.Index)}

	result, err := m.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return fmt.Errorf("failed to drop stash: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git stash drop failed: %s", result.Stderr)
	}

	return nil
}

// Clear removes all stash entries.
func (m *manager) Clear(ctx context.Context, repo *repository.Repository) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	result, err := m.executor.Run(ctx, repo.Path, "stash", "clear")
	if err != nil {
		return fmt.Errorf("failed to clear stashes: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("git stash clear failed: %s", result.Stderr)
	}

	return nil
}

// Count returns the number of stash entries.
func (m *manager) Count(ctx context.Context, repo *repository.Repository) (int, error) {
	stashes, err := m.List(ctx, repo, ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(stashes), nil
}

// parseStashLine parses a stash list line.
func parseStashLine(line string, index int) (*Stash, error) {
	// Format: stash@{N}|SHA|message|timestamp
	parts := strings.SplitN(line, "|", 4)
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid stash line format")
	}

	ref := parts[0]
	sha := parts[1]
	message := parts[2]
	timestampStr := parts[3]

	// Parse timestamp
	timestamp, _ := strconv.ParseInt(timestampStr, 10, 64)
	date := time.Unix(timestamp, 0)

	// Extract branch from message if present (format: "On <branch>: <message>")
	branch := ""
	if strings.HasPrefix(message, "On ") {
		if colonIdx := strings.Index(message, ":"); colonIdx > 3 {
			branch = message[3:colonIdx]
			message = strings.TrimSpace(message[colonIdx+1:])
		}
	}

	return &Stash{
		Index:   index,
		Ref:     ref,
		Message: message,
		Branch:  branch,
		SHA:     sha,
		Date:    date,
	}, nil
}
