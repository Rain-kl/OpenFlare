// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// TestGormAccessLogInsertBatchHooks 覆盖访问日志写入入口：
// 冻结检查、hook 入队、不直接落库、flush 后可见（行为与旧 repository clickhouse 包装一致）。
func TestGormAccessLogInsertBatchHooks(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		return "", nil
	})
	defer ResetForTest()

	s := newTestGormStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	var hooked []analyticsmodel.NodeAccessLog
	SetAccessLogHooks(AccessLogHooks{
		QueueNodeAccessLogs: func(logs []analyticsmodel.NodeAccessLog) {
			hooked = append(hooked, logs...)
		},
	})
	defer SetAccessLogHooks(AccessLogHooks{})

	records := []*model.OpenFlareAccessLog{
		{NodeID: "n1", LoggedAt: now, RemoteAddr: "1.1.1.1", StatusCode: 200, BytesSent: 100},
		{NodeID: "n1", LoggedAt: now, RemoteAddr: "2.2.2.2", StatusCode: 404},
	}
	if err := s.InsertBatch(ctx, records); err != nil {
		t.Fatalf("insert batch: %v", err)
	}
	if len(hooked) != 2 || hooked[0].RemoteAddr != "1.1.1.1" || hooked[0].BytesSent != 100 || hooked[1].StatusCode != 404 {
		t.Fatalf("hook rows mismatch: %+v", hooked)
	}
	// 写入入口只入队、不直接落库。
	rows, err := s.List(ctx, model.OpenFlareAccessLogQuery{NodeID: "n1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("entry insert must not write rows, got %d", len(rows))
	}
	// flush 后可见。
	if err := s.BatchInsertNodeAccessLogs(ctx, hooked); err != nil {
		t.Fatalf("flush: %v", err)
	}
	rows, err = s.List(ctx, model.OpenFlareAccessLogQuery{NodeID: "n1"})
	if err != nil {
		t.Fatalf("list after flush: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("list after flush want 2, got %d", len(rows))
	}
}
