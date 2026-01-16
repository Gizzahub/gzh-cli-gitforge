# Advanced TUI Design

## Overview

**Feature**: Rich terminal UI for complex operations
**Priority**: P1 (highest in Phase 8)
**Phase**: 8.1
**Status**: Design

## Problem Statement

### Current Limitations (v0.4.0)

**Problem 1: Text-only output lacks interactivity**
```bash
$ gz-git status
Repository: /home/user/repo1 (main)
  Status: clean
  Ahead: 2, Behind: 1

Repository: /home/user/repo2 (develop)
  Status: dirty
  Modified: 3 files
  Ahead: 0, Behind: 5

Repository: /home/user/repo3 (feature/new-api)
  Status: clean
  Ahead: 1, Behind: 0
...
# 50 more repositories
```

**Issues**:
- ❌ No way to select specific repos
- ❌ Cannot perform batch operations
- ❌ No real-time updates during sync
- ❌ Hard to scan visually (50+ repos)
- ❌ No filtering or sorting

**Problem 2: Complex multi-repo workflows are tedious**
```bash
# Want to sync only dirty repos?
# Must: read output → identify repos → run sync with paths manually
gz-git sync from-config -c sync.yaml /path/to/repo2 /path/to/repo7 ...
```

## Goals

1. **Visual clarity**: Easy to scan status of many repos
2. **Interactivity**: Select repos, filter, sort
3. **Batch operations**: Act on selected repos
4. **Real-time feedback**: Live progress updates
5. **Optional**: TUI is opt-in (`--interactive` flag)

## Design

### 1. TUI Framework Choice

**Decision**: Bubble Tea (Charm.sh)

**Why**:
- ✅ Modern, actively maintained
- ✅ Rich ecosystem (lipgloss for styling, bubbles for components)
- ✅ Elm-inspired architecture (predictable state management)
- ✅ Great documentation and examples
- ✅ Used by popular tools (gh dash, soft serve)

**Alternatives considered**:
- tview: More widget-based, but heavier
- termui: Unmaintained
- Custom: Too much effort

### 2. UI Mockups

#### Status View (Interactive)

```
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ gz-git status --interactive                              Filter: │
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

┌─ Repositories (12 selected / 50 total) ──────────────────────────┐
│                                                                   │
│ [x] backend-api      main      ↑2 ↓1  ✓ Clean                   │
│ [x] frontend-web     develop   ↑0 ↓5  ✗ Dirty (3 files)         │
│ [ ] mobile-app       main      ↑0 ↓0  ✓ Clean                   │
│ [x] data-pipeline    feature   ↑1 ↓0  ⟳ Syncing... [████░░] 65% │
│ [ ] ml-models        main      ↑0 ↓3  ✓ Clean                   │
│ ...                                                               │
│                                                                   │
│ 47 more (scroll with j/k or arrows)                              │
└───────────────────────────────────────────────────────────────────┘

┌─ Details: backend-api ────────────────────────────────────────────┐
│ Branch: main                                                      │
│ Remote: origin (git@github.com:company/backend-api.git)          │
│ Status: Clean                                                     │
│ Commits: 2 ahead, 1 behind                                        │
│ Last commit: feat: add user API (2 hours ago)                    │
└───────────────────────────────────────────────────────────────────┘

┌─ Actions ─────────────────────────────────────────────────────────┐
│ [Space] Toggle  [a] Select All  [n] Select None  [/] Filter      │
│ [Enter] Details [s] Sync        [p] Pull         [f] Fetch       │
│ [q] Quit        [?] Help                                          │
└───────────────────────────────────────────────────────────────────┘
```

**Key Features**:
- Checkbox selection (`[x]` / `[ ]`)
- Status icons (`✓` clean, `✗` dirty, `⟳` syncing)
- Ahead/behind indicators (`↑2 ↓1`)
- Progress bars for active operations
- Details pane for selected repo
- Action bar at bottom

#### Sync Progress View

```
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ gz-git sync from-config --interactive                            ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

┌─ Overall Progress ────────────────────────────────────────────────┐
│ 12 / 50 completed  [█████░░░░░░░░░░░░░░░░] 24%  ETA: 3m 15s     │
└───────────────────────────────────────────────────────────────────┘

┌─ Active Operations (Parallel: 5) ─────────────────────────────────┐
│                                                                   │
│ ⟳ backend-api        Pulling... [████████░░] 85%                 │
│ ⟳ frontend-web       Cloning... [███░░░░░░░] 35%                 │
│ ⟳ data-pipeline      Fetching... [██████████] 100%               │
│ ⟳ ml-models          Pulling... [██░░░░░░░░] 25%                 │
│ ⟳ auth-service       Cloning... [█░░░░░░░░░] 15%                 │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘

┌─ Completed (12) ──────────────────────────────────────────────────┐
│ ✓ admin-dashboard    Synced (2.3s)                               │
│ ✓ api-gateway        Synced (1.8s)                               │
│ ✓ monitoring         Synced (3.1s)                               │
│ ...                                                               │
└───────────────────────────────────────────────────────────────────┘

┌─ Failed (2) ──────────────────────────────────────────────────────┐
│ ✗ legacy-api         Merge conflict (needs manual resolution)    │
│ ✗ deprecated-sdk     Authentication failed                       │
└───────────────────────────────────────────────────────────────────┘

[p] Pause  [r] Resume  [c] Cancel  [d] Show Details  [q] Quit
```

