// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

// CleanupSummary 汇总本次清理结果。
type CleanupSummary struct {
	ActiveDatabase string `json:"active_database"`
	// RetentionDays 访问日志（节点访问/用户访问）保留天数，按日志库读取。
	RetentionDays int `json:"retention_days"`
	// MetricRetentionDays 性能指标（CPU/内存/磁盘/网络）保留天数，三库共用短留存。
	MetricRetentionDays int   `json:"metric_retention_days"`
	Deleted             int64 `json:"deleted"`
	// Tables 记录本次清理的物理表简写名（去掉 of_ 前缀，如 node_access_logs 对应
	// of_node_access_logs；CH 侧物理表名相同，简写仅便于状态展示）。
	Tables []string `json:"tables"`
}

// defaultLogRetentionDays 默认日志保留天数（配置缺失/非法时回退）。
const defaultLogRetentionDays = 90

// defaultMetricRetentionDays 默认性能指标保留天数（配置缺失/非法时回退）。
// 性能数据价值衰减快，默认短留存（3 天）。
const defaultMetricRetentionDays = 3

// partitionLeadMonths 清理时确保「当前月 + 未来 2 个月」分区持续存在。
const partitionLeadMonths = 2

// accessLogPartitionTables 按月分区的访问日志表（分区预建/空分区清理共用）。
var accessLogPartitionTables = []string{"of_node_access_logs", "w_user_access_logs"}

// retentionDaysForDatabase 按给定日志库读取保留天数（默认 90）。
func retentionDaysForDatabase(ctx context.Context, dbName string) int {
	key := model.ConfigKeyLogRetentionDaysPostgres
	switch dbName {
	case dbNameSQLite:
		key = model.ConfigKeyLogRetentionDaysSQLite
	case dbNameClickHouse:
		key = model.ConfigKeyLogRetentionDaysClickHouse
	}
	v, err := getConfig(ctx, key)
	if err != nil {
		if !errors.Is(err, errConfigReaderNotWired) {
			logger.ErrorF(ctx, "读取日志保留天数配置失败(key=%s)，回退默认 %d 天: %v", key, defaultLogRetentionDays, err)
		}
		return defaultLogRetentionDays
	}
	days, perr := strconv.Atoi(v)
	if perr != nil || days <= 0 {
		logger.ErrorF(ctx, "日志保留天数配置非法(key=%s, value=%q)，回退默认 %d 天", key, v, defaultLogRetentionDays)
		return defaultLogRetentionDays
	}
	return days
}

// metricRetentionDays 读取性能指标保留天数（三库共用，默认 3 天）。
func metricRetentionDays(ctx context.Context) int {
	v, err := getConfig(ctx, model.ConfigKeyMetricRetentionDays)
	if err != nil {
		if !errors.Is(err, errConfigReaderNotWired) {
			logger.ErrorF(ctx, "读取性能指标保留天数配置失败(key=%s)，回退默认 %d 天: %v", model.ConfigKeyMetricRetentionDays, defaultMetricRetentionDays, err)
		}
		return defaultMetricRetentionDays
	}
	days, perr := strconv.Atoi(v)
	if perr != nil || days <= 0 {
		logger.ErrorF(ctx, "性能指标保留天数配置非法(key=%s, value=%q)，回退默认 %d 天", model.ConfigKeyMetricRetentionDays, v, defaultMetricRetentionDays)
		return defaultMetricRetentionDays
	}
	return days
}

// CleanupExpired 按当前激活库保留天数清理过期日志（每日由 system_cleanup 调用）：
// 访问日志（节点访问/用户访问）按 log_retention_days_* 清理；
// 性能指标（CPU/内存/磁盘/网络）按三库共用的短留存 metric_retention_days 清理。
func CleanupExpired(ctx context.Context) (*CleanupSummary, error) {
	dbName, err := resolveDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve active database: %w", err)
	}
	s, err := Active(ctx)
	if err != nil {
		return nil, err
	}
	days := retentionDaysForDatabase(ctx, dbName)
	metricDays := metricRetentionDays(ctx)
	cutoff := time.Now().AddDate(0, 0, -days)
	metricCutoff := time.Now().AddDate(0, 0, -metricDays)
	summary := &CleanupSummary{ActiveDatabase: dbName, RetentionDays: days, MetricRetentionDays: metricDays, Tables: []string{}}

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

	// 过期数据删除后清理旧月份空分区表，避免分区表无限累积；
	// 仅删「当前月之前」且无数据的分区（best-effort，失败不阻断数据保留清理）。
	if err := s.AccessLogs.DropEmptyPartitions(ctx, now); err != nil {
		logger.WarnF(ctx, "drop empty log partitions failed: %v", err)
	}

	if err := cleanupTable("metric_snapshots", func() (int64, error) {
		return s.Observability.DeleteMetricSnapshotsBefore(ctx, metricCutoff)
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
		for _, table := range accessLogPartitionTables {
			out = append(out, fmt.Sprintf(
				"CREATE TABLE IF NOT EXISTS %s_%s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')",
				table, suffix, table, fromDay, toDay))
		}
	}
	return out
}

// partitionNameMonth 解析按月分区表名 <table>_YYYYMM 的所属月份；命名不匹配返回 (零值, false)。
func partitionNameMonth(table, name string) (time.Time, bool) {
	suffix, ok := strings.CutPrefix(name, table+"_")
	if !ok || len(suffix) != 6 {
		return time.Time{}, false
	}
	m, err := time.Parse("200601", suffix)
	if err != nil {
		return time.Time{}, false
	}
	return m, true
}

// dropEligiblePartitionNames 返回 before 月份之前、命名合法的分区表名（是否为空由调用方校验）。
func dropEligiblePartitionNames(table string, names []string, before time.Time) []string {
	beforeMonth := time.Date(before.Year(), before.Month(), 1, 0, 0, 0, 0, time.UTC)
	out := make([]string, 0, len(names))
	for _, name := range names {
		month, ok := partitionNameMonth(table, name)
		if !ok || !month.Before(beforeMonth) {
			continue // 非法命名或当月/未来月分区，必须保留
		}
		out = append(out, name)
	}
	return out
}
