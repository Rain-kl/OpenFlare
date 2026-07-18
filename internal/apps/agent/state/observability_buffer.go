// Package state persists agent runtime state and observability snapshots.
package state

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
)

const observabilityBufferWindowSeconds = 60

// ObservabilityBufferRecord stores observability facts for a single time window.
// Disk JSON is schema-v2 only: host_metrics / edge_health / access_logs.
// Pre-v2 buffers are discarded on load (binary upgrade without data-dir wipe).
type ObservabilityBufferRecord struct {
	WindowStartedAtUnix int64                        `json:"window_started_at_unix"`
	HostMetrics         *protocol.NodeMetricSnapshot `json:"host_metrics,omitempty"`
	EdgeHealth          *protocol.NodeEdgeHealth     `json:"edge_health,omitempty"`
	AccessLogs          []protocol.NodeAccessLog     `json:"access_logs,omitempty"`
	QueuedAtUnix        int64                        `json:"queued_at_unix"`
}

// ObservabilityBufferStore persists observability records to disk for replay on heartbeat.
type ObservabilityBufferStore struct {
	path        string
	mu          sync.Mutex
	cache       []ObservabilityBufferRecord
	cacheLoaded bool
}

// NewObservabilityBufferStore creates a store backed by the file at path.
func NewObservabilityBufferStore(path string) *ObservabilityBufferStore {
	return &ObservabilityBufferStore{path: filepath.Clean(path)}
}

// Upsert inserts or merges an observability record and prunes entries older than retainAfterUnix.
func (s *ObservabilityBufferStore) Upsert(record ObservabilityBufferRecord, retainAfterUnix int64) error {
	if s == nil || record.WindowStartedAtUnix <= 0 || (record.HostMetrics == nil && record.EdgeHealth == nil && len(record.AccessLogs) == 0) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	records = pruneObservabilityBufferRecords(records, retainAfterUnix)
	replaced := false
	for index := range records {
		if records[index].WindowStartedAtUnix != record.WindowStartedAtUnix {
			continue
		}
		records[index] = mergeObservabilityBufferRecord(records[index], record)
		replaced = true
		break
	}
	if !replaced {
		records = append(records, record)
	}
	sort.Slice(records, func(i int, j int) bool {
		return records[i].WindowStartedAtUnix < records[j].WindowStartedAtUnix
	})
	return s.saveUnlocked(records)
}

func mergeObservabilityBufferRecord(existing ObservabilityBufferRecord, incoming ObservabilityBufferRecord) ObservabilityBufferRecord {
	merged := existing
	if incoming.HostMetrics != nil {
		merged.HostMetrics = incoming.HostMetrics
	}
	if incoming.EdgeHealth != nil {
		merged.EdgeHealth = incoming.EdgeHealth
	}
	merged.AccessLogs = mergeAccessLogs(existing.AccessLogs, incoming.AccessLogs)
	if incoming.QueuedAtUnix > 0 {
		merged.QueuedAtUnix = incoming.QueuedAtUnix
	}
	return merged
}

func mergeAccessLogs(existing []protocol.NodeAccessLog, incoming []protocol.NodeAccessLog) []protocol.NodeAccessLog {
	if len(existing) == 0 && len(incoming) == 0 {
		return nil
	}
	merged := make([]protocol.NodeAccessLog, 0, len(existing)+len(incoming))
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	appendIfNeeded := func(items []protocol.NodeAccessLog) {
		for _, item := range items {
			key := accessLogKey(item)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, item)
		}
	}
	appendIfNeeded(existing)
	appendIfNeeded(incoming)
	sort.Slice(merged, func(i int, j int) bool {
		if merged[i].LoggedAtUnix == merged[j].LoggedAtUnix {
			return accessLogKey(merged[i]) < accessLogKey(merged[j])
		}
		return merged[i].LoggedAtUnix < merged[j].LoggedAtUnix
	})
	return merged
}

func accessLogKey(item protocol.NodeAccessLog) string {
	return strconv.FormatInt(item.LoggedAtUnix, 10) + "|" + item.RemoteAddr + "|" + item.Host + "|" + item.Path + "|" + strconv.Itoa(item.StatusCode)
}

// Replayable returns buffered records from windows before currentWindowStartedAtUnix.
func (s *ObservabilityBufferStore) Replayable(currentWindowStartedAtUnix int64, retainAfterUnix int64) ([]ObservabilityBufferRecord, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.loadUnlocked()
	if err != nil {
		return nil, err
	}
	records = pruneObservabilityBufferRecords(records, retainAfterUnix)
	if err = s.saveUnlocked(records); err != nil {
		return nil, err
	}
	result := make([]ObservabilityBufferRecord, 0, len(records))
	for _, record := range records {
		if currentWindowStartedAtUnix > 0 && record.WindowStartedAtUnix >= currentWindowStartedAtUnix {
			continue
		}
		result = append(result, record)
	}
	return result, nil
}

