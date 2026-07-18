package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
)

func TestObservabilityBufferStoreUpsertReplayAndAck(t *testing.T) {
	store := NewObservabilityBufferStore(filepath.Join(t.TempDir(), "observability-buffer.json"))

	if err := store.Upsert(ObservabilityBufferRecord{
		WindowStartedAtUnix: 1710403200,
		HostMetrics:         &protocol.NodeMetricSnapshot{CapturedAtUnix: 1710403205},
		EdgeHealth:          &protocol.NodeEdgeHealth{CapturedAtUnix: 1710403205, Connections: 5},
		QueuedAtUnix:        1710403205,
	}, 1710403000); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if err := store.Upsert(ObservabilityBufferRecord{
		WindowStartedAtUnix: 1710403200,
		HostMetrics:         &protocol.NodeMetricSnapshot{CapturedAtUnix: 1710403255, CPUUsagePercent: 40},
		EdgeHealth:          &protocol.NodeEdgeHealth{CapturedAtUnix: 1710403255, Connections: 12},
		QueuedAtUnix:        1710403255,
	}, 1710403000); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	if err := store.Upsert(ObservabilityBufferRecord{
		WindowStartedAtUnix: 1710403260,
		HostMetrics:         &protocol.NodeMetricSnapshot{CapturedAtUnix: 1710403265},
		QueuedAtUnix:        1710403265,
	}, 1710403000); err != nil {
		t.Fatalf("third upsert failed: %v", err)
	}

	records, err := store.Replayable(1710403260, 1710403000)
	if err != nil {
		t.Fatalf("Replayable failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one replayable record before current window, got %d", len(records))
	}
	if records[0].EdgeHealth == nil || records[0].EdgeHealth.Connections != 12 {
		t.Fatalf("expected replayable record to keep latest upsert, got %+v", records[0])
	}

	if err = store.Ack([]int64{1710403200}, 1710403000); err != nil {
		t.Fatalf("Ack failed: %v", err)
	}
	records, err = store.Replayable(0, 1710403000)
	if err != nil {
		t.Fatalf("Replayable after ack failed: %v", err)
	}
	if len(records) != 1 || records[0].WindowStartedAtUnix != 1710403260 {
		t.Fatalf("unexpected records after ack: %+v", records)
	}
}

func TestObservabilityBufferStoreMergesAccessLogsWithinWindow(t *testing.T) {
	store := NewObservabilityBufferStore(filepath.Join(t.TempDir(), "observability-buffer.json"))

	if err := store.Upsert(ObservabilityBufferRecord{
		WindowStartedAtUnix: 1710403200,
		AccessLogs: []protocol.NodeAccessLog{
			{LoggedAtUnix: 1710403201, RemoteAddr: "10.0.0.1", Host: "app.example.com", Path: "/a", StatusCode: 200},
		},
	}, 1710403000); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if err := store.Upsert(ObservabilityBufferRecord{
		WindowStartedAtUnix: 1710403200,
		AccessLogs: []protocol.NodeAccessLog{
			{LoggedAtUnix: 1710403201, RemoteAddr: "10.0.0.1", Host: "app.example.com", Path: "/a", StatusCode: 200},
			{LoggedAtUnix: 1710403205, RemoteAddr: "10.0.0.2", Host: "app.example.com", Path: "/b", StatusCode: 502},
		},
	}, 1710403000); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	records, err := store.Replayable(0, 1710403000)
	if err != nil {
		t.Fatalf("Replayable failed: %v", err)
	}
	if len(records) != 1 || len(records[0].AccessLogs) != 2 {
		t.Fatalf("expected merged access logs, got %+v", records)
	}
}

func TestObservabilityWindowStartedAt(t *testing.T) {
	if value := ObservabilityWindowStartedAt(nil, &protocol.NodeEdgeHealth{CapturedAtUnix: 1710403259}); value != 1710403200 {
		t.Fatalf("unexpected edge-health window start: %d", value)
	}
	if value := ObservabilityWindowStartedAt(&protocol.NodeMetricSnapshot{CapturedAtUnix: 1710403259}, nil); value != 1710403200 {
		t.Fatalf("unexpected host-metrics window start: %d", value)
	}
}

func TestObservabilityBufferStoreDiscardsLegacyDiskJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observability-buffer.json")
	legacy := `[{
		"window_started_at_unix": 1710403200,
		"snapshot": {"captured_at_unix": 1710403205, "cpu_usage_percent": 11.5},
		"openresty_observation": {"captured_at_unix": 1710403206, "openresty_connections": 7},
		"traffic_report": {"request_count": 42},
		"access_logs": [{"logged_at_unix": 1710403201, "path": "/", "status_code": 200}]
	}]`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	store := NewObservabilityBufferStore(path)
	records, err := store.Replayable(0, 0)
	if err != nil {
		t.Fatalf("Replayable: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected legacy buffer discarded, got %+v", records)
	}
	// Replayable may rewrite an empty v2 array; legacy keys must be gone.
	if body, err := os.ReadFile(path); err == nil {
		raw := string(body)
		for _, key := range []string{`"snapshot"`, `"openresty_observation"`, `"traffic_report"`} {
			if strings.Contains(raw, key) {
				t.Fatalf("legacy key %s still present: %s", key, raw)
			}
		}
	}

	// Fresh upsert after discard should create a clean v2 file.
	if err := store.Upsert(ObservabilityBufferRecord{
		WindowStartedAtUnix: 1710403200,
		HostMetrics:         &protocol.NodeMetricSnapshot{CapturedAtUnix: 1710403205},
	}, 0); err != nil {
		t.Fatalf("Upsert after discard: %v", err)
	}
	records, err = store.Replayable(0, 0)
	if err != nil {
		t.Fatalf("Replayable after rebuild: %v", err)
	}
	if len(records) != 1 || records[0].HostMetrics == nil {
		t.Fatalf("expected rebuilt buffer, got %+v", records)
	}
}

func TestObservabilityBufferStoreDiscardsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observability-buffer.json")
	if err := os.WriteFile(path, []byte(`{not-json`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store := NewObservabilityBufferStore(path)
	records, err := store.Replayable(0, 0)
	if err != nil {
		t.Fatalf("Replayable should not fail: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected empty after discard, got %+v", records)
	}
	// Corrupt payload must not remain; empty rewrite is fine.
	if body, err := os.ReadFile(path); err == nil && strings.Contains(string(body), "not-json") {
		t.Fatalf("corrupt content still on disk: %s", body)
	}
}

func TestObservabilityBufferStoreKeepsModernJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observability-buffer.json")
	modern := `[{
		"window_started_at_unix": 1710403200,
		"host_metrics": {"captured_at_unix": 1710403205, "cpu_usage_percent": 3},
		"edge_health": {"captured_at_unix": 1710403205, "status": "healthy", "connections": 2},
		"access_logs": []
	}]`
	if err := os.WriteFile(path, []byte(modern), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store := NewObservabilityBufferStore(path)
	records, err := store.Replayable(0, 0)
	if err != nil {
		t.Fatalf("Replayable: %v", err)
	}
	if len(records) != 1 || records[0].HostMetrics == nil || records[0].HostMetrics.CPUUsagePercent != 3 {
		t.Fatalf("modern buffer should be kept: %+v", records)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("modern buffer file should remain: %v", err)
	}
}
