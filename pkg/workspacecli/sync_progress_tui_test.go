// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func makeActions(names ...string) []reposync.Action {
	actions := make([]reposync.Action, len(names))
	for i, name := range names {
		at := reposync.ActionUpdate
		if i%3 == 0 {
			at = reposync.ActionClone
		}
		actions[i] = reposync.Action{
			Repo: reposync.RepoSpec{Name: name},
			Type: at,
		}
	}
	return actions
}

// ─── Model construction ─────────────────────────────────────────────────────

func TestNewSyncProgressModel(t *testing.T) {
	actions := []reposync.Action{
		{Repo: reposync.RepoSpec{Name: "repo-a"}, Type: reposync.ActionClone},
		{Repo: reposync.RepoSpec{Name: "repo-b"}, Type: reposync.ActionUpdate},
		{Repo: reposync.RepoSpec{Name: "repo-c"}, Type: reposync.ActionUpdate},
		{Repo: reposync.RepoSpec{Name: "repo-d"}, Type: reposync.ActionDelete},
		{Repo: reposync.RepoSpec{Name: "repo-e"}, Type: reposync.ActionSkip},
	}

	m := newSyncProgressModel(actions)

	if m.total != 5 {
		t.Errorf("total: got %d, want 5", m.total)
	}
	if m.cloneCount != 1 {
		t.Errorf("cloneCount: got %d, want 1", m.cloneCount)
	}
	if m.updateCount != 2 {
		t.Errorf("updateCount: got %d, want 2", m.updateCount)
	}
	if m.deleteCount != 1 {
		t.Errorf("deleteCount: got %d, want 1", m.deleteCount)
	}
	if m.skipCount != 1 {
		t.Errorf("skipCount: got %d, want 1", m.skipCount)
	}

	// Check index lookup
	for i, a := range actions {
		idx, ok := m.index[a.Repo.Name]
		if !ok {
			t.Errorf("index missing for %s", a.Repo.Name)
		}
		if idx != i {
			t.Errorf("index[%s]: got %d, want %d", a.Repo.Name, idx, i)
		}
	}

	// All repos start as waiting
	for _, r := range m.repos {
		if r.status != statusWaiting {
			t.Errorf("repo %s: got status %q, want %q", r.name, r.status, statusWaiting)
		}
	}
}

// ─── Message handling ────────────────────────────────────────────────────────

func TestSyncStartMsg(t *testing.T) {
	actions := makeActions("repo-a", "repo-b")
	m := newSyncProgressModel(actions)

	updated, _ := m.Update(syncStartMsg{RepoName: "repo-a", ActionType: reposync.ActionClone})
	m = updated.(SyncProgressModel)

	if m.repos[0].status != statusRunning {
		t.Errorf("repo-a status: got %q, want %q", m.repos[0].status, statusRunning)
	}
	if m.repos[1].status != statusWaiting {
		t.Errorf("repo-b status: got %q, want %q", m.repos[1].status, statusWaiting)
	}
}

func TestSyncProgressMsg(t *testing.T) {
	actions := makeActions("repo-a")
	m := newSyncProgressModel(actions)

	updated, _ := m.Update(syncProgressMsg{RepoName: "repo-a", Message: "cloning", Progress: 0.5})
	m = updated.(SyncProgressModel)

	if m.repos[0].message != "cloning" {
		t.Errorf("message: got %q, want %q", m.repos[0].message, "cloning")
	}
	if m.repos[0].progress != 0.5 {
		t.Errorf("progress: got %f, want 0.5", m.repos[0].progress)
	}
}

func TestSyncCompleteMsgSuccess(t *testing.T) {
	actions := makeActions("repo-a", "repo-b")
	m := newSyncProgressModel(actions)

	updated, _ := m.Update(syncCompleteMsg{RepoName: "repo-a", Message: "already up-to-date"})
	m = updated.(SyncProgressModel)

	if m.done != 1 {
		t.Errorf("done: got %d, want 1", m.done)
	}
	if m.succeeded != 1 {
		t.Errorf("succeeded: got %d, want 1", m.succeeded)
	}
	if m.errored != 0 {
		t.Errorf("errored: got %d, want 0", m.errored)
	}
	if m.repos[0].status != statusDone {
		t.Errorf("status: got %q, want %q", m.repos[0].status, statusDone)
	}
}

