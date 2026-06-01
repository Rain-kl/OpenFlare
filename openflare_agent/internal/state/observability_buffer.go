package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"openflare-agent/internal/protocol"
)

const observabilityBufferWindowSeconds = 60

type ObservabilityBufferRecord struct {
	WindowStartedAtUnix  int64                              `json:"window_started_at_unix"`
	Snapshot             *protocol.NodeMetricSnapshot       `json:"snapshot,omitempty"`
	OpenrestyObservation *protocol.NodeOpenrestyObservation `json:"openresty_observation,omitempty"`
	TrafficReport        *protocol.NodeTrafficReport        `json:"traffic_report,omitempty"`
	AccessLogs           []protocol.NodeAccessLog           `json:"access_logs,omitempty"`
	QueuedAtUnix         int64                              `json:"queued_at_unix"`
}

type ObservabilityBufferStore struct {
	path string
	mu   sync.Mutex
}

func NewObservabilityBufferStore(path string) *ObservabilityBufferStore {
	return &ObservabilityBufferStore{path: filepath.Clean(path)}
}

func (s *ObservabilityBufferStore) Upsert(record ObservabilityBufferRecord, retainAfterUnix int64) error {
	if s == nil || record.WindowStartedAtUnix <= 0 || (record.Snapshot == nil && record.OpenrestyObservation == nil && record.TrafficReport == nil && len(record.AccessLogs) == 0) {
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
	if incoming.Snapshot != nil {
		merged.Snapshot = incoming.Snapshot
	}
	if incoming.OpenrestyObservation != nil {
		merged.OpenrestyObservation = incoming.OpenrestyObservation
	}
	if incoming.TrafficReport != nil {
		merged.TrafficReport = incoming.TrafficReport
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
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []ObservabilityBufferRecord{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return []ObservabilityBufferRecord{}, nil
	}
	var records []ObservabilityBufferRecord
	if err = json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *ObservabilityBufferStore) saveUnlocked(records []ObservabilityBufferRecord) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func ObservabilityWindowStartedAt(snapshot *protocol.NodeMetricSnapshot, openresty *protocol.NodeOpenrestyObservation, traffic *protocol.NodeTrafficReport) int64 {
	if traffic != nil && traffic.WindowStartedAtUnix > 0 {
		return traffic.WindowStartedAtUnix - (traffic.WindowStartedAtUnix % observabilityBufferWindowSeconds)
	}
	if openresty != nil && openresty.CapturedAtUnix > 0 {
		return openresty.CapturedAtUnix - (openresty.CapturedAtUnix % observabilityBufferWindowSeconds)
	}
	if snapshot == nil || snapshot.CapturedAtUnix <= 0 {
		return 0
	}
	return snapshot.CapturedAtUnix - (snapshot.CapturedAtUnix % observabilityBufferWindowSeconds)
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
