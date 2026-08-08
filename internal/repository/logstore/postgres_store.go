// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"gorm.io/gorm"
)

// 常用批处理/分页默认值与常量（mnd 禁魔法数字）。
const (
	insertBatchSize   = 500  // GORM 分批落库批次大小
	defaultTopN       = 10   // 维度 Top-N 默认返回条数
	migrationPageSize = 100  // 迁移复制每页条数
	defaultPageSize   = 20   // 用户访问日志默认分页大小
	hourBucketSeconds = 3600 // 小时级聚合分桶秒数
	dayDuration       = 24 * time.Hour
	topUserAgents     = 100 // 浏览器/系统/设备分布按 user_agent 分组取 TopN

	// 排序方向与 List 次排序列（与 CH nodeAccessLogOrderClause 对齐）。
	sortOrderDesc        = "DESC"
	sortOrderAsc         = "ASC"
	sortColumnStatusCode = "status_code"
	sortColumnRemoteAddr = "remote_addr"
	sortColumnHost       = "host"
	sortColumnPath       = "path"
)

// gormLogStore 是 PG/SQLite 共用的 GORM 日志存储实现。
type gormLogStore struct {
	db *gorm.DB
	// skipFreeze 为 true 时跳过迁移冻结检查（仅迁移目标 store 使用）。
	skipFreeze bool
}

func newGormStore(db *gorm.DB) *gormLogStore { return &gormLogStore{db: db} }

// userAccessLogGormStore 实现 UserAccessLogStore。gormLogStore 已占用 List/Count 方法名
// （AccessLogStore 接口），Go 不允许同一类型声明同名不同签名的方法，故用户访问日志
// 用独立类型复用同一 *gorm.DB；Task 5 装配时 UserAccessLogs 应使用本类型。
type userAccessLogGormStore struct {
	*gormLogStore
}

func newUserAccessLogGormStore(db *gorm.DB) *userAccessLogGormStore {
	return &userAccessLogGormStore{gormLogStore: newGormStore(db)}
}

// 编译期断言：gormLogStore 实现 AccessLogStore/ObservabilityStore；
// userAccessLogGormStore 实现 UserAccessLogStore。
var (
	_ AccessLogStore     = (*gormLogStore)(nil)
	_ ObservabilityStore = (*gormLogStore)(nil)
	_ StatusStore        = (*gormLogStore)(nil)
	_ UserAccessLogStore = (*userAccessLogGormStore)(nil)
)

// ActiveDatabase 返回 gorm 分支的日志主库名（按方言：postgres|sqlite）。
func (s *gormLogStore) ActiveDatabase(_ context.Context) (string, error) {
	if isPostgresDialect(s.db) {
		return dbNamePostgres, nil
	}
	return dbNameSQLite, nil
}

// ClickHouseOperationalStats 非 CH 激活时返回 nil（StatusStore 接口约定）。
func (s *gormLogStore) ClickHouseOperationalStats(_ context.Context) (*analyticsmodel.ClickHouseOperationalStats, error) {
	return nil, nil
}

// ensureWritable 冻结期拒绝写入。
func (s *gormLogStore) ensureWritable(ctx context.Context) error {
	if !s.skipFreeze && Migrating(ctx) {
		return ErrMigrating
	}
	return nil
}

// InsertBatch 节点访问日志写入入口：冻结检查后经 hook 入队（异步），与现状一致。
func (s *gormLogStore) InsertBatch(ctx context.Context, records []*model.OpenFlareAccessLog) error {
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	rows := make([]analyticsmodel.NodeAccessLog, 0, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		rows = append(rows, toAnalyticsNodeAccessLog(r))
	}
	if h := currentAccessLogHooks().QueueNodeAccessLogs; h != nil {
		h(rows)
	}
	return nil
}

// BatchInsertNodeAccessLogs 是 batchwriter flush 目标：GORM 分批落库。
func (s *gormLogStore) BatchInsertNodeAccessLogs(ctx context.Context, rows []analyticsmodel.NodeAccessLog) error {
	if len(rows) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return s.db.WithContext(ctx).CreateInBatches(rows, insertBatchSize).Error
}

// countToUint64 将 COUNT 结果转为 uint64（COUNT 非负；防御负值避免 int64→uint64 溢出告警）。
func countToUint64(v int64) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

// nodeIDScope 附加 node_id 过滤；空 nodeID 表示全部节点（与 CH 语义一致）。
func nodeIDScope(q *gorm.DB, nodeID string) *gorm.DB {
	if nodeID == "" {
		return q
	}
	return q.Where("node_id = ?", nodeID)
}

func (s *gormLogStore) List(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]*model.OpenFlareAccessLog, error) {
	f := toNodeAccessLogFilter(query)
	order, err := nodeAccessLogOrderClauseGORM(f.SortBy, f.SortOrder)
	if err != nil {
		return nil, err
	}
	var rows []analyticsmodel.NodeAccessLog
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	q = q.Order(order)
	// 对齐 CH ListNodeAccessLogs 的 0-based 分页：仅 PageSize>0 时 LIMIT/OFFSET（Page<0 归零），
	// PageSize<=0 时与 CH 一致不加分页（返回全部匹配行）。
	q = applyNodeAccessLogPagination(q, f)
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return fromAnalyticsNodeAccessLogs(rows), nil
}

// nodeAccessLogOrderClauseGORM 对齐 CH nodeAccessLogOrderClause：默认按 logged_at 排序；
// 仅支持 status_code/remote_addr/host/path 作为次排序列；其它 SortBy 直接报错（不静默忽略）。
func nodeAccessLogOrderClauseGORM(sortBy, sortOrder string) (string, error) {
	direction := sortOrderDesc
	if sortOrder == "asc" {
		direction = sortOrderAsc
	}
	switch strings.TrimSpace(sortBy) {
	case "", "logged_at":
		// 默认路径与历史行为完全一致。
		return "logged_at " + direction + ", id " + direction, nil
	case sortColumnStatusCode, sortColumnRemoteAddr, sortColumnHost, sortColumnPath:
		column := strings.TrimSpace(sortBy)
		return column + " " + direction + ", logged_at " + direction + ", id " + direction, nil
	default:
		return "", fmt.Errorf("unsupported sort_by: %s", sortBy)
	}
}

func (s *gormLogStore) Count(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, int64, int64, error) {
	f := toNodeAccessLogFilter(query)
	var total, uniqIP, bytesSent int64
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	if err := q.Count(&total).Error; err != nil {
		return 0, 0, 0, err
	}
	// 对齐 CH CountNodeAccessLogs：distinct IP 排除空 remote_addr（uniqExactIf(remote_addr, remote_addr != '')）。
	// 独立查询链，避免 remote_addr <> '' 条件泄漏到下面的 bytes_sent 求和。
	uniqQ := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	uniqQ = uniqQ.Where("remote_addr <> ''").Distinct("remote_addr")
	if err := uniqQ.Count(&uniqIP).Error; err != nil {
		return 0, 0, 0, err
	}
	if err := q.Select("COALESCE(SUM(bytes_sent),0)").Scan(&bytesSent).Error; err != nil {
		return 0, 0, 0, err
	}
	return total, uniqIP, bytesSent, nil
}

