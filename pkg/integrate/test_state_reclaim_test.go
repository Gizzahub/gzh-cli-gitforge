// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// testStatePrefix names the state dir TestMain creates for a suite. The
// owning test binary's pid follows it, so a later suite can tell a dir that
// a killed or timed-out suite left behind from one a live suite still uses
// (ISSUE-84).
const testStatePrefix = "gz-git-integrate-state-"

func newTestStateDir() (string, error) {
	return os.MkdirTemp("", fmt.Sprintf("%s%d-", testStatePrefix, os.Getpid()))
}

// reclaimDeadTestStateDirs removes the state dirs under tmp whose owning
// process has exited, and returns the names it removed. A dir is kept when
// its name carries no pid (the naming before ISSUE-84), when the owner is
// alive, or when its liveness cannot be read. A pid reused by an unrelated
// process keeps the dir until that process exits too. Liveness is read in
// this process's pid namespace, so a suite in another container sharing
// TMPDIR looks dead.
func reclaimDeadTestStateDirs(tmp string) []string {
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return nil
	}
	var removed []string
	for _, entry := range entries {
		rest, ok := strings.CutPrefix(entry.Name(), testStatePrefix)
		if !ok || !entry.IsDir() {
			continue
		}
		pidText, _, ok := strings.Cut(rest, "-")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(pidText)
		if err != nil || pid <= 0 || pid == os.Getpid() {
			continue
		}
		if alive, err := processAlive(pid); err != nil || alive {
			continue
		}
		if err := os.RemoveAll(filepath.Join(tmp, entry.Name())); err != nil {
			fmt.Fprintf(os.Stderr, "reclaim test state dir %s: %v\n", entry.Name(), err)
			continue
		}
		removed = append(removed, entry.Name())
	}
	return removed
}
