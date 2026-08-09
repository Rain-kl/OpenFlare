// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"strings"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
)

// TestClickHouseHourlyDelegationRegression 验证 CH 后端小时级聚合读委托 analyticsrepo：
// 未初始化 CH 连接时返回 analyticsrepo 的 "clickhouse connection is not initialized" 错误
// （而非未实现/panic），证明 3 个方法都路由到 CH 原生查询。
func TestClickHouseHourlyDelegationRegression(t *testing.T) {
	if db.ChConnReady() {
		t.Skip("clickhouse connection initialized; skipping delegation regression")
	}
	s := newClickHouseStore()
	ctx := context.Background()
	now := time.Now()
	check := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: want clickhouse-not-initialized error, got nil", name)
		}
		if !strings.Contains(err.Error(), "clickhouse connection is not initialized") {
			t.Fatalf("%s: unexpected error %v", name, err)
		}
	}
	_, err := s.ListTrafficHourly(ctx, "n1", now)
	check("ListTrafficHourly", err)
	_, err = s.ListAccessLogHourly(ctx, "n1", now)
	check("ListAccessLogHourly", err)
	_, err = s.ListMetricHourly(ctx, "n1", now)
	check("ListMetricHourly", err)
}
