// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// CoprocessSchema is the only wire schema task-sync-apply-v1 accepts.
const CoprocessSchema = 1

const (
	coprocessBeginType   = "begin"
	coprocessAckType     = "ack"
	coprocessDenyType    = "deny"
	coprocessAcquireType = "acquire"
	coprocessCheckType   = "check"
	coprocessRecordType  = "record"
	coprocessCloseType   = "close"
	coprocessResultType  = "result"
)

// applySteps is the exact order of CE syncStages; see the package documentation.
var applySteps = []string{
	"checkpoint-intent", "checkpoint-created", "checkout-intent", "checked-out",
	"branch-intent", "branch-updated", "attach-intent", "attached",
}

// coprocessMessage is the single envelope of both directions.
type coprocessMessage struct {
	Schema   int              `json:"schema"`
	Type     string           `json:"type"`
	PlanID   string           `json:"plan_id"`
	Step     string           `json:"step"`
	Sequence uint64           `json:"sequence"`
	Begin    *coprocessBegin  `json:"begin,omitempty"`
	Record   *coprocessRecord `json:"record,omitempty"`
	Result   *CoprocessResult `json:"result,omitempty"`
	Reason   string           `json:"reason,omitempty"`
}

type coprocessBegin struct {
	Plan      coprocessPlan      `json:"plan"`
	Candidate coprocessCandidate `json:"candidate"`
}

type coprocessPlan struct {
	PlanID                string   `json:"plan_id"`
	GenerationDigest      string   `json:"generation_digest"`
	AuthorityDigest       string   `json:"authority_digest"`
	ProviderBindingDigest string   `json:"provider_binding_digest"`
	CanonicalWorktree     string   `json:"canonical_worktree"`
	TaskWorktree          string   `json:"task_worktree"`
	CommonDir             string   `json:"common_dir"`
	TaskBranch            string   `json:"task_branch"`
	SourceRef             string   `json:"source_ref"`
	ExpectedTaskHEAD      string   `json:"expected_task_head"`
	ExpectedSourceOID     string   `json:"expected_source_oid"`
	AllowedPaths          []string `json:"allowed_paths"`
	ProtectedPaths        []string `json:"protected_paths"`
}

type coprocessCandidate struct {
	Handle       string `json:"handle"`
	CandidateOID string `json:"candidate_oid"`
	SourceOID    string `json:"source_oid"`
	OriginalHEAD string `json:"original_head"`
	CommonDir    string `json:"common_dir"`
}

type coprocessRecord struct {
	CheckpointRef  string `json:"checkpoint_ref"`
	CandidateOID   string `json:"candidate_oid"`
	OriginalHEAD   string `json:"original_head"`
	ActualHead     string `json:"actual_head"`
	EvidenceDigest string `json:"evidence_digest"`
}

// CoprocessResult is the final wire result. SourceHead is set only when the
// pinned source was verified, that is on an applied outcome.
type CoprocessResult struct {
	Outcome        Outcome `json:"outcome"`
	Applied        bool    `json:"applied"`
	Reason         string  `json:"reason"`
	CheckpointRef  string  `json:"checkpoint_ref"`
	OriginalHead   string  `json:"original_head"`
	ActualHead     string  `json:"actual_head"`
	SourceHead     string  `json:"source_head"`
	EvidenceDigest string  `json:"evidence_digest"`
}

// decodeCoprocessInbound accepts exactly one complete CE-to-provider message.
func decodeCoprocessInbound(line []byte) (coprocessMessage, error) {
	var message coprocessMessage
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&message); err != nil {
		return coprocessMessage{}, errors.New("coprocess message is not valid")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return coprocessMessage{}, errors.New("coprocess message has trailing data")
	}
	if message.Schema != CoprocessSchema || message.PlanID == "" {
		return coprocessMessage{}, errors.New("coprocess message schema or plan is unsupported")
	}
	if message.Record != nil || message.Result != nil {
		return coprocessMessage{}, errors.New("coprocess message carries a provider payload")
	}
	switch message.Type {
	case coprocessBeginType:
		if message.Begin == nil || message.Begin.Plan.PlanID != message.PlanID || message.Sequence != 0 || message.Step != "" {
			return coprocessMessage{}, errors.New("coprocess begin is incomplete")
		}
	case coprocessAckType, coprocessDenyType:
		if message.Begin != nil {
			return coprocessMessage{}, errors.New("coprocess response carries a begin payload")
		}
	default:
		return coprocessMessage{}, errors.New("coprocess message type is unsupported")
	}
	return message, nil
}

func (begin coprocessBegin) request(fence ApplyFenceProvider) ApplyRequest {
	return ApplyRequest{Plan: ApplyPlan(begin.Plan), Candidate: CandidateDescriptor(begin.Candidate), Fence: fence}
}

func knownApplyStep(step string) bool {
	for _, known := range applySteps {
		if known == step {
			return true
		}
	}
	return false
}

func evidenceDigest(kind string, value any) (string, error) {
	raw, err := json.Marshal(struct {
		Kind  string `json:"kind"`
		Value any    `json:"value"`
	}{kind, value})
	if err != nil {
		return "", fmt.Errorf("encode evidence: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func recordEvidenceDigest(binding string, record ApplyRecord) (string, error) {
	return evidenceDigest("task-sync-apply-v1/record", struct {
		Binding string
		Record  ApplyRecord
	}{binding, record})
}

func resultEvidenceDigest(binding string, plan ApplyPlan, result CoprocessResult) (string, error) {
	result.EvidenceDigest = ""
	return evidenceDigest("task-sync-apply-v1/result", struct {
		Binding, PlanID string
		Result          CoprocessResult
	}{binding, plan.PlanID, result})
}
