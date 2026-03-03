// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/tui"
)

// ─── Bubble Tea messages ─────────────────────────────────────────────────────

type syncStartMsg struct {
	RepoName   string
	ActionType reposync.ActionType
}

type syncProgressMsg struct {
	RepoName string
	Message  string
	Progress float64
}

type syncCompleteMsg struct {
	RepoName string
	Message  string
	Error    error
}

type syncAllDoneMsg struct{}

type syncTickMsg time.Time

// ─── Per-repo state ──────────────────────────────────────────────────────────

type syncRepoStatus string

const (
	statusWaiting syncRepoStatus = "waiting"
	statusRunning syncRepoStatus = "running"
	statusDone    syncRepoStatus = "done"
	statusError   syncRepoStatus = "error"
)

type syncRepoState struct {
	name       string
	actionType reposync.ActionType
	status     syncRepoStatus
	message    string
	progress   float64
}

// ─── SyncProgressModel ──────────────────────────────────────────────────────

// SyncProgressModel is a Bubble Tea model for displaying sync progress
// in an alternate screen buffer. It supports viewport scrolling when
// the repository list exceeds terminal height.
type SyncProgressModel struct {
	repos     []syncRepoState
	index     map[string]int // repo name → index in repos
	total     int
	done      int
	errored   int
	succeeded int

	cloneCount  int
	updateCount int
	deleteCount int
	skipCount   int

	errDetails []string // post-summary error details

	width     int
	height    int
	scrollTop int

	startTime    time.Time
	allDone      bool
	quitting     bool
	spinnerFrame int
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// newSyncProgressModel creates a model from planned actions.
func newSyncProgressModel(actions []reposync.Action) SyncProgressModel {
	repos := make([]syncRepoState, 0, len(actions))
	idx := make(map[string]int, len(actions))

	var clones, updates, deletes, skips int
	for i, a := range actions {
		repos = append(repos, syncRepoState{
			name:       a.Repo.Name,
			actionType: a.Type,
			status:     statusWaiting,
		})
		idx[a.Repo.Name] = i

		switch a.Type {
		case reposync.ActionClone:
			clones++
		case reposync.ActionUpdate:
			updates++
		case reposync.ActionDelete:
			deletes++
		case reposync.ActionSkip:
			skips++
		}
	}

	return SyncProgressModel{
		repos:       repos,
		index:       idx,
		total:       len(actions),
		cloneCount:  clones,
		updateCount: updates,
		deleteCount: deletes,
		skipCount:   skips,
		startTime:   time.Now(),
	}
}

// Init starts the spinner tick.
func (m SyncProgressModel) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return syncTickMsg(t)
	})
}

// Update processes messages and updates model state.
func (m SyncProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "j", "down":
			m.scrollDown()
		case "k", "up":
			m.scrollUp()
		case "G", "end":
			m.scrollToEnd()
		case "g", "home":
			m.scrollTop = 0
		}
		return m, nil

	case syncStartMsg:
		if idx, ok := m.index[msg.RepoName]; ok {
			m.repos[idx].status = statusRunning
			m.repos[idx].message = string(msg.ActionType) + "..."
		}
		return m, nil

	case syncProgressMsg:
		if idx, ok := m.index[msg.RepoName]; ok {
			m.repos[idx].message = msg.Message
			m.repos[idx].progress = msg.Progress
		}
		return m, nil

	case syncCompleteMsg:
		m.done++
		if idx, ok := m.index[msg.RepoName]; ok {
			if msg.Error != nil {
				m.errored++
				m.repos[idx].status = statusError
				errMsg := msg.Error.Error()
				firstLine := errMsg
				if nl := strings.Index(errMsg, "\n"); nl > 0 {
					firstLine = errMsg[:nl]
				}
				m.repos[idx].message = firstLine
				m.errDetails = append(m.errDetails, fmt.Sprintf("%s: %s", msg.RepoName, errMsg))
			} else {
				m.succeeded++
				m.repos[idx].status = statusDone
				m.repos[idx].message = msg.Message
			}
		}
		return m, nil

	case syncAllDoneMsg:
		m.allDone = true
		return m, tea.Quit

	case syncTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, tickCmd()
	}

	return m, nil
}

