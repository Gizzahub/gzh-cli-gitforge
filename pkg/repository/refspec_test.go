// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"
)

func TestValidateRefspec(t *testing.T) {
	tests := []struct {
		name        string
		refspec     string
		wantErr     bool
		wantSource  string
		wantDest    string
		wantForce   bool
		errContains string
	}{
		// Valid cases
		{
			name:       "simple branch",
			refspec:    "main",
			wantErr:    false,
			wantSource: "main",
			wantDest:   "",
			wantForce:  false,
		},
		{
			name:       "branch mapping",
			refspec:    "develop:master",
			wantErr:    false,
			wantSource: "develop",
			wantDest:   "master",
			wantForce:  false,
		},
		{
			name:       "force push",
			refspec:    "+develop:master",
			wantErr:    false,
			wantSource: "develop",
			wantDest:   "master",
			wantForce:  true,
		},
		{
			name:       "full ref path",
			refspec:    "refs/heads/main:refs/heads/master",
			wantErr:    false,
			wantSource: "refs/heads/main",
			wantDest:   "refs/heads/master",
			wantForce:  false,
		},
		{
			name:       "branch with slashes",
			refspec:    "feature/xyz-123:release/v1.0",
			wantErr:    false,
			wantSource: "feature/xyz-123",
			wantDest:   "release/v1.0",
			wantForce:  false,
		},
		{
			name:       "branch with underscores and dots",
			refspec:    "feat_v1.2.3:main_v1.2.3",
			wantErr:    false,
			wantSource: "feat_v1.2.3",
			wantDest:   "main_v1.2.3",
			wantForce:  false,
		},

		// Invalid cases - empty/malformed
		{
			name:        "empty refspec",
			refspec:     "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "double colon",
			refspec:     "develop::master",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:        "empty source",
			refspec:     ":master",
			wantErr:     true,
			errContains: "source (left side) cannot be empty",
		},
		{
			name:        "empty destination",
			refspec:     "develop:",
			wantErr:     true,
			errContains: "destination (right side) cannot be empty",
		},

		// Invalid cases - branch name rules
		{
			name:        "starts with dash",
			refspec:     "-invalid",
			wantErr:     true,
			errContains: "cannot start with -",
		},
		{
			name:        "starts with dot",
			refspec:     ".invalid",
			wantErr:     true,
			errContains: "cannot start with",
		},
		{
			name:        "ends with dot",
			refspec:     "invalid.",
			wantErr:     true,
			errContains: "cannot end with",
		},
		{
			name:        "ends with .lock",
			refspec:     "branch.lock",
			wantErr:     true,
			errContains: "cannot end with",
		},
		{
			name:        "consecutive dots",
			refspec:     "branch..name",
			wantErr:     true,
			errContains: "invalid pattern",
		},
		{
			name:        "consecutive slashes",
			refspec:     "branch//name",
			wantErr:     true,
			errContains: "invalid pattern",
		},
		{
			name:        "contains space",
			refspec:     "branch name",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:        "contains tilde",
			refspec:     "branch~name",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:        "contains caret",
			refspec:     "branch^name",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:        "contains question mark",
			refspec:     "branch?name",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:        "contains asterisk",
			refspec:     "branch*name",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:        "contains bracket",
			refspec:     "branch[name",
			wantErr:     true,
			errContains: "invalid character",
		},

		// Edge cases
		{
			name:       "single character branch",
			refspec:    "a",
			wantErr:    false,
			wantSource: "a",
			wantDest:   "",
		},
		{
			name:       "numeric branch",
			refspec:    "123",
			wantErr:    false,
			wantSource: "123",
			wantDest:   "",
		},
		{
			name:       "alphanumeric mix",
			refspec:    "v1.2.3-rc.1:v1.2.3",
			wantErr:    false,
			wantSource: "v1.2.3-rc.1",
			wantDest:   "v1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ValidateRefspec(tt.refspec)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateRefspec(%q) expected error, got nil", tt.refspec)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateRefspec(%q) error = %v, want error containing %q", tt.refspec, err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateRefspec(%q) unexpected error: %v", tt.refspec, err)
				return
			}

			if parsed.Source != tt.wantSource {
				t.Errorf("ValidateRefspec(%q) Source = %q, want %q", tt.refspec, parsed.Source, tt.wantSource)
			}

			if parsed.Destination != tt.wantDest {
				t.Errorf("ValidateRefspec(%q) Destination = %q, want %q", tt.refspec, parsed.Destination, tt.wantDest)
			}

			if parsed.Force != tt.wantForce {
				t.Errorf("ValidateRefspec(%q) Force = %v, want %v", tt.refspec, parsed.Force, tt.wantForce)
			}
		})
	}
}

