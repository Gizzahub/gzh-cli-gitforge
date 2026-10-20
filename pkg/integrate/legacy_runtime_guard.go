// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	legacyRuntimeGuardSchema     = 1
	legacyRuntimeGuardCapability = "ce-legacy-cleanup-guard-v1"
	legacyAttestationCapability  = "ce-legacy-cleanup-attestation-v1"
	legacyAttestationMaxOutput   = 64 << 10
	legacyAttestationTimeout     = 10 * time.Second
	legacyProcessStartTolerance  = 2 * time.Second
)

// LegacyRuntimeGuard binds a plan to the CE verifier selected by the host.
// The verifier is an external trust boundary: native code verifies its on-disk
// identity and asks it only for the public attestation protocol.
type LegacyRuntimeGuard struct {
	SchemaVersion      int           `json:"schema_version"`
	Capability         string        `json:"capability"`
	OperationID        string        `json:"operation_id"`
	RepositoryRoot     string        `json:"repository_root"`
	CommonDir          string        `json:"common_dir"`
	TaskBranch         string        `json:"task_branch"`
	TaskWorktree       string        `json:"task_worktree"`
	TaskOID            string        `json:"task_oid"`
	Owner              OwnerIdentity `json:"owner"`
	VerifierExecutable string        `json:"verifier_executable"`
	VerifierSHA256     string        `json:"verifier_sha256"`
}

// ValidateForPlan confirms that this CE-owned guard is exactly bound to plan.
func (g *LegacyRuntimeGuard) ValidateForPlan(p *LegacyCleanupPlan) error {
	if g == nil || p == nil {
		return fmt.Errorf("legacy runtime guard is required")
	}
	if g.SchemaVersion != legacyRuntimeGuardSchema || g.Capability != legacyRuntimeGuardCapability || !validLowerHex(g.OperationID, 32) {
		return fmt.Errorf("unsupported or invalid legacy runtime guard")
	}
	if !g.Owner.sameAs(p.Owner) || g.RepositoryRoot != p.RepositoryRoot || g.CommonDir != p.CommonDir || g.TaskBranch != p.TaskBranch || g.TaskWorktree != p.TaskWorktree || g.TaskOID != p.TaskOID {
		return fmt.Errorf("legacy runtime guard does not bind to cleanup plan")
	}
	if !filepath.IsAbs(g.VerifierExecutable) || filepath.Clean(g.VerifierExecutable) != g.VerifierExecutable || !validSHA256(g.VerifierSHA256) {
		return fmt.Errorf("legacy runtime guard verifier identity is invalid")
	}
	return nil
}

func validLowerHex(value string, minLength int) bool {
	if len(value) < minLength || len(value)%2 != 0 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

// ReadLegacyRuntimeGuard loads the CE-owned public verifier binding. It reads
// no CE runtime state or receipt; only the explicit guard document is parsed.
func ReadLegacyRuntimeGuard(path string) (*LegacyRuntimeGuard, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("runtime guard path must be a clean absolute path")
	}
	data, err := readBoundedRegularFile(path, legacyCleanupMaxPlanBytes, "legacy runtime guard")
	if err != nil {
		return nil, err
	}
	var guard LegacyRuntimeGuard
	if err := strictSingleJSON(data, &guard); err != nil {
		return nil, err
	}
	if guard.SchemaVersion != legacyRuntimeGuardSchema || guard.Capability != legacyRuntimeGuardCapability || !validLowerHex(guard.OperationID, 32) || !filepath.IsAbs(guard.VerifierExecutable) || filepath.Clean(guard.VerifierExecutable) != guard.VerifierExecutable || !validSHA256(guard.VerifierSHA256) {
		return nil, fmt.Errorf("unsupported or invalid legacy runtime guard")
	}
	return &guard, nil
}

