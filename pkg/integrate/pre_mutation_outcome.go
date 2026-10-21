// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

const (
	preMutationOutcomeCapability = "pre-mutation-outcome/v1"
	preMutationOutcomesSuffix    = "outcomes"

	preMutationPhaseTargetLockWait    = "target_lock_wait"
	preMutationPhaseReadinessRejected = "readiness_rejected"
)

var errPreMutationOutcomeUnbound = errors.New("pre-mutation outcome is not bound to the current execution")

// PreMutationOutcome is a durable refusal bound to one complete execution
// context. It is deliberately separate from Receipt: receipts describe runs
// that reached their intent boundary, while this document proves only that
// this invocation stopped before that boundary and before any target push.
// Its absence never proves that a push did not occur.
type PreMutationOutcome struct {
	SchemaVersion              int           `json:"schema_version"`
	Capability                 string        `json:"capability"`
	ExecutionID                string        `json:"execution_id"`
	Generation                 string        `json:"generation"`
	Owner                      OwnerIdentity `json:"owner"`
	RepositoryCommonDir        string        `json:"repository_common_dir"`
	TaskBranch                 string        `json:"task_branch"`
	TaskSHA                    string        `json:"task_sha"`
	ExpectedSource             string        `json:"expected_source"`
	TargetRef                  string        `json:"target_ref"`
	PreMutation                bool          `json:"pre_mutation"`
	Phase                      string        `json:"phase"`
	ReasonClass                string        `json:"reason_class"`
	ReconciledReceiptSHA256    string        `json:"reconciled_receipt_sha256,omitempty"`
	LegacyArtifactSHA256       string        `json:"legacy_artifact_sha256,omitempty"`
	ReconciledDiagnosticSHA256 string        `json:"reconciled_diagnostic_sha256,omitempty"`
	LegacySourceRevision       string        `json:"legacy_source_revision,omitempty"`
}

// preMutationCapture carries a fully bound candidate across Check's target
// ownership wait. It is in-memory until Run classifies a refusal; Check itself
// remains read-only and can never mint a negative outcome.
type preMutationCapture struct {
	execution ExecutionContext
	candidate *PreMutationOutcome
}

func (c *preMutationCapture) capture(ctx context.Context, g gitRepo, preflight targetPreflight) error {
	if c == nil {
		return nil
	}
	common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || common == "" {
		if err == nil {
			err = fmt.Errorf("git common directory is empty")
		}
		return fmt.Errorf("resolve pre-mutation common directory: %w", err)
	}
	sha, ok, err := g.revParse(ctx, preflight.branch)
	if err != nil {
		return err
	}
	// A source that differs from the approval cannot receive a retry-safe
	// refusal. Check will surface its normal mismatch without durable outcome.
	if !ok || sha != c.execution.ExpectSource {
		return nil
	}
	c.candidate = &PreMutationOutcome{
		SchemaVersion:       evidenceSchemaVersion,
		Capability:          preMutationOutcomeCapability,
		ExecutionID:         c.execution.ExecutionID,
		Generation:          c.execution.Generation,
		Owner:               c.execution.Owner,
		RepositoryCommonDir: common,
		TaskBranch:          preflight.branch,
		TaskSHA:             sha,
		ExpectedSource:      c.execution.ExpectSource,
		TargetRef:           preflight.targetRef,
		PreMutation:         true,
	}
	return nil
}

func preMutationOutcomePath(common, executionID string) string {
	digest := sha256ExecutionID(executionID)
	return filepath.Join(evidenceRoot(common), preMutationOutcomesSuffix, digest+".json")
}

func evidenceExecutionRunLockPath(common, executionID string) string {
	return evidenceReceiptPath(common, executionID) + ".run"
}

