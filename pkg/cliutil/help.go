package cliutil

import "strings"

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

// StripIndent removes common leading indentation from a multiline string.
func StripIndent(s string) string {
	return strings.TrimSpace(s) // Simple trim for now, mostly handled by backticks in Go
}
