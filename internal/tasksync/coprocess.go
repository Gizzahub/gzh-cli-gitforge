// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	coprocessMaxLine = 1 << 20
	// DefaultCoprocessResponseTimeout bounds each wait on the owner.
	DefaultCoprocessResponseTimeout = 60 * time.Second
)

// ErrCoprocessBegin means no valid begin arrived, so nothing was invoked.
var ErrCoprocessBegin = errors.New("task sync begin was not accepted")

// ErrCoprocessResult means the single final result could not be delivered.
var ErrCoprocessResult = errors.New("task sync result was not delivered")

var errLeaseLost = errors.New("task sync lease is lost")

type coprocessLine struct {
	raw []byte
	err error
}

// coprocessSession is both fence provider and lease for one begin. ApplyLocal
// calls it sequentially. A loss is sticky: no later call performs I/O.
type coprocessSession struct {
	lines    <-chan coprocessLine
	out      io.Writer
	timeout  time.Duration
	plan     ApplyPlan
	step     string
	sequence uint64
	lost     bool
	closed   bool
}

type applyFunc func(context.Context, ApplyRequest) ApplyResult

// RunCoprocess serves one task-sync-apply-v1 exchange over the given streams.
func RunCoprocess(ctx context.Context, in io.Reader, out io.Writer, timeout time.Duration) error {
	return runCoprocess(ctx, in, out, timeout, ApplyLocal)
}

func runCoprocess(ctx context.Context, in io.Reader, out io.Writer, timeout time.Duration, apply applyFunc) error {
	if timeout <= 0 {
		timeout = DefaultCoprocessResponseTimeout
	}
	done := make(chan struct{})
	defer close(done)
	session := &coprocessSession{lines: readCoprocessLines(in, done), out: out, timeout: timeout}
	begin, err := session.receive(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCoprocessBegin, err)
	}
	if begin.Type != coprocessBeginType {
		return fmt.Errorf("%w: first message is not begin", ErrCoprocessBegin)
	}
	request := begin.Begin.request(session)
	session.plan = request.Plan
	outcome := apply(ctx, request)
	return session.finish(ctx, outcome)
}

// readCoprocessLines stops at the first read failure; stdin EOF is a failure.
func readCoprocessLines(in io.Reader, done <-chan struct{}) <-chan coprocessLine {
	lines := make(chan coprocessLine)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 0, 64*1024), coprocessMaxLine)
		for scanner.Scan() {
			select {
			case lines <- coprocessLine{raw: append([]byte{}, scanner.Bytes()...)}:
			case <-done:
				return
			}
		}
		err := scanner.Err()
		if err == nil {
			err = io.EOF
		}
		select {
		case lines <- coprocessLine{err: err}:
		case <-done:
		}
	}()
	return lines
}

func (s *coprocessSession) receive(ctx context.Context) (coprocessMessage, error) {
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case line, open := <-s.lines:
		if !open {
			return coprocessMessage{}, io.EOF
		}
		if line.err != nil {
			return coprocessMessage{}, line.err
		}
		return decodeCoprocessInbound(line.raw)
	case <-timer.C:
		return coprocessMessage{}, errors.New("owner response timed out")
	case <-ctx.Done():
		return coprocessMessage{}, ctx.Err()
	}
}

// send gives up after the timeout. The abandoned writer can only finish a line
// nobody follows, because every path out of a failed send stops further sends.
func (s *coprocessSession) send(ctx context.Context, message coprocessMessage) error {
	raw, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode coprocess message: %w", err)
	}
	written := make(chan error, 1)
	go func() {
		_, writeErr := s.out.Write(append(raw, '\n'))
		written <- writeErr
	}()
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case err := <-written:
		return err
	case <-timer.C:
		return errors.New("owner stopped reading")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *coprocessSession) envelope(kind string) coprocessMessage {
	s.sequence++
	return coprocessMessage{Schema: CoprocessSchema, Type: kind, PlanID: s.plan.PlanID, Step: s.step, Sequence: s.sequence}
}

func (s *coprocessSession) exchange(ctx context.Context, request coprocessMessage) error {
	if s.lost {
		return errLeaseLost
	}
	if err := s.exchangeOnce(ctx, request); err != nil {
		s.lost = true
		return fmt.Errorf("%w: %w", errLeaseLost, err)
	}
	return nil
}

func (s *coprocessSession) exchangeOnce(ctx context.Context, request coprocessMessage) error {
	if err := s.send(ctx, request); err != nil {
		return err
	}
	response, err := s.receive(ctx)
	if err != nil {
		return err
	}
	if response.PlanID != request.PlanID || response.Step != request.Step || response.Sequence != request.Sequence {
		return errors.New("owner response does not match the request")
	}
	switch response.Type {
	case coprocessAckType:
		return nil
	case coprocessDenyType:
		return errors.New("owner denied the request")
	default:
		return errors.New("owner sent begin during an apply")
	}
}

// Acquire implements ApplyFenceProvider; the session is its own single lease.
func (s *coprocessSession) Acquire(ctx context.Context, _ ApplyPlan) (ApplyLease, error) {
	if err := s.exchange(ctx, s.envelope(coprocessAcquireType)); err != nil {
		return nil, err
	}
	return s, nil
}

// Check implements ApplyLease.
func (s *coprocessSession) Check(ctx context.Context) error {
	if s.closed {
		return errLeaseLost
	}
	return s.exchange(ctx, s.envelope(coprocessCheckType))
}

// Record implements ApplyLease with the provider-produced evidence digest.
func (s *coprocessSession) Record(ctx context.Context, record ApplyRecord) error {
	if s.closed || !knownApplyStep(record.Step) {
		s.lost = true
		return errLeaseLost
	}
	digest, err := recordEvidenceDigest(s.plan.ProviderBindingDigest, record)
	if err != nil {
		s.lost = true
		return err
	}
	s.step = record.Step
	message := s.envelope(coprocessRecordType)
	message.Record = &coprocessRecord{CheckpointRef: record.CheckpointRef, CandidateOID: record.CandidateOID, OriginalHEAD: record.OriginalHEAD, ActualHead: record.ActualHEAD, EvidenceDigest: digest}
	return s.exchange(ctx, message)
}

// Close implements ApplyLease. Only an acknowledged close counts as closed.
func (s *coprocessSession) Close() error {
	if s.closed {
		return errLeaseLost
	}
	s.closed = true
	// Close has no context in ApplyLease; the send and receive timeouts bound it.
	return s.exchange(context.Background(), s.envelope(coprocessCloseType))
}

func (s *coprocessSession) finish(ctx context.Context, outcome ApplyResult) error {
	result := CoprocessResult{Outcome: outcome.Outcome, Applied: outcome.Applied, Reason: outcome.Reason, CheckpointRef: outcome.CheckpointRef, OriginalHead: outcome.OriginalHEAD, ActualHead: outcome.ActualHEAD}
	if outcome.Applied {
		result.SourceHead = s.plan.ExpectedSourceOID
	}
	digest, err := resultEvidenceDigest(s.plan.ProviderBindingDigest, s.plan, result)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCoprocessResult, err)
	}
	result.EvidenceDigest = digest
	message := s.envelope(coprocessResultType)
	message.Result = &result
	// A loss must not silence the result: the owner may still be reading. Use a
	// fresh context so a canceled apply can still report what it observed.
	if err := s.send(context.WithoutCancel(ctx), message); err != nil {
		return fmt.Errorf("%w: %w", ErrCoprocessResult, err)
	}
	return nil
}