// Ack removes acknowledged observability windows and prunes entries older than retainAfterUnix.
func (s *ObservabilityBufferStore) Ack(windowStartedAtUnix []int64, retainAfterUnix int64) error {
	if s == nil || len(windowStartedAtUnix) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	acked := make(map[int64]struct{}, len(windowStartedAtUnix))
	for _, value := range windowStartedAtUnix {
		if value > 0 {
			acked[value] = struct{}{}
		}
	}
	filtered := make([]ObservabilityBufferRecord, 0, len(records))
	for _, record := range records {
		if _, ok := acked[record.WindowStartedAtUnix]; ok {
			continue
		}
		filtered = append(filtered, record)
	}
	filtered = pruneObservabilityBufferRecords(filtered, retainAfterUnix)
	return s.saveUnlocked(filtered)
}

func (s *ObservabilityBufferStore) loadUnlocked() ([]ObservabilityBufferRecord, error) {
	if s.cacheLoaded {
		copied := make([]ObservabilityBufferRecord, len(s.cache))
		copy(copied, s.cache)
		return copied, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.cache = []ObservabilityBufferRecord{}
			s.cacheLoaded = true
			return []ObservabilityBufferRecord{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		s.cache = []ObservabilityBufferRecord{}
		s.cacheLoaded = true
		return []ObservabilityBufferRecord{}, nil
	}

	// Binary upgrade: drop pre-v2 or corrupt buffer entirely; agent rebuilds on subsequent heartbeats.
	records, reason, ok := parseObservabilityBufferDisk(data)
	if !ok {
		s.discardBufferFile(reason)
		return []ObservabilityBufferRecord{}, nil
	}

	s.cache = records
	s.cacheLoaded = true
	copied := make([]ObservabilityBufferRecord, len(s.cache))
	copy(copied, s.cache)
	return copied, nil
}

// parseObservabilityBufferDisk returns v2 records, or ok=false when the on-disk file should be wiped.
func parseObservabilityBufferDisk(data []byte) (records []ObservabilityBufferRecord, reason string, ok bool) {
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return []ObservabilityBufferRecord{}, "", true
	}
	// Valid buffer is a JSON array of window records.
	if !strings.HasPrefix(raw, "[") {
		return nil, "legacy or unreadable observability buffer", false
	}
	// Pre-v2 keys: discard whole file (no field migration).
	if strings.Contains(raw, `"snapshot"`) ||
		strings.Contains(raw, `"openresty_observation"`) ||
		strings.Contains(raw, `"traffic_report"`) {
		return nil, "legacy observability buffer format", false
	}
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, "observability buffer JSON decode failed", false
	}
	return records, "", true
}

func (s *ObservabilityBufferStore) discardBufferFile(reason string) {
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		slog.Warn("remove observability buffer failed", "path", s.path, "reason", reason, "error", err)
	} else {
		slog.Info("discarded observability buffer; will rebuild on run", "path", s.path, "reason", reason)
	}
	s.cache = []ObservabilityBufferRecord{}
	s.cacheLoaded = true
}

func (s *ObservabilityBufferStore) saveUnlocked(records []ObservabilityBufferRecord) error {
	if err := os.MkdirAll(filepath.Dir(s.path), stateDirPerm); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, data, stateFilePerm); err != nil { //nolint:gosec // path is agent-local buffer path from config
		return err
	}
	s.cache = records
	s.cacheLoaded = true
	return nil
}

// ObservabilityWindowStartedAt returns the 60s window start for host metrics or edge health.
func ObservabilityWindowStartedAt(hostMetrics *protocol.NodeMetricSnapshot, edgeHealth *protocol.NodeEdgeHealth) int64 {
	if edgeHealth != nil && edgeHealth.CapturedAtUnix > 0 {
		return edgeHealth.CapturedAtUnix - (edgeHealth.CapturedAtUnix % observabilityBufferWindowSeconds)
	}
	if hostMetrics == nil || hostMetrics.CapturedAtUnix <= 0 {
		return 0
	}
	return hostMetrics.CapturedAtUnix - (hostMetrics.CapturedAtUnix % observabilityBufferWindowSeconds)
}

func pruneObservabilityBufferRecords(records []ObservabilityBufferRecord, retainAfterUnix int64) []ObservabilityBufferRecord {
	if len(records) == 0 {
		return []ObservabilityBufferRecord{}
	}
	filtered := make([]ObservabilityBufferRecord, 0, len(records))
	for _, record := range records {
		if record.WindowStartedAtUnix <= 0 {
			continue
		}
		if retainAfterUnix > 0 && record.WindowStartedAtUnix < retainAfterUnix {
			continue
		}
		filtered = append(filtered, record)
	}
	sort.Slice(filtered, func(i int, j int) bool {
		return filtered[i].WindowStartedAtUnix < filtered[j].WindowStartedAtUnix
	})
	return filtered
}
