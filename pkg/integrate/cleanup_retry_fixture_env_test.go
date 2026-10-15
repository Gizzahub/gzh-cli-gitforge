// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupInterceptIgnoresAmbientBashInitializer(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "initializer-ran")
	initializer := filepath.Join(root, "bash-env")
	if err := os.WriteFile(initializer, []byte(": > \"$TASK302_BASH_ENV_MARKER\"\nexit 73\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BASH_ENV", initializer)
	t.Setenv("TASK302_BASH_ENV_MARKER", marker)
	f := &cleanupRetryFX{}
	f.interceptGit(t, `if [[ "$1" == "version" ]]; then
  printf '%s\n' 'injected fixture fault' >&2
  exit 42
fi`)
	result, err := f.exec.Run(context.Background(), root, "version")
	if err != nil || result == nil || result.ExitCode != 42 || !strings.Contains(result.Stderr, "injected fixture fault") {
		t.Fatalf("fault injection was replaced by ambient startup: result=%+v err=%v", result, err)
	}
	result, err = f.exec.Run(context.Background(), root, "--version")
	if err != nil || result == nil || result.ExitCode != 0 || !strings.HasPrefix(result.Stdout, "git version ") {
		t.Fatalf("real Git delegation failed: result=%+v err=%v", result, err)
	}
	if _, err := os.Lstat(marker); !os.IsNotExist(err) {
		t.Fatalf("ambient Bash initializer executed: %v", err)
	}
}
