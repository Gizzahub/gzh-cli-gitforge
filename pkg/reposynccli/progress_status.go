// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"
	"io"
	"sync"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// StatusProgressIndicator provides real-time progress updates for health checks.
type StatusProgressIndicator struct {
	out   io.Writer
	mu    sync.Mutex
	quiet bool
}

// NewStatusProgressIndicator creates a progress indicator.
func NewStatusProgressIndicator(out io.Writer, _ int, quiet bool) *StatusProgressIndicator {
	return &StatusProgressIndicator{
		out:   out,
		quiet: quiet,
	}
}

// OnStart is called when a repository health check starts.
func (p *StatusProgressIndicator) OnStart(_ reposync.RepoSpec) {
	// No-op: progress is reported per-result in OnComplete.
}

// OnComplete is called when a repository health check completes.
func (p *StatusProgressIndicator) OnComplete(health reposync.RepoHealth) {
	if p.quiet {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

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

	name := health.Repo.Name
	if name == "" {
		name = health.Repo.TargetPath
	}
	fmt.Fprintf(p.out, "%s %s (%dms)\n",
		icon, name, health.Duration.Milliseconds())
}

// Done signals that all health checks are complete.
func (p *StatusProgressIndicator) Done() {
	// No cleanup needed — each OnComplete writes a full line with \n.
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
