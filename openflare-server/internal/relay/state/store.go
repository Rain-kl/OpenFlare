package state

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
)

type Store struct {
	path string
	mu   sync.RWMutex
}

type State struct {
	LastAuthToken          string `json:"last_auth_token"`
	LastProfileFingerprint string `json:"last_profile_fingerprint"`
	LastCPUStatTotal       uint64 `json:"last_cpu_stat_total"`
	LastCPUStatIdle        uint64 `json:"last_cpu_stat_idle"`
	LastMetricAtUnix       int64  `json:"last_metric_at_unix"`
}

func NewStore(path string) *Store {
	return &Store{
		path: path,
	}
}

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

func (s *Store) Save(state *State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	slog.Debug("saving relay state")
	return os.WriteFile(s.path, data, 0644)
}
