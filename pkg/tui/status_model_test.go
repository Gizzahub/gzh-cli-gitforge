// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func TestNewStatusModel(t *testing.T) {
	repos := []reposync.RepoHealth{
		{
			Repo: reposync.RepoSpec{
				Name:       "test-repo",
				TargetPath: "/path/to/repo",
			},
			HealthStatus: reposync.HealthHealthy,
		},
	}

	model := NewStatusModel(repos)

	if len(model.repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(model.repos))
	}

	if model.cursor != 0 {
		t.Error("expected cursor to be 0")
	}

	if model.ready {
		t.Error("expected ready to be false initially")
	}
}

func TestStatusModelUpdate(t *testing.T) {
	repos := []reposync.RepoHealth{
		{
			Repo: reposync.RepoSpec{
				Name:       "repo1",
				TargetPath: "/path/to/repo1",
			},
			HealthStatus: reposync.HealthHealthy,
		},
		{
			Repo: reposync.RepoSpec{
				Name:       "repo2",
				TargetPath: "/path/to/repo2",
			},
			HealthStatus: reposync.HealthWarning,
		},
	}

	model := NewStatusModel(repos)

	// Test window size message
	msg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updated, _ := model.Update(msg)
	m := updated.(StatusModel)

	if m.width != 100 || m.height != 30 {
		t.Errorf("expected width=100 height=30, got width=%d height=%d", m.width, m.height)
	}

	if !m.ready {
		t.Error("expected ready to be true after window size message")
	}

	// Test navigation down
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	updated, _ = m.Update(keyMsg)
	m = updated.(StatusModel)

	if m.cursor != 1 {
		t.Errorf("expected cursor to be 1, got %d", m.cursor)
	}

	// Test navigation up
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	updated, _ = m.Update(keyMsg)
	m = updated.(StatusModel)

	if m.cursor != 0 {
		t.Errorf("expected cursor to be 0, got %d", m.cursor)
	}

	// Test selection toggle
	keyMsg = tea.KeyMsg{Type: tea.KeySpace}
	updated, _ = m.Update(keyMsg)
	m = updated.(StatusModel)

	if !m.selected["/path/to/repo1"] {
		t.Error("expected repo1 to be selected")
	}

	// Test select all
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	updated, _ = m.Update(keyMsg)
	m = updated.(StatusModel)

	if len(m.selected) != 2 {
		t.Errorf("expected 2 selected repos, got %d", len(m.selected))
	}

	// Test deselect all
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	updated, _ = m.Update(keyMsg)
	m = updated.(StatusModel)

	if len(m.selected) != 0 {
		t.Errorf("expected 0 selected repos, got %d", len(m.selected))
	}
}

func TestGetStatusDisplay(t *testing.T) {
	tests := []struct {
		name         string
		health       reposync.RepoHealth
		expectedIcon string
		expectedText string
	}{
		{
			name: "healthy and clean",
			health: reposync.RepoHealth{
				HealthStatus:   reposync.HealthHealthy,
				WorkTreeStatus: reposync.WorkTreeClean,
				DivergenceType: reposync.DivergenceNone,
			},
			expectedIcon: "✓",
			expectedText: "Clean",
		},
		{
			name: "healthy but dirty",
			health: reposync.RepoHealth{
				HealthStatus:   reposync.HealthHealthy,
				WorkTreeStatus: reposync.WorkTreeDirty,
				ModifiedFiles:  5,
				UntrackedFiles: 3,
			},
			expectedIcon: "✗",
			expectedText: "Dirty (8 files)",
		},
		{
			name: "warning",
			health: reposync.RepoHealth{
				HealthStatus: reposync.HealthWarning,
			},
			expectedIcon: "⚠",
			expectedText: "Warning",
		},
		{
			name: "error",
			health: reposync.RepoHealth{
				HealthStatus: reposync.HealthError,
			},
			expectedIcon: "✗",
			expectedText: "Error",
		},
		{
			name: "unreachable",
			health: reposync.RepoHealth{
				HealthStatus: reposync.HealthUnreachable,
			},
			expectedIcon: "⊘",
			expectedText: "Unreachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon, text := getStatusDisplay(tt.health)
			if icon != tt.expectedIcon {
				t.Errorf("expected icon %q, got %q", tt.expectedIcon, icon)
			}
			if text != tt.expectedText {
				t.Errorf("expected text %q, got %q", tt.expectedText, text)
			}
		})
	}
}
