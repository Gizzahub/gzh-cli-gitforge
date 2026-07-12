// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/watch"
)

// TestJSONFormatter_EscapesSpecialChars verifies that paths/files containing
// characters that must be escaped in JSON (double quotes, backslashes — e.g.
// Windows paths) still produce valid JSON. The hand-built string version this
// replaced emitted broken JSON for such inputs.
func TestJSONFormatter_EscapesSpecialChars(t *testing.T) {
	f := &jsonFormatter{}

	event := watch.Event{
		Path:      `C:\repos\weird "name"\proj`,
		Type:      watch.EventTypeModified,
		Timestamp: time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC),
		Files:     []string{`dir\file "quoted".go`, "normal.go"},
	}

	out := f.Format(event)

	if !json.Valid([]byte(out)) {
		t.Fatalf("formatter emitted invalid JSON: %q", out)
	}

	var decoded watchEventJSON
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("unmarshal round-trip failed: %v", err)
	}
	if decoded.Path != event.Path {
		t.Errorf("path not preserved: got %q, want %q", decoded.Path, event.Path)
	}
	if len(decoded.Files) != len(event.Files) || decoded.Files[0] != event.Files[0] {
		t.Errorf("files not preserved: got %v, want %v", decoded.Files, event.Files)
	}
}

// TestJSONFormatter_NilFilesIsEmptyArray guards that a nil Files slice serializes
// as [] (not null), so downstream consumers can always iterate the array.
func TestJSONFormatter_NilFilesIsEmptyArray(t *testing.T) {
	f := &jsonFormatter{}

	out := f.Format(watch.Event{
		Path:      "/repo",
		Type:      watch.EventTypeClean,
		Timestamp: time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC),
		Files:     nil,
	})

	if !json.Valid([]byte(out)) {
		t.Fatalf("invalid JSON: %q", out)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if string(decoded["files"]) != "[]" {
		t.Errorf("nil files should serialize as [], got %s", decoded["files"])
	}
}

// TestWatchProgressSuppressedForMachineFormats locks the gate used by the watch
// header/footer: machine formats (json/llm) must not emit progress chrome to
// stdout, while human formats still do.
func TestWatchProgressSuppressedForMachineFormats(t *testing.T) {
	cases := []struct {
		format string
		want   bool
	}{
		{"default", true},
		{"compact", true},
		{"json", false},
		{"llm", false},
	}
	for _, tc := range cases {
		if got := shouldShowProgress(tc.format, false); got != tc.want {
			t.Errorf("shouldShowProgress(%q, false) = %v, want %v", tc.format, got, tc.want)
		}
	}
}
