// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package wizard

import (
	"strings"
	"testing"
)

func TestNewSyncSetupWizard(t *testing.T) {
	w := NewSyncSetupWizard()

	if w == nil {
		t.Fatal("NewSyncSetupWizard returned nil")
	}

	// Check defaults
	if w.opts.CloneProto != "ssh" {
		t.Errorf("default CloneProto = %q, want 'ssh'", w.opts.CloneProto)
	}

	if w.opts.SubgroupMode != "flat" {
		t.Errorf("default SubgroupMode = %q, want 'flat'", w.opts.SubgroupMode)
	}

	if !w.opts.IncludePrivate {
		t.Error("default IncludePrivate should be true")
	}

	if w.opts.Parallel != 10 {
		t.Errorf("default Parallel = %d, want 10", w.opts.Parallel)
	}
}

func TestSyncSetupWizard_BuildCommand(t *testing.T) {
	tests := []struct {
		name     string
		opts     SyncSetupOptions
		wantPart []string // Parts that should be in the command
		dontWant []string // Parts that should NOT be in the command
	}{
		{
			name: "basic github",
			opts: SyncSetupOptions{
				Provider:       "github",
				Organization:   "myorg",
				TargetPath:     "/tmp/repos",
				CloneProto:     "ssh",
				IncludePrivate: true,
				Parallel:       10,
			},
			wantPart: []string{
				"gz-git forge from",
				"--provider github",
				"--org myorg",
				"--path /tmp/repos",
			},
			dontWant: []string{
				"--clone-proto", // ssh is default
				"--parallel",    // 10 is default
				"--include-subgroups",
			},
		},
		{
			name: "gitlab with subgroups",
			opts: SyncSetupOptions{
				Provider:         "gitlab",
				Organization:     "parent/child",
				TargetPath:       "~/repos",
				BaseURL:          "https://gitlab.company.com",
				Token:            "${GITLAB_TOKEN}",
				CloneProto:       "ssh",
				SSHPort:          2224,
				IncludeSubgroups: true,
				SubgroupMode:     "nested",
				IncludePrivate:   true,
				Parallel:         10,
			},
			wantPart: []string{
				"--provider gitlab",
				"--org parent/child",
				"--base-url https://gitlab.company.com",
				"--token ${GITLAB_TOKEN}",
				"--ssh-port 2224",
				"--include-subgroups",
				"--subgroup-mode nested",
				// --parallel 10 is default, so not included
			},
		},
		{
			name: "https clone protocol",
			opts: SyncSetupOptions{
				Provider:       "github",
				Organization:   "myorg",
				TargetPath:     "/tmp",
				CloneProto:     "https",
				IncludePrivate: true,
				Parallel:       10,
			},
			wantPart: []string{
				"--clone-proto https",
			},
		},
		{
			name: "include archived and forks",
			opts: SyncSetupOptions{
				Provider:        "github",
				Organization:    "myorg",
				TargetPath:      "/tmp",
				CloneProto:      "ssh",
				IncludeArchived: true,
				IncludeForks:    true,
				IncludePrivate:  true,
				Parallel:        10,
			},
			wantPart: []string{
				"--include-archived",
				"--include-forks",
			},
		},
		{
			name: "exclude private",
			opts: SyncSetupOptions{
				Provider:       "github",
				Organization:   "myorg",
				TargetPath:     "/tmp",
				CloneProto:     "ssh",
				IncludePrivate: false,
				Parallel:       10,
			},
			wantPart: []string{
				"--include-private=false",
			},
		},
		{
			name: "plain token masked",
			opts: SyncSetupOptions{
				Provider:       "github",
				Organization:   "myorg",
				TargetPath:     "/tmp",
				Token:          "ghp_1234567890abcdef",
				CloneProto:     "ssh",
				IncludePrivate: true,
				Parallel:       10,
			},
			wantPart: []string{
				"--token $TOKEN", // Plain token should be masked
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &SyncSetupWizard{opts: tt.opts}
			cmd := w.BuildCommand()

			for _, part := range tt.wantPart {
				if !strings.Contains(cmd, part) {
					t.Errorf("BuildCommand() missing %q\nGot: %s", part, cmd)
				}
			}

			for _, part := range tt.dontWant {
				if strings.Contains(cmd, part) {
					t.Errorf("BuildCommand() should not contain %q\nGot: %s", part, cmd)
				}
			}
		})
	}
}

func TestSanitizeTokenForDisplay(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{"empty", "", "(not set)"},
		{"env var", "${GITHUB_TOKEN}", "${GITHUB_TOKEN}"},
		{"short token", "abc", "****"},
		{"long token", "ghp_1234567890abcdef", "ghp_...cdef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeTokenForDisplay(tt.token)
			if got != tt.want {
				t.Errorf("SanitizeTokenForDisplay(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestFormatBool(t *testing.T) {
	if FormatBool(true) != "yes" {
		t.Error("FormatBool(true) should return 'yes'")
	}
	if FormatBool(false) != "no" {
		t.Error("FormatBool(false) should return 'no'")
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		value      int
		defaultVal int
		want       string
	}{
		{0, 5, "5 (default)"},
		{5, 5, "5 (default)"},
		{10, 5, "10"},
		{22, 22, "22 (default)"},
		{2224, 22, "2224"},
	}

	for _, tt := range tests {
		got := FormatInt(tt.value, tt.defaultVal)
		if got != tt.want {
			t.Errorf("FormatInt(%d, %d) = %q, want %q", tt.value, tt.defaultVal, got, tt.want)
		}
	}
}
