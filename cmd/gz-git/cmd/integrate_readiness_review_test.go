// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

func TestReadinessReviewCommandsHaveNoConfirmationBypass(t *testing.T) {
	for _, command := range []*cobra.Command{integrateReadinessUpdateReviewCmd, integrateReadinessUpdateAutoCmd, integrateReadinessUpdateRunCmd} {
		for _, name := range []string{"yes", "force", "non-interactive", "batch"} {
			if command.Flags().Lookup(name) != nil {
				t.Errorf("%s defines forbidden --%s", command.CommandPath(), name)
			}
		}
	}
}

func TestReadinessRunDispatchesFromGlobalModeBeforeGitWork(t *testing.T) {
	t.Run("manual rejects non-terminal", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("HOME", t.TempDir())
		t.Setenv("APPDATA", t.TempDir())
		err := integrateReadinessUpdateRunCmd.RunE(integrateReadinessUpdateRunCmd, nil)
		if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
			t.Fatalf("manual run error = %v", err)
		}
	})
	t.Run("automatic requires source before git", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("HOME", t.TempDir())
		t.Setenv("APPDATA", t.TempDir())
		paths, err := config.NewPaths()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(paths.ConfigDir, "config.yaml"), []byte("integration:\n  readinessApproval: automatic\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		readinessReviewExpectSource, readinessReviewStatement = "", ""
		t.Cleanup(func() { readinessReviewExpectSource, readinessReviewStatement = "", "" })
		err = integrateReadinessUpdateRunCmd.RunE(integrateReadinessUpdateRunCmd, nil)
		if err == nil || !strings.Contains(err.Error(), "--expect-source") {
			t.Fatalf("automatic run error = %v", err)
		}
	})
}

func TestEscapeTerminalControls(t *testing.T) {
	got := escapeTerminalControls("+ valid\n\x1b[2J\ttext\x7f\u202e한글\xff")
	if got != "+ valid\n\\x1b[2J\ttext\\x7f\\u202e한글\\xff" {
		t.Fatalf("escaped diff = %q", got)
	}
}

func TestInteractiveReadinessApprovalRejectsEOF(t *testing.T) {
	approved, err := interactiveReadinessApproval(bytes.NewBufferString("yes"))
	if err == nil || approved {
		t.Fatalf("EOF approval = (%v, %v), want refusal", approved, err)
	}
}

func TestReadinessAutoDeniedByDefaultBeforeGitWork(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	readinessReviewExpectSource = strings.Repeat("a", 40)
	readinessReviewStatement = "테스트완료"
	t.Cleanup(func() { readinessReviewExpectSource, readinessReviewStatement = "", "" })
	err := integrateReadinessUpdateAutoCmd.RunE(integrateReadinessUpdateAutoCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("auto error = %v", err)
	}
}