func (s *gormLogStore) TrafficSummary(ctx context.Context, query model.OpenFlareAccessLogQuery) (model.OpenFlareAccessLogTrafficSummary, error) {
	f := toNodeAccessLogFilter(query)
	var out struct {
		RequestCount  int64
		ErrorCount    int64
		UniqueIPCount int64
		BytesSent     int64
		RequestLength int64
		NodeCount     int64
	}
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	err := q.Select(`
        COUNT(*) AS request_count,
        COUNT(*) FILTER (WHERE status_code >= 500) AS error_count,
        ` + distinctNonEmptyCountSQL(s.db, "remote_addr") + ` AS unique_ip_count,
        COALESCE(SUM(bytes_sent),0) AS bytes_sent,
        COALESCE(SUM(request_length),0) AS request_length,
        ` + distinctNonEmptyCountSQL(s.db, "node_id") + ` AS node_count`).Scan(&out).Error
	if err != nil {
		return model.OpenFlareAccessLogTrafficSummary{}, err
	}
	return model.OpenFlareAccessLogTrafficSummary{
		RequestCount:  out.RequestCount,
		ErrorCount:    out.ErrorCount,
		UniqueIPCount: out.UniqueIPCount,
		BytesSent:     out.BytesSent,
		RequestLength: out.RequestLength,
		NodeCount:     out.NodeCount,
	}, nil
}

func (s *gormLogStore) ValueCounts(ctx context.Context, query model.OpenFlareAccessLogQuery, column string, limit int) ([]model.OpenFlareAccessLogValueCount, error) {
	col, ok := nodeAccessLogValueColumn(column)
	if !ok {
		return nil, fmt.Errorf("unsupported value count column: %s", column)
	}
	f := toNodeAccessLogFilter(query)
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	// 数值列（如 status_code）扫描进 string 会报错，统一经方言 CAST(... AS TEXT) 转文本。
	q = q.Select(textCastSQL(s.db, col) + " AS value, COUNT(*) AS count")
	type row struct {
		Value string
		Count int64
	}
	var rows []row
	if err := q.Group(col).Order("count DESC").Limit(limitOr(limit, defaultTopN)).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.OpenFlareAccessLogValueCount, len(rows))
	for i, r := range rows {
		out[i] = model.OpenFlareAccessLogValueCount{Value: r.Value, Count: r.Count}
	}
	return out, nil
}

func (s *gormLogStore) NodeAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]model.OpenFlareAccessLogNodeAggregate, error) {
	f := toNodeAccessLogFilter(query)
	type row struct {
		NodeID        string
		RequestCount  int64
		ErrorCount    int64
		UniqueIPCount int64
	}
	var rows []row
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	// 对齐 CH NodeAggregatesNodeAccessLogs：distinct IP 排除空 remote_addr；空 node_id 不参与聚合。
	q = q.Select("node_id, COUNT(*) AS request_count, COUNT(*) FILTER (WHERE status_code >= 500) AS error_count, " + distinctNonEmptyCountSQL(s.db, "remote_addr") + " AS unique_ip_count").
		Where("node_id <> ''")
	if err := q.Group("node_id").Order("request_count DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.OpenFlareAccessLogNodeAggregate, len(rows))
	for i, r := range rows {
		out[i] = model.OpenFlareAccessLogNodeAggregate{NodeID: r.NodeID, RequestCount: r.RequestCount, ErrorCount: r.ErrorCount, UniqueIPCount: r.UniqueIPCount}
	}
	return out, nil
}

func (s *gormLogStore) RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareAccessLogRegionCount, error) {
	type row struct {
		Region string
		Count  int64
	}
	var rows []row
	q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}).
		Select("region, COUNT(*) AS count").
		Where("node_id = ? AND region <> '' AND logged_at >= ?", nodeID, since)
	if err := q.Group("region").Order("count DESC").Limit(limitOr(limit, defaultTopN)).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.OpenFlareAccessLogRegionCount, len(rows))
	for i, r := range rows {
		out[i] = &model.OpenFlareAccessLogRegionCount{Region: r.Region, Count: r.Count}
	}
	return out, nil
}

func (s *gormLogStore) BucketAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketAggregate, error) {
	f := toNodeAccessLogFilter(query)
	expr := timeBucketSQL(s.db, "logged_at", bucketSeconds)
	type row struct {
		BucketEpoch      int64
		RequestCount     int64
		SuccessCount     int64
		ClientErrorCount int64
		ServerErrorCount int64
		UniqueIPCount    int64
		UniqueHostCount  int64
		BytesSent        int64
		RequestLength    int64
	}
	var rows []row
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	// 对齐 CH BucketAggregatesNodeAccessLogs：success/client_error/server_error、
	// 排除空串的 distinct IP/Host、bytes_sent/request_length 求和。
	q = q.Select(
		expr + " AS bucket_epoch, " +
			"COUNT(*) AS request_count, " +
			"COUNT(*) FILTER (WHERE status_code < 400) AS success_count, " +
			"COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500) AS client_error_count, " +
			"COUNT(*) FILTER (WHERE status_code >= 500) AS server_error_count, " +
			distinctNonEmptyCountSQL(s.db, "remote_addr") + " AS unique_ip_count, " +
			distinctNonEmptyCountSQL(s.db, "host") + " AS unique_host_count, " +
			"COALESCE(SUM(bytes_sent),0) AS bytes_sent, " +
			"COALESCE(SUM(request_length),0) AS request_length",
	)
	if err := q.Group(expr).Order("bucket_epoch ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.NodeAccessLogBucketAggregate, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.NodeAccessLogBucketAggregate{
			BucketEpoch:      r.BucketEpoch,
			RequestCount:     r.RequestCount,
			SuccessCount:     r.SuccessCount,
			ClientErrorCount: r.ClientErrorCount,
			ServerErrorCount: r.ServerErrorCount,
			UniqueIPCount:    r.UniqueIPCount,
			UniqueHostCount:  r.UniqueHostCount,
			BytesSent:        r.BytesSent,
			RequestLength:    r.RequestLength,
		}
	}
	return out, nil
}

// CountBuckets 返回过滤窗口内的时间分桶数量。
func (s *gormLogStore) CountBuckets(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) (int64, error) {
	f := toNodeAccessLogFilter(query)
	expr := timeBucketSQL(s.db, "logged_at", bucketSeconds)
	var total int64
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	if err := q.Group(expr).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// BucketDimensions 返回分桶 × 维度值计数（维度值为 trim 后文本）。
func (s *gormLogStore) BucketDimensions(ctx context.Context, query model.OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketDimension, error) {
	col, ok := nodeAccessLogValueColumn(column)
	if !ok {
		return nil, fmt.Errorf("unsupported bucket dimension column: %s", column)
	}
	f := toNodeAccessLogFilter(query)
	expr := timeBucketSQL(s.db, "logged_at", bucketSeconds)
	valueExpr := "trim(" + textCastSQL(s.db, col) + ")"
	type row struct {
		BucketEpoch int64
		Value       string
	}
	var rows []row
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	q = q.Select(expr + " AS bucket_epoch, " + valueExpr + " AS value").
		Where(valueExpr + " != ''")
	if err := q.Group(expr + ", " + valueExpr).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.NodeAccessLogBucketDimension, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.NodeAccessLogBucketDimension{BucketEpoch: r.BucketEpoch, Value: r.Value}
	}
	return out, nil
}