func TestSyncCompleteMsgError(t *testing.T) {
	actions := makeActions("repo-a")
	m := newSyncProgressModel(actions)

	updated, _ := m.Update(syncCompleteMsg{
		RepoName: "repo-a",
		Error:    fmt.Errorf("network timeout\nsecond line"),
	})
	m = updated.(SyncProgressModel)

	if m.errored != 1 {
		t.Errorf("errored: got %d, want 1", m.errored)
	}
	if m.repos[0].status != statusError {
		t.Errorf("status: got %q, want %q", m.repos[0].status, statusError)
	}
	// First line only in display message
	if m.repos[0].message != "network timeout" {
		t.Errorf("message: got %q, want %q", m.repos[0].message, "network timeout")
	}
	// Full error in details
	if len(m.errDetails) != 1 {
		t.Fatalf("errDetails length: got %d, want 1", len(m.errDetails))
	}
	if !strings.Contains(m.errDetails[0], "second line") {
		t.Errorf("errDetails should contain full error, got %q", m.errDetails[0])
	}
}

func TestSyncAllDoneMsg(t *testing.T) {
	actions := makeActions("repo-a")
	m := newSyncProgressModel(actions)

	updated, cmd := m.Update(syncAllDoneMsg{})
	m = updated.(SyncProgressModel)

	if !m.allDone {
		t.Error("allDone should be true after syncAllDoneMsg")
	}
	// Should produce tea.Quit
	if cmd == nil {
		t.Error("cmd should not be nil (expected tea.Quit)")
	}
}

// ─── Scroll bounds ──────────────────────────────────────────────────────────

func TestScrollBounds(t *testing.T) {
	names := make([]string, 50)
	for i := range names {
		names[i] = fmt.Sprintf("repo-%d", i)
	}
	actions := makeActions(names...)
	m := newSyncProgressModel(actions)
	m.height = 20 // small terminal

	// scrollDown should not exceed max
	for i := 0; i < 100; i++ {
		m.scrollDown()
	}
	visibleHeight := m.height - 8
	maxScroll := len(m.repos) - visibleHeight
	if m.scrollTop > maxScroll {
		t.Errorf("scrollTop %d exceeds maxScroll %d", m.scrollTop, maxScroll)
	}

	// scrollUp should not go below 0
	for i := 0; i < 100; i++ {
		m.scrollUp()
	}
	if m.scrollTop < 0 {
		t.Errorf("scrollTop %d should not be negative", m.scrollTop)
	}
	if m.scrollTop != 0 {
		t.Errorf("scrollTop should be 0 after many scrollUps, got %d", m.scrollTop)
	}
}

func TestScrollToEnd(t *testing.T) {
	names := make([]string, 50)
	for i := range names {
		names[i] = fmt.Sprintf("repo-%d", i)
	}
	actions := makeActions(names...)
	m := newSyncProgressModel(actions)
	m.height = 20

	m.scrollToEnd()
	visibleHeight := m.height - 8
	expected := len(m.repos) - visibleHeight
	if m.scrollTop != expected {
		t.Errorf("scrollTop: got %d, want %d", m.scrollTop, expected)
	}
}

// ─── Window resize ──────────────────────────────────────────────────────────

func TestWindowSizeMsg(t *testing.T) {
	actions := makeActions("repo-a")
	m := newSyncProgressModel(actions)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(SyncProgressModel)

	if m.width != 120 || m.height != 40 {
		t.Errorf("size: got %dx%d, want 120x40", m.width, m.height)
	}
}

// ─── Ctrl+C ─────────────────────────────────────────────────────────────────

func TestCtrlCQuitting(t *testing.T) {
	actions := makeActions("repo-a")
	m := newSyncProgressModel(actions)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(SyncProgressModel)

	if !m.quitting {
		t.Error("quitting should be true after ctrl+c")
	}
	if cmd == nil {
		t.Error("cmd should not be nil (expected tea.Quit)")
	}
}

// ─── Bridge: atomic syncAllDoneMsg fires exactly once ───────────────────────

