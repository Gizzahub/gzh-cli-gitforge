// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReadinessApprovalUsesGlobalConfigOnly(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	paths, err := NewPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.ConfigDir, "config.yaml"), []byte("integration:\n  readinessApproval: automatic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".gz-git.yaml"), []byte("integration:\n  readinessApproval: manual\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	got, err := LoadReadinessApproval()
	if err != nil {
		t.Fatalf("LoadReadinessApproval: %v", err)
	}
	if got != ReadinessApprovalAutomatic {
		t.Fatalf("approval = %q, want automatic", got)
	}
}

func TestLoadReadinessApprovalDefaultsAndRejectsUnknownValue(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	got, err := LoadReadinessApproval()
	if err != nil {
		t.Fatalf("default LoadReadinessApproval: %v", err)
	}
	if got != ReadinessApprovalManual {
		t.Fatalf("default approval = %q, want manual", got)
	}
	paths, err := NewPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.ConfigDir, "config.yaml"), []byte("integration:\n  readinessApproval: unsafe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReadinessApproval(); err == nil || !strings.Contains(err.Error(), "readinessApproval") {
		t.Fatalf("unknown value error = %v", err)
	}
}
