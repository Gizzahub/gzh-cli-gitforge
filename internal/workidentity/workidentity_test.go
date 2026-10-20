// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package workidentity

import (
	"strings"
	"testing"
)

func TestParseGitConfig(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exitCode int
		stdout   string
		stderr   string
		want     string
		set      bool
		err      string
	}{
		{name: "one value", stdout: "alice\n", want: "alice", set: true},
		// Set but empty is not unset: the caller refuses it.
		{name: "empty value", stdout: "\n", set: true},
		{name: "unset key", exitCode: 1},
		{name: "unreadable config", exitCode: 1, stderr: "warning: unable to access '/x': Is a directory\n", err: "read global Git ce.workActor: exit 1: warning: unable to access"},
		{name: "other failure without stderr", exitCode: 128, err: "read global Git ce.workActor: exit 128"},
		{name: "two values", stdout: "alice\nbob\n", err: "must have exactly one value; found 2"},
		// Trimming every newline would read this as the single value alice.
		{name: "second value empty", stdout: "alice\n\n", err: "must have exactly one value; found 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, set, err := ParseGitConfig(ActorGitKey, tc.exitCode, tc.stdout, tc.stderr)
			if tc.err == "" {
				if err != nil || got != tc.want || set != tc.set {
					t.Fatalf("ParseGitConfig = %q, %v, %v; want %q, %v", got, set, err, tc.want, tc.set)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.err) || got != "" || set {
				t.Fatalf("ParseGitConfig = %q, %v, %v; want error containing %q", got, set, err, tc.err)
			}
		})
	}
}
