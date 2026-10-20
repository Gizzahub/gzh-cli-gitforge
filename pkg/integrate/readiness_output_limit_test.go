// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteReadiness_NonzeroAndOversizeFailUnavailable(t *testing.T) {
	dir := t.TempDir()
	// The oversize runners print a valid ready result first and then 3 MiB,
	// so only the capture limit can reject them: whitespace padding keeps
	// stdout decodable apart from its size, and stderr is never parsed.
	ready := "printf '{\"version\":1,\"status\":\"ready\",\"summary\":\"s\"}'\n"
	pad := "head -c 3145728 /dev/zero | tr '\\000' ' '"
	for _, tt := range []struct{ name, body, want string }{
		{"nonzero", "#!/bin/sh\necho bad >&2\nexit 7\n", "runner failed: bad"},
		{"stdout", "#!/bin/sh\n" + ready + pad + "\n", "runner output exceeded limit"},
		{"stderr", "#!/bin/sh\n" + ready + pad + " >&2\n", "runner output exceeded limit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := filepath.Join(dir, tt.name)
			if err := os.WriteFile(runner, []byte(tt.body), 0o755); err != nil {
				t.Fatal(err)
			}
			result, err := executeReadinessWithTimeout(context.Background(), runner, dir, dir, "a", "b", 10*time.Second)
			if err == nil || !strings.Contains(err.Error(), tt.want) || result.Status != "unavailable" {
				t.Fatalf("result = %q, %v; want unavailable with %q", result.Status, err, tt.want)
			}
			if len(result.stderr) > readinessMaxIO {
				t.Fatalf("captured stderr = %d bytes, want at most %d", len(result.stderr), readinessMaxIO)
			}
		})
	}
}

// TestLimitedBufferCopyHonorsLimit drives the buffer through io.Copy, the
// path os/exec uses, which prefers a ReaderFrom over Write when one exists.
func TestLimitedBufferCopyHonorsLimit(t *testing.T) {
	b := limitedBuffer{limit: 4}
	if _, err := io.Copy(&b, strings.NewReader("0123456789")); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != "0123" || !b.exceeded {
		t.Fatalf("buffer = %q, exceeded = %v; want %q, true", got, b.exceeded, "0123")
	}
}
