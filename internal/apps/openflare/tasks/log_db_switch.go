// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/chwriter"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const copyBatchSize = 1000

// 迁移目标库名常量（normalizeTarget 归一化后的取值）。
const (
	targetPostgres   = "postgres"
	targetSQLite     = "sqlite"
	targetClickHouse = "clickhouse"
)

type logDBSwitchPayload struct {
	Target string `json:"target"`
}

// LogDBSwitchHandler 切换日志数据库任务处理器。
type LogDBSwitchHandler struct{}

// ValidatePayload 校验并规范化参数。
func (h *LogDBSwitchHandler) ValidatePayload(payload []byte) ([]byte, error) {
	var p logDBSwitchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}
	p.Target = normalizeTarget(p.Target)
	if !validTarget(p.Target) {
		return nil, fmt.Errorf("invalid target log database: %s", p.Target)
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeTarget(v string) string {
	switch v {
	case targetPostgres, "postgresql":
		return targetPostgres
	case targetSQLite, "sqlite3":
		return targetSQLite
	case targetClickHouse, "ch":
		return targetClickHouse
	}
	return v
}

func validTarget(v string) bool {
	return v == targetPostgres || v == targetSQLite || v == targetClickHouse
}

// Execute 执行迁移。
func (h *LogDBSwitchHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	var p logDBSwitchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}
	p.Target = normalizeTarget(p.Target)
	if err := validateSwitch(ctx, p.Target); err != nil {
		return nil, err
	}

	source, err := currentLogDatabase(ctx)
	if err != nil {
		task.AppendLog(ctx, "Failed to read log database: %v", err)
		return nil, err
	}
	task.AppendLog(ctx, "Starting log database migration: %s -> %s", source, p.Target)

	// 设置迁移冻结标记（置位后由 ensureWritable 拒绝新写入）。
	if err := setMigrationFlag(ctx, "migrating"); err != nil {
		return nil, err
	}
	// 失败也清除（SaveOrUpdateSystemConfig 会失效 RAM 缓存并广播），保持源库可写。
	defer func() {
		if err := setMigrationFlag(ctx, ""); err != nil {
			logger.ErrorF(ctx, "Failed to clear log migration freeze marker: %v", err)
		}
	}()

	// 冻结标记置位后再排空在途批次（chwriter + 用户访问日志 writer），
	// 保证排空完成后不再有新批次进入源库。
	if err := drainLogWriters(ctx); err != nil {
		return nil, fmt.Errorf("failed to drain log write queue: %w", err)
	}

	src, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	dst, err := buildTargetStore(ctx, p.Target)
	if err != nil {
		return nil, err
	}

	// 清空目标库日志表（幂等重试前提）。
	if err := clearTargetLogTables(ctx, dst); err != nil {
		return nil, err
	}

	// PG 目标：按源库时间范围预建分区，避免历史数据复制报 "no partition of relation found"。
	if err := ensureTargetPartitions(ctx, src, dst, p.Target); err != nil {
		return nil, err
	}

	// 逐表复制（6 张日志表）。
	if err := copyAccessLogs(ctx, src, dst); err != nil {
		return nil, err
	}
	if err := copyUserAccessLogs(ctx, src, dst); err != nil {
		return nil, err
	}
	if err := copyObservability(ctx, src, dst); err != nil {
		return nil, err
	}

	// 翻转主库标记。
	if err := flipLogDatabase(ctx, p.Target); err != nil {
		return nil, err
	}
	task.AppendLog(ctx, "Log database switched to %s; writes resumed", p.Target)
	return &task.TaskResult{Message: fmt.Sprintf("Log database switched from %s to %s", source, p.Target)}, nil
}

func validateSwitch(ctx context.Context, target string) error {
	source, err := currentLogDatabase(ctx)
	if err != nil {
		return err
	}
	if source == target {
		return errors.New("target log database is already active; no migration is required")
	}
	switch target {
	case "clickhouse":
		if !config.Config.ClickHouse.Enabled {
			return errors.New("ClickHouse is disabled; migration to ClickHouse is unavailable")
		}
	case "postgres":
		if !config.Config.Database.Enabled {
			return errors.New("PostgreSQL is disabled and SQLite is the primary database; migration to PostgreSQL is unavailable")
		}
	case "sqlite":
		if config.Config.Database.Enabled {
			return errors.New("PostgreSQL is the primary database; SQLite cannot be selected for log storage")
		}
	}
	return nil
}

