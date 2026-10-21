// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// setProfileOverride points --profile at name and restores the previous value
// when the test ends.
func setProfileOverride(t *testing.T, name string) {
	t.Helper()
	prev := profileOverride
	t.Cleanup(func() { profileOverride = prev })
	profileOverride = name
}

// writeProfileFile writes a profile YAML into the isolated config dir's
// profiles directory, where ProfilePath looks it up by name.
func writeProfileFile(t *testing.T, name, content string) {
	t.Helper()
	configHome, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	profilesDir := filepath.Join(configHome, config.ConfigDirName, config.ProfilesDirName)
	if err := os.MkdirAll(profilesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, profilesDir, name+".yaml", content)
}

// TestLoadEffectiveConfigFailsOnAMissingProfileOverride asserts a --profile
// naming a profile that does not exist is an error, not a silent fallback to
// the active profile.
func TestLoadEffectiveConfigFailsOnAMissingProfileOverride(t *testing.T) {
	isolateConfigHome(t)
	setProfileOverride(t, "nope")

	_, err := LoadEffectiveConfig(rootCmd, nil)
	if err == nil {
		t.Fatal("expected an error for a --profile that does not exist")
	}
	if !strings.Contains(err.Error(), "profile 'nope' not found") {
		t.Errorf("error %q does not name the missing profile", err)
	}
}

// TestLoadEffectiveConfigFailsOnARemovedKeyInTheProfileOverride asserts a
// profile file still declaring a removed key fails the load with the key named.
func TestLoadEffectiveConfigFailsOnARemovedKeyInTheProfileOverride(t *testing.T) {
	isolateConfigHome(t)
	setProfileOverride(t, "stale")

	writeProfileFile(t, "stale", "name: stale\nintegration:\n  readinessApproval: automatic\n")

	_, err := LoadEffectiveConfig(rootCmd, nil)
	if err == nil {
		t.Fatal("expected an error for a removed key in the --profile file")
	}
	if !strings.Contains(err.Error(), "integration.readinessApproval") {
		t.Errorf("error %q does not name the removed key", err)
	}
}

// TestLoadEffectiveConfigAppliesAValidProfileOverride asserts a valid --profile
// still applies: its provider and baseURL reach the effective config.
func TestLoadEffectiveConfigAppliesAValidProfileOverride(t *testing.T) {
	isolateConfigHome(t)
	setProfileOverride(t, "work")

	writeProfileFile(t, "work", "name: work\nprovider: gitlab\nbaseURL: https://gitlab.example.com\n")

	effective, err := LoadEffectiveConfig(rootCmd, nil)
	if err != nil {
		t.Fatalf("LoadEffectiveConfig: %v", err)
	}
	if effective.Provider != "gitlab" {
		t.Errorf("provider = %q, want gitlab from the --profile override", effective.Provider)
	}
	if effective.BaseURL != "https://gitlab.example.com" {
		t.Errorf("baseURL = %q, want the profile's value", effective.BaseURL)
	}
}
