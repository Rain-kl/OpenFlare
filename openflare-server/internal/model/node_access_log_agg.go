package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type shardBucketAggregateRow struct {
	BucketEpoch      int64 `gorm:"column:bucket_epoch"`
	RequestCount     int64 `gorm:"column:request_count"`
	SuccessCount     int64 `gorm:"column:success_count"`
	ClientErrorCount int64 `gorm:"column:client_error_count"`
	ServerErrorCount int64 `gorm:"column:server_error_count"`
}

type shardBucketDimensionRow struct {
	BucketEpoch int64  `gorm:"column:bucket_epoch"`
	Value       string `gorm:"column:value"`
}

type shardIPAggregateRow struct {
	RemoteAddr       string `gorm:"column:remote_addr"`
	RequestCount     int64  `gorm:"column:request_count"`
	SuccessCount     int64  `gorm:"column:success_count"`
	ClientErrorCount int64  `gorm:"column:client_error_count"`
	ServerErrorCount int64  `gorm:"column:server_error_count"`
	LastSeenEpoch    int64  `gorm:"column:last_seen_epoch"`
}

type shardIPSummaryRow struct {
	RemoteAddr     string `gorm:"column:remote_addr"`
	TotalRequests  int64  `gorm:"column:total_requests"`
	RecentRequests int64  `gorm:"column:recent_requests"`
	LastSeenEpoch  int64  `gorm:"column:last_seen_epoch"`
}

type shardIPTrendRow struct {
	BucketEpoch  int64 `gorm:"column:bucket_epoch"`
	RequestCount int64 `gorm:"column:request_count"`
}

func buildNodeAccessLogBucketRows(query NodeAccessLogBucketQuery) ([]*NodeAccessLogBucketRow, error) {
	db := normalizeShardedDB(DB)
	filter := nodeAccessLogQueryFromBucket(query)
	clause, args := buildNodeAccessLogFilterClause(filter)
	bucketSeconds := int64(query.FoldMinutes * 60)
	if bucketSeconds <= 0 {
		bucketSeconds = 180
	}
	bucketExpr := accessLogBucketEpochExpr(databaseDialect(db), bucketSeconds)

	type bucketAccumulator struct {
		requestCount     int64
		uniqueIPs        map[string]struct{}
		uniqueHosts      map[string]struct{}
		successCount     int64
		clientErrorCount int64
		serverErrorCount int64
	}
	accumulators := make(map[int64]*bucketAccumulator)

	for _, table := range observabilityShardTables("node_access_logs") {
		var partials []shardBucketAggregateRow
		sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	COUNT(*) AS request_count,
	SUM(CASE WHEN status_code < 400 THEN 1 ELSE 0 END) AS success_count,
	SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END) AS client_error_count,
	SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) AS server_error_count
FROM %s
WHERE %s
GROUP BY bucket_epoch`, bucketExpr, table, clause)
		if err := db.Raw(sql, args...).Scan(&partials).Error; err != nil {
			return nil, err
		}
		for _, partial := range partials {
			accumulator := accumulators[partial.BucketEpoch]
			if accumulator == nil {
				accumulator = &bucketAccumulator{
					uniqueIPs:   make(map[string]struct{}),
					uniqueHosts: make(map[string]struct{}),
				}
				accumulators[partial.BucketEpoch] = accumulator
			}
			accumulator.requestCount += partial.RequestCount
			accumulator.successCount += partial.SuccessCount
			accumulator.clientErrorCount += partial.ClientErrorCount
			accumulator.serverErrorCount += partial.ServerErrorCount
		}

		for _, column := range []string{"remote_addr", "host"} {
			dimensions, err := queryBucketDimensionRows(db, table, clause, args, column, bucketExpr)
			if err != nil {
				return nil, err
			}
			for _, item := range dimensions {
				accumulator := accumulators[item.BucketEpoch]
				if accumulator == nil {
					accumulator = &bucketAccumulator{
						uniqueIPs:   make(map[string]struct{}),
						uniqueHosts: make(map[string]struct{}),
					}
					accumulators[item.BucketEpoch] = accumulator
				}
				trimmed := strings.TrimSpace(item.Value)
				if trimmed == "" {
					continue
				}
				switch column {
				case "remote_addr":
					accumulator.uniqueIPs[trimmed] = struct{}{}
				case "host":
					accumulator.uniqueHosts[trimmed] = struct{}{}
				}
			}
		}
	}

	rows := make([]*NodeAccessLogBucketRow, 0, len(accumulators))
	for bucketEpoch, accumulator := range accumulators {
		rows = append(rows, &NodeAccessLogBucketRow{
			BucketEpoch:      bucketEpoch,
			RequestCount:     accumulator.requestCount,
			UniqueIPCount:    int64(len(accumulator.uniqueIPs)),
			UniqueHostCount:  int64(len(accumulator.uniqueHosts)),
			SuccessCount:     accumulator.successCount,
			ClientErrorCount: accumulator.clientErrorCount,
			ServerErrorCount: accumulator.serverErrorCount,
		})
	}
	sortNodeAccessLogBucketRows(rows, query.SortBy, query.SortOrder)
	return rows, nil
}

func queryBucketDimensionRows(db *gorm.DB, table string, clause string, args []any, column string, bucketExpr string) ([]shardBucketDimensionRow, error) {
	var rows []shardBucketDimensionRow
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	TRIM(%s) AS value
FROM %s
WHERE %s AND TRIM(%s) <> ''
GROUP BY bucket_epoch, TRIM(%s)`, bucketExpr, column, table, clause, column, column)
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func buildNodeAccessLogBucketIPRows(query NodeAccessLogBucketIPQuery) ([]*NodeAccessLogBucketIPRow, error) {
	if query.BucketStartedAt.IsZero() {
		return []*NodeAccessLogBucketIPRow{}, nil
	}
	foldMinutes := query.FoldMinutes
	if foldMinutes <= 0 {
		foldMinutes = 3
	}
	bucketStartedAt := query.BucketStartedAt.UTC()
	filter := NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Path:       query.Path,
		Since:      bucketStartedAt,
		Until:      bucketStartedAt.Add(time.Duration(foldMinutes) * time.Minute),
	}
	rows, err := queryIPAggregateRows(filter, false)
	if err != nil {
		return nil, err
	}
	sortNodeAccessLogBucketIPRows(rows, query.SortBy, query.SortOrder)
	return rows, nil
}

