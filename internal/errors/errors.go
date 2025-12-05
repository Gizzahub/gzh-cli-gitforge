// Package errors provides gitforge-specific error types and utilities.
package errors

import (
	"errors"
	"fmt"
)

// Common error types
var (
	// ErrNotFound indicates a resource was not found
	ErrNotFound = errors.New("resource not found")

	// ErrUnauthorized indicates authentication failure
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden indicates permission denial
	ErrForbidden = errors.New("forbidden")

	// ErrRateLimited indicates API rate limit exceeded
	ErrRateLimited = errors.New("rate limit exceeded")

	// ErrInvalidInput indicates invalid user input
	ErrInvalidInput = errors.New("invalid input")

	// ErrAPIError indicates a general API error
	ErrAPIError = errors.New("API error")

	// ErrNetworkError indicates a network-related error
	ErrNetworkError = errors.New("network error")
)

// ForgeError represents a forge-specific error with context
type ForgeError struct {
	Forge      string // "github", "gitlab", "gitea"
	Operation  string // e.g., "create_repo", "list_prs"
	StatusCode int
	Err        error
}

// Error implements the error interface
func (e *ForgeError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s %s failed (HTTP %d): %v", e.Forge, e.Operation, e.StatusCode, e.Err)
	}
	return fmt.Sprintf("%s %s failed: %v", e.Forge, e.Operation, e.Err)
}

// Unwrap returns the underlying error
func (e *ForgeError) Unwrap() error {
	return e.Err
}

// NewForgeError creates a new ForgeError
func NewForgeError(forge, operation string, err error) *ForgeError {
	return &ForgeError{
		Forge:     forge,
		Operation: operation,
		Err:       err,
	}
}

// NewForgeHTTPError creates a new ForgeError with HTTP status code
func NewForgeHTTPError(forge, operation string, statusCode int, err error) *ForgeError {
	return &ForgeError{
		Forge:      forge,
		Operation:  operation,
		StatusCode: statusCode,
		Err:        err,
	}
}

// GitHub-specific errors
type GitHubError struct {
	Operation  string
	StatusCode int
	Message    string
	Err        error
}

func (e *GitHubError) Error() string {
	return fmt.Sprintf("GitHub %s failed (HTTP %d): %s", e.Operation, e.StatusCode, e.Message)
}

func (e *GitHubError) Unwrap() error {
	return e.Err
}

// GitLab-specific errors
type GitLabError struct {
	Operation  string
	StatusCode int
	Message    string
	Err        error
}

func (e *GitLabError) Error() string {
	return fmt.Sprintf("GitLab %s failed (HTTP %d): %s", e.Operation, e.StatusCode, e.Message)
}

func (e *GitLabError) Unwrap() error {
	return e.Err
}

// Gitea-specific errors
type GiteaError struct {
	Operation  string
	StatusCode int
	Message    string
	Err        error
}

func (e *GiteaError) Error() string {
	return fmt.Sprintf("Gitea %s failed (HTTP %d): %s", e.Operation, e.StatusCode, e.Message)
}

func (e *GiteaError) Unwrap() error {
	return e.Err
}

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// WrapWithMessage wraps an error with a formatted message
func WrapWithMessage(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// IsNotFound checks if the error is a not found error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsUnauthorized checks if the error is an unauthorized error
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsForbidden checks if the error is a forbidden error
func IsForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// IsRateLimited checks if the error is a rate limit error
func IsRateLimited(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// IsForgeError checks if the error is a ForgeError
func IsForgeError(err error) bool {
	var forgeErr *ForgeError
	return errors.As(err, &forgeErr)
}
