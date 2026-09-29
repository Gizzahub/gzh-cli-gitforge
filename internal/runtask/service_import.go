// One-shot carryover of in-flight CE run records into gz-git state.

package runtask

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ImportCEReport is the answer of the explicit one-shot import.
//
//nolint:tagliatelle // CE-parity JSON keys are the task-manager contract.
type ImportCEReport struct {
	SchemaVersion int    `json:"schemaVersion"`
	Imported      bool   `json:"imported"`
	DryRun        bool   `json:"dryRun,omitempty"`
	Executions    int    `json:"executions"`
	Receipts      int    `json:"receipts"`
	SourceDir     string `json:"sourceDir"`
	TargetDir     string `json:"targetDir"`
}

// ceRuntimeStateDir names CE's own state root. The lifecycle never reads it;
// only ImportCE does, because ADR-0055 keeps the carryover explicit rather
// than adding a silent dual read to every command.
const ceRuntimeStateDir = "ce"

// ImportCE copies in-flight CE task-runtime records into this runtime's
// state directory, byte for byte. Executions and receipts are evidence: they
// carry CE's tool stamps and CE's words, and rewriting them would falsify
// what happened; deriveState reconciles their worktrees and branches against
// the live repository the same way it reconciles any reclaimed run.
//
// It refuses to merge: gz-git state that already exists is an operator
// decision, not an import. It refuses to invent: a source with neither
// executions nor receipts has nothing to carry over. Records are parsed
// before they are copied, so a corrupt CE file stops here instead of
// surfacing later as an unreadable state.
func (s *Service) ImportCE(ctx context.Context, dryRun bool) (ImportCEReport, error) {
	r, err := s.command(ctx, s.root, "git", "rev-parse", "--git-common-dir")
	if err != nil || r.ExitCode != 0 {
		return ImportCEReport{}, fmt.Errorf("resolve git common dir: %s", firstLine(r.Stderr))
	}
	commonDir := firstLine(r.Stdout)
	source := newStateStore(filepath.Join(commonDir, ceRuntimeStateDir))
	target := newStateStore(commonDir)
	report := ImportCEReport{SchemaVersion: 1, DryRun: dryRun, SourceDir: source.dir, TargetDir: target.dir}

	for _, name := range []string{"executions.json", "receipts.jsonl"} {
		if _, err := os.Stat(filepath.Join(target.dir, name)); err == nil {
			return report, fmt.Errorf("%s already exists; import refuses to merge existing gz-git state", filepath.Join(target.dir, name))
		}
	}
	executions, executionsFound, err := readImportExecutions(source.dir)
	if err != nil {
		return report, err
	}
	receipts, receiptsFound, err := readImportReceipts(source.dir)
	if err != nil {
		return report, err
	}
	if !executionsFound && !receiptsFound {
		return report, fmt.Errorf("no CE task runtime records at %s; nothing to import", source.dir)
	}
	if executionsFound {
		report.Executions = len(executions)
	}
	if receiptsFound {
		report.Receipts = len(receipts)
	}
	if dryRun {
		return report, nil
	}
	if err := os.MkdirAll(target.dir, 0o755); err != nil { //nolint:gosec // CE-layout state directory parity
		return report, fmt.Errorf("create %s: %w", target.dir, err)
	}
	if executionsFound {
		if err := copyFile(filepath.Join(source.dir, "executions.json"), filepath.Join(target.dir, "executions.json")); err != nil {
			return report, err
		}
	}
	if receiptsFound {
		if err := copyFile(filepath.Join(source.dir, "receipts.jsonl"), filepath.Join(target.dir, "receipts.jsonl")); err != nil {
			return report, err
		}
	}
	syncDir(target.dir)
	report.Imported = true
	return report, nil
}

// readImportExecutions reports found=false when CE recorded no
// executions.json; a present file must parse.
func readImportExecutions(dir string) ([]TaskExecution, bool, error) {
	b, err := os.ReadFile(filepath.Join(dir, "executions.json")) //nolint:gosec // the path is CE's record under the resolved git common dir
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read CE executions: %w", err)
	}
	var out []TaskExecution
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, false, fmt.Errorf("decode CE executions: %w", err)
	}
	return out, true, nil
}

// readImportReceipts reports found=false when CE recorded no receipts.jsonl.
func readImportReceipts(dir string) (receipts map[string]TaskReceipt, found bool, err error) {
	f, err := os.Open(filepath.Join(dir, "receipts.jsonl")) //nolint:gosec // the path is CE's record under the resolved git common dir
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read CE receipts: %w", err)
	}
	defer func() { _ = f.Close() }()
	parsed, parseErr := parseReceipts(f)
	return parsed, parseErr == nil, parseErr
}

// copyFile copies bytes verbatim and fsyncs the result, so an imported
// receipt log survives a crash the same way an appended one does.
func copyFile(src, dst string) error {
	b, err := os.ReadFile(src) //nolint:gosec // the source is CE's record under the resolved git common dir
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	tmp := dst + ".import.tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil { //nolint:gosec // byte-for-byte import keeps CE's file mode parity
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := syncFile(tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("sync %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("place %s: %w", dst, err)
	}
	return nil
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