// acquireExecutionRunLock serializes every execution-context run for one
// execution id from preflight through push/reclaim. Receipt writes retain
// their narrower CAS lock; this distinct outer lock prevents one invocation
// from recording a pre-push refusal while another invocation with the same
// identity advances to intent or pushes.
func acquireExecutionRunLock(ctx context.Context, common, executionID string) (*receiptWriteLock, error) {
	dir := filepath.Dir(evidenceReceiptPath(common, executionID))
	if err := os.MkdirAll(dir, 0o700); err != nil { // #nosec G301 -- Git common directory is repository-owned
		return nil, fmt.Errorf("create integration run lock directory: %w", err)
	}
	f, err := os.OpenFile(evidenceExecutionRunLockPath(common, executionID), os.O_RDWR|os.O_CREATE, 0o600) // #nosec G304 -- fixed SHA-256 name below Git common directory
	if err != nil {
		return nil, fmt.Errorf("open integration run lock: %w", err)
	}
	for {
		locked, lockErr := tryLockFile(f)
		if lockErr != nil {
			_ = f.Close()
			return nil, fmt.Errorf("lock integration run %s: %w", executionID, lockErr)
		}
		if locked {
			return &receiptWriteLock{file: f}, nil
		}
		timer := time.NewTimer(lockPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = f.Close()
			return nil, fmt.Errorf("wait for integration run %s: %w", executionID, ctx.Err())
		case <-timer.C:
		}
	}
}

func sha256ExecutionID(executionID string) string {
	digest := sha256.Sum256([]byte(executionID))
	return fmt.Sprintf("%x", digest[:])
}

func readPreMutationOutcome(common, executionID string) (*PreMutationOutcome, bool, error) {
	path := preMutationOutcomePath(common, executionID)
	data, err := os.ReadFile(path) // #nosec G304 -- fixed SHA-256 name below Git common directory
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read pre-mutation outcome: %w", err)
	}
	var outcome PreMutationOutcome
	if err := jsonUnmarshalStrict(data, &outcome); err != nil {
		return nil, false, fmt.Errorf("parse pre-mutation outcome %s: %w", path, err)
	}
	if err := outcome.validate(); err != nil {
		return nil, false, fmt.Errorf("invalid pre-mutation outcome %s: %w", path, err)
	}
	return &outcome, true, nil
}

func (o *PreMutationOutcome) validate() error {
	if o == nil {
		return fmt.Errorf("outcome is nil")
	}
	if o.SchemaVersion != evidenceSchemaVersion {
		return fmt.Errorf("unsupported schema_version %d", o.SchemaVersion)
	}
	if o.Capability != preMutationOutcomeCapability || !o.PreMutation {
		return fmt.Errorf("not a pre-mutation-outcome/v1 document")
	}
	executionContext := ExecutionContext{ExecutionID: o.ExecutionID, Generation: o.Generation, Owner: o.Owner, ExpectSource: o.ExpectedSource}
	if err := executionContext.Validate(); err != nil {
		return err
	}
	if !filepath.IsAbs(o.RepositoryCommonDir) || filepath.Clean(o.RepositoryCommonDir) != o.RepositoryCommonDir {
		return fmt.Errorf("repository_common_dir is not a clean absolute path")
	}
	if err := gitcmd.SanitizeBranchName(o.TaskBranch); err != nil {
		return fmt.Errorf("invalid task_branch: %w", err)
	}
	sha, err := NormalizeExpectSource(o.TaskSHA)
	if err != nil || sha != o.ExpectedSource {
		return fmt.Errorf("task_sha does not match expected_source")
	}
	if !strings.HasPrefix(o.TargetRef, "refs/heads/") {
		return fmt.Errorf("target_ref is not a head ref")
	}
	if err := gitcmd.SanitizeBranchName(strings.TrimPrefix(o.TargetRef, "refs/heads/")); err != nil {
		return fmt.Errorf("invalid target_ref: %w", err)
	}
	if err := o.validateReconciliationProvenance(); err != nil {
		return err
	}
	return validatePreMutationPhase(o.Phase, o.ReasonClass)
}

func (o *PreMutationOutcome) validateReconciliationProvenance() error {
	values := []string{o.ReconciledReceiptSHA256, o.LegacyArtifactSHA256, o.ReconciledDiagnosticSHA256, o.LegacySourceRevision}
	present := 0
	for _, value := range values {
		if value != "" {
			present++
		}
	}
	if present != 0 && present != len(values) {
		return fmt.Errorf("incomplete legacy reconciliation provenance")
	}
	for _, value := range values[:3] {
		if value != "" && !isLowerSHA256(value) {
			return fmt.Errorf("invalid reconciliation SHA-256")
		}
	}
	if values[3] != "" {
		if _, err := NormalizeExpectSource(values[3]); err != nil {
			return fmt.Errorf("invalid legacy source revision")
		}
	}
	return nil
}

