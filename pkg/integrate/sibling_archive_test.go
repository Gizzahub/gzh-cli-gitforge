// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSiblingInstructionAliasesCannotEscapeArchive(t *testing.T) {
	for _, target := range []string{"AGENTS.md", "../outside", "missing.md"} {
		t.Run(target, func(t *testing.T) {
			var input bytes.Buffer
			writer := tar.NewWriter(&input)
			content := []byte("instructions\n")
			if err := writer.WriteHeader(&tar.Header{Name: "AGENTS.md", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(content))}); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write(content); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"CLAUDE.md", "GEMINI.md"} {
				if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeSymlink, Linkname: target, Mode: 0o777}); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			data, err := materializeSiblingAgentAliases(context.Background(), input.Bytes())
			if target != "AGENTS.md" {
				if err == nil {
					t.Fatal("unsafe instruction alias accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(t.TempDir(), "snapshot")
			if err := extractGitArchive(context.Background(), root, data); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"CLAUDE.md", "GEMINI.md"} {
				info, err := os.Lstat(filepath.Join(root, name))
				if err != nil || !info.Mode().IsRegular() {
					t.Fatalf("alias is not materialized: info=%v err=%v", info, err)
				}
				actual, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || !bytes.Equal(actual, content) {
					t.Fatalf("instruction contents changed: %q err=%v", actual, err)
				}
			}
		})
	}
}

func TestSiblingNestedInstructionAliasesMaterializeOwnDirectoryTarget(t *testing.T) {
	input := siblingInstructionArchive(
		t,
		siblingArchiveEntry{name: "one/AGENTS.md", body: []byte("one\n"), mode: 0o640},
		siblingArchiveEntry{name: "one/CLAUDE.md", typeflag: tar.TypeSymlink, linkname: "AGENTS.md"},
		siblingArchiveEntry{name: "two/AGENTS.md", body: []byte("two\n"), mode: 0o600},
		siblingArchiveEntry{name: "two/GEMINI.md", typeflag: tar.TypeSymlink, linkname: "AGENTS.md"},
	)
	data, err := materializeSiblingAgentAliases(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "snapshot")
	if err := extractGitArchive(context.Background(), root, data); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		name string
		body []byte
		mode os.FileMode
	}{
		{name: "one/CLAUDE.md", body: []byte("one\n"), mode: 0o640},
		{name: "two/GEMINI.md", body: []byte("two\n"), mode: 0o600},
	} {
		info, err := os.Lstat(filepath.Join(root, want.name))
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != want.mode {
			t.Fatalf("materialized %s: info=%v err=%v", want.name, info, err)
		}
		body, err := os.ReadFile(filepath.Join(root, want.name))
		if err != nil || !bytes.Equal(body, want.body) {
			t.Fatalf("materialized %s: body=%q err=%v", want.name, body, err)
		}
	}
}

func TestSiblingNestedInstructionAliasesRejectUnsafeOrShadowedForms(t *testing.T) {
	regular := siblingArchiveEntry{name: "nested/AGENTS.md", body: []byte("instructions\n"), mode: 0o644}
	for _, tc := range []struct {
		name    string
		entries []siblingArchiveEntry
	}{
		{"traversal target", []siblingArchiveEntry{regular, {name: "nested/CLAUDE.md", typeflag: tar.TypeSymlink, linkname: "../AGENTS.md"}}},
		{"absolute target", []siblingArchiveEntry{regular, {name: "nested/CLAUDE.md", typeflag: tar.TypeSymlink, linkname: "/AGENTS.md"}}},
		{"backslash target", []siblingArchiveEntry{regular, {name: "nested/CLAUDE.md", typeflag: tar.TypeSymlink, linkname: `dir\\AGENTS.md`}}},
		{"cross directory target", []siblingArchiveEntry{regular, {name: "nested/CLAUDE.md", typeflag: tar.TypeSymlink, linkname: "other/AGENTS.md"}}},
		{"missing target", []siblingArchiveEntry{{name: "nested/CLAUDE.md", typeflag: tar.TypeSymlink, linkname: "AGENTS.md"}}},
		{"nonregular target", []siblingArchiveEntry{{name: "nested/AGENTS.md", typeflag: tar.TypeDir}, {name: "nested/CLAUDE.md", typeflag: tar.TypeSymlink, linkname: "AGENTS.md"}}},
		{"nested agents alias", []siblingArchiveEntry{regular, {name: "nested/AGENTS.md", typeflag: tar.TypeSymlink, linkname: "AGENTS.md"}}},
		{"alias chain", []siblingArchiveEntry{regular, {name: "nested/CLAUDE.md", typeflag: tar.TypeSymlink, linkname: "AGENTS.md"}, {name: "nested/GEMINI.md", typeflag: tar.TypeSymlink, linkname: "CLAUDE.md"}}},
		{"duplicate target shadow", []siblingArchiveEntry{regular, regular}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := materializeSiblingAgentAliases(context.Background(), siblingInstructionArchive(t, tc.entries...)); err == nil {
				t.Fatal("unsafe nested instruction alias accepted")
			}
		})
	}
}

