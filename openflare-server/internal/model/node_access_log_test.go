package model

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestListNodeAccessLogsPaginatedAcrossShards(t *testing.T) {
	db := openBareTestSQLiteDB(t, "node_access_log_pagination.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := autoMigrateAll(db); err != nil {
		t.Fatalf("auto migrate db: %v", err)
	}
	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
	})

	now := time.Now().UTC()
	for index := range 15 {
		record := &NodeAccessLog{
			NodeID:     "node-page",
			LoggedAt:   now.Add(-time.Duration(index) * time.Minute),
			RemoteAddr: fmt.Sprintf("203.0.113.%d", (index%5)+1),
			Host:       "example.com",
			Path:       fmt.Sprintf("/path-%02d", index),
			StatusCode: 200,
		}
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed access log %d: %v", index, err)
		}
	}

	query := NodeAccessLogQuery{
		NodeID:    "node-page",
		Page:      1,
		PageSize:  5,
		SortBy:    "logged_at",
		SortOrder: "desc",
	}
	page, err := ListNodeAccessLogs(query)
	if err != nil {
		t.Fatalf("ListNodeAccessLogs failed: %v", err)
	}
	if len(page) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(page))
	}
	if page[0].Path != "/path-05" || page[4].Path != "/path-09" {
		t.Fatalf("unexpected page ordering: %+v", page)
	}

	totalRecords, totalIPs, err := CountNodeAccessLogs(query)
	if err != nil {
		t.Fatalf("CountNodeAccessLogs failed: %v", err)
	}
	if totalRecords != 15 {
		t.Fatalf("expected total_records=15, got %d", totalRecords)
	}
	if totalIPs != 5 {
		t.Fatalf("expected total_ip=5, got %d", totalIPs)
	}
}

func TestNodeAccessLogOptimizedQueriesMatchReference(t *testing.T) {
	db := openBareTestSQLiteDB(t, "node_access_log_correctness.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := autoMigrateAll(db); err != nil {
		t.Fatalf("auto migrate db: %v", err)
	}
	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
	})

	now := time.Now().UTC()
	records := []*NodeAccessLog{
		{NodeID: "node-a", LoggedAt: now.Add(-5 * time.Minute), RemoteAddr: "1.1.1.1", Host: "a.example.com", Path: "/alpha", StatusCode: 200},
		{NodeID: "node-a", LoggedAt: now.Add(-4 * time.Minute), RemoteAddr: "2.2.2.2", Host: "a.example.com", Path: "/beta", StatusCode: 404},
		{NodeID: "node-b", LoggedAt: now.Add(-3 * time.Minute), RemoteAddr: "1.1.1.1", Host: "b.example.com", Path: "/gamma", StatusCode: 502},
		{NodeID: "node-b", LoggedAt: now.Add(-2 * time.Minute), RemoteAddr: " 3.3.3.3 ", Host: "b.example.com", Path: "/delta", StatusCode: 200},
		{NodeID: "node-b", LoggedAt: now.Add(-1 * time.Minute), RemoteAddr: "", Host: "b.example.com", Path: "/empty-ip", StatusCode: 200},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed access log: %v", err)
		}
	}

	baseQuery := NodeAccessLogQuery{
		Since:     now.Add(-10 * time.Minute),
		SortBy:    "logged_at",
		SortOrder: "desc",
	}
	reference, err := listNodeAccessLogsAcrossShards(baseQuery)
	if err != nil {
		t.Fatalf("reference list failed: %v", err)
	}
	referenceTotal, referenceIPs, err := countNodeAccessLogsReference(baseQuery)
	if err != nil {
		t.Fatalf("reference count failed: %v", err)
	}

	totalRecords, totalIPs, err := CountNodeAccessLogs(baseQuery)
	if err != nil {
		t.Fatalf("CountNodeAccessLogs failed: %v", err)
	}
	if totalRecords != referenceTotal {
		t.Fatalf("total_records mismatch: got %d want %d", totalRecords, referenceTotal)
	}
	if totalIPs != referenceIPs {
		t.Fatalf("total_ip mismatch: got %d want %d", totalIPs, referenceIPs)
	}
	if totalRecords != int64(len(reference)) {
		t.Fatalf("total_records should equal reference rows: got %d want %d", totalRecords, len(reference))
	}

	for page := range 3 {
		query := baseQuery
		query.Page = page
		query.PageSize = 2
		pageRows, err := ListNodeAccessLogs(query)
		if err != nil {
			t.Fatalf("ListNodeAccessLogs page %d failed: %v", page, err)
		}
		start, end := paginateBounds(len(reference), page, query.PageSize)
		if start >= len(reference) {
			if len(pageRows) != 0 {
				t.Fatalf("page %d expected empty slice, got %d rows", page, len(pageRows))
			}
			continue
		}
		want := reference[start:end]
		if !nodeAccessLogsEqual(pageRows, want) {
			t.Fatalf("page %d mismatch:\n got=%+v\nwant=%+v", page, pageRows, want)
		}
	}

	filteredQuery := NodeAccessLogQuery{
		NodeID:    "node-a",
		Since:     baseQuery.Since,
		SortBy:    "status_code",
		SortOrder: "asc",
		Page:      0,
		PageSize:  10,
	}
	filteredReference, err := listNodeAccessLogsAcrossShards(filteredQuery)
	if err != nil {
		t.Fatalf("filtered reference list failed: %v", err)
	}
	filteredRows, err := ListNodeAccessLogs(filteredQuery)
	if err != nil {
		t.Fatalf("filtered ListNodeAccessLogs failed: %v", err)
	}
	if !nodeAccessLogsEqual(filteredRows, filteredReference) {
		t.Fatalf("filtered list mismatch:\n got=%+v\nwant=%+v", filteredRows, filteredReference)
	}
	filteredTotal, filteredIPs, err := CountNodeAccessLogs(filteredQuery)
	if err != nil {
		t.Fatalf("filtered CountNodeAccessLogs failed: %v", err)
	}
	wantFilteredTotal, wantFilteredIPs, err := countNodeAccessLogsReference(filteredQuery)
	if err != nil {
		t.Fatalf("filtered reference count failed: %v", err)
	}
	if filteredTotal != wantFilteredTotal || filteredIPs != wantFilteredIPs {
		t.Fatalf("filtered count mismatch: got (%d,%d) want (%d,%d)", filteredTotal, filteredIPs, wantFilteredTotal, wantFilteredIPs)
	}
}

