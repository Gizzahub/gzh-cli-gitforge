//go:build linux || darwin || freebsd || netbsd

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// ISSUE-106: ext4 reuses a freed inode number, so a directory removed and
// re-created at the same path can keep its device and inode. Birth time is the
// part of the identity that a re-creation must move.
func TestLegacyPhysicalIdentityBirthMovesWhenAPathIsRecreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := legacyPhysicalIdentity(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if before.BirthNS == 0 {
		// APFS always records one, so a zero there means the read broke.
		if runtime.GOOS == "darwin" {
			t.Fatal("no birth time read on darwin")
		}
		t.Skip("this filesystem records no birth time")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// Kernel file timestamps advance on a coarse clock tick; step past it.
	time.Sleep(20 * time.Millisecond)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	after, err := legacyPhysicalIdentity(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if after.BirthNS == before.BirthNS {
		t.Fatalf("re-created directory kept its birth time %d", before.BirthNS)
	}
	reused := before
	reused.BirthNS = after.BirthNS
	if reused == before {
		t.Fatal("an identity differing only in birth time compared equal")
	}
}
