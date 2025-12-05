package commit

import (
	"testing"
)

func TestSmartPush_New(t *testing.T) {
	sp := NewSmartPush()
	if sp == nil {
		t.Fatal("NewSmartPush() returned nil")
	}
}

func TestPushCheck_FormatPushCheck(t *testing.T) {
	tests := []struct {
		name  string
		check *PushCheck
		want  string
	}{
		{
			name:  "nil check",
			check: nil,
			want:  "",
		},
		{
			name: "safe to push",
			check: &PushCheck{
				Safe: true,
				Issues: []PushIssue{
					{Severity: "info", Message: "up-to-date"},
				},
			},
			want: "✓ Safe to push\n\nInfo:\n  ℹ up-to-date\n",
		},
		{
			name: "blocked with errors and recommendations",
			check: &PushCheck{
				Safe: false,
				Issues: []PushIssue{
					{Severity: "error", Message: "uncommitted changes", Blocker: true},
				},
				Recommendations: []string{"commit changes first"},
			},
			want: "✗ Push blocked\n\nErrors:\n  ✗ uncommitted changes\n\nRecommendations:\n  → commit changes first\n",
		},
		{
			name: "mixed issues",
			check: &PushCheck{
				Safe: true,
				Issues: []PushIssue{
					{Severity: "warning", Message: "no upstream"},
					{Severity: "info", Message: "2 commits ahead"},
				},
				Recommendations: []string{"set upstream branch"},
			},
			want: "✓ Safe to push\n\nWarnings:\n  ⚠ no upstream\n\nInfo:\n  ℹ 2 commits ahead\n\nRecommendations:\n  → set upstream branch\n",
		},
		{
			name: "all severity types",
			check: &PushCheck{
				Safe: false,
				Issues: []PushIssue{
					{Severity: "error", Message: "error 1", Blocker: true},
					{Severity: "error", Message: "error 2", Blocker: false},
					{Severity: "warning", Message: "warning 1"},
					{Severity: "warning", Message: "warning 2"},
					{Severity: "info", Message: "info 1"},
				},
				Recommendations: []string{"fix error 1", "fix error 2"},
			},
			want: "✗ Push blocked\n\nErrors:\n  ✗ error 1\n  ✗ error 2\n\nWarnings:\n  ⚠ warning 1\n  ⚠ warning 2\n\nInfo:\n  ℹ info 1\n\nRecommendations:\n  → fix error 1\n  → fix error 2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPushCheck(tt.check)
			if got != tt.want {
				t.Errorf("FormatPushCheck() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPushOptions_Defaults(t *testing.T) {
	opts := PushOptions{}

	if opts.Remote != "" {
		t.Errorf("default Remote should be empty, got %q", opts.Remote)
	}
	if opts.Branch != "" {
		t.Errorf("default Branch should be empty, got %q", opts.Branch)
	}
	if opts.Force {
		t.Error("default Force should be false")
	}
	if opts.SetUpstream {
		t.Error("default SetUpstream should be false")
	}
	if opts.DryRun {
		t.Error("default DryRun should be false")
	}
	if opts.SkipChecks {
		t.Error("default SkipChecks should be false")
	}
}

func TestProtectedBranches(t *testing.T) {
	tests := []struct {
		branch      string
		isProtected bool
	}{
		{"main", true},
		{"master", true},
		{"develop", true},
		{"release", true},
		{"feature-branch", false},
		{"bugfix", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got := protectedBranches[tt.branch]
			if got != tt.isProtected {
				t.Errorf("branch %q protected = %v, want %v", tt.branch, got, tt.isProtected)
			}
		})
	}
}

func TestPushIssue_Struct(t *testing.T) {
	issue := PushIssue{
		Severity: "error",
		Message:  "test error",
		Blocker:  true,
	}

	if issue.Severity != "error" {
		t.Errorf("Severity = %q, want %q", issue.Severity, "error")
	}
	if issue.Message != "test error" {
		t.Errorf("Message = %q, want %q", issue.Message, "test error")
	}
	if !issue.Blocker {
		t.Error("Blocker should be true")
	}
}

func TestPushCheck_Struct(t *testing.T) {
	check := &PushCheck{
		Safe: true,
		Issues: []PushIssue{
			{Severity: "info", Message: "test"},
		},
		Recommendations: []string{"recommendation 1"},
	}

	if !check.Safe {
		t.Error("Safe should be true")
	}
	if len(check.Issues) != 1 {
		t.Errorf("Issues length = %d, want 1", len(check.Issues))
	}
	if len(check.Recommendations) != 1 {
		t.Errorf("Recommendations length = %d, want 1", len(check.Recommendations))
	}
}