func countNodeAccessLogsReference(query NodeAccessLogQuery) (int64, int64, error) {
	all, err := listNodeAccessLogsAcrossShards(query)
	if err != nil {
		return 0, 0, err
	}
	ips := make(map[string]struct{})
	for _, item := range all {
		if item == nil {
			continue
		}
		if trimmed := strings.TrimSpace(item.RemoteAddr); trimmed != "" {
			ips[trimmed] = struct{}{}
		}
	}
	return int64(len(all)), int64(len(ips)), nil
}

func nodeAccessLogsEqual(left []*NodeAccessLog, right []*NodeAccessLog) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] == nil || right[index] == nil {
			if left[index] != right[index] {
				return false
			}
			continue
		}
		if left[index].ID != right[index].ID ||
			left[index].NodeID != right[index].NodeID ||
			!left[index].LoggedAt.Equal(right[index].LoggedAt) ||
			left[index].RemoteAddr != right[index].RemoteAddr ||
			left[index].Host != right[index].Host ||
			left[index].Path != right[index].Path ||
			left[index].StatusCode != right[index].StatusCode {
			return false
		}
	}
	return true
}

func TestNodeAccessLogAggregationsMatchReference(t *testing.T) {
	db := openBareTestSQLiteDB(t, "node_access_log_agg.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := autoMigrateAll(db); err != nil {
		t.Fatalf("auto migrate db: %v", err)
	}
	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
	})

	now := time.Date(2026, 3, 19, 8, 12, 30, 0, time.UTC)
	records := []*NodeAccessLog{
		{NodeID: "node-folded", LoggedAt: now.Add(-4 * time.Minute), RemoteAddr: "203.0.113.1", Host: "alpha.example.com", Path: "/first", StatusCode: 200},
		{NodeID: "node-folded", LoggedAt: now.Add(-3 * time.Minute), RemoteAddr: "203.0.113.1", Host: "alpha.example.com", Path: "/second", StatusCode: 502},
		{NodeID: "node-folded", LoggedAt: now.Add(-2 * time.Minute), RemoteAddr: "203.0.113.2", Host: "beta.example.com", Path: "/third", StatusCode: 404},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed access log: %v", err)
		}
	}

	since := now.Add(-10 * time.Minute)
	bucketRows, err := buildNodeAccessLogBucketRows(NodeAccessLogBucketQuery{
		NodeID: "node-folded", Since: since, FoldMinutes: 5, SortBy: "request_count", SortOrder: "desc",
	})
	if err != nil {
		t.Fatalf("buildNodeAccessLogBucketRows failed: %v", err)
	}
	referenceBuckets := referenceBucketRows(records, 5, "request_count", "desc")
	if !bucketRowsEqual(bucketRows, referenceBuckets) {
		t.Fatalf("bucket rows mismatch:\n got=%+v\nwant=%+v", bucketRows, referenceBuckets)
	}

	if len(bucketRows) == 0 {
		t.Fatal("expected bucket rows before bucket ip verification")
	}
	bucketStartedAt := time.Unix(bucketRows[0].BucketEpoch, 0).UTC()
	bucketIPRows, err := buildNodeAccessLogBucketIPRows(NodeAccessLogBucketIPQuery{
		NodeID: "node-folded", BucketStartedAt: bucketStartedAt, FoldMinutes: 5, SortBy: "request_count", SortOrder: "desc",
	})
	if err != nil {
		t.Fatalf("buildNodeAccessLogBucketIPRows failed: %v", err)
	}
	referenceBucketIPs := referenceBucketIPRows(records, bucketStartedAt, 5, "request_count", "desc")
	if !bucketIPRowsEqual(bucketIPRows, referenceBucketIPs) {
		if len(bucketIPRows) > 0 && len(referenceBucketIPs) > 0 {
			t.Fatalf("bucket ip rows mismatch:\n got=%+v\nwant=%+v", *bucketIPRows[0], *referenceBucketIPs[0])
		}
		t.Fatalf("bucket ip rows mismatch:\n got=%+v\nwant=%+v", bucketIPRows, referenceBucketIPs)
	}

	recentSince := now.Add(-150 * time.Minute)
	summaryRows, err := buildNodeAccessLogIPSummaryRows(NodeAccessLogIPSummaryQuery{
		NodeID: "node-folded", Since: since, SortBy: "total_requests", SortOrder: "desc",
	}, recentSince)
	if err != nil {
		t.Fatalf("buildNodeAccessLogIPSummaryRows failed: %v", err)
	}
	referenceSummaries := referenceIPSummaryRows(records, since, recentSince, "total_requests", "desc")
	if !ipSummaryRowsEqual(summaryRows, referenceSummaries) {
		t.Fatalf("ip summary rows mismatch:\n got=%+v\nwant=%+v", summaryRows, referenceSummaries)
	}

	trendRows, err := queryIPTrendRows(NodeAccessLogIPTrendQuery{
		NodeID: "node-folded", RemoteAddr: "203.0.113.1", Since: since, BucketMinutes: 5,
	})
	if err != nil {
		t.Fatalf("queryIPTrendRows failed: %v", err)
	}
	referenceTrend := referenceIPTrendRows(records, "203.0.113.1", 5)
	if !trendRowsEqual(trendRows, referenceTrend) {
		t.Fatalf("trend rows mismatch:\n got=%+v\nwant=%+v", trendRows, referenceTrend)
	}
}

