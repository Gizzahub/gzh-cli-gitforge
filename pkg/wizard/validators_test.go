// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package wizard

import (
	"os"
	"testing"
)

func TestValidateProvider(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"github is valid", "github", false},
		{"gitlab is valid", "gitlab", false},
		{"gitea is valid", "gitea", false},
		{"invalid provider", "bitbucket", true},
		{"uppercase invalid", "GitHub", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProvider(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProvider(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateProviderRequired(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is invalid", "", true},
		{"github is valid", "github", false},
		{"invalid provider", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProviderRequired(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProviderRequired(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCloneProto(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"ssh is valid", "ssh", false},
		{"https is valid", "https", false},
		{"invalid protocol", "git", true},
		{"uppercase invalid", "SSH", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCloneProto(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCloneProto(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"valid port 22", "22", false},
		{"valid port 443", "443", false},
		{"valid port 2224", "2224", false},
		{"valid port 0", "0", false},
		{"valid port 65535", "65535", false},
		{"port too high", "65536", true},
		{"negative port", "-1", true},
		{"non-numeric", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePort(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"valid https URL", "https://gitlab.com", false},
		{"valid http URL", "http://localhost:8080", false},
		{"valid URL with path", "https://gitlab.company.com/api/v4", false},
		{"env var reference", "${GITLAB_URL}", false},
		{"missing scheme", "gitlab.com", true},
		{"invalid scheme", "git://github.com", true},
		{"missing host", "https://", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"temp dir exists", tmpDir, false},
		{"new subdir in temp", tmpDir + "/new", false},
		{"deeply nested new dir", tmpDir + "/a/b/c", true}, // parent doesn't exist
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePathWithHome(t *testing.T) {
	// This test checks home directory expansion
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home directory")
	}

	// Home directory should exist
	err = ValidatePath("~/")
	if err != nil {
		t.Errorf("ValidatePath(~/) should be valid, home=%s, err=%v", home, err)
	}
}

func TestValidateNotEmpty(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is invalid", "", true},
		{"whitespace only is invalid", "   ", true},
		{"valid string", "test", false},
		{"string with spaces", "test value", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNotEmpty(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNotEmpty(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateOrganization(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is invalid", "", true},
		{"simple name", "myorg", false},
		{"name with dash", "my-org", false},
		{"name with underscore", "my_org", false},
		{"name with slash", "parent/child", false},
		{"name with numbers", "org123", false},
		{"name with space", "my org", true},
		{"name with special char", "org@test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOrganization(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOrganization(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateParallel(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"valid 1", "1", false},
		{"valid 5", "5", false},
		{"valid 50", "50", false},
		{"zero is invalid", "0", true},
		{"too high", "51", true},
		{"non-numeric", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParallel(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateParallel(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSubgroupMode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"flat is valid", "flat", false},
		{"nested is valid", "nested", false},
		{"invalid mode", "recursive", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSubgroupMode(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSubgroupMode(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"env var reference", "${GITHUB_TOKEN}", false},
		{"env var with underscore", "${MY_API_TOKEN}", false},
		{"long token", "ghp_1234567890abcdef", false},
		{"short token", "short", true},
		{"empty env var name", "${}", true},
		{"env var starting with number", "${1TOKEN}", true},
		{"env var with lowercase", "${my_token}", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToken(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateToken(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateProfileName(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty is invalid", "", true},
		{"simple name", "work", false},
		{"name with dash", "my-profile", false},
		{"name with underscore", "my_profile", false},
		{"name with numbers", "work123", false},
		{"name with space", "my profile", true},
		{"name with dot", "my.profile", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfileName(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfileName(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{"empty returns 0", "", 0},
		{"valid port", "22", 22},
		{"custom port", "2224", 2224},
		{"invalid returns 0", "abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePort(tt.value)
			if got != tt.want {
				t.Errorf("ParsePort(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseParallel(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		defaultVal int
		want       int
	}{
		{"empty returns default", "", 5, 5},
		{"valid value", "10", 5, 10},
		{"invalid returns default", "abc", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseParallel(tt.value, tt.defaultVal)
			if got != tt.want {
				t.Errorf("ParseParallel(%q, %d) = %d, want %d", tt.value, tt.defaultVal, got, tt.want)
			}
		})
	}
}
