// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	siblingArchiveLimit           int64 = 256 << 20
	siblingArchiveInitOutputLimit       = 64 << 10
	siblingArchiveEntryLimit            = 100000
)

type prepareInput struct {
	Name string
	OID  string
	data []byte
}

func requireCanonicalRemote(ctx context.Context, g gitRepo, name, want string) error {
	canonicalWant, err := canonicalPushEndpoint(want)
	if err != nil {
		return fmt.Errorf("%s canonical expected remote: %w", name, err)
	}
	for _, push := range []bool{false, true} {
		args := make([]string, 0, 4)
		args = append(args, "remote", "get-url", "--all")
		if push {
			args = []string{"remote", "get-url", "--push", "--all"}
		}
		args = append(args, "origin")
		raw, err := isolatedGitOutput(ctx, g, args...)
		if err != nil {
			return fmt.Errorf("%s %s remote: %w", name, map[bool]string{false: "fetch", true: "push"}[push], err)
		}
		urls := splitNonEmpty(raw)
		if len(urls) != 1 {
			return fmt.Errorf("%s must have exactly one canonical %s remote", name, map[bool]string{false: "fetch", true: "push"}[push])
		}
		got, err := canonicalPushEndpoint(urls[0])
		if err != nil || got != canonicalWant {
			if err != nil {
				return fmt.Errorf("%s canonical %s remote: %w", name, map[bool]string{false: "fetch", true: "push"}[push], err)
			}
			return fmt.Errorf("%s has non-canonical %s remote", name, map[bool]string{false: "fetch", true: "push"}[push])
		}
	}
	return nil
}

func gitArchive(ctx context.Context, g gitRepo, oid string) ([]byte, error) {
	return gitArchiveWithLimit(ctx, g, oid, siblingArchiveLimit)
}

func gitArchiveWithLimit(ctx context.Context, g gitRepo, oid string, limit int64) ([]byte, error) {
	if !fullObjectID(oid) {
		return nil, fmt.Errorf("archive object ID is not full")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("archive limit must be positive")
	}
	scratch, err := os.MkdirTemp("", "gzh-git-archive-*")
	if err != nil {
		return nil, fmt.Errorf("create archive repository: %w", err)
	}
	defer os.RemoveAll(scratch)
	home := filepath.Join(scratch, "home")
	if err := os.MkdirAll(filepath.Join(home, "xdg"), 0o700); err != nil {
		return nil, fmt.Errorf("create archive environment: %w", err)
	}
	objects, err := archiveObjectDirectory(g.dir)
	if err != nil {
		return nil, err
	}
	bare := filepath.Join(scratch, "archive.git")
	env := isolatedArchiveGitEnv(home)
	init, overflow, err := g.exec.RunWithOutputLimitCleanEnv(ctx, scratch, env, siblingArchiveInitOutputLimit, "init", "--bare", "--template=", bare)
	if err != nil {
		return nil, fmt.Errorf("initialize archive repository: %w", err)
	}
	if overflow || init.ExitCode != 0 {
		return nil, fmt.Errorf("initialize archive repository failed: %w: %s", init.Error, strings.TrimSpace(init.Stderr))
	}
	if err := os.MkdirAll(filepath.Join(bare, "objects", "info"), 0o700); err != nil {
		return nil, fmt.Errorf("create archive alternates directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(bare, "objects", "info", "alternates"), []byte(objects+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write archive alternates: %w", err)
	}
	res, overflow, err := g.exec.RunWithOutputLimitCleanEnv(ctx, bare, env, limit, "archive", "--format=tar", oid)
	if err != nil {
		return nil, fmt.Errorf("run git archive: %w", err)
	}
	if overflow {
		return nil, fmt.Errorf("archive size exceeds permitted range")
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("git archive failed: %w: %s", res.Error, strings.TrimSpace(res.Stderr))
	}
	data := []byte(res.Stdout)
	if len(data) == 0 {
		return nil, fmt.Errorf("archive size 0 exceeds permitted range")
	}
	return data, nil
}

