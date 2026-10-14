package integrate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

const (
	legacyReconcileCapability       = "integrate-reconcile-pre-mutation-v1"
	legacyArtifactSHA256            = "36bd85e8b21d6b0aaecb2f33d55f98bbae462b0bf4d7472142c6a9bd23a77e20"
	legacyArtifactRevision          = "b32360a11619f176a4bb848bdfa274c45ced9dad"
	maxLegacyEvidenceBytes    int64 = 1 << 20
	maxLegacyArtifactBytes    int64 = 256 << 20
)

// LegacyReconcileOptions binds one legacy refusal to the current execution.
type LegacyReconcileOptions struct {
	RepoPath, Branch, Target, EvidenceFile, LegacyArtifact string
	NoFetch                                                bool
	Execution                                              *ExecutionContext
}

// LegacyReconcileReport contains the durable reconciled refusal.
type LegacyReconcileReport struct {
	Outcome                                                                 *PreMutationOutcome
	OriginalReceiptSHA256, LegacyArtifactSHA256, ReconciledDiagnosticSHA256 string
}

// LegacyReconcileProtocol is the machine response for integrate reconcile.
type LegacyReconcileProtocol struct {
	SchemaVersion              int                 `json:"schema_version"`
	Command                    string              `json:"command"`
	Reconciled                 bool                `json:"reconciled"`
	ExitCode                   int                 `json:"exit_code"`
	OriginalReceiptSHA256      string              `json:"original_receipt_sha256"`
	LegacyArtifactSHA256       string              `json:"legacy_artifact_sha256"`
	ReconciledDiagnosticSHA256 string              `json:"reconciled_diagnostic_sha256"`
	PreMutationOutcome         *PreMutationOutcome `json:"pre_mutation_outcome"`
}
type legacyDiagnostic struct {
	Command          string   `json:"command"`
	Executable       string   `json:"executable"`
	ExecutableSHA256 string   `json:"executable_sha256"`
	Cwd              string   `json:"cwd"`
	Args             []string `json:"args"`
	ExitCode         int      `json:"exit_code"`
	Stdout           string   `json:"stdout"`
}
type legacyEvidence struct {
	SchemaVersion            int              `json:"schema_version"`
	OriginalReceiptSHA256    string           `json:"original_receipt_sha256"`
	SingleCommandDiagnostics legacyDiagnostic `json:"single_command_diagnostics"`
}
type legacyRun struct {
	SchemaVersion          int      `json:"schema_version"`
	Command                string   `json:"command"`
	Integrated             bool     `json:"integrated"`
	ExecutionID            string   `json:"execution_id"`
	Generation             string   `json:"generation"`
	TaskSHA                string   `json:"task_sha"`
	TargetRef              string   `json:"target_ref"`
	SourcePushed           bool     `json:"source_pushed"`
	ReceiptPhase           string   `json:"receipt_phase"`
	WorktreeRemoved        bool     `json:"worktree_removed"`
	LocalBranchRemoved     bool     `json:"local_branch_removed"`
	RemoteBranchRemoved    bool     `json:"remote_branch_removed"`
	ReclaimSkipped         string   `json:"reclaim_skipped"`
	ReclaimDone            []string `json:"reclaim_done"`
	ReclaimFailed          []string `json:"reclaim_failed"`
	ResourceWaitDurationMS int      `json:"resource_wait_duration_ms"`
	ValidationDurationMS   int      `json:"validation_duration_ms"`
	ExitCode               int      `json:"exit_code"`
	Error                  string   `json:"error"`
}
type legacyPrepared struct {
	evidence     legacyEvidence
	diagnostic   legacyDiagnostic
	repo         gitRepo
	root, common string
}

