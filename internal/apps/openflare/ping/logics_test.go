// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ping

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPingTestDB(t *testing.T) context.Context {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "ping.db")
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(
		&model.SystemConfig{},
		&model.OpenFlareNode{},
		&model.OpenFlarePingTask{},
		&analyticsmodel.NodePingRecord{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	db.SetDB(conn)
	logstore.ResetForTest()
	logstore.SetConfigReader(func(_ context.Context, _ string) (string, error) {
		return "sqlite", nil
	})
	logstore.SetObservabilityHooks(logstore.ObservabilityHooks{
		QueuePingRecord: func(row analyticsmodel.NodePingRecord) {
			s, err := logstore.Active(context.Background())
			if err != nil {
				t.Errorf("logstore.Active: %v", err)
				return
			}
			if err := s.Observability.BatchInsertNodePingRecords(context.Background(), []analyticsmodel.NodePingRecord{row}); err != nil {
				t.Errorf("BatchInsertNodePingRecords: %v", err)
			}
		},
	})
	t.Cleanup(func() {
		logstore.SetObservabilityHooks(logstore.ObservabilityHooks{})
		logstore.ResetForTest()
		db.SetDB(nil)
	})
	return context.Background()
}

func TestPingTaskCRUDReorderAndStats(t *testing.T) {
	ctx := setupPingTestDB(t)

	node := &model.OpenFlareNode{
		NodeID:      "node-alpha",
		Name:        "Alpha",
		AccessToken: "tok-alpha",
		NodeType:    "edge_node",
	}
	if err := db.DB(ctx).Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	task1, err := CreateTask(ctx, TaskInput{
		Name:      "Cloudflare DNS",
		Clients:   []string{"node-alpha"},
		DefaultOn: true,
		Type:      "icmp",
		Target:    "1.1.1.1",
		Interval:  60,
	})
	if err != nil {
		t.Fatalf("CreateTask 1: %v", err)
	}
	task2, err := CreateTask(ctx, TaskInput{
		Name:      "Google HTTP",
		Clients:   []string{"node-alpha"},
		DefaultOn: false,
		Type:      "http",
		Target:    "https://www.google.com/generate_204",
		Interval:  30,
	})
	if err != nil {
		t.Fatalf("CreateTask 2: %v", err)
	}

	// Reorder so task2 precedes task1.
	if err := ReorderTasks(ctx, map[uint]int{task2.ID: 0, task1.ID: 1}); err != nil {
		t.Fatalf("ReorderTasks: %v", err)
	}
	listed, err := ListTasks(ctx)
	if err != nil || len(listed) != 2 || listed[0].ID != task2.ID {
		t.Fatalf("expected task2 first after reorder, got %+v err=%v", listed, err)
	}

	// Attach new node should bind only default_on tasks.
	AttachNewNode(ctx, "node-beta")
	t1Reload, err := GetTask(ctx, task1.ID)
	if err != nil {
		t.Fatalf("GetTask 1: %v", err)
	}
	t2Reload, err := GetTask(ctx, task2.ID)
	if err != nil {
		t.Fatalf("GetTask 2: %v", err)
	}
	if len(t1Reload.Clients) != 2 || len(t2Reload.Clients) != 1 {
		t.Fatalf("unexpected default_on clients: t1=%v t2=%v", t1Reload.Clients, t2Reload.Clients)
	}

	// Record ping samples (3 valid + 1 loss) and verify unsampled stats.
	base := time.Now().UTC().Add(-10 * time.Minute)
	samples := []int{10, 20, -1, 30}
	for i, val := range samples {
		err := RecordResultWithError(ctx, "node-alpha", protocol.PingResultPayload{
			TaskID:     uint64(task1.ID),
			PingType:   "icmp",
			Value:      val,
			FinishedAt: base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("RecordResultWithError[%d]: %v", i, err)
		}
	}

	view, err := GetNodePingRecords(ctx, node.ID, NodePingRecordsQuery{
		TaskID: task1.ID,
		Hours:  1,
	})
	if err != nil {
		t.Fatalf("GetNodePingRecords: %v", err)
	}
	if len(view.Tasks) != 1 || len(view.Records) != 4 {
		t.Fatalf("unexpected view: tasks=%d records=%d", len(view.Tasks), len(view.Records))
	}
	stats := view.Tasks[0]
	if stats.Min != 10 || stats.Max != 30 || stats.Avg != 20 || stats.Latest != 30 || stats.Loss != 25 || stats.Total != 4 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	// Detach deleted node.
	DetachDeletedNode(ctx, "node-beta")
	t1AfterDetach, _ := GetTask(ctx, task1.ID)
	if len(t1AfterDetach.Clients) != 1 {
		t.Fatalf("expected node-beta detached, got %v", t1AfterDetach.Clients)
	}

	// Delete tasks purges both task and ping records.
	deleted, err := DeleteTasks(ctx, []uint{task1.ID})
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteTasks: deleted=%d err=%v", deleted, err)
	}
}