// IPAggregates 按 remote_addr 聚合（exactRemoteAddr 时精确到指定 IP）。
func (s *gormLogStore) IPAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]analyticsmodel.NodeAccessLogIPAggregate, error) {
	f := toNodeAccessLogFilter(query)
	type row struct {
		RemoteAddr       string
		RequestCount     int64
		SuccessCount     int64
		ClientErrorCount int64
		ServerErrorCount int64
		LastSeenEpoch    int64
	}
	var rows []row
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	q = q.Select(`remote_addr, COUNT(*) AS request_count,
		COUNT(*) FILTER (WHERE status_code < 400) AS success_count,
		COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500) AS client_error_count,
		COUNT(*) FILTER (WHERE status_code >= 500) AS server_error_count,
		MAX(` + epochSQL(s.db, "logged_at") + `) AS last_seen_epoch`)
	q = q.Where("remote_addr != ''")
	if exactRemoteAddr {
		trimmed := strings.TrimSpace(f.RemoteAddr)
		if trimmed == "" {
			return []analyticsmodel.NodeAccessLogIPAggregate{}, nil
		}
		q = q.Where("remote_addr = ?", trimmed)
	}
	if err := q.Group("remote_addr").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.NodeAccessLogIPAggregate, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.NodeAccessLogIPAggregate{
			RemoteAddr:       r.RemoteAddr,
			RequestCount:     r.RequestCount,
			SuccessCount:     r.SuccessCount,
			ClientErrorCount: r.ClientErrorCount,
			ServerErrorCount: r.ServerErrorCount,
			LastSeenEpoch:    r.LastSeenEpoch,
		}
	}
	return out, nil
}

// IPSummaries 按 IP 汇总（region 取该 IP 最近一条日志的 region；recent_requests 恒 0）。
func (s *gormLogStore) IPSummaries(ctx context.Context, query model.OpenFlareAccessLogQuery, _ time.Time) ([]analyticsmodel.NodeAccessLogIPSummary, error) {
	f := toNodeAccessLogFilter(query)
	type row struct {
		RemoteAddr      string
		Region          string
		TotalRequests   int64
		Success2xxCount int64
		SuccessRatio    float64
		RequestLength   int64
		BytesSent       int64
		LastSeenEpoch   int64
	}
	var rows []row
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	q = q.Select(`
		remote_addr,
		(SELECT t2.region FROM of_node_access_logs t2 WHERE t2.remote_addr = of_node_access_logs.remote_addr ORDER BY t2.logged_at DESC LIMIT 1) AS region,
		COUNT(*) AS total_requests,
		COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) AS success2xx_count,
		CASE WHEN COUNT(*) = 0 THEN 0.0 ELSE CAST(COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) AS REAL) / CAST(COUNT(*) AS REAL) END AS success_ratio,
		COALESCE(SUM(request_length),0) AS request_length,
		COALESCE(SUM(bytes_sent),0) AS bytes_sent,
		MAX(` + epochSQL(s.db, "logged_at") + `) AS last_seen_epoch`)
	q = q.Where("remote_addr != ''")
	q = q.Group("remote_addr").Order("total_requests DESC, last_seen_epoch DESC, remote_addr ASC")
	// 对齐 CH IPSummariesNodeAccessLogs 的 0-based 分页（仅 PageSize>0 时分页）。
	q = applyNodeAccessLogPagination(q, f)
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.NodeAccessLogIPSummary, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.NodeAccessLogIPSummary{
			RemoteAddr:      r.RemoteAddr,
			Region:          r.Region,
			TotalRequests:   r.TotalRequests,
			Success2xxCount: r.Success2xxCount,
			SuccessRatio:    r.SuccessRatio,
			BytesReceived:   r.RequestLength,
			BytesSent:       r.BytesSent,
			RecentRequests:  0,
			LastSeenEpoch:   r.LastSeenEpoch,
		}
	}
	return out, nil
}

// CountIPSummaries 返回匹配过滤的 distinct IP 数量。
func (s *gormLogStore) CountIPSummaries(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, error) {
	f := toNodeAccessLogFilter(query)
	var total int64
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	q = q.Where("remote_addr != ''")
	if err := q.Group("remote_addr").Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// WAFIPAggregates 按 IP 聚合 WAF 自动规则所需的状态码/主机分布。
func (s *gormLogStore) WAFIPAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]analyticsmodel.NodeAccessLogWAFIPAggregate, error) {
	f := toNodeAccessLogFilter(query)
	type row struct {
		RemoteAddr       string
		RequestCount     int64
		Status404Count   int64
		ClientErrorCount int64
		ServerErrorCount int64
		LastSeenEpoch    int64
	}
	var rows []row
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	q = q.Select(`remote_addr, COUNT(*) AS request_count,
		COUNT(*) FILTER (WHERE status_code = 404) AS status404_count,
		COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500) AS client_error_count,
		COUNT(*) FILTER (WHERE status_code >= 500) AS server_error_count,
		MAX(` + epochSQL(s.db, "logged_at") + `) AS last_seen_epoch`)
	q = q.Where("remote_addr != ''")
	if err := q.Group("remote_addr").Scan(&rows).Error; err != nil {
		return nil, err
	}
	aggregates := make(map[string]*analyticsmodel.NodeAccessLogWAFIPAggregate)
	order := make([]string, 0, len(rows))
	for _, r := range rows {
		remoteAddr := strings.TrimSpace(r.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		aggregates[remoteAddr] = &analyticsmodel.NodeAccessLogWAFIPAggregate{
			RemoteAddr:       remoteAddr,
			RequestCount:     r.RequestCount,
			Status404Count:   r.Status404Count,
			ClientErrorCount: r.ClientErrorCount,
			ServerErrorCount: r.ServerErrorCount,
			IPHostCount:      0,
			LastSeenEpoch:    r.LastSeenEpoch,
			StatusCounts:     make(map[int]int64),
		}
		order = append(order, remoteAddr)
	}
	if len(aggregates) > 0 {
		if err := s.mergeWAFIPStatusCounts(ctx, f, aggregates); err != nil {
			return nil, err
		}
		if err := s.mergeWAFIPHostCounts(ctx, f, aggregates); err != nil {
			return nil, err
		}
	}
	result := make([]analyticsmodel.NodeAccessLogWAFIPAggregate, 0, len(order))
	for _, remoteAddr := range order {
		if a := aggregates[remoteAddr]; a != nil {
			result = append(result, *a)
		}
	}
	return result, nil
}

// mergeWAFIPStatusCounts 填充 WAF 每 IP 状态码分布。
func (s *gormLogStore) mergeWAFIPStatusCounts(ctx context.Context, f analyticsmodel.NodeAccessLogFilter, aggregates map[string]*analyticsmodel.NodeAccessLogWAFIPAggregate) error {
	type row struct {
		RemoteAddr  string
		StatusCode  int32
		StatusCount int64
	}
	var rows []row
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	q = q.Select("remote_addr, status_code, COUNT(*) AS status_count").
		Where("remote_addr != ''")
	if err := q.Group("remote_addr, status_code").Scan(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		a := aggregates[strings.TrimSpace(r.RemoteAddr)]
		if a == nil {
			continue
		}
		if a.StatusCounts == nil {
			a.StatusCounts = make(map[int]int64)
		}
		a.StatusCounts[int(r.StatusCode)] += r.StatusCount
	}
	return nil
}

// mergeWAFIPHostCounts 按 (remote_addr, host) 行数累加 IP 字面量 host 的访问行数。
func (s *gormLogStore) mergeWAFIPHostCounts(ctx context.Context, f analyticsmodel.NodeAccessLogFilter, aggregates map[string]*analyticsmodel.NodeAccessLogWAFIPAggregate) error {
	type row struct {
		RemoteAddr string
		Host       string
		RowCount   int64
	}
	var rows []row
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	q = q.Select("remote_addr, host, COUNT(*) AS row_count").
		Where("remote_addr != '' AND trim(host) != ''")
	if err := q.Group("remote_addr, host").Scan(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		a := aggregates[strings.TrimSpace(r.RemoteAddr)]
		if a == nil {
			continue
		}
		if isIPLiteralHost(r.Host) {
			a.IPHostCount += r.RowCount
		}
	}
	return nil
}

// IPTrend 按 IP × 时间桶聚合请求数。
func (s *gormLogStore) IPTrend(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogIPTrend, error) {
	f := toNodeAccessLogFilter(query)
	expr := timeBucketSQL(s.db, "logged_at", bucketSeconds)
	type row struct {
		BucketEpoch  int64
		RequestCount int64
	}
	var rows []row
	q := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), f)
	q = q.Select(expr + " AS bucket_epoch, COUNT(*) AS request_count")
	if err := q.Group(expr).Order("bucket_epoch ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.NodeAccessLogIPTrend, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.NodeAccessLogIPTrend{BucketEpoch: r.BucketEpoch, RequestCount: r.RequestCount}
	}
	return out, nil
}

