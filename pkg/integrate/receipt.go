// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// Durable integration evidence (TASK-302). A run that carries an execution
// context records a receipt under the repository's canonical Git common
// directory before it pushes, so a crash between push and reclaim leaves a
// machine-readable fact instead of folklore. The receipt never grants
// authority by itself: cleanup re-proves the remote target, the residuals,
// and the caller's identity before removing anything.
const (
	evidenceSchemaVersion  = 1
	evidenceDirName        = "gz-integrate"
	evidenceVersionName    = "v1"
	evidenceReceiptsSuffix = "receipts"
	// registrationMarkerName lives in a worktree's own Git directory. It binds
	// one physical worktree to the run that registered it, so a later branch
	// reused at the same commit cannot satisfy an older generation's receipt.
	registrationMarkerName = "gz-integrate-registration.json"

	// Receipt phases advance only forward.
	evidencePhaseIntent   = "intent"
	evidencePhasePushed   = "pushed"
	evidencePhaseComplete = "complete"
)

// ErrEvidenceReplay reports a durable run identity reused with a different
// generation, owner, repository, task, or target. The receipt is immutable
// identity evidence; the replaying run pushed nothing.
var ErrEvidenceReplay = errors.New("execution identity conflicts with the durable receipt")

// ErrReceiptCASConflict reports a receipt write refused because the durable
// document no longer matches what the writer last read: created since,
// removed, advanced to another revision, or rebound to another identity. The
// writer's in-memory receipt is untouched and must be re-read before retry.
var ErrReceiptCASConflict = errors.New("integrate receipt changed since it was read")

// ErrInvalidExecutionContext reports a malformed or partial execution context.
var ErrInvalidExecutionContext = errors.New("invalid execution context")

// OwnerIdentity names who executed a run: a logical actor on a logical host.
// Host is a declared device name (GIT_WORK_HOST / ce.workHost), never a
// guessed hostname; the physical host appears only in the liveness claim.
type OwnerIdentity struct {
	Actor string `json:"actor"`
	Host  string `json:"host"`
	Kind  string `json:"kind"`
}

func (o OwnerIdentity) String() string {
	return o.Actor + "@" + o.Host + "(" + o.Kind + ")"
}

// sameAs compares two owners for receipt CAS. Kind is part of the identity:
// the same actor name driven as agent and as human are different principals.
func (o OwnerIdentity) sameAs(other OwnerIdentity) bool {
	return o.Actor == other.Actor && o.Host == other.Host && o.Kind == other.Kind
}

// ExecutionContext is the immutable identity a caller supplies to run. It is
// all-or-nothing: every field must be present and valid, and --expect-source
// is part of the context, so an approved run names the exact commit it
// approved. Absent context keeps the legacy run behavior.
type ExecutionContext struct {
	ExecutionID  string
	Generation   string
	Owner        OwnerIdentity
	ExpectSource string
}

// validateOpaqueToken accepts an opaque nonempty identifier. Receipts key by
// sha256(execution-id), so the value never becomes a path; the narrow rules
// still keep it printable, single-token, and impossible to mistake for a git
// option.
func validateOpaqueToken(value, name string) error {
	if value == "" {
		return fmt.Errorf("%w: %s is empty", ErrInvalidExecutionContext, name)
	}
	if len(value) > 128 {
		return fmt.Errorf("%w: %s is longer than 128 characters", ErrInvalidExecutionContext, name)
	}
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("%w: %s starts with a dash", ErrInvalidExecutionContext, name)
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) || r == '/' || r == '\\' {
			return fmt.Errorf("%w: %s contains invalid character %q", ErrInvalidExecutionContext, name, r)
		}
	}
	return nil
}