func TestSiblingInstructionAliasMaterializationCannotExpandArchiveBeyondLimit(t *testing.T) {
	input := siblingInstructionArchive(
		t,
		siblingArchiveEntry{name: "nested/AGENTS.md", body: bytes.Repeat([]byte("x"), 80), mode: 0o644},
		siblingArchiveEntry{name: "nested/CLAUDE.md", typeflag: tar.TypeSymlink, linkname: "AGENTS.md"},
		siblingArchiveEntry{name: "nested/GEMINI.md", typeflag: tar.TypeSymlink, linkname: "AGENTS.md"},
	)
	regular, err := collectSiblingRootInstructions(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rewriteSiblingArchiveWithLimit(context.Background(), input, regular, int64(len(input))); err == nil {
		t.Fatal("materialized archive exceeded the declared output limit")
	}
}

func TestSiblingInstructionSizeLimitAppliesOnlyToAliasTargetsAndRootCompatibility(t *testing.T) {
	overLimit := bytes.Repeat([]byte("x"), int(siblingInstructionFileLimit)+1)
	for _, name := range []string{"nested/AGENTS.md", "nested/CLAUDE.md", "nested/GEMINI.md"} {
		t.Run("unaliased "+name, func(t *testing.T) {
			if _, err := materializeSiblingAgentAliases(context.Background(), siblingInstructionArchive(
				t,
				siblingArchiveEntry{name: name, body: overLimit, mode: 0o644},
			)); err != nil {
				t.Fatalf("unaliased nested instruction file rejected: %v", err)
			}
		})
	}
	if _, err := materializeSiblingAgentAliases(context.Background(), siblingInstructionArchive(
		t,
		siblingArchiveEntry{name: "nested/AGENTS.md", body: overLimit, mode: 0o644},
		siblingArchiveEntry{name: "nested/CLAUDE.md", typeflag: tar.TypeSymlink, linkname: "AGENTS.md"},
	)); err == nil {
		t.Fatal("oversized nested alias target accepted")
	}
	if _, err := materializeSiblingAgentAliases(context.Background(), siblingInstructionArchive(
		t,
		siblingArchiveEntry{name: "AGENTS.md", body: overLimit, mode: 0o644},
	)); err == nil {
		t.Fatal("oversized root instruction file accepted")
	}
}

func TestProvisionSiblingMaterializesNestedInstructionAliasesAndReleases(t *testing.T) {
	fx, dep := siblingFixture(t)
	for _, file := range []struct {
		name, body string
		mode       os.FileMode
	}{
		{"one/AGENTS.md", "one\n", 0o640},
		{"two/AGENTS.md", "two\n", 0o600},
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(fx.Worktree, file.name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fx.Worktree, file.name), []byte(file.body), file.mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, alias := range []struct{ name string }{{"one/CLAUDE.md"}, {"two/GEMINI.md"}} {
		if err := os.Symlink("AGENTS.md", filepath.Join(fx.Worktree, alias.name)); err != nil {
			t.Fatal(err)
		}
	}
	runGitInTest(t, fx.Worktree, "add", "one", "two")
	runGitInTest(t, fx.Worktree, "commit", "-m", "nested instructions")
	runGitInTest(t, fx.Worktree, "push", fx.Origin, "HEAD:master")
	runGitInTest(t, fx.Clone, "fetch", fx.Origin, "refs/heads/master:refs/remotes/origin/master")
	runGitInTest(t, fx.Clone, "reset", "--hard", "origin/master")

	root := filepath.Join(t.TempDir(), "task")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	lease, err := ProvisionSibling(context.Background(), root, fx.Clone, dep)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Manifest == "" {
		t.Fatal("provision did not record a manifest")
	}
	for _, name := range []string{"one/CLAUDE.md", "two/GEMINI.md"} {
		info, err := os.Lstat(filepath.Join(lease.Destination, name))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("provisioned alias %s: info=%v err=%v", name, info, err)
		}
	}
	if err := ReleaseSibling(lease); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lease.Destination); !os.IsNotExist(err) {
		t.Fatalf("released destination remains: %v", err)
	}
}

