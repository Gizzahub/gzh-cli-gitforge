// Package errors provides custom error types and utilities for error handling.
package errors

import (
	"errors"
	"fmt"
)

// Standard errors - define domain-specific errors here.
var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrConfigNotFound = errors.New("config not found")
	ErrInvalidConfig  = errors.New("invalid config")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrTimeout        = errors.New("operation timed out")

	// Git-specific errors.
	ErrNotGitRepository = errors.New("not a git repository")
	ErrDirtyWorkingTree = errors.New("working tree has uncommitted changes")
	ErrBranchExists     = errors.New("branch already exists")
	ErrBranchNotFound   = errors.New("branch not found")
	ErrRemoteNotFound   = errors.New("remote not found")
	ErrMergeConflict    = errors.New("merge conflict")
	ErrDetachedHead     = errors.New("HEAD is detached")
)

// Wrap combines two errors, preserving the chain for errors.Is/As.
// If err is nil, returns target. If target is nil, returns err.
func Wrap(err, target error) error {
	if err == nil {
		return target
	}
	if target == nil {
		return err
	}
	return fmt.Errorf("%w: %w", target, err)
}

// WrapWithMessage wraps an error with additional context message.
func WrapWithMessage(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// New creates a new error with the given message.
func New(message string) error {
	return errors.New(message)
}

// Is reports whether any error in err's tree matches target.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's tree that matches target.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Join returns an error that wraps the given errors.
func Join(errs ...error) error {
	return errors.Join(errs...)
}
