// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cliutil

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewExitError_NilPassthrough(t *testing.T) {
	if err := NewExitError(ExitPartialFailed, nil); err != nil {
		t.Fatalf("NewExitError(_, nil) = %v, want nil", err)
	}
}

func TestExitError_UnwrapAndAs(t *testing.T) {
	base := errors.New("boom")
	wrapped := NewExitError(ExitPartialFailed, fmt.Errorf("context: %w", base))

	if !errors.Is(wrapped, base) {
		t.Errorf("errors.Is(wrapped, base) = false, want true")
	}

	var exitErr *ExitError
	if !errors.As(wrapped, &exitErr) {
		t.Fatalf("errors.As failed to extract *ExitError")
	}
	if exitErr.Code != ExitPartialFailed {
		t.Errorf("Code = %d, want %d", exitErr.Code, ExitPartialFailed)
	}
}

func TestExitCodeForError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil is success", nil, ExitOK},
		{"plain error defaults to 1", errors.New("x"), ExitToolError},
		{"tool error code 1", NewExitError(ExitToolError, errors.New("bad flag")), ExitToolError},
		{"partial failure code 2", NewExitError(ExitPartialFailed, errors.New("3 of 3 failed")), ExitPartialFailed},
		{"wrapped exit error keeps its code", fmt.Errorf("outer: %w", NewExitError(ExitPartialFailed, errors.New("inner"))), ExitPartialFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCodeForError(tt.err); got != tt.want {
				t.Errorf("ExitCodeForError() = %d, want %d", got, tt.want)
			}
		})
	}
}
