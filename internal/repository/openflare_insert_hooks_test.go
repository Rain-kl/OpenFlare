// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// Hook setters are process-global; keep these tests serial.

func TestObservabilityInsertHooksAreInvoked(t *testing.T) {
	var gotSnapshot analyticsmodel.NodeMetricSnapshot
	SetObservabilityInsertHooks(ObservabilityInsertHooks{
		QueueMetricSnapshot: func(s analyticsmodel.NodeMetricSnapshot) {
			gotSnapshot = s
		},
	})
	t.Cleanup(func() {
		SetObservabilityInsertHooks(ObservabilityInsertHooks{})
	})

	record := &model.OpenFlareMetricSnapshot{
		NodeID:     "node-1",
		CapturedAt: time.Unix(100, 0).UTC(),
	}
	if err := (clickhouseObservabilityStore{}).InsertMetricSnapshot(context.Background(), record); err != nil {
		t.Fatalf("InsertMetricSnapshot error = %v", err)
	}
	if gotSnapshot.NodeID != "node-1" {
		t.Fatalf("hook node id = %q, want node-1", gotSnapshot.NodeID)
	}
}

func TestAccessLogInsertHooksAreInvoked(t *testing.T) {
	var got []analyticsmodel.NodeAccessLog
	SetAccessLogInsertHooks(AccessLogInsertHooks{
		QueueNodeAccessLogs: func(logs []analyticsmodel.NodeAccessLog) {
			got = append([]analyticsmodel.NodeAccessLog(nil), logs...)
		},
	})
	t.Cleanup(func() {
		SetAccessLogInsertHooks(AccessLogInsertHooks{})
	})

	records := []*model.OpenFlareAccessLog{
		{NodeID: "n1", Path: "/a"},
		{NodeID: "n1", Path: "/b"},
	}
	if err := (clickhouseAccessLogStore{}).InsertBatch(context.Background(), records); err != nil {
		t.Fatalf("InsertBatch error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("hook logs = %d, want 2", len(got))
	}
	if got[0].Path != "/a" || got[1].Path != "/b" {
		t.Fatalf("hook paths = %q/%q, want /a /b", got[0].Path, got[1].Path)
	}
}

func TestInsertHooksNoopWhenUnset(t *testing.T) {
	SetObservabilityInsertHooks(ObservabilityInsertHooks{})
	SetAccessLogInsertHooks(AccessLogInsertHooks{})

	if err := (clickhouseObservabilityStore{}).InsertMetricSnapshot(context.Background(), &model.OpenFlareMetricSnapshot{NodeID: "x"}); err != nil {
		t.Fatalf("InsertMetricSnapshot with nil hook error = %v", err)
	}
	if err := (clickhouseAccessLogStore{}).InsertBatch(context.Background(), []*model.OpenFlareAccessLog{{NodeID: "x"}}); err != nil {
		t.Fatalf("InsertBatch with nil hook error = %v", err)
	}
}
