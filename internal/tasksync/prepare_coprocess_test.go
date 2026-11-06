// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestPrepareCoprocessRejectsOversizedOrTrailingInputBeforePreparing(t *testing.T) {
	tests := map[string]string{
		"malformed":              "{",
		"trailing document":      "{} {}",
		"input beyond the limit": "{}" + strings.Repeat(" ", coprocessMaxLine) + "{}",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			calls := 0
			var out bytes.Buffer
			err := runPrepareCoprocess(context.Background(), strings.NewReader(input), &out, func(context.Context, Request) Result {
				calls++
				return Result{}
			})
			if !errors.Is(err, ErrCoprocessBegin) || calls != 0 || out.Len() != 0 {
				t.Fatalf("err=%v calls=%d output=%q", err, calls, out.String())
			}
		})
	}
}

func TestPrepareCoprocessResultLossPreservesCandidate(t *testing.T) {
	f := newFixture(t, false)
	raw, err := json.Marshal(f.request())
	if err != nil {
		t.Fatal(err)
	}
	err = RunPrepareCoprocess(context.Background(), bytes.NewReader(raw), failingWriter{})
	if !errors.Is(err, ErrCoprocessResult) {
		t.Fatalf("result loss must be uncertain: %v", err)
	}
	if got := gitOutput(t, f.task, "rev-parse", "HEAD"); got != f.taskHead {
		t.Fatalf("task HEAD moved to %s", got)
	}
	refs := strings.Fields(gitOutput(t, f.canonical, "for-each-ref", "--format=%(refname)", refPrefix))
	if len(refs) != 1 || !strings.HasPrefix(refs[0], refPrefix) {
		t.Fatalf("prepared candidate was not retained: %v", refs)
	}
}
