// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/db/idgen"
)

const openFlareAccessLogTable = "of_node_access_logs"

type accessLogStore interface {
	InsertBatch(ctx context.Context, records []*OpenFlareAccessLog) error
	List(ctx context.Context, query OpenFlareAccessLogQuery) ([]*OpenFlareAccessLog, error)
	Count(ctx context.Context, query OpenFlareAccessLogQuery) (int64, int64, error)
	RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareAccessLogRegionCount, error)
	BucketAggregates(ctx context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) ([]openFlareAccessLogBucketAggregateRow, error)
	BucketDimensions(ctx context.Context, filter OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]openFlareAccessLogBucketDimensionRow, error)
	IPAggregates(ctx context.Context, filter OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]openFlareAccessLogIPAggregateRow, error)
	IPSummaries(ctx context.Context, filter OpenFlareAccessLogQuery, recentSince time.Time) ([]openFlareAccessLogIPSummaryRow, error)
	IPTrend(ctx context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) ([]openFlareAccessLogIPTrendRow, error)
	DeleteAll(ctx context.Context) (int64, error)
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
	DeleteByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error)
}

var (
	accessLogStoreMu     sync.RWMutex
	accessLogStoreHolder accessLogStore
)

func currentAccessLogStore() accessLogStore {
	accessLogStoreMu.RLock()
	defer accessLogStoreMu.RUnlock()
	if accessLogStoreHolder != nil {
		return accessLogStoreHolder
	}
	return clickhouseAccessLogStore{}
}

// SetAccessLogStoreForTest swaps the access log store implementation for unit tests.
func SetAccessLogStoreForTest(store accessLogStore) func() {
	accessLogStoreMu.Lock()
	previous := accessLogStoreHolder
	accessLogStoreHolder = store
	accessLogStoreMu.Unlock()
	return func() {
		accessLogStoreMu.Lock()
		accessLogStoreHolder = previous
		accessLogStoreMu.Unlock()
	}
}

// NewMemoryAccessLogStore returns an in-memory access log store for unit tests.
func NewMemoryAccessLogStore() accessLogStore {
	return &memoryAccessLogStore{
		records: make([]*OpenFlareAccessLog, 0),
	}
}

type clickhouseAccessLogStore struct{}

func (clickhouseAccessLogStore) conn() (driver.Conn, error) {
	if db.ChConn == nil {
		return nil, errors.New(errClickHouseNotInitialized)
	}
	return db.ChConn, nil
}