func TestParsedRefspec_GetSourceBranch(t *testing.T) {
	tests := []struct {
		name       string
		refspec    string
		wantBranch string
	}{
		{
			name:       "simple branch",
			refspec:    "main",
			wantBranch: "main",
		},
		{
			name:       "full ref path",
			refspec:    "refs/heads/develop",
			wantBranch: "develop",
		},
		{
			name:       "branch mapping",
			refspec:    "feature/xyz:main",
			wantBranch: "feature/xyz",
		},
		{
			name:       "full ref mapping",
			refspec:    "refs/heads/feature:refs/heads/main",
			wantBranch: "feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ValidateRefspec(tt.refspec)
			if err != nil {
				t.Fatalf("ValidateRefspec(%q) error: %v", tt.refspec, err)
			}

			got := parsed.GetSourceBranch()
			if got != tt.wantBranch {
				t.Errorf("GetSourceBranch() = %q, want %q", got, tt.wantBranch)
			}
		})
	}
}

func TestParsedRefspec_GetDestinationBranch(t *testing.T) {
	tests := []struct {
		name       string
		refspec    string
		wantBranch string
	}{
		{
			name:       "simple branch (no destination)",
			refspec:    "main",
			wantBranch: "main",
		},
		{
			name:       "branch mapping",
			refspec:    "develop:master",
			wantBranch: "master",
		},
		{
			name:       "full ref mapping",
			refspec:    "refs/heads/develop:refs/heads/master",
			wantBranch: "master",
		},
		{
			name:       "feature to main",
			refspec:    "feature/xyz:main",
			wantBranch: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ValidateRefspec(tt.refspec)
			if err != nil {
				t.Fatalf("ValidateRefspec(%q) error: %v", tt.refspec, err)
			}

			got := parsed.GetDestinationBranch()
			if got != tt.wantBranch {
				t.Errorf("GetDestinationBranch() = %q, want %q", got, tt.wantBranch)
			}
		})
	}
}

func TestParsedRefspec_String(t *testing.T) {
	tests := []struct {
		name    string
		refspec string
		want    string
	}{
		{
			name:    "simple branch",
			refspec: "main",
			want:    "main",
		},
		{
			name:    "branch mapping",
			refspec: "develop:master",
			want:    "develop:master",
		},
		{
			name:    "force push",
			refspec: "+develop:master",
			want:    "+develop:master",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ValidateRefspec(tt.refspec)
			if err != nil {
				t.Fatalf("ValidateRefspec(%q) error: %v", tt.refspec, err)
			}

			got := parsed.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name        string
		branchName  string
		wantErr     bool
		errContains string
	}{
		// Valid cases
		{name: "simple", branchName: "main", wantErr: false},
		{name: "with dash", branchName: "feature-xyz", wantErr: false},
		{name: "with slash", branchName: "feature/xyz", wantErr: false},
		{name: "with dot", branchName: "v1.2.3", wantErr: false},
		{name: "with underscore", branchName: "my_branch", wantErr: false},
		{name: "numeric", branchName: "123", wantErr: false},
		{name: "long name", branchName: "feature/very-long-branch-name-with-many-parts", wantErr: false},

		// Invalid cases
		{name: "empty", branchName: "", wantErr: true, errContains: "cannot be empty"},
		{name: "starts with dash", branchName: "-invalid", wantErr: true, errContains: "cannot start with -"},
		{name: "starts with dot", branchName: ".invalid", wantErr: true, errContains: "cannot start with"},
		{name: "starts with slash", branchName: "/invalid", wantErr: true, errContains: "cannot start with"},
		{name: "ends with dot", branchName: "invalid.", wantErr: true, errContains: "cannot end with"},
		{name: "ends with slash", branchName: "invalid/", wantErr: true, errContains: "cannot end with"},
		{name: "ends with .lock", branchName: "branch.lock", wantErr: true, errContains: "cannot end with"},
		{name: "consecutive dots", branchName: "branch..name", wantErr: true, errContains: "invalid pattern"},
		{name: "consecutive slashes", branchName: "branch//name", wantErr: true, errContains: "invalid pattern"},
		{name: "contains space", branchName: "branch name", wantErr: true, errContains: "invalid character"},
		{name: "contains @{", branchName: "branch@{name", wantErr: true, errContains: "invalid pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBranchName(tt.branchName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateBranchName(%q) expected error, got nil", tt.branchName)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("validateBranchName(%q) error = %v, want error containing %q", tt.branchName, err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("validateBranchName(%q) unexpected error: %v", tt.branchName, err)
			}
		})
	}
}

// contains is a helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
