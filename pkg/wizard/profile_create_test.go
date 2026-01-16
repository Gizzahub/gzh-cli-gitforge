// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package wizard

import (
	"testing"
)

func TestNewProfileCreateWizard(t *testing.T) {
	tests := []struct {
		name        string
		profileName string
	}{
		{"simple name", "work"},
		{"with dash", "my-profile"},
		{"with underscore", "my_profile"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewProfileCreateWizard(tt.profileName)

			if w == nil {
				t.Fatal("NewProfileCreateWizard returned nil")
			}

			if w.profileName != tt.profileName {
				t.Errorf("profileName = %q, want %q", w.profileName, tt.profileName)
			}

			if w.profile == nil {
				t.Fatal("profile is nil")
			}

			if w.profile.Name != tt.profileName {
				t.Errorf("profile.Name = %q, want %q", w.profile.Name, tt.profileName)
			}

			// Check defaults
			if w.profile.CloneProto != "ssh" {
				t.Errorf("default CloneProto = %q, want 'ssh'", w.profile.CloneProto)
			}

			if w.profile.Parallel != 5 {
				t.Errorf("default Parallel = %d, want 5", w.profile.Parallel)
			}
		})
	}
}

func TestProfileCreateWizard_ProfileDefaults(t *testing.T) {
	w := NewProfileCreateWizard("test")

	// Verify all expected defaults
	if w.profile.CloneProto != "ssh" {
		t.Errorf("CloneProto default = %q, want 'ssh'", w.profile.CloneProto)
	}

	if w.profile.Parallel != 5 {
		t.Errorf("Parallel default = %d, want 5", w.profile.Parallel)
	}

	// These should be zero/empty by default
	if w.profile.Provider != "" {
		t.Errorf("Provider should be empty by default, got %q", w.profile.Provider)
	}

	if w.profile.BaseURL != "" {
		t.Errorf("BaseURL should be empty by default, got %q", w.profile.BaseURL)
	}

	if w.profile.Token != "" {
		t.Errorf("Token should be empty by default, got %q", w.profile.Token)
	}

	if w.profile.SSHPort != 0 {
		t.Errorf("SSHPort should be 0 by default, got %d", w.profile.SSHPort)
	}

	if w.profile.IncludeSubgroups {
		t.Error("IncludeSubgroups should be false by default")
	}

	if w.profile.SubgroupMode != "" {
		t.Errorf("SubgroupMode should be empty by default, got %q", w.profile.SubgroupMode)
	}
}
