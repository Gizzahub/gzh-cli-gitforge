// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Planning spinner model ─────────────────────────────────────────────────

type planPhaseMsg string

type planDoneMsg struct{}

type planSpinnerTickMsg time.Time

// PlanningSpinnerModel shows a single-line spinner during the planning phase.
type PlanningSpinnerModel struct {
	phase        string
	spinnerFrame int
	done         bool
}

func newPlanningSpinnerModel() PlanningSpinnerModel {
	return PlanningSpinnerModel{
		phase: "Planning...",
	}
}

func (m PlanningSpinnerModel) Init() tea.Cmd {
	return planSpinnerTickCmd()
}

func planSpinnerTickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return planSpinnerTickMsg(t)
	})
}

func (m PlanningSpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case planPhaseMsg:
		m.phase = string(msg)
		return m, nil
	case planDoneMsg:
		m.done = true
		return m, tea.Quit
	case planSpinnerTickMsg:
		_ = msg
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, planSpinnerTickCmd()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m PlanningSpinnerModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("\r%s %s", spinnerFrames[m.spinnerFrame], m.phase)
}

// ─── planningProgressWriter ─────────────────────────────────────────────────

// planningProgressWriter intercepts planning output to update the spinner phase.
type planningProgressWriter struct {
	program  *tea.Program
	fallback io.Writer
}

func newPlanningProgressWriter(program *tea.Program, fallback io.Writer) *planningProgressWriter {
	return &planningProgressWriter{
		program:  program,
		fallback: fallback,
	}
}

func (w *planningProgressWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}

	// Parse known planning message patterns and update spinner
	switch {
	case strings.HasPrefix(msg, "Planning nested workspace"):
		w.program.Send(planPhaseMsg(msg))
	case strings.HasPrefix(msg, "Planning git workspace"):
		w.program.Send(planPhaseMsg(msg))
	case strings.HasPrefix(msg, "Planning config workspace"):
		w.program.Send(planPhaseMsg(msg))
	case strings.HasPrefix(msg, "Found ") && strings.Contains(msg, "repositories"):
		w.program.Send(planPhaseMsg(msg))
	case strings.HasPrefix(msg, "Found ") && strings.Contains(msg, "workspaces"):
		w.program.Send(planPhaseMsg(msg))
	case strings.Contains(msg, "Config warning"):
		// Pass warnings through to fallback
		return w.fallback.Write(p)
	default:
		// Ignore most planning output in TTY mode (spinner covers it)
		// Only pass through warnings and errors
		if strings.Contains(msg, "⚠") || strings.Contains(msg, "error") || strings.Contains(msg, "Error") {
			return w.fallback.Write(p)
		}
	}

	return len(p), nil
}
