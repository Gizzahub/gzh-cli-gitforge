// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func runGitInTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.CommandContext(context.Background(), "git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// lockedBuffer collects the waiting notice while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fastLockPolling makes lock waits observable within a test's runtime.
func fastLockPolling(t *testing.T) {
	t.Helper()
	poll, notice := lockPollInterval, lockNoticeInterval
	lockPollInterval, lockNoticeInterval = 10*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { lockPollInterval, lockNoticeInterval = poll, notice })
}

func waitForText(t *testing.T, what string, read func() string, want string) {
	t.Helper()
	deadline := time.Now().Add(testWaitTimeout)
	for !strings.Contains(read(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("%s never contained %q within %s; got:\n%s", what, want, testWaitTimeout, read())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