func buildNodeAccessLogIPSummaryRows(query NodeAccessLogIPSummaryQuery, recentSince time.Time) ([]*NodeAccessLogIPSummaryRow, error) {
	filter := NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	}
	db := normalizeShardedDB(DB)
	clause, args := buildNodeAccessLogFilterClause(filter)
	lastSeenExpr := accessLogEpochExpr(databaseDialect(db))

	type accumulator struct {
		totalRequests  int64
		recentRequests int64
		lastSeenEpoch  int64
	}
	accumulators := make(map[string]*accumulator)

	for _, table := range observabilityShardTables("node_access_logs") {
		recentClause := "0"
		queryArgs := make([]any, 0, len(args)+1)
		if !recentSince.IsZero() {
			recentClause = "CASE WHEN logged_at >= ? THEN 1 ELSE 0 END"
			queryArgs = append(queryArgs, recentSince)
		}
		queryArgs = append(queryArgs, args...)
		var partials []shardIPSummaryRow
		sql := fmt.Sprintf(`
SELECT
	TRIM(remote_addr) AS remote_addr,
	COUNT(*) AS total_requests,
	SUM(%s) AS recent_requests,
	MAX(%s) AS last_seen_epoch
FROM %s
WHERE %s AND TRIM(remote_addr) <> ''
GROUP BY TRIM(remote_addr)`, recentClause, lastSeenExpr, table, clause)
		if err := db.Raw(sql, queryArgs...).Scan(&partials).Error; err != nil {
			return nil, err
		}
		for _, partial := range partials {
			remoteAddr := strings.TrimSpace(partial.RemoteAddr)
			if remoteAddr == "" {
				continue
			}
			acc := accumulators[remoteAddr]
			if acc == nil {
				acc = &accumulator{}
				accumulators[remoteAddr] = acc
			}
			acc.totalRequests += partial.TotalRequests
			acc.recentRequests += partial.RecentRequests
			if partial.LastSeenEpoch > acc.lastSeenEpoch {
				acc.lastSeenEpoch = partial.LastSeenEpoch
			}
		}
	}

	rows := make([]*NodeAccessLogIPSummaryRow, 0, len(accumulators))
	for remoteAddr, acc := range accumulators {
		rows = append(rows, &NodeAccessLogIPSummaryRow{
			RemoteAddr:     remoteAddr,
			TotalRequests:  acc.totalRequests,
			RecentRequests: acc.recentRequests,
			LastSeenEpoch:  acc.lastSeenEpoch,
		})
	}
	sortNodeAccessLogIPSummaryRows(rows, query.SortBy, query.SortOrder)
	return rows, nil
}