func archiveObjectDirectory(dir string) (string, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve archive checkout: %w", err)
	}
	objects := filepath.Join(root, ".git", "objects")
	info, err := os.Stat(objects)
	if err != nil {
		return "", fmt.Errorf("archive object directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("archive object directory is not a directory")
	}
	return objects, nil
}

func isolatedArchiveGitEnv(home string) []string {
	return []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, "xdg"),
		"PATH=" + os.Getenv("PATH"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_COUNT=0",
		"GIT_ATTR_NOSYSTEM=1",
		"GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"LC_ALL=C",
	}
}

func isolatedGitOutput(ctx context.Context, g gitRepo, args ...string) (string, error) {
	raw, err := isolatedGitRawOutput(ctx, g, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(raw), nil
}

func isolatedGitRawOutput(ctx context.Context, g gitRepo, args ...string) (string, error) {
	res, err := g.exec.RunWithEnv(ctx, g.dir, isolatedGitEnv(), args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(res.Stderr))
	}
	return res.Stdout, nil
}

func isolatedGitEnv() []string {
	return []string{
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"LC_ALL=C",
	}
}

// statusCleanExceptUntracked accepts only complete NUL-delimited untracked
// records admitted by the operation's explicit runtime-metadata policy.
func statusCleanExceptUntracked(status string, allowPath func(string) bool) bool {
	if status == "" {
		return true
	}
	if !strings.HasSuffix(status, "\x00") {
		return false
	}
	records := bytes.Split([]byte(status), []byte{0})
	for index, record := range records {
		if len(record) == 0 {
			if index == len(records)-1 {
				continue
			}
			return false
		}
		if len(record) < 4 || string(record[:2]) != "??" || record[2] != ' ' {
			return false
		}
		name := string(record[3:])
		if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "/") || name == ".." || strings.HasPrefix(name, "../") || !allowPath(name) {
			return false
		}
	}
	return true
}

func isCELogRuntimeArtifactPath(name string) bool {
	parts := strings.Split(name, "/")
	for index, part := range parts {
		if part != ".ce" {
			continue
		}
		relative := parts[index+1:]
		switch {
		case len(relative) == 2 && relative[0] == "audit" && relative[1] == "events.jsonl":
			return true
		case len(relative) == 2 && relative[0] == "heartbeat" && (relative[1] == "events.jsonl" || relative[1] == "state.json"):
			return true
		case len(relative) == 3 && relative[0] == "heartbeat" && relative[1] == "sessions" && relative[2] != "" && strings.HasSuffix(relative[2], ".json"):
			return true
		}
	}
	return false
}

func fullObjectID(oid string) bool {
	if len(oid) != 40 && len(oid) != 64 {
		return false
	}
	for _, char := range oid {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func extractGitArchive(ctx context.Context, destination string, data []byte) (err error) {
	if err := rejectPathSymlinks(filepath.Dir(destination)); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		if err == nil {
			return fmt.Errorf("destination already exists: %s", destination)
		}
		return err
	}
	if err := os.Mkdir(destination, 0o750); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(destination)
		}
	}()

	reader := tar.NewReader(bytes.NewReader(data))
	rootHarness := archiveRegularRootHarness(data)
	entries := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > siblingArchiveEntryLimit {
			return fmt.Errorf("archive has too many entries")
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		// git archive emits a PAX global header for commit metadata. It carries
		// no filesystem entry; tar.Reader applies it to following headers.
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue
		}
		if err := extractArchiveEntry(ctx, destination, reader, header, rootHarness); err != nil {
			return err
		}
	}
}

// rootHarnessFiles are the root instruction files a repository keeps as one
// regular file plus symlink aliases to it. Either file may be the regular one:
// older repositories alias AGENTS.md and GEMINI.md to CLAUDE.md, while the
// harness campaign flipped them so CLAUDE.md and GEMINI.md alias AGENTS.md.
var rootHarnessFiles = map[string]bool{"AGENTS.md": true, "CLAUDE.md": true, "GEMINI.md": true}

