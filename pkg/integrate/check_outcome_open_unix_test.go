//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/gizzahub/gzh-cli-gitforge/internal/safefs"
)

func TestMakeOutcomeOpenRejectsReplacedPaths(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "outcome.json")
			if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			root, err := safefs.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			if _, err := root.Lstat("outcome.json"); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if kind == "fifo" {
				if err := unix.Mkfifo(path, 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				target := filepath.Join(dir, "target.json")
				if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			result := make(chan error, 1)
			go func() {
				f, err := openMakeOutcomeReportFile(root, "outcome.json")
				if f != nil {
					_ = f.Close()
				}
				result <- err
			}()
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("replaced nonregular path was accepted")
				}
			case <-time.After(2 * time.Second):
				// Unblock a regressed FIFO reader before failing, so the test
				// does not leave a background goroutine waiting for a writer.
				fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0)
				if err == nil {
					_ = unix.Close(fd)
				}
				<-result
				t.Fatal("opening a replaced report path blocked")
			}
		})
	}
}