func (s *gormLogStore) DeleteAll(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("1 = 1").Delete(&analyticsmodel.NodeAccessLog{})
	return res.RowsAffected, res.Error
}

func (s *gormLogStore) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("logged_at < ?", cutoff).Delete(&analyticsmodel.NodeAccessLog{})
	return res.RowsAffected, res.Error
}

func (s *gormLogStore) DeleteByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("node_id = ? AND logged_at < ?", nodeID, before).Delete(&analyticsmodel.NodeAccessLog{})
	return res.RowsAffected, res.Error
}

// ListForMigration 按 id 升序分页读取（迁移复制用）。
func (s *gormLogStore) ListForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeAccessLog, error) {
	var rows []analyticsmodel.NodeAccessLog
	q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}).
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limitOr(limit, migrationPageSize))
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// MigrationRange 返回 of_node_access_logs.logged_at 的最小/最大值（空表返回零值）。
// 用 ORDER BY ± LIMIT 1 经 GORM schema 扫描，SQLite/PG 方言时间转换一致。
func (s *gormLogStore) MigrationRange(ctx context.Context) (time.Time, time.Time, error) {
	return gormMigrationRange(ctx, s.db, "logged_at", analyticsmodel.NodeAccessLog{}, func(v *analyticsmodel.NodeAccessLog) time.Time {
		return v.LoggedAt
	})
}

// EnsurePartitions 幂等预建 PG 分区（按月）覆盖 [from, to] 月份；
// 非 PG 方言为 no-op（SQLite 无分区、CH 不走本实现）。
func (s *gormLogStore) EnsurePartitions(ctx context.Context, from, to time.Time) error {
	if !isPostgresDialect(s.db) {
		return nil
	}
	for _, sql := range partitionStatementsRange(from, to) {
		if err := s.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("ensure partition: %w", err)
		}
	}
	return nil
}

// gormMigrationRange 按时间列 ORDER BY ± LIMIT 1 取首尾记录（经 GORM schema 扫描，
// 避免 SQLite 时间存文本导致 MIN/MAX 原始 Scan 失败）；空表返回两个零值。
func gormMigrationRange[T any](
	ctx context.Context,
	gdb *gorm.DB,
	column string,
	model T,
	timeOf func(*T) time.Time,
) (time.Time, time.Time, error) {
	var first, last T
	found := false
	for _, order := range []string{"ASC", "DESC"} {
		out := &first
		if order == "DESC" {
			out = &last
		}
		res := gdb.WithContext(ctx).Model(model).Order(column + " " + order).Limit(1).Take(out)
		if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return time.Time{}, time.Time{}, fmt.Errorf("query migration range %s: %w", column, res.Error)
		}
		if res.Error == nil {
			found = true
		}
	}
	if !found {
		return time.Time{}, time.Time{}, nil
	}
	return timeOf(&first).UTC(), timeOf(&last).UTC(), nil
}

