// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// FilterType represents the type of filter applied.
type FilterType string

const (
	FilterNone  FilterType = ""
	FilterDirty FilterType = "dirty"   // Show only dirty repos
	FilterClean FilterType = "clean"   // Show only clean repos
	FilterAhead FilterType = "ahead"   // Show only repos ahead of remote
	FilterAll   FilterType = "all"     // Show all (reset filter)
)

// StatusModel represents the TUI state for repository status display.
type StatusModel struct {
	repos       []reposync.RepoHealth
	allRepos    []reposync.RepoHealth // unfiltered list
	selected    map[string]bool       // repo path -> selected
	cursor      int                   // current cursor position
	filter      FilterType            // current filter
	width       int                   // terminal width
	height      int                   // terminal height
	ready       bool                  // terminal size received
	action      string                // pending action: "sync", "pull", "fetch"
	showDetail  bool                  // show detail view
	err         error                 // last error
}

// NewStatusModel creates a new status TUI model.
func NewStatusModel(repos []reposync.RepoHealth) StatusModel {
	return StatusModel{
		repos:    repos,
		allRepos: repos, // Keep unfiltered copy
		selected: make(map[string]bool),
		cursor:   0,
		ready:    false,
		filter:   FilterNone,
	}
}

// Init initializes the model (required by Bubble Tea).
func (m StatusModel) Init() tea.Cmd {
	return nil
}

// Update handles all messages and updates the model state.
func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.repos)-1 {
				m.cursor++
			}

		case " ": // Space - toggle selection
			if m.cursor < len(m.repos) {
				path := m.repos[m.cursor].Repo.TargetPath
				m.selected[path] = !m.selected[path]
			}

		case "a": // Select all
			for _, repo := range m.repos {
				m.selected[repo.Repo.TargetPath] = true
			}

		case "n": // Deselect all
			m.selected = make(map[string]bool)

		case "home", "g":
			m.cursor = 0

		case "end", "G":
			m.cursor = len(m.repos) - 1

		case "s": // Sync selected repos
			return m.executeAction("sync")

		case "p": // Pull selected repos
			return m.executeAction("pull")

		case "f": // Fetch selected repos
			return m.executeAction("fetch")

		case "enter": // Show details
			m.showDetail = !m.showDetail

		case "/": // Toggle filter: dirty repos only
			if m.filter == FilterDirty {
				m.setFilter(FilterNone)
			} else {
				m.setFilter(FilterDirty)
			}

		case "1": // Show dirty only
			m.setFilter(FilterDirty)

		case "2": // Show clean only
			m.setFilter(FilterClean)

		case "3": // Show ahead only
			m.setFilter(FilterAhead)

		case "0": // Show all (clear filter)
			m.setFilter(FilterNone)
		}
	}

	return m, nil
}

// View renders the current UI state.
func (m StatusModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Header
	b.WriteString(renderHeader(m))
	b.WriteString("\n\n")

	// Repository list
	b.WriteString(renderRepoList(m))
	b.WriteString("\n\n")

	// Footer with actions
	b.WriteString(renderFooter(m))

	return b.String()
}

// renderHeader renders the header section.
func renderHeader(m StatusModel) string {
	selectedCount := len(m.selected)
	totalCount := len(m.repos)
	allCount := len(m.allRepos)

	titleText := fmt.Sprintf(" gz-git status --tui (%d selected / %d", selectedCount, totalCount)
	if m.filter != FilterNone {
		titleText += fmt.Sprintf(" of %d", allCount)
	}
	titleText += ") "

	if m.filter != FilterNone {
		titleText += fmt.Sprintf(" [Filter: %s]", m.filter)
	}

	return HeaderStyle.Render(titleText)
}

// renderRepoList renders the list of repositories.
func renderRepoList(m StatusModel) string {
	if len(m.repos) == 0 {
		return SubtleStyle.Render("  No repositories found")
	}

	var b strings.Builder

	// Calculate visible range (simple scrolling)
	visibleHeight := m.height - 10 // Reserve space for header/footer
	if visibleHeight < 1 {
		visibleHeight = 10
	}

	start := m.cursor - visibleHeight/2
	if start < 0 {
		start = 0
	}
	end := start + visibleHeight
	if end > len(m.repos) {
		end = len(m.repos)
		start = end - visibleHeight
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		repo := m.repos[i]
		isCursor := i == m.cursor
		isSelected := m.selected[repo.Repo.TargetPath]

		b.WriteString(renderRepoLine(repo, isCursor, isSelected))
		b.WriteString("\n")
	}

	// Show scroll indicator if needed
	if len(m.repos) > visibleHeight {
		scrollInfo := fmt.Sprintf("  (%d-%d of %d)", start+1, end, len(m.repos))
		b.WriteString(SubtleStyle.Render(scrollInfo))
	}

	return b.String()
}

