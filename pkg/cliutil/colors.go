// Copyright (c) 2026 Archmagece
// SPDX-License-Identifier: MIT

package cliutil

import (
	"os"

	"github.com/mattn/go-isatty"
)

// Raw ANSI escape sequences. These are the immutable source values; the
// exported Color* vars below are reset to these by EnableColors and blanked by
// DisableColors. Bold variants are used for help/usage headers; the plain and
// gray/red variants are used by command output (watch, doctor).
const (
	ansiCyanBold    = "\033[1;36m"
	ansiGreenBold   = "\033[1;32m"
	ansiYellowBold  = "\033[1;33m"
	ansiMagentaBold = "\033[1;35m"
	ansiCyan        = "\033[36m"
	ansiGreen       = "\033[32m"
	ansiYellow      = "\033[33m"
	ansiMagenta     = "\033[35m"
	ansiRed         = "\033[31m"
	ansiGray        = "\033[90m"
	ansiReset       = "\033[0m"
)

// Exported color codes. They are vars (not consts) so the color gate can blank
// them when output is not a terminal, which lets every existing `cliutil.Color*`
// reference become a no-op without touching the call sites. Blanking happens in
// this package's init(), which runs before the cmd package builds its help
// strings — so help text assembled from these vars is already colorless in a
// non-terminal environment.
var (
	ColorCyanBold    = ansiCyanBold
	ColorGreenBold   = ansiGreenBold
	ColorYellowBold  = ansiYellowBold
	ColorMagentaBold = ansiMagentaBold
	ColorCyan        = ansiCyan
	ColorGreen       = ansiGreen
	ColorYellow      = ansiYellow
	ColorMagenta     = ansiMagenta
	ColorRed         = ansiRed
	ColorGray        = ansiGray
	ColorReset       = ansiReset
)

func init() {
	if !ColorsEnabled() {
		DisableColors()
	}
}

// IsTerminal reports whether the given file descriptor is a terminal. It is the
// single TTY-detection helper shared by the color gate (stdout) and the
// destructive-op confirmation prompt (stdin, see bulk_common).
func IsTerminal(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// ColorsEnabled reports whether ANSI color should be emitted, based on the
// environment and whether stdout is a terminal.
func ColorsEnabled() bool {
	return colorsEnabled(os.LookupEnv, IsTerminal(os.Stdout.Fd()))
}

// colorsEnabled is the pure decision function behind ColorsEnabled, taking its
// inputs as parameters so the precedence can be unit-tested without touching
// real env vars or file descriptors.
//
// Precedence (first match wins):
//  1. NO_COLOR present (any value, per https://no-color.org) → disabled.
//  2. TERM=dumb (terminal cannot render ANSI) → disabled.
//  3. otherwise → follow whether stdout is a terminal.
func colorsEnabled(lookupEnv func(string) (string, bool), stdoutIsTerminal bool) bool {
	if _, ok := lookupEnv("NO_COLOR"); ok {
		return false
	}
	if term, _ := lookupEnv("TERM"); term == "dumb" {
		return false
	}
	return stdoutIsTerminal
}

// DisableColors blanks every exported color code. Idempotent.
func DisableColors() {
	ColorCyanBold = ""
	ColorGreenBold = ""
	ColorYellowBold = ""
	ColorMagentaBold = ""
	ColorCyan = ""
	ColorGreen = ""
	ColorYellow = ""
	ColorMagenta = ""
	ColorRed = ""
	ColorGray = ""
	ColorReset = ""
}

// EnableColors restores every exported color code to its ANSI value. It exists
// mainly so tests can force colors on regardless of the test environment.
func EnableColors() {
	ColorCyanBold = ansiCyanBold
	ColorGreenBold = ansiGreenBold
	ColorYellowBold = ansiYellowBold
	ColorMagentaBold = ansiMagentaBold
	ColorCyan = ansiCyan
	ColorGreen = ansiGreen
	ColorYellow = ansiYellow
	ColorMagenta = ansiMagenta
	ColorRed = ansiRed
	ColorGray = ansiGray
	ColorReset = ansiReset
}