**Key Features**:
- Overall progress bar with ETA
- Live updates of active operations
- Per-repo progress bars
- Categorized results (active, completed, failed)
- Pause/resume capability

#### Cleanup View (Branch Selection)

```
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ gz-git cleanup branch --interactive                              ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

Repository: backend-api (15 stale branches found)

┌─ Select Branches to Delete ───────────────────────────────────────┐
│                                                                   │
│ [x] feature/old-login     Merged   30 days ago   main            │
│ [x] bugfix/temp-fix       Merged   15 days ago   develop         │
│ [ ] feature/wip           Active    2 days ago   -               │
│ [x] release/v1.0.0        Merged   60 days ago   main            │
│ [ ] feature/important     Active    1 day ago    -               │
│ [x] hotfix/security       Merged   45 days ago   main            │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘

┌─ Summary ─────────────────────────────────────────────────────────┐
│ Selected: 4 branches                                              │
│ Safe to delete: 4 (all merged)                                    │
│ Warnings: 0                                                       │
└───────────────────────────────────────────────────────────────────┘

[Space] Toggle  [a] Select All Merged  [n] Deselect All
[Enter] Confirm Deletion  [q] Cancel
```

### 3. Key Components

#### Model-Update-View (Bubble Tea Architecture)

```go
// Model holds the application state
type StatusModel struct {
    repos       []RepoStatus
    selected    map[string]bool
    cursor      int
    filter      string
    loading     bool
    syncProgress map[string]int // repo -> progress %
}

// Msg represents events
type Msg interface{}
type repoUpdatedMsg struct{ repo RepoStatus }
type syncProgressMsg struct{ repo string; progress int }
type syncCompleteMsg struct{ repo string; err error }

// Init initializes the model
func (m StatusModel) Init() tea.Cmd {
    return loadRepos()
}

// Update handles messages and updates state
func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case " ": // Space - toggle selection
            m.toggleSelection()
        case "s": // Sync selected repos
            return m, syncRepos(m.getSelected())
        }
    case syncProgressMsg:
        m.syncProgress[msg.repo] = msg.progress
    }
    return m, nil
}

// View renders the UI
func (m StatusModel) View() string {
    var b strings.Builder

    // Header
    b.WriteString(headerStyle.Render("gz-git status --interactive"))

    // Repo list
    for i, repo := range m.repos {
        line := renderRepoLine(repo, m.selected[repo.Name], i == m.cursor)
        b.WriteString(line)
    }

    // Actions
    b.WriteString(actionsStyle.Render("Actions..."))

    return b.String()
}
```

#### Styling with Lipgloss

```go
import "github.com/charmbracelet/lipgloss"

var (
    headerStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("15")).
        Background(lipgloss.Color("62")).
        Padding(0, 1)

    selectedStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("10")).
        Bold(true)

    dirtyStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("9"))

    cleanStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("10"))

    progressStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("14"))
)

func renderRepoLine(repo RepoStatus, selected bool, cursor bool) string {
    checkbox := "[ ]"
    if selected {
        checkbox = "[x]"
    }

    status := cleanStyle.Render("✓ Clean")
    if repo.IsDirty {
        status = dirtyStyle.Render(fmt.Sprintf("✗ Dirty (%d files)", repo.ModifiedFiles))
    }

    line := fmt.Sprintf("%s %s %s %s %s",
        checkbox,
        repo.Name,
        repo.Branch,
        fmt.Sprintf("↑%d ↓%d", repo.Ahead, repo.Behind),
        status,
    )

    if cursor {
        return selectedStyle.Render("> " + line)
    }
    return "  " + line
}
```

### 4. Real-time Updates

**Challenge**: Update UI while long-running operations execute

**Solution**: Goroutines + channels + Bubble Tea messages

```go
// Sync repos in background, send progress updates
func syncRepos(repos []string) tea.Cmd {
    return func() tea.Msg {
        // Create channel for progress updates
        progressCh := make(chan syncProgressMsg)

        // Start sync in goroutine
        go func() {
            for _, repo := range repos {
                err := performSync(repo, progressCh)
                if err != nil {
                    // Send error msg
                }
            }
            close(progressCh)
        }()

        // Return command that listens to channel
        return tea.Batch(
            listenForProgress(progressCh),
        )
    }
}

func listenForProgress(ch <-chan syncProgressMsg) tea.Cmd {
    return func() tea.Msg {
        return <-ch // Block until message arrives
    }
}
```

