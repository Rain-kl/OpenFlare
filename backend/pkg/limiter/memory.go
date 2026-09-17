// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package limiter provides in-memory rate limiting utilities without external project dependencies.
package limiter

import (
	"context"
	"sync"
	"time"
)

// Rate specifies a rate limit of Limit events permitted within a Period.
type Rate struct {
	Limit  int
	Period time.Duration
}

// Result holds the outcome of an Allow check.
type Result struct {
	Allowed    bool
	Remaining  int
	ResetAfter time.Duration
	RetryAfter time.Duration
}

type memoryEntry struct {
	timestamps []time.Time
	lastSeen   time.Time
}

func (e *memoryEntry) prune(cutoff time.Time) {
	validIdx := len(e.timestamps)
	for i, ts := range e.timestamps {
		if ts.After(cutoff) {
			validIdx = i
			break
		}
	}
	if validIdx > 0 && validIdx <= len(e.timestamps) {
		e.timestamps = e.timestamps[validIdx:]
	}
}

func (e *memoryEntry) calcBlockedResult(limit int, period time.Duration, now time.Time) *Result {
	currentCount := len(e.timestamps)
	if currentCount == 0 {
		return &Result{
			Allowed:    false,
			Remaining:  limit,
			ResetAfter: period,
			RetryAfter: 0,
		}
	}

	oldest := e.timestamps[0]
	retryAfter := max(0, oldest.Add(period).Sub(now))

	newest := e.timestamps[currentCount-1]
	resetAfter := max(0, newest.Add(period).Sub(now))

	return &Result{
		Allowed:    false,
		Remaining:  limit - currentCount,
		ResetAfter: resetAfter,
		RetryAfter: retryAfter,
	}
}

// MemoryLimiter implements an in-memory sliding window rate limiter.
type MemoryLimiter struct {
	mu      sync.Mutex
	entries map[string]*memoryEntry
}

// NewMemoryLimiter creates a new in-memory rate limiter.
func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{
		entries: make(map[string]*memoryEntry),
	}
}

// Allow checks whether 1 event for key is permitted under rate.
func (m *MemoryLimiter) Allow(ctx context.Context, key string, rate Rate) (*Result, error) {
	return m.AllowN(ctx, key, rate, 1)
}

// AllowN checks whether n events for key are permitted under rate.
func (m *MemoryLimiter) AllowN(_ context.Context, key string, rate Rate, n int) (*Result, error) {
	if rate.Limit <= 0 || rate.Period <= 0 || n <= 0 {
		return &Result{Allowed: true}, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rate.Period)

	entry, ok := m.entries[key]
	if !ok {
		entry = &memoryEntry{}
		m.entries[key] = entry
	}
	entry.lastSeen = now
	entry.prune(cutoff)

	if len(entry.timestamps)+n > rate.Limit {
		return entry.calcBlockedResult(rate.Limit, rate.Period, now), nil
	}

	for i := 0; i < n; i++ {
		entry.timestamps = append(entry.timestamps, now)
	}

	remaining := max(0, rate.Limit-len(entry.timestamps))

	return &Result{
		Allowed:    true,
		Remaining:  remaining,
		ResetAfter: rate.Period,
		RetryAfter: 0,
	}, nil
}

// Reset clears rate limit state for key.
func (m *MemoryLimiter) Reset(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return nil
}
