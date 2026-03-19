package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type NodeAccessLog struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	NodeID     string    `json:"node_id" gorm:"index:,composite:node_logged_at,priority:1;size:64;not null"`
	LoggedAt   time.Time `json:"logged_at" gorm:"index;index:,composite:node_logged_at,priority:2"`
	RemoteAddr string    `json:"remote_addr" gorm:"index;size:128"`
	Region     string    `json:"region" gorm:"size:128"`
	Host       string    `json:"host" gorm:"index;size:255"`
	Path       string    `json:"path" gorm:"size:2048"`
	StatusCode int       `json:"status_code" gorm:"index"`
	CreatedAt  time.Time `json:"created_at"`
}

type NodeAccessLogRegionCount struct {
	Region string `json:"region"`
	Count  int64  `json:"count"`
}

type NodeAccessLogQuery struct {
	NodeID     string
	RemoteAddr string
	Host       string
	Path       string
	Since      time.Time
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

type NodeAccessLogBucketQuery struct {
	NodeID      string
	RemoteAddr  string
	Host        string
	Path        string
	Since       time.Time
	Page        int
	PageSize    int
	SortBy      string
	SortOrder   string
	FoldMinutes int
}

type NodeAccessLogBucketRow struct {
	BucketEpoch      int64 `json:"bucket_epoch"`
	RequestCount     int64 `json:"request_count"`
	UniqueIPCount    int64 `json:"unique_ip_count"`
	UniqueHostCount  int64 `json:"unique_host_count"`
	SuccessCount     int64 `json:"success_count"`
	ClientErrorCount int64 `json:"client_error_count"`
	ServerErrorCount int64 `json:"server_error_count"`
}

type NodeAccessLogIPSummaryQuery struct {
	NodeID     string
	RemoteAddr string
	Host       string
	Since      time.Time
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

type NodeAccessLogIPSummaryRow struct {
	RemoteAddr     string `json:"remote_addr"`
	TotalRequests  int64  `json:"total_requests"`
	RecentRequests int64  `json:"recent_requests"`
	LastSeenEpoch  int64  `json:"last_seen_epoch"`
}

type NodeAccessLogIPTrendQuery struct {
	NodeID        string
	RemoteAddr    string
	Host          string
	Since         time.Time
	BucketMinutes int
}

type NodeAccessLogTrendPointRow struct {
	BucketEpoch  int64 `json:"bucket_epoch"`
	RequestCount int64 `json:"request_count"`
}

func (log *NodeAccessLog) BeforeCreate(tx *gorm.DB) error {
	return assignObservabilityID(&log.ID)
}

func ListNodeAccessLogs(query NodeAccessLogQuery) (logs []*NodeAccessLog, err error) {
	all, err := listNodeAccessLogsAcrossShards(query)
	if err != nil {
		return nil, err
	}
	start, end := paginateBounds(len(all), query.Page, query.PageSize)
	if start >= len(all) {
		return []*NodeAccessLog{}, nil
	}
	return all[start:end], nil
}

func CountNodeAccessLogs(query NodeAccessLogQuery) (totalRecords int64, totalIPs int64, err error) {
	all, err := listNodeAccessLogsAcrossShards(query)
	if err != nil {
		return 0, 0, err
	}
	ips := make(map[string]struct{}, len(all))
	for _, item := range all {
		if item == nil {
			continue
		}
		trimmed := strings.TrimSpace(item.RemoteAddr)
		if trimmed != "" {
			ips[trimmed] = struct{}{}
		}
	}
	return int64(len(all)), int64(len(ips)), nil
}

func ListNodeAccessLogRegionCounts(nodeID string, since time.Time, limit int) (items []*NodeAccessLogRegionCount, err error) {
	logs, err := listNodeAccessLogsAcrossShards(NodeAccessLogQuery{
		NodeID: nodeID,
		Since:  since,
	})
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64)
	for _, item := range logs {
		if item == nil {
			continue
		}
		region := strings.TrimSpace(item.Region)
		if region == "" {
			continue
		}
		counts[region]++
	}
	items = make([]*NodeAccessLogRegionCount, 0, len(counts))
	for region, count := range counts {
		items = append(items, &NodeAccessLogRegionCount{
			Region: region,
			Count:  count,
		})
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Region < items[j].Region
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func ListNodeAccessLogBuckets(query NodeAccessLogBucketQuery) (items []*NodeAccessLogBucketRow, err error) {
	rows, err := buildNodeAccessLogBucketRows(query)
	if err != nil {
		return nil, err
	}
	start, end := paginateBounds(len(rows), query.Page, query.PageSize)
	if start >= len(rows) {
		return []*NodeAccessLogBucketRow{}, nil
	}
	return rows[start:end], nil
}

func CountNodeAccessLogBuckets(query NodeAccessLogBucketQuery) (total int64, err error) {
	rows, err := buildNodeAccessLogBucketRows(query)
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

func ListNodeAccessLogIPSummaries(query NodeAccessLogIPSummaryQuery, recentSince time.Time) (items []*NodeAccessLogIPSummaryRow, err error) {
	rows, err := buildNodeAccessLogIPSummaryRows(query, recentSince)
	if err != nil {
		return nil, err
	}
	start, end := paginateBounds(len(rows), query.Page, query.PageSize)
	if start >= len(rows) {
		return []*NodeAccessLogIPSummaryRow{}, nil
	}
	return rows[start:end], nil
}

func CountNodeAccessLogIPSummaries(query NodeAccessLogIPSummaryQuery) (total int64, err error) {
	rows, err := buildNodeAccessLogIPSummaryRows(query, time.Time{})
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

func ListNodeAccessLogIPTrend(query NodeAccessLogIPTrendQuery) (items []*NodeAccessLogTrendPointRow, err error) {
	logs, err := listNodeAccessLogsAcrossShards(NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	})
	if err != nil {
		return nil, err
	}
	remoteAddr := strings.TrimSpace(query.RemoteAddr)
	if remoteAddr == "" {
		return []*NodeAccessLogTrendPointRow{}, nil
	}
	buckets := make(map[int64]int64)
	for _, item := range logs {
		if item == nil || strings.TrimSpace(item.RemoteAddr) != remoteAddr {
			continue
		}
		bucketEpoch := bucketEpochForTime(item.LoggedAt, query.BucketMinutes)
		buckets[bucketEpoch]++
	}
	items = make([]*NodeAccessLogTrendPointRow, 0, len(buckets))
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

func DeleteNodeAccessLogsBefore(before time.Time) (deleted int64, err error) {
	return deleteAcrossShards(DB, "node_access_logs", &NodeAccessLog{}, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("logged_at < ?", before)
	})
}

func DeleteAllNodeAccessLogs(db *gorm.DB) (deleted int64, err error) {
	return deleteAcrossShards(db, "node_access_logs", &NodeAccessLog{}, nil)
}

func NodeAccessLogExists(db *gorm.DB, record *NodeAccessLog) (bool, error) {
	if record == nil {
		return false, nil
	}
	db = normalizeShardedDB(db)
	for _, table := range observabilityShardTables("node_access_logs") {
		var count int64
		if err := db.Table(table).
			Where(
				"node_id = ? AND logged_at = ? AND remote_addr = ? AND host = ? AND path = ? AND status_code = ?",
				record.NodeID,
				record.LoggedAt,
				record.RemoteAddr,
				record.Host,
				record.Path,
				record.StatusCode,
			).
			Limit(1).
			Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func DeleteNodeAccessLogsByNodeBefore(db *gorm.DB, nodeID string, before time.Time) (deleted int64, err error) {
	return deleteAcrossShards(db, "node_access_logs", &NodeAccessLog{}, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("node_id = ? AND logged_at < ?", nodeID, before)
	})
}

func buildNodeAccessLogQuery(db *gorm.DB, query NodeAccessLogQuery) *gorm.DB {
	if db == nil {
		db = DB.Model(&NodeAccessLog{})
	}
	if db.Statement == nil || db.Statement.Model == nil {
		db = db.Model(&NodeAccessLog{})
	}
	return applyNodeAccessLogFilters(db, query)
}

func applyNodeAccessLogFilters(db *gorm.DB, query NodeAccessLogQuery) *gorm.DB {
	if trimmed := strings.TrimSpace(query.NodeID); trimmed != "" {
		db = db.Where("node_id LIKE ?", "%"+trimmed+"%")
	}
	if trimmed := strings.TrimSpace(query.RemoteAddr); trimmed != "" {
		db = db.Where("remote_addr LIKE ?", "%"+trimmed+"%")
	}
	if trimmed := strings.TrimSpace(query.Host); trimmed != "" {
		db = db.Where("host LIKE ?", "%"+trimmed+"%")
	}
	if trimmed := strings.TrimSpace(query.Path); trimmed != "" {
		db = db.Where("path LIKE ?", "%"+trimmed+"%")
	}
	if !query.Since.IsZero() {
		db = db.Where("logged_at >= ?", query.Since)
	}
	return db
}

func listNodeAccessLogsAcrossShards(query NodeAccessLogQuery) ([]*NodeAccessLog, error) {
	items, err := queryAcrossShards("node_access_logs", func(tx *gorm.DB) ([]*NodeAccessLog, error) {
		var shardRows []*NodeAccessLog
		if err := applyNodeAccessLogFilters(tx, query).Find(&shardRows).Error; err != nil {
			return nil, err
		}
		return shardRows, nil
	})
	if err != nil {
		return nil, err
	}
	sortNodeAccessLogs(items, query.SortBy, query.SortOrder)
	return items, nil
}

func buildNodeAccessLogBucketRows(query NodeAccessLogBucketQuery) ([]*NodeAccessLogBucketRow, error) {
	logs, err := listNodeAccessLogsAcrossShards(NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Path:       query.Path,
		Since:      query.Since,
	})
	if err != nil {
		return nil, err
	}
	type bucketAccumulator struct {
		requestCount     int64
		uniqueIPs        map[string]struct{}
		uniqueHosts      map[string]struct{}
		successCount     int64
		clientErrorCount int64
		serverErrorCount int64
	}
	accumulators := make(map[int64]*bucketAccumulator)
	for _, item := range logs {
		if item == nil {
			continue
		}
		bucketEpoch := bucketEpochForTime(item.LoggedAt, query.FoldMinutes)
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
	sortNodeAccessLogBucketRows(rows, query.SortBy, query.SortOrder)
	return rows, nil
}

func buildNodeAccessLogIPSummaryRows(query NodeAccessLogIPSummaryQuery, recentSince time.Time) ([]*NodeAccessLogIPSummaryRow, error) {
	logs, err := listNodeAccessLogsAcrossShards(NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	})
	if err != nil {
		return nil, err
	}
	type accumulator struct {
		totalRequests  int64
		recentRequests int64
		lastSeenAt     time.Time
	}
	accumulators := make(map[string]*accumulator)
	for _, item := range logs {
		if item == nil {
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
	sortNodeAccessLogIPSummaryRows(rows, query.SortBy, query.SortOrder)
	return rows, nil
}

func sortNodeAccessLogs(items []*NodeAccessLog, sortBy string, sortOrder string) {
	desc := normalizeSortOrder(sortOrder) != "asc"
	sort.Slice(items, func(i int, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		var compare int
		switch strings.TrimSpace(sortBy) {
		case "status_code":
			compare = compareInt(left.StatusCode, right.StatusCode)
		case "remote_addr":
			compare = strings.Compare(left.RemoteAddr, right.RemoteAddr)
		case "host":
			compare = strings.Compare(left.Host, right.Host)
		case "path":
			compare = strings.Compare(left.Path, right.Path)
		default:
			compare = compareTime(left.LoggedAt, right.LoggedAt)
		}
		if compare == 0 {
			compare = compareTime(left.LoggedAt, right.LoggedAt)
		}
		if compare == 0 {
			compare = compareUint(left.ID, right.ID)
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
}

func sortNodeAccessLogBucketRows(items []*NodeAccessLogBucketRow, sortBy string, sortOrder string) {
	desc := normalizeSortOrder(sortOrder) != "asc"
	sort.Slice(items, func(i int, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		var compare int
		switch strings.TrimSpace(sortBy) {
		case "request_count":
			compare = compareInt64(left.RequestCount, right.RequestCount)
		default:
			compare = compareInt64(left.BucketEpoch, right.BucketEpoch)
		}
		if compare == 0 {
			compare = compareInt64(left.BucketEpoch, right.BucketEpoch)
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
}

func sortNodeAccessLogIPSummaryRows(items []*NodeAccessLogIPSummaryRow, sortBy string, sortOrder string) {
	desc := normalizeSortOrder(sortOrder) != "asc"
	sort.Slice(items, func(i int, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		var compare int
		switch strings.TrimSpace(sortBy) {
		case "recent_requests":
			compare = compareInt64(left.RecentRequests, right.RecentRequests)
		case "last_seen_at":
			compare = compareInt64(left.LastSeenEpoch, right.LastSeenEpoch)
		case "remote_addr":
			compare = strings.Compare(left.RemoteAddr, right.RemoteAddr)
		default:
			compare = compareInt64(left.TotalRequests, right.TotalRequests)
		}
		if compare == 0 {
			compare = compareInt64(left.LastSeenEpoch, right.LastSeenEpoch)
		}
		if compare == 0 {
			compare = strings.Compare(left.RemoteAddr, right.RemoteAddr)
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
}

func paginateBounds(total int, page int, pageSize int) (int, int) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		return 0, total
	}
	start := page * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return start, end
}

func bucketEpochForTime(value time.Time, bucketMinutes int) int64 {
	bucketSeconds := int64(bucketMinutes * 60)
	if bucketSeconds <= 0 {
		bucketSeconds = 180
	}
	return (value.UTC().Unix() / bucketSeconds) * bucketSeconds
}

func compareTime(left time.Time, right time.Time) int {
	switch {
	case left.After(right):
		return 1
	case left.Before(right):
		return -1
	default:
		return 0
	}
}

func compareInt(left int, right int) int {
	switch {
	case left > right:
		return 1
	case left < right:
		return -1
	default:
		return 0
	}
}

func compareInt64(left int64, right int64) int {
	switch {
	case left > right:
		return 1
	case left < right:
		return -1
	default:
		return 0
	}
}

func compareUint(left uint, right uint) int {
	switch {
	case left > right:
		return 1
	case left < right:
		return -1
	default:
		return 0
	}
}

func buildNodeAccessLogSortClause(sortBy string, sortOrder string) string {
	column := "logged_at"
	switch strings.TrimSpace(sortBy) {
	case "status_code":
		column = "status_code"
	case "remote_addr":
		column = "remote_addr"
	case "host":
		column = "host"
	case "path":
		column = "path"
	}
	order := normalizeSortOrder(sortOrder)
	if column == "logged_at" {
		return fmt.Sprintf("%s %s, id %s", column, order, order)
	}
	return fmt.Sprintf("%s %s, logged_at desc, id desc", column, order)
}

func buildNodeAccessLogBucketSortClause(sortBy string, sortOrder string) string {
	order := normalizeSortOrder(sortOrder)
	switch strings.TrimSpace(sortBy) {
	case "request_count":
		return fmt.Sprintf("request_count %s, bucket_epoch desc", order)
	default:
		return fmt.Sprintf("bucket_epoch %s", order)
	}
}

func buildNodeAccessLogIPSummarySortClause(sortBy string, sortOrder string) string {
	order := normalizeSortOrder(sortOrder)
	switch strings.TrimSpace(sortBy) {
	case "recent_requests":
		return fmt.Sprintf("recent_requests %s, last_seen_epoch desc, remote_addr asc", order)
	case "last_seen_at":
		return fmt.Sprintf("last_seen_epoch %s, total_requests desc, remote_addr asc", order)
	case "remote_addr":
		return fmt.Sprintf("remote_addr %s", order)
	default:
		return fmt.Sprintf("total_requests %s, last_seen_epoch desc, remote_addr asc", order)
	}
}

func accessLogBucketEpochExpr(bucketMinutes int) string {
	bucketSeconds := bucketMinutes * 60
	if bucketSeconds <= 0 {
		bucketSeconds = 180
	}
	switch DB.Dialector.Name() {
	case "postgres":
		return fmt.Sprintf("CAST(floor(extract(epoch from logged_at) / %d) * %d AS BIGINT)", bucketSeconds, bucketSeconds)
	default:
		return fmt.Sprintf("CAST((strftime('%%s', logged_at) / %d) * %d AS INTEGER)", bucketSeconds, bucketSeconds)
	}
}

func accessLogEpochExpr(expression string) string {
	switch DB.Dialector.Name() {
	case "postgres":
		return fmt.Sprintf("CAST(extract(epoch from %s) AS BIGINT)", expression)
	default:
		return fmt.Sprintf("CAST(strftime('%%s', %s) AS INTEGER)", expression)
	}
}

func normalizeSortOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		return "asc"
	}
	return "desc"
}