func validatePreMutationPhase(phase, reason string) error {
	switch phase {
	case preMutationPhaseTargetLockWait:
		if reason != "target_lock_wait" {
			return fmt.Errorf("target_lock_wait reason_class is %q", reason)
		}
	case preMutationPhaseReadinessRejected:
		if reason != "not_ready" {
			return fmt.Errorf("readiness_rejected reason_class is %q", reason)
		}
	default:
		return fmt.Errorf("unknown pre-mutation phase %q", phase)
	}
	return nil
}

func isLowerSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func samePreMutationIdentity(left, right *PreMutationOutcome) bool {
	return left != nil && right != nil && left.ExecutionID == right.ExecutionID && left.Generation == right.Generation &&
		left.Owner.sameAs(right.Owner) && left.RepositoryCommonDir == right.RepositoryCommonDir &&
		left.TaskBranch == right.TaskBranch && left.TaskSHA == right.TaskSHA && left.ExpectedSource == right.ExpectedSource &&
		left.TargetRef == right.TargetRef
}

// recordPreMutationOutcome writes a refusal only after Run has classified the
// current control-flow path and acquired the execution-run lock. A durable
// receipt of any phase blocks it: an old negative outcome must never override
// an intent or a possibly pushed run.
func recordPreMutationOutcome(ctx context.Context, capture *preMutationCapture, phase string) (*PreMutationOutcome, error) {
	if capture == nil || capture.candidate == nil {
		return nil, errPreMutationOutcomeUnbound
	}
	candidate := *capture.candidate
	candidate.Phase = phase
	switch phase {
	case preMutationPhaseTargetLockWait:
		candidate.ReasonClass = "target_lock_wait"
	case preMutationPhaseReadinessRejected:
		candidate.ReasonClass = "not_ready"
	default:
		return nil, fmt.Errorf("unknown pre-mutation phase %q", phase)
	}
	if err := candidate.validate(); err != nil {
		return nil, err
	}
	lock, err := acquireReceiptWriteLock(ctx, candidate.RepositoryCommonDir, candidate.ExecutionID)
	if err != nil {
		return nil, err
	}
	defer lock.release()
	if receipt, exists, err := readReceipt(candidate.RepositoryCommonDir, candidate.ExecutionID); err != nil {
		return nil, err
	} else if exists {
		return nil, fmt.Errorf("execution-id %s already has receipt phase=%s source_pushed=%t", candidate.ExecutionID, receipt.Phase, receipt.SourcePushed)
	}
	if recorded, exists, err := readPreMutationOutcome(candidate.RepositoryCommonDir, candidate.ExecutionID); err != nil {
		return nil, err
	} else if exists && !samePreMutationIdentity(recorded, &candidate) {
		return nil, fmt.Errorf("execution-id %s has contradictory pre-mutation identity", candidate.ExecutionID)
	} else if exists && recorded.ReconciledReceiptSHA256 != "" {
		// A legacy reconciliation is an attestation, not a retry cache. A later
		// normal refusal must not replace its immutable provenance with an
		// ordinary outcome document.
		if phase != preMutationPhaseReadinessRejected {
			return nil, fmt.Errorf("execution-id %s has reconciled readiness outcome", candidate.ExecutionID)
		}
		return recorded, nil
	}
	dir := filepath.Dir(preMutationOutcomePath(candidate.RepositoryCommonDir, candidate.ExecutionID))
	if err := os.MkdirAll(dir, 0o700); err != nil { // #nosec G301 -- Git common directory is repository-owned
		return nil, fmt.Errorf("create pre-mutation outcome directory: %w", err)
	}
	data, err := json.Marshal(&candidate)
	if err != nil {
		return nil, fmt.Errorf("marshal pre-mutation outcome: %w", err)
	}
	if err := atomicStateWrite(preMutationOutcomePath(candidate.RepositoryCommonDir, candidate.ExecutionID), append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write pre-mutation outcome: %w", err)
	}
	return &candidate, nil
}
