// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package ratelimit

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Limiter handles API rate limiting with retry logic.
type Limiter struct {
	mu         sync.Mutex
	limit      int
	remaining  int
	resetTime  time.Time
	retryAfter time.Duration
	maxRetries int
}

// NewLimiter creates a new rate limiter with default settings.
func NewLimiter(limit int) *Limiter {
	if limit <= 0 {
		limit = 5000 // Default GitHub rate limit
	}
	return &Limiter{
		limit:      limit,
		remaining:  limit,
		resetTime:  time.Now().Add(1 * time.Hour),
		maxRetries: 3,
	}
}

// Wait blocks until rate limit allows making a request.
func (l *Limiter) Wait(ctx context.Context) error {
	l.mu.Lock()

	// Check if we need to wait for retry-after
	if l.retryAfter > 0 {
		waitDuration := l.retryAfter
		l.retryAfter = 0
		l.mu.Unlock()

		if err := sleep(ctx, waitDuration); err != nil {
			return err
		}

		l.mu.Lock()
	}

	// Check rate limit
	if l.remaining <= 0 && time.Now().Before(l.resetTime) {
		waitDuration := time.Until(l.resetTime)
		l.mu.Unlock()

		if err := sleep(ctx, waitDuration); err != nil {
			return err
		}

		l.mu.Lock()
	}

	l.remaining--
	l.mu.Unlock()

	return nil
}

// UpdateFromHeaders updates rate limit information from response headers.
// Supports GitHub (X-RateLimit-*) and GitLab (RateLimit-*) headers.
func (l *Limiter) UpdateFromHeaders(resp *http.Response) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// GitHub style headers
	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if r, err := strconv.Atoi(remaining); err == nil {
			l.remaining = r
		}
	}
	if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
		if lim, err := strconv.Atoi(limit); err == nil {
			l.limit = lim
		}
	}
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if r, err := strconv.ParseInt(reset, 10, 64); err == nil {
			l.resetTime = time.Unix(r, 0)
		}
	}

	// GitLab style headers (fallback)
	if remaining := resp.Header.Get("RateLimit-Remaining"); remaining != "" {
		if r, err := strconv.Atoi(remaining); err == nil {
			l.remaining = r
		}
	}
	if limit := resp.Header.Get("RateLimit-Limit"); limit != "" {
		if lim, err := strconv.Atoi(limit); err == nil {
			l.limit = lim
		}
	}
	if reset := resp.Header.Get("RateLimit-Reset"); reset != "" {
		if r, err := strconv.ParseInt(reset, 10, 64); err == nil {
			l.resetTime = time.Unix(r, 0)
		}
	}

	// Retry-After header (both)
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			l.retryAfter = time.Duration(seconds) * time.Second
		}
	}
}

// SetRetryAfter sets the retry-after duration.
func (l *Limiter) SetRetryAfter(duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.retryAfter = duration
}

// Status returns current rate limit status.
func (l *Limiter) Status() (remaining, limit int, resetTime time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.remaining, l.limit, l.resetTime
}

// CalculateBackoff calculates exponential backoff with jitter.
func CalculateBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	// Base backoff: 2^attempt seconds
	backoff := time.Duration(1<<uint(attempt)) * time.Second

	// Cap at 60 seconds
	if backoff > 60*time.Second {
		backoff = 60 * time.Second
	}

	// Add jitter (10% of backoff)
	jitter := time.Duration(rand.Float64() * float64(backoff) * 0.1)

	return backoff + jitter
}

// ShouldRetry determines if a response indicates we should retry.
func ShouldRetry(resp *http.Response) bool {
	if resp == nil {
		return false
	}

	// Retry on rate limit errors
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}

	// Retry on server errors
	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		return true
	}

	// Secondary rate limit (GitHub specific)
	if resp.StatusCode == http.StatusForbidden {
		if resp.Header.Get("X-RateLimit-Remaining") != "0" {
			return true
		}
	}

	return false
}

// RetryableError represents an error that can be retried.
type RetryableError struct {
	Err           error
	RetryAfter    time.Duration
	AttemptsLeft  int
	NextRetryTime time.Time
}

func (e *RetryableError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%v (retry after %v, %d attempts left)",
			e.Err, e.RetryAfter, e.AttemptsLeft)
	}
	return fmt.Sprintf("%v (%d attempts left)", e.Err, e.AttemptsLeft)
}

// IsRetryable returns true if the error is retryable.
func (e *RetryableError) IsRetryable() bool {
	return e.AttemptsLeft > 0
}

// Unwrap returns the underlying error.
func (e *RetryableError) Unwrap() error {
	return e.Err
}

func sleep(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
