// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPrecedence(t *testing.T) {
	// Create temp config directory
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	// Setup: Create config directory structure
	configDir := filepath.Join(tmpDir, ".config", "gz-git")
	profilesDir := filepath.Join(configDir, "profiles")
	stateDir := filepath.Join(configDir, "state")

	os.MkdirAll(profilesDir, 0700)
	os.MkdirAll(stateDir, 0700)

	// Create global config
	globalConfig := &GlobalConfig{
		ActiveProfile: "default",
		Defaults: map[string]interface{}{
			"parallel":   10, // Global default
			"cloneProto": "https",
		},
	}
	mgr, _ := NewManager()
	mgr.SaveGlobalConfig(globalConfig)

	// Create default profile
	defaultProfile := &Profile{
		Name:     "default",
		Provider: "github",   // From profile
		Parallel: 10,          // Profile overrides global
		Token:    "prof-tok", // From profile
	}
	mgr.SaveProfile(defaultProfile)

	// Set active profile
	paths, _ := NewPaths()
	paths.SetActiveProfile("default")

	// Load config
	loader, err := NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Test 1: Profile overrides global
	effective, err := loader.ResolveConfig(nil)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if effective.Parallel != 10 {
		t.Errorf("Parallel = %d, want 10 (from profile, not global)", effective.Parallel)
	}
	if effective.GetSource("parallel") != "profile:default" {
		t.Errorf("Source = %s, want profile:default", effective.GetSource("parallel"))
	}

	// Test 2: Flags override profile
	flags := map[string]interface{}{
		"parallel": 20,
		"provider": "gitlab",
	}
	effective, err = loader.ResolveConfig(flags)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if effective.Parallel != 20 {
		t.Errorf("Parallel = %d, want 20 (from flags)", effective.Parallel)
	}
	if effective.GetSource("parallel") != "flag" {
		t.Errorf("Source = %s, want flag", effective.GetSource("parallel"))
	}

	if effective.Provider != "gitlab" {
		t.Errorf("Provider = %s, want gitlab (from flags)", effective.Provider)
	}

	// Test 3: Unset values use defaults
	if effective.CloneProto != "https" {
		t.Errorf("CloneProto = %s, want https (from global)", effective.CloneProto)
	}
}

func TestProjectConfigPrecedence(t *testing.T) {
	// Create temp directories
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	// Create project directory
	projectDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(projectDir, 0755)

	// Change to project directory
	oldWd, _ := os.Getwd()
	os.Chdir(projectDir)
	defer os.Chdir(oldWd)

	// Setup global config
	configDir := filepath.Join(tmpDir, ".config", "gz-git")
	profilesDir := filepath.Join(configDir, "profiles")
	os.MkdirAll(profilesDir, 0700)

	mgr, _ := NewManager()

	// Create profile
	profile := &Profile{
		Name:     "work",
		Parallel: 10,
		Sync: &SyncConfig{
			Strategy: "reset",
		},
	}
	mgr.SaveProfile(profile)

	paths, _ := NewPaths()
	paths.SetActiveProfile("work")

	// Create project config that overrides profile
	projectConfig := &ProjectConfig{
		Profile: "work",
		Sync: &SyncConfig{
			Strategy: "pull", // Override profile's reset
		},
	}
	mgr.SaveProjectConfig(projectConfig)

	// Load and resolve
	loader, err := NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	effective, err := loader.ResolveConfig(nil)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	// Project config should override profile
	if effective.Sync.Strategy != "pull" {
		t.Errorf("Sync.Strategy = %s, want pull (from project)", effective.Sync.Strategy)
	}

	// Profile value should still apply for non-overridden values
	if effective.Parallel != 10 {
		t.Errorf("Parallel = %d, want 10 (from profile)", effective.Parallel)
	}
}

func TestEffectiveConfigGetters(t *testing.T) {
	cfg := &EffectiveConfig{
		Provider: "gitlab",
		Parallel: 10,
		Sources: map[string]string{
			"provider": "profile:work",
			"parallel": "flag",
		},
	}

	// Test GetString
	val, ok := cfg.GetString("Provider")
	if !ok || val != "gitlab" {
		t.Errorf("GetString(Provider) = %v, %v, want gitlab, true", val, ok)
	}

	_, ok = cfg.GetString("NonExistent")
	if ok {
		t.Errorf("GetString(NonExistent) should return false")
	}

	// Test GetInt
	intVal, ok := cfg.GetInt("Parallel")
	if !ok || intVal != 10 {
		t.Errorf("GetInt(Parallel) = %v, %v, want 10, true", intVal, ok)
	}

	// Test GetSource
	src := cfg.GetSource("provider")
	if src != "profile:work" {
		t.Errorf("GetSource(provider) = %s, want profile:work", src)
	}

	src = cfg.GetSource("unknown")
	if src != "default" {
		t.Errorf("GetSource(unknown) = %s, want default", src)
	}
}

