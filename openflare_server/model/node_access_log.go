package model

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type NodeAccessLog struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	NodeID     string    `json:"node_id" gorm:"index:idx_node_access_logs_node_logged_at,priority:1;size:64;not null"`
	LoggedAt   time.Time `json:"logged_at" gorm:"index:idx_node_access_logs_logged_at;index:idx_node_access_logs_node_logged_at,priority:2"`
	RemoteAddr string    `json:"remote_addr" gorm:"index:idx_node_access_logs_remote_addr;size:128"`
	Region     string    `json:"region" gorm:"size:128"`
	Host       string    `json:"host" gorm:"index:idx_node_access_logs_host;size:255"`
	Path       string    `json:"path" gorm:"size:2048"`
	StatusCode int       `json:"status_code" gorm:"index:idx_node_access_logs_status_code"`
	RawJSON    string    `json:"raw_json" gorm:"type:text"`
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

func ListNodeAccessLogs(query NodeAccessLogQuery) (logs []*NodeAccessLog, err error) {
	offset := query.Page * query.PageSize
	db := buildNodeAccessLogQuery(DB, query).
		Order(buildNodeAccessLogSortClause(query.SortBy, query.SortOrder)).
		Limit(query.PageSize).
		Offset(offset)
	err = db.Find(&logs).Error
	return logs, err
}

func CountNodeAccessLogs(query NodeAccessLogQuery) (totalRecords int64, totalIPs int64, err error) {
	base := buildNodeAccessLogQuery(DB.Model(&NodeAccessLog{}), query)
	if err = base.Count(&totalRecords).Error; err != nil {
		return 0, 0, err
	}
	distinctQuery := buildNodeAccessLogQuery(DB.Model(&NodeAccessLog{}), query).
		Where("remote_addr <> ''").
		Distinct("remote_addr")
	if err = distinctQuery.Count(&totalIPs).Error; err != nil {
		return 0, 0, err
	}
	return totalRecords, totalIPs, nil
}

func ListNodeAccessLogRegionCounts(nodeID string, since time.Time, limit int) (items []*NodeAccessLogRegionCount, err error) {
	query := DB.Model(&NodeAccessLog{}).
		Select("region as region, count(*) as count").
		Where("region <> ''")
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	if !since.IsZero() {
		query = query.Where("logged_at >= ?", since)
	}
	query = query.Group("region").Order("count desc, region asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err = query.Scan(&items).Error
	return items, err
}

func ListNodeAccessLogBuckets(query NodeAccessLogBucketQuery) (items []*NodeAccessLogBucketRow, err error) {
	offset := query.Page * query.PageSize
	bucketExpr := accessLogBucketEpochExpr(query.FoldMinutes)
	base := buildNodeAccessLogQuery(DB.Model(&NodeAccessLog{}), NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Path:       query.Path,
		Since:      query.Since,
	})
	err = base.Select(fmt.Sprintf(
		"%s as bucket_epoch, count(*) as request_count, count(distinct remote_addr) as unique_ip_count, count(distinct host) as unique_host_count, sum(case when status_code < 400 then 1 else 0 end) as success_count, sum(case when status_code >= 400 and status_code < 500 then 1 else 0 end) as client_error_count, sum(case when status_code >= 500 then 1 else 0 end) as server_error_count",
		bucketExpr,
	)).
		Group(bucketExpr).
		Order(buildNodeAccessLogBucketSortClause(query.SortBy, query.SortOrder)).
		Limit(query.PageSize).
		Offset(offset).
		Scan(&items).Error
	return items, err
}

func CountNodeAccessLogBuckets(query NodeAccessLogBucketQuery) (total int64, err error) {
	bucketExpr := accessLogBucketEpochExpr(query.FoldMinutes)
	base := buildNodeAccessLogQuery(DB.Model(&NodeAccessLog{}), NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Path:       query.Path,
		Since:      query.Since,
	})
	rows := []struct {
		BucketEpoch int64 `gorm:"column:bucket_epoch"`
	}{}
	err = base.Select(fmt.Sprintf("%s as bucket_epoch", bucketExpr)).
		Group(bucketExpr).
		Scan(&rows).Error
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

func ListNodeAccessLogIPSummaries(query NodeAccessLogIPSummaryQuery, recentSince time.Time) (items []*NodeAccessLogIPSummaryRow, err error) {
	offset := query.Page * query.PageSize
	base := buildNodeAccessLogQuery(DB.Model(&NodeAccessLog{}), NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	}).Where("remote_addr <> ''")
	lastSeenExpr := accessLogEpochExpr("max(logged_at)")
	err = base.Select(
		"remote_addr as remote_addr, count(*) as total_requests, sum(case when logged_at >= ? then 1 else 0 end) as recent_requests, "+lastSeenExpr+" as last_seen_epoch",
		recentSince,
	).
		Group("remote_addr").
		Order(buildNodeAccessLogIPSummarySortClause(query.SortBy, query.SortOrder)).
		Limit(query.PageSize).
		Offset(offset).
		Scan(&items).Error
	return items, err
}

func CountNodeAccessLogIPSummaries(query NodeAccessLogIPSummaryQuery) (total int64, err error) {
	base := buildNodeAccessLogQuery(DB.Model(&NodeAccessLog{}), NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	}).Where("remote_addr <> ''")
	rows := []struct {
		RemoteAddr string `gorm:"column:remote_addr"`
	}{}
	err = base.Select("remote_addr").
		Group("remote_addr").
		Scan(&rows).Error
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

func ListNodeAccessLogIPTrend(query NodeAccessLogIPTrendQuery) (items []*NodeAccessLogTrendPointRow, err error) {
	bucketExpr := accessLogBucketEpochExpr(query.BucketMinutes)
	base := buildNodeAccessLogQuery(DB.Model(&NodeAccessLog{}), NodeAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	}).Where("remote_addr = ?", strings.TrimSpace(query.RemoteAddr))
	err = base.Select(fmt.Sprintf("%s as bucket_epoch, count(*) as request_count", bucketExpr)).
		Group(bucketExpr).
		Order("bucket_epoch asc").
		Scan(&items).Error
	return items, err
}

func DeleteNodeAccessLogsBefore(before time.Time) (deleted int64, err error) {
	result := DB.Where("logged_at < ?", before).Delete(&NodeAccessLog{})
	return result.RowsAffected, result.Error
}

func buildNodeAccessLogQuery(db *gorm.DB, query NodeAccessLogQuery) *gorm.DB {
	if db == nil {
		db = DB.Model(&NodeAccessLog{})
	}
	if db.Statement == nil || db.Statement.Model == nil {
		db = db.Model(&NodeAccessLog{})
	}
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
