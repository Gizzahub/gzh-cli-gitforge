// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// preparationWaitDelay bounds how long Cmd.Wait keeps reading the
// stdout/stderr pipe after Cancel fires for a preparation subprocess. A
// preparation command may fork a whole process tree; killing only the direct
// child leaves grandchildren holding a duplicated copy of the pipe's write
// end, so Cmd.Wait would block for EOF forever. Cmd.WaitDelay forces the pipe
// closed and Wait to return once the delay elapses, regardless of what Cancel
// managed to kill.
const preparationWaitDelay = 10 * time.Second

// preparationProcessAttachFailure fails closed before the command is released.
// On Windows the handshake keeps cmd.exe from spawning the command until
// release, so killing cmd here cannot leave a descendant behind. WaitDelay
// bounds Wait if a platform process unexpectedly retains one of the pipes.
func preparationProcessAttachFailure(cmd *exec.Cmd, processTree *preparationProcessTree, cause error) error {
	cancelErr := processTree.cancel()
	var killErr error
	if cmd.Process != nil {
		killErr = cmd.Process.Kill()
	}
	return errors.Join(fmt.Errorf("attach process tree: %w", cause), cancelErr, killErr, cmd.Wait())
}

func withoutEnv(env []string, key string) []string {
	return withoutEnvCase(env, key, runtime.GOOS == "windows")
}

func withoutEnvCase(env []string, key string, insensitive bool) []string {
	out := make([]string, 0, len(env))
	for _, item := range env {
		name, _, hasValue := strings.Cut(item, "=")
		matches := name == key || (insensitive && strings.EqualFold(name, key))
		if !hasValue || !matches {
			out = append(out, item)
		}
	}
	return out
}