func TestApplyDefaults(t *testing.T) {
	loader := &ConfigLoader{}
	cfg := &EffectiveConfig{
		Sources: make(map[string]string),
	}

	loader.applyDefaults(cfg)

	// Check default values
	if cfg.Parallel != 10 {
		t.Errorf("Default Parallel = %d, want 10", cfg.Parallel)
	}
	if cfg.CloneProto != "ssh" {
		t.Errorf("Default CloneProto = %s, want ssh", cfg.CloneProto)
	}
	if cfg.Sync.Strategy != "pull" {
		t.Errorf("Default Sync.Strategy = %s, want pull", cfg.Sync.Strategy)
	}

	// Check sources
	if cfg.GetSource("parallel") != "default" {
		t.Errorf("Source for parallel = %s, want default", cfg.GetSource("parallel"))
	}
}

func TestApplyProfile(t *testing.T) {
	loader := &ConfigLoader{
		activeProfile: &Profile{
			Name:       "test",
			Provider:   "gitlab",
			BaseURL:    "https://gitlab.com",
			Token:      "secret",
			CloneProto: "ssh",
			SSHPort:    2224,
			Parallel:   15,
			Sync: &SyncConfig{
				Strategy:   "reset",
				MaxRetries: 5,
			},
		},
	}

	cfg := &EffectiveConfig{
		Sources: make(map[string]string),
	}

	loader.applyProfile(cfg)

	// Check values from profile
	if cfg.Provider != "gitlab" {
		t.Errorf("Provider = %s, want gitlab", cfg.Provider)
	}
	if cfg.BaseURL != "https://gitlab.com" {
		t.Errorf("BaseURL = %s, want https://gitlab.com", cfg.BaseURL)
	}
	if cfg.Token != "secret" {
		t.Errorf("Token = %s, want secret", cfg.Token)
	}
	if cfg.CloneProto != "ssh" {
		t.Errorf("CloneProto = %s, want ssh", cfg.CloneProto)
	}
	if cfg.SSHPort != 2224 {
		t.Errorf("SSHPort = %d, want 2224", cfg.SSHPort)
	}
	if cfg.Parallel != 15 {
		t.Errorf("Parallel = %d, want 15", cfg.Parallel)
	}
	if cfg.Sync.Strategy != "reset" {
		t.Errorf("Sync.Strategy = %s, want reset", cfg.Sync.Strategy)
	}
	if cfg.Sync.MaxRetries != 5 {
		t.Errorf("Sync.MaxRetries = %d, want 10", cfg.Sync.MaxRetries)
	}

	// Check sources
	expectedSource := "profile:test"
	if cfg.GetSource("provider") != expectedSource {
		t.Errorf("Source = %s, want %s", cfg.GetSource("provider"), expectedSource)
	}
}

func TestApplyFlags(t *testing.T) {
	loader := &ConfigLoader{}
	cfg := &EffectiveConfig{
		Provider:   "github", // Existing value
		CloneProto: "ssh",    // Existing value
		Sources:    make(map[string]string),
	}

	flags := map[string]interface{}{
		"provider":    "gitlab", // Override
		"base-url":    "https://gitlab.company.com",
		"token":       "flag-token",
		"parallel":    20,
		"ssh-port":    2224,
		"clone-proto": "https", // Override
	}

	loader.applyFlags(cfg, flags)

	// Check overridden values
	if cfg.Provider != "gitlab" {
		t.Errorf("Provider = %s, want gitlab", cfg.Provider)
	}
	if cfg.BaseURL != "https://gitlab.company.com" {
		t.Errorf("BaseURL = %s, want https://gitlab.company.com", cfg.BaseURL)
	}
	if cfg.Token != "flag-token" {
		t.Errorf("Token = %s, want flag-token", cfg.Token)
	}
	if cfg.Parallel != 20 {
		t.Errorf("Parallel = %d, want 20", cfg.Parallel)
	}
	if cfg.SSHPort != 2224 {
		t.Errorf("SSHPort = %d, want 2224", cfg.SSHPort)
	}
	if cfg.CloneProto != "https" {
		t.Errorf("CloneProto = %s, want https", cfg.CloneProto)
	}

	// Check sources
	if cfg.GetSource("provider") != "flag" {
		t.Errorf("Source = %s, want flag", cfg.GetSource("provider"))
	}
}
