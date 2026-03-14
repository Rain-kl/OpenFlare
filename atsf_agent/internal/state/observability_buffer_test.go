package state

import (
	"path/filepath"
	"testing"

	"atsflare-agent/internal/protocol"
)

func TestObservabilityBufferStoreUpsertReplayAndAck(t *testing.T) {
	store := NewObservabilityBufferStore(filepath.Join(t.TempDir(), "observability-buffer.json"))

	if err := store.Upsert(ObservabilityBufferRecord{
		WindowStartedAtUnix: 1710403200,
		Snapshot:            &protocol.NodeMetricSnapshot{CapturedAtUnix: 1710403205},
		TrafficReport:       &protocol.NodeTrafficReport{WindowStartedAtUnix: 1710403200, WindowEndedAtUnix: 1710403260, RequestCount: 5},
		QueuedAtUnix:        1710403205,
	}, 1710403000); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if err := store.Upsert(ObservabilityBufferRecord{
		WindowStartedAtUnix: 1710403200,
		Snapshot:            &protocol.NodeMetricSnapshot{CapturedAtUnix: 1710403255},
		TrafficReport:       &protocol.NodeTrafficReport{WindowStartedAtUnix: 1710403200, WindowEndedAtUnix: 1710403260, RequestCount: 12},
		QueuedAtUnix:        1710403255,
	}, 1710403000); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	if err := store.Upsert(ObservabilityBufferRecord{
		WindowStartedAtUnix: 1710403260,
		Snapshot:            &protocol.NodeMetricSnapshot{CapturedAtUnix: 1710403265},
		TrafficReport:       &protocol.NodeTrafficReport{WindowStartedAtUnix: 1710403260, WindowEndedAtUnix: 1710403320, RequestCount: 2},
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
	if records[0].TrafficReport == nil || records[0].TrafficReport.RequestCount != 12 {
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

func TestObservabilityWindowStartedAt(t *testing.T) {
	if value := ObservabilityWindowStartedAt(nil, &protocol.NodeTrafficReport{WindowStartedAtUnix: 1710403200}); value != 1710403200 {
		t.Fatalf("unexpected traffic window start: %d", value)
	}
	if value := ObservabilityWindowStartedAt(&protocol.NodeMetricSnapshot{CapturedAtUnix: 1710403259}, nil); value != 1710403200 {
		t.Fatalf("unexpected snapshot-derived window start: %d", value)
	}
}