func currentLogDatabase(ctx context.Context) (string, error) {
	cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDatabase)
	if err != nil {
		return "", fmt.Errorf("failed to read log database: %w", err)
	}
	if cfg.Value == "" {
		return "", errors.New("log database configuration is empty")
	}
	return cfg.Value, nil
}

// drainLogWriters 等待 chwriter（节点访问日志 + 可观测 4 表）的在途批次全部落库。
// 见设计 §7.2：先排空再冻结。用户访问日志（w_user_access_logs）记录已禁用，无在途批次。
func drainLogWriters(ctx context.Context) error {
	return chwriter.Drain(ctx)
}

// setMigrationFlag 写入迁移冻结标记。用 SaveOrUpdateSystemConfig：行缺失时 upsert，
// 并失效 RAM 缓存 + 广播其他节点，保证 logstore.Migrating/resolveDatabase 立即生效。
func setMigrationFlag(ctx context.Context, v string) error {
	return repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDBMigration, v)
}

// flipLogDatabase 翻转日志主库。同上用 SaveOrUpdateSystemConfig，确保各进程缓存失效后指向新库。
func flipLogDatabase(ctx context.Context, target string) error {
	return repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDatabase, target)
}

// buildTargetStore 构造目标库 Store（不经过 Active 缓存，直接 Build）。
// 迁移期间冻结标记已置位，目标库的清空/复制写入必须放行，故使用 BuildForMigration。
func buildTargetStore(ctx context.Context, database string) (*logstore.Store, error) {
	return logstore.BuildForMigration(ctx, database)
}

func clearTargetLogTables(ctx context.Context, dst *logstore.Store) error {
	// 依次清空 6 张表：AccessLogs.DeleteAll、UserAccessLogs.DeleteAll、Observability.DeleteAll*
	// （SQLite/PG 用 DeleteAll；CH 用 TRUNCATE 语义）。
	if _, err := dst.AccessLogs.DeleteAll(ctx); err != nil {
		return fmt.Errorf("failed to clear target access logs: %w", err)
	}
	if _, err := dst.UserAccessLogs.DeleteAll(ctx); err != nil {
		return fmt.Errorf("failed to clear target user access logs: %w", err)
	}
	for _, fn := range []func(context.Context) (int64, error){
		dst.Observability.DeleteAllMetricSnapshots,
		dst.Observability.DeleteAllEdgeHealth,
		dst.Observability.DeleteAllNodeObservationFrps,
		dst.Observability.DeleteAllNodeObservationFrpc,
	} {
		if _, err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

// ensureTargetPartitions 目标为 PG 时，按源库时间范围（两表合并）预建分区，
// 否则复制历史数据会报 "no partition of relation found"；目标非 PG 为 no-op。
func ensureTargetPartitions(ctx context.Context, src, dst *logstore.Store, target string) error {
	if target != targetPostgres {
		return nil
	}
	from, to, err := migrationRange(ctx, src)
	if err != nil {
		return err
	}
	if from.IsZero() || to.IsZero() {
		task.AppendLog(ctx, "Source database contains no logs; skipping partition precreation")
		return nil
	}
	if err := dst.AccessLogs.EnsurePartitions(ctx, from, to.AddDate(0, 1, 0)); err != nil {
		return fmt.Errorf("failed to precreate target PostgreSQL partitions: %w", err)
	}
	task.AppendLog(ctx, "Precreated target PostgreSQL partitions from %s to %s", from.Format("2006-01"), to.Format("2006-01"))
	return nil
}

// migrationRange 合并源库节点访问日志（logged_at）与用户访问日志（created_at）
// 的最小/最大时间；任一表为空时忽略该表。
func migrationRange(ctx context.Context, src *logstore.Store) (time.Time, time.Time, error) {
	fromAccess, toAccess, err := src.AccessLogs.MigrationRange(ctx)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to read source access log time range: %w", err)
	}
	fromUser, toUser, err := src.UserAccessLogs.MigrationRange(ctx)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to read source user access log time range: %w", err)
	}
	return minTime(fromAccess, fromUser), maxTime(toAccess, toUser), nil
}

func minTime(a, b time.Time) time.Time {
	switch {
	case a.IsZero():
		return b
	case b.IsZero():
		return a
	case a.Before(b):
		return a
	default:
		return b
	}
}

func maxTime(a, b time.Time) time.Time {
	switch {
	case a.IsZero():
		return b
	case b.IsZero():
		return a
	case a.After(b):
		return a
	default:
		return b
	}
}