// archiveRegularRootHarness returns the root harness files the archive holds
// as regular files, the only targets a harness alias may point at.
func archiveRegularRootHarness(data []byte) map[string]bool {
	regular := map[string]bool{}
	reader := tar.NewReader(bytes.NewReader(data))
	for {
		header, err := reader.Next()
		if err != nil {
			return regular
		}
		if header.Typeflag == tar.TypeReg && rootHarnessFiles[header.Name] {
			regular[header.Name] = true
		}
	}
}

func extractArchiveEntry(ctx context.Context, destination string, reader *tar.Reader, header *tar.Header, rootHarness map[string]bool) error {
	rel, err := safeArchivePath(header.Name, header.Typeflag == tar.TypeDir)
	if err != nil {
		return err
	}
	name := filepath.Join(destination, filepath.FromSlash(rel))
	if err := ensureArchiveContained(destination, name); err != nil {
		return err
	}
	if header.Typeflag == tar.TypeDir {
		return createArchiveDirectory(name, header.Name)
	}
	if header.Typeflag == tar.TypeSymlink && isRootHarnessAlias(header, rootHarness) {
		return os.Symlink(header.Linkname, name)
	}
	if header.Typeflag != tar.TypeReg {
		return fmt.Errorf("archive contains unsupported entry %q", header.Name)
	}
	return extractArchiveFile(ctx, name, header, reader)
}

func isRootHarnessAlias(header *tar.Header, rootHarness map[string]bool) bool {
	return rootHarnessFiles[header.Name] && header.Linkname != header.Name && rootHarness[header.Linkname]
}

func createArchiveDirectory(name, archiveName string) error {
	if err := os.Mkdir(name, 0o750); err != nil {
		if !os.IsExist(err) {
			return err
		}
		info, statErr := os.Stat(name)
		if statErr != nil || !info.IsDir() {
			return fmt.Errorf("archive path collision: %s", archiveName)
		}
	}
	return nil
}

func extractArchiveFile(ctx context.Context, name string, header *tar.Header, reader io.Reader) error {
	if header.Size < 0 || header.Size > siblingArchiveLimit {
		return fmt.Errorf("archive file %q has invalid size", header.Name)
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		return err
	}
	// #nosec G304 -- name is a canonical relative tar path joined below an owned destination.
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := copyArchiveFile(ctx, file, reader, header.Size)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if header.Mode < 0 || header.Mode > 0o777 {
		return fmt.Errorf("archive file %q has invalid mode", header.Name)
	}
	return os.Chmod(name, os.FileMode(uint32(header.Mode)))
}

func copyArchiveFile(ctx context.Context, destination io.Writer, source io.Reader, size int64) (int64, error) {
	buffer := make([]byte, 32<<10)
	var copied int64
	for copied < size {
		if err := ctx.Err(); err != nil {
			return copied, err
		}
		want := int64(len(buffer))
		if remaining := size - copied; remaining < want {
			want = remaining
		}
		n, readErr := source.Read(buffer[:want])
		if n > 0 {
			written, writeErr := destination.Write(buffer[:n])
			copied += int64(written)
			if writeErr != nil {
				return copied, writeErr
			}
			if written != n {
				return copied, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) && copied == size {
				return copied, nil
			}
			return copied, readErr
		}
		if n == 0 {
			return copied, io.ErrNoProgress
		}
	}
	return copied, nil
}

func safeArchivePath(name string, directory bool) (string, error) {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || path.IsAbs(name) || hasWindowsVolume(name) {
		return "", fmt.Errorf("archive contains unsafe path %q", name)
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("archive contains unsafe path %q", name)
	}
	canonical := name
	if directory {
		canonical = strings.TrimSuffix(canonical, "/")
	}
	if clean != canonical {
		return "", fmt.Errorf("archive contains non-canonical path %q", name)
	}
	return clean, nil
}

func hasWindowsVolume(name string) bool {
	if strings.HasPrefix(name, "//") {
		return true
	}
	return len(name) >= 2 && name[1] == ':' && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z'))
}

func ensureArchiveContained(destination, name string) error {
	rel, err := filepath.Rel(destination, name)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("archive path escapes destination: %s", name)
	}
	return nil
}

func rejectPathSymlinks(name string) error {
	info, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("preparation path is symlink: %s", name)
	}
	return nil
}
