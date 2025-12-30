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

func TestGetStatusIconForStatus(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"clean", "✓"},
		{"dirty", "⚠"},
		{"conflict", "⚡"},
		{"rebase-in-progress", "↻"},
		{"merge-in-progress", "⇄"},
		{"error", "✗"},
		{"no-remote", "⚠"},
		{"no-upstream", "⚠"},
		{"unknown", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := getStatusIconForStatus(tt.status)
			if got != tt.want {
				t.Errorf("getStatusIconForStatus(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
