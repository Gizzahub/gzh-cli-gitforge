// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertRemovedKeyError(t *testing.T, err error, key, path string) {
	t.Helper()
	if err == nil {
		t.Fatalf("load accepted removed %s", key)
	}
	parts := strings.Split(key, ".")
	reason := removedKeyReasons[parts[len(parts)-2]][parts[len(parts)-1]]
	if reason == "" {
		t.Fatalf("%s is not a removed key", key)
	}
	for _, want := range []string{
		key + " was removed: " + reason,
		"delete it from " + path,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}
}

// TestRepoRootRemovedKeysFailTheLoad pins the no-fallback removal of the
// legacy make gate: a repository-root declaration that still carries one of
// its keys fails with an error naming the key and the file to delete it from,
// in both YAML and JSON, instead of being silently ignored.
func TestRepoRootRemovedKeysFailTheLoad(t *testing.T) {
	cases := []struct {
		name, file, body, key string
	}{
		{"yaml makeTimeout", ".gz-git.yaml", "branch:\n  integrationBranch: master\n  makeTimeout: 90m\n", "branch.makeTimeout"},
		{"yml makeTimeout", ".gz-git.yml", "branch:\n  makeTimeout: 67m\n", "branch.makeTimeout"},
		{"json makeTimeout", ".gz-git.json", `{"branch": {"integrationBranch": "master", "makeTimeout": "45m"}}`, "branch.makeTimeout"},
		{"json numeric makeTimeout", ".gz-git.json", `{"branch": {"makeTimeout": 30}}`, "branch.makeTimeout"},
		{"yaml makeOutcomeReport", ".gz-git.yaml", "branch:\n  makeOutcomeReport:\n    version: 1\n    targets: [check]\n", "branch.makeOutcomeReport"},
		{"json makeOutcomeReport", ".gz-git.json", `{"branch": {"makeOutcomeReport": {"version": 1, "targets": ["lint"]}}}`, "branch.makeOutcomeReport"},
		{"yaml prepareProfile", ".gz-git.yaml", "branch:\n  prepareProfile: familybook-ent-v1\n", "branch.prepareProfile"},
		{"json prepareProfile", ".gz-git.json", `{"branch": {"prepareProfile": "cargo-workspace-v1"}}`, "branch.prepareProfile"},
		{"yaml merge supplies makeTimeout", ".gz-git.yaml", "base: &base\n  makeTimeout: 90m\nbranch:\n  <<: *base\n", "branch.makeTimeout"},
		{"yaml readiness", ".gz-git.yaml", "branch:\n  integrationBranch: master\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n", "branch.readiness"},
		{"yaml readiness with only prepareProfile", ".gz-git.yaml", "branch:\n  readiness:\n    prepareProfile: infra-ops-python-uv-v1\n", "branch.readiness"},
		{"yaml empty readiness", ".gz-git.yaml", "branch:\n  readiness: {}\n", "branch.readiness"},
		{"yaml null readiness", ".gz-git.yaml", "branch:\n  readiness:\n", "branch.readiness"},
		{"json readiness", ".gz-git.json", `{"branch": {"readiness": {"version": 1}}}`, "branch.readiness"},
		{"yaml prepareHighLoad", ".gz-git.yaml", "branch:\n  prepareHighLoad: true\n", "branch.prepareHighLoad"},
		{"json prepareHighLoad", ".gz-git.json", `{"branch": {"prepareHighLoad": false}}`, "branch.prepareHighLoad"},
		{"yaml resourceHighLoad", ".gz-git.yaml", "branch:\n  resourceHighLoad: true\n", "branch.resourceHighLoad"},
		{"yaml lockWait", ".gz-git.yaml", "branch:\n  lockWait: 1m\n", "branch.lockWait"},
		{"json resourceHighLoad", ".gz-git.json", `{"branch": {"resourceHighLoad": true}}`, "branch.resourceHighLoad"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := writeConfigFile(t, root, tc.file, tc.body)
			_, err := LoadRepoRootTaskPattern(root)
			assertRemovedKeyError(t, err, tc.key, path)
		})
	}
}

