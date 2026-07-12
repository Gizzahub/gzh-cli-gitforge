// Copyright (c) 2026 Archmagece
// SPDX-License-Identifier: MIT

package cliutil

import (
	"strings"
	"testing"
)

// TestColorsEnabledDecision exercises the precedence of the color gate: NO_COLOR
// (presence, any value) and TERM=dumb both force-disable regardless of the TTY,
// otherwise the decision follows whether stdout is a terminal.
func TestColorsEnabledDecision(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		isTTY bool
		want  bool
	}{
		{"terminal, no env", map[string]string{}, true, true},
		{"not a terminal, no env", map[string]string{}, false, false},
		{"NO_COLOR set beats terminal", map[string]string{"NO_COLOR": "1"}, true, false},
		{"NO_COLOR empty value still disables", map[string]string{"NO_COLOR": ""}, true, false},
		{"NO_COLOR=0 still disables (presence, not value)", map[string]string{"NO_COLOR": "0"}, true, false},
		{"TERM=dumb disables on a terminal", map[string]string{"TERM": "dumb"}, true, false},
		{"TERM=xterm keeps terminal colors", map[string]string{"TERM": "xterm-256color"}, true, true},
		{"TERM=dumb but not a terminal", map[string]string{"TERM": "dumb"}, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(k string) (string, bool) {
				v, ok := tt.env[k]
				return v, ok
			}
			if got := colorsEnabled(lookup, tt.isTTY); got != tt.want {
				t.Errorf("colorsEnabled(env=%v, tty=%v) = %v, want %v", tt.env, tt.isTTY, got, tt.want)
			}
		})
	}
}

// TestDisableEnableColors verifies the blank/restore mechanism that the gate and
// tests rely on. It restores the environment-driven state afterwards so it does
// not leak color state into other tests.
func TestDisableEnableColors(t *testing.T) {
	t.Cleanup(restoreColorState)

	DisableColors()
	blanked := map[string]string{
		"ColorCyanBold": ColorCyanBold,
		"ColorGreen":    ColorGreen,
		"ColorRed":      ColorRed,
		"ColorGray":     ColorGray,
		"ColorReset":    ColorReset,
	}
	for name, v := range blanked {
		if v != "" {
			t.Errorf("after DisableColors, %s = %q, want empty", name, v)
		}
	}

	EnableColors()
	if !strings.Contains(ColorCyanBold, "\x1b") || ColorReset == "" {
		t.Errorf("after EnableColors, expected ANSI restored, got ColorCyanBold=%q ColorReset=%q", ColorCyanBold, ColorReset)
	}
}

// restoreColorState resets the exported color vars to whatever the current
// environment dictates, matching what init() would have chosen.
func restoreColorState() {
	if ColorsEnabled() {
		EnableColors()
	} else {
		DisableColors()
	}
}
