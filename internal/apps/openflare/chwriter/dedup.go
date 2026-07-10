// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package chwriter

import (
	"sync"
	"time"
)

const dedupTTL = 2 * time.Minute

type dedupSet struct {
	mu          sync.Mutex
	keys        map[string]time.Time
	lastCleanup time.Time
}

func newDedupSet() *dedupSet {
	return &dedupSet{
		keys:        make(map[string]time.Time),
		lastCleanup: time.Now(),
	}
}

// markIfNew records key when it has not been seen within dedupTTL.
func (s *dedupSet) markIfNew(key string) bool {
	if s == nil || key == "" {
		return false
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked(now)

	if expiresAt, exists := s.keys[key]; exists && now.Before(expiresAt) {
		return false
	}
	s.keys[key] = now.Add(dedupTTL)
	return true
}

// unmark removes a key so a later enqueue or flush retry may accept it again.
func (s *dedupSet) unmark(key string) {
	if s == nil || key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, key)
}

func (s *dedupSet) cleanupExpiredLocked(now time.Time) {
	if now.Sub(s.lastCleanup) < 30*time.Second {
		return
	}
	for existing, expiresAt := range s.keys {
		if now.After(expiresAt) {
			delete(s.keys, existing)
		}
	}
	s.lastCleanup = now
}