func referenceBucketRows(records []*NodeAccessLog, foldMinutes int, sortBy string, sortOrder string) []*NodeAccessLogBucketRow {
	type bucketAccumulator struct {
		requestCount     int64
		uniqueIPs        map[string]struct{}
		uniqueHosts      map[string]struct{}
		successCount     int64
		clientErrorCount int64
		serverErrorCount int64
	}
	accumulators := make(map[int64]*bucketAccumulator)
	for _, item := range records {
		if item == nil {
			continue
		}
		bucketEpoch := bucketEpochForTime(item.LoggedAt, foldMinutes)
		accumulator := accumulators[bucketEpoch]
		if accumulator == nil {
			accumulator = &bucketAccumulator{
				uniqueIPs:   make(map[string]struct{}),
				uniqueHosts: make(map[string]struct{}),
			}
			accumulators[bucketEpoch] = accumulator
		}
		accumulator.requestCount++
		if trimmed := strings.TrimSpace(item.RemoteAddr); trimmed != "" {
			accumulator.uniqueIPs[trimmed] = struct{}{}
		}
		if trimmed := strings.TrimSpace(item.Host); trimmed != "" {
			accumulator.uniqueHosts[trimmed] = struct{}{}
		}
		switch {
		case item.StatusCode < 400:
			accumulator.successCount++
		case item.StatusCode < 500:
			accumulator.clientErrorCount++
		default:
			accumulator.serverErrorCount++
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
	sortNodeAccessLogBucketRows(rows, sortBy, sortOrder)
	return rows
}

func referenceBucketIPRows(records []*NodeAccessLog, bucketStartedAt time.Time, foldMinutes int, sortBy string, sortOrder string) []*NodeAccessLogBucketIPRow {
	type accumulator struct {
		requestCount     int64
		successCount     int64
		clientErrorCount int64
		serverErrorCount int64
		lastSeenAt       time.Time
	}
	accumulators := make(map[string]*accumulator)
	until := bucketStartedAt.Add(time.Duration(foldMinutes) * time.Minute)
	for _, item := range records {
		if item == nil || item.LoggedAt.Before(bucketStartedAt) || !item.LoggedAt.Before(until) {
			continue
		}
		remoteAddr := strings.TrimSpace(item.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		acc := accumulators[remoteAddr]
		if acc == nil {
			acc = &accumulator{}
			accumulators[remoteAddr] = acc
		}
		acc.requestCount++
		switch {
		case item.StatusCode < 400:
			acc.successCount++
		case item.StatusCode < 500:
			acc.clientErrorCount++
		default:
			acc.serverErrorCount++
		}
		if item.LoggedAt.After(acc.lastSeenAt) {
			acc.lastSeenAt = item.LoggedAt
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
			LastSeenEpoch:    acc.lastSeenAt.Unix(),
		})
	}
	sortNodeAccessLogBucketIPRows(rows, sortBy, sortOrder)
	return rows
}

func referenceIPSummaryRows(records []*NodeAccessLog, since time.Time, recentSince time.Time, sortBy string, sortOrder string) []*NodeAccessLogIPSummaryRow {
	type accumulator struct {
		totalRequests  int64
		recentRequests int64
		lastSeenAt     time.Time
	}
	accumulators := make(map[string]*accumulator)
	for _, item := range records {
		if item == nil || item.LoggedAt.Before(since) {
			continue
		}
		remoteAddr := strings.TrimSpace(item.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		acc := accumulators[remoteAddr]
		if acc == nil {
			acc = &accumulator{}
			accumulators[remoteAddr] = acc
		}
		acc.totalRequests++
		if !recentSince.IsZero() && !item.LoggedAt.Before(recentSince) {
			acc.recentRequests++
		}
		if item.LoggedAt.After(acc.lastSeenAt) {
			acc.lastSeenAt = item.LoggedAt
		}
	}
	rows := make([]*NodeAccessLogIPSummaryRow, 0, len(accumulators))
	for remoteAddr, acc := range accumulators {
		rows = append(rows, &NodeAccessLogIPSummaryRow{
			RemoteAddr:     remoteAddr,
			TotalRequests:  acc.totalRequests,
			RecentRequests: acc.recentRequests,
			LastSeenEpoch:  acc.lastSeenAt.Unix(),
		})
	}
	sortNodeAccessLogIPSummaryRows(rows, sortBy, sortOrder)
	return rows
}

func referenceIPTrendRows(records []*NodeAccessLog, remoteAddr string, bucketMinutes int) []*NodeAccessLogTrendPointRow {
	buckets := make(map[int64]int64)
	for _, item := range records {
		if item == nil || strings.TrimSpace(item.RemoteAddr) != remoteAddr {
			continue
		}
		buckets[bucketEpochForTime(item.LoggedAt, bucketMinutes)]++
	}
	rows := make([]*NodeAccessLogTrendPointRow, 0, len(buckets))
	for bucketEpoch, requestCount := range buckets {
		rows = append(rows, &NodeAccessLogTrendPointRow{BucketEpoch: bucketEpoch, RequestCount: requestCount})
	}
	sort.Slice(rows, func(i int, j int) bool { return rows[i].BucketEpoch < rows[j].BucketEpoch })
	return rows
}

func bucketRowsEqual(left []*NodeAccessLogBucketRow, right []*NodeAccessLogBucketRow) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] == nil || right[index] == nil {
			if left[index] != right[index] {
				return false
			}
			continue
		}
		if *left[index] != *right[index] {
			return false
		}
	}
	return true
}

