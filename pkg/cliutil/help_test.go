// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cliutil

import (
	"strings"
	"testing"
)

func TestQuickStartHelp(t *testing.T) {
	content := "  gz-git status\n  gz-git fetch"
	result := QuickStartHelp(content)

	if !strings.Contains(result, "Quick Start:") {
		t.Error("Expected 'Quick Start:' in output")
	}
	if !strings.Contains(result, content) {
		t.Error("Expected content to be included")
	}
	if !strings.Contains(result, ColorCyanBold) {
		t.Error("Expected cyan color code")
	}
	if !strings.Contains(result, ColorReset) {
		t.Error("Expected color reset code")
	}
}

func TestStripIndent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "  hello world  ",
			expected: "hello world",
		},
		{
			name:     "multiline",
			input:    "\n  line1\n  line2\n",
			expected: "line1\n  line2",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripIndent(tt.input)
			if result != tt.expected {
				t.Errorf("StripIndent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestColorConstants(t *testing.T) {
	// Verify color constants are ANSI escape sequences
	if !strings.HasPrefix(ColorCyanBold, "\033[") {
		t.Error("ColorCyanBold should be ANSI escape sequence")
	}
	if !strings.HasPrefix(ColorGreenBold, "\033[") {
		t.Error("ColorGreenBold should be ANSI escape sequence")
	}
	if !strings.HasPrefix(ColorYellowBold, "\033[") {
		t.Error("ColorYellowBold should be ANSI escape sequence")
	}
	if !strings.HasPrefix(ColorMagentaBold, "\033[") {
		t.Error("ColorMagentaBold should be ANSI escape sequence")
	}
	if ColorReset != "\033[0m" {
		t.Error("ColorReset should be ANSI reset sequence")
	}
}