// ReconcileLegacyPreMutation attests one verified b323 readiness refusal.
func ReconcileLegacyPreMutation(ctx context.Context, exec *gitcmd.Executor, opts LegacyReconcileOptions) (*LegacyReconcileReport, error) {
	prepared, err := prepareLegacyReconciliation(ctx, exec, opts)
	if err != nil {
		return nil, err
	}
	evidence, diag, g, root, common := prepared.evidence, prepared.diagnostic, prepared.repo, prepared.root, prepared.common
	outer, err := acquireExecutionRunLock(ctx, common, opts.Execution.ExecutionID)
	if err != nil {
		return nil, err
	}
	defer outer.release()
	lockedRoot, checkErr := g.toplevel(ctx)
	if checkErr != nil {
		return nil, checkErr
	}
	lockedCommon, checkErr := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if checkErr != nil {
		return nil, checkErr
	}
	if err := validateLockedRepositoryBinding(root, common, diag.Cwd, lockedRoot, lockedCommon); err != nil {
		return nil, err
	}
	if dirty, checkErr := g.porcelain(ctx); checkErr != nil || dirty != "" {
		if checkErr != nil {
			return nil, checkErr
		}
		return nil, fmt.Errorf("current worktree is not clean")
	}
	branch, checkErr := g.currentBranch(ctx)
	if checkErr != nil || (opts.Branch != "" && branch != opts.Branch) {
		return nil, fmt.Errorf("current branch does not match legacy context")
	}
	if opts.Branch == "" {
		opts.Branch = branch
	}
	head, checkErr := g.headSHA(ctx)
	if checkErr != nil || head != opts.Execution.ExpectSource {
		return nil, fmt.Errorf("current head does not match expected source")
	}
	targetBranch := strings.TrimPrefix(strings.TrimPrefix(opts.Target, "refs/heads/"), "origin/")
	diagnosticDigest, err := legacyDiagnosticDigest(diag)
	if err != nil {
		return nil, err
	}
	candidate := &PreMutationOutcome{SchemaVersion: evidenceSchemaVersion, Capability: preMutationOutcomeCapability, ExecutionID: opts.Execution.ExecutionID, Generation: opts.Execution.Generation, Owner: opts.Execution.Owner, RepositoryCommonDir: common, TaskBranch: opts.Branch, TaskSHA: head, ExpectedSource: opts.Execution.ExpectSource, TargetRef: "refs/heads/" + targetBranch, PreMutation: true, Phase: preMutationPhaseReadinessRejected, ReasonClass: "not_ready", ReconciledReceiptSHA256: evidence.OriginalReceiptSHA256, LegacyArtifactSHA256: diag.ExecutableSHA256, ReconciledDiagnosticSHA256: diagnosticDigest, LegacySourceRevision: legacyArtifactRevision}
	if err := candidate.validate(); err != nil {
		return nil, err
	}
	return persistLegacyOutcome(ctx, common, opts.Execution.ExecutionID, evidence, diag, candidate)
}

func persistLegacyOutcome(ctx context.Context, common, executionID string, evidence legacyEvidence, diag legacyDiagnostic, candidate *PreMutationOutcome) (*LegacyReconcileReport, error) {
	lock, err := acquireReceiptWriteLock(ctx, common, executionID)
	if err != nil {
		return nil, err
	}
	defer lock.release()
	if r, ok, err := readReceipt(common, executionID); err != nil {
		return nil, err
	} else if ok {
		return nil, fmt.Errorf("execution-id %s already has receipt phase=%s", executionID, r.Phase)
	}
	if prior, ok, err := readPreMutationOutcome(common, executionID); err != nil {
		return nil, err
	} else if ok {
		if !sameReconciledOutcome(prior, candidate) {
			return nil, fmt.Errorf("execution-id %s has contradictory outcome", executionID)
		}
		return &LegacyReconcileReport{prior, evidence.OriginalReceiptSHA256, diag.ExecutableSHA256, candidate.ReconciledDiagnosticSHA256}, nil
	}
	data, err := json.Marshal(candidate)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(preMutationOutcomePath(common, executionID)), 0o700); err != nil { // #nosec G301 -- Git common directory is repository-owned
		return nil, fmt.Errorf("create pre-mutation outcome directory: %w", err)
	}
	if err := atomicStateWrite(preMutationOutcomePath(common, executionID), append(data, '\n')); err != nil {
		return nil, err
	}
	return &LegacyReconcileReport{candidate, evidence.OriginalReceiptSHA256, diag.ExecutableSHA256, candidate.ReconciledDiagnosticSHA256}, nil
}

func prepareLegacyReconciliation(ctx context.Context, exec *gitcmd.Executor, opts LegacyReconcileOptions) (legacyPrepared, error) {
	if exec == nil {
		return legacyPrepared{}, fmt.Errorf("git executor is nil")
	}
	if opts.Execution == nil {
		return legacyPrepared{}, ErrInvalidExecutionContext
	}
	if err := opts.Execution.Validate(); err != nil {
		return legacyPrepared{}, err
	}
	if !filepath.IsAbs(opts.EvidenceFile) || filepath.Clean(opts.EvidenceFile) != opts.EvidenceFile || !filepath.IsAbs(opts.LegacyArtifact) || filepath.Clean(opts.LegacyArtifact) != opts.LegacyArtifact {
		return legacyPrepared{}, fmt.Errorf("evidence-file and legacy-artifact must be clean absolute paths")
	}
	evidence, diag, err := readLegacyEvidence(opts.EvidenceFile)
	if err != nil {
		return legacyPrepared{}, err
	}
	if err := verifyLegacyArtifact(opts.LegacyArtifact, diag.ExecutableSHA256); err != nil {
		return legacyPrepared{}, err
	}
	dir := opts.RepoPath
	if dir == "" {
		dir = "."
	}
	g := newGitRepo(exec, dir)
	root, err := g.toplevel(ctx)
	if err != nil {
		return legacyPrepared{}, err
	}
	if root != diag.Cwd {
		return legacyPrepared{}, fmt.Errorf("legacy diagnostic cwd does not match current worktree")
	}
	common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return legacyPrepared{}, err
	}
	if err := validateLegacyRun(diag, opts, root); err != nil {
		return legacyPrepared{}, err
	}
	return legacyPrepared{evidence, diag, g, root, common}, nil
}

