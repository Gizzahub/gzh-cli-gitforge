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