func limitOr(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// applyNodeAccessLogPagination 镜像 CH ListNodeAccessLogs/IPSummariesNodeAccessLogs 的 0-based 分页：
// 仅 PageSize>0 时加 LIMIT PageSize OFFSET Page*PageSize（Page<0 归零）；PageSize<=0 时与 CH 一致
// 不加 LIMIT/OFFSET，返回全部匹配行。
func applyNodeAccessLogPagination(q *gorm.DB, f analyticsmodel.NodeAccessLogFilter) *gorm.DB {
	if f.PageSize <= 0 {
		return q
	}
	if f.Page < 0 {
		f.Page = 0
	}
	return q.Limit(f.PageSize).Offset(f.Page * f.PageSize)
}

// offsetOf 返回用户访问日志分页偏移（1-based，镜像 CH ListAccessLogs：page<1 归 1、
// pageSize<1 用 20、offset=(page-1)*pageSize）。节点访问日志不经过本函数，走
// applyNodeAccessLogPagination（0-based，与 CH node_access_log 路径一致）。
func offsetOf(page, pageSize int) int {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return (page - 1) * pageSize
}

func nodeAccessLogValueColumn(column string) (string, bool) {
	switch column {
	case "remote_addr":
		return "remote_addr", true
	case "host":
		return "host", true
	case "path":
		return "path", true
	case "region":
		return "region", true
	case "status_code":
		return "status_code", true
	case "user_agent":
		return "user_agent", true
	case "cache_status":
		return "cache_status", true
	}
	return "", false
}

// toNodeAccessLogFilter 由 model 查询 DTO 转为 analytics 过滤结构。
func toNodeAccessLogFilter(query model.OpenFlareAccessLogQuery) analyticsmodel.NodeAccessLogFilter {
	return analyticsmodel.NodeAccessLogFilter{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Hosts:      query.Hosts,
		Path:       query.Path,
		Since:      query.Since,
		Until:      query.Until,
		Page:       query.Page,
		PageSize:   query.PageSize,
		SortBy:     query.SortBy,
		SortOrder:  query.SortOrder,
	}
}

// applyNodeAccessLogFilter 对齐 CH 过滤语义（node_access_log_filter.go）：
// node_id trim 后等值；remote_addr/host/path 前缀 LIKE；hosts 走 lower(trim(host)) IN（参数已归一化）；
// since 闭区间 >=；until 开区间 <。
func applyNodeAccessLogFilter(q *gorm.DB, f analyticsmodel.NodeAccessLogFilter) *gorm.DB {
	if nodeID := strings.TrimSpace(f.NodeID); nodeID != "" {
		q = q.Where("node_id = ?", nodeID)
	}
	if remoteAddr := strings.TrimSpace(f.RemoteAddr); remoteAddr != "" {
		q = q.Where("remote_addr LIKE ?", remoteAddr+"%")
	}
	hosts := normalizeNodeAccessLogHosts(f.Hosts)
	if len(hosts) > 0 {
		q = q.Where("lower(trim(host)) IN ?", hosts)
	} else if host := strings.TrimSpace(f.Host); host != "" {
		q = q.Where("host LIKE ?", host+"%")
	}
	if path := strings.TrimSpace(f.Path); path != "" {
		q = q.Where("path LIKE ?", path+"%")
	}
	if !f.Since.IsZero() {
		q = q.Where("logged_at >= ?", f.Since)
	}
	if !f.Until.IsZero() {
		q = q.Where("logged_at < ?", f.Until)
	}
	return q
}

// normalizeNodeAccessLogHosts 对 hosts 归一化：trim + lowercase + 去重去空。
func normalizeNodeAccessLogHosts(hosts []string) []string {
	if len(hosts) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(hosts))
	result := make([]string, 0, len(hosts))
	for _, host := range hosts {
		trimmed := strings.ToLower(strings.TrimSpace(host))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// isIPLiteralHost 判断 host 是否为 IP 字面量，镜像旧 CH hostIsIP 判定：
// 含 ':' 且不以 '[' 开头 → 取首个 ':' 前片段；否则去除所有 '['/']' 后经 net.ParseIP 校验。
func isIPLiteralHost(host string) bool {
	h := strings.TrimSpace(host)
	if h == "" {
		return false
	}
	if strings.Contains(h, ":") && !strings.HasPrefix(h, "[") {
		h, _, _ = strings.Cut(h, ":")
	} else {
		h = strings.NewReplacer("[", "", "]", "").Replace(h)
	}
	return net.ParseIP(strings.TrimSpace(h)) != nil
}

// toAnalyticsNodeAccessLog 将业务模型转为 analytics 落库模型（含 math 边界保护）。
func toAnalyticsNodeAccessLog(record *model.OpenFlareAccessLog) analyticsmodel.NodeAccessLog {
	var bytesSent uint64
	if record.BytesSent > 0 {
		bytesSent = uint64(record.BytesSent)
	}
	var requestLength uint64
	if record.RequestLength > 0 {
		requestLength = uint64(record.RequestLength)
	}
	var requestTimeMs uint32
	if record.RequestTimeMs > 0 && record.RequestTimeMs <= int64(math.MaxUint32) {
		requestTimeMs = uint32(record.RequestTimeMs)
	}
	statusCode := record.StatusCode
	switch {
	case statusCode > math.MaxInt32:
		statusCode = math.MaxInt32
	case statusCode < math.MinInt32:
		statusCode = math.MinInt32
	}
	return analyticsmodel.NodeAccessLog{
		ID:            record.ID,
		NodeID:        record.NodeID,
		LoggedAt:      record.LoggedAt,
		RemoteAddr:    record.RemoteAddr,
		Region:        record.Region,
		Host:          record.Host,
		Path:          record.Path,
		UserAgent:     record.UserAgent,
		CacheStatus:   record.CacheStatus,
		StatusCode:    int32(statusCode),
		BytesSent:     bytesSent,
		RequestLength: requestLength,
		RequestTimeMs: requestTimeMs,
		CreatedAt:     record.CreatedAt,
	}
}

// fromAnalyticsNodeAccessLogs 将 analytics 落库模型转回业务模型（含 math 边界保护）。
func fromAnalyticsNodeAccessLogs(rows []analyticsmodel.NodeAccessLog) []*model.OpenFlareAccessLog {
	result := make([]*model.OpenFlareAccessLog, len(rows))
	for index, row := range rows {
		var bytesSent int64
		if row.BytesSent <= math.MaxInt64 {
			bytesSent = int64(row.BytesSent)
		} else {
			bytesSent = math.MaxInt64
		}
		var requestLength int64
		if row.RequestLength <= math.MaxInt64 {
			requestLength = int64(row.RequestLength)
		} else {
			requestLength = math.MaxInt64
		}
		result[index] = &model.OpenFlareAccessLog{
			ID:            row.ID,
			NodeID:        row.NodeID,
			LoggedAt:      row.LoggedAt,
			RemoteAddr:    row.RemoteAddr,
			Region:        row.Region,
			Host:          row.Host,
			Path:          row.Path,
			UserAgent:     row.UserAgent,
			CacheStatus:   row.CacheStatus,
			StatusCode:    int(row.StatusCode),
			BytesSent:     bytesSent,
			RequestLength: requestLength,
			RequestTimeMs: int64(row.RequestTimeMs),
			CreatedAt:     row.CreatedAt,
		}
	}
	return result
}

// ---- ObservabilityStore ----

// InsertMetricSnapshot 写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
func (s *gormLogStore) InsertMetricSnapshot(ctx context.Context, record *model.OpenFlareMetricSnapshot) error {
	if record == nil {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	if h := currentObservabilityHooks().QueueMetricSnapshot; h != nil {
		h(toAnalyticsNodeMetricSnapshot(record))
	}
	return nil
}

func (s *gormLogStore) ListMetricSnapshots(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareMetricSnapshot, error) {
	var rows []analyticsmodel.NodeMetricSnapshot
	q := nodeIDScope(s.db.WithContext(ctx), nodeID).Where("captured_at >= ?", since).Order("captured_at DESC, id DESC")
	if err := q.Limit(limitOr(limit, migrationPageSize)).Find(&rows).Error; err != nil {
		return nil, err
	}
	return fromAnalyticsNodeMetricSnapshots(rows), nil
}

func (s *gormLogStore) DeleteAllMetricSnapshots(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("1 = 1").Delete(&analyticsmodel.NodeMetricSnapshot{})
	return res.RowsAffected, res.Error
}

func (s *gormLogStore) DeleteMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("captured_at < ?", cutoff).Delete(&analyticsmodel.NodeMetricSnapshot{})
	return res.RowsAffected, res.Error
}

// BatchInsertNodeMetricSnapshots 是 batchwriter flush 目标：GORM 分批落库。
func (s *gormLogStore) BatchInsertNodeMetricSnapshots(ctx context.Context, rows []analyticsmodel.NodeMetricSnapshot) error {
	if len(rows) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return s.db.WithContext(ctx).CreateInBatches(rows, insertBatchSize).Error
}

// ListTrafficHourly 按小时从 of_node_access_logs 实时聚合，对齐 CH ListNodeTrafficHourly 口径：
// request_count=COUNT(*)、error_count=5xx、unique_visitor_count 恒 0（UV 需 raw uniqExact）。
func (s *gormLogStore) ListTrafficHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.NodeTrafficHourly, error) {
	expr := timeBucketSQL(s.db, "logged_at", hourBucketSeconds)
	type row struct {
		NodeID       string
		HourEpoch    int64
		RequestCount int64
		ErrorCount   int64
	}
	var rows []row
	q := nodeIDScope(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), strings.TrimSpace(nodeID))
	if !since.IsZero() {
		q = q.Where("logged_at >= ?", since.UTC())
	}
	q = q.Select(
		"node_id, " + expr + " AS hour_epoch, " +
			"COUNT(*) AS request_count, " +
			"COUNT(*) FILTER (WHERE status_code >= 500) AS error_count",
	)
	if err := q.Group("node_id, " + expr).Order("hour_epoch ASC, node_id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.NodeTrafficHourly, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.NodeTrafficHourly{
			NodeID:             r.NodeID,
			Hour:               time.Unix(r.HourEpoch, 0).UTC(),
			RequestCount:       r.RequestCount,
			ErrorCount:         r.ErrorCount,
			UniqueVisitorCount: 0,
		}
	}
	return out, nil
}

// ListAccessLogHourly 按 node/hour/host 从 of_node_access_logs 实时聚合，
// 对齐 CH of_access_log_hourly 字段（error_count=5xx、bytes_sent/request_length 求和）。
func (s *gormLogStore) ListAccessLogHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.AccessLogHourly, error) {
	expr := timeBucketSQL(s.db, "logged_at", hourBucketSeconds)
	type row struct {
		NodeID        string
		HourEpoch     int64
		Host          string
		RequestCount  int64
		ErrorCount    int64
		BytesSent     int64
		RequestLength int64
	}
	var rows []row
	q := nodeIDScope(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), strings.TrimSpace(nodeID))
	if !since.IsZero() {
		q = q.Where("logged_at >= ?", since.UTC())
	}
	q = q.Select(
		"node_id, " + expr + " AS hour_epoch, host, " +
			"COUNT(*) AS request_count, " +
			"COUNT(*) FILTER (WHERE status_code >= 500) AS error_count, " +
			"COALESCE(SUM(bytes_sent),0) AS bytes_sent, " +
			"COALESCE(SUM(request_length),0) AS request_length",
	)
	if err := q.Group("node_id, host, " + expr).Order("hour_epoch ASC, node_id ASC, host ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.AccessLogHourly, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.AccessLogHourly{
			NodeID:        r.NodeID,
			Hour:          time.Unix(r.HourEpoch, 0).UTC(),
			Host:          r.Host,
			RequestCount:  r.RequestCount,
			ErrorCount:    r.ErrorCount,
			BytesSent:     r.BytesSent,
			RequestLength: r.RequestLength,
		}
	}
	return out, nil
}