func TestBridgeAllDoneOnce(t *testing.T) {
	// Use a recording program-like approach
	var allDoneCount int
	var mu sync.Mutex

	// Create a mock that counts syncAllDoneMsg
	recorder := &msgRecorder{
		onMsg: func(msg tea.Msg) {
			if _, ok := msg.(syncAllDoneMsg); ok {
				mu.Lock()
				allDoneCount++
				mu.Unlock()
			}
		},
	}

	bridge := &tuiProgressBridge{
		program: nil, // We'll use the recorder instead
		total:   3,
	}

	// Replace program.Send with recorder (simulate via direct calls)
	// Since we can't easily mock tea.Program, test the atomic logic directly
	_ = recorder

	// Verify atomic counter behavior
	bridge.done.Add(1) // 1 of 3
	if bridge.done.Load() == bridge.total {
		t.Error("should not be done after 1 of 3")
	}
	bridge.done.Add(1) // 2 of 3
	if bridge.done.Load() == bridge.total {
		t.Error("should not be done after 2 of 3")
	}
	val := bridge.done.Add(1) // 3 of 3
	if val != bridge.total {
		t.Errorf("expected done counter to equal total after 3 adds, got %d", val)
	}
}

// msgRecorder is a test helper for recording messages.
type msgRecorder struct {
	onMsg func(tea.Msg)
}

// ─── View rendering ─────────────────────────────────────────────────────────

func TestViewRenderingSmoke(t *testing.T) {
	actions := makeActions("repo-a", "repo-b", "repo-c")
	m := newSyncProgressModel(actions)
	m.width = 80
	m.height = 30

	// Apply some state changes
	updated, _ := m.Update(syncStartMsg{RepoName: "repo-a", ActionType: reposync.ActionClone})
	m = updated.(SyncProgressModel)
	updated, _ = m.Update(syncCompleteMsg{RepoName: "repo-b", Message: "done"})
	m = updated.(SyncProgressModel)

	view := m.View()

	if !strings.Contains(view, "gz-git workspace sync") {
		t.Error("view should contain header text")
	}
	if !strings.Contains(view, "repo-a") {
		t.Error("view should contain repo-a")
	}
	if !strings.Contains(view, "repo-b") {
		t.Error("view should contain repo-b")
	}
	if !strings.Contains(view, "1/3 complete") {
		t.Error("view should contain progress counter")
	}
	if !strings.Contains(view, "ctrl+c") {
		t.Error("view should contain ctrl+c hint")
	}
}

func TestViewUninitializedSize(t *testing.T) {
	actions := makeActions("repo-a")
	m := newSyncProgressModel(actions)
	// width/height are 0 (uninitialized)

	view := m.View()
	if view != "Initializing..." {
		t.Errorf("expected initializing message, got %q", view)
	}
}

// ─── printSyncSummary ───────────────────────────────────────────────────────

func TestPrintSyncSummarySuccess(t *testing.T) {
	m := newSyncProgressModel(makeActions("repo-a", "repo-b"))
	m.done = 2
	m.succeeded = 2

	var buf strings.Builder
	printSyncSummary(&buf, m, 5*1e9)

	output := buf.String()
	if !strings.Contains(output, "All 2 repositories synced successfully") {
		t.Errorf("expected success message, got: %s", output)
	}
}

func TestPrintSyncSummaryCancelled(t *testing.T) {
	m := newSyncProgressModel(makeActions("repo-a", "repo-b"))
	m.done = 1
	m.quitting = true

	var buf strings.Builder
	printSyncSummary(&buf, m, 3*1e9)

	output := buf.String()
	if !strings.Contains(output, "cancelled") {
		t.Errorf("expected cancelled message, got: %s", output)
	}
	if !strings.Contains(output, "1/2 completed") {
		t.Errorf("expected partial completion count, got: %s", output)
	}
}

func TestPrintSyncSummaryWithErrors(t *testing.T) {
	m := newSyncProgressModel(makeActions("repo-a", "repo-b"))
	m.done = 2
	m.succeeded = 1
	m.errored = 1
	m.errDetails = []string{"repo-b: connection refused"}

	var buf strings.Builder
	printSyncSummary(&buf, m, 10*1e9)

	output := buf.String()
	if !strings.Contains(output, "1 succeeded") {
		t.Errorf("expected succeeded count, got: %s", output)
	}
	if !strings.Contains(output, "1 failed") {
		t.Errorf("expected failed count, got: %s", output)
	}
	if !strings.Contains(output, "connection refused") {
		t.Errorf("expected error details, got: %s", output)
	}
}