// copyAccessLogs 从 src 复制节点访问日志到 dst。
func copyAccessLogs(ctx context.Context, src, dst *logstore.Store) error {
	// 注意：迁移期间 src 已冻结，但复制读取不受冻结影响；每批按 id 升序扫描。
	var lastID uint64
	for {
		rows, err := listNodeAccessLogsByID(ctx, src, lastID, copyBatchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		if err := dst.AccessLogs.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
			return fmt.Errorf("failed to write target access logs (batch %d): %w", lastID, err)
		}
		task.AppendLog(ctx, "Copied %d access logs (through id=%d)", len(rows), rows[len(rows)-1].ID)
		lastID = rows[len(rows)-1].ID
		if len(rows) < copyBatchSize {
			break
		}
	}
	return nil
}

func listNodeAccessLogsByID(ctx context.Context, src *logstore.Store, afterID uint64, limit int) ([]analyticsmodel.NodeAccessLog, error) {
	return src.AccessLogs.ListForMigration(ctx, afterID, limit)
}

// copyUserAccessLogs 从 src 复制用户访问日志到 dst（按 id 升序分批）。
func copyUserAccessLogs(ctx context.Context, src, dst *logstore.Store) error {
	var lastID uint64
	for {
		rows, err := src.UserAccessLogs.ListForMigration(ctx, lastID, copyBatchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := dst.UserAccessLogs.BatchInsert(ctx, rows); err != nil {
			return fmt.Errorf("failed to write target user access logs (batch %d): %w", lastID, err)
		}
		lastID = rows[len(rows)-1].ID
		task.AppendLog(ctx, "Copied %d user access logs (through id=%d)", len(rows), lastID)
		if len(rows) < copyBatchSize {
			return nil
		}
	}
}

// copyObservability 复制 4 张可观测表，每张表按 id 升序分批复制，
// 以每批最后一条 id 作为下一批游标（不使用 len 近似）。
func copyObservability(ctx context.Context, src, dst *logstore.Store) error {
	if err := copyObsTable(ctx, "metric_snapshots",
		src.Observability.ListMetricSnapshotsForMigration,
		dst.Observability.BatchInsertNodeMetricSnapshots,
		lastMetricSnapshotID); err != nil {
		return err
	}
	if err := copyObsTable(ctx, "edge_health",
		src.Observability.ListEdgeHealthForMigration,
		dst.Observability.BatchInsertNodeEdgeHealth,
		lastEdgeHealthID); err != nil {
		return err
	}
	if err := copyObsTable(ctx, "obs_frps",
		src.Observability.ListNodeObsFrpsForMigration,
		dst.Observability.BatchInsertNodeObsFrps,
		lastObsFrpsID); err != nil {
		return err
	}
	if err := copyObsTable(ctx, "obs_frpc",
		src.Observability.ListNodeObsFrpcForMigration,
		dst.Observability.BatchInsertNodeObsFrpc,
		lastObsFrpcID); err != nil {
		return err
	}
	return nil
}

// copyObsTable 按 id 升序分批复制单张可观测表；idOf 返回批内最后一条 id。
func copyObsTable[T any](ctx context.Context, name string,
	list func(context.Context, uint64, int) ([]T, error),
	insert func(context.Context, []T) error,
	idOf func([]T) uint64,
) error {
	var lastID uint64
	for {
		rows, err := list(ctx, lastID, copyBatchSize)
		if err != nil {
			return fmt.Errorf("failed to copy %s: %w", name, err)
		}
		if len(rows) == 0 {
			return nil
		}
		if err := insert(ctx, rows); err != nil {
			return fmt.Errorf("failed to copy %s: %w", name, err)
		}
		lastID = idOf(rows)
		task.AppendLog(ctx, "Copied %s: %d records (through id=%d)", name, len(rows), lastID)
		if len(rows) < copyBatchSize {
			return nil
		}
	}
}

func lastMetricSnapshotID(rows []analyticsmodel.NodeMetricSnapshot) uint64 {
	return rows[len(rows)-1].ID
}
func lastEdgeHealthID(rows []analyticsmodel.NodeEdgeHealth) uint64 { return rows[len(rows)-1].ID }
func lastObsFrpsID(rows []analyticsmodel.NodeObsFrps) uint64       { return rows[len(rows)-1].ID }
func lastObsFrpcID(rows []analyticsmodel.NodeObsFrpc) uint64       { return rows[len(rows)-1].ID }
