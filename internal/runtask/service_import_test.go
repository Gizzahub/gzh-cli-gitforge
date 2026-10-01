package runtask

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newImportRepo builds a real git repository in a temp directory and returns
// its root with a service constructed the way production builds one. The
// process working directory is deliberately left where go test put it: these
// tests must prove that ImportCE resolves every path from the service root,
// not from wherever the test process happens to run.
func newImportRepo(t *testing.T) (string, *Service) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	gitConfig := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitConfig, []byte("[init]\n\tdefaultBranch = main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	git := func(args ...string) {
		t.Helper()
		c := exec.CommandContext(t.Context(), "git", args...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init")
	return root, NewService(root, NewExecutor(), NewEngine())
}

// writeCEFixture records CE's two files at the literal documented path
// `<git common dir>/ce/task-runtime/v1/`. The path segments are spelled out
// here on purpose: building them through newStateStore, ceRuntimeStateStore,
// or any other helper the code under test uses would make the tests repeat
// the very bug they exist to catch.
func writeCEFixture(t *testing.T, root string) (executionsPath, receiptsPath string) {
	t.Helper()
	ceDir := filepath.Join(root, ".git", "ce", "task-runtime", "v1")
	if err := os.MkdirAll(ceDir, 0o750); err != nil {
		t.Fatal(err)
	}
	executions := []byte(`[{"task":"task-257","type":"fix","source":"board","owner":{"actor":"grok","host":"mst","kind":"agent"},"branch":"dev/grok/mst/fix/importce-source-path","worktree":"/tmp/importce-fixture","startedAt":"2026-10-01T09:00:00Z","updatedAt":"2026-10-01T09:30:00Z"}]` + "\n")
	receipts := []byte(`{"task":"task-257","operation":"run-start","status":"ACTIVE","owner":{"actor":"grok","host":"mst","kind":"agent"},"branch":"dev/grok/mst/fix/importce-source-path","worktree":"/tmp/importce-fixture","createdAt":"2026-10-01T09:00:00Z","sourcePushed":false,"worktreeRemoved":false,"localBranchRemoved":false,"remoteBranchRemoved":false}` + "\n")
	executionsPath = filepath.Join(ceDir, "executions.json")
	receiptsPath = filepath.Join(ceDir, "receipts.jsonl")
	if err := os.WriteFile(executionsPath, executions, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptsPath, receipts, 0o600); err != nil {
		t.Fatal(err)
	}
	return executionsPath, receiptsPath
}

func TestImportCEDryRunReportsCountsAndWritesNothing(t *testing.T) {
	root, service := newImportRepo(t)
	writeCEFixture(t, root)

	report, err := service.ImportCE(t.Context(), true)
	if err != nil {
		t.Fatalf("ImportCE dry run: %v", err)
	}
	if !report.DryRun {
		t.Fatalf("report says DryRun=false: %+v", report)
	}
	if report.Executions != 1 || report.Receipts != 1 {
		t.Fatalf("counts = %d executions, %d receipts, want 1 and 1", report.Executions, report.Receipts)
	}
	wantSource := filepath.Join(root, ".git", "ce", "task-runtime", "v1")
	if report.SourceDir != wantSource {
		t.Fatalf("SourceDir = %q, want %q", report.SourceDir, wantSource)
	}
	wantTarget := filepath.Join(root, ".git", "gz-git", "task-runtime", "v1")
	if report.TargetDir != wantTarget {
		t.Fatalf("TargetDir = %q, want %q", report.TargetDir, wantTarget)
	}
	if report.Imported {
		t.Fatalf("dry run reported Imported=true: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "gz-git")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote into the runtime state dir: stat = %v, want not exist", err)
	}
}

func TestImportCECopiesRecordsIntoRuntimeState(t *testing.T) {
	root, service := newImportRepo(t)
	executionsPath, receiptsPath := writeCEFixture(t, root)

	report, err := service.ImportCE(t.Context(), false)
	if err != nil {
		t.Fatalf("ImportCE: %v", err)
	}
	if !report.Imported {
		t.Fatalf("report says Imported=false: %+v", report)
	}
	if report.Executions != 1 || report.Receipts != 1 {
		t.Fatalf("counts = %d executions, %d receipts, want 1 and 1", report.Executions, report.Receipts)
	}
	targetDir := filepath.Join(root, ".git", "gz-git", "task-runtime", "v1")
	for _, pair := range [][2]string{
		{executionsPath, filepath.Join(targetDir, "executions.json")},
		{receiptsPath, filepath.Join(targetDir, "receipts.jsonl")},
	} {
		want, err := os.ReadFile(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(pair[1])
		if err != nil {
			t.Fatalf("imported %s: %v", pair[1], err)
		}
		if !bytes.Equal(want, got) {
			t.Fatalf("%s was not copied byte for byte from %s", pair[1], pair[0])
		}
	}
}

func TestImportCERefusesExistingRuntimeState(t *testing.T) {
	root, service := newImportRepo(t)
	writeCEFixture(t, root)

	if _, err := service.ImportCE(t.Context(), false); err != nil {
		t.Fatalf("first import: %v", err)
	}
	report, err := service.ImportCE(t.Context(), false)
	if err == nil {
		t.Fatal("second import succeeded; want refusal because gz-git state now exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("refusal error = %v, want the existing gz-git state refusal", err)
	}
	if report.Imported {
		t.Fatalf("refused import reported Imported=true: %+v", report)
	}
}
