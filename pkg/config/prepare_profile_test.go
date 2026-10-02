// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParsePrepareProfileDocument(t *testing.T) {
	for _, tt := range []struct {
		name    string
		doc     string
		isJSON  bool
		want    string
		present bool
		wantErr bool
	}{
		{name: "yaml familybook", doc: "branch:\n  prepareProfile: familybook-ent-v1\n", want: PrepareProfileFamilybookEntV1, present: true},
		{name: "yaml flow", doc: "branch:\n  prepareProfile: flow-taskchain-local-subprojects-v1\n", want: PrepareProfileFlowTaskchainLocalSubprojectsV1, present: true},
		{name: "yaml cargo", doc: "branch:\n  prepareProfile: cargo-workspace-v1\n", want: PrepareProfileCargoWorkspaceV1, present: true},
		{name: "json cargo", doc: `{"branch":{"prepareProfile":"cargo-workspace-v1"}}`, isJSON: true, want: PrepareProfileCargoWorkspaceV1, present: true},
		{name: "json", doc: `{"branch":{"prepareProfile":"familybook-ent-v1"}}`, isJSON: true, want: PrepareProfileFamilybookEntV1, present: true},
		{name: "absent", doc: "branch:\n  defaultBranch: main\n"},
		{name: "legacy yaml branch shorthand", doc: "branch: main\n"},
		{name: "legacy json branch shorthand", doc: `{"branch":"main"}`, isJSON: true},
		{name: "unsupported", doc: "branch:\n  prepareProfile: unknown\n", wantErr: true},
		{name: "yaml invalid type", doc: "branch:\n  prepareProfile: 1\n", wantErr: true},
		{name: "json invalid type", doc: `{"branch":{"prepareProfile":true}}`, isJSON: true, wantErr: true},
		{name: "yaml duplicate branch", doc: "branch: main\nbranch:\n  prepareProfile: familybook-ent-v1\n", wantErr: true},
		{name: "json duplicate branch", doc: `{"branch":"main","branch":{"prepareProfile":"familybook-ent-v1"}}`, isJSON: true, wantErr: true},
		{name: "yaml duplicate key", doc: "branch:\n  prepareProfile: familybook-ent-v1\n  prepareProfile: familybook-ent-v1\n", wantErr: true},
		{name: "json duplicate key", doc: `{"branch":{"prepareProfile":"familybook-ent-v1","prepareProfile":"familybook-ent-v1"}}`, isJSON: true, wantErr: true},
		{name: "yaml multiple documents", doc: "branch:\n  prepareProfile: familybook-ent-v1\n---\nbranch: main\n", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, present, err := ParsePrepareProfileDocument([]byte(tt.doc), tt.isJSON)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePrepareProfileDocument() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && (got != tt.want || present != tt.present) {
				t.Fatalf("ParsePrepareProfileDocument() = (%q, %v), want (%q, %v)", got, present, tt.want, tt.present)
			}
		})
	}
}

func TestPrepareProfileProjectConfigAndScope(t *testing.T) {
	input := "branch:\n  prepareProfile: flow-taskchain-local-subprojects-v1\n"
	var project ProjectConfig
	if err := yaml.Unmarshal([]byte(input), &project); err != nil {
		t.Fatal(err)
	}
	if project.Branch == nil || project.Branch.PrepareProfile != PrepareProfileFlowTaskchainLocalSubprojectsV1 {
		t.Fatalf("prepare profile was not preserved: %+v", project.Branch)
	}
	if err := NewValidator().ValidateProjectConfig(&project); err != nil {
		t.Fatalf("repository-root preparation profile did not validate: %v", err)
	}
	out, err := yaml.Marshal(&project)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "prepareProfile: flow-taskchain-local-subprojects-v1") {
		t.Fatalf("prepare profile missing from round trip: %s", out)
	}

	v := NewValidator()
	branch := &BranchConfig{PrepareProfile: PrepareProfileFamilybookEntV1}
	if err := v.ValidateProfile(&Profile{Name: "work", Branch: branch}); err == nil {
		t.Fatal("profile preparation profile accepted")
	}
	if err := v.ValidateConfig(&Config{Branch: branch}); err == nil {
		t.Fatal("recursive config preparation profile accepted")
	}
	if err := v.ValidateWorkspace(&Workspace{Branch: branch}, "repo"); err == nil {
		t.Fatal("workspace preparation profile accepted")
	}
	if err := v.ValidateProjectConfig(&ProjectConfig{Branch: &BranchConfig{PrepareProfile: "unknown"}}); err == nil {
		t.Fatal("unsupported repository-root preparation profile accepted")
	}
}
