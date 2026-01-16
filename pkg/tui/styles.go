// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package tui

import "github.com/charmbracelet/lipgloss"

// Pre-defined styles for consistent UI appearance.
var (
	// HeaderStyle is used for the main header bar.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	// CursorStyle highlights the currently selected line.
	CursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("6")).
			Bold(true)

	// UnhealthyStyle is used for repositories with health issues.
	UnhealthyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))

	// DirtyStyle is used for repositories with uncommitted changes.
	DirtyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	// SubtleStyle is used for less important information (footer, scroll indicators).
	SubtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)
