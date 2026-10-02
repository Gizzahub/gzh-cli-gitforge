package benchmarks

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestBenchmarkBinaryIsPrivate pins the two properties every benchmark in
// this package depends on: the measured gz-git is built by a real
// `make build` into the test's private TempDir (created location, executable,
// runs), and the repository-root gz-git this suite used to overwrite is left
// exactly as it was — present and byte-identical when it existed, absent when
// it did not.
func TestBenchmarkBinaryIsPrivate(t *testing.T) {
	root := repositoryRoot(t)
	rootBinary := filepath.Join(root, "gz-git")
	before := snapshotBinary(t, rootBinary)

	binPath := buildPrivateBinary(t)

	if rel, err := filepath.Rel(root, binPath); err == nil && !strings.HasPrefix(rel, "..") {
		t.Fatalf("private binary %s was built inside the repository (path relative to root: %s)", binPath, rel)
	}
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("built binary %s is missing: %v", binPath, err)
	}
	if info.IsDir() {
		t.Fatalf("built binary path %s is a directory", binPath)
	}
	out, err := exec.CommandContext(t.Context(), binPath, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("built binary %s does not run: %v\nOutput: %s", binPath, err, out)
	}
	t.Logf("private binary runs: %s", strings.TrimSpace(string(out)))

	assertBinaryUnchanged(t, rootBinary, before)
}

// TestBenchmarkRecordRejectsDirtyTree proves `make benchmark-record` refuses
// to collect measurements from a dirty source tree: a tracked modification or
// a non-ignored untracked file each make the target exit non-zero without
// creating its OUTPUT_DIR. The real checkout is never dirtied — every
// scenario runs in a fresh clone of this repository checked out at the task
// SHA.
func TestBenchmarkRecordRejectsDirtyTree(t *testing.T) {
	root := repositoryRoot(t)
	taskSHA, err := gitOutput(t, root, "rev-parse", "HEAD")
	if err != nil || taskSHA == "" {
		t.Fatalf("resolving task SHA: err=%v output=%q", err, taskSHA)
	}

	scenarios := []struct {
		name  string
		dirty func(t *testing.T, cloneDir string)
	}{
		{
			name: "tracked modification",
			dirty: func(t *testing.T, cloneDir string) {
				t.Helper()
				path := filepath.Join(cloneDir, "go.mod")
				data, err := os.ReadFile(path) // #nosec G304 -- fixed clone-local path built from the test's TempDir
				if err != nil {
					t.Fatalf("reading cloned go.mod: %v", err)
				}
				probe := []byte("\n// benchmark-record dirty probe\n")
				modified := append(slices.Clone(data), probe...)
				if err := os.WriteFile(path, modified, 0o600); err != nil {
					t.Fatalf("modifying cloned go.mod: %v", err)
				}
			},
		},
		{
			name: "untracked file",
			dirty: func(t *testing.T, cloneDir string) {
				t.Helper()
				path := filepath.Join(cloneDir, "benchmark-record-dirty-probe.txt")
				if err := os.WriteFile(path, []byte("untracked probe\n"), 0o600); err != nil {
					t.Fatalf("creating untracked probe file: %v", err)
				}
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			cloneDir := cloneSourceAtTaskSHA(t, root, taskSHA)
			scenario.dirty(t, cloneDir)

			status, err := gitOutput(t, cloneDir, "status", "--porcelain", "--untracked-files=normal")
			if err != nil {
				t.Fatalf("checking clone dirty state: %v", err)
			}
			if status == "" {
				t.Fatalf("precondition failed: clone is clean, expected %q to make it dirty", scenario.name)
			}

			outDir := filepath.Join(t.TempDir(), "record")
			cmd := exec.CommandContext(t.Context(), "make", "benchmark-record", "OUTPUT_DIR="+outDir)
			cmd.Dir = cloneDir
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("make benchmark-record accepted a dirty tree\nOutput: %s", output)
			}
			t.Logf("refused as expected:\n%s", output)

			if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("OUTPUT_DIR %s was created despite the dirty-tree refusal: %v", outDir, statErr)
			}
		})
	}
}

// repositoryRoot returns the absolute path of this package's repository root.
// `go test` runs the test binary with the working directory set to the
// package directory, so the root is one level up.
func repositoryRoot(tb testing.TB) string {
	tb.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		tb.Fatalf("resolving repository root: %v", err)
	}
	return root
}

// binarySnapshot records a file's presence and, when present, its size,
// permission bits, and SHA-256 so a later comparison can prove the file was
// left untouched.
type binarySnapshot struct {
	exists bool
	size   int64
	mode   os.FileMode
	sha256 string
}

// snapshotBinary captures the current state of the file at path.
func snapshotBinary(tb testing.TB, path string) binarySnapshot {
	tb.Helper()
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return binarySnapshot{}
	}
	if err != nil {
		tb.Fatalf("statting %s: %v", path, err)
	}
	sum, err := fileSHA256(path)
	if err != nil {
		tb.Fatalf("hashing %s: %v", path, err)
	}
	return binarySnapshot{exists: true, size: info.Size(), mode: info.Mode(), sha256: sum}
}

// assertBinaryUnchanged fails the test when the file at path is no longer in
// the state recorded in before.
func assertBinaryUnchanged(tb testing.TB, path string, before binarySnapshot) {
	tb.Helper()
	after := snapshotBinary(tb, path)
	if before.exists != after.exists {
		tb.Fatalf("binary %s existence changed: existed=%v before the run, existed=%v after", path, before.exists, after.exists)
	}
	if !before.exists {
		return
	}
	if after.sha256 != before.sha256 {
		tb.Fatalf("binary %s content changed during the run (sha256 %s -> %s)", path, before.sha256, after.sha256)
	}
	if after.size != before.size || after.mode != before.mode {
		tb.Fatalf("binary %s metadata changed during the run: size %d -> %d, mode %v -> %v",
			path, before.size, after.size, before.mode, after.mode)
	}
}

// fileSHA256 returns the hex-encoded SHA-256 digest of the file at path.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- test-only path built from the repository root
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// cloneSourceAtTaskSHA clones the local source checkout into the test's
// private TempDir and checks out taskSHA explicitly, so the scenarios that
// follow mutate the clone and never the real checkout.
func cloneSourceAtTaskSHA(t *testing.T, sourceRoot, taskSHA string) string {
	t.Helper()
	cloneDir := filepath.Join(t.TempDir(), "source")
	clone := exec.CommandContext(t.Context(), "git", "clone", "--quiet", sourceRoot, cloneDir)
	if out, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("cloning %s into %s: %v\nOutput: %s", sourceRoot, cloneDir, err, out)
	}
	checkout := exec.CommandContext(t.Context(), "git", "-C", cloneDir, "checkout", "--quiet", taskSHA)
	if out, err := checkout.CombinedOutput(); err != nil {
		t.Fatalf("checking out task SHA %s: %v\nOutput: %s", taskSHA, err, out)
	}
	return cloneDir
}

// gitOutput runs one git command in dir and returns its combined output with
// surrounding whitespace trimmed.
func gitOutput(tb testing.TB, dir string, args ...string) (string, error) {
	tb.Helper()
	cmd := exec.CommandContext(tb.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
