// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDetectConfigFile_FindsParentFromSubdir is the core footgun fix (AC1):
// DetectConfigFile used to probe only the given directory, so a command run from
// a workspace subdirectory would miss the parent .gz-git.yaml and propose a new
// one in the wrong place. It must now walk upward and resolve the parent config.
func TestDetectConfigFile_FindsParentFromSubdir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	parent := filepath.Join(home, "workspace")
	sub := filepath.Join(parent, "project", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(parent, ".gz-git.yaml")
	if err := os.WriteFile(want, []byte("# parent config"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DetectConfigFile(sub)
	if err != nil {
		t.Fatalf("expected to find parent config from subdir, got error: %v", err)
	}
	if got != want {
		t.Errorf("DetectConfigFile(%s) = %s, want parent config %s", sub, got, want)
	}
}

// TestConfigDiscovery_Consistency is the anti-regression keystone (AC2): the
// three discovery entry points must agree on the same config file when invoked
// from the same working directory. Divergence here is exactly the original bug
// (config show found the parent while workspace sync did not).
func TestConfigDiscovery_Consistency(t *testing.T) {
	// Resolve symlinks: on macOS os.Getwd() (used by FindProjectConfig) returns
	// the /private/var real path while t.TempDir() returns the /var symlink form.
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	parent := filepath.Join(home, "ws")
	sub := filepath.Join(parent, "proj", "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	wantFile := filepath.Join(parent, ".gz-git.yaml")
	if err := os.WriteFile(wantFile, []byte("# config"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(sub) // Go 1.24+: sets cwd to sub for this test, auto-restored

	projectPath, err := FindProjectConfig()
	if err != nil {
		t.Fatalf("FindProjectConfig: %v", err)
	}
	detectPath, err := DetectConfigFile(".")
	if err != nil {
		t.Fatalf("DetectConfigFile: %v", err)
	}
	recursiveDir, err := FindConfigRecursive(".", ".gz-git.yaml")
	if err != nil {
		t.Fatalf("FindConfigRecursive: %v", err)
	}

	if projectPath != wantFile {
		t.Errorf("FindProjectConfig = %s, want %s", projectPath, wantFile)
	}
	if detectPath != wantFile {
		t.Errorf("DetectConfigFile(.) = %s, want %s", detectPath, wantFile)
	}
	if recursiveDir != parent {
		t.Errorf("FindConfigRecursive dir = %s, want %s", recursiveDir, parent)
	}
	// All three must converge: same file, from the same cwd.
	if projectPath != detectPath || filepath.Dir(projectPath) != recursiveDir {
		t.Errorf("discovery disagreement: project=%s detect=%s recursiveDir=%s",
			projectPath, detectPath, recursiveDir)
	}
}

// TestConfigDiscovery_StopsAtHome enforces the $HOME ceiling (AC3): discovery
// must not ascend above the user's home directory (preventing leaks into other
// users' or system directories), while still matching a config located exactly
// at $HOME (inclusive ceiling).
func TestConfigDiscovery_StopsAtHome(t *testing.T) {
	t.Run("does not cross above $HOME", func(t *testing.T) {
		base := t.TempDir()
		home := filepath.Join(base, "home")
		sub := filepath.Join(home, "a", "b")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		// Config ABOVE $HOME must be invisible to discovery.
		aboveHome := filepath.Join(base, ".gz-git.yaml")
		if err := os.WriteFile(aboveHome, []byte("# above home"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)

		if got, err := DetectConfigFile(sub); err == nil {
			t.Errorf("discovery crossed above $HOME and found %s", got)
		}
	})

	t.Run("matches config located at $HOME", func(t *testing.T) {
		base := t.TempDir()
		home := filepath.Join(base, "home")
		sub := filepath.Join(home, "a", "b")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		atHome := filepath.Join(home, ".gz-git.yaml")
		if err := os.WriteFile(atHome, []byte("# home config"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)

		got, err := DetectConfigFile(sub)
		if err != nil {
			t.Fatalf("expected to find config at $HOME, got error: %v", err)
		}
		if got != atHome {
			t.Errorf("got %s, want config at $HOME %s", got, atHome)
		}
	})
}
