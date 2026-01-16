// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// StatusProgressIndicator provides real-time progress updates for health checks.
type StatusProgressIndicator struct {
	out      io.Writer
	total    int
	current  int
	mu       sync.Mutex
	startTime time.Time
	quiet    bool
}

// NewStatusProgressIndicator creates a progress indicator.
func NewStatusProgressIndicator(out io.Writer, total int, quiet bool) *StatusProgressIndicator {
	return &StatusProgressIndicator{
		out:       out,
		total:     total,
		current:   0,
		startTime: time.Now(),
		quiet:     quiet,
	}
}

// OnStart is called when a repository health check starts.
func (p *StatusProgressIndicator) OnStart(repo reposync.RepoSpec) {
	if p.quiet {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.current++
	name := repo.Name
	if name == "" {
		name = repo.TargetPath
	}

	elapsed := time.Since(p.startTime)
	fmt.Fprintf(p.out, "\r[%d/%d] Checking %s... (%.1fs)",
		p.current, p.total, name, elapsed.Seconds())
}

// OnComplete is called when a repository health check completes.
func (p *StatusProgressIndicator) OnComplete(health reposync.RepoHealth) {
	// Progress is shown in OnStart, just update the indicator
	if !p.quiet {
		p.mu.Lock()
		defer p.mu.Unlock()

		// Optionally show icon for completed check
		icon := ""
		switch health.HealthStatus {
		case reposync.HealthHealthy:
			icon = "✓"
		case reposync.HealthWarning:
			icon = "⚠"
		case reposync.HealthError:
			icon = "✗"
		case reposync.HealthUnreachable:
			icon = "⊘"
		}

		// Brief inline update (overwritten by next OnStart)
		name := health.Repo.Name
		if name == "" {
			name = health.Repo.TargetPath
		}
		fmt.Fprintf(p.out, "\r%s %s (%.0fms)                    ",
			icon, name, float64(health.Duration.Milliseconds()))
	}
}

// Done clears the progress line.
func (p *StatusProgressIndicator) Done() {
	if !p.quiet {
		fmt.Fprintf(p.out, "\r%80s\r", "") // Clear line
	}
}

// DiagnosticProgressAdapter adapts StatusProgressIndicator to the diagnostic executor's needs.
type DiagnosticProgressAdapter struct {
	indicator *StatusProgressIndicator
}

// OnRepoStart notifies when a repository check starts.
func (a *DiagnosticProgressAdapter) OnRepoStart(repo reposync.RepoSpec) {
	if a.indicator != nil {
		a.indicator.OnStart(repo)
	}
}

// OnRepoComplete notifies when a repository check completes.
func (a *DiagnosticProgressAdapter) OnRepoComplete(health reposync.RepoHealth) {
	if a.indicator != nil {
		a.indicator.OnComplete(health)
	}
}