func TestControllerConfigRemovedKeysFailTheLoad(t *testing.T) {
	cases := []struct {
		name, workspace, key string
	}{
		{"integration prepareProfile", "    branch:\n      integrationBranch: develop\n    integration:\n      prepareProfile: familybook-ent-v1\n", "workspaces.engine.integration.prepareProfile"},
		{"branch makeTimeout", "    branch:\n      integrationBranch: develop\n      makeTimeout: 90m\n    integration: {}\n", "workspaces.engine.branch.makeTimeout"},
		{"branch prepareProfile", "    branch:\n      integrationBranch: develop\n      prepareProfile: familybook-ent-v1\n    integration: {}\n", "workspaces.engine.branch.prepareProfile"},
		{"integration readinessApproval", "    branch:\n      integrationBranch: develop\n    integration:\n      readinessApproval:\n        approvers: [ops]\n", "workspaces.engine.integration.readinessApproval"},
		{"branch readiness", "    branch:\n      integrationBranch: develop\n      readiness:\n        version: 1\n    integration: {}\n", "workspaces.engine.branch.readiness"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeConfigFile(t, dir, "controller.yaml", "workspaces:\n  engine:\n    url: https://example.invalid/engine.git\n"+tc.workspace)
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				t.Fatal(err)
			}
			_, err = LoadControllerConfig(path)
			assertRemovedKeyError(t, err, tc.key, resolved)
		})
	}
}

func TestRecursiveConfigRemovedKeyFailsTheLoad(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, ".gz-git.yaml", "branch:\n  makeTimeout: 90m\n")
	resolved, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = LoadConfigRecursive(dir, ".gz-git.yaml")
	assertRemovedKeyError(t, err, "branch.makeTimeout", resolved)
}

func TestProjectConfigFileRemovedKeyFailsTheLoad(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, ".gz-git.json", `{"branch": {"makeTimeout": "90m"}}`)
	var cfg ProjectConfig
	assertRemovedKeyError(t, unmarshalFile(path, &cfg), "branch.makeTimeout", path)
}

func TestGlobalConfigReadinessApprovalFailsTheLoad(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yaml", "integration:\n  readinessApproval:\n    approvers: [ops]\n")
	var cfg GlobalConfig
	assertRemovedKeyError(t, unmarshalFile(path, &cfg), "integration.readinessApproval", path)
}

// TestPreviouslyUnguardedLoadersRejectRemovedKeys covers the loaders that
// decoded .gz-git.yaml without the removed-key guard before they were routed
// through ReadConfigFile and RejectRemovedKeys.
func TestPreviouslyUnguardedLoadersRejectRemovedKeys(t *testing.T) {
	for _, key := range []string{"readiness", "prepareHighLoad", "resourceHighLoad"} {
		t.Run("workspace access "+key, func(t *testing.T) {
			root := t.TempDir()
			repo := filepath.Join(root, "repo")
			if err := os.MkdirAll(repo, 0o750); err != nil {
				t.Fatal(err)
			}
			path := writeConfigFile(t, root, ".gz-git.yaml", "branch:\n  "+key+": true\nworkspaces:\n  repo:\n    path: repo\n")
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = WorkspacePushAccess(repo)
			assertRemovedKeyError(t, err, "branch."+key, resolved)
		})
		t.Run("workspace clones "+key, func(t *testing.T) {
			root := t.TempDir()
			path := writeConfigFile(t, root, ".gz-git.yaml", "workspaces:\n  child:\n    branch:\n      "+key+": true\n")
			_, err := DeclaredWorkspaceClones(root)
			assertRemovedKeyError(t, err, "workspaces.child.branch."+key, path)
		})
	}
	t.Run("read config file integration readinessApproval", func(t *testing.T) {
		dir := t.TempDir()
		path := writeConfigFile(t, dir, ".gz-git.yaml", "integration:\n  readinessApproval: {}\n")
		_, err := ReadConfigFile(path)
		assertRemovedKeyError(t, err, "integration.readinessApproval", path)
	})
}
