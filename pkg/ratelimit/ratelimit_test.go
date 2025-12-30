// Package ratelimit provides rate limiting utilities for Git forge APIs.
package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestNewLimiter(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		expectedLimit int
	}{
		{"positive limit", 1000, 1000},
		{"zero limit uses default", 0, 5000},
		{"negative limit uses default", -1, 5000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLimiter(tt.limit)
			if l == nil {
				t.Fatal("NewLimiter returned nil")
			}
			remaining, limit, _ := l.Status()
			if limit != tt.expectedLimit {
				t.Errorf("expected limit %d, got %d", tt.expectedLimit, limit)
			}
			if remaining != tt.expectedLimit {
				t.Errorf("expected remaining %d, got %d", tt.expectedLimit, remaining)
			}
		})
	}
}

func TestLimiter_Wait(t *testing.T) {
	t.Run("immediate return when remaining > 0", func(t *testing.T) {
		l := NewLimiter(100)
		ctx := context.Background()

		start := time.Now()
		err := l.Wait(ctx)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if elapsed > 100*time.Millisecond {
			t.Errorf("Wait took too long: %v", elapsed)
		}

		remaining, _, _ := l.Status()
		if remaining != 99 {
			t.Errorf("expected remaining 99, got %d", remaining)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		l := NewLimiter(100)
		l.SetRetryAfter(10 * time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := l.Wait(ctx)
		if err == nil {
			t.Error("expected error from canceled context")
		}
	})
}

func TestLimiter_UpdateFromHeaders(t *testing.T) {
	t.Run("GitHub style headers", func(t *testing.T) {
		l := NewLimiter(5000)
		resetTime := time.Now().Add(1 * time.Hour).Unix()

		header := make(http.Header)
		header.Set("X-RateLimit-Remaining", "4500")
		header.Set("X-RateLimit-Limit", "5000")
		header.Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))

		resp := &http.Response{Header: header}

		l.UpdateFromHeaders(resp)

		remaining, limit, reset := l.Status()
		if remaining != 4500 {
			t.Errorf("expected remaining 4500, got %d", remaining)
		}
		if limit != 5000 {
			t.Errorf("expected limit 5000, got %d", limit)
		}
		if reset.Unix() != resetTime {
			t.Errorf("expected reset time %d, got %d", resetTime, reset.Unix())
		}
	})

	t.Run("GitLab style headers", func(t *testing.T) {
		l := NewLimiter(2000)
		resetTime := time.Now().Add(30 * time.Minute).Unix()

		header := make(http.Header)
		header.Set("RateLimit-Remaining", "1800")
		header.Set("RateLimit-Limit", "2000")
		header.Set("RateLimit-Reset", strconv.FormatInt(resetTime, 10))

		resp := &http.Response{Header: header}

		l.UpdateFromHeaders(resp)

		remaining, limit, reset := l.Status()
		if remaining != 1800 {
			t.Errorf("expected remaining 1800, got %d", remaining)
		}
		if limit != 2000 {
			t.Errorf("expected limit 2000, got %d", limit)
		}
		if reset.Unix() != resetTime {
			t.Errorf("expected reset time %d, got %d", resetTime, reset.Unix())
		}
	})

	t.Run("Retry-After header", func(t *testing.T) {
		l := NewLimiter(5000)

		header := make(http.Header)
		header.Set("Retry-After", "30")

		resp := &http.Response{Header: header}

		l.UpdateFromHeaders(resp)
		// retryAfter is private, but affects Wait behavior
	})
}

func TestLimiter_SetRetryAfter(t *testing.T) {
	l := NewLimiter(100)
	l.SetRetryAfter(5 * time.Second)
	// Cannot directly verify private field, but it should affect Wait behavior
}

