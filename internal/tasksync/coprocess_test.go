// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fakeOwner plays CE over pipes. respond decides each reply; returning false
// closes stdin, which is what a killed owner looks like to the provider.
type fakeOwner struct {
	t        *testing.T
	in       *io.PipeWriter
	seen     []coprocessMessage
	result   *CoprocessResult
	respond  func(coprocessMessage) (string, bool)
	finished chan error
}

func ack(m coprocessMessage) string {
	raw, _ := json.Marshal(coprocessMessage{Schema: CoprocessSchema, Type: coprocessAckType, PlanID: m.PlanID, Step: m.Step, Sequence: m.Sequence})
	return string(raw)
}

func deny(m coprocessMessage) string {
	raw, _ := json.Marshal(coprocessMessage{Schema: CoprocessSchema, Type: coprocessDenyType, PlanID: m.PlanID, Step: m.Step, Sequence: m.Sequence, Reason: "denied"})
	return string(raw)
}

func wireBegin(request ApplyRequest) string {
	begin := coprocessBegin{Plan: coprocessPlan(request.Plan), Candidate: coprocessCandidate(request.Candidate)}
	raw, _ := json.Marshal(coprocessMessage{Schema: CoprocessSchema, Type: coprocessBeginType, PlanID: request.Plan.PlanID, Begin: &begin})
	return string(raw)
}

func driveOwner(t *testing.T, request ApplyRequest, respond func(coprocessMessage) (string, bool)) *fakeOwner {
	t.Helper()
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	owner := &fakeOwner{t: t, in: stdinW, respond: respond, finished: make(chan error, 1)}
	go func() {
		owner.finished <- runCoprocess(context.Background(), stdinR, stdoutW, 5*time.Second, ApplyLocal)
		_ = stdoutW.Close()
	}()
	if _, err := io.WriteString(stdinW, wireBegin(request)+"\n"); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdoutR)
	scanner.Buffer(nil, coprocessMaxLine)
	alive := true
	for scanner.Scan() {
		var message coprocessMessage
		decoder := json.NewDecoder(strings.NewReader(scanner.Text()))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&message); err != nil {
			t.Fatal(err)
		}
		owner.seen = append(owner.seen, message)
		if message.Type == coprocessResultType {
			owner.result = message.Result
			continue
		}
		if !alive {
			t.Fatalf("provider called %s after the owner was gone", message.Type)
		}
		line, ok := respond(message)
		if !ok {
			alive = false
			_ = stdinW.Close()
			continue
		}
		if _, err := io.WriteString(stdinW, line+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	_ = stdinW.Close()
	if err := <-owner.finished; err != nil {
		t.Fatalf("runCoprocess: %v", err)
	}
	if owner.result == nil {
		t.Fatal("no result")
	}
	return owner
}

func (o *fakeOwner) count(kind string) int {
	total := 0
	for _, message := range o.seen {
		if message.Type == kind {
			total++
		}
	}
	return total
}

func preparedApply(t *testing.T) (fixture, ApplyRequest) {
	t.Helper()
	f := newFixture(t, false)
	prepared := PrepareClean(context.Background(), f.request()).Prepared
	if prepared == nil {
		t.Fatal("prepare failed")
	}
	return f, boundApplyRequest(f, prepared.Descriptor())
}

