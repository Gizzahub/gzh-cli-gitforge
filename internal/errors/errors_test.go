package errors

import (
	"errors"
	"testing"
)

func TestWrap(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		wantIs error
	}{
		{
			name:   "wrap with target",
			err:    errors.New("original error"),
			target: ErrNotFound,
			wantIs: ErrNotFound,
		},
		{
			name:   "nil err returns target",
			err:    nil,
			target: ErrNotFound,
			wantIs: ErrNotFound,
		},
		{
			name:   "nil target returns err",
			err:    errors.New("original"),
			target: nil,
			wantIs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.err, tt.target)
			if tt.wantIs != nil && !Is(got, tt.wantIs) {
				t.Errorf("Wrap() error should match %v", tt.wantIs)
			}
		})
	}
}

func TestWrapWithMessage(t *testing.T) {
	original := errors.New("original error")
	wrapped := WrapWithMessage(original, "context")

	if wrapped == nil {
		t.Error("WrapWithMessage should return non-nil error")
	}

	if !Is(wrapped, original) {
		t.Error("wrapped error should match original")
	}

	// nil error should return nil.
	if WrapWithMessage(nil, "context") != nil {
		t.Error("WrapWithMessage(nil) should return nil")
	}
}

func TestGitSpecificErrors(t *testing.T) {
	// Verify git-specific errors are defined.
	gitErrors := []error{
		ErrNotGitRepository,
		ErrDirtyWorkingTree,
		ErrBranchExists,
		ErrBranchNotFound,
		ErrRemoteNotFound,
		ErrMergeConflict,
		ErrDetachedHead,
	}

	for _, err := range gitErrors {
		if err == nil {
			t.Error("git-specific error should not be nil")
		}
	}
}
