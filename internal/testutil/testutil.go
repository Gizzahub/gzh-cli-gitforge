// Package testutil provides utilities for testing gitforge functionality.
package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// AssertEqual checks if two values are equal and fails the test if not
func AssertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// AssertNotNil checks if a value is not nil and fails the test if it is
func AssertNotNil(t *testing.T, got interface{}) {
	t.Helper()
	if got == nil {
		t.Error("expected non-nil value, got nil")
	}
}

// AssertNil checks if a value is nil and fails the test if not
func AssertNil(t *testing.T, got interface{}) {
	t.Helper()
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// AssertNoError checks if an error is nil and fails the test if not
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// AssertError checks if an error is not nil and fails the test if it is
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// TempDir creates a temporary directory for testing
func TempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gitforge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// TempFile creates a temporary file with the given content
func TempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := TempDir(t)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

// MockHTTPServer creates a mock HTTP server for testing
func MockHTTPServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

// MockJSONResponse creates a mock HTTP handler that returns a JSON response
func MockJSONResponse(t *testing.T, statusCode int, data interface{}) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if err := json.NewEncoder(w).Encode(data); err != nil {
			t.Errorf("failed to encode JSON: %v", err)
		}
	}
}

// MockErrorResponse creates a mock HTTP handler that returns an error response
func MockErrorResponse(t *testing.T, statusCode int, message string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		fmt.Fprintf(w, `{"message": "%s"}`, message)
	}
}

// CaptureOutput captures stdout and stderr during test execution
func CaptureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rOut)
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rErr)
		errC <- buf.String()
	}()

	fn()

	wOut.Close()
	wErr.Close()

	stdout = <-outC
	stderr = <-errC

	return stdout, stderr
}

// NewTestConfig creates a test configuration for forge clients
func NewTestConfig() map[string]interface{} {
	return map[string]interface{}{
		"github_token": "test-github-token",
		"gitlab_token": "test-gitlab-token",
		"gitea_url":    "https://gitea.example.com",
		"gitea_token":  "test-gitea-token",
	}
}

// SkipIfShort skips the test if running in short mode
func SkipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
}

// ReadFixture reads a test fixture file
func ReadFixture(t *testing.T, filename string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", filename, err)
	}
	return data
}
