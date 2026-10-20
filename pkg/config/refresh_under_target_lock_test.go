// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseRefreshUnderTargetLockDocumentStrict(t *testing.T) {
	for _, tc := range []struct {
		name, doc  string
		json, want bool
		bad        bool
	}{
		{"absent", "branch:\n  integrationBranch: develop\n", false, false, false},
		{"yaml true", "branch:\n  refreshUnderTargetLock: true\n", false, true, false},
		{"yaml false", "branch:\n  refreshUnderTargetLock: false\n", false, false, false},
		{"json true", `{"branch":{"refreshUnderTargetLock":true}}`, true, true, false},
		{"json null", `{"branch":{"refreshUnderTargetLock":null}}`, true, false, true},
		{"json string", `{"branch":{"refreshUnderTargetLock":"true"}}`, true, false, true},
		{"json duplicate branch", `{"branch":{},"branch":{"refreshUnderTargetLock":true}}`, true, false, true},
		{"json duplicate key", `{"branch":{"refreshUnderTargetLock":true,"refreshUnderTargetLock":false}}`, true, false, true},
		{"null", "branch:\n  refreshUnderTargetLock: null\n", false, false, true},
		{"string", "branch:\n  refreshUnderTargetLock: 'true'\n", false, false, true},
		{"duplicate", "branch:\n  refreshUnderTargetLock: true\n  refreshUnderTargetLock: false\n", false, false, true},
		{"merge", "base: &b\n  refreshUnderTargetLock: true\nbranch:\n  <<: *b\n", false, false, true},
		{"root merge", "base: &b\n  branch:\n    refreshUnderTargetLock: true\n<<: *b\n", false, false, true},
		{"alias", "base: &b\n  refreshUnderTargetLock: true\nbranch: *b\n", false, false, true},
		{"multi document", "branch:\n  refreshUnderTargetLock: true\n---\nbranch: {}\n", false, false, true},
		{"legacy shorthand", "branch: develop\n", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRefreshUnderTargetLockDocument([]byte(tc.doc), tc.json)
			if tc.bad {
				if err == nil {
					t.Fatalf("wanted strict parse failure, got %v", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v", got, err)
			}
		})
	}
}

func TestRefreshUnderTargetLockProjectScopeAndParentIsolation(t *testing.T) {
	var project ProjectConfig
	if err := yaml.Unmarshal([]byte("branch:\n  refreshUnderTargetLock: true\n"), &project); err != nil {
		t.Fatal(err)
	}
	if project.Branch == nil || !project.Branch.RefreshUnderTargetLock {
		t.Fatalf("project declaration lost: %#v", project.Branch)
	}
	if err := NewValidator().ValidateProjectConfig(&project); err != nil {
		t.Fatalf("root declaration rejected: %v", err)
	}
	v := NewValidator()
	branch := &BranchConfig{RefreshUnderTargetLock: true}
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
	if child.Branch == nil || child.Branch.RefreshUnderTargetLock {
		t.Fatalf("parent declaration propagated: %#v", child.Branch)
	}
}
