// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"errors"
	"fmt"
	"testing"
)

func TestRunExitCode(t *testing.T) {
	integrated := &RunReport{Integrated: true}
	cases := []struct {
		name   string
		report *RunReport
		err    error
		want   int
	}{
		{name: "no error is success", report: integrated, err: nil, want: RunExitOK},
		{name: "integrated but reclaim incomplete is exit 3", report: integrated, err: errors.New("cleanup failed"), want: RunExitReclaimIncomplete},
		{name: "source mismatch is exit 4", report: nil, err: ErrSourceMismatch, want: RunExitSourceMismatch},
		{name: "target mismatch is exit 4", report: nil, err: ErrTargetMismatch, want: RunExitSourceMismatch},
		{name: "wrapped source mismatch is exit 4", report: nil, err: fmt.Errorf("run refused: %w: sha", ErrSourceMismatch), want: RunExitSourceMismatch},
		{name: "interrupted is exit 2", report: nil, err: ErrInterrupted, want: RunExitInterrupted},
		{name: "implicit source is target is exit 1", report: nil, err: ErrImplicitSourceIsTarget, want: RunExitToolError},
		{name: "unreadiness is exit 1", report: nil, err: errors.New("integration branch is not ready: behind the target"), want: RunExitToolError},
		{name: "unknown failure is exit 2", report: nil, err: errors.New("boom"), want: RunExitInterrupted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RunExitCode(tc.report, tc.err); got != tc.want {
				t.Fatalf("RunExitCode = %d, want %d", got, tc.want)
			}
		})
	}
}