// BuildLegacyReconcileProtocol renders a successful reconciliation response.
func BuildLegacyReconcileProtocol(r *LegacyReconcileReport) *LegacyReconcileProtocol {
	return &LegacyReconcileProtocol{1, "integrate-reconcile", true, 0, r.OriginalReceiptSHA256, r.LegacyArtifactSHA256, r.ReconciledDiagnosticSHA256, r.Outcome}
}

func readLegacyEvidence(path string) (legacyEvidence, legacyDiagnostic, error) {
	b, err := readBoundedRegularFile(path, maxLegacyEvidenceBytes, "legacy evidence")
	if err != nil {
		return legacyEvidence{}, legacyDiagnostic{}, err
	}
	var raw map[string]json.RawMessage
	if err := rejectDuplicateJSONKeys(b); err != nil {
		return legacyEvidence{}, legacyDiagnostic{}, err
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return legacyEvidence{}, legacyDiagnostic{}, err
	}
	if len(raw) != 3 || raw["schema_version"] == nil || raw["original_receipt_sha256"] == nil || raw["single_command_diagnostics"] == nil || isJSONNull(raw["schema_version"]) || isJSONNull(raw["original_receipt_sha256"]) || isJSONNull(raw["single_command_diagnostics"]) {
		return legacyEvidence{}, legacyDiagnostic{}, fmt.Errorf("legacy evidence shape is invalid")
	}
	var e legacyEvidence
	if err := jsonUnmarshalStrict(b, &e); err != nil {
		return e, legacyDiagnostic{}, err
	}
	if e.SchemaVersion != 1 || !isLowerSHA256(e.OriginalReceiptSHA256) {
		return e, legacyDiagnostic{}, fmt.Errorf("legacy evidence is invalid")
	}
	if err := validateLegacyDiagnosticShape(e.SingleCommandDiagnostics); err != nil {
		return e, legacyDiagnostic{}, err
	}
	return e, e.SingleCommandDiagnostics, nil
}

func verifyLegacyArtifact(path, want string) error {
	b, err := readBoundedRegularFile(path, maxLegacyArtifactBytes, "legacy artifact")
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	got := hex.EncodeToString(sum[:])
	if got != legacyArtifactSHA256 || want != got {
		return fmt.Errorf("legacy artifact SHA-256 is not allowlisted")
	}
	info, err := buildinfo.Read(bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("read legacy build info: %w", err)
	}
	revision, modified := "", ""
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			revision = s.Value
		}
		if s.Key == "vcs.modified" {
			modified = s.Value
		}
	}
	if revision != legacyArtifactRevision || modified != "false" {
		return fmt.Errorf("legacy artifact build provenance is not clean b323")
	}
	return nil
}

