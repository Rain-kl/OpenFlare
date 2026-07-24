// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package analytics provides ClickHouse data access for analytics tables.
package analytics

import (
	"context"
	"fmt"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

func userAccessLogConn() error {
	if db.ChConn == nil {
		return fmt.Errorf("clickhouse native connection is not initialized")
	}
	return nil
}

// CountAccessLogs returns the number of access logs matching filter.
func CountAccessLogs(ctx context.Context, filter AccessLogFilter) (uint64, error) {
	clause, args, ok := buildUserAccessLogFilterClause(filter)
	if !ok {
		return 0, nil
	}
	if err := userAccessLogConn(); err != nil {
		return 0, err
	}
	tableName := analyticsmodel.UserAccessLog{}.TableName()
	sql := fmt.Sprintf("SELECT count() FROM %s WHERE %s", tableName, clause)
	var count uint64
	if err := db.ChConn.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count access logs: %w", err)
	}
	return count, nil
}

// ListAccessLogs returns paginated access logs and the total match count.
func ListAccessLogs(ctx context.Context, filter AccessLogFilter, page, pageSize int) ([]analyticsmodel.UserAccessLog, uint64, error) {
	clause, args, ok := buildUserAccessLogFilterClause(filter)
	if !ok {
		return []analyticsmodel.UserAccessLog{}, 0, nil
	}
	if err := userAccessLogConn(); err != nil {
		return nil, 0, err
	}

	tableName := analyticsmodel.UserAccessLog{}.TableName()
	countSQL := fmt.Sprintf("SELECT count() FROM %s WHERE %s", tableName, clause)
	var total uint64
	if err := db.ChConn.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
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
	rows, err := db.ChConn.Query(ctx, listSQL, listArgs...)
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