func TestCoprocessCompleteApplySpeaksCEStagesInOrder(t *testing.T) {
	f, request := preparedApply(t)
	owner := driveOwner(t, request, func(m coprocessMessage) (string, bool) { return ack(m), true })
	if owner.result.Outcome != OutcomeApplied || !owner.result.Applied || owner.result.SourceHead != f.source {
		t.Fatalf("result=%+v", owner.result)
	}
	var steps []string
	headAt := map[string]string{}
	for index, message := range owner.seen {
		if message.Sequence != uint64(index+1) {
			t.Fatalf("message %d has sequence %d", index, message.Sequence)
		}
		if message.Type != coprocessRecordType {
			continue
		}
		steps = append(steps, message.Step)
		headAt[message.Step] = message.Record.ActualHead
		if len(message.Record.EvidenceDigest) != 64 {
			t.Fatalf("digest %q", message.Record.EvidenceDigest)
		}
	}
	// This is CE's syncStages; a change on either side must fail here first.
	want := []string{"checkpoint-intent", "checkpoint-created", "checkout-intent", "checked-out", "branch-intent", "branch-updated", "attach-intent", "attached"}
	if !reflect.DeepEqual(steps, want) || !reflect.DeepEqual(applySteps, want) {
		t.Fatalf("steps=%v", steps)
	}
	candidate := request.Candidate.CandidateOID
	for step, head := range map[string]string{"checkout-intent": f.taskHead, "checked-out": candidate, "attached": candidate} {
		if headAt[step] != head {
			t.Fatalf("%s observed %s, want %s", step, headAt[step], head)
		}
	}
	last := owner.seen[len(owner.seen)-2]
	if last.Type != coprocessCloseType || len(owner.result.EvidenceDigest) != 64 {
		t.Fatalf("last=%+v result=%+v", last, owner.result)
	}
}

func assertInterrupted(t *testing.T, f fixture, owner *fakeOwner, wantHead string) {
	t.Helper()
	got := owner.result
	if got.Outcome != OutcomeUncertain || got.Applied || got.SourceHead != "" {
		t.Fatalf("result=%+v", got)
	}
	if got.CheckpointRef == "" || gitOutput(t, f.canonical, "rev-parse", got.CheckpointRef) != f.taskHead {
		t.Fatalf("checkpoint was not preserved: %+v", got)
	}
	if head := gitOutput(t, f.task, "rev-parse", "HEAD"); head != wantHead {
		t.Fatalf("HEAD=%s want %s", head, wantHead)
	}
	if owner.count(coprocessAcquireType) != 1 {
		t.Fatal("the apply was re-executed")
	}
}

func TestCoprocessLeaseLossStopsMutationAndKeepsCheckpoint(t *testing.T) {
	garbage := func(coprocessMessage) string { return `{"schema":1,"type":"ack","plan_id":"p","extra":1}` }
	tests := []struct {
		name    string
		trigger func(coprocessMessage) bool
		reply   func(coprocessMessage) (string, bool)
	}{
		{"check deny after grant expiry", checkAfter("checkpoint-created"), func(m coprocessMessage) (string, bool) { return deny(m), true }},
		{"record store failure", recordOf("checkout-intent"), func(m coprocessMessage) (string, bool) { return deny(m), true }},
		{"stdin EOF", checkAfter("checkpoint-created"), func(coprocessMessage) (string, bool) { return "", false }},
		{"decode failure", checkAfter("checkpoint-created"), func(m coprocessMessage) (string, bool) { return garbage(m), true }},
		{"wrong sequence", checkAfter("checkpoint-created"), func(m coprocessMessage) (string, bool) {
			m.Sequence++
			return ack(m), true
		}},
		{"second begin", checkAfter("checkpoint-created"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, request := preparedApply(t)
			fired := 0
			owner := driveOwner(t, request, func(m coprocessMessage) (string, bool) {
				if fired == 0 && tt.trigger(m) {
					fired++
					if tt.reply == nil {
						return wireBegin(request), true
					}
					return tt.reply(m)
				}
				return ack(m), true
			})
			if fired != 1 {
				t.Fatal("trigger never fired")
			}
			assertInterrupted(t, f, owner, f.taskHead)
			after := owner.seen[len(owner.seen)-1]
			if after.Type != coprocessResultType {
				t.Fatalf("last message %s", after.Type)
			}
			if owner.count(coprocessCloseType) != 0 {
				t.Fatal("lost lease sent close")
			}
		})
	}
}

func checkAfter(step string) func(coprocessMessage) bool {
	return func(m coprocessMessage) bool { return m.Type == coprocessCheckType && m.Step == step }
}

func recordOf(step string) func(coprocessMessage) bool {
	return func(m coprocessMessage) bool { return m.Type == coprocessRecordType && m.Step == step }
}

