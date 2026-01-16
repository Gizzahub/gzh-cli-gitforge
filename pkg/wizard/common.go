// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package wizard

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Icons for wizard output.
const (
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "⚠"
	IconRocket  = "🚀"
	IconBroom   = "🧹"
	IconGear    = "⚙"
	IconInfo    = "ℹ"
	IconArrow   = "→"
)

// Styles for wizard output.
var (
	// TitleStyle is used for wizard titles.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("62")).
			MarginBottom(1)

	// SubtitleStyle is used for section headers.
	SubtitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("245"))

	// SuccessStyle is used for success messages.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	// ErrorStyle is used for error messages.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))

	// WarningStyle is used for warning messages.
	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	// DimStyle is used for less important text.
	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// KeyStyle is used for config keys.
	KeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("45"))

	// ValueStyle is used for config values.
	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
)

// Printer handles wizard output.
type Printer struct {
	Out io.Writer
}

// NewPrinter creates a new Printer with stdout as default.
func NewPrinter() *Printer {
	return &Printer{Out: os.Stdout}
}

// PrintHeader prints a wizard header with icon.
func (p *Printer) PrintHeader(icon, title string) {
	fmt.Fprintln(p.Out)
	fmt.Fprintln(p.Out, TitleStyle.Render(icon+" "+title))
	fmt.Fprintln(p.Out)
}

// PrintSubtitle prints a section subtitle.
func (p *Printer) PrintSubtitle(title string) {
	fmt.Fprintln(p.Out, SubtitleStyle.Render(title))
}

// PrintSuccess prints a success message.
func (p *Printer) PrintSuccess(msg string) {
	fmt.Fprintln(p.Out, SuccessStyle.Render(IconSuccess+" "+msg))
}

// PrintError prints an error message.
func (p *Printer) PrintError(msg string) {
	fmt.Fprintln(p.Out, ErrorStyle.Render(IconError+" "+msg))
}

// PrintWarning prints a warning message.
func (p *Printer) PrintWarning(msg string) {
	fmt.Fprintln(p.Out, WarningStyle.Render(IconWarning+" "+msg))
}

// PrintInfo prints an info message.
func (p *Printer) PrintInfo(msg string) {
	fmt.Fprintln(p.Out, DimStyle.Render(IconInfo+" "+msg))
}

// PrintKeyValue prints a key-value pair.
func (p *Printer) PrintKeyValue(key, value string) {
	fmt.Fprintf(p.Out, "  %s %s\n",
		KeyStyle.Render(key+":"),
		ValueStyle.Render(value))
}

// PrintSummary prints a configuration summary.
func (p *Printer) PrintSummary(title string, items map[string]string) {
	fmt.Fprintln(p.Out)
	p.PrintSubtitle(title)
	fmt.Fprintln(p.Out)

	for key, value := range items {
		if value != "" {
			p.PrintKeyValue(key, value)
		}
	}
}

// PrintOrderedSummary prints a configuration summary in order.
func (p *Printer) PrintOrderedSummary(title string, keys []string, items map[string]string) {
	fmt.Fprintln(p.Out)
	p.PrintSubtitle(title)
	fmt.Fprintln(p.Out)

	for _, key := range keys {
		if value, ok := items[key]; ok && value != "" {
			p.PrintKeyValue(key, value)
		}
	}
}

// PrintNextSteps prints next steps.
func (p *Printer) PrintNextSteps(steps []string) {
	fmt.Fprintln(p.Out)
	p.PrintSubtitle("Next Steps")
	fmt.Fprintln(p.Out)

	for i, step := range steps {
		fmt.Fprintf(p.Out, "  %d. %s\n", i+1, step)
	}
}

// PrintDivider prints a horizontal divider.
func (p *Printer) PrintDivider() {
	fmt.Fprintln(p.Out, DimStyle.Render(strings.Repeat("─", 50)))
}

// SanitizeTokenForDisplay masks a token for display.
func SanitizeTokenForDisplay(token string) string {
	if token == "" {
		return "(not set)"
	}

	// Check if it's an environment variable reference
	if strings.HasPrefix(token, "${") && strings.HasSuffix(token, "}") {
		return token // Show env var reference as-is
	}

	// Mask the token
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// FormatBool formats a boolean for display.
func FormatBool(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// FormatInt formats an integer for display, returning "(default)" for zero values.
func FormatInt(i int, defaultVal int) string {
	if i == 0 || i == defaultVal {
		return fmt.Sprintf("%d (default)", defaultVal)
	}
	return fmt.Sprintf("%d", i)
}