// ListMetricHourly 按小时从 of_node_metric_snapshots 实时聚合，口径对齐 CH raw 兜底
// listNodeMetricHourlyFromRaw：avg cpu/memory 用量、每节点相邻采样计数器增量
// （LAG 按 captured_at,id 排序；负增量按 0 丢弃）、reported_nodes=distinct node_id。
func (s *gormLogStore) ListMetricHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.NodeMetricHourly, error) {
	expr := timeBucketSQL(s.db, "captured_at", hourBucketSeconds)
	where := "1 = 1"
	var args []any
	if trimmed := strings.TrimSpace(nodeID); trimmed != "" {
		where += " AND node_id = ?"
		args = append(args, trimmed)
	}
	if !since.IsZero() {
		where += " AND captured_at >= ?"
		args = append(args, since.UTC())
	}
	counterDelta := func(col string) string {
		lag := "LAG(" + col + ", 1, " + col + ") OVER (PARTITION BY node_id ORDER BY captured_at, id)"
		return "CASE WHEN " + col + " - " + lag + " < 0 THEN 0 ELSE " + col + " - " + lag + " END"
	}
	sql := `
SELECT hour_epoch,
	AVG(cpu_usage_percent) AS average_cpu_usage_percent,
	AVG(memory_usage_percent) AS average_memory_usage_percent,
	SUM(rx_delta) AS network_rx_bytes,
	SUM(tx_delta) AS network_tx_bytes,
	SUM(read_delta) AS disk_read_bytes,
	SUM(write_delta) AS disk_write_bytes,
	COUNT(DISTINCT node_id) AS reported_nodes
FROM (
	SELECT node_id, ` + expr + ` AS hour_epoch,
		cpu_usage_percent,
		CASE WHEN memory_total_bytes > 0 THEN (memory_used_bytes * 100.0) / memory_total_bytes ELSE 0 END AS memory_usage_percent,
		` + counterDelta("network_rx_bytes") + ` AS rx_delta,
		` + counterDelta("network_tx_bytes") + ` AS tx_delta,
		` + counterDelta("disk_read_bytes") + ` AS read_delta,
		` + counterDelta("disk_write_bytes") + ` AS write_delta
	FROM of_node_metric_snapshots
	WHERE ` + where + `
)
GROUP BY hour_epoch
ORDER BY hour_epoch ASC`
	type row struct {
		HourEpoch                 int64
		AverageCPUUsagePercent    float64
		AverageMemoryUsagePercent float64
		NetworkRxBytes            int64
		NetworkTxBytes            int64
		DiskReadBytes             int64
		DiskWriteBytes            int64
		ReportedNodes             int64
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.NodeMetricHourly, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.NodeMetricHourly{
			Hour:                      time.Unix(r.HourEpoch, 0).UTC(),
			AverageCPUUsagePercent:    r.AverageCPUUsagePercent,
			AverageMemoryUsagePercent: r.AverageMemoryUsagePercent,
			NetworkRxBytes:            r.NetworkRxBytes,
			NetworkTxBytes:            r.NetworkTxBytes,
			DiskReadBytes:             r.DiskReadBytes,
			DiskWriteBytes:            r.DiskWriteBytes,
			ReportedNodes:             int(r.ReportedNodes),
		}
	}
	return out, nil
}

// InsertEdgeHealth 写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
func (s *gormLogStore) InsertEdgeHealth(ctx context.Context, record *model.OpenFlareEdgeHealth) error {
	if record == nil {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	if h := currentObservabilityHooks().QueueEdgeHealth; h != nil {
		h(toAnalyticsNodeEdgeHealth(record))
	}
	return nil
}

func (s *gormLogStore) ListEdgeHealth(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareEdgeHealth, error) {
	var rows []analyticsmodel.NodeEdgeHealth
	if err := nodeIDScope(s.db.WithContext(ctx), nodeID).Where("captured_at >= ?", since).
		Order("captured_at DESC, id DESC").Limit(limitOr(limit, migrationPageSize)).Find(&rows).Error; err != nil {
		return nil, err
	}
	return fromAnalyticsNodeEdgeHealths(rows), nil
}

func (s *gormLogStore) DeleteAllEdgeHealth(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("1 = 1").Delete(&analyticsmodel.NodeEdgeHealth{})
	return res.RowsAffected, res.Error
}

func (s *gormLogStore) DeleteEdgeHealthBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("captured_at < ?", cutoff).Delete(&analyticsmodel.NodeEdgeHealth{})
	return res.RowsAffected, res.Error
}

// BatchInsertNodeEdgeHealth 是 batchwriter flush 目标：GORM 分批落库。
func (s *gormLogStore) BatchInsertNodeEdgeHealth(ctx context.Context, rows []analyticsmodel.NodeEdgeHealth) error {
	if len(rows) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return s.db.WithContext(ctx).CreateInBatches(rows, insertBatchSize).Error
}

// InsertNodeObservationFrps 写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
func (s *gormLogStore) InsertNodeObservationFrps(ctx context.Context, record *model.OpenFlareNodeObservationFrps) error {
	if record == nil {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	if h := currentObservabilityHooks().QueueNodeObsFrps; h != nil {
		h(toAnalyticsNodeObsFrps(record))
	}
	return nil
}

func (s *gormLogStore) ListNodeObservationFrps(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrps, error) {
	var rows []analyticsmodel.NodeObsFrps
	if err := nodeIDScope(s.db.WithContext(ctx), nodeID).Where("captured_at >= ?", since).
		Order("captured_at DESC, id DESC").Limit(limitOr(limit, migrationPageSize)).Find(&rows).Error; err != nil {
		return nil, err
	}
	return fromAnalyticsNodeObsFrps(rows), nil
}

func (s *gormLogStore) DeleteAllNodeObservationFrps(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("1 = 1").Delete(&analyticsmodel.NodeObsFrps{})
	return res.RowsAffected, res.Error
}

func (s *gormLogStore) DeleteNodeObservationFrpsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("captured_at < ?", cutoff).Delete(&analyticsmodel.NodeObsFrps{})
	return res.RowsAffected, res.Error
}

// BatchInsertNodeObsFrps 是 batchwriter flush 目标：GORM 分批落库。
func (s *gormLogStore) BatchInsertNodeObsFrps(ctx context.Context, rows []analyticsmodel.NodeObsFrps) error {
	if len(rows) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return s.db.WithContext(ctx).CreateInBatches(rows, insertBatchSize).Error
}

// InsertNodeObservationFrpc 写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
func (s *gormLogStore) InsertNodeObservationFrpc(ctx context.Context, record *model.OpenFlareNodeObservationFrpc) error {
	if record == nil {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	if h := currentObservabilityHooks().QueueNodeObsFrpc; h != nil {
		h(toAnalyticsNodeObsFrpc(record))
	}
	return nil
}

func (s *gormLogStore) ListNodeObservationFrpc(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrpc, error) {
	var rows []analyticsmodel.NodeObsFrpc
	if err := nodeIDScope(s.db.WithContext(ctx), nodeID).Where("captured_at >= ?", since).
		Order("captured_at DESC, id DESC").Limit(limitOr(limit, migrationPageSize)).Find(&rows).Error; err != nil {
		return nil, err
	}
	return fromAnalyticsNodeObsFrpc(rows), nil
}

func (s *gormLogStore) DeleteAllNodeObservationFrpc(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("1 = 1").Delete(&analyticsmodel.NodeObsFrpc{})
	return res.RowsAffected, res.Error
}

func (s *gormLogStore) DeleteNodeObservationFrpcBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("captured_at < ?", cutoff).Delete(&analyticsmodel.NodeObsFrpc{})
	return res.RowsAffected, res.Error
}

