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
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestFlowTaskchainPrepareSnapshotsOnlyCanonicalCleanPrimaryCheckouts(t *testing.T) {
	devbox := flowTaskchainFixture(t)
	inputs, err := snapshotPrepareInputs(context.Background(), newGitRepo(gitcmd.NewExecutor(), devbox), flowTaskchainLocalSubprojectsV1)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 2 || !fullObjectID(inputs[0].OID) || !fullObjectID(inputs[1].OID) {
		t.Fatalf("inputs = %#v", inputs)
	}

	// A later checkout change cannot change the archive bytes already captured.
	writeFile(t, filepath.Join(devbox, "flow-taskchain-engine"), "snapshot.txt", "later\n")
	runGitInTest(t, filepath.Join(devbox, "flow-taskchain-engine"), "add", "snapshot.txt")
	runGitInTest(t, filepath.Join(devbox, "flow-taskchain-engine"), "commit", "-m", "later")
	prepared := t.TempDir()
	runGitInTest(t, prepared, "init")
	writeFile(t, prepared, "tracked", "base\n")
	runGitInTest(t, prepared, "add", "tracked")
	runGitInTest(t, prepared, "commit", "-m", "base")
	if err := prepareFlowTaskchainLocalSubprojects(context.Background(), prepared, inputs); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(prepared, "flow-taskchain-engine", "snapshot.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original\n" {
		t.Fatalf("prepared tree used changed checkout: %q", data)
	}
}

func TestFlowTaskchainPrepareUsesPrimaryFromLinkedTaskWorktree(t *testing.T) {
	devbox := flowTaskchainFixture(t)
	task := filepath.Join(t.TempDir(), "task")
	runGitInTest(t, devbox, "worktree", "add", "-b", "dev/actor/feat/task", task)
	g := newGitRepo(gitcmd.NewExecutor(), task)
	sha := strings.TrimSpace(runGitInTest(t, task, "rev-parse", "HEAD"))
	prepared, err := prepareLegacyTreesWithProfile(context.Background(), g, TargetPlan{BranchSHA: sha, TargetSHA: sha}, nil, flowTaskchainLocalSubprojectsV1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prepared.evidence(), "flow-taskchain-engine@") || !strings.Contains(prepared.evidence(), "flow-taskchain-mcp@") {
		t.Fatalf("missing snapshot evidence: %q", prepared.evidence())
	}
	if err := prepared.cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestFlowTaskchainPrepareRejectsDirtySymlinkAndWrongRemote(t *testing.T) {
	devbox := flowTaskchainFixture(t)
	engine := filepath.Join(devbox, "flow-taskchain-engine")

	writeFile(t, engine, "untracked", "x")
	if _, err := snapshotPrepareInputs(context.Background(), newGitRepo(gitcmd.NewExecutor(), devbox), flowTaskchainLocalSubprojectsV1); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty checkout err=%v", err)
	}
	if err := os.Remove(filepath.Join(engine, "untracked")); err != nil {
		t.Fatal(err)
	}
	runGitInTest(t, engine, "remote", "set-url", "origin", "https://example.invalid/wrong.git")
	if _, err := snapshotPrepareInputs(context.Background(), newGitRepo(gitcmd.NewExecutor(), devbox), flowTaskchainLocalSubprojectsV1); err == nil || !strings.Contains(err.Error(), "non-canonical fetch") {
		t.Fatalf("wrong fetch err=%v", err)
	}
	runGitInTest(t, engine, "remote", "set-url", "origin", flowTaskchainSubprojects[0].remote)
	runGitInTest(t, engine, "remote", "set-url", "--push", "origin", "https://example.invalid/wrong.git")
	if _, err := snapshotPrepareInputs(context.Background(), newGitRepo(gitcmd.NewExecutor(), devbox), flowTaskchainLocalSubprojectsV1); err == nil || !strings.Contains(err.Error(), "non-canonical push") {
		t.Fatalf("wrong push err=%v", err)
	}
	runGitInTest(t, engine, "remote", "set-url", "--push", "origin", flowTaskchainSubprojects[0].remote)
	runGitInTest(t, engine, "remote", "set-url", "--add", "origin", "https://example.invalid/second.git")
	if _, err := snapshotPrepareInputs(context.Background(), newGitRepo(gitcmd.NewExecutor(), devbox), flowTaskchainLocalSubprojectsV1); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("multiple fetch remotes err=%v", err)
	}
}

