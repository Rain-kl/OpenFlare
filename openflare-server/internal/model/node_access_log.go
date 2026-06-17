package model

import (
	"fmt"
	"sort"
	"strings"
	"sync"
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
	Until      time.Time
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

type NodeAccessLogBucketIPQuery struct {
	NodeID          string
	RemoteAddr      string
	Host            string
	Path            string
	BucketStartedAt time.Time
	FoldMinutes     int
	Page            int
	PageSize        int
	SortBy          string
	SortOrder       string
}

type NodeAccessLogBucketIPRow struct {
	RemoteAddr       string `json:"remote_addr"`
	RequestCount     int64  `json:"request_count"`
	SuccessCount     int64  `json:"success_count"`
	ClientErrorCount int64  `json:"client_error_count"`
	ServerErrorCount int64  `json:"server_error_count"`
	LastSeenEpoch    int64  `json:"last_seen_epoch"`
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

func (log *NodeAccessLog) BeforeCreate(*gorm.DB) error {
	return assignObservabilityID(&log.ID)
}

func ListNodeAccessLogs(query NodeAccessLogQuery) (logs []*NodeAccessLog, err error) {
	if query.PageSize > 0 {
		return listNodeAccessLogsPaginatedAcrossShards(query)
	}
	return listNodeAccessLogsAcrossShards(query)
}

func ListNodeAccessLogsForWAFIPGroup(query NodeAccessLogQuery) ([]*NodeAccessLog, error) {
	return listNodeAccessLogsAcrossShards(query)
}

func CountNodeAccessLogs(query NodeAccessLogQuery) (totalRecords int64, totalIPs int64, err error) {
	db := normalizeShardedDB(DB)
	var countErr error
	var distinctErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		totalRecords, countErr = countNodeAccessLogRecordsAcrossShards(db, query)
	}()
	go func() {
		defer wg.Done()
		totalIPs, distinctErr = countDistinctNodeAccessLogIPsAcrossShards(db, query)
	}()
	wg.Wait()
	if countErr != nil {
		return 0, 0, countErr
	}
	if distinctErr != nil {
		return 0, 0, distinctErr
	}
	return totalRecords, totalIPs, nil
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

func ListNodeAccessLogBucketIPs(query NodeAccessLogBucketIPQuery) (items []*NodeAccessLogBucketIPRow, err error) {
	rows, err := buildNodeAccessLogBucketIPRows(query)
	if err != nil {
		return nil, err
	}
	start, end := paginateBounds(len(rows), query.Page, query.PageSize)
	if start >= len(rows) {
		return []*NodeAccessLogBucketIPRow{}, nil
	}
	return rows[start:end], nil
}

func CountNodeAccessLogBucketIPs(query NodeAccessLogBucketIPQuery) (total int64, err error) {
	rows, err := buildNodeAccessLogBucketIPRows(query)
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
	return queryIPTrendRows(query)
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

func buildNodeAccessLogFilterClause(query NodeAccessLogQuery) (string, []any) {
	parts := make([]string, 0, 6)
	args := make([]any, 0, 6)
	if trimmed := strings.TrimSpace(query.NodeID); trimmed != "" {
		parts = append(parts, "node_id = ?")
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(query.RemoteAddr); trimmed != "" {
		parts = append(parts, "remote_addr LIKE ?")
		args = append(args, trimmed+"%")
	}
	if trimmed := strings.TrimSpace(query.Host); trimmed != "" {
		parts = append(parts, "host LIKE ?")
		args = append(args, trimmed+"%")
	}
	if trimmed := strings.TrimSpace(query.Path); trimmed != "" {
		parts = append(parts, "path LIKE ?")
		args = append(args, trimmed+"%")
	}
	if !query.Since.IsZero() {
		parts = append(parts, "logged_at >= ?")
		args = append(args, query.Since)
	}
	if !query.Until.IsZero() {
		parts = append(parts, "logged_at < ?")
		args = append(args, query.Until)
	}
	if len(parts) == 0 {
		return "TRUE", nil
	}
	return strings.Join(parts, " AND "), args
}

func applyNodeAccessLogFilters(db *gorm.DB, query NodeAccessLogQuery) *gorm.DB {
	clause, args := buildNodeAccessLogFilterClause(query)
	if clause == "TRUE" {
		return db
	}
	return db.Where(clause, args...)
}

func countNodeAccessLogRecordsAcrossShards(db *gorm.DB, query NodeAccessLogQuery) (int64, error) {
	tables := observabilityShardTables("node_access_logs")
	counts := make([]int64, len(tables))
	errs := make([]error, len(tables))

	var wg sync.WaitGroup
	for index, table := range tables {
		wg.Add(1)
		go func(index int, table string) {
			defer wg.Done()
			var count int64
			errs[index] = applyNodeAccessLogFilters(db.Table(table), query).Count(&count).Error
			counts[index] = count
		}(index, table)
	}
	wg.Wait()

	var total int64
	for index := range tables {
		if errs[index] != nil {
			return 0, errs[index]
		}
		total += counts[index]
	}
	return total, nil
}

func countDistinctNodeAccessLogIPsAcrossShards(db *gorm.DB, query NodeAccessLogQuery) (int64, error) {
	clause, args := buildNodeAccessLogFilterClause(query)
	tables := observabilityShardTables("node_access_logs")
	unionParts := make([]string, 0, len(tables))
	allArgs := make([]any, 0, len(args)*len(tables))
	for _, table := range tables {
		unionParts = append(unionParts, fmt.Sprintf(
			"SELECT TRIM(remote_addr) AS remote_addr FROM %s WHERE %s AND remote_addr <> ''",
			table,
			clause,
		))
		allArgs = append(allArgs, args...)
	}
	sql := fmt.Sprintf(`
SELECT COUNT(*) FROM (
	SELECT remote_addr
	FROM (%s) AS all_ips
	GROUP BY remote_addr
) AS ips`, strings.Join(unionParts, " UNION ALL "))
	var total int64
	if err := db.Raw(sql, allArgs...).Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
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

func listNodeAccessLogsPaginatedAcrossShards(query NodeAccessLogQuery) ([]*NodeAccessLog, error) {
	fetchLimit := nodeAccessLogFetchLimit(query.Page, query.PageSize)
	orderClause := nodeAccessLogOrderClause(query.SortBy, query.SortOrder)

	items := make([]*NodeAccessLog, 0, fetchLimit*observabilityShardCount)
	db := normalizeShardedDB(DB)
	for _, table := range observabilityShardTables("node_access_logs") {
		var shardRows []*NodeAccessLog
		tx := applyNodeAccessLogFilters(db.Table(table), query).Order(orderClause).Limit(fetchLimit)
		if err := tx.Find(&shardRows).Error; err != nil {
			return nil, err
		}
		items = append(items, shardRows...)
	}

	sortNodeAccessLogs(items, query.SortBy, query.SortOrder)
	start, end := paginateBounds(len(items), query.Page, query.PageSize)
	if start >= len(items) {
		return []*NodeAccessLog{}, nil
	}
	return items[start:end], nil
}

func nodeAccessLogFetchLimit(page int, pageSize int) int {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		return 0
	}
	return (page + 1) * pageSize
}

func nodeAccessLogOrderClause(sortBy string, sortOrder string) string {
	direction := "DESC"
	if normalizeSortOrder(sortOrder) == "asc" {
		direction = "ASC"
	}
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
	if column == "logged_at" {
		return column + " " + direction + ", id " + direction
	}
	return column + " " + direction + ", logged_at " + direction + ", id " + direction
}

func sortNodeAccessLogBucketIPRows(items []*NodeAccessLogBucketIPRow, sortBy string, sortOrder string) {
	desc := normalizeSortOrder(sortOrder) != "asc"
	sort.Slice(items, func(i int, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		var compare int
		switch strings.TrimSpace(sortBy) {
		case "last_seen_at":
			compare = compareInt64(left.LastSeenEpoch, right.LastSeenEpoch)
		case "remote_addr":
			compare = strings.Compare(left.RemoteAddr, right.RemoteAddr)
		default:
			compare = compareInt64(left.RequestCount, right.RequestCount)
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

func normalizeSortOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		return "asc"
	}
	return "desc"
}