func verifyLegacyVerifier(g *LegacyRuntimeGuard) error {
	info, err := os.Lstat(g.VerifierExecutable)
	if err != nil {
		return fmt.Errorf("inspect CE verifier: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("CE verifier must be a regular non-symlink executable")
	}
	data, err := readBoundedRegularFile(g.VerifierExecutable, 128<<20, "CE verifier")
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != g.VerifierSHA256 {
		return fmt.Errorf("CE verifier digest changed")
	}
	return nil
}

type legacyCleanupAttestation struct {
	SchemaVersion     int                `json:"schema_version"`
	Capability        string             `json:"capability"`
	Guard             LegacyRuntimeGuard `json:"guard"`
	PlanDigest        string             `json:"plan_digest"`
	Attempt           string             `json:"attempt"`
	CEPid             int                `json:"ce_pid"`
	CEStartedAt       time.Time          `json:"ce_started_at"`
	ProviderPID       int                `json:"provider_pid"`
	ProviderStartedAt time.Time          `json:"provider_started_at"`
	Allowed           bool               `json:"allowed"`
}

type legacyGuardChecker func(context.Context) error

func newLegacyGuardChecker(p *LegacyCleanupPlan, attempt string) (legacyGuardChecker, error) {
	if p.RuntimeGuard == nil {
		return func(context.Context) error { return nil }, nil
	}
	if !validLowerHex(attempt, 32) {
		return nil, fmt.Errorf("runtime attempt must be nonempty lowercase hexadecimal")
	}
	guard := *p.RuntimeGuard
	return func(ctx context.Context) error {
		if err := verifyLegacyVerifier(&guard); err != nil {
			return err
		}
		return attestLegacyRuntime(ctx, p, &guard, attempt)
	}, nil
}

func attestLegacyRuntime(ctx context.Context, p *LegacyCleanupPlan, guard *LegacyRuntimeGuard, attempt string) error {
	callCtx, cancel := context.WithTimeout(ctx, legacyAttestationTimeout)
	defer cancel()
	args := []string{"task", "legacy-cleanup", "attest", "--operation", guard.OperationID, "--attempt", attempt, "--plan-digest", p.PlanDigest, "--json"}
	cmd := exec.CommandContext(callCtx, guard.VerifierExecutable, args...) // #nosec G204 -- executable and every protocol argument are validated contract fields; no shell is used.
	cmd.Dir = p.RepositoryRoot
	var stdout, stderr boundedLegacyBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	// Check the limit first: an overflowing Write ends the copy with
	// io.ErrShortWrite, so Run fails too, and that failure would hide
	// the cause.
	if stdout.exceeded() || stderr.exceeded() {
		return fmt.Errorf("CE runtime attestation exceeds output limit")
	}
	if err != nil {
		return fmt.Errorf("CE runtime attestation failed: %w", err)
	}
	var result legacyCleanupAttestation
	if err := strictSingleJSON(stdout.Bytes(), &result); err != nil {
		return fmt.Errorf("parse CE runtime attestation: %w", err)
	}
	if result.SchemaVersion != 1 || result.Capability != legacyAttestationCapability || !result.Allowed || result.PlanDigest != p.PlanDigest || result.Attempt != attempt || !reflect.DeepEqual(result.Guard, *guard) {
		return fmt.Errorf("CE runtime attestation binding was refused or changed")
	}
	if result.ProviderPID != os.Getpid() || result.CEPid != os.Getppid() {
		return fmt.Errorf("CE runtime attestation process identity changed")
	}
	if err := verifyLegacyProcessStart(ctx, result.ProviderPID, result.ProviderStartedAt); err != nil {
		return fmt.Errorf("verify provider process identity: %w", err)
	}
	if err := verifyLegacyProcessStart(ctx, result.CEPid, result.CEStartedAt); err != nil {
		return fmt.Errorf("verify CE process identity: %w", err)
	}
	return nil
}

// boundedLegacyBuffer keeps at most legacyAttestationMaxOutput bytes. The
// buffer is a named field, not embedded: an embedded bytes.Buffer promotes
// ReadFrom, which os/exec's copy prefers over Write, and that path would
// skip the limit entirely (ISSUE-77, as ISSUE-70 for limitedBuffer).
type boundedLegacyBuffer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	overflow bool
}

// Write fails on overflow so the copy stops and the verifier's pipe is
// closed; attestLegacyRuntime reports the limit, not the copy error.
func (b *boundedLegacyBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf.Len()+len(data) > legacyAttestationMaxOutput {
		b.overflow = true
		return 0, io.ErrShortWrite
	}
	return b.buf.Write(data)
}

func (b *boundedLegacyBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

func (b *boundedLegacyBuffer) exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.overflow
}

func strictSingleJSON(data []byte, value any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func verifyLegacyProcessStart(ctx context.Context, pid int, reported time.Time) error {
	if pid <= 1 || reported.IsZero() || reported.After(time.Now().Add(legacyProcessStartTolerance)) {
		return fmt.Errorf("invalid process start identity")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "lstart=") // #nosec G204 -- pid is validated integer and fixed ps arguments are used.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("observe process start: %w", err)
	}
	actual, err := time.ParseInLocation(time.ANSIC, strings.TrimSpace(string(out)), time.Local)
	if err != nil {
		return fmt.Errorf("parse process start: %w", err)
	}
	if actual.Sub(reported).Abs() > legacyProcessStartTolerance {
		return fmt.Errorf("process start identity mismatch")
	}
	return nil
}
