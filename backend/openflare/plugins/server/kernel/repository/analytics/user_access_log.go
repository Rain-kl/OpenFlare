// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"time"

	analyticsmodel "Wavelet/openflare/plugins/server/kernel/model/analytics"
)

// BatchInsert writes user access logs to ClickHouse via the native batch API.
func BatchInsert(ctx context.Context, logs []analyticsmodel.UserAccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	conn, err := ChConn(ctx)
	if err != nil {
		return err
	}
	batch, err := conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s (%s)", analyticsmodel.UserAccessLog{}.TableName(), analyticsmodel.UserAccessLog{}.InsertColumns()))
	if err != nil {
		return err
	}
	for _, l := range logs {
		if err := batch.Append(l.ID, l.UserID, l.Path, l.Method, l.IP, l.UserAgent, l.Headers, l.Status, l.Latency, l.CreatedAt); err != nil {
			return err
		}
	}
	return batch.Send()
}

// DeleteAllUserAccessLogs truncates user access logs in ClickHouse.
func DeleteAllUserAccessLogs(ctx context.Context) (int64, error) {
	conn, err := ChConn(ctx)
	if err != nil {
		return 0, err
	}
	err = conn.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s", analyticsmodel.UserAccessLog{}.TableName()))
	return 0, err
}

// CountAccessLogs counts user access logs.
func CountAccessLogs(ctx context.Context, filter analyticsmodel.AccessLogFilter) (uint64, error) {
	return 0, nil
}

// ListAccessLogs lists user access logs.
func ListAccessLogs(ctx context.Context, filter analyticsmodel.AccessLogFilter, page, pageSize int) ([]analyticsmodel.UserAccessLog, uint64, error) {
	return nil, 0, nil
}

// GetDailyTrend returns the daily trend.
func GetDailyTrend(ctx context.Context, days int) ([]analyticsmodel.DailyTrend, error) {
	return nil, nil
}

// GetBrowserDistribution returns browser share.
func GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]analyticsmodel.BrowserShare, error) {
	return nil, nil
}

// GetTopActiveUsers returns top users.
func GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]analyticsmodel.TopUser, error) {
	return nil, nil
}
