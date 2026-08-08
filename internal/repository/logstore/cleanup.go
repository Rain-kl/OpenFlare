// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
)

// CleanupSummary 汇总本次清理结果。
type CleanupSummary struct {
	ActiveDatabase string `json:"active_database"`
	RetentionDays  int    `json:"retention_days"`
	Deleted        int64  `json:"deleted"`
	// Tables 记录本次清理的物理表简写名（去掉 of_ 前缀，如 node_access_logs 对应
	// of_node_access_logs；CH 侧物理表名相同，简写仅便于状态展示）。
	Tables []string `json:"tables"`
}

// defaultLogRetentionDays 默认日志保留天数（配置缺失/非法时回退）。
const defaultLogRetentionDays = 90

// partitionLeadMonths 清理时确保「当前月 + 未来 2 个月」分区持续存在。
const partitionLeadMonths = 2

// retentionDaysForActive 按当前激活库读取保留天数（默认 90）。
func retentionDaysForActive(ctx context.Context) int {
	key := model.ConfigKeyLogRetentionDaysPostgres
	if dbName, _ := resolveDatabase(ctx); dbName == dbNameSQLite {
		key = model.ConfigKeyLogRetentionDaysSQLite
	} else if dbName == dbNameClickHouse {
		key = model.ConfigKeyLogRetentionDaysClickHouse
	}
	v, err := getConfig(ctx, key)
	if err != nil {
		return defaultLogRetentionDays
	}
	days, perr := strconv.Atoi(v)
	if perr != nil || days <= 0 {
		return defaultLogRetentionDays
	}
	return days
}

// CleanupExpired 按当前激活库保留天数清理过期日志（每日由 system_cleanup 调用）。
func CleanupExpired(ctx context.Context) (*CleanupSummary, error) {
	s, err := Active(ctx)
	if err != nil {
		return nil, err
	}
	days := retentionDaysForActive(ctx)
	cutoff := time.Now().AddDate(0, 0, -days)
	summary := &CleanupSummary{RetentionDays: days, Tables: []string{}}
	summary.ActiveDatabase, _ = resolveDatabase(ctx)

	// PG 分区表仅在迁移时预建「当前+2 月」分区，此处确保分区持续存在，
	// 否则跨月后新写入会报 "no partition of relation found"（SQLite/CH 为 no-op）。
	now := time.Now().UTC()
	if err := s.AccessLogs.EnsurePartitions(ctx, now, now.AddDate(0, partitionLeadMonths, 0)); err != nil {
		return nil, fmt.Errorf("ensure partitions: %w", err)
	}

	if err := cleanupTable("node_access_logs", func() (int64, error) {
		return s.AccessLogs.DeleteBefore(ctx, cutoff)
	}, summary); err != nil {
		return nil, err
	}
	if err := cleanupTable("metric_snapshots", func() (int64, error) {
		return s.Observability.DeleteMetricSnapshotsBefore(ctx, cutoff)
	}, summary); err != nil {
		return nil, err
	}
	if err := cleanupTable("edge_health", func() (int64, error) {
		return s.Observability.DeleteEdgeHealthBefore(ctx, cutoff)
	}, summary); err != nil {
		return nil, err
	}
	if err := cleanupTable("obs_frps", func() (int64, error) {
		return s.Observability.DeleteNodeObservationFrpsBefore(ctx, cutoff)
	}, summary); err != nil {
		return nil, err
	}
	if err := cleanupTable("obs_frpc", func() (int64, error) {
		return s.Observability.DeleteNodeObservationFrpcBefore(ctx, cutoff)
	}, summary); err != nil {
		return nil, err
	}
	return summary, nil
}

func cleanupTable(name string, fn func() (int64, error), summary *CleanupSummary) error {
	n, err := fn()
	if err != nil {
		return fmt.Errorf("cleanup %s: %w", name, err)
	}
	summary.Deleted += n
	summary.Tables = append(summary.Tables, name)
	return nil
}

// partitionStatementsRange 生成覆盖 [from, to] 全部月份的两表分区 DDL，
// 幂等 CREATE TABLE IF NOT EXISTS ... PARTITION OF ... FOR VALUES FROM ... TO ...。
// 入参为任意时间点：按各自所在月份生成，含 from 月与 to 月（to 常用 max+1 月兜底）。
func partitionStatementsRange(from, to time.Time) []string {
	var out []string
	start := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	for ; start.Before(end); start = start.AddDate(0, 1, 0) {
		monthEnd := start.AddDate(0, 1, 0)
		suffix := start.Format("200601")
		fromDay := start.Format("2006-01-02")
		toDay := monthEnd.Format("2006-01-02")
		for _, table := range []string{"of_node_access_logs", "w_user_access_logs"} {
			out = append(out, fmt.Sprintf(
				"CREATE TABLE IF NOT EXISTS %s_%s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')",
				table, suffix, table, fromDay, toDay))
		}
	}
	return out
}
