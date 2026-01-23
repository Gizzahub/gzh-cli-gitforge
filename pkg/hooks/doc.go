// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package hooks provides secure command execution for before/after hooks.
//
// Hooks are executed without shell interpretation for security:
//   - No pipes (|)
//   - No redirects (>, <, >>)
//   - No variable expansion ($VAR)
//   - No subshell execution
//
// Commands are parsed as simple space-separated arguments with basic quoting support.
// For complex operations, users should create scripts and call them from hooks.
//
// Example usage:
//
//	hooks := &config.Hooks{
//	    Before: []string{"mkdir -p logs"},
//	    After:  []string{"make setup", "echo done"},
//	}
//
//	err := hooks.Execute(ctx, hooks, "before", "/path/to/workdir", logger)
//	if err != nil {
//	    return fmt.Errorf("before hooks failed: %w", err)
//	}
//
// Security considerations:
//   - 30 second timeout per command
//   - Direct exec.Command (no shell)
//   - Working directory must exist
//   - Environment inherited from parent process
package hooks
