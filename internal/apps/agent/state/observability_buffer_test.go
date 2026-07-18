package state

import (
	"path/filepath"
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
