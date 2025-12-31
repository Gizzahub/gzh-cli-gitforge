// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package parser

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseError represents a parsing error.
type ParseError struct {
	// Line is the line number where parsing failed (0-indexed).
	Line int

	// Content is the content that failed to parse.
	Content string

	// Reason describes why parsing failed.
	Reason string

	// Cause is the underlying error, if any.
	Cause error
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	msg := fmt.Sprintf("parse error at line %d: %s", e.Line, e.Reason)
	if e.Content != "" {
		msg += fmt.Sprintf(" (content: %q)", e.Content)
	}
	if e.Cause != nil {
		msg += fmt.Sprintf(": %v", e.Cause)
	}
	return msg
}

// Unwrap implements error unwrapping.
func (e *ParseError) Unwrap() error {
	return e.Cause
}

// Is implements error comparison.
func (e *ParseError) Is(target error) bool {
	_, ok := target.(*ParseError)
	return ok
}

// SplitLines splits text into lines, handling different line endings.
// Returns a slice of lines with line endings removed.
func SplitLines(text string) []string {
	if text == "" {
		return []string{}
	}

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines
}

// ParseKeyValue parses a "key: value" or "key=value" format line.
// Returns the key and value, with whitespace trimmed.
func ParseKeyValue(line string, separator string) (string, string, error) {
	parts := strings.SplitN(line, separator, 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid key-value format")
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	return key, value, nil
}

// ParseInt safely parses a string to int.
// Returns 0 if parsing fails (instead of error).
func ParseInt(s string) int {
	val, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return val
}

// ParseBool safely parses a string to bool.
// Recognizes: "true", "yes", "1" as true; "false", "no", "0" as false.
func ParseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "yes" || s == "1"
}

// ParseTimestamp parses a Unix timestamp string.
// Returns zero time if parsing fails.
func ParseTimestamp(s string) time.Time {
	timestamp, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

// ParseDate parses a date string in common Git formats.
// Supports: RFC3339, "2006-01-02", Unix timestamp.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)

	// Try Unix timestamp first
	if timestamp, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(timestamp, 0), nil
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try date only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}

	// Try Git's default format
	if t, err := time.Parse("Mon Jan 2 15:04:05 2006 -0700", s); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", s)
}

// ParseRef parses a Git reference in format "ref -> target" or just "ref".
// Returns the ref name and optionally the target it points to.
func ParseRef(line string) (ref string, target string) {
	parts := strings.Split(line, "->")
	ref = strings.TrimSpace(parts[0])

	if len(parts) > 1 {
		target = strings.TrimSpace(parts[1])
	}

	return ref, target
}

// ParseCommitHash extracts a commit hash from various formats.
// Supports full SHA-1 (40 chars) and abbreviated (7+ chars).
func ParseCommitHash(s string) (string, error) {
	s = strings.TrimSpace(s)

	// Full SHA-1: 40 hex characters
	if len(s) == 40 && isHex(s) {
		return s, nil
	}

	// Abbreviated: 7-40 hex characters
	if len(s) >= 7 && len(s) <= 40 && isHex(s) {
		return s, nil
	}

	return "", fmt.Errorf("invalid commit hash format: %s", s)
}

// isHex checks if a string contains only hexadecimal characters.
func isHex(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// ParseFileMode parses a Git file mode (e.g., "100644", "100755").
// Returns the mode as an integer.
func ParseFileMode(s string) (int, error) {
	mode, err := strconv.ParseInt(strings.TrimSpace(s), 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid file mode: %s", s)
	}
	return int(mode), nil
}

// IsEmptyLine checks if a line is empty or contains only whitespace.
func IsEmptyLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

// TrimPrefix removes a prefix from a string if present.
// Unlike strings.TrimPrefix, this also trims whitespace after removal.
func TrimPrefix(s, prefix string) string {
	s = strings.TrimPrefix(s, prefix)
	return strings.TrimSpace(s)
}

// SplitFields splits a line into fields, handling quoted strings.
// This is more sophisticated than strings.Fields() as it respects quotes.
func SplitFields(line string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		switch r {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
		case ' ', '\t':
			if inQuotes {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	return fields
}

// ExtractBetween extracts text between two delimiters.
// Returns empty string if delimiters not found.
func ExtractBetween(s, start, end string) string {
	startIdx := strings.Index(s, start)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(start)

	endIdx := strings.Index(s[startIdx:], end)
	if endIdx == -1 {
		return ""
	}

	return s[startIdx : startIdx+endIdx]
}
