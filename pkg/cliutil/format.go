package cliutil

import (
	"fmt"
	"strings"
)

// CoreFormats contains the default formats supported by all commands
var CoreFormats = []string{"default", "compact", "json", "llm"}

// TabularFormats contains formats meant for tabular data output
var TabularFormats = []string{"default", "compact", "json", "llm", "table", "csv", "markdown"}

// ValidateFormat checks if the given format is in the allowed list
func ValidateFormat(format string, allowed []string) error {
	for _, f := range allowed {
		if format == f {
			return nil
		}
	}
	return fmt.Errorf("invalid format: %s (allowed: %s)", format, strings.Join(allowed, ", "))
}

// IsMachineFormat returns true for formats intended for machine consumption
func IsMachineFormat(format string) bool {
	f := strings.ToLower(format)
	return f == "json" || f == "llm" || f == "csv"
}