func legacyDiagnosticDigest(d legacyDiagnostic) (string, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("marshal legacy diagnostic: %w", err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

func validateLegacyRun(d legacyDiagnostic, o LegacyReconcileOptions, cwd string) error {
	if !validLegacyDiagnosticBinding(d, cwd) {
		return fmt.Errorf("legacy diagnostic binding is invalid")
	}
	if !sameLegacyContextArgs(d.Args, o) {
		return fmt.Errorf("legacy diagnostic arguments do not match execution context")
	}
	if err := rejectDuplicateJSONKeys([]byte(d.Stdout)); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(d.Stdout), &raw); err != nil {
		return err
	}
	if err := validateLegacyRunShape(raw); err != nil {
		return err
	}
	var run legacyRun
	if err := jsonUnmarshalStrict([]byte(d.Stdout), &run); err != nil {
		return err
	}
	if !legacyRunMatchesReadinessRefusal(run, o.Target) {
		return fmt.Errorf("legacy run JSON is not the b323 readiness refusal")
	}
	return nil
}

func legacyRunMatchesReadinessRefusal(run legacyRun, target string) bool {
	targetBranch := strings.TrimPrefix(strings.TrimPrefix(target, "refs/heads/"), "origin/")
	return legacyRunIdentityIsEmpty(run) && legacyRunMutationIsNegative(run) && run.SchemaVersion == 1 && run.Command == "integrate-run" && run.TargetRef == "refs/heads/"+targetBranch && run.ResourceWaitDurationMS >= 0 && run.ValidationDurationMS >= 0 && run.ExitCode == 1 && run.Error == "not ready"
}

func legacyRunIdentityIsEmpty(run legacyRun) bool {
	return !run.Integrated && run.ExecutionID == "" && run.Generation == "" && run.TaskSHA == "" && run.ReceiptPhase == "" && run.ReclaimSkipped == ""
}

func legacyRunMutationIsNegative(run legacyRun) bool {
	return !run.SourcePushed && !run.WorktreeRemoved && !run.LocalBranchRemoved && !run.RemoteBranchRemoved && run.ReclaimDone == nil && run.ReclaimFailed == nil
}

func validLegacyDiagnosticBinding(d legacyDiagnostic, cwd string) bool {
	return d.ExitCode == 1 && d.Command != "" && d.Command == d.Executable && d.Cwd == cwd && filepath.IsAbs(d.Command) && filepath.IsAbs(d.Executable) && len(d.Args) > 0
}

func sameLegacyContextArgs(args []string, o LegacyReconcileOptions) bool {
	want := []string{"integrate", "run", "--target", o.Target}
	if o.NoFetch {
		want = append(want, "--no-fetch")
	}
	want = append(want, "--expect-source", o.Execution.ExpectSource, "--execution-id", o.Execution.ExecutionID, "--generation", o.Execution.Generation, "--owner-actor", o.Execution.Owner.Actor, "--owner-host", o.Execution.Owner.Host, "--owner-kind", o.Execution.Owner.Kind, "--json")
	if len(args) != len(want) {
		return false
	}
	for i := range want {
		if args[i] != want[i] {
			return false
		}
	}
	return true
}

func validateLegacyRunShape(raw map[string]json.RawMessage) error {
	if len(raw) != 19 {
		return fmt.Errorf("legacy run JSON field count is invalid")
	}
	for _, key := range []string{"schema_version", "command", "integrated", "execution_id", "generation", "task_sha", "target_ref", "source_pushed", "receipt_phase", "worktree_removed", "local_branch_removed", "remote_branch_removed", "reclaim_skipped", "resource_wait_duration_ms", "validation_duration_ms", "exit_code", "error"} {
		if raw[key] == nil || isJSONNull(raw[key]) {
			return fmt.Errorf("legacy run JSON missing or null %s", key)
		}
	}
	for _, key := range []string{"reclaim_done", "reclaim_failed"} {
		if raw[key] == nil || !isJSONNull(raw[key]) {
			return fmt.Errorf("legacy run JSON %s must be null", key)
		}
	}
	return nil
}

func sameReconciledOutcome(a, b *PreMutationOutcome) bool {
	return samePreMutationIdentity(a, b) && a.ReconciledReceiptSHA256 == b.ReconciledReceiptSHA256 && a.LegacyArtifactSHA256 == b.LegacyArtifactSHA256 && a.ReconciledDiagnosticSHA256 == b.ReconciledDiagnosticSHA256 && a.LegacySourceRevision == b.LegacySourceRevision
}

// LegacyReconcileCapability identifies the supported reconciliation contract.
func LegacyReconcileCapability() string { return legacyReconcileCapability }

func validateLockedRepositoryBinding(root, common, diagnosticCWD, lockedRoot, lockedCommon string) error {
	if lockedRoot != root || lockedRoot != diagnosticCWD {
		return fmt.Errorf("repository root changed while waiting for execution lock")
	}
	if lockedCommon == "" || lockedCommon != common {
		return fmt.Errorf("repository common directory changed while waiting for execution lock")
	}
	return nil
}

func readBoundedRegularFile(path string, limit int64, label string) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", label, err)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular non-symlink file", label)
	}
	if before.Size() > limit {
		return nil, fmt.Errorf("%s exceeds %d-byte limit", label, limit)
	}
	f, err := os.Open(path) // #nosec G304 -- caller supplied file is checked below before parsing
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", label, err)
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", label, err)
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return nil, fmt.Errorf("%s changed or is not a regular file", label)
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s exceeds %d-byte limit", label, limit)
	}
	return b, nil
}

func isJSONNull(value json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}

func validateLegacyDiagnosticShape(d legacyDiagnostic) error {
	if d.Command == "" || d.Executable == "" || d.ExecutableSHA256 == "" || d.Cwd == "" || d.Args == nil || d.Stdout == "" || !filepath.IsAbs(d.Command) || !filepath.IsAbs(d.Executable) || !filepath.IsAbs(d.Cwd) || !isLowerSHA256(d.ExecutableSHA256) {
		return fmt.Errorf("legacy diagnostic shape is invalid")
	}
	return nil
}
