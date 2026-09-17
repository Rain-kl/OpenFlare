// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import (
	"context"
	"time"
)

// Rate specifies a rate limit of Limit events permitted within a Period.
type Rate struct {
	Limit  int           `json:"limit"`
	Period time.Duration `json:"period"`
}

// RateLimitResult holds the outcome of a rate limit check.
type RateLimitResult struct {
	Allowed    bool          `json:"allowed"`
	Remaining  int           `json:"remaining"`
	ResetAfter time.Duration `json:"reset_after"`
	RetryAfter time.Duration `json:"retry_after"`
}

// LimiterService defines the rate limiting service contract for cross-plugin communication.
type LimiterService interface {
	// Allow checks whether 1 event for the given key is permitted under the specified rate.
	Allow(ctx context.Context, key string, rate Rate) (*RateLimitResult, error)

	// AllowN checks whether n events for the given key are permitted under the specified rate.
	AllowN(ctx context.Context, key string, rate Rate, n int) (*RateLimitResult, error)

	// Reset clears the rate limit state for the given key.
	Reset(ctx context.Context, key string) error
}
