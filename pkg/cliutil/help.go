// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cliutil

import "strings"

// ANSI escape sequences for bold color output in terminal help text.
const (
	ColorCyanBold    = "\033[1;36m"
	ColorGreenBold   = "\033[1;32m"
	ColorYellowBold  = "\033[1;33m"
	ColorMagentaBold = "\033[1;35m"
	ColorReset       = "\033[0m"
)

// QuickStartHelp returns a standardized "Quick Start" help string with colors.
// It wraps the content (which should contain the examples) with the styled header.
// It automatically handles the newline prefix.
func QuickStartHelp(content string) string {
	// Ensure content is not empty and has proper indentation if needed,
	// but mostly we just wrap it.
	return " " + ColorCyanBold + "Quick Start:" + ColorReset + "\n" + content
}

// ExitCodesBulkHelp returns the standardized "Exit Codes" help section for
// bulk commands. It is appended to a command's Long description so `--help`
// documents the exit-code contract.
func ExitCodesBulkHelp() string {
	return "\n\n " + ColorCyanBold + "Exit Codes:" + ColorReset + `
  0  all repositories processed successfully
  1  tool or configuration error (nothing ran)
  2  one or more repositories failed`
}

// ExitCodesConflictHelp returns the "Exit Codes" help section for
// `conflict detect`, which follows the grep-style convention instead.
func ExitCodesConflictHelp() string {
	return "\n\n " + ColorCyanBold + "Exit Codes:" + ColorReset + `
  0  no conflicts detected
  1  conflicts found
  2  execution error`
}

// StripIndent removes common leading indentation from a multiline string.
func StripIndent(s string) string {
	return strings.TrimSpace(s) // Simple trim for now, mostly handled by backticks in Go
}
