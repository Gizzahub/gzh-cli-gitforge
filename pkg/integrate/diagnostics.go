// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// writeDiagnostic keeps failure output after temporary integration worktrees
// are removed. The files may contain credentials printed by a failing tool,
// so only the current user may read the directory and its contents.
func writeDiagnostic(kind string, output []byte) (string, error) {
	if len(output) == 0 {
		return "", nil
	}
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("private diagnostic files are unsupported on Windows")
	}
	if kind == "" || strings.ContainsAny(kind, `/\\.`) {
		return "", fmt.Errorf("invalid diagnostic kind %q", kind)
	}
	for _, r := range kind {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return "", fmt.Errorf("invalid diagnostic kind %q", kind)
		}
	}
	stateRoot := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(stateRoot) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home for diagnostic: %w", err)
		}
		stateRoot = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(stateRoot, "gz-git", "integrate", "diagnostics")
	if err := os.MkdirAll(dir, 0o700); err != nil { // #nosec G703 -- local XDG state path; kind is restricted to a safe basename
		return "", fmt.Errorf("create diagnostic directory: %w", err)
	}
	info, err := os.Lstat(dir) // #nosec G703 -- inspect the exact local state directory before writing
	if err != nil {
		return "", fmt.Errorf("inspect diagnostic directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("diagnostic directory is not a regular directory: %s", dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil { // #nosec G302 G703 -- checked local state directory needs owner traversal
		return "", fmt.Errorf("secure diagnostic directory: %w", err)
	}
	f, err := os.CreateTemp(dir, kind+"-*.log")
	if err != nil {
		return "", fmt.Errorf("create diagnostic file: %w", err)
	}
	path := f.Name()
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(path) // #nosec G703 -- path came from os.CreateTemp in the checked directory
		return "", fmt.Errorf("secure diagnostic file: %w", err)
	}
	_, writeErr := f.Write(output)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path) // #nosec G703 -- path came from os.CreateTemp in the checked directory
		return "", fmt.Errorf("write diagnostic file: %w", firstDiagnosticError(writeErr, closeErr))
	}
	return path, nil
}

func firstDiagnosticError(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
