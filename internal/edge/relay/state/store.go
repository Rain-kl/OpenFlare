// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package state persists relay runtime state to disk.
package state

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
)

const relayStateFilePerm = 0o600

// Store reads and writes relay state from a JSON file.
type Store struct {
	path string
	mu   sync.RWMutex
}

// State holds persisted relay metrics and authentication state.
type State struct {
	LastAuthToken          string `json:"last_auth_token"`
	LastProfileFingerprint string `json:"last_profile_fingerprint"`
	LastCPUStatTotal       uint64 `json:"last_cpu_stat_total"`
	LastCPUStatIdle        uint64 `json:"last_cpu_stat_idle"`
	LastMetricAtUnix       int64  `json:"last_metric_at_unix"`
}

// NewStore creates a state store backed by the file at path.
func NewStore(path string) *Store {
	return &Store{
		path: path,
	}
}

// Load reads the persisted state from disk, returning an empty state if the file is missing.
func (s *Store) Load() (*State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return &State{}, nil // Return empty state on corrupted file
	}
	return &state, nil
}

// Save writes state to disk.
func (s *Store) Save(state *State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	slog.Debug("saving relay state")
	return os.WriteFile(s.path, data, relayStateFilePerm)
}
