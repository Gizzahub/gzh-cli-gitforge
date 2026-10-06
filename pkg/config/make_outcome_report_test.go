// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseMakeOutcomeReportDocumentStrictV1(t *testing.T) {
	validYAML := "metadata: retained\nbranch:\n  makeOutcomeReport:\n    version: 1\n    targets: [check, lint]\n"
	validJSON := `{"metadata":"retained","branch":{"makeOutcomeReport":{"version":1,"targets":["check","lint"]}}}`

	for _, tt := range []struct {
		name   string
		doc    string
		isJSON bool
		want   *MakeOutcomeReport
		ok     bool
	}{
		{name: "yaml", doc: validYAML, want: &MakeOutcomeReport{Version: 1, Targets: []string{"check", "lint"}}, ok: true},
		{name: "json", doc: validJSON, isJSON: true, want: &MakeOutcomeReport{Version: 1, Targets: []string{"check", "lint"}}, ok: true},
		{name: "absent keeps unrelated config valid", doc: "branch:\n  defaultBranch: main\nmetadata: retained\n", ok: true},
		{name: "null", doc: "branch:\n  makeOutcomeReport: null\n"},
		{name: "empty targets", doc: "branch:\n  makeOutcomeReport:\n    version: 1\n    targets: []\n"},
		{name: "wrong target", doc: "branch:\n  makeOutcomeReport:\n    version: 1\n    targets: [test]\n"},
		{name: "duplicate target", doc: "branch:\n  makeOutcomeReport:\n    version: 1\n    targets: [check, check]\n"},
		{name: "unknown field", doc: "branch:\n  makeOutcomeReport:\n    version: 1\n    targets: [check]\n    command: make check\n"},
		{name: "yaml duplicate", doc: "branch:\n  makeOutcomeReport:\n    version: 1\n    version: 1\n    targets: [check]\n"},
		{name: "yaml alias", doc: "report: &report\n  version: 1\n  targets: [check]\nbranch:\n  makeOutcomeReport: *report\n"},
		{name: "yaml merge", doc: "base: &base\n  version: 1\n  targets: [check]\nbranch:\n  makeOutcomeReport:\n    <<: *base\n"},
		{name: "aliased branch hides declaration", doc: "branchDefinition: &branchDefinition\n  makeOutcomeReport:\n    version: 1\n    targets: [check]\nbranch: *branchDefinition\n"},
		{name: "branch merge hides declaration", doc: "branchBase: &branchBase\n  makeOutcomeReport:\n    version: 1\n    targets: [check]\nbranch:\n  <<: *branchBase\n"},
		{name: "root merge hides branch declaration", doc: "rootBase: &rootBase\n  branch:\n    makeOutcomeReport:\n      version: 1\n      targets: [check]\n<<: *rootBase\n"},
		{name: "root inline merge hides branch declaration", doc: "<<: {branch: {makeOutcomeReport: {version: 1, targets: [check]}}}\n"},
		{name: "root inline merge sequence hides branch declaration", doc: "<<:\n  - {branch: {makeOutcomeReport: {version: 1, targets: [check]}}}\n"},
		{name: "branch inline merge hides declaration", doc: "branch:\n  <<: {makeOutcomeReport: {version: 1, targets: [check]}}\n"},
		{name: "branch inline merge sequence hides declaration", doc: "branch:\n  <<:\n    - {makeOutcomeReport: {version: 1, targets: [check]}}\n"},
		{name: "legacy aliased branch remains absent", doc: "branchDefinition: &branchDefinition\n  defaultBranch: main\nbranch: *branchDefinition\n", ok: true},
		{name: "legacy branch merge remains absent", doc: "branchBase: &branchBase\n  defaultBranch: main\nbranch:\n  <<: *branchBase\n", ok: true},
		{name: "legacy root merge remains absent", doc: "rootBase: &rootBase\n  branch:\n    defaultBranch: main\n<<: *rootBase\n", ok: true},
		{name: "legacy root inline merge remains absent", doc: "<<: {branch: {defaultBranch: main}}\n", ok: true},
		{name: "legacy root inline merge sequence remains absent", doc: "<<:\n  - {branch: {defaultBranch: main}}\n", ok: true},
		{name: "yaml multi document", doc: validYAML + "---\nmetadata: second\n"},
		{name: "json duplicate contract key", doc: `{"branch":{"makeOutcomeReport":{"version":1,"version":1,"targets":["check"]}}}`, isJSON: true},
		{name: "json duplicate branch key", doc: `{"branch":{"makeOutcomeReport":{"version":1,"targets":["check"]}},"branch":{}}`, isJSON: true},
		{name: "json trailing data", doc: validJSON + ` {}`, isJSON: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMakeOutcomeReportDocument([]byte(tt.doc), tt.isJSON)
			if tt.ok != (err == nil) {
				t.Fatalf("ParseMakeOutcomeReportDocument() error = %v, want ok=%v", err, tt.ok)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("report = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseMakeOutcomeReportDocumentRejectsAliasMergeCycle(t *testing.T) {
	doc := "branchBase: &branchBase\n  <<: *branchBase\nbranch:\n  <<: *branchBase\n"
	if _, err := ParseMakeOutcomeReportDocument([]byte(doc), false); err == nil {
		t.Fatal("alias/merge cycle was accepted")
	}
}

func TestParseMakeOutcomeReportDocumentAcceptsUnrelatedYAMLAnchors(t *testing.T) {
	doc := "metadata: &metadata\n  owner: platform\nmetadataCopy: *metadata\nbranch:\n  defaultBranch: &defaultBranch main\n  protectedBranches: [*defaultBranch]\n  makeOutcomeReport:\n    version: 1\n    targets: [check]\n"
	got, err := ParseMakeOutcomeReportDocument([]byte(doc), false)
	if err != nil {
		t.Fatalf("unrelated YAML anchors rejected: %v", err)
	}
	want := &MakeOutcomeReport{Version: 1, Targets: []string{"check"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("report = %#v, want %#v", got, want)
	}
}

func TestMakeOutcomeReportProjectRoundTripAndScope(t *testing.T) {
	report := &MakeOutcomeReport{Version: 1, Targets: []string{"check", "lint"}}
	var project ProjectConfig
	if err := yaml.Unmarshal([]byte("branch:\n  makeOutcomeReport:\n    version: 1\n    targets: [check, lint]\n"), &project); err != nil {
		t.Fatal(err)
	}
	if project.Branch == nil || !reflect.DeepEqual(project.Branch.MakeOutcomeReport, report) {
		t.Fatalf("report was not preserved: %+v", project.Branch)
	}
	if err := NewValidator().ValidateProjectConfig(&project); err != nil {
		t.Fatalf("report did not validate: %v", err)
	}
	out, err := yaml.Marshal(&project)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "makeOutcomeReport:") {
		t.Fatalf("report missing from project round trip:\n%s", out)
	}

	v := NewValidator()
	if err := v.ValidateConfig(&Config{Branch: &BranchConfig{MakeOutcomeReport: report}}); err == nil {
		t.Fatal("parent/global config make outcome report accepted")
	}
	if err := v.ValidateWorkspace(&Workspace{Branch: &BranchConfig{MakeOutcomeReport: report}}, "repo"); err == nil {
		t.Fatal("workspace make outcome report accepted")
	}
	if err := v.ValidateProfile(&Profile{Name: "work", Branch: &BranchConfig{MakeOutcomeReport: report}}); err == nil {
		t.Fatal("profile make outcome report accepted")
	}
}

func TestLoadRepoRootTaskPatternStrictlyLoadsMakeOutcomeReport(t *testing.T) {
	for _, tt := range []struct {
		name string
		file string
		body string
		want *MakeOutcomeReport
	}{
		{
			name: "yaml",
			file: ".gz-git.yaml",
			body: "branch:\n  taskPattern: feat/*\n  makeOutcomeReport:\n    version: 1\n    targets: [check]\n",
			want: &MakeOutcomeReport{Version: 1, Targets: []string{"check"}},
		},
		{
			name: "json",
			file: ".gz-git.json",
			body: `{"branch":{"taskPattern":["feat/*"],"makeOutcomeReport":{"version":1,"targets":["lint"]}}}`,
			want: &MakeOutcomeReport{Version: 1, Targets: []string{"lint"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, tt.file), []byte(tt.body), 0o600); err != nil {
				t.Fatal(err)
			}
			decl, err := LoadRepoRootTaskPattern(root)
			if err != nil {
				t.Fatalf("LoadRepoRootTaskPattern: %v", err)
			}
			if !reflect.DeepEqual(decl.MakeOutcomeReport, tt.want) {
				t.Fatalf("MakeOutcomeReport = %#v, want %#v", decl.MakeOutcomeReport, tt.want)
			}
		})
	}

	root := t.TempDir()
	body := "branch:\n  makeOutcomeReport:\n    version: 1\n    targets: [check]\n    command: make check\n"
	if err := os.WriteFile(filepath.Join(root, ".gz-git.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRepoRootTaskPattern(root); err == nil {
		t.Fatal("LoadRepoRootTaskPattern accepted malformed makeOutcomeReport")
	}
}

func TestMakeOutcomeReportIncludes(t *testing.T) {
	var absent *MakeOutcomeReport
	if absent.Includes("check") {
		t.Fatal("nil report included a target")
	}
	report := &MakeOutcomeReport{Targets: []string{"check"}}
	if !report.Includes("check") || report.Includes("lint") {
		t.Fatal("Includes did not use exact target membership")
	}
}
