// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// ErrResourceBusy reports that another gz-git process holds a local resource
// fence. gz-git never waits for it: whether and when to retry belongs to the
// caller, which owns any ordering of integrations.
var ErrResourceBusy = errors.New("integration resource is busy")

// maxLockHolderBytes bounds the holder record read back from a lock file.
const maxLockHolderBytes = 256

// resourceLock is an exclusive lock on a file below the Git common directory.
// The lock belongs to the open file description, so the kernel drops it when
// the holder closes the file or dies.
type resourceLock struct{ file *os.File }

func (l *resourceLock) release() {
	if l != nil && l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

// tryAcquireResourceLock takes the lock at path or refuses at once with
// ErrResourceBusy, naming the holder it recorded. what names the resource in
// the refusal. A holder records its pid and start time in the file so the
// refusal can say who to wait for; an unreadable record is reported as
// unidentified rather than failing the refusal.
func tryAcquireResourceLock(path, what string) (*resourceLock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) // #nosec G304 -- fixed digest names below the Git common directory
	if err != nil {
		return nil, fmt.Errorf("open %s lock: %w", what, err)
	}
	locked, err := tryLockFile(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock %s: %w", what, err)
	}
	if !locked {
		holder := readLockHolder(f)
		_ = f.Close()
		return nil, fmt.Errorf("%w: %s is held by %s; gz-git does not wait for it, retry after that process finishes", ErrResourceBusy, what, holder)
	}
	// The record is advisory: a failed write leaves the lock held and only
	// makes a later refusal report an unidentified holder.
	if err := f.Truncate(0); err == nil {
		_, _ = f.WriteAt([]byte(fmt.Sprintf("pid=%d since=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))), 0)
	}
	return &resourceLock{file: f}, nil
}

func readLockHolder(f *os.File) string {
	buf := make([]byte, maxLockHolderBytes)
	n, err := f.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return "an unidentified process"
	}
	record := strings.TrimSpace(string(buf[:n]))
	if record == "" || strings.ContainsFunc(record, func(r rune) bool { return r < ' ' || r == 0x7f }) {
		return "an unidentified process"
	}
	return "another process (" + record + ")"
}
