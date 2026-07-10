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

func main() {
	if !db.ChConnReady() {
		fmt.Fprintln(os.Stderr, "ChConn not ready — check config.yaml clickhouse.enabled")
		os.Exit(1)
	}
	ctx := context.Background()
	chwriter.Init(ctx)
	now := time.Now().UTC()
	nodeID := "e2e-app-" + now.Format("150405")
	if err := model.InsertOpenFlareMetricSnapshot(ctx, &model.OpenFlareMetricSnapshot{
		NodeID: nodeID, CapturedAt: now, CPUUsagePercent: 41.2,
		MemoryUsedBytes: 123, MemoryTotalBytes: 1000,
		StorageUsedBytes: 456, StorageTotalBytes: 2000,
		DiskReadBytes: 11, DiskWriteBytes: 22, NetworkRxBytes: 33, NetworkTxBytes: 44,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "insert:", err)
		os.Exit(1)
	}
	fmt.Println("queued", nodeID)
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := model.ListOpenFlareMetricSnapshotsSince(ctx, nodeID, now.Add(-time.Minute), 5)
		if err != nil {
			fmt.Fprintln(os.Stderr, "list:", err)
			os.Exit(1)
		}
		if len(rows) > 0 {
			fmt.Printf("OK flushed id=%d cpu=%.1f\n", rows[0].ID, rows[0].CPUUsagePercent)
			latest, err := model.ListOpenFlareLatestMetricSnapshotsSince(ctx, "", now.Add(-time.Hour))
			if err != nil {
				fmt.Fprintln(os.Stderr, "latest:", err)
				os.Exit(1)
			}
			ok := false
			for _, r := range latest {
				if r != nil && r.NodeID == nodeID {
					ok = true
				}
			}
			if !ok {
				fmt.Fprintln(os.Stderr, "FAIL latest-per-node missing node")
				os.Exit(1)
			}
			fmt.Println("OK latest-per-node includes node")
			for _, s := range chwriter.WriterStats() {
				fmt.Printf("writer %s running=%v depth=%d drops=%d flush_err=%d\n",
					s.Name, s.Running, s.Depth, s.Drops, s.FlushErrors)
			}
			_ = chwriter.Stop(ctx)
			return
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Fprintln(os.Stderr, "FAIL: not flushed within 45s")
	os.Exit(1)
}
