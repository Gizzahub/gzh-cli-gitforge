// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestReadinessPreparationStrictAndDigest(t *testing.T) {
	base := Readiness{Version: 1, Runner: ".gz-git/readiness/check"}
	legacy, err := CanonicalReadiness(base)
	if err != nil || legacy != `{"version":1,"runner":".gz-git/readiness/check"}` {
		t.Fatalf("legacy digest input changed: %s %v", legacy, err)
	}
	base.PrepareProfile = NDStackCheckedTestsV1
	prepared, err := CanonicalReadiness(base)
	if err != nil || prepared == legacy {
		t.Fatalf("profile missing from contract digest: %s %v", prepared, err)
	}
	for _, tc := range []struct {
		doc   string
		json  bool
		valid bool
	}{
		{`{"branch":{"readiness":{"version":1,"runner":".gz-git/readiness/check","prepareProfile":"nd-stack-checked-tests-v1"}}}`, true, true},
		{`{"branch":{"readiness":{"version":1,"runner":".gz-git/readiness/check","prepareProfile":null}}}`, true, false},
		{`{"branch":{"readiness":{"version":1,"runner":".gz-git/readiness/check","prepareProfile":""}}}`, true, false},
		{`{"branch":{"readiness":{"version":1,"runner":".gz-git/readiness/check","prepareProfile":"shell"}}}`, true, false},
		{"branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    prepareProfile: nd-stack-checked-tests-v1\n", false, true},
		{"branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    prepareProfile: null\n", false, false},
		{"branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    prepareProfile: false\n", false, false},
		{"branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    prepareProfile: nd-stack-checked-tests-v1\n    prepareProfile: nd-stack-checked-tests-v1\n", false, false},
	} {
		_, present, err := ParseReadinessDocument([]byte(tc.doc), tc.json)
		if (err == nil && present) != tc.valid {
			t.Fatalf("valid=%v got present=%v err=%v for %s", tc.valid, present, err, tc.doc)
		}
	}
}

func TestParseReadinessDocument_StrictV1(t *testing.T) {
	for _, tt := range []struct {
		name string
		doc  string
		ok   bool
	}{
		{"valid", "branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n", true},
		{"unknown", "branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    args: [oops]\n", false},
		{"wrong version", "branch:\n  readiness:\n    version: 2\n    runner: .gz-git/readiness/check\n", false},
		{"traversal", "branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/../check\n", false},
		{"outside", "branch:\n  readiness:\n    version: 1\n    runner: script/check\n", false},
		{"duplicate yaml", "branch:\n  readiness:\n    version: 1\n    version: 1\n    runner: .gz-git/readiness/check\n", false},
		{"backslash", "branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness\\check\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, present, err := ParseReadinessDocument([]byte(tt.doc), false)
			if tt.ok && (err != nil || !present) {
				t.Fatalf("ParseReadinessDocument() = %v, %v", got, err)
			}
			if !tt.ok && err == nil {
				t.Fatal("ParseReadinessDocument() unexpectedly succeeded")
			}
		})
	}
}

func TestParseReadinessDocument_RejectsDuplicateJSON(t *testing.T) {
	for _, doc := range []string{
		`{"branch":"develop","branch":{"readiness":{"version":1,"runner":".gz-git/readiness/check"}}}`,
		`{"branch":{"readiness":{"version":1,"runner":".gz-git/readiness/check"},"readiness":{"version":1,"runner":".gz-git/readiness/check"}}}`,
		`{"branch":{"readiness":{"version":1,"version":1,"runner":".gz-git/readiness/check"}}}`,
	} {
		if _, _, err := ParseReadinessDocument([]byte(doc), true); err == nil {
			t.Fatalf("duplicate JSON readiness path accepted: %s", doc)
		}
	}
}

func TestParseReadinessDocument_RejectsDuplicateYAMLPath(t *testing.T) {
	for _, doc := range []string{
		"branch: develop\nbranch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n",
		"branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n",
	} {
		if _, _, err := ParseReadinessDocument([]byte(doc), false); err == nil {
			t.Fatalf("duplicate YAML readiness path accepted: %s", doc)
		}
	}
}

func TestParseReadinessDocument_LegacyBranchShorthandIsAbsent(t *testing.T) {
	for _, tt := range []struct {
		name   string
		doc    string
		isJSON bool
	}{
		{name: "yaml", doc: "branch: develop\n"},
		{name: "json", doc: `{"branch":"develop"}`, isJSON: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, present, err := ParseReadinessDocument([]byte(tt.doc), tt.isJSON)
			if err != nil || present {
				t.Fatalf("legacy shorthand changed behavior: present=%v err=%v", present, err)
			}
		})
	}
}

func TestProjectConfigReadinessRoundTripAndSchema(t *testing.T) {
	input := "branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n"
	var project ProjectConfig
	if err := yaml.Unmarshal([]byte(input), &project); err != nil {
		t.Fatal(err)
	}
	if project.Branch == nil || project.Branch.Readiness == nil {
		t.Fatalf("readiness was not preserved: %+v", project.Branch)
	}
	if err := NewValidator().ValidateProjectConfig(&project); err != nil {
		t.Fatalf("readiness did not validate: %v", err)
	}
	out, err := yaml.Marshal(&project)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "readiness:") || !strings.Contains(ExampleConfig, "runner: .gz-git/readiness/check") {
		t.Fatalf("readiness missing from round trip or schema:\n%s", out)
	}
}

