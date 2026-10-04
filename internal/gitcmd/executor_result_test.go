package gitcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// exitErrorWithCode returns a real *exec.ExitError produced by a process that
// exits with the given code.
func exitErrorWithCode(t *testing.T, code int) *exec.ExitError {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}
	err := exec.CommandContext(context.Background(), "/bin/sh", "-c", fmt.Sprintf("exit %d", code)).Run() // #nosec G204 -- fixed test fixture.
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("expected *exec.ExitError, got %T (%v)", err, err)
	}
	return exitError
}

// TestCommandResult covers every branch of commandResult directly so that its
// coverage never depends on process-exit timing.
func TestCommandResult(t *testing.T) {
	exitError := exitErrorWithCode(t, 3)
	plainError := errors.New("plain failure")

	tests := []struct {
		name         string
		commandErr   error
		wantExitCode int
	}{
		{name: "nil error", commandErr: nil, wantExitCode: 0},
		{name: "exit error", commandErr: exitError, wantExitCode: 3},
		{name: "wrapped exit error", commandErr: fmt.Errorf("wait: %w", exitError), wantExitCode: 3},
		{name: "plain error", commandErr: plainError, wantExitCode: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := commandResult("out", "err", tt.commandErr, time.Now())
			if result.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.wantExitCode)
			}
			if !errors.Is(result.Error, tt.commandErr) {
				t.Errorf("Error = %v, want %v", result.Error, tt.commandErr)
			}
			if result.Stdout != "out" || result.Stderr != "err" {
				t.Errorf("Stdout/Stderr = %q/%q, want out/err", result.Stdout, result.Stderr)
			}
		})
	}
}

// TestExecutorRunWithOutputLimitKillsLongRunningProcess uses a fixture that
// keeps running after exceeding the limit, so the cancellation always hits a
// live process and the result is deterministic.
func TestExecutorRunWithOutputLimitKillsLongRunningProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}
	binary := filepath.Join(t.TempDir(), "git")
	script := "#!/bin/sh\nprintf '0123456789'\nexec sleep 5\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	executor := NewExecutor(WithGitBinary(binary), WithTimeout(30*time.Second))

	started := time.Now()
	result, overflow, err := executor.RunWithOutputLimit(context.Background(), t.TempDir(), nil, 4, "archive")
	elapsed := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed >= 3*time.Second {
		t.Errorf("call took %v, want < 3s (process must be killed, not awaited)", elapsed)
	}
	if !overflow {
		t.Errorf("overflow = false, want true")
	}
	if result.Stdout != "0123" {
		t.Errorf("stdout = %q, want truncated %q", result.Stdout, "0123")
	}
	if result.Error == nil {
		t.Errorf("Result.Error = nil, want kill error")
	}
	if result.ExitCode == 0 {
		t.Errorf("ExitCode = 0, want non-zero")
	}
}