func queryIPAggregateRows(filter NodeAccessLogQuery, exactRemoteAddr bool) ([]*NodeAccessLogBucketIPRow, error) {
	db := normalizeShardedDB(DB)
	clause, args := buildNodeAccessLogFilterClause(filter)
	lastSeenExpr := accessLogEpochExpr(databaseDialect(db))

	type accumulator struct {
		requestCount     int64
		successCount     int64
		clientErrorCount int64
		serverErrorCount int64
		lastSeenEpoch    int64
	}
	accumulators := make(map[string]*accumulator)

	for _, table := range observabilityShardTables("node_access_logs") {
		queryClause := clause
		queryArgs := append([]any{}, args...)
		if exactRemoteAddr {
			trimmed := strings.TrimSpace(filter.RemoteAddr)
			if trimmed == "" {
				return []*NodeAccessLogBucketIPRow{}, nil
			}
			queryClause = combineSQLClauses(queryClause, "TRIM(remote_addr) = ?")
			queryArgs = append(queryArgs, trimmed)
		}
		var partials []shardIPAggregateRow
		sql := fmt.Sprintf(`
SELECT
	TRIM(remote_addr) AS remote_addr,
	COUNT(*) AS request_count,
	SUM(CASE WHEN status_code < 400 THEN 1 ELSE 0 END) AS success_count,
	SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END) AS client_error_count,
	SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) AS server_error_count,
	MAX(%s) AS last_seen_epoch
FROM %s
WHERE %s AND TRIM(remote_addr) <> ''
GROUP BY TRIM(remote_addr)`, lastSeenExpr, table, queryClause)
		if err := db.Raw(sql, queryArgs...).Scan(&partials).Error; err != nil {
			return nil, err
		}
		for _, partial := range partials {
			remoteAddr := strings.TrimSpace(partial.RemoteAddr)
			if remoteAddr == "" {
				continue
			}
			acc := accumulators[remoteAddr]
			if acc == nil {
				acc = &accumulator{}
				accumulators[remoteAddr] = acc
			}
			acc.requestCount += partial.RequestCount
			acc.successCount += partial.SuccessCount
			acc.clientErrorCount += partial.ClientErrorCount
			acc.serverErrorCount += partial.ServerErrorCount
			if partial.LastSeenEpoch > acc.lastSeenEpoch {
				acc.lastSeenEpoch = partial.LastSeenEpoch
			}
		}
	}

	rows := make([]*NodeAccessLogBucketIPRow, 0, len(accumulators))
	for remoteAddr, acc := range accumulators {
		rows = append(rows, &NodeAccessLogBucketIPRow{
			RemoteAddr:       remoteAddr,
			RequestCount:     acc.requestCount,
			SuccessCount:     acc.successCount,
			ClientErrorCount: acc.clientErrorCount,
			ServerErrorCount: acc.serverErrorCount,
			LastSeenEpoch:    acc.lastSeenEpoch,
		})
	}
	return rows, nil
}

func queryIPTrendRows(query NodeAccessLogIPTrendQuery) ([]*NodeAccessLogTrendPointRow, error) {
	remoteAddr := strings.TrimSpace(query.RemoteAddr)
	if remoteAddr == "" {
		return []*NodeAccessLogTrendPointRow{}, nil
	}
	db := normalizeShardedDB(DB)
	filter := NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: remoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	bucketSeconds := int64(query.BucketMinutes * 60)
	if bucketSeconds <= 0 {
		bucketSeconds = 1800
	}
	bucketExpr := accessLogBucketEpochExpr(databaseDialect(db), bucketSeconds)
	queryClause := combineSQLClauses(clause, "TRIM(remote_addr) = ?")
	queryArgs := append(append([]any{}, args...), remoteAddr)

	buckets := make(map[int64]int64)
	for _, table := range observabilityShardTables("node_access_logs") {
		var partials []shardIPTrendRow
		sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	COUNT(*) AS request_count
FROM %s
WHERE %s
GROUP BY bucket_epoch`, bucketExpr, table, queryClause)
		if err := db.Raw(sql, queryArgs...).Scan(&partials).Error; err != nil {
			return nil, err
		}
		for _, partial := range partials {
			buckets[partial.BucketEpoch] += partial.RequestCount
		}
	}

	items := make([]*NodeAccessLogTrendPointRow, 0, len(buckets))
	for bucketEpoch, requestCount := range buckets {
		items = append(items, &NodeAccessLogTrendPointRow{
			BucketEpoch:  bucketEpoch,
			RequestCount: requestCount,
		})
	}
	sort.Slice(items, func(i int, j int) bool {
		return items[i].BucketEpoch < items[j].BucketEpoch
	})
	return items, nil
}

func nodeAccessLogQueryFromBucket(query NodeAccessLogBucketQuery) NodeAccessLogQuery {
	return NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Path:       query.Path,
		Since:      query.Since,
	}
}

func databaseDialect(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		return "sqlite"
	}
	switch db.Dialector.Name() {
	case "postgres":
		return "postgres"
	default:
		return "sqlite"
	}
}

func accessLogBucketEpochExpr(dialect string, bucketSeconds int64) string {
	switch dialect {
	case "postgres":
		return fmt.Sprintf("FLOOR(EXTRACT(EPOCH FROM logged_at AT TIME ZONE 'UTC') / %d) * %d", bucketSeconds, bucketSeconds)
	default:
		return fmt.Sprintf("(CAST(strftime('%%s', logged_at) AS INTEGER) / %d) * %d", bucketSeconds, bucketSeconds)
	}
}

func accessLogEpochExpr(dialect string) string {
	switch dialect {
	case "postgres":
		return "FLOOR(EXTRACT(EPOCH FROM logged_at AT TIME ZONE 'UTC'))::bigint"
	default:
		return "CAST((julianday(logged_at) - 2440587.5) * 86400 AS INTEGER)"
	}
}

func combineSQLClauses(left string, right string) string {
	if strings.TrimSpace(left) == "" || left == "TRUE" {
		return right
	}
	return left + " AND " + right
}
