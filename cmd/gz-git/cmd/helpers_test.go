package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func TestFormatUpstreamFixHint(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		remote string
		want   string
	}{
		{
			name:   "empty branch returns empty string",
			branch: "",
			remote: "origin",
			want:   "",
		},
		{
			name:   "master branch with origin",
			branch: "master",
			remote: "origin",
			want:   "    → Fix: git branch --set-upstream-to=origin/master master\n",
		},
		{
			name:   "main branch with empty remote defaults to origin",
			branch: "main",
			remote: "",
			want:   "    → Fix: git branch --set-upstream-to=origin/main main\n",
		},
		{
			name:   "feature branch with upstream remote",
			branch: "feature/my-feature",
			remote: "upstream",
			want:   "    → Fix: git branch --set-upstream-to=upstream/feature/my-feature feature/my-feature\n",
		},
		{
			name:   "develop branch with custom remote",
			branch: "develop",
			remote: "github",
			want:   "    → Fix: git branch --set-upstream-to=github/develop develop\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatUpstreamFixHint(tt.branch, tt.remote)
			if got != tt.want {
				t.Errorf("FormatUpstreamFixHint(%q, %q) = %q, want %q", tt.branch, tt.remote, got, tt.want)
			}
		})
	}
}

func TestGetBulkStatusIcon(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		changesCount int
		want         string
	}{
		// Clean state
		{"clean status", "clean", 0, "✓"},

		// Success states with changes
		{"success with changes", "success", 5, "✓"},
		{"fetched with changes", "fetched", 3, "✓"},
		{"pulled with changes", "pulled", 2, "✓"},
		{"pushed with changes", "pushed", 1, "✓"},
		{"updated with changes", "updated", 10, "✓"},

		// Success states without changes
		{"success no changes", "success", 0, "="},
		{"fetched no changes", "fetched", 0, "="},
		{"pulled no changes", "pulled", 0, "="},
		{"pushed no changes", "pushed", 0, "="},

		// Up-to-date states
		{"nothing-to-push", "nothing-to-push", 0, "="},
		{"up-to-date", "up-to-date", 0, "="},

		// Error states
		{"error", "error", 0, "✗"},

		// Conflict/in-progress states
		{"conflict", "conflict", 0, "⚡"},
		{"rebase-in-progress", "rebase-in-progress", 0, "↻"},
		{"merge-in-progress", "merge-in-progress", 0, "⇄"},
		{"dirty", "dirty", 0, "⚠"},

		// Skipped/dry-run states
		{"skipped", "skipped", 0, "⊘"},
		{"would-fetch", "would-fetch", 0, "→"},
		{"would-pull", "would-pull", 0, "→"},
		{"would-push", "would-push", 0, "→"},

		// Warning states
		{"no-remote", "no-remote", 0, "⚠"},
		{"no-upstream", "no-upstream", 0, "⚠"},

		// Unknown state
		{"unknown", "some-unknown-status", 0, "•"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBulkStatusIcon(tt.status, tt.changesCount)
			if got != tt.want {
				t.Errorf("getBulkStatusIcon(%q, %d) = %q, want %q", tt.status, tt.changesCount, got, tt.want)
			}
		})
	}
}

func TestGetBulkStatusIconSimple(t *testing.T) {
	// getBulkStatusIconSimple should behave like getBulkStatusIcon with changesCount=0
	tests := []struct {
		status string
		want   string
	}{
		{"clean", "✓"},
		{"success", "="}, // No changes count, so shows =
		{"up-to-date", "="},
		{"error", "✗"},
		{"conflict", "⚡"},
		{"dirty", "⚠"},
		{"skipped", "⊘"},
		{"would-fetch", "→"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := getBulkStatusIconSimple(tt.status)
			if got != tt.want {
				t.Errorf("getBulkStatusIconSimple(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestGetSummaryIcon(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		// Directional icons for operations
		{"up-to-date", "="},
		{"nothing-to-push", "="},
		{"fetched", "↓"},
		{"pulled", "↓"},
		{"updated", "↓"},
		{"pushed", "↑"},
		{"success", "✓"},

		// Dry-run
		{"would-fetch", "→"},
		{"would-pull", "→"},
		{"would-push", "→"},
		{"would-update", "→"},

		// Warning/error states
		{"skipped", "⊘"},
		{"dirty", "⚠"},
		{"error", "✗"},
		{"no-remote", "⚠"},
		{"no-upstream", "⚠"},
		{"auth-required", "🔐"},
		{"conflict", "⚡"},
		{"rebase-in-progress", "↻"},
		{"merge-in-progress", "⇄"},

		// Unknown
		{"some-unknown", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := getSummaryIcon(tt.status)
			if got != tt.want {
				t.Errorf("getSummaryIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestWriteSummaryLine(t *testing.T) {
	tests := []struct {
		name     string
		verb     string
		total    int
		summary  map[string]int
		duration time.Duration
		contains []string
	}{
		{
			name:     "fetch with mixed statuses",
			verb:     "Fetched",
			total:    6,
			summary:  map[string]int{"up-to-date": 4, "fetched": 2},
			duration: 1200 * time.Millisecond,
			contains: []string{"Fetched 6 repos", "=4 up-to-date", "↓2 fetched", "1.2s"},
		},
		{
			name:     "push all up-to-date",
			verb:     "Pushed",
			total:    3,
			summary:  map[string]int{"nothing-to-push": 3},
			duration: 500 * time.Millisecond,
			contains: []string{"Pushed 3 repos", "=3 nothing-to-push", "500ms"},
		},
		{
			name:     "with errors",
			verb:     "Pulled",
			total:    5,
			summary:  map[string]int{"pulled": 3, "error": 2},
			duration: 2 * time.Second,
			contains: []string{"Pulled 5 repos", "↓3 pulled", "✗2 error", "2s"},
		},
		{
			name:     "empty summary",
			verb:     "Updated",
			total:    0,
			summary:  map[string]int{},
			duration: 100 * time.Millisecond,
			contains: []string{"Updated 0 repos", "100ms"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			WriteSummaryLine(&buf, tt.verb, tt.total, tt.summary, tt.duration)
			got := buf.String()
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("WriteSummaryLine() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestWriteHealthSummaryLine(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		summary  reposync.HealthSummary
		duration time.Duration
		contains []string
	}{
		{
			name:  "all healthy",
			total: 6,
			summary: reposync.HealthSummary{
				Healthy: 6, Total: 6,
			},
			duration: 1500 * time.Millisecond,
			contains: []string{"Status 6 repos", "✓6 healthy", "1.5s"},
		},
		{
			name:  "mixed health",
			total: 8,
			summary: reposync.HealthSummary{
				Healthy: 5, Warning: 2, Error: 1, Total: 8,
			},
			duration: 3 * time.Second,
			contains: []string{"Status 8 repos", "✓5 healthy", "⚠2 warning", "✗1 error", "3s"},
		},
		{
			name:  "with unreachable",
			total: 4,
			summary: reposync.HealthSummary{
				Healthy: 2, Unreachable: 2, Total: 4,
			},
			duration: 30 * time.Second,
			contains: []string{"Status 4 repos", "✓2 healthy", "⊘2 unreachable"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			WriteHealthSummaryLine(&buf, tt.total, tt.summary, tt.duration)
			got := buf.String()
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("WriteHealthSummaryLine() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}