func TestCoprocessExternalHeadMoveIsRefusedByBoundary(t *testing.T) {
	f, request := preparedApply(t)
	moved := false
	var outside string
	owner := driveOwner(t, request, func(m coprocessMessage) (string, bool) {
		if !moved && checkAfter("checkout-intent")(m) {
			moved = true
			runGit(t, f.task, "commit", "--allow-empty", "-m", "external writer")
			outside = gitOutput(t, f.task, "rev-parse", "HEAD")
		}
		return ack(m), true
	})
	assertInterrupted(t, f, owner, outside)
}

func TestCoprocessAcquireDenialIsRefusedBeforeAnyChange(t *testing.T) {
	f, request := preparedApply(t)
	owner := driveOwner(t, request, func(m coprocessMessage) (string, bool) { return deny(m), true })
	if owner.result.Outcome != OutcomeRefused || owner.result.CheckpointRef != "" || len(owner.seen) != 2 {
		t.Fatalf("result=%+v seen=%d", owner.result, len(owner.seen))
	}
	if gitOutput(t, f.task, "rev-parse", "HEAD") != f.taskHead {
		t.Fatal("task moved")
	}
}

func TestCoprocessResponseTimeoutIsLeaseLoss(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinW.Close()
	stdoutR, stdoutW := io.Pipe()
	go func() { _, _ = io.Copy(io.Discard, stdoutR) }()
	calls := 0
	apply := func(ctx context.Context, request ApplyRequest) ApplyResult {
		calls++
		_, err := request.Fence.Acquire(ctx, request.Plan)
		if err == nil || !errors.Is(err, errLeaseLost) {
			t.Errorf("Acquire err=%v", err)
		}
		return applyRefused("apply fence was denied")
	}
	finished := make(chan error, 1)
	go func() {
		finished <- runCoprocess(context.Background(), stdinR, stdoutW, 50*time.Millisecond, apply)
	}()
	if _, err := io.WriteString(stdinW, `{"schema":1,"type":"begin","plan_id":"p","step":"","sequence":0,"begin":{"plan":{"plan_id":"p"},"candidate":{}}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestCoprocessResultLossIsReported(t *testing.T) {
	in := strings.NewReader(`{"schema":1,"type":"begin","plan_id":"p","step":"","sequence":0,"begin":{"plan":{"plan_id":"p"},"candidate":{}}}` + "\n")
	calls := 0
	apply := func(context.Context, ApplyRequest) ApplyResult {
		calls++
		return ApplyResult{Outcome: OutcomeApplied, Applied: true}
	}
	err := runCoprocess(context.Background(), in, failingWriter{}, time.Second, apply)
	if !errors.Is(err, ErrCoprocessResult) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCoprocessRejectsInvalidBeginWithoutApplying(t *testing.T) {
	valid := `{"schema":1,"type":"begin","plan_id":"p","step":"","sequence":0,"begin":{"plan":{"plan_id":"p"},"candidate":{}}}`
	for name, input := range map[string]string{
		"empty stdin":    "",
		"not json":       "nope\n",
		"unknown field":  strings.Replace(valid, `"step"`, `"future":1,"step"`, 1) + "\n",
		"unknown nested": strings.Replace(valid, `"candidate":{}`, `"candidate":{"future":1}`, 1) + "\n",
		"wrong schema":   strings.Replace(valid, `"schema":1`, `"schema":2`, 1) + "\n",
		"trailing":       strings.TrimSuffix(valid, "\n") + " {}\n",
		"plan mismatch":  strings.Replace(valid, `"plan":{"plan_id":"p"}`, `"plan":{"plan_id":"q"}`, 1) + "\n",
		"ack first":      `{"schema":1,"type":"ack","plan_id":"p","step":"","sequence":0}` + "\n",
		"oversized":      `{"pad":"` + strings.Repeat("x", coprocessMaxLine) + "\"}\n",
	} {
		t.Run(name, func(t *testing.T) {
			apply := func(context.Context, ApplyRequest) ApplyResult {
				t.Error("apply ran")
				return ApplyResult{}
			}
			var out strings.Builder
			err := runCoprocess(context.Background(), strings.NewReader(input), &out, time.Second, apply)
			if !errors.Is(err, ErrCoprocessBegin) || out.Len() != 0 {
				t.Fatalf("err=%v out=%q", err, out.String())
			}
		})
	}
}
