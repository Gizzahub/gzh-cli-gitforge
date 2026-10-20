package integrate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestReadLegacyEvidenceRejectsUnknownAndMalformedShape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	valid := legacyEvidence{SchemaVersion: 1, OriginalReceiptSHA256: strings.Repeat("a", 64), SingleCommandDiagnostics: legacyDiagnostic{Command: "/bin/gz", Executable: "/bin/gz", ExecutableSHA256: strings.Repeat("b", 64), Cwd: "/tmp", Args: []string{"integrate", "run"}, ExitCode: 1, Stdout: "{}"}}
	b, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "evidence.json")
	if err = os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = readLegacyEvidence(path); err != nil {
		t.Fatalf("valid evidence: %v", err)
	}
	if err = os.WriteFile(path, append(b, []byte(` `)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = readLegacyEvidence(path); err != nil {
		t.Fatalf("whitespace must be accepted: %v", err)
	}
	if err = os.WriteFile(path, []byte(`{"schema_version":1,"original_receipt_sha256":"`+strings.Repeat("a", 64)+`","single_command_diagnostics":{},"extra":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = readLegacyEvidence(path); err == nil {
		t.Fatal("unknown field accepted")
	}
	if err = os.WriteFile(path, []byte(`{"schema_version":1,"schema_version":1,"original_receipt_sha256":"`+strings.Repeat("a", 64)+`","single_command_diagnostics":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = readLegacyEvidence(path); err == nil {
		t.Fatal("duplicate evidence key accepted")
	}
	if _, err = readBoundedRegularFile(dir, maxLegacyEvidenceBytes, "evidence"); err == nil {
		t.Fatal("directory accepted as evidence")
	}
}

func TestReconcileLegacyPreMutationWritesOnlyBoundNegativeAndRejectsPriorReceipt(t *testing.T) {
	t.Parallel()
	artifact := os.Getenv("GZ_GIT_LEGACY_TEST_ARTIFACT")
	if artifact == "" {
		t.Skip("GZ_GIT_LEGACY_TEST_ARTIFACT is required for retained-artifact integration coverage")
	}
	if _, err := os.Stat(artifact); err != nil {
		t.Skipf("retained test artifact unavailable: %v", err)
	}
	fx := readyTaskFixture(t)
	sha := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	root := gitOutput(t, fx.Worktree, "rev-parse", "--show-toplevel")
	makeEvidence := func(id string) string {
		o := LegacyReconcileOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", Target: "develop", Execution: &ExecutionContext{ExecutionID: id, Generation: "g1", Owner: OwnerIdentity{Actor: "codex", Host: "testhost", Kind: "agent"}, ExpectSource: sha}}
		args := []string{"integrate", "run", "--target", "develop", "--expect-source", sha, "--execution-id", id, "--generation", "g1", "--owner-actor", "codex", "--owner-host", "testhost", "--owner-kind", "agent", "--json"}
		run := legacyRun{SchemaVersion: 1, Command: "integrate-run", TargetRef: "refs/heads/develop", ExitCode: 1, Error: "not ready"}
		out, err := json.Marshal(run)
		if err != nil {
			t.Fatal(err)
		}
		e := legacyEvidence{SchemaVersion: 1, OriginalReceiptSHA256: strings.Repeat("c", 64), SingleCommandDiagnostics: legacyDiagnostic{Command: "/historical/gz-git", Executable: "/historical/gz-git", ExecutableSHA256: legacyArtifactSHA256, Cwd: root, Args: args, ExitCode: 1, Stdout: string(out)}}
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), id+".json")
		if err = os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		o.EvidenceFile = path
		o.LegacyArtifact = artifact
		report, err := ReconcileLegacyPreMutation(context.Background(), gitcmd.NewExecutor(), o)
		if err != nil {
			t.Fatalf("reconcile %s: %v", id, err)
		}
		if report.Outcome == nil || report.Outcome.ReconciledReceiptSHA256 != e.OriginalReceiptSHA256 {
			t.Fatalf("report=%+v", report)
		}
		return id
	}
	first := makeEvidence("legacy-bound")
	common := gitOutput(t, fx.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	stored, ok, err := readPreMutationOutcome(common, first)
	if err != nil || !ok || stored.LegacyArtifactSHA256 != legacyArtifactSHA256 {
		t.Fatalf("stored=%+v ok=%t err=%v", stored, ok, err)
	}
	second := "legacy-receipt"
	receipt := &Receipt{SchemaVersion: 1, ExecutionID: second, Generation: "g1", Owner: OwnerIdentity{Actor: "codex", Host: "testhost", Kind: "agent"}, RepositoryCommonDir: common, TaskBranch: "dev/actor/feat/task", TaskSHA: sha, TargetRef: "refs/heads/develop", Phase: evidencePhaseIntent}
	if err = createReceipt(context.Background(), common, receipt); err != nil {
		t.Fatal(err)
	}
	// Build a syntactically valid attestation for the receipt-bound execution; it must still fail closed.
	_ = makeEvidence // separate branch below avoids a false success path
	o := LegacyReconcileOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task", Target: "develop", Execution: &ExecutionContext{ExecutionID: second, Generation: "g1", Owner: OwnerIdentity{Actor: "codex", Host: "testhost", Kind: "agent"}, ExpectSource: sha}, LegacyArtifact: artifact}
	args := []string{"integrate", "run", "--target", "develop", "--expect-source", sha, "--execution-id", second, "--generation", "g1", "--owner-actor", "codex", "--owner-host", "testhost", "--owner-kind", "agent", "--json"}
	run := legacyRun{SchemaVersion: 1, Command: "integrate-run", TargetRef: "refs/heads/develop", ExitCode: 1, Error: "not ready"}
	out, _ := json.Marshal(run)
	data, _ := json.Marshal(legacyEvidence{SchemaVersion: 1, OriginalReceiptSHA256: strings.Repeat("c", 64), SingleCommandDiagnostics: legacyDiagnostic{Command: "/historical/gz-git", Executable: "/historical/gz-git", ExecutableSHA256: legacyArtifactSHA256, Cwd: root, Args: args, ExitCode: 1, Stdout: string(out)}})
	o.EvidenceFile = filepath.Join(t.TempDir(), "receipt.json")
	if err = os.WriteFile(o.EvidenceFile, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = ReconcileLegacyPreMutation(context.Background(), gitcmd.NewExecutor(), o); err == nil {
		t.Fatal("prior receipt accepted")
	}
}

func TestValidateLegacyRunRejectsSyntheticPositiveAndWrongContext(t *testing.T) {
	t.Parallel()
	o := LegacyReconcileOptions{Target: "develop", Execution: &ExecutionContext{ExecutionID: "run-1", Generation: "g1", Owner: OwnerIdentity{Actor: "codex", Host: "host", Kind: "agent"}, ExpectSource: strings.Repeat("a", 40)}}
	args := []string{"integrate", "run", "--target", "develop", "--expect-source", o.Execution.ExpectSource, "--execution-id", "run-1", "--generation", "g1", "--owner-actor", "codex", "--owner-host", "host", "--owner-kind", "agent", "--json"}
	run := legacyRun{SchemaVersion: 1, Command: "integrate-run", TargetRef: "refs/heads/develop", ReclaimDone: nil, ReclaimFailed: nil, ExitCode: 1, Error: "not ready"}
	out, _ := json.Marshal(run)
	d := legacyDiagnostic{Command: "/bin/gz", Executable: "/bin/gz", Cwd: "/tmp", Args: args, ExitCode: 1, Stdout: string(out)}
	if err := validateLegacyRun(d, o, "/tmp"); err != nil {
		t.Fatalf("valid legacy shape rejected: %v", err)
	}
	run.Integrated = true
	out, _ = json.Marshal(run)
	d.Stdout = string(out)
	if err := validateLegacyRun(d, o, "/tmp"); err == nil {
		t.Fatal("positive legacy run accepted")
	}
	run.Integrated = false
	run.ResourceWaitDurationMS = 0
	out, _ = json.Marshal(run)
	d.Stdout = strings.Replace(string(out), `"resource_wait_duration_ms":0`, `"resource_wait_duration_ms":null`, 1)
	if err := validateLegacyRun(d, o, "/tmp"); err == nil {
		t.Fatal("null duration accepted")
	}
	d.Stdout = string(out)
	d.Args = append([]string(nil), args...)
	d.Args[3] = "other"
	if err := validateLegacyRun(d, o, "/tmp"); err == nil {
		t.Fatal("wrong target arguments accepted")
	}
}

func TestReconciledOutcomeRejectsPartialProvenance(t *testing.T) {
	t.Parallel()
	o := &PreMutationOutcome{SchemaVersion: 1, Capability: preMutationOutcomeCapability, ExecutionID: "run-1", Generation: "g1", Owner: OwnerIdentity{Actor: "codex", Host: "host", Kind: "agent"}, RepositoryCommonDir: "/tmp/git", TaskBranch: "feature/x", TaskSHA: strings.Repeat("a", 40), ExpectedSource: strings.Repeat("a", 40), TargetRef: "refs/heads/develop", PreMutation: true, Phase: preMutationPhaseReadinessRejected, ReasonClass: "not_ready", ReconciledReceiptSHA256: strings.Repeat("a", 64)}
	if err := o.validate(); err == nil {
		t.Fatal("partial reconciliation provenance accepted")
	}
}

func TestRecordPreMutationOutcomeRetainsReconciledProvenance(t *testing.T) {
	t.Parallel()
	fx := readyTaskFixture(t)
	exec := outcomeExecution(t, fx.Worktree, "reconciled-preserved")
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	capture := &preMutationCapture{execution: *exec}
	if err := capture.capture(context.Background(), g, targetPreflight{branch: "dev/actor/feat/task", targetRef: "refs/heads/develop"}); err != nil {
		t.Fatal(err)
	}
	if capture.candidate == nil {
		t.Fatal("missing bound candidate")
	}
	reconciled := *capture.candidate
	reconciled.Phase, reconciled.ReasonClass = preMutationPhaseReadinessRejected, "not_ready"
	reconciled.ReconciledReceiptSHA256 = strings.Repeat("a", 64)
	reconciled.LegacyArtifactSHA256 = strings.Repeat("b", 64)
	reconciled.ReconciledDiagnosticSHA256 = strings.Repeat("c", 64)
	reconciled.LegacySourceRevision = strings.Repeat("d", 40)
	if err := reconciled.validate(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(preMutationOutcomePath(reconciled.RepositoryCommonDir, reconciled.ExecutionID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(&reconciled)
	if err != nil {
		t.Fatal(err)
	}
	if err := atomicStateWrite(preMutationOutcomePath(reconciled.RepositoryCommonDir, reconciled.ExecutionID), b); err != nil {
		t.Fatal(err)
	}
	got, err := recordPreMutationOutcome(context.Background(), capture, preMutationPhaseReadinessRejected)
	if err != nil || got == nil || got.ReconciledDiagnosticSHA256 != reconciled.ReconciledDiagnosticSHA256 {
		t.Fatalf("outcome=%+v err=%v", got, err)
	}
}

func TestValidateLockedRepositoryBindingRejectsReplacement(t *testing.T) {
	t.Parallel()
	if err := validateLockedRepositoryBinding("/repo", "/repo/.git", "/repo", "/repo", "/repo/.git"); err != nil {
		t.Fatalf("unchanged binding: %v", err)
	}
	if err := validateLockedRepositoryBinding("/repo", "/repo/.git", "/repo", "/other", "/repo/.git"); err == nil {
		t.Fatal("changed root accepted")
	}
	if err := validateLockedRepositoryBinding("/repo", "/repo/.git", "/repo", "/repo", "/other/.git"); err == nil {
		t.Fatal("changed common directory accepted")
	}
}