// renderRepoLine renders a single repository line.
func renderRepoLine(repo reposync.RepoHealth, isCursor, isSelected bool) string {
	// Checkbox
	checkbox := "[ ]"
	if isSelected {
		checkbox = "[✓]"
	}

	// Repository name (shortened path)
	name := repo.Repo.Name
	if name == "" {
		name = repo.Repo.TargetPath
	}
	if len(name) > 30 {
		name = "..." + name[len(name)-27:]
	}

	// Branch
	branch := repo.CurrentBranch
	if branch == "" {
		branch = "HEAD"
	}
	if len(branch) > 15 {
		branch = branch[:12] + "..."
	}

	// Ahead/Behind
	divergence := fmt.Sprintf("↑%d ↓%d", repo.AheadBy, repo.BehindBy)

	// Status icon and text
	statusIcon, statusText := getStatusDisplay(repo)

	// Build line
	line := fmt.Sprintf("  %s %-30s %-15s %-10s %s %s",
		checkbox,
		name,
		branch,
		divergence,
		statusIcon,
		statusText,
	)

	// Apply styling
	if isCursor {
		return CursorStyle.Render(line)
	} else if repo.HealthStatus != reposync.HealthHealthy {
		return UnhealthyStyle.Render(line)
	} else if repo.WorkTreeStatus != reposync.WorkTreeClean {
		return DirtyStyle.Render(line)
	}

	return line
}

// getStatusDisplay returns status icon and text for a repository.
func getStatusDisplay(repo reposync.RepoHealth) (string, string) {
	// Health status
	switch repo.HealthStatus {
	case reposync.HealthHealthy:
		if repo.WorkTreeStatus != reposync.WorkTreeClean {
			return "✗", fmt.Sprintf("Dirty (%d files)", repo.ModifiedFiles+repo.UntrackedFiles)
		}
		if repo.DivergenceType != reposync.DivergenceNone {
			return "⚠", "Diverged"
		}
		return "✓", "Clean"

	case reposync.HealthWarning:
		if repo.ConflictFiles > 0 {
			return "✗", fmt.Sprintf("Conflict (%d files)", repo.ConflictFiles)
		}
		return "⚠", "Warning"

	case reposync.HealthError:
		if repo.NetworkStatus == reposync.NetworkUnreachable {
			return "✗", "Network error"
		}
		return "✗", "Error"

	case reposync.HealthUnreachable:
		return "⊘", "Unreachable"

	default:
		return "?", "Unknown"
	}
}

// renderFooter renders the footer with action hints.
func renderFooter(m StatusModel) string {
	selectedCount := len(m.selected)

	// Navigation actions (always visible)
	navActions := []string{
		"↑↓/j/k: Navigate",
		"Space: Toggle",
		"a: Select All",
		"n: None",
	}

	// Batch actions (visible when items selected)
	batchActions := []string{}
	if selectedCount > 0 {
		batchActions = append(batchActions,
			fmt.Sprintf("s: Sync (%d)", selectedCount),
			fmt.Sprintf("p: Pull (%d)", selectedCount),
			fmt.Sprintf("f: Fetch (%d)", selectedCount),
		)
	}

	// Filter actions
	filterActions := []string{"1: Dirty", "2: Clean", "3: Ahead", "0: All"}

	// Detail action
	detailActions := []string{"Enter: Details"}

	// Combine all actions
	allActions := append(navActions, batchActions...)
	allActions = append(allActions, filterActions...)
	allActions = append(allActions, detailActions...)
	allActions = append(allActions, "q: Quit")

	return SubtleStyle.Render("  " + strings.Join(allActions, "  │  "))
}

// GetSelectedPaths returns the list of selected repository paths.
func (m StatusModel) GetSelectedPaths() []string {
	paths := make([]string, 0, len(m.selected))
	for path, selected := range m.selected {
		if selected {
			paths = append(paths, path)
		}
	}
	return paths
}

// GetAction returns the pending action (sync, pull, fetch) or empty string.
func (m StatusModel) GetAction() string {
	return m.action
}

// setFilter applies a filter and resets cursor position.
func (m *StatusModel) setFilter(filter FilterType) {
	m.filter = filter
	if filter == FilterNone || filter == FilterAll {
		m.repos = m.allRepos
	} else {
		m.repos = m.applyFilter(filter)
	}
	m.cursor = 0
}

// executeAction sets the action and quits if items are selected.
func (m *StatusModel) executeAction(action string) (tea.Model, tea.Cmd) {
	if len(m.selected) > 0 {
		m.action = action
		return m, tea.Quit
	}
	return m, nil
}

// applyFilter filters repositories based on the filter type.
func (m StatusModel) applyFilter(filter FilterType) []reposync.RepoHealth {
	if filter == FilterNone || filter == FilterAll {
		return m.allRepos
	}

	filtered := make([]reposync.RepoHealth, 0)

	for _, repo := range m.allRepos {
		include := false

		switch filter {
		case FilterDirty:
			include = repo.WorkTreeStatus == reposync.WorkTreeDirty
		case FilterClean:
			include = repo.WorkTreeStatus == reposync.WorkTreeClean
		case FilterAhead:
			include = repo.AheadBy > 0
		}

		if include {
			filtered = append(filtered, repo)
		}
	}

	return filtered
}