func TestReadinessRejectedOutsideProjectRoot(t *testing.T) {
	readiness := &Readiness{Version: 1, Runner: ".gz-git/readiness/check"}
	v := NewValidator()
	if err := v.ValidateConfig(&Config{Branch: &BranchConfig{Readiness: readiness}}); err == nil {
		t.Fatal("recursive config readiness accepted")
	}
	if err := v.ValidateWorkspace(&Workspace{Branch: &BranchConfig{Readiness: readiness}}, "repo"); err == nil {
		t.Fatal("workspace readiness accepted")
	}
	if err := v.ValidateProfile(&Profile{Name: "work", Branch: &BranchConfig{Readiness: readiness}}); err == nil {
		t.Fatal("profile readiness accepted")
	}
}

func TestReadinessTimeoutContract(t *testing.T) {
	base := Readiness{Version: 1, Runner: ".gz-git/readiness/check"}
	budget, err := base.ExecutionTimeout()
	if err != nil || budget != 15*time.Minute {
		t.Fatalf("default budget = %v, %v", budget, err)
	}
	legacy, err := ReadinessDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"1ns", "50ms", "30m", "2h"} {
		t.Run(value, func(t *testing.T) {
			want, err := time.ParseDuration(value)
			if err != nil {
				t.Fatal(err)
			}
			contract := base
			contract.Timeout = value
			budget, err := contract.ExecutionTimeout()
			if err != nil || budget != want {
				t.Fatalf("budget = %v, %v", budget, err)
			}
			digest, err := ReadinessDigest(contract)
			if err != nil || digest == legacy {
				t.Fatalf("timeout missing from digest: %s, %v", digest, err)
			}
			contract.PrepareProfile = NDStackCheckedTestsV1
			for _, isJSON := range []bool{false, true} {
				project := map[string]any{"branch": map[string]any{"readiness": contract}}
				var doc []byte
				if isJSON {
					doc, err = json.Marshal(project)
				} else {
					doc, err = yaml.Marshal(project)
				}
				if err != nil {
					t.Fatal(err)
				}
				got, present, err := ParseReadinessDocument(doc, isJSON)
				if err != nil || !present || got != contract {
					t.Fatalf("roundtrip JSON=%v: %+v, %v, %v", isJSON, got, present, err)
				}
			}
		})
	}
	changed := base
	changed.Timeout = "15m"
	digest, err := ReadinessDigest(changed)
	if err != nil || digest == legacy {
		t.Fatalf("explicit timeout must change digest: %s, %v", digest, err)
	}
}

func TestReadinessTimeoutRejectsMalformedDocuments(t *testing.T) {
	for _, value := range []string{`""`, `null`, `42`, `true`, `[]`, `{}`, `" 1m"`, `"1m "`, `"0s"`, `"-1s"`, `"tomorrow"`, `"999999999999999999999h"`, `"2h1ns"`} {
		for _, isJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/%s", isJSON, value), func(t *testing.T) {
				doc := fmt.Sprintf("branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    timeout: %s\n", value)
				if isJSON {
					doc = fmt.Sprintf(`{"branch":{"readiness":{"version":1,"runner":".gz-git/readiness/check","timeout":%s}}}`, value)
				}
				if _, _, err := ParseReadinessDocument([]byte(doc), isJSON); err == nil {
					t.Fatalf("accepted invalid timeout: %s", doc)
				}
			})
		}
	}
	for _, tc := range []struct {
		doc    string
		isJSON bool
	}{
		{"branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    timeout: 1m\n    timeout: 1m\n", false},
		{`{"branch":{"readiness":{"version":1,"runner":".gz-git/readiness/check","timeout":"1m","timeout":"1m"}}}`, true},
		{"branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    timeoutSeconds: 60\n", false},
		{`{"branch":{"readiness":{"version":1,"runner":".gz-git/readiness/check","timeoutSeconds":60}}}`, true},
	} {
		if _, _, err := ParseReadinessDocument([]byte(tc.doc), tc.isJSON); err == nil {
			t.Fatalf("accepted duplicate or unknown field: %s", tc.doc)
		}
	}
}

func TestLoadProjectConfigRejectsInvalidTimeoutBeforeDecoding(t *testing.T) {
	for _, ext := range []string{"yaml", "yml", "json"} {
		for _, value := range []string{`null`, `""`, `42`, `"0s"`, `" 1m"`, `"2h1ns"`, `"30m"`} {
			t.Run(ext+"/"+value, func(t *testing.T) {
				dir := t.TempDir()
				t.Chdir(dir)
				doc := fmt.Sprintf("branch:\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    timeout: %s\n", value)
				if ext == "json" {
					doc = fmt.Sprintf(`{"branch":{"readiness":{"version":1,"runner":".gz-git/readiness/check","timeout":%s}}}`, value)
				}
				if err := os.WriteFile(filepath.Join(dir, ".gz-git."+ext), []byte(doc), 0o600); err != nil {
					t.Fatal(err)
				}
				manager := &Manager{validator: NewValidator()}
				got, err := manager.LoadProjectConfig()
				if value != `"30m"` {
					if err == nil {
						t.Fatalf("invalid timeout accepted: %+v", got)
					}
					return
				}
				if err != nil || got == nil || got.Branch == nil || got.Branch.Readiness == nil || got.Branch.Readiness.Timeout != "30m" {
					t.Fatalf("valid timeout not preserved: %+v %v", got, err)
				}
			})
		}
	}
}
