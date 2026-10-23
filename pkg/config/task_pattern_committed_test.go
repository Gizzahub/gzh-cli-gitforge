// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const committedTestCommit = "0ee4886d16bb1076194d0ed188eef375609cc70b"

func committedFile(path, data string) CommittedConfigFile {
	return CommittedConfigFile{Path: path, Data: []byte(data)}
}

func TestLoadCommittedTaskPattern_ReadsRootDeclaration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got, err := LoadCommittedTaskPattern(root, committedTestCommit, []CommittedConfigFile{
		committedFile(".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: [dev/*]\n"),
		committedFile("README.md", "branch:\n  prepareProfile: ignored\n"),
	})
	if err != nil {
		t.Fatalf("LoadCommittedTaskPattern: %v", err)
	}
	if got.Source != filepath.Join(root, ".gz-git.yaml") {
		t.Fatalf("source = %q", got.Source)
	}
	if got.Commit != committedTestCommit {
		t.Fatalf("commit = %q", got.Commit)
	}
	if !slices.Equal(got.Patterns, []string{"dev/*"}) {
		t.Fatalf("patterns = %v", got.Patterns)
	}
	if !slices.Equal([]string(got.IntegrationBranch), []string{"develop"}) {
		t.Fatalf("integration branch = %v", got.IntegrationBranch)
	}
}

// TestLoadCommittedTaskPattern_RootOrderMatchesWorkingTree pins the same
// .yaml, .yml, .json precedence LoadRepoRootTaskPattern applies, and the
// JSON decoder for a .json-only commit.
func TestLoadCommittedTaskPattern_RootOrderMatchesWorkingTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got, err := LoadCommittedTaskPattern(root, committedTestCommit, []CommittedConfigFile{
		committedFile(".gz-git.json", `{"branch":{"taskPattern":["json/*"]}}`),
		committedFile(".gz-git.yml", "branch:\n  taskPattern: [yml/*]\n"),
	})
	if err != nil {
		t.Fatalf("LoadCommittedTaskPattern: %v", err)
	}
	if !slices.Equal(got.Patterns, []string{"yml/*"}) || got.Source != filepath.Join(root, ".gz-git.yml") {
		t.Fatalf(".yml must win over .json: patterns=%v source=%q", got.Patterns, got.Source)
	}

	got, err = LoadCommittedTaskPattern(root, committedTestCommit, []CommittedConfigFile{
		committedFile(".gz-git.json", `{"branch":{"taskPattern":["json/*"]}}`),
	})
	if err != nil {
		t.Fatalf("LoadCommittedTaskPattern json: %v", err)
	}
	if !slices.Equal(got.Patterns, []string{"json/*"}) {
		t.Fatalf("json patterns = %v", got.Patterns)
	}
}

func TestLoadCommittedTaskPattern_MissingDeclarationReportsFact(t *testing.T) {
	t.Parallel()
	got, err := LoadCommittedTaskPattern(t.TempDir(), committedTestCommit, nil)
	if err != nil {
		t.Fatalf("LoadCommittedTaskPattern: %v", err)
	}
	if got.Source != "" || !slices.Contains(got.Facts, factNoDeclaration) {
		t.Fatalf("source=%q facts=%v, want no declaration", got.Source, got.Facts)
	}
}

// TestLoadCommittedTaskPattern_RemovedKeyNamesFileAndCommit keeps the
// removed-key guard on the committed bytes: reading the release source
// instead of the working tree must not let a removed key through.
func TestLoadCommittedTaskPattern_RemovedKeyNamesFileAndCommit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := LoadCommittedTaskPattern(root, committedTestCommit, []CommittedConfigFile{
		committedFile(".gz-git.yaml", "branch:\n  taskPattern: [dev/*]\n  prepareProfile: pnpm-frozen-lockfile-v1\n"),
	})
	if err == nil {
		t.Fatal("expected removed-key error")
	}
	for _, want := range []string{"branch.prepareProfile was removed", filepath.Join(root, ".gz-git.yaml"), "at commit " + committedTestCommit[:12]} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestLoadCommittedTaskPattern_LiteralProtectedNameRejected(t *testing.T) {
	t.Parallel()
	_, err := LoadCommittedTaskPattern(t.TempDir(), committedTestCommit, []CommittedConfigFile{
		committedFile(".gz-git.yaml", "branch:\n  taskPattern: master\n"),
	})
	if err == nil || !strings.Contains(err.Error(), "master") || !strings.Contains(err.Error(), "at commit ") {
		t.Fatalf("err = %v, want literal protected name refusal at commit", err)
	}
}

func TestLoadCommittedTaskPattern_NestedDeclarationsReportedOrSkipped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got, err := LoadCommittedTaskPattern(root, committedTestCommit, []CommittedConfigFile{
		committedFile("sub/.gz-git.yaml", "branch:\n  taskPattern: [nested/*]\n"),
		committedFile("vendor/x/.gz-git.yaml", "branch:\n  taskPattern: [vendored/*]\n"),
		committedFile("node_modules/y/.gz-git.yaml", "branch:\n  prepareProfile: ignored\n"),
	})
	if err != nil {
		t.Fatalf("LoadCommittedTaskPattern: %v", err)
	}
	want := "ignored non-root taskPattern: " + filepath.Join(root, "sub", ".gz-git.yaml") + " at commit " + committedTestCommit[:12]
	if !slices.Contains(got.Facts, want) {
		t.Fatalf("facts = %v, want %q", got.Facts, want)
	}
	for _, fact := range got.Facts {
		if strings.Contains(fact, "vendor") || strings.Contains(fact, "node_modules") {
			t.Fatalf("skipped directory reported: %q", fact)
		}
	}
	if len(got.Patterns) != 0 {
		t.Fatalf("nested declaration must not apply: %v", got.Patterns)
	}
}

func TestLoadCommittedTaskPattern_RequiresCommit(t *testing.T) {
	t.Parallel()
	if _, err := LoadCommittedTaskPattern(t.TempDir(), " ", nil); err == nil {
		t.Fatal("expected error for empty commit")
	}
}