// Validate checks a complete execution context. A partially supplied context
// is malformed: callers must pass every field or none.
func (e *ExecutionContext) Validate() error {
	if e == nil {
		return nil
	}
	if err := validateOpaqueToken(e.ExecutionID, "execution-id"); err != nil {
		return err
	}
	if err := validateOpaqueToken(e.Generation, "generation"); err != nil {
		return err
	}
	if err := validateOpaqueToken(e.Owner.Actor, "owner-actor"); err != nil {
		return err
	}
	if err := validateOpaqueToken(e.Owner.Host, "owner-host"); err != nil {
		return err
	}
	switch e.Owner.Kind {
	case "agent", "human":
	default:
		return fmt.Errorf("%w: owner-kind must be agent or human", ErrInvalidExecutionContext)
	}
	expect, err := NormalizeExpectSource(e.ExpectSource)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidExecutionContext, err)
	}
	e.ExpectSource = expect
	return nil
}

// Caller-owner resolution follows the native work identity chain: environment
// first (GZ_GIT_TASK_ACTOR for an agent, GIT_WORK_ACTOR for a person,
// GIT_WORK_HOST for the device), global Git config second (ce.workActor,
// ce.workHost). There is no hostname fallback — a guessed host makes two
// devices indistinguishable, so an unresolved host is an error.
const (
	callerActorEnvAgent = "GZ_GIT_TASK_ACTOR"
	callerActorEnvHuman = "GIT_WORK_ACTOR"
	callerHostEnv       = "GIT_WORK_HOST"
	callerActorGitKey   = "ce.workActor"
	callerHostGitKey    = "ce.workHost"
)

// ResolveCallerOwner resolves the identity of the process asking for cleanup.
// It never guesses: an unconfigured actor or host is an error, because owner
// comparison is the cleanup authorization.
func ResolveCallerOwner(ctx context.Context, exec *gitcmd.Executor) (OwnerIdentity, error) {
	if exec == nil {
		return OwnerIdentity{}, fmt.Errorf("git executor is nil")
	}
	actor, kind := strings.TrimSpace(os.Getenv(callerActorEnvAgent)), "agent"
	if actor == "" {
		actor, kind = strings.TrimSpace(os.Getenv(callerActorEnvHuman)), "human"
	}
	host := strings.TrimSpace(os.Getenv(callerHostEnv))
	gitValue := func(key string) (string, error) {
		out, err := exec.RunOutput(ctx, ".", "config", "--global", "--includes", "--get", key)
		if err != nil {
			return "", fmt.Errorf("read global Git %s: %w", key, err)
		}
		return strings.TrimSpace(out), nil
	}
	if actor == "" {
		configured, err := gitValue(callerActorGitKey)
		if err != nil {
			return OwnerIdentity{}, err
		}
		actor, kind = configured, "human"
	}
	if host == "" {
		configured, err := gitValue(callerHostGitKey)
		if err != nil {
			return OwnerIdentity{}, err
		}
		host = configured
	}
	owner := OwnerIdentity{Actor: actor, Host: host, Kind: kind}
	if err := validateOpaqueToken(owner.Actor, "caller actor"); err != nil {
		return OwnerIdentity{}, fmt.Errorf("caller identity is not configured: set %s for an agent, %s for a person, or global Git %s", callerActorEnvAgent, callerActorEnvHuman, callerActorGitKey)
	}
	if err := validateOpaqueToken(owner.Host, "caller host"); err != nil {
		return OwnerIdentity{}, fmt.Errorf("device identity is not configured: set %s or global Git %s", callerHostEnv, callerHostGitKey)
	}
	return owner, nil
}

// evidenceClaim is liveness diagnostics for the process that last mutated a
// receipt. It never authorizes anything; cleanup uses it only to refuse when
// the last holder is positively alive or unknowable.
type evidenceClaim struct {
	PID         int       `json:"pid"`
	ProcessHost string    `json:"process_host"`
	StartedAt   time.Time `json:"started_at"`
}

type evidenceControllerProvenance struct {
	Path           string   `json:"path"`
	Digest         string   `json:"digest"`
	Workspace      string   `json:"workspace"`
	Remote         string   `json:"remote"`
	RemoteURL      string   `json:"remote_url"`
	Integration    []string `json:"integration"`
	TaskPattern    []string `json:"task_pattern"`
	PrepareProfile string   `json:"prepare_profile"`
}

// evidenceReclaimRefs records the resolved Git branch names used by normal
// reclaim. Absence in a legacy receipt is unknown, never permission to delete.
type evidenceReclaimRefs struct {
	Default     string `json:"default"`
	Integration string `json:"integration"`
}

