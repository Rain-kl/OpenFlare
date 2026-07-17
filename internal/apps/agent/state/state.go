package state

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const (
	stateDirPerm      = 0o750
	stateFilePerm     = 0o600
	nodeIDRandomBytes = 8
)

// PagesDeployment records a Pages project tracked by the agent and the last
// applied package hash for that project's active deployment.
// ProjectID is the stable identity; DeploymentID/Hash follow control-plane "latest".
type PagesDeployment struct {
	ProjectID    uint   `json:"project_id"`
	DeploymentID uint   `json:"deployment_id,omitempty"`
	Hash         string `json:"hash"`
	Checksum     string `json:"checksum,omitempty"`
}

// Snapshot represents the state of the agent at a given point in time.
type Snapshot struct {
	NodeID                 string            `json:"node_id"`
	CurrentVersion         string            `json:"current_version"`
	CurrentChecksum        string            `json:"current_checksum"`
	PagesDeployments       []PagesDeployment `json:"pages_deployments"`
	BlockedVersion         string            `json:"blocked_version"`
	BlockedChecksum        string            `json:"blocked_checksum"`
	BlockedReason          string            `json:"blocked_reason"`
	LastError              string            `json:"last_error"`
	OpenrestyStatus        string            `json:"openresty_status"`
	OpenrestyMessage       string            `json:"openresty_message"`
	LastProfileFingerprint string            `json:"last_profile_fingerprint"`
	LastCPUStatTotal       uint64            `json:"last_cpu_stat_total"`
	LastCPUStatIdle        uint64            `json:"last_cpu_stat_idle"`
	LastMetricAtUnix       int64             `json:"last_metric_at_unix"`
	AccessLogOffset        int64             `json:"access_log_offset"`
}

// Store manages the storage and retrieval of the agent state snapshot.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore creates a new Store instance at the given path.
func NewStore(path string) *Store {
	return &Store{path: filepath.Clean(path)}
}

// Load loads the snapshot from the store.
func (s *Store) Load() (*Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadUnlocked()
}

// EnsureNodeID returns the existing node ID, or generates and saves a new one if it does not exist.
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

// Save saves the given snapshot to the store.
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
	if err := os.MkdirAll(filepath.Dir(s.path), stateDirPerm); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, stateFilePerm)
}

func newNodeID() (string, error) {
	buf := make([]byte, nodeIDRandomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "node-" + hex.EncodeToString(buf), nil
}