// BatchInsertNodeObsFrpc 是 batchwriter flush 目标：GORM 分批落库。
func (s *gormLogStore) BatchInsertNodeObsFrpc(ctx context.Context, rows []analyticsmodel.NodeObsFrpc) error {
	if len(rows) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return s.db.WithContext(ctx).CreateInBatches(rows, insertBatchSize).Error
}

// ListMetricSnapshotsForMigration 按 id 升序分页读取（迁移复制用）。
func (s *gormLogStore) ListMetricSnapshotsForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeMetricSnapshot, error) {
	var rows []analyticsmodel.NodeMetricSnapshot
	q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeMetricSnapshot{}).
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limitOr(limit, migrationPageSize))
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListEdgeHealthForMigration 按 id 升序分页读取（迁移复制用）。
func (s *gormLogStore) ListEdgeHealthForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeEdgeHealth, error) {
	var rows []analyticsmodel.NodeEdgeHealth
	q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeEdgeHealth{}).
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limitOr(limit, migrationPageSize))
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListNodeObsFrpsForMigration 按 id 升序分页读取（迁移复制用）。
func (s *gormLogStore) ListNodeObsFrpsForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeObsFrps, error) {
	var rows []analyticsmodel.NodeObsFrps
	q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeObsFrps{}).
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limitOr(limit, migrationPageSize))
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListNodeObsFrpcForMigration 按 id 升序分页读取（迁移复制用）。
func (s *gormLogStore) ListNodeObsFrpcForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeObsFrpc, error) {
	var rows []analyticsmodel.NodeObsFrpc
	q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeObsFrpc{}).
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limitOr(limit, migrationPageSize))
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ---- UserAccessLogStore ----

// BatchInsert 是 batchwriter flush 目标：GORM 分批落库。
func (s *userAccessLogGormStore) BatchInsert(ctx context.Context, logs []analyticsmodel.UserAccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return s.db.WithContext(ctx).CreateInBatches(logs, insertBatchSize).Error
}

// DeleteAll 清空全部用户访问日志（迁移「覆盖目标库已有日志」幂等前提用）。
func (s *userAccessLogGormStore) DeleteAll(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("1 = 1").Delete(&analyticsmodel.UserAccessLog{})
	return res.RowsAffected, res.Error
}

// ListForMigration 按 id 升序分页读取（迁移复制用）。
func (s *userAccessLogGormStore) ListForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.UserAccessLog, error) {
	var rows []analyticsmodel.UserAccessLog
	q := s.db.WithContext(ctx).Model(&analyticsmodel.UserAccessLog{}).
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limitOr(limit, migrationPageSize))
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// MigrationRange 返回 w_user_access_logs.created_at 的最小/最大值（空表返回零值）。
func (s *userAccessLogGormStore) MigrationRange(ctx context.Context) (time.Time, time.Time, error) {
	return gormMigrationRange(ctx, s.db, "created_at", analyticsmodel.UserAccessLog{}, func(v *analyticsmodel.UserAccessLog) time.Time {
		return v.CreatedAt
	})
}

func (s *userAccessLogGormStore) Count(ctx context.Context, filter analyticsmodel.AccessLogFilter) (uint64, error) {
	where, args, ok := buildUserAccessLogWhere(filter)
	if !ok {
		return 0, nil
	}
	var total int64
	if err := s.db.WithContext(ctx).Model(&analyticsmodel.UserAccessLog{}).Where(where, args...).Count(&total).Error; err != nil {
		return 0, err
	}
	return countToUint64(total), nil
}

func (s *userAccessLogGormStore) List(ctx context.Context, filter analyticsmodel.AccessLogFilter, page, pageSize int) ([]analyticsmodel.UserAccessLog, uint64, error) {
	where, args, ok := buildUserAccessLogWhere(filter)
	if !ok {
		return []analyticsmodel.UserAccessLog{}, 0, nil
	}
	var total int64
	if err := s.db.WithContext(ctx).Model(&analyticsmodel.UserAccessLog{}).Where(where, args...).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []analyticsmodel.UserAccessLog{}, 0, nil
	}
	var rows []analyticsmodel.UserAccessLog
	q := s.db.WithContext(ctx).Where(where, args...).Order("created_at DESC, id DESC")
	if err := q.Limit(limitOr(pageSize, defaultPageSize)).Offset(offsetOf(page, pageSize)).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, countToUint64(total), nil
}

// buildUserAccessLogWhere 构建用户访问日志过滤条件（Count/List 共用，保证口径一致）。
// 镜像 CH buildUserAccessLogFilterClause：user_id IN、path LIKE '%trim(path)%'、
// StartTime 闭区间 >=、EndTime 闭区间 <=（与 CH 一致）；空非 nil UserIDs 视为无匹配（ok=false）。
func buildUserAccessLogWhere(filter analyticsmodel.AccessLogFilter) (string, []any, bool) {
	if filter.UserIDs != nil && len(filter.UserIDs) == 0 {
		return "", nil, false
	}
	var parts []string
	var args []any
	if filter.UserIDs != nil {
		parts = append(parts, "user_id IN ?")
		args = append(args, filter.UserIDs)
	}
	if trimmed := strings.TrimSpace(filter.Path); trimmed != "" {
		parts = append(parts, "path LIKE ?")
		args = append(args, "%"+trimmed+"%")
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
		// GORM 对裸字符串 "1" 会误判为按主键查询，统一用显式恒真条件。
		return "1 = 1", args, true
	}
	return strings.Join(parts, " AND "), args, true
}

func (s *userAccessLogGormStore) GetDailyTrend(ctx context.Context, days int) ([]analyticsmodel.DailyTrend, error) {
	if days <= 0 {
		days = 7
	}
	// 镜像 CH access_log_stats.go：起点 = (days-1) 天前当日零点；必须返回恰好 days 个日历日并补零。
	start := time.Now().AddDate(0, 0, -(days - 1)).Truncate(dayDuration)
	type row struct {
		Date string
		Cnt  uint64
	}
	var rows []row
	err := s.db.WithContext(ctx).Model(&analyticsmodel.UserAccessLog{}).
		Select(dailyTrendDateSQL(s.db)+" AS date, COUNT(*) AS cnt").
		Where("created_at >= ?", start).
		Group("date").Order("date ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]uint64, len(rows))
	for _, r := range rows {
		counts[r.Date] = r.Cnt
	}
	out := make([]analyticsmodel.DailyTrend, 0, days)
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		out = append(out, analyticsmodel.DailyTrend{Date: d, Count: counts[d]})
	}
	return out, nil
}

func (s *userAccessLogGormStore) GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]analyticsmodel.BrowserShare, error) {
	return s.userAgentGroupCount(ctx, startTime, "browser")
}