type evidenceProvenance struct {
	DeclarationPatterns []string                      `json:"declaration_patterns"`
	DeclarationFacts    []string                      `json:"declaration_facts,omitempty"`
	Controller          *evidenceControllerProvenance `json:"controller,omitempty"`
	ReclaimRefs         *evidenceReclaimRefs          `json:"reclaim_refs,omitempty"`
}

// Receipt is the durable integration evidence document. Field names are the
// published TASK-302 JSON contract; phase/revision/claim are diagnostics and
// never a substitute for re-proving facts.
type Receipt struct {
	SchemaVersion       int                 `json:"schema_version"`
	ExecutionID         string              `json:"execution_id"`
	Generation          string              `json:"generation"`
	Owner               OwnerIdentity       `json:"owner"`
	RepositoryCommonDir string              `json:"repository_common_dir"`
	TaskBranch          string              `json:"task_branch"`
	TaskWorktree        string              `json:"task_worktree"`
	TaskSHA             string              `json:"task_sha"`
	TargetRef           string              `json:"target_ref"`
	TargetBeforeSHA     string              `json:"target_before_sha"`
	Remote              string              `json:"remote"`
	PushRemote          string              `json:"push_remote"`
	SourcePushed        bool                `json:"source_pushed"`
	WorktreeRemoved     bool                `json:"worktree_removed"`
	LocalBranchRemoved  bool                `json:"local_branch_removed"`
	RemoteBranchRemoved bool                `json:"remote_branch_removed"`
	Phase               string              `json:"phase"`
	Revision            int                 `json:"revision"`
	Claim               evidenceClaim       `json:"claim"`
	Provenance          *evidenceProvenance `json:"provenance,omitempty"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

func evidenceRoot(common string) string {
	return filepath.Join(common, evidenceDirName, evidenceVersionName)
}

func evidenceReceiptPath(common, executionID string) string {
	digest := sha256.Sum256([]byte(executionID))
	return filepath.Join(evidenceRoot(common), evidenceReceiptsSuffix, fmt.Sprintf("%x.json", digest))
}

// readReceipt loads a receipt without creating anything. A missing receipt is
// ok=false, not an error; an unparseable one is an error, because guessing
// around damaged evidence would fabricate authority.
func readReceipt(common, executionID string) (*Receipt, bool, error) {
	data, err := os.ReadFile(evidenceReceiptPath(common, executionID)) // #nosec G304 -- fixed digest path below the Git common directory
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read integrate receipt: %w", err)
	}
	var r Receipt
	if err := jsonUnmarshalStrict(data, &r); err != nil {
		return nil, false, fmt.Errorf("parse integrate receipt %s: %w", evidenceReceiptPath(common, executionID), err)
	}
	if r.SchemaVersion != evidenceSchemaVersion {
		return nil, false, fmt.Errorf("integrate receipt %s has unsupported schema_version %d", evidenceReceiptPath(common, executionID), r.SchemaVersion)
	}
	return &r, true, nil
}

// writeReceipt persists a receipt durably (temp + fsync + rename + dir fsync)
// as an optimistic CAS. The caller must already hold the target lock for the
// destination the receipt names; this then takes the per-execution write
// lock — the order target-lock → execution-write-lock is the only one, so it
// cannot deadlock — and refuses when the durable document is not exactly what
// the caller last read: same revision, same immutable identity. Two runs
// sharing an execution-id but pushing different targets hold different target
// locks, so that pair is serialized here instead. Nothing but file reads and
// the atomic write happens under the execution lock: no git, no network. A
// refused CAS leaves the in-memory receipt byte-identical, revision and claim
// included. Every write refreshes the liveness claim to the writer.
func writeReceipt(ctx context.Context, common string, r *Receipt) error {
	if r == nil {
		return fmt.Errorf("receipt is nil")
	}
	lock, err := acquireReceiptWriteLock(ctx, common, r.ExecutionID)
	if err != nil {
		return err
	}
	defer lock.release()
	return writeReceiptLocked(common, r, r)
}

// createReceipt writes a receipt that must not exist yet. The create-versus-
// existing check is the collision guard for a concurrent same-execution-id
// begin: if another run recorded this identity first, the loser fails with
// ErrEvidenceReplay (different identity) or ErrReceiptCASConflict (same
// identity) before it pushes anything.
func createReceipt(ctx context.Context, common string, r *Receipt) error {
	if r == nil {
		return fmt.Errorf("receipt is nil")
	}
	lock, err := acquireReceiptWriteLock(ctx, common, r.ExecutionID)
	if err != nil {
		return err
	}
	defer lock.release()
	return writeReceiptLocked(common, r, nil)
}

// receiptWriteLock serializes receipt mutation for one execution id. The
// target lock keys by destination ref, so two runs sharing an execution id
// but pushing different targets are not serialized by it; this lock is.
type receiptWriteLock struct{ file *os.File }

func (l *receiptWriteLock) release() {
	if l != nil && l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

func evidenceReceiptLockPath(common, executionID string) string {
	return evidenceReceiptPath(common, executionID) + ".lock"
}

// acquireReceiptWriteLock blocks until this process owns the per-execution
// receipt lock. The lock belongs to the open file description, so the kernel
// drops it if the holder dies. Callers already hold the target lock; nothing
// may take these two locks in the opposite order.
func acquireReceiptWriteLock(ctx context.Context, common, executionID string) (*receiptWriteLock, error) {
	receiptPath := evidenceReceiptPath(common, executionID)
	dir := filepath.Dir(receiptPath)
	if err := os.MkdirAll(dir, 0o700); err != nil { // #nosec G301 -- receipt state is per-execution private evidence
		return nil, fmt.Errorf("create integrate receipt directory: %w", err)
	}
	f, err := os.OpenFile(evidenceReceiptLockPath(common, executionID), os.O_RDWR|os.O_CREATE, 0o600) // #nosec G304 -- fixed digest below the Git common directory
	if err != nil {
		return nil, fmt.Errorf("open integrate receipt write lock: %w", err)
	}
	for {
		locked, err := tryLockFile(f)
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("lock integrate receipt %s: %w", executionID, err)
		}
		if locked {
			return &receiptWriteLock{file: f}, nil
		}
		timer := time.NewTimer(hostLockPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = f.Close()
			return nil, fmt.Errorf("wait for integrate receipt write lock %s: %w", executionID, ctx.Err())
		case <-timer.C:
		}
	}
}

// writeReceiptLocked performs the CAS-checked durable write while the
// per-execution write lock is held. expect is the document the writer last
// read; nil means the writer believes no receipt exists yet. Revision and
// claim are mutated only after every check has passed, so a refused CAS
// leaves r exactly as the caller supplied it.
func writeReceiptLocked(common string, r, expect *Receipt) error {
	onDisk, ok, err := readReceipt(common, r.ExecutionID)
	if err != nil {
		return err
	}
	switch {
	case expect == nil && ok:
		if conflicts := onDisk.identityConflicts(r); len(conflicts) > 0 {
			return fmt.Errorf("%w: %w: execution-id %s recorded %s as %s; refusing to create as %s", ErrReceiptCASConflict, ErrEvidenceReplay, r.ExecutionID, strings.Join(conflicts, ", "), describeReceiptIdentity(onDisk), describeReceiptIdentity(r))
		}
		return fmt.Errorf("%w: execution-id %s was recorded concurrently", ErrReceiptCASConflict, r.ExecutionID)
	case expect != nil && !ok:
		return fmt.Errorf("%w: execution-id %s disappeared", ErrReceiptCASConflict, r.ExecutionID)
	case expect != nil && ok:
		if onDisk.Revision != expect.Revision {
			return fmt.Errorf("%w: execution-id %s advanced to revision %d, expected %d", ErrReceiptCASConflict, r.ExecutionID, onDisk.Revision, expect.Revision)
		}
		if conflicts := onDisk.identityConflicts(r); len(conflicts) > 0 {
			return fmt.Errorf("%w: %w: execution-id %s was rebound (%s) from %s to %s", ErrReceiptCASConflict, ErrEvidenceReplay, r.ExecutionID, strings.Join(conflicts, ", "), describeReceiptIdentity(onDisk), describeReceiptIdentity(r))
		}
	}
	revision, claim := r.Revision, r.Claim
	refreshClaim(r)
	r.Revision++
	if err := atomicStateWrite(evidenceReceiptPath(common, r.ExecutionID), mustMarshalJSON(r)); err != nil {
		r.Revision, r.Claim = revision, claim
		return fmt.Errorf("write integrate receipt: %w", err)
	}
	return nil
}

func refreshClaim(r *Receipt) {
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	r.Claim = evidenceClaim{PID: os.Getpid(), ProcessHost: host, StartedAt: time.Now().UTC()}
}

// physicalHost returns the host the process runs on, for liveness only.
func physicalHost() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return host
}

// identityConflicts lists the immutable fields on which a durable run
// identity may not be replayed with different values.
func (r *Receipt) identityConflicts(candidate *Receipt) []string {
	var conflicts []string
	if r.Generation != candidate.Generation {
		conflicts = append(conflicts, "generation")
	}
	if !r.Owner.sameAs(candidate.Owner) {
		conflicts = append(conflicts, "owner")
	}
	if r.RepositoryCommonDir != candidate.RepositoryCommonDir {
		conflicts = append(conflicts, "repository_common_dir")
	}
	if r.TaskSHA != candidate.TaskSHA {
		conflicts = append(conflicts, "task_sha")
	}
	if r.TargetRef != candidate.TargetRef {
		conflicts = append(conflicts, "target_ref")
	}
	if r.TaskBranch != candidate.TaskBranch {
		conflicts = append(conflicts, "task_branch")
	}
	if r.TaskWorktree != candidate.TaskWorktree {
		conflicts = append(conflicts, "task_worktree")
	}
	// Patterns, controller identity and resolved branch protections are captured
	// facts for this run. A waiter or intent replay must not replace them.
	if !reflect.DeepEqual(r.Provenance, candidate.Provenance) {
		conflicts = append(conflicts, "provenance")
	}
	return conflicts
}

// registrationMarker binds a physical worktree to the run that registered it.
type registrationMarker struct {
	SchemaVersion int    `json:"schema_version"`
	ExecutionID   string `json:"execution_id"`
	Generation    string `json:"generation"`
	TaskSHA       string `json:"task_sha"`
}

func writeRegistrationMarker(ctx context.Context, exec *gitcmd.Executor, worktree string, m registrationMarker) error {
	gitDir, err := newGitRepo(exec, worktree).output(ctx, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return fmt.Errorf("resolve worktree git dir for registration: %w", err)
	}
	if err := validateOpaqueToken(m.ExecutionID, "execution-id"); err != nil {
		return err
	}
	if err := atomicStateWrite(filepath.Join(gitDir, registrationMarkerName), mustMarshalJSON(m)); err != nil {
		return fmt.Errorf("write worktree registration marker: %w", err)
	}
	return nil
}

// readRegistrationMarker returns the worktree's registration, if any.
func readRegistrationMarker(ctx context.Context, exec *gitcmd.Executor, worktree string) (registrationMarker, bool, error) {
	var m registrationMarker
	gitDir, err := newGitRepo(exec, worktree).output(ctx, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return m, false, fmt.Errorf("resolve worktree git dir for registration: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(gitDir, registrationMarkerName)) // #nosec G304 -- fixed marker name inside the worktree's Git directory
	if err != nil {
		if os.IsNotExist(err) {
			return m, false, nil
		}
		return m, false, fmt.Errorf("read worktree registration marker: %w", err)
	}
	if err := jsonUnmarshalStrict(data, &m); err != nil {
		return m, false, fmt.Errorf("parse worktree registration marker: %w", err)
	}
	return m, true, nil
}

// jsonUnmarshalStrict decodes with DisallowUnknownFields so a damaged or
// foreign document fails loudly instead of half-loading.
func jsonUnmarshalStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func mustMarshalJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		// Receipt types contain only strings, ints, bools, times, and slices
		// thereof; marshaling cannot fail.
		panic(fmt.Sprintf("integrate receipt marshal: %v", err))
	}
	return append(data, '\n')
}
