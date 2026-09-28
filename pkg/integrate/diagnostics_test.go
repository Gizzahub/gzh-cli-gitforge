// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteDiagnosticPrivateAndUnique(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("private POSIX diagnostic permissions are unavailable on Windows")
	}
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	first, err := writeDiagnostic("make-check", []byte("first failure\n"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeDiagnostic("make-check", []byte("second failure\n"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, filepath.Join(state, "gz-git", "integrate", "diagnostics")+string(filepath.Separator)) {
		t.Fatalf("unexpected diagnostic paths: %q, %q", first, second)
	}
	for _, tc := range []struct {
		path string
		want string
	}{
		{first, "first failure\n"},
		{second, "second failure\n"},
	} {
		data, err := os.ReadFile(tc.path)
		if err != nil || string(data) != tc.want {
			t.Fatalf("read %q: %q, %v", tc.path, data, err)
		}
		info, err := os.Stat(tc.path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("file mode %q: %v, %v", tc.path, info, err)
		}
	}
	info, err := os.Stat(filepath.Dir(first))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode: %v, %v", info, err)
	}
}

func TestWriteDiagnosticRejectsInvalidKind(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if _, err := writeDiagnostic("../escape", []byte("failure")); err == nil {
		t.Fatal("expected invalid kind to be rejected")
	}
	path, err := writeDiagnostic("make-check", nil)
	if err != nil || path != "" {
		t.Fatalf("empty output: path=%q err=%v", path, err)
	}
}
