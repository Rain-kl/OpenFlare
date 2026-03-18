package state

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Snapshot struct {
	NodeID                 string `json:"node_id"`
	CurrentVersion         string `json:"current_version"`
	CurrentChecksum        string `json:"current_checksum"`
	BlockedVersion         string `json:"blocked_version"`
	BlockedChecksum        string `json:"blocked_checksum"`
	BlockedReason          string `json:"blocked_reason"`
	LastError              string `json:"last_error"`
	OpenrestyStatus        string `json:"openresty_status"`
	OpenrestyMessage       string `json:"openresty_message"`
	LastProfileFingerprint string `json:"last_profile_fingerprint"`
	LastCPUStatTotal       uint64 `json:"last_cpu_stat_total"`
	LastCPUStatIdle        uint64 `json:"last_cpu_stat_idle"`
	LastMetricAtUnix       int64  `json:"last_metric_at_unix"`
	AccessLogOffset        int64  `json:"access_log_offset"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store {
	return &Store{path: filepath.Clean(path)}
}

func (s *Store) Load() (*Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadUnlocked()
}

func (s *Store) EnsureNodeID() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadUnlocked()
	if err != nil {
		return "", err
	}
	if snapshot.NodeID != "" {
		return snapshot.NodeID, nil
	}
	snapshot.NodeID, err = newNodeID()
	if err != nil {
		return "", err
	}
	if err = s.saveUnlocked(snapshot); err != nil {
		return "", err
	}
	return snapshot.NodeID, nil
}

func (s *Store) Save(snapshot *Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUnlocked(snapshot)
}

func (s *Store) loadUnlocked() (*Snapshot, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Snapshot{}, nil
		}
		return nil, err
	}
	snapshot := &Snapshot{}
	if len(data) == 0 {
		return snapshot, nil
	}
	if err = json.Unmarshal(data, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *Store) saveUnlocked(snapshot *Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func newNodeID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "node-" + hex.EncodeToString(buf), nil
}