func (clickhouseAccessLogStore) InsertBatch(ctx context.Context, records []*OpenFlareAccessLog) error {
	if len(records) == 0 {
		return nil
	}
	conn, err := clickhouseAccessLogStore{}.conn()
	if err != nil {
		return err
	}
	batch, err := conn.PrepareBatch(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, node_id, logged_at, remote_addr, region, host, path, status_code, created_at)",
		openFlareAccessLogTable,
	))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, record := range records {
		if record == nil {
			continue
		}
		id := record.ID
		if id == 0 {
			id = uint(idgen.NextUint64ID())
		}
		createdAt := record.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if err := batch.Append(
			uint64(id),
			record.NodeID,
			record.LoggedAt.UTC(),
			record.RemoteAddr,
			record.Region,
			record.Host,
			record.Path,
			int32(record.StatusCode),
			createdAt.UTC(),
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (clickhouseAccessLogStore) List(ctx context.Context, query OpenFlareAccessLogQuery) ([]*OpenFlareAccessLog, error) {
	conn, err := clickhouseAccessLogStore{}.conn()
	if err != nil {
		return nil, err
	}
	clause, args := buildOpenFlareAccessLogFilterClause(query)
	sql := fmt.Sprintf(`
SELECT id, node_id, logged_at, remote_addr, region, host, path, status_code, created_at
FROM %s
WHERE %s
ORDER BY %s`, openFlareAccessLogTable, clause, openFlareAccessLogOrderClause(query.SortBy, query.SortOrder))
	if query.PageSize > 0 {
		if query.Page < 0 {
			query.Page = 0
		}
		sql += " LIMIT ? OFFSET ?"
		args = append(args, query.PageSize, query.Page*query.PageSize)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanOpenFlareAccessLogRows(rows)
}

func scanOpenFlareAccessLogRows(rows driver.Rows) ([]*OpenFlareAccessLog, error) {
	var result []*OpenFlareAccessLog
	for rows.Next() {
		var (
			id         uint64
			nodeID     string
			loggedAt   time.Time
			remoteAddr string
			region     string
			host       string
			path       string
			statusCode int32
			createdAt  time.Time
		)
		if err := rows.Scan(&id, &nodeID, &loggedAt, &remoteAddr, &region, &host, &path, &statusCode, &createdAt); err != nil {
			return nil, err
		}
		result = append(result, &OpenFlareAccessLog{
			ID:         uint(id),
			NodeID:     nodeID,
			LoggedAt:   loggedAt.UTC(),
			RemoteAddr: remoteAddr,
			Region:     region,
			Host:       host,
			Path:       path,
			StatusCode: int(statusCode),
			CreatedAt:  createdAt.UTC(),
		})
	}
	return result, nil
}

func (clickhouseAccessLogStore) Count(ctx context.Context, query OpenFlareAccessLogQuery) (int64, int64, error) {
	conn, err := clickhouseAccessLogStore{}.conn()
	if err != nil {
		return 0, 0, err
	}
	clause, args := buildOpenFlareAccessLogFilterClause(query)
	var totalRecords int64
	countSQL := fmt.Sprintf("SELECT count() FROM %s WHERE %s", openFlareAccessLogTable, clause)
	if err := conn.QueryRow(ctx, countSQL, args...).Scan(&totalRecords); err != nil {
		return 0, 0, err
	}
	ipSQL := fmt.Sprintf(`
SELECT count() FROM (
	SELECT trim(remote_addr) AS remote_addr
	FROM %s
	WHERE %s AND remote_addr != ''
	GROUP BY trim(remote_addr)
)`, openFlareAccessLogTable, clause)
	var totalIPs int64
	if err := conn.QueryRow(ctx, ipSQL, args...).Scan(&totalIPs); err != nil {
		return 0, 0, err
	}
	return totalRecords, totalIPs, nil
}

func (clickhouseAccessLogStore) RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareAccessLogRegionCount, error) {
	conn, err := clickhouseAccessLogStore{}.conn()
	if err != nil {
		return nil, err
	}
	filter := OpenFlareAccessLogQuery{NodeID: nodeID, Since: since}
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	sql := fmt.Sprintf(`
SELECT trim(region) AS region, count() AS count
FROM %s
WHERE %s AND trim(region) != ''
GROUP BY trim(region)
ORDER BY count DESC, region ASC`, openFlareAccessLogTable, clause)
	if limit > 0 {
		sql += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []*OpenFlareAccessLogRegionCount
	for rows.Next() {
		var item OpenFlareAccessLogRegionCount
		if err := rows.Scan(&item.Region, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, &item)
	}
	return result, nil
}

func (clickhouseAccessLogStore) BucketAggregates(ctx context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) ([]openFlareAccessLogBucketAggregateRow, error) {
	conn, err := clickhouseAccessLogStore{}.conn()
	if err != nil {
		return nil, err
	}
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	bucketExpr := openFlareAccessLogBucketEpochExpr(bucketSeconds)
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	count() AS request_count,
	countIf(status_code < 400) AS success_count,
	countIf(status_code >= 400 AND status_code < 500) AS client_error_count,
	countIf(status_code >= 500) AS server_error_count
FROM %s
WHERE %s
GROUP BY bucket_epoch`, bucketExpr, openFlareAccessLogTable, clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []openFlareAccessLogBucketAggregateRow
	for rows.Next() {
		var item openFlareAccessLogBucketAggregateRow
		if err := rows.Scan(&item.BucketEpoch, &item.RequestCount, &item.SuccessCount, &item.ClientErrorCount, &item.ServerErrorCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (clickhouseAccessLogStore) BucketDimensions(ctx context.Context, filter OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]openFlareAccessLogBucketDimensionRow, error) {
	conn, err := clickhouseAccessLogStore{}.conn()
	if err != nil {
		return nil, err
	}
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	bucketExpr := openFlareAccessLogBucketEpochExpr(bucketSeconds)
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	trim(%s) AS value
FROM %s
WHERE %s AND trim(%s) != ''
GROUP BY bucket_epoch, trim(%s)`, bucketExpr, column, openFlareAccessLogTable, clause, column, column)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []openFlareAccessLogBucketDimensionRow
	for rows.Next() {
		var item openFlareAccessLogBucketDimensionRow
		if err := rows.Scan(&item.BucketEpoch, &item.Value); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (clickhouseAccessLogStore) IPAggregates(ctx context.Context, filter OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]openFlareAccessLogIPAggregateRow, error) {
	conn, err := clickhouseAccessLogStore{}.conn()
	if err != nil {
		return nil, err
	}
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	queryClause := clause
	queryArgs := append([]any{}, args...)
	if exactRemoteAddr {
		trimmed := strings.TrimSpace(filter.RemoteAddr)
		if trimmed == "" {
			return []openFlareAccessLogIPAggregateRow{}, nil
		}
		queryClause = combineOpenFlareAccessLogSQLClauses(queryClause, "trim(remote_addr) = ?")
		queryArgs = append(queryArgs, trimmed)
	}
	lastSeenExpr := openFlareAccessLogEpochExpr()
	sql := fmt.Sprintf(`
SELECT
	trim(remote_addr) AS remote_addr,
	count() AS request_count,
	countIf(status_code < 400) AS success_count,
	countIf(status_code >= 400 AND status_code < 500) AS client_error_count,
	countIf(status_code >= 500) AS server_error_count,
	max(%s) AS last_seen_epoch
FROM %s
WHERE %s AND trim(remote_addr) != ''
GROUP BY trim(remote_addr)`, lastSeenExpr, openFlareAccessLogTable, queryClause)
	rows, err := conn.Query(ctx, sql, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []openFlareAccessLogIPAggregateRow
	for rows.Next() {
		var item openFlareAccessLogIPAggregateRow
		if err := rows.Scan(&item.RemoteAddr, &item.RequestCount, &item.SuccessCount, &item.ClientErrorCount, &item.ServerErrorCount, &item.LastSeenEpoch); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (clickhouseAccessLogStore) IPSummaries(ctx context.Context, filter OpenFlareAccessLogQuery, recentSince time.Time) ([]openFlareAccessLogIPSummaryRow, error) {
	conn, err := clickhouseAccessLogStore{}.conn()
	if err != nil {
		return nil, err
	}
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	lastSeenExpr := openFlareAccessLogEpochExpr()
	recentClause := "0"
	queryArgs := make([]any, 0, len(args)+1)
	if !recentSince.IsZero() {
		recentClause = "if(logged_at >= ?, 1, 0)"
		queryArgs = append(queryArgs, recentSince)
	}
	queryArgs = append(queryArgs, args...)
	sql := fmt.Sprintf(`
SELECT
	trim(remote_addr) AS remote_addr,
	count() AS total_requests,
	sum(%s) AS recent_requests,
	max(%s) AS last_seen_epoch
FROM %s
WHERE %s AND trim(remote_addr) != ''
GROUP BY trim(remote_addr)`, recentClause, lastSeenExpr, openFlareAccessLogTable, clause)
	rows, err := conn.Query(ctx, sql, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []openFlareAccessLogIPSummaryRow
	for rows.Next() {
		var item openFlareAccessLogIPSummaryRow
		if err := rows.Scan(&item.RemoteAddr, &item.TotalRequests, &item.RecentRequests, &item.LastSeenEpoch); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (clickhouseAccessLogStore) IPTrend(ctx context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) ([]openFlareAccessLogIPTrendRow, error) {
	conn, err := clickhouseAccessLogStore{}.conn()
	if err != nil {
		return nil, err
	}
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	bucketExpr := openFlareAccessLogBucketEpochExpr(bucketSeconds)
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	count() AS request_count
FROM %s
WHERE %s
GROUP BY bucket_epoch
ORDER BY bucket_epoch ASC`, bucketExpr, openFlareAccessLogTable, clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []openFlareAccessLogIPTrendRow
	for rows.Next() {
		var item openFlareAccessLogIPTrendRow
		if err := rows.Scan(&item.BucketEpoch, &item.RequestCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (s clickhouseAccessLogStore) DeleteAll(ctx context.Context) (int64, error) {
	return s.deleteWithCount(ctx, "SELECT count() FROM "+openFlareAccessLogTable, nil, "ALTER TABLE "+openFlareAccessLogTable+" DELETE WHERE 1")
}

func (s clickhouseAccessLogStore) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.deleteWithCount(
		ctx,
		fmt.Sprintf("SELECT count() FROM %s WHERE logged_at < ?", openFlareAccessLogTable),
		[]any{cutoff.UTC()},
		fmt.Sprintf("ALTER TABLE %s DELETE WHERE logged_at < ?", openFlareAccessLogTable),
		cutoff.UTC(),
	)
}

func (s clickhouseAccessLogStore) DeleteByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error) {
	return s.deleteWithCount(
		ctx,
		fmt.Sprintf("SELECT count() FROM %s WHERE node_id = ? AND logged_at < ?", openFlareAccessLogTable),
		[]any{nodeID, before.UTC()},
		fmt.Sprintf("ALTER TABLE %s DELETE WHERE node_id = ? AND logged_at < ?", openFlareAccessLogTable),
		nodeID, before.UTC(),
	)
}

func (clickhouseAccessLogStore) deleteWithCount(ctx context.Context, countSQL string, countArgs []any, deleteSQL string, deleteArgs ...any) (int64, error) {
	conn, err := clickhouseAccessLogStore{}.conn()
	if err != nil {
		return 0, err
	}
	var count int64
	if err := conn.QueryRow(ctx, countSQL, countArgs...).Scan(&count); err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}
	if err := conn.Exec(ctx, deleteSQL, deleteArgs...); err != nil {
		return 0, err
	}
	return count, nil
}

func openFlareAccessLogBucketEpochExpr(bucketSeconds int64) string {
	return fmt.Sprintf("toInt64(intDiv(toUnixTimestamp(logged_at), %d) * %d)", bucketSeconds, bucketSeconds)
}

func openFlareAccessLogEpochExpr() string {
	return "toInt64(toUnixTimestamp(logged_at))"
}