// Package main is a manual smoke tool for ClickHouse app write path.
// Usage (from repo root, with config.yaml and Docker CH up):
//
//	go run ./scripts/live_ch_smoke
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/chwriter"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	flushWaitTimeout = 45 * time.Second
	listLimit        = 5
	pollInterval     = 2 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if !db.ChConnReady() {
		return fmt.Errorf("ChConn not ready — check config.yaml clickhouse.enabled")
	}
	ctx := context.Background()
	chwriter.Init(ctx)
	defer func() { _ = chwriter.Stop(ctx) }()

	now := time.Now().UTC()
	nodeID := "e2e-app-" + now.Format("150405")
	if err := model.InsertOpenFlareMetricSnapshot(ctx, &model.OpenFlareMetricSnapshot{
		NodeID: nodeID, CapturedAt: now, CPUUsagePercent: 41.2,
		MemoryUsedBytes: 123, MemoryTotalBytes: 1000,
		StorageUsedBytes: 456, StorageTotalBytes: 2000,
		DiskReadBytes: 11, DiskWriteBytes: 22, NetworkRxBytes: 33, NetworkTxBytes: 44,
	}); err != nil {
		return fmt.Errorf("insert: %w", err)
	}
	fmt.Println("queued", nodeID)

	if err := waitForSnapshot(ctx, nodeID, now); err != nil {
		return err
	}
	if err := assertLatestIncludes(ctx, nodeID, now); err != nil {
		return err
	}
	for _, s := range chwriter.WriterStats() {
		fmt.Printf("writer %s running=%v depth=%d drops=%d flush_err=%d\n",
			s.Name, s.Running, s.Depth, s.Drops, s.FlushErrors)
	}
	return nil
}

func waitForSnapshot(ctx context.Context, nodeID string, now time.Time) error {
	deadline := time.Now().Add(flushWaitTimeout)
	for time.Now().Before(deadline) {
		rows, err := model.ListOpenFlareMetricSnapshotsSince(ctx, nodeID, now.Add(-time.Minute), listLimit)
		if err != nil {
			return fmt.Errorf("list: %w", err)
		}
		if len(rows) > 0 {
			fmt.Printf("OK flushed id=%d cpu=%.1f\n", rows[0].ID, rows[0].CPUUsagePercent)
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("not flushed within timeout")
}

func assertLatestIncludes(ctx context.Context, nodeID string, now time.Time) error {
	latest, err := model.ListOpenFlareLatestMetricSnapshotsSince(ctx, "", now.Add(-time.Hour))
	if err != nil {
		return fmt.Errorf("latest: %w", err)
	}
	for _, r := range latest {
		if r != nil && r.NodeID == nodeID {
			fmt.Println("OK latest-per-node includes node")
			return nil
		}
	}
	return fmt.Errorf("latest-per-node missing node %s", nodeID)
}