func TestGitArchiveStopsAtConfiguredLimit(t *testing.T) {
	bin := t.TempDir()
	path := filepath.Join(bin, "git")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n[ \"$GIT_NO_LAZY_FETCH\" = 1 ] && [ \"$GIT_CONFIG_NOSYSTEM\" = 1 ] && [ \"$GIT_CONFIG_GLOBAL\" = /dev/null ] || exit 7\nprintf '%s' '012345678901234567890123456789012'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	_, err := gitArchiveWithLimit(context.Background(), newGitRepo(gitcmd.NewExecutor(), t.TempDir()), strings.Repeat("a", 40), 32)
	if err == nil || !strings.Contains(err.Error(), "size exceeds") {
		t.Fatalf("over-limit archive err=%v", err)
	}
}

func TestGitArchiveUsesRecordedOIDWithoutReplaceRefs(t *testing.T) {
	repo := t.TempDir()
	runGitInTest(t, repo, "init")
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

func TestFlowTaskchainPrepareRejectsSubprojectSymlink(t *testing.T) {
	devbox := flowTaskchainFixture(t)
	engine := filepath.Join(devbox, "flow-taskchain-engine")
	outside := filepath.Join(t.TempDir(), "engine")
	if err := os.Rename(engine, outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, engine); err != nil {
		t.Fatal(err)
	}
	_, err := snapshotPrepareInputs(context.Background(), newGitRepo(gitcmd.NewExecutor(), devbox), flowTaskchainLocalSubprojectsV1)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("subproject symlink err=%v", err)
	}
}

func TestFlowTaskchainSnapshotDisablesRepositoryFsmonitor(t *testing.T) {
	devbox := flowTaskchainFixture(t)
	engine := filepath.Join(devbox, "flow-taskchain-engine")
	marker := filepath.Join(t.TempDir(), "fsmonitor-ran")
	hook := filepath.Join(t.TempDir(), "fsmonitor")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGitInTest(t, engine, "config", "core.fsmonitor", hook)
	if _, err := snapshotPrepareInputs(context.Background(), newGitRepo(gitcmd.NewExecutor(), devbox), flowTaskchainLocalSubprojectsV1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("repository fsmonitor ran: %v", err)
	}
}

func TestExtractGitArchiveRejectsEscapesAndLinks(t *testing.T) {
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

func TestExtractGitArchiveCancelsAndCleansDestination(t *testing.T) {
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

func flowTaskchainFixture(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	devbox := filepath.Join(home, "mydevbox", flowTaskchainDevboxName)
	if err := os.MkdirAll(devbox, 0o755); err != nil {
		t.Fatal(err)
	}
	runGitInTest(t, devbox, "init")
	writeFile(t, devbox, "Makefile", "check:\n\t@true\nlint:\n\t@true\n")
	runGitInTest(t, devbox, "add", "Makefile")
	runGitInTest(t, devbox, "commit", "-m", "root")
	for _, spec := range flowTaskchainSubprojects {
		child := filepath.Join(devbox, spec.name)
		if err := os.Mkdir(child, 0o755); err != nil {
			t.Fatal(err)
		}
		runGitInTest(t, child, "init")
		writeFile(t, child, "snapshot.txt", "original\n")
		runGitInTest(t, child, "add", "snapshot.txt")
		runGitInTest(t, child, "commit", "-m", "snapshot")
		runGitInTest(t, child, "remote", "add", "origin", spec.remote)
	}
	return devbox
}

func tarArchive(t *testing.T, name, body string, kind byte) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := tar.NewWriter(&out)
	if kind == 0 {
		kind = tar.TypeReg
	}
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: kind}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
