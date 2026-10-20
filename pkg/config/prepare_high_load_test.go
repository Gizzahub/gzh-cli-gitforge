// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParsePrepareHighLoadDocumentStrict(t *testing.T) {
	for _, tc := range []struct {
		name, doc  string
		json, want bool
		bad        bool
	}{
		{"absent", "branch:\n  integrationBranch: develop\n", false, false, false},
		{"yaml true", "branch:\n  prepareHighLoad: true\n", false, true, false},
		{"json false", `{"branch":{"prepareHighLoad":false}}`, true, false, false},
		{"json null", `{"branch":{"prepareHighLoad":null}}`, true, false, true},
		{"json number", `{"branch":{"prepareHighLoad":1}}`, true, false, true},
		{"json duplicate branch", `{"branch":{},"branch":{"prepareHighLoad":true}}`, true, false, true},
		{"json duplicate key", `{"branch":{"prepareHighLoad":true,"prepareHighLoad":false}}`, true, false, true},
		{"null", "branch:\n  prepareHighLoad: null\n", false, false, true},
		{"string", "branch:\n  prepareHighLoad: 'true'\n", false, false, true},
		{"duplicate", "branch:\n  prepareHighLoad: true\n  prepareHighLoad: false\n", false, false, true},
		{"merge", "base: &b\n  prepareHighLoad: true\nbranch:\n  <<: *b\n", false, false, true},
		{"root merge", "base: &b\n  branch:\n    prepareHighLoad: true\n<<: *b\n", false, false, true},
		{"alias", "base: &b\n  prepareHighLoad: true\nbranch: *b\n", false, false, true},
		{"multi document", "branch:\n  prepareHighLoad: true\n---\nbranch:\n  prepareHighLoad: false\n", false, false, true},
		{"legacy shorthand", "branch: develop\n", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePrepareHighLoadDocument([]byte(tc.doc), tc.json)
			if tc.bad {
				if err == nil {
					t.Fatal("wanted strict parse failure")
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v", got, err)
			}
		})
	}
}

func TestPrepareHighLoadProjectScopeAndParentIsolation(t *testing.T) {
	var project ProjectConfig
	if err := yaml.Unmarshal([]byte("branch:\n  prepareHighLoad: true\n"), &project); err != nil {
		t.Fatal(err)
	}
	if project.Branch == nil || !project.Branch.PrepareHighLoad {
		t.Fatalf("project declaration lost: %#v", project.Branch)
	}
	if err := NewValidator().ValidateProjectConfig(&project); err != nil {
		t.Fatalf("root declaration rejected: %v", err)
	}
	roundTrip, err := yaml.Marshal(&project)
	if err != nil || !strings.Contains(string(roundTrip), "prepareHighLoad: true") {
		t.Fatalf("round trip = %q, %v", roundTrip, err)
	}
	v := NewValidator()
	branch := &BranchConfig{PrepareHighLoad: true}
	if err := v.ValidateProfile(&Profile{Name: "work", Branch: branch}); err == nil {
		t.Fatal("profile declaration accepted")
	}
	if err := v.ValidateConfig(&Config{Branch: branch}); err == nil {
		t.Fatal("recursive declaration accepted")
	}
	if err := v.ValidateWorkspace(&Workspace{Branch: branch}, "repo"); err == nil {
		t.Fatal("workspace declaration accepted")
	}
	child := &Config{}
	mergeParentConfig(child, &Config{Branch: branch})
	if child.Branch == nil || child.Branch.PrepareHighLoad {
		t.Fatalf("parent declaration propagated: %#v", child.Branch)
	}
}
