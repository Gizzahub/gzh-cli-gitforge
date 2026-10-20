// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestStatusCleanExceptUntrackedRejectsMalformedOrDirtyRecords(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		status string
		clean  bool
	}{
		{name: "empty", clean: true},
		{name: "allowed untracked record", status: "?? .ce/audit/events.jsonl\x00", clean: true},
		{name: "missing terminal NUL", status: "?? .ce/audit/events.jsonl"},
		{name: "double terminal NUL", status: "?? .ce/audit/events.jsonl\x00\x00"},
		{name: "short record", status: "?? \x00"},
		{name: "rename two path record", status: "R  AGENTS.md\x00CLAUDE.md\x00"},
		{name: "staged allowed path", status: "A  .ce/audit/events.jsonl\x00"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := statusCleanExceptUntracked(test.status, isCELogRuntimeArtifactPath); got != test.clean {
				t.Fatalf("statusCleanExceptUntracked(%q) = %t, want %t", test.status, got, test.clean)
			}
		})
	}
}

func TestGitArchiveStopsAtConfiguredLimit(t *testing.T) {
	bin := t.TempDir()
	path := filepath.Join(bin, "git")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n[ \"$GIT_NO_LAZY_FETCH\" = 1 ] && [ \"$GIT_CONFIG_NOSYSTEM\" = 1 ] && [ \"$GIT_CONFIG_GLOBAL\" = /dev/null ] || exit 7\ncase \"$1\" in\ninit) mkdir -p \"$4/objects/info\" ;;\narchive) printf '%s' '012345678901234567890123456789012' ;;\nesac\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git", "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := gitArchiveWithLimit(context.Background(), newGitRepo(gitcmd.NewExecutor(), repo), strings.Repeat("a", 40), 32)
	if err == nil || !strings.Contains(err.Error(), "size exceeds") {
		t.Fatalf("over-limit archive err=%v", err)
	}
}

func TestGitArchiveUsesRecordedOIDWithoutReplaceRefs(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	runGitInTest(t, repo, "init")
	setArchiveFixtureGitIdentity(t, repo)
	writeFile(t, repo, "payload.txt", "source-oid-content\n")
	runGitInTest(t, repo, "add", "payload.txt")
	runGitInTest(t, repo, "commit", "-m", "original")
	original := strings.TrimSpace(runGitInTest(t, repo, "rev-parse", "HEAD"))
	writeFile(t, repo, "payload.txt", "replacement-content\n")
	runGitInTest(t, repo, "add", "payload.txt")
	runGitInTest(t, repo, "commit", "-m", "replacement")
	replacement := strings.TrimSpace(runGitInTest(t, repo, "rev-parse", "HEAD"))
	runGitInTest(t, repo, "replace", original, replacement)
	archive, err := gitArchive(context.Background(), newGitRepo(gitcmd.NewExecutor(), repo), original)
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			t.Fatal(nextErr)
		}
		if header.Name == "payload.txt" {
			payload, err := io.ReadAll(io.LimitReader(reader, header.Size))
			if err != nil {
				t.Fatal(err)
			}
			if string(payload) != "source-oid-content\n" {
				t.Fatalf("archive content follows replacement ref: %q", payload)
			}
			return
		}
	}
	t.Fatal("payload.txt absent from archive")
}

func TestGitArchiveDoesNotRunChildOrAmbientFilters(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("filter fixture uses a Unix shell script")
	}
	for _, test := range []struct {
		name    string
		install func(t *testing.T, repo, filter string)
	}{
		{
			name: "child local smudge config",
			install: func(t *testing.T, repo, filter string) {
				t.Helper()
				runGitInTest(t, repo, "config", "filter.repro.smudge", filter)
			},
		},
		{
			name: "child local process config",
			install: func(t *testing.T, repo, filter string) {
				t.Helper()
				runGitInTest(t, repo, "config", "filter.repro.process", filter)
			},
		},
		{
			name: "ambient config injection",
			install: func(t *testing.T, _ string, filter string) {
				t.Helper()
				t.Setenv("GIT_CONFIG_COUNT", "1")
				t.Setenv("GIT_CONFIG_KEY_0", "filter.repro.smudge")
				t.Setenv("GIT_CONFIG_VALUE_0", filter)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			runGitInTest(t, repo, "init")
			setArchiveFixtureGitIdentity(t, repo)
			writeFile(t, repo, ".gitattributes", "payload.txt filter=repro\n")
			writeFile(t, repo, "payload.txt", "raw blob\n")
			runGitInTest(t, repo, "add", ".gitattributes", "payload.txt")
			runGitInTest(t, repo, "commit", "-m", "filtered payload")
			marker := filepath.Join(t.TempDir(), "filter-ran")
			filter := filepath.Join(t.TempDir(), "smudge")
			if err := os.WriteFile(filter, []byte("#!/bin/sh\ntouch "+marker+"\ncat >/dev/null\nprintf 'altered bytes\\n'\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			test.install(t, repo, filter)
			oid := strings.TrimSpace(runGitInTest(t, repo, "rev-parse", "HEAD"))
			archive, err := gitArchive(context.Background(), newGitRepo(gitcmd.NewExecutor(), repo), oid)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("filter executed: %v", err)
			}
			if got := archiveFile(t, archive, "payload.txt"); got != "raw blob\n" {
				t.Fatalf("archive content = %q", got)
			}
		})
	}
}