### 5. Command Integration

**Flag-based activation**:
```bash
# Traditional text output
gz-git status

# Interactive TUI
gz-git status --interactive
# or
gz-git status -i
```

**Implementation**:
```go
func runStatus(cmd *cobra.Command, args []string) error {
    interactive, _ := cmd.Flags().GetBool("interactive")

    repos, err := loadRepos()
    if err != nil {
        return err
    }

    if interactive {
        // Launch TUI
        model := NewStatusModel(repos)
        p := tea.NewProgram(model)
        if err := p.Start(); err != nil {
            return err
        }
    } else {
        // Traditional output
        for _, repo := range repos {
            fmt.Printf("Repository: %s\n", repo.Name)
            // ...
        }
    }

    return nil
}
```

## Implementation Plan

### Week 1: Foundation
- [ ] **Day 1-2**: Bubble Tea setup and basic model
  - Initialize Bubble Tea project
  - Create basic status model (repos list)
  - Keyboard navigation (j/k, arrows)
  - Quit functionality (q)

- [ ] **Day 3-4**: Styling and layout
  - Lipgloss integration
  - Header, list, details, actions sections
  - Colors and icons
  - Responsive layout

- [ ] **Day 5**: Selection mechanism
  - Checkbox toggling (Space)
  - Select all / none (a, n)
  - Track selected repos

### Week 2: Interactivity
- [ ] **Day 1-2**: Filtering and sorting
  - Filter input (/)
  - Filter by status (dirty, clean)
  - Sort by name, status, ahead/behind

- [ ] **Day 3-4**: Real-time sync
  - Background sync execution
  - Progress updates via channels
  - Live progress bars
  - Error handling

- [ ] **Day 5**: Polish
  - Help screen (?)
  - Keyboard shortcuts hints
  - Error messages
  - Loading states

### Week 3: Additional Views
- [ ] **Day 1-2**: Cleanup TUI
  - Branch selection view
  - Merged/stale detection
  - Batch delete confirmation

- [ ] **Day 3-4**: Testing
  - Unit tests (model logic)
  - Integration tests (full TUI flow)
  - Manual testing (various terminal sizes)

- [ ] **Day 5**: Documentation
  - TUI usage guide
  - Keyboard shortcuts reference
  - Screenshots/GIFs

## Testing Strategy

### Unit Tests
```go
func TestStatusModel_ToggleSelection(t *testing.T) {
    m := StatusModel{
        repos: []RepoStatus{{Name: "repo1"}},
        selected: make(map[string]bool),
    }
    m.toggleSelection()
    assert.True(t, m.selected["repo1"])
}
```

### Integration Tests
```go
func TestTUI_SelectAndSync(t *testing.T) {
    // Send key sequence: Space, s, Enter
    // Verify sync called with correct repos
}
```

### Manual Testing
- [ ] Different terminal sizes (80x24, 120x40, 200x50)
- [ ] Color terminal vs monochrome
- [ ] Tmux/screen compatibility
- [ ] SSH sessions
- [ ] Large repo lists (100+, 1000+)

## Dependencies

```go
require (
    github.com/charmbracelet/bubbletea v0.25.0
    github.com/charmbracelet/lipgloss v0.9.1
    github.com/charmbracelet/bubbles v0.17.1 // Optional: pre-built components
)
```

## Performance Considerations

1. **Large repo lists** (1000+):
   - Virtualized scrolling (render only visible rows)
   - Lazy loading (load repos on demand)
   - Pagination

2. **Real-time updates**:
   - Throttle updates (max 10 FPS)
   - Batch UI updates
   - Debounce rapid changes

3. **Memory usage**:
   - Don't load full git history in TUI
   - Stream output instead of buffering
   - Clean up goroutines on exit

## Accessibility

- [ ] Keyboard-only navigation (no mouse required)
- [ ] Clear visual hierarchy (headers, borders)
- [ ] Color-blind friendly (not relying solely on color)
- [ ] High-contrast mode
- [ ] Screen reader support (via plain text fallback)

## Future Enhancements

- [ ] Mouse support (click to select)
- [ ] Search/filter with regex
- [ ] Save selections to config
- [ ] Custom keybindings
- [ ] Themes (light/dark/custom)
- [ ] TUI for other commands (branch, tag, etc.)

## References

- [Bubble Tea Tutorial](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
- [Lipgloss Examples](https://github.com/charmbracelet/lipgloss/tree/master/examples)
- [gh dash](https://github.com/dlvhdr/gh-dash) - Similar TUI for GitHub
- [lazygit](https://github.com/jesseduffield/lazygit) - TUI Git client (inspiration)

---

**Version**: 1.0
**Last Updated**: 2026-01-16
**Author**: Design spec for implementation