// View renders the TUI in the alternate screen.
func (m SyncProgressModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")

	// Repository list with viewport
	repoLines := m.renderRepoList()
	b.WriteString(repoLines)

	// Footer / progress bar
	b.WriteString("\n")
	b.WriteString(m.renderProgressBar())
	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// ─── View helpers ────────────────────────────────────────────────────────────

func (m SyncProgressModel) renderHeader() string {
	var parts []string
	if m.cloneCount > 0 {
		parts = append(parts, fmt.Sprintf("+%d clone", m.cloneCount))
	}
	if m.updateCount > 0 {
		parts = append(parts, fmt.Sprintf("↓%d update", m.updateCount))
	}
	if m.deleteCount > 0 {
		parts = append(parts, fmt.Sprintf("✗%d delete", m.deleteCount))
	}
	if m.skipCount > 0 {
		parts = append(parts, fmt.Sprintf("⊘%d skip", m.skipCount))
	}
	summary := strings.Join(parts, "  ")

	title := fmt.Sprintf(" gz-git workspace sync  (%d repositories)  [%s] ", m.total, summary)
	return tui.HeaderStyle.Render(title)
}

func (m SyncProgressModel) renderRepoList() string {
	// Calculate visible area: total height minus header(2) + footer(3) + progress(1) + padding(2)
	visibleHeight := m.height - 8
	if visibleHeight < 5 {
		visibleHeight = 5
	}
	if visibleHeight > len(m.repos) {
		visibleHeight = len(m.repos)
	}

	// Clamp scrollTop
	maxScroll := len(m.repos) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollTop > maxScroll {
		m.scrollTop = maxScroll
	}

	var b strings.Builder
	end := m.scrollTop + visibleHeight
	if end > len(m.repos) {
		end = len(m.repos)
	}

	for i := m.scrollTop; i < end; i++ {
		r := m.repos[i]
		b.WriteString(m.renderRepoLine(r))
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.repos) > visibleHeight {
		scrollInfo := fmt.Sprintf("  (%d-%d of %d)  j/k: scroll", m.scrollTop+1, end, len(m.repos))
		b.WriteString(tui.SubtleStyle.Render(scrollInfo))
	}

	return b.String()
}

func (m SyncProgressModel) renderRepoLine(r syncRepoState) string {
	var icon, msg string

	switch r.status {
	case statusWaiting:
		icon = "○"
		msg = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("waiting")
	case statusRunning:
		icon = spinnerFrames[m.spinnerFrame]
		fmtMsg := r.message
		if r.progress > 0 {
			fmtMsg = fmt.Sprintf("%s (%.0f%%)", r.message, r.progress*100)
		}
		msg = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(fmtMsg)
	case statusDone:
		icon = "✓"
		msg = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(r.message)
	case statusError:
		icon = "✗"
		msg = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(r.message)
	}

	// Truncate message to keep lines within terminal width
	name := r.name
	if len(name) > 30 {
		name = name[:27] + "..."
	}
	truncMsg := msg
	// Rough truncation based on raw length (lipgloss escapes make exact difficult)
	if len(r.message) > 45 {
		truncMsg = msg[:len(msg)-len(r.message)+42] + "..."
	}

	return fmt.Sprintf("  %s %-30s  %s", icon, name, truncMsg)
}

func (m SyncProgressModel) renderProgressBar() string {
	elapsed := time.Since(m.startTime).Truncate(time.Second)

	barWidth := 24
	filled := 0
	if m.total > 0 {
		filled = (m.done * barWidth) / m.total
	}
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	bar := fmt.Sprintf("[%s%s]", strings.Repeat("█", filled), strings.Repeat("░", empty))

	status := fmt.Sprintf("  %s  %d/%d complete", bar, m.done, m.total)
	if m.errored > 0 {
		status += fmt.Sprintf("  (%d error)", m.errored)
	}
	status += fmt.Sprintf("  %s elapsed", elapsed)

	return status
}

func (m SyncProgressModel) renderFooter() string {
	return tui.SubtleStyle.Render("  ctrl+c: cancel  j/k: scroll")
}

// ─── Scroll helpers ──────────────────────────────────────────────────────────

func (m *SyncProgressModel) scrollDown() {
	visibleHeight := m.height - 8
	if visibleHeight < 5 {
		visibleHeight = 5
	}
	maxScroll := len(m.repos) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollTop < maxScroll {
		m.scrollTop++
	}
}

func (m *SyncProgressModel) scrollUp() {
	if m.scrollTop > 0 {
		m.scrollTop--
	}
}

func (m *SyncProgressModel) scrollToEnd() {
	visibleHeight := m.height - 8
	if visibleHeight < 5 {
		visibleHeight = 5
	}
	maxScroll := len(m.repos) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.scrollTop = maxScroll
}

// ─── tuiProgressBridge ──────────────────────────────────────────────────────

// tuiProgressBridge implements reposync.ProgressSink by forwarding events
// to a Bubble Tea program via program.Send() (goroutine-safe).
type tuiProgressBridge struct {
	program *tea.Program
	total   int32
	done    atomic.Int32
}

func newTUIProgressBridge(program *tea.Program, total int) *tuiProgressBridge {
	return &tuiProgressBridge{
		program: program,
		total:   int32(total),
	}
}

// OnStart implements reposync.ProgressSink.
func (b *tuiProgressBridge) OnStart(action reposync.Action) {
	b.program.Send(syncStartMsg{
		RepoName:   action.Repo.Name,
		ActionType: action.Type,
	})
}

// OnProgress implements reposync.ProgressSink.
func (b *tuiProgressBridge) OnProgress(action reposync.Action, message string, progress float64) {
	if strings.Contains(message, "retrying") {
		return
	}
	b.program.Send(syncProgressMsg{
		RepoName: action.Repo.Name,
		Message:  message,
		Progress: progress,
	})
}

// OnComplete implements reposync.ProgressSink.
func (b *tuiProgressBridge) OnComplete(result reposync.ActionResult) {
	var errVal error
	if result.Error != nil {
		errVal = result.Error
	}
	b.program.Send(syncCompleteMsg{
		RepoName: result.Action.Repo.Name,
		Message:  result.Message,
		Error:    errVal,
	})

	if b.done.Add(1) == b.total {
		b.program.Send(syncAllDoneMsg{})
	}
}

// ─── syncTUIResult ──────────────────────────────────────────────────────────

// syncTUIResult holds the outcome of the TUI-driven sync.
type syncTUIResult struct {
	ExecResult reposync.ExecutionResult
	Duration   time.Duration
	Cancelled  bool
	ErrDetails []string
}

// ─── runSyncTUI ─────────────────────────────────────────────────────────────

// runSyncTUI runs workspace sync inside a Bubble Tea alternate screen.
// It owns the full lifecycle: model → program → bridge → orch.Run → summary.
func runSyncTUI(
	ctx context.Context,
	out io.Writer,
	actions []reposync.Action,
	runOpts reposync.RunOptions,
	state reposync.StateStore,
	planner reposync.Planner,
	executor reposync.Executor,
) (*syncTUIResult, error) {
	model := newSyncProgressModel(actions)

	program := tea.NewProgram(model, tea.WithAltScreen())
	bridge := newTUIProgressBridge(program, len(actions))

	// Context with cancel for Ctrl+C propagation
	orchCtx, orchCancel := context.WithCancel(ctx)
	defer orchCancel()

	orch := reposync.NewOrchestrator(planner, executor, state)

	// Channel to collect orch.Run result
	type orchResult struct {
		exec reposync.ExecutionResult
		err  error
	}
	orchCh := make(chan orchResult, 1)

	// Run orchestrator in background goroutine
	go func() {
		execResult, err := orch.Run(orchCtx, reposync.RunRequest{
			RunOptions: runOpts,
			Progress:   bridge,
			State:      state,
		})
		orchCh <- orchResult{exec: execResult, err: err}
	}()

	// Run TUI (blocks until tea.Quit)
	finalModel, err := program.Run()
	if err != nil {
		orchCancel()
		return nil, fmt.Errorf("TUI error: %w", err)
	}

	// Extract final model state
	fm, ok := finalModel.(SyncProgressModel)
	if !ok {
		orchCancel()
		return nil, fmt.Errorf("unexpected model type after TUI run")
	}

	result := &syncTUIResult{
		Duration:   time.Since(fm.startTime),
		Cancelled:  fm.quitting,
		ErrDetails: fm.errDetails,
	}

	// If user cancelled, cancel orchestrator and drain
	if fm.quitting {
		orchCancel()
	}

	// Wait for orchestrator to finish
	orchRes := <-orchCh
	if orchRes.err != nil && !fm.quitting {
		return nil, fmt.Errorf("workspace sync failed: %w", orchRes.err)
	}
	result.ExecResult = orchRes.exec

	// Print summary to normal screen
	printSyncSummary(out, fm, result.Duration)

	return result, nil
}

// printSyncSummary outputs the final result on the normal screen after
// the alternate screen exits.
func printSyncSummary(out io.Writer, m SyncProgressModel, elapsed time.Duration) {
	fmt.Fprintln(out)

	if m.quitting {
		fmt.Fprintf(out, "\033[33m⚠  Sync cancelled by user after %s.\033[0m\n", elapsed.Truncate(time.Second))
		if m.done > 0 {
			fmt.Fprintf(out, "   %d/%d completed before cancellation.\n", m.done, m.total)
		}
		return
	}

	if m.errored == 0 {
		fmt.Fprintf(out, "\033[32m✓ All %d repositories synced successfully.\033[0m  (%s)\n",
			m.total, elapsed.Truncate(time.Second))
	} else {
		succeeded := m.done - m.errored
		fmt.Fprintf(out, "\033[33m⚠  %d succeeded, %d failed out of %d repositories.\033[0m  (%s)\n",
			succeeded, m.errored, m.total, elapsed.Truncate(time.Second))
		fmt.Fprintf(out, "\n\033[31mErrors:\033[0m\n")
		for _, detail := range m.errDetails {
			for i, line := range strings.Split(detail, "\n") {
				if i == 0 {
					fmt.Fprintf(out, "  ✗ %s\n", line)
				} else if strings.TrimSpace(line) != "" {
					fmt.Fprintf(out, "    %s\n", line)
				}
			}
		}
	}
}
