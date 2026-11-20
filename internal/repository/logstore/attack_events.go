// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

func appendWAFLogFilter(parts []string, args []any, action string) ([]string, []any) {
	switch action {
	case "all":
		parts = append(parts, "waf_action <> ''")
	case "blocked", "observed":
		parts = append(parts, "waf_action <> '' AND waf_action = ?")
		args = append(args, action)
	}
	return parts, args
}

func attackEventSQL(bucket, epoch, where string) string {
	return fmt.Sprintf(`SELECT remote_addr, host, %s AS bucket_unix,
		MIN(%s) AS first_unix, MAX(%s) AS last_unix, COUNT(*) AS count
		FROM %s WHERE %s GROUP BY remote_addr, host, bucket_unix`,
		bucket, epoch, epoch, (analyticsmodel.NodeAccessLog{}).TableName(), where)
}

const attackEventPageSQL = ` ORDER BY last_unix DESC, remote_addr, host, bucket_unix LIMIT ? OFFSET ?`
const attackEventCountSQL = `SELECT COUNT(*) FROM (%s) AS attack_events`

func (s *gormLogStore) AttackEvents(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]model.OpenFlareAttackEvent, int64, error) {
	where, args := buildNodeAccessLogFilterParts(toNodeAccessLogFilter(query))
	sql := attackEventSQL(timeBucketSQL(s.db, "logged_at", bucketSeconds), epochSQL(s.db, "logged_at"), where)
	var total int64
	if err := s.db.WithContext(ctx).Raw(fmt.Sprintf(attackEventCountSQL, sql), args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	events := make([]model.OpenFlareAttackEvent, 0)
	err := s.db.WithContext(ctx).Raw(sql+attackEventPageSQL, append(args, query.PageSize, query.Page*query.PageSize)...).Scan(&events).Error
	return events, total, err
}

func (s *clickhouseLogStore) AttackEvents(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]model.OpenFlareAttackEvent, int64, error) {
	conn, err := chConn()
	if err != nil {
		return nil, 0, err
	}
	where, args := buildCHNodeAccessLogFilterClause(toNodeAccessLogFilter(query))
	sql := attackEventSQL(chNodeAccessLogBucketEpochExpr(bucketSeconds), chNodeAccessLogEpochExpr(), where)
	var total uint64
	if err := conn.QueryRow(ctx, fmt.Sprintf(attackEventCountSQL, sql), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := conn.Query(ctx, sql+attackEventPageSQL, append(args, query.PageSize, query.Page*query.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	events := make([]model.OpenFlareAttackEvent, 0)
	for rows.Next() {
		var event model.OpenFlareAttackEvent
		var count uint64
		if err := rows.Scan(&event.RemoteAddr, &event.Host, &event.BucketUnix, &event.FirstUnix, &event.LastUnix, &count); err != nil {
			return nil, 0, err
		}
		event.Count = safeInt64Count(count)
		events = append(events, event)
	}
	return events, safeInt64Count(total), rows.Err()
}