func bucketIPRowsEqual(left []*NodeAccessLogBucketIPRow, right []*NodeAccessLogBucketIPRow) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] == nil || right[index] == nil {
			if left[index] != right[index] {
				return false
			}
			continue
		}
		if *left[index] != *right[index] {
			return false
		}
	}
	return true
}

func ipSummaryRowsEqual(left []*NodeAccessLogIPSummaryRow, right []*NodeAccessLogIPSummaryRow) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] == nil || right[index] == nil {
			if left[index] != right[index] {
				return false
			}
			continue
		}
		if *left[index] != *right[index] {
			return false
		}
	}
	return true
}

func trendRowsEqual(left []*NodeAccessLogTrendPointRow, right []*NodeAccessLogTrendPointRow) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] == nil || right[index] == nil {
			if left[index] != right[index] {
				return false
			}
			continue
		}
		if *left[index] != *right[index] {
			return false
		}
	}
	return true
}

func TestNodeAccessLogOrderClauseMatchesSort(t *testing.T) {
	if got := nodeAccessLogOrderClause("logged_at", "desc"); got != "logged_at DESC, id DESC" {
		t.Fatalf("unexpected logged_at order clause: %q", got)
	}
	if got := nodeAccessLogOrderClause("status_code", "asc"); got != "status_code ASC, logged_at ASC, id ASC" {
		t.Fatalf("unexpected status_code order clause: %q", got)
	}
}