func archiveFile(t *testing.T, archive []byte, want string) string {
	t.Helper()
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name != want {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(reader, header.Size))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	t.Fatalf("%s absent from archive", want)
	return ""
}

func TestExtractGitArchiveRejectsEscapesAndLinks(t *testing.T) {
	t.Parallel()
	for name, archive := range map[string][]byte{
		"escape": tarArchive(t, "../outside", "x", 0),
		"link":   tarArchive(t, "link", "", tar.TypeSymlink),
		"drive":  tarArchive(t, "C:/outside", "x", 0),
		"slash":  tarArchive(t, "folder\\outside", "x", 0),
	} {
		t.Run(name, func(t *testing.T) {
			if err := extractGitArchive(context.Background(), filepath.Join(t.TempDir(), "child"), archive); err == nil {
				t.Fatal("unsafe archive was extracted")
			}
		})
	}
}

func TestExtractGitArchiveAllowsOnlyRootClaudeAliases(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "child")
	archive := tarArchiveEntries(
		t,
		tarEntry{name: "AGENTS.md", link: "CLAUDE.md", kind: tar.TypeSymlink},
		tarEntry{name: "GEMINI.md", link: "CLAUDE.md", kind: tar.TypeSymlink},
		tarEntry{name: "CLAUDE.md", body: "instructions\n"},
	)
	if err := extractGitArchive(context.Background(), destination, archive); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AGENTS.md", "GEMINI.md"} {
		info, err := os.Lstat(filepath.Join(destination, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is not a symlink", name)
		}
		data, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "instructions\n" {
			t.Fatalf("%s content = %q", name, data)
		}
	}
}

// The harness campaign made AGENTS.md the regular file, with CLAUDE.md and
// GEMINI.md as symlinks to it; flow-taskchain-engine and -mcp ship that layout.
func TestExtractGitArchiveAllowsRootAgentsCanonicalAliases(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "child")
	archive := tarArchiveEntries(
		t,
		tarEntry{name: "AGENTS.md", body: "instructions\n"},
		tarEntry{name: "CLAUDE.md", link: "AGENTS.md", kind: tar.TypeSymlink},
		tarEntry{name: "GEMINI.md", link: "AGENTS.md", kind: tar.TypeSymlink},
	)
	if err := extractGitArchive(context.Background(), destination, archive); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CLAUDE.md", "GEMINI.md"} {
		info, err := os.Lstat(filepath.Join(destination, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is not a symlink", name)
		}
		data, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "instructions\n" {
			t.Fatalf("%s content = %q", name, data)
		}
	}
}

func TestExtractGitArchiveRejectsOtherSymlinks(t *testing.T) {
	t.Parallel()
	for name, archive := range map[string][]byte{
		"missing Claude target": tarArchiveEntries(t, tarEntry{name: "AGENTS.md", link: "CLAUDE.md", kind: tar.TypeSymlink}),
		"nested alias":          tarArchiveEntries(t, tarEntry{name: "nested/AGENTS.md", link: "CLAUDE.md", kind: tar.TypeSymlink}, tarEntry{name: "CLAUDE.md", body: "instructions\n"}),
		"wrong target":          tarArchiveEntries(t, tarEntry{name: "AGENTS.md", link: "nested/CLAUDE.md", kind: tar.TypeSymlink}, tarEntry{name: "CLAUDE.md", body: "instructions\n"}),
		"other root link":       tarArchiveEntries(t, tarEntry{name: "OTHER.md", link: "CLAUDE.md", kind: tar.TypeSymlink}, tarEntry{name: "CLAUDE.md", body: "instructions\n"}),
		"missing Agents target": tarArchiveEntries(t, tarEntry{name: "CLAUDE.md", link: "AGENTS.md", kind: tar.TypeSymlink}),
		"alias loop":            tarArchiveEntries(t, tarEntry{name: "CLAUDE.md", link: "AGENTS.md", kind: tar.TypeSymlink}, tarEntry{name: "AGENTS.md", link: "CLAUDE.md", kind: tar.TypeSymlink}),
		"self alias":            tarArchiveEntries(t, tarEntry{name: "AGENTS.md", body: "instructions\n"}, tarEntry{name: "CLAUDE.md", link: "CLAUDE.md", kind: tar.TypeSymlink}),
		"alias to non-harness":  tarArchiveEntries(t, tarEntry{name: "CLAUDE.md", link: "README.md", kind: tar.TypeSymlink}, tarEntry{name: "README.md", body: "instructions\n"}),
	} {
		t.Run(name, func(t *testing.T) {
			if err := extractGitArchive(context.Background(), filepath.Join(t.TempDir(), "child"), archive); err == nil {
				t.Fatal("unsupported symlink was extracted")
			}
		})
	}
}

