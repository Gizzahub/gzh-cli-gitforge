package cmd

import "testing"

func TestFormatUpstreamFixHint(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{
			name:   "empty branch returns empty string",
			branch: "",
			want:   "",
		},
		{
			name:   "master branch",
			branch: "master",
			want:   "    → Fix: git branch --set-upstream-to=origin/master master\n",
		},
		{
			name:   "main branch",
			branch: "main",
			want:   "    → Fix: git branch --set-upstream-to=origin/main main\n",
		},
		{
			name:   "feature branch with slash",
			branch: "feature/my-feature",
			want:   "    → Fix: git branch --set-upstream-to=origin/feature/my-feature feature/my-feature\n",
		},
		{
			name:   "develop branch",
			branch: "develop",
			want:   "    → Fix: git branch --set-upstream-to=origin/develop develop\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatUpstreamFixHint(tt.branch)
			if got != tt.want {
				t.Errorf("FormatUpstreamFixHint(%q) = %q, want %q", tt.branch, got, tt.want)
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
