// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// FormatHealthIcon returns an icon representing the health status.
func FormatHealthIcon(status reposync.HealthStatus) string {
	switch status {
	case reposync.HealthHealthy:
		return "✓"
	case reposync.HealthWarning:
		return "⚠"
	case reposync.HealthError:
		return "✗"
	case reposync.HealthUnreachable:
		return "⊘"
	default:
		return "?"
	}
}

// FormatRepoName extracts a display name from a RepoSpec.
func FormatRepoName(repo reposync.RepoSpec) string {
	if repo.Name != "" {
		return repo.Name
	}
	return repo.TargetPath
}

// FormatStatusText returns a textual description of repository status.
func FormatStatusText(health reposync.RepoHealth) string {
	var parts []string

	// Health status
	parts = append(parts, string(health.HealthStatus))

	// Divergence info
	switch health.DivergenceType {
	case reposync.DivergenceNone:
		parts = append(parts, "up-to-date")
	case reposync.DivergenceFastForward:
		parts = append(parts, fmt.Sprintf("%d↓ behind", health.BehindBy))
	case reposync.DivergenceDiverged:
		parts = append(parts, fmt.Sprintf("%d↑ %d↓ diverged", health.AheadBy, health.BehindBy))
	case reposync.DivergenceAhead:
		parts = append(parts, fmt.Sprintf("%d↑ ahead", health.AheadBy))
	case reposync.DivergenceConflict:
		parts = append(parts, "conflict")
	case reposync.DivergenceNoUpstream:
		parts = append(parts, "no-upstream")
	}

	// Working tree status
	if health.WorkTreeStatus == reposync.WorkTreeDirty {
		parts = append(parts, "dirty")
	} else if health.WorkTreeStatus == reposync.WorkTreeConflict {
		parts = append(parts, "conflict")
	}

	result := parts[0]
	if len(parts) > 1 {
		result += "    " + parts[1]
		for i := 2; i < len(parts); i++ {
			result += " + " + parts[i]
		}
	}

	return result
}
