package runtask

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CommandRequest names one external command: the tool, its fixed argv, the
// working directory, and extra environment entries.
type CommandRequest struct {
	Command string
	Args    []string
	WorkDir string
	Env     map[string]string
}

// CommandResult carries output and exit state for one command execution,
// including the evidence fields receipts record when the command never ran.
type CommandResult struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	Command    string
	Executable string
	Args       []string
	WorkDir    string
	Error      string
	NotStarted bool
}

// CommandRunner executes an external command and captures stdout/stderr separately.
type CommandRunner interface {
	Run(context.Context, CommandRequest) (CommandResult, error)
}

// Executor is the production command runner. Only fixed-argv invocations of
// audited tools (git, wt) go through it; user input never reaches a shell.
type Executor struct{}

// NewExecutor returns the production command runner.
func NewExecutor() *Executor { return &Executor{} }

// Run executes req with a resolved absolute executable path and separated
// output capture, mapping cancellation and lookup failures onto the recorded
// exit codes (125 canceled, 127 not started).
func (e *Executor) Run(ctx context.Context, req CommandRequest) (CommandResult, error) {
	result := CommandResult{Command: req.Command, Args: append([]string(nil), req.Args...), WorkDir: req.WorkDir}
	if err := ctx.Err(); err != nil {
		result.ExitCode, result.Error, result.NotStarted = 125, err.Error(), true
		return result, fmt.Errorf("command %s canceled before start: %w", strings.Join(append([]string{req.Command}, req.Args...), " "), err)
	}
	executable, err := exec.LookPath(req.Command)
	if err != nil {
		result.ExitCode, result.Error, result.NotStarted = 127, err.Error(), true
		return result, fmt.Errorf("resolve command %s: %w", req.Command, err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		result.ExitCode, result.Error, result.NotStarted = 127, err.Error(), true
		return result, fmt.Errorf("resolve command %s path: %w", req.Command, err)
	}
	result.Executable = executable
	cmd := exec.CommandContext(ctx, executable, req.Args...) //nolint:gosec // fixed tool name, caller-supplied argv only
	cmd.Dir, cmd.Env = req.WorkDir, os.Environ()
	for key, value := range req.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	result.Stdout, result.Stderr = stdout.String(), stderr.String()
	if err == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		result.Error = ctx.Err().Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode, result.NotStarted = 125, cmd.Process == nil
		}
		return result, fmt.Errorf("command %s canceled: %w", strings.Join(append([]string{req.Command}, req.Args...), " "), ctx.Err())
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	result.ExitCode, result.Error, result.NotStarted = 127, err.Error(), true
	return result, fmt.Errorf("start command %s: %w", req.Command, err)
}