// userAgentGroupCount 按 user_agent 分组后在 Go 侧按组分类（browser/os/device）聚合，
// 与旧 CH GetBrowserDistribution 统计口径一致（先按 user_agent 分组，再分类求和）。
func (s *gormLogStore) userAgentGroupCount(ctx context.Context, startTime time.Time, group string) ([]analyticsmodel.BrowserShare, error) {
	type row struct {
		UserAgent string
		Cnt       uint64
	}
	var rows []row
	err := s.db.WithContext(ctx).Model(&analyticsmodel.UserAccessLog{}).
		Select("user_agent, COUNT(*) AS cnt").
		Where("created_at >= ?", startTime).
		Group("user_agent").Order("cnt DESC").Limit(topUserAgents).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]uint64)
	for _, r := range rows {
		var label string
		switch group {
		case "os":
			label = analyticsmodel.ParseOSName(r.UserAgent)
		case "device":
			label = analyticsmodel.ParseDeviceType(r.UserAgent)
		default:
			label = analyticsmodel.ParseBrowserName(r.UserAgent)
		}
		counts[label] += r.Cnt
	}
	out := make([]analyticsmodel.BrowserShare, 0, len(counts))
	for label, count := range counts {
		out = append(out, analyticsmodel.BrowserShare{Browser: label, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out, nil
}

func (s *userAccessLogGormStore) GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]analyticsmodel.TopUser, error) {
	type row struct {
		UserID uint64
		Cnt    uint64
	}
	var rows []row
	err := s.db.WithContext(ctx).Model(&analyticsmodel.UserAccessLog{}).
		Select("user_id, COUNT(*) AS cnt").
		Where("user_id <> 0 AND created_at >= ?", startTime).
		Group("user_id").Order("cnt DESC").Limit(limitOr(limit, defaultTopN)).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.TopUser, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.TopUser{UserID: r.UserID, Count: r.Cnt}
	}
	return out, nil
}

// ---- ObservabilityStore 转换辅助（从旧 openflare_observability_store.go 复制） ----

const edgeHealthStatusUnknown = "unknown"

func normalizeEdgeHealthStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return edgeHealthStatusUnknown
	}
	return status
}

func toAnalyticsNodeMetricSnapshot(record *model.OpenFlareMetricSnapshot) analyticsmodel.NodeMetricSnapshot {
	return analyticsmodel.NodeMetricSnapshot{
		ID:                uint64(record.ID),
		NodeID:            record.NodeID,
		CapturedAt:        record.CapturedAt,
		CPUUsagePercent:   record.CPUUsagePercent,
		MemoryUsedBytes:   record.MemoryUsedBytes,
		MemoryTotalBytes:  record.MemoryTotalBytes,
		StorageUsedBytes:  record.StorageUsedBytes,
		StorageTotalBytes: record.StorageTotalBytes,
		DiskReadBytes:     record.DiskReadBytes,
		DiskWriteBytes:    record.DiskWriteBytes,
		NetworkRxBytes:    record.NetworkRxBytes,
		NetworkTxBytes:    record.NetworkTxBytes,
		CreatedAt:         record.CreatedAt,
	}
}

func fromAnalyticsNodeMetricSnapshots(rows []analyticsmodel.NodeMetricSnapshot) []*model.OpenFlareMetricSnapshot {
	result := make([]*model.OpenFlareMetricSnapshot, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareMetricSnapshot{
			ID:                uint(row.ID),
			NodeID:            row.NodeID,
			CapturedAt:        row.CapturedAt,
			CPUUsagePercent:   row.CPUUsagePercent,
			MemoryUsedBytes:   row.MemoryUsedBytes,
			MemoryTotalBytes:  row.MemoryTotalBytes,
			StorageUsedBytes:  row.StorageUsedBytes,
			StorageTotalBytes: row.StorageTotalBytes,
			DiskReadBytes:     row.DiskReadBytes,
			DiskWriteBytes:    row.DiskWriteBytes,
			NetworkRxBytes:    row.NetworkRxBytes,
			NetworkTxBytes:    row.NetworkTxBytes,
			CreatedAt:         row.CreatedAt,
		}
	}
	return result
}

func toAnalyticsNodeEdgeHealth(record *model.OpenFlareEdgeHealth) analyticsmodel.NodeEdgeHealth {
	return analyticsmodel.NodeEdgeHealth{
		ID:          uint64(record.ID),
		NodeID:      record.NodeID,
		CapturedAt:  record.CapturedAt,
		Status:      normalizeEdgeHealthStatus(record.Status),
		Connections: record.Connections,
		CreatedAt:   record.CreatedAt,
	}
}

func fromAnalyticsNodeEdgeHealths(rows []analyticsmodel.NodeEdgeHealth) []*model.OpenFlareEdgeHealth {
	result := make([]*model.OpenFlareEdgeHealth, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareEdgeHealth{
			ID:          uint(row.ID),
			NodeID:      row.NodeID,
			CapturedAt:  row.CapturedAt,
			Status:      normalizeEdgeHealthStatus(row.Status),
			Connections: row.Connections,
			CreatedAt:   row.CreatedAt,
		}
	}
	return result
}

func toAnalyticsNodeObsFrps(record *model.OpenFlareNodeObservationFrps) analyticsmodel.NodeObsFrps {
	return analyticsmodel.NodeObsFrps{
		ID:              uint64(record.ID),
		NodeID:          record.NodeID,
		CapturedAt:      record.CapturedAt,
		FrpsConnections: openFlareObservabilityIntToInt32(record.FrpsConnections),
		FrpsProxyCount:  openFlareObservabilityIntToInt32(record.FrpsProxyCount),
		FrpsClientCount: openFlareObservabilityIntToInt32(record.FrpsClientCount),
		FrpsProxies:     record.FrpsProxies,
		CreatedAt:       record.CreatedAt,
	}
}

func fromAnalyticsNodeObsFrps(rows []analyticsmodel.NodeObsFrps) []*model.OpenFlareNodeObservationFrps {
	result := make([]*model.OpenFlareNodeObservationFrps, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareNodeObservationFrps{
			ID:              uint(row.ID),
			NodeID:          row.NodeID,
			CapturedAt:      row.CapturedAt,
			FrpsConnections: int(row.FrpsConnections),
			FrpsProxyCount:  int(row.FrpsProxyCount),
			FrpsClientCount: int(row.FrpsClientCount),
			FrpsProxies:     row.FrpsProxies,
			CreatedAt:       row.CreatedAt,
		}
	}
	return result
}

func toAnalyticsNodeObsFrpc(record *model.OpenFlareNodeObservationFrpc) analyticsmodel.NodeObsFrpc {
	return analyticsmodel.NodeObsFrpc{
		ID:                   uint64(record.ID),
		NodeID:               record.NodeID,
		CapturedAt:           record.CapturedAt,
		TunnelStatus:         record.TunnelStatus,
		ConnectedRelaysCount: openFlareObservabilityIntToInt32(record.ConnectedRelaysCount),
		CreatedAt:            record.CreatedAt,
	}
}

func openFlareObservabilityIntToInt32(value int) int32 {
	switch {
	case value > math.MaxInt32:
		return math.MaxInt32
	case value < math.MinInt32:
		return math.MinInt32
	default:
		return int32(value)
	}
}

func fromAnalyticsNodeObsFrpc(rows []analyticsmodel.NodeObsFrpc) []*model.OpenFlareNodeObservationFrpc {
	result := make([]*model.OpenFlareNodeObservationFrpc, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareNodeObservationFrpc{
			ID:                   uint(row.ID),
			NodeID:               row.NodeID,
			CapturedAt:           row.CapturedAt,
			TunnelStatus:         row.TunnelStatus,
			ConnectedRelaysCount: int(row.ConnectedRelaysCount),
			CreatedAt:            row.CreatedAt,
		}
	}
	return result
}
