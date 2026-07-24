//go:build live_ch

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package chwriter_test

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/chwriter"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// Run with Docker ClickHouse + config.yaml:
//
//	go test -tags live_ch ./internal/apps/openflare/chwriter -run TestLiveAppWritePath -count=1 -timeout 2m
func TestLiveAppWritePath(t *testing.T) {
	if !db.ChConnReady() {
		t.Skip("ClickHouse connection not ready")
	}
	ctx := context.Background()
	chwriter.Init(ctx)

	now := time.Now().UTC()
	nodeID := "e2e-app-write-" + now.Format("150405")
	if err := model.InsertOpenFlareMetricSnapshot(ctx, &model.OpenFlareMetricSnapshot{
		NodeID:            nodeID,
		CapturedAt:        now,
		CPUUsagePercent:   33.3,
		MemoryUsedBytes:   111,
		MemoryTotalBytes:  1000,
		StorageUsedBytes:  222,
		StorageTotalBytes: 2000,
		DiskReadBytes:     10,
		DiskWriteBytes:    20,
		NetworkRxBytes:    30,
		NetworkTxBytes:    40,
	}); err != nil {
		t.Fatalf("InsertOpenFlareMetricSnapshot: %v", err)
	}

	deadline := time.Now().Add(45 * time.Second)
	var found bool
	for time.Now().Before(deadline) {
		rows, err := model.ListOpenFlareMetricSnapshotsSince(ctx, nodeID, now.Add(-time.Minute), 10)
		if err != nil {
			t.Fatalf("ListOpenFlareMetricSnapshotsSince: %v", err)
		}
		if len(rows) > 0 {
			found = true
			t.Logf("found snapshot id=%d cpu=%.1f after flush", rows[0].ID, rows[0].CPUUsagePercent)
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !found {
		t.Fatal("metric snapshot not visible in ClickHouse after flush wait")
	}

	latest, err := model.ListOpenFlareLatestMetricSnapshotsSince(ctx, "", now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("ListOpenFlareLatestMetricSnapshotsSince: %v", err)
	}
	var latestOK bool
	for _, row := range latest {
		if row != nil && row.NodeID == nodeID {
			latestOK = true
			break
		}
	}
	if !latestOK {
		t.Fatalf("latest-per-node query missing node %s (rows=%d)", nodeID, len(latest))
	}

	stats := chwriter.WriterStats()
	if len(stats) == 0 {
		t.Fatal("WriterStats empty after Init")
	}
	for _, s := range stats {
		t.Logf("writer %s running=%v depth=%d drops=%d flush_err=%d", s.Name, s.Running, s.Depth, s.Drops, s.FlushErrors)
	}
}
