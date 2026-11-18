// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/pkg/util"
)

const userAccessLogFilterClauseCapacity = 4

// BatchInsert 是 batchwriter flush 目标：CH 原生批量写入；冻结期拒绝写入，空批次直接返回。
func (s *clickhouseUserAccessLogStore) BatchInsert(ctx context.Context, logs []analyticsmodel.UserAccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	conn, err := chConn()
	if err != nil {
		return err
	}

	batch, err := conn.PrepareBatch(ctx, analyticsmodel.UserAccessLog{}.BatchInsertSQL())
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}

	for _, logItem := range logs {
		if err := batch.Append(
			logItem.ID,
			logItem.UserID,
			logItem.Path,
			logItem.Method,
			logItem.IP,
			logItem.UserAgent,
			logItem.Headers,
			logItem.Status,
			logItem.Latency,
			logItem.CreatedAt,
		); err != nil {
			return fmt.Errorf("append access log to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}

// DeleteAll 清空全部用户访问日志（TRUNCATE 语义，迁移「覆盖目标库已有日志」幂等前提用）。
func (s *clickhouseUserAccessLogStore) DeleteAll(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	conn, err := chConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, analyticsmodel.UserAccessLog{}.TableName())
	if err != nil {
		return 0, err
	}
	return outcome.DeletedCount, nil
}

func (s *clickhouseUserAccessLogStore) Count(ctx context.Context, filter analyticsmodel.AccessLogFilter) (uint64, error) {
	clause, args, ok := buildCHUserAccessLogFilterClause(filter)
	if !ok {
		return 0, nil
	}
	conn, err := chConn()
	if err != nil {
		return 0, err
	}
	tableName := analyticsmodel.UserAccessLog{}.TableName()
	sql := fmt.Sprintf("SELECT count() FROM %s WHERE %s", tableName, clause)
	var count uint64
	if err := conn.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count access logs: %w", err)
	}
	return count, nil
}

func (s *clickhouseUserAccessLogStore) List(ctx context.Context, filter analyticsmodel.AccessLogFilter, page, pageSize int) ([]analyticsmodel.UserAccessLog, uint64, error) {
	clause, args, ok := buildCHUserAccessLogFilterClause(filter)
	if !ok {
		return []analyticsmodel.UserAccessLog{}, 0, nil
	}
	conn, err := chConn()
	if err != nil {
		return nil, 0, err
	}

	tableName := analyticsmodel.UserAccessLog{}.TableName()
	countSQL := fmt.Sprintf("SELECT count() FROM %s WHERE %s", tableName, clause)
	var total uint64
	if err := conn.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count access logs: %w", err)
	}
	if total == 0 {
		return []analyticsmodel.UserAccessLog{}, 0, nil
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	listSQL := fmt.Sprintf(`
SELECT id, user_id, path, method, ip, user_agent, headers, status, latency, created_at
FROM %s
WHERE %s
ORDER BY created_at DESC, id DESC
LIMIT ? OFFSET ?`, tableName, clause)
	listArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := conn.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	logs := make([]analyticsmodel.UserAccessLog, 0, pageSize)
	for rows.Next() {
		var (
			item      analyticsmodel.UserAccessLog
			createdAt time.Time
		)
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Path,
			&item.Method,
			&item.IP,
			&item.UserAgent,
			&item.Headers,
			&item.Status,
			&item.Latency,
			&createdAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan access log row: %w", err)
		}
		item.CreatedAt = createdAt
		logs = append(logs, item)
	}
	return logs, total, nil
}

func buildCHUserAccessLogFilterClause(filter analyticsmodel.AccessLogFilter) (string, []any, bool) {
	if filter.UserIDs != nil && len(filter.UserIDs) == 0 {
		return "", nil, false
	}

	parts := make([]string, 0, userAccessLogFilterClauseCapacity)
	args := make([]any, 0, userAccessLogFilterClauseCapacity)
	if filter.UserIDs != nil {
		placeholders := make([]string, len(filter.UserIDs))
		for index, userID := range filter.UserIDs {
			placeholders[index] = "?"
			args = append(args, userID)
		}
		parts = append(parts, fmt.Sprintf("user_id IN (%s)", strings.Join(placeholders, ", ")))
	}
	if trimmed := strings.TrimSpace(filter.Path); trimmed != "" {
		parts = append(parts, "path LIKE ?")
		args = append(args, "%"+util.EscapeLike(trimmed)+"%")
	}
	if filter.StartTime != nil {
		parts = append(parts, "created_at >= ?")
		args = append(args, *filter.StartTime)
	}
	if filter.EndTime != nil {
		parts = append(parts, "created_at <= ?")
		args = append(args, *filter.EndTime)
	}
	if len(parts) == 0 {
		return "1", args, true
	}
	return strings.Join(parts, " AND "), args, true
}
