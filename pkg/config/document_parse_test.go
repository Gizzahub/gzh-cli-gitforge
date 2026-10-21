// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import "testing"

func TestRejectMultiDocumentYAML(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		wantErr    bool
	}{
		{"empty", "", false},
		{"single", "branch:\n  integrationBranch: master\n", false},
		{"leading marker", "---\nbranch: {}\n", false},
		{"second document", "branch: {}\n---\nbranch:\n  readiness: {}\n", true},
		{"trailing marker", "branch: {}\n---\n", true},
		{"syntax error", "branch: [\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := RejectMultiDocumentYAML([]byte(tc.data)); (err != nil) != tc.wantErr {
				t.Fatalf("RejectMultiDocumentYAML() error = %v, wantErr %t", err, tc.wantErr)
			}
		})
	}
}

func TestParseJSONObjectUnique(t *testing.T) {
	if got, err := parseJSONObjectUnique([]byte(`{"a": 1, "b": {"a": 2}}`)); err != nil || len(got) != 2 {
		t.Fatalf("parseJSONObjectUnique() = %v, %v", got, err)
	}
	for _, data := range []string{`{"a": 1, "a": 2}`, `[1]`, `{"a": 1} {}`} {
		if _, err := parseJSONObjectUnique([]byte(data)); err == nil {
			t.Fatalf("parseJSONObjectUnique(%s) accepted", data)
		}
	}
}
