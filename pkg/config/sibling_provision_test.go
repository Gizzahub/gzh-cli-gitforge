// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"strings"
	"testing"
)

func TestParseSiblingProvisionDocument(t *testing.T) {
	oid := "0123456789012345678901234567890123456789"
	valid := "branch:\n  siblingProvision:\n    version: 1\n    siblings:\n      - name: gitrump-ce\n        remote: ssh://git@example.test/gitrump-ce.git\n        path: ../gitrump-ce\n        sourceBranch: master\n        revision: source-tip\n      - name: pinned\n        remote: ssh://git@example.test/pinned.git\n        path: ../pinned\n        revision: " + oid + "\n"
	p, present, err := ParseSiblingProvisionDocument([]byte(valid), false)
	if err != nil || !present || len(p.Siblings) != 2 {
		t.Fatalf("parse = %#v, %v, %v", p, present, err)
	}
	for name, doc := range map[string]string{
		"traversal":          "branch:\n  siblingProvision:\n    version: 1\n    siblings: [{name: ce, remote: x, path: ../../ce, sourceBranch: master, revision: source-tip}]\n",
		"tip branch missing": "branch:\n  siblingProvision:\n    version: 1\n    siblings: [{name: ce, remote: x, path: ../ce, revision: source-tip}]\n",
		"pinned branch":      "branch:\n  siblingProvision:\n    version: 1\n    siblings: [{name: ce, remote: x, path: ../ce, sourceBranch: master, revision: " + oid + "}]\n",
		"duplicate":          "branch:\n  siblingProvision: {version: 1, siblings: []}\n  siblingProvision: {version: 1, siblings: []}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseSiblingProvisionDocument([]byte(doc), false); err == nil {
				t.Fatal("accepted invalid declaration")
			}
		})
	}
}

func TestSiblingContractRejectsUnknownAndDuplicateFields(t *testing.T) {
	entry := `{"name":"core","remote":"ssh://git@example.test/core.git","path":"../core","sourceBranch":"master","revision":"source-tip"}`
	validJSON := `{"branch":{"siblingProvision":{"version":1,"siblings":[` + entry + `]}}}`
	validYAML := "branch:\n  siblingProvision:\n    version: 1\n    siblings:\n      - name: core\n        remote: ssh://git@example.test/core.git\n        path: ../core\n        sourceBranch: master\n        revision: source-tip\n"
	for _, tc := range []struct {
		name, doc string
		json      bool
	}{
		{"JSON contract unknown", strings.Replace(validJSON, `"version":1`, `"version":1,"command":"unsafe"`, 1), true},
		{"JSON contract duplicate", strings.Replace(validJSON, `"version":1`, `"version":1,"version":1`, 1), true},
		{"JSON entry unknown", strings.Replace(validJSON, `"name":"core"`, `"name":"core","command":"unsafe"`, 1), true},
		{"JSON entry duplicate", strings.Replace(validJSON, `"name":"core"`, `"name":"core","name":"other"`, 1), true},
		{"YAML contract unknown", strings.Replace(validYAML, "    version: 1", "    version: 1\n    command: unsafe", 1), false},
		{"YAML entry duplicate", strings.Replace(validYAML, "      - name: core", "      - name: core\n        name: other", 1), false},
		{"YAML entry unknown", strings.Replace(validYAML, "      - name: core", "      - name: core\n        command: unsafe", 1), false},
		{"source path escape", strings.Replace(validJSON, `"name":"core"`, `"name":"../../outside"`, 1), true},
		{"duplicate destination", strings.Replace(validJSON, "["+entry+"]", "["+entry+","+strings.Replace(entry, `"name":"core"`, `"name":"other"`, 1)+"]", 1), true},
		{"credentials", strings.Replace(validJSON, "ssh://git@example.test/core.git", "https://token@example.test/core.git", 1), true},
		{"oversized", `{"branch":{"siblingProvision":{"version":1,"siblings":[` + strings.TrimSuffix(strings.Repeat(entry+",", 33), ",") + `]}}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := ParseSiblingProvisionDocument([]byte(tc.doc), tc.json); err == nil {
				t.Fatal("unsafe declaration accepted")
			}
		})
	}
}