func TestLimiter_Status(t *testing.T) {
	l := NewLimiter(1000)

	remaining, limit, resetTime := l.Status()

	if remaining != 1000 {
		t.Errorf("expected remaining 1000, got %d", remaining)
	}
	if limit != 1000 {
		t.Errorf("expected limit 1000, got %d", limit)
	}
	if resetTime.Before(time.Now()) {
		t.Error("expected reset time to be in the future")
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name        string
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{"attempt 0", 0, 900 * time.Millisecond, 1100 * time.Millisecond},
		{"attempt 1", 1, 1800 * time.Millisecond, 2200 * time.Millisecond},
		{"attempt 2", 2, 3600 * time.Millisecond, 4400 * time.Millisecond},
		{"attempt 10 (capped)", 10, 54 * time.Second, 66 * time.Second},
		{"negative attempt", -1, 900 * time.Millisecond, 1100 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := CalculateBackoff(tt.attempt)
			if backoff < tt.minExpected || backoff > tt.maxExpected {
				t.Errorf("expected backoff between %v and %v, got %v",
					tt.minExpected, tt.maxExpected, backoff)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		resp     *http.Response
		expected bool
	}{
		{
			name:     "nil response",
			resp:     nil,
			expected: false,
		},
		{
			name: "429 Too Many Requests",
			resp: &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{},
			},
			expected: true,
		},
		{
			name: "500 Internal Server Error",
			resp: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     http.Header{},
			},
			expected: true,
		},
		{
			name: "502 Bad Gateway",
			resp: &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     http.Header{},
			},
			expected: true,
		},
		{
			name: "503 Service Unavailable",
			resp: &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Header:     http.Header{},
			},
			expected: true,
		},
		{
			name: "403 with remaining quota (secondary rate limit)",
			//nolint:bodyclose // test stub response, no actual body to close
			resp: func() *http.Response {
				h := make(http.Header)
				h.Set("X-RateLimit-Remaining", "100")
				return &http.Response{StatusCode: http.StatusForbidden, Header: h, Body: http.NoBody}
			}(),
			expected: true,
		},
		{
			name: "403 without remaining quota (not retryable)",
			//nolint:bodyclose // test stub response, no actual body to close
			resp: func() *http.Response {
				h := make(http.Header)
				h.Set("X-RateLimit-Remaining", "0")
				return &http.Response{StatusCode: http.StatusForbidden, Header: h, Body: http.NoBody}
			}(),
			expected: false,
		},
		{
			name: "200 OK",
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
			},
			expected: false,
		},
		{
			name: "400 Bad Request",
			resp: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{},
			},
			expected: false,
		},
		{
			name: "404 Not Found",
			resp: &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     http.Header{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldRetry(tt.resp)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRetryableError(t *testing.T) {
	t.Run("error with retry after", func(t *testing.T) {
		err := &RetryableError{
			Err:          context.DeadlineExceeded,
			RetryAfter:   30 * time.Second,
			AttemptsLeft: 2,
		}

		if !err.IsRetryable() {
			t.Error("expected error to be retryable")
		}

		msg := err.Error()
		if msg == "" {
			t.Error("expected non-empty error message")
		}
	})

	t.Run("error without retry after", func(t *testing.T) {
		err := &RetryableError{
			Err:          context.DeadlineExceeded,
			AttemptsLeft: 1,
		}

		if !err.IsRetryable() {
			t.Error("expected error to be retryable")
		}

		msg := err.Error()
		if msg == "" {
			t.Error("expected non-empty error message")
		}
	})

	t.Run("error not retryable", func(t *testing.T) {
		err := &RetryableError{
			Err:          context.DeadlineExceeded,
			AttemptsLeft: 0,
		}

		if err.IsRetryable() {
			t.Error("expected error to not be retryable")
		}
	})

	t.Run("Unwrap", func(t *testing.T) {
		innerErr := context.DeadlineExceeded
		err := &RetryableError{
			Err: innerErr,
		}

		if !errors.Is(err, innerErr) {
			t.Error("expected Unwrap to return inner error")
		}
	})
}
