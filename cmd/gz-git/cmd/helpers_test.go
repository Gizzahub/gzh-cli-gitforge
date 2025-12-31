package cmd

import "testing"

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
		{"success", "="},   // No changes count, so shows =
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