type siblingArchiveEntry struct {
	name, linkname string
	body           []byte
	mode           int64
	typeflag       byte
}

func siblingInstructionArchive(t *testing.T, entries ...siblingArchiveEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		header := &tar.Header{Name: entry.name, Typeflag: typeflag, Linkname: entry.linkname, Mode: entry.mode}
		if typeflag == tar.TypeReg {
			header.Size = int64(len(entry.body))
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if typeflag == tar.TypeReg {
			if _, err := writer.Write(entry.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestSiblingExactPinDoesNotSwitchThePrimarySource(t *testing.T) {
	fx, dep := siblingFixture(t)
	pinned := strings.TrimSpace(runGitInTest(t, fx.Clone, "rev-parse", "HEAD"))
	writeFile(t, fx.Worktree, "payload.txt", "new tip\n")
	runGitInTest(t, fx.Worktree, "commit", "-am", "advance dependency")
	runGitInTest(t, fx.Worktree, "push", fx.Origin, "HEAD:master")
	runGitInTest(t, fx.Clone, "fetch", fx.Origin, "refs/heads/master:refs/remotes/origin/master")
	runGitInTest(t, fx.Clone, "reset", "--hard", "origin/master")
	tip := strings.TrimSpace(runGitInTest(t, fx.Clone, "rev-parse", "HEAD"))
	dep.Revision, dep.SourceBranch = pinned, ""
	root := filepath.Join(t.TempDir(), "task")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	lease, err := ProvisionSibling(context.Background(), root, fx.Clone, dep)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(lease.Destination, "payload.txt"))
	if err != nil || string(data) != "source\n" || lease.OID != pinned {
		t.Fatalf("pinned snapshot: data=%q oid=%s err=%v", data, lease.OID, err)
	}
	if got := strings.TrimSpace(runGitInTest(t, fx.Clone, "rev-parse", "HEAD")); got != tip || got == pinned {
		t.Fatalf("primary moved: got=%s tip=%s pin=%s", got, tip, pinned)
	}
	if err := ReleaseSibling(lease); err != nil {
		t.Fatal(err)
	}
}

func TestSiblingReleaseRefusesModeChangesAndMalformedIDs(t *testing.T) {
	fx, dep := siblingFixture(t)
	root := filepath.Join(t.TempDir(), "task")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	lease, err := ProvisionSibling(context.Background(), root, fx.Clone, dep)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(lease.Destination, "payload.txt")
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseSibling(lease); err == nil {
		t.Fatal("mode change must refuse release")
	}
	if err := os.Chmod(file, info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseSibling(lease); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../outside/aaaaaaaaaaaaaaaaaaaaa", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", ""} {
		if _, _, err := LoadPersistedSiblingLease(context.Background(), fx.Clone, id); err == nil {
			t.Fatalf("invalid lease ID accepted: %q", id)
		}
	}
}
