// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

// Package ratelimit provides rate limiting functionality for API calls.
//
// This package implements a token bucket algorithm to prevent exceeding
// API rate limits when interacting with Git forges (GitHub, GitLab, Gitea).
//
// # Usage
//
//	limiter := ratelimit.NewLimiter(5000) // 5000 requests/hour
//	limiter.Wait()                         // Block until request allowed
//	remaining, limit, reset := limiter.Status()
package ratelimit