// A symlink entry that reuses the name of a regular harness file must not
// replace it, and the failed extraction must not leave a partial tree. Two
// guards cover this: the alias check rejects a link whose target is itself a
// link, and when the target is regular the already-extracted file refuses the
// symlink. A regular file arriving after the alias must not write through it.
func TestExtractGitArchiveRejectsAliasShadowingRegularTarget(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		archive []byte
		check   func(error) bool
	}{
		"target is an alias": {
			archive: tarArchiveEntries(
				t,
				tarEntry{name: "AGENTS.md", body: "instructions\n"},
				tarEntry{name: "CLAUDE.md", link: "AGENTS.md", kind: tar.TypeSymlink},
				tarEntry{name: "AGENTS.md", link: "CLAUDE.md", kind: tar.TypeSymlink},
			),
			check: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), `unsupported entry "AGENTS.md"`)
			},
		},
		"target is regular": {
			archive: tarArchiveEntries(
				t,
				tarEntry{name: "AGENTS.md", body: "instructions\n"},
				tarEntry{name: "CLAUDE.md", body: "other\n"},
				tarEntry{name: "AGENTS.md", link: "CLAUDE.md", kind: tar.TypeSymlink},
			),
			check: func(err error) bool { return errors.Is(err, os.ErrExist) },
		},
		"regular file after the alias": {
			archive: tarArchiveEntries(
				t,
				tarEntry{name: "CLAUDE.md", body: "other\n"},
				tarEntry{name: "AGENTS.md", link: "CLAUDE.md", kind: tar.TypeSymlink},
				tarEntry{name: "AGENTS.md", body: "instructions\n"},
			),
			// O_EXCL refuses the existing link instead of writing through it
			// into CLAUDE.md.
			check: func(err error) bool {
				var pathErr *os.PathError
				return errors.Is(err, os.ErrExist) && errors.As(err, &pathErr) && filepath.Base(pathErr.Path) == "AGENTS.md"
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "child")
			err := extractGitArchive(context.Background(), destination, tc.archive)
			if !tc.check(err) {
				t.Fatalf("archive redefining AGENTS.md as a symlink: err = %v", err)
			}
			if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
				t.Fatalf("failed extraction left destination: %v", statErr)
			}
		})
	}
}

func TestExtractGitArchiveCancelsAndCleansDestination(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "child")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := extractGitArchive(ctx, destination, tarArchive(t, "file", "x", 0))
	if err == nil {
		t.Fatal("canceled extraction passed")
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("canceled extraction left destination: %v", statErr)
	}
}

func TestCopyArchiveFileChecksCancellationBetweenChunks(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	source := &cancelAfterFirstRead{reader: bytes.NewReader(bytes.Repeat([]byte{'x'}, 64<<10)), cancel: cancel}
	var destination bytes.Buffer
	copied, err := copyArchiveFile(ctx, &destination, source, 64<<10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("copy error = %v, want context cancellation", err)
	}
	if copied == 0 || copied >= 64<<10 {
		t.Fatalf("copied = %d, want partial payload", copied)
	}
}

type cancelAfterFirstRead struct {
	reader *bytes.Reader
	cancel context.CancelFunc
	read   bool
}

func (r *cancelAfterFirstRead) Read(data []byte) (int, error) {
	n, err := r.reader.Read(data)
	if !r.read {
		r.read = true
		r.cancel()
	}
	return n, err
}

func setArchiveFixtureGitIdentity(t *testing.T, repo string) {
	t.Helper()
	runGitInTest(t, repo, "config", "user.name", "Test Fixture")
	runGitInTest(t, repo, "config", "user.email", "fixture@example.invalid")
}

type tarEntry struct {
	name string
	body string
	link string
	kind byte
}

func tarArchive(t *testing.T, name, body string, kind byte) []byte {
	t.Helper()
	return tarArchiveEntries(t, tarEntry{name: name, body: body, kind: kind})
}

func tarArchiveEntries(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := tar.NewWriter(&out)
	for _, entry := range entries {
		if entry.kind == 0 {
			entry.kind = tar.TypeReg
		}
		if err := writer.WriteHeader(&tar.Header{Name: entry.name, Linkname: entry.link, Mode: 0o644, Size: int64(len(entry.body)), Typeflag: entry.kind}); err != nil {
			t.Fatal(err)
		}
		if entry.kind == tar.TypeReg {
			if _, err := writer.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
