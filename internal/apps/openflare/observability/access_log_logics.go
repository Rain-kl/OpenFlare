// Package observability provides monitoring, metrics, and access log analysis for OpenFlare.
package observability

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	defaultAccessLogPageSize   = 20
	maxAccessLogPageSize       = 200
	defaultAccessLogSortBy     = "logged_at"
	defaultAccessLogSortOrder  = "desc"
	accessLogSortOrderAsc      = "asc"
	defaultAccessLogFoldMinute = 3
	defaultIPTrendHours        = 24
	defaultIPTrendBucketMinute = 30
	maxIPTrendHours            = 168
	nodeAccessLogRetentionDays = 90
	defaultAccessLogQueryDays  = 7
	accessLogFieldRemoteAddr   = "remote_addr"
	accessLogFieldRequestCount = "request_count"
)

var defaultAccessLogQueryWindow = defaultAccessLogQueryDays * 24 * time.Hour

// AccessLogQuery filters access log list queries.
type AccessLogQuery struct {
	NodeID      string `json:"node_id"`
	RemoteAddr  string `json:"remote_addr"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	Page        int    `json:"page"`
	PageSize    int    `json:"page_size"`
	SortBy      string `json:"sort_by"`
	SortOrder   string `json:"sort_order"`
	FoldMinutes int    `json:"fold_minutes"`
}

// AccessLogView is a single access log row.
type AccessLogView struct {
	ID         string    `json:"id"`
	NodeID     string    `json:"node_id"`
	NodeName   string    `json:"node_name"`
	LoggedAt   time.Time `json:"logged_at"`
	RemoteAddr string    `json:"remote_addr"`
	Region     string    `json:"region"`
	Host       string    `json:"host"`
	Path       string    `json:"path"`
	UserAgent  string    `json:"user_agent"`
	StatusCode int       `json:"status_code"`
}

// AccessLogList is a paginated access log response.
type AccessLogList struct {
	Items       []AccessLogView `json:"items"`
	Page        int             `json:"page"`
	PageSize    int             `json:"page_size"`
	HasMore     bool            `json:"has_more"`
	TotalRecord int64           `json:"total_record"`
	TotalIP     int64           `json:"total_ip"`
}

// FoldedAccessLogView is a folded access log bucket.
type FoldedAccessLogView struct {
	BucketStartedAt  time.Time `json:"bucket_started_at"`
	RequestCount     int64     `json:"request_count"`
	UniqueIPCount    int64     `json:"unique_ip_count"`
	UniqueHostCount  int64     `json:"unique_host_count"`
	SuccessCount     int64     `json:"success_count"`
	ClientErrorCount int64     `json:"client_error_count"`
	ServerErrorCount int64     `json:"server_error_count"`
}

// FoldedAccessLogList is a paginated folded access log response.
type FoldedAccessLogList struct {
	Items       []FoldedAccessLogView `json:"items"`
	Page        int                   `json:"page"`
	PageSize    int                   `json:"page_size"`
	HasMore     bool                  `json:"has_more"`
	TotalBucket int64                 `json:"total_bucket"`
	TotalRecord int64                 `json:"total_record"`
	TotalIP     int64                 `json:"total_ip"`
	FoldMinutes int                   `json:"fold_minutes"`
}

// FoldedAccessLogIPQuery filters folded IP summary queries.
type FoldedAccessLogIPQuery struct {
	NodeID          string `json:"node_id"`
	RemoteAddr      string `json:"remote_addr"`
	Host            string `json:"host"`
	Path            string `json:"path"`
	BucketStartedAt string `json:"bucket_started_at"`
	FoldMinutes     int    `json:"fold_minutes"`
	Page            int    `json:"page"`
	PageSize        int    `json:"page_size"`
	SortBy          string `json:"sort_by"`
	SortOrder       string `json:"sort_order"`
}

// FoldedAccessLogIPView is a folded IP row.
type FoldedAccessLogIPView struct {
	RemoteAddr       string    `json:"remote_addr"`
	RequestCount     int64     `json:"request_count"`
	SuccessCount     int64     `json:"success_count"`
	ClientErrorCount int64     `json:"client_error_count"`
	ServerErrorCount int64     `json:"server_error_count"`
	LastSeenAt       time.Time `json:"last_seen_at"`
}

// FoldedAccessLogIPList is a paginated folded IP response.
type FoldedAccessLogIPList struct {
	Items           []FoldedAccessLogIPView `json:"items"`
	Page            int                     `json:"page"`
	PageSize        int                     `json:"page_size"`
	HasMore         bool                    `json:"has_more"`
	TotalIP         int64                   `json:"total_ip"`
	BucketStartedAt time.Time               `json:"bucket_started_at"`
	FoldMinutes     int                     `json:"fold_minutes"`
	SortBy          string                  `json:"sort_by"`
	SortOrder       string                  `json:"sort_order"`
}

// AccessLogIPSummaryQuery filters IP summary list queries.
type AccessLogIPSummaryQuery struct {
	NodeID     string `json:"node_id"`
	RemoteAddr string `json:"remote_addr"`
	Host       string `json:"host"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	SortBy     string `json:"sort_by"`
	SortOrder  string `json:"sort_order"`
}

// AccessLogIPSummaryView is an IP summary row.
type AccessLogIPSummaryView struct {
	RemoteAddr     string    `json:"remote_addr"`
	TotalRequests  int64     `json:"total_requests"`
	RecentRequests int64     `json:"recent_requests"`
	LastSeenAt     time.Time `json:"last_seen_at"`
}

// AccessLogIPSummaryList is a paginated IP summary response.
type AccessLogIPSummaryList struct {
	Items     []AccessLogIPSummaryView `json:"items"`
	Page      int                      `json:"page"`
	PageSize  int                      `json:"page_size"`
	HasMore   bool                     `json:"has_more"`
	TotalIP   int64                    `json:"total_ip"`
	SortBy    string                   `json:"sort_by"`
	SortOrder string                   `json:"sort_order"`
}

// AccessLogIPTrendQuery filters IP trend queries.
type AccessLogIPTrendQuery struct {
	NodeID        string `json:"node_id"`
	RemoteAddr    string `json:"remote_addr"`
	Host          string `json:"host"`
	Hours         int    `json:"hours"`
	BucketMinutes int    `json:"bucket_minutes"`
}

// AccessLogIPTrendPoint is an IP trend bucket.
type AccessLogIPTrendPoint struct {
	BucketStartedAt time.Time `json:"bucket_started_at"`
	RequestCount    int64     `json:"request_count"`
}

// AccessLogIPTrendView is the IP trend response.
type AccessLogIPTrendView struct {
	RemoteAddr    string                  `json:"remote_addr"`
	Hours         int                     `json:"hours"`
	BucketMinutes int                     `json:"bucket_minutes"`
	Points        []AccessLogIPTrendPoint `json:"points"`
}

// AccessLogOverviewQuery filters access log overview queries.
type AccessLogOverviewQuery struct {
	NodeID string `json:"node_id"`
	Host   string `json:"host"`
	Hours  int    `json:"hours"`
}

// AccessLogOverviewMetricPoint is a single overview trend bucket.
type AccessLogOverviewMetricPoint struct {
	BucketStartedAt time.Time `json:"bucket_started_at"`
	Value           int64     `json:"value"`
}

// AccessLogOverviewSummary is the headline totals for the overview window.
type AccessLogOverviewSummary struct {
	TotalRequests   int64 `json:"total_requests"`
	TotalVisits     int64 `json:"total_visits"`
	BandwidthServed int64 `json:"bandwidth_served"`
}

// AccessLogOverviewTrends groups sparkline/series data for the overview.
type AccessLogOverviewTrends struct {
	Requests  []AccessLogOverviewMetricPoint `json:"requests"`
	Visits    []AccessLogOverviewMetricPoint `json:"visits"`
	Bandwidth []AccessLogOverviewMetricPoint `json:"bandwidth"`
}

// AccessLogOverview is the access-log analytics overview payload.
type AccessLogOverview struct {
	GeneratedAt         time.Time                `json:"generated_at"`
	Hours               int                      `json:"hours"`
	Summary             AccessLogOverviewSummary `json:"summary"`
	Trends              AccessLogOverviewTrends  `json:"trends"`
	TopPaths            []DistributionItem       `json:"top_paths"`
	TopHosts            []DistributionItem       `json:"top_hosts"`
	TopIPs              []DistributionItem       `json:"top_ips"`
	DeviceTypes         []DistributionItem       `json:"device_types"`
	TopBrowsers         []DistributionItem       `json:"top_browsers"`
	TopOperatingSystems []DistributionItem       `json:"top_operating_systems"`
	TopUserAgents       []DistributionItem       `json:"top_user_agents"`
	StatusCodes         []DistributionItem       `json:"status_codes"`
}

// AccessLogCleanupInput is the cleanup request payload.
type AccessLogCleanupInput struct {
	RetentionDays int `json:"retention_days"`
}

// AccessLogCleanupResult is the cleanup response payload.
type AccessLogCleanupResult struct {
	RetentionDays int       `json:"retention_days"`
	DeletedCount  int64     `json:"deleted_count"`
	Cutoff        time.Time `json:"cutoff"`
}

const (
	defaultAccessLogOverviewHours  = 24
	maxAccessLogOverviewHours      = 24 * 30
	accessLogOverviewTopLimit      = 10
	accessLogOverviewUASampleLimit = 200
)

// GetAccessLogOverview returns summary metrics, trends, and top rankings.
func GetAccessLogOverview(ctx context.Context, input AccessLogOverviewQuery) (*AccessLogOverview, error) {
	normalized := normalizeAccessLogOverviewQuery(input)
	now := time.Now().UTC()
	since := now.Add(-time.Duration(normalized.Hours) * time.Hour)
	query := model.OpenFlareAccessLogQuery{
		NodeID: normalized.NodeID,
		Host:   normalized.Host,
		Since:  since,
		Until:  now,
	}

	summaryRow, err := model.TrafficSummaryOpenFlareAccessLogs(ctx, query)
	if err != nil {
		return nil, err
	}

	requestPoints, visitPoints, bandwidthPoints := buildAccessLogOverviewTrends(
		ctx, now, normalized.Hours, query,
	)
	deviceTypes, topBrowsers, topOSes, topUserAgents := buildAccessLogUADistributions(ctx, query)

	return &AccessLogOverview{
		GeneratedAt: now,
		Hours:       normalized.Hours,
		Summary: AccessLogOverviewSummary{
			TotalRequests:   summaryRow.RequestCount,
			TotalVisits:     summaryRow.UniqueIPCount,
			BandwidthServed: summaryRow.BytesSent,
		},
		Trends: AccessLogOverviewTrends{
			Requests:  requestPoints,
			Visits:    visitPoints,
			Bandwidth: bandwidthPoints,
		},
		TopPaths:            valueCountDistribution(ctx, query, "path", accessLogOverviewTopLimit),
		TopHosts:            valueCountDistribution(ctx, query, "host", accessLogOverviewTopLimit),
		TopIPs:              valueCountDistribution(ctx, query, "remote_addr", accessLogOverviewTopLimit),
		DeviceTypes:         deviceTypes,
		TopBrowsers:         topBrowsers,
		TopOperatingSystems: topOSes,
		TopUserAgents:       topUserAgents,
		StatusCodes:         valueCountDistribution(ctx, query, "status_code", accessLogOverviewTopLimit),
	}, nil
}

func normalizeAccessLogOverviewQuery(input AccessLogOverviewQuery) AccessLogOverviewQuery {
	hours := input.Hours
	if hours <= 0 {
		hours = defaultAccessLogOverviewHours
	}
	if hours > maxAccessLogOverviewHours {
		hours = maxAccessLogOverviewHours
	}
	return AccessLogOverviewQuery{
		NodeID: strings.TrimSpace(input.NodeID),
		Host:   strings.TrimSpace(input.Host),
		Hours:  hours,
	}
}

func valueCountDistribution(
	ctx context.Context,
	query model.OpenFlareAccessLogQuery,
	column string,
	limit int,
) []DistributionItem {
	rows, err := model.ValueCountsOpenFlareAccessLogs(ctx, query, column, limit)
	if err != nil {
		logger.ErrorF(ctx, "[AccessLog] ValueCountsOpenFlareAccessLogs failed for column %s: %v", column, err)
		return []DistributionItem{}
	}
	if len(rows) == 0 {
		return []DistributionItem{}
	}
	items := make([]DistributionItem, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Value) == "" || row.Count <= 0 {
			continue
		}
		items = append(items, DistributionItem{Key: row.Value, Value: row.Count})
	}
	return items
}

func buildAccessLogUADistributions(
	ctx context.Context,
	query model.OpenFlareAccessLogQuery,
) (
	deviceTypes []DistributionItem,
	topBrowsers []DistributionItem,
	topOSes []DistributionItem,
	topUserAgents []DistributionItem,
) {
	uaRows := valueCountDistribution(ctx, query, "user_agent", accessLogOverviewUASampleLimit)
	if len(uaRows) == 0 {
		return []DistributionItem{}, []DistributionItem{}, []DistributionItem{}, []DistributionItem{}
	}

	deviceAcc := make(distributionAccumulator)
	browserAcc := make(distributionAccumulator)
	osAcc := make(distributionAccumulator)
	for _, row := range uaRows {
		ua := row.Key
		count := row.Value
		deviceAcc[analyticsrepo.ParseDeviceType(ua)] += count
		browserAcc[analyticsrepo.ParseBrowserName(ua)] += count
		osAcc[analyticsrepo.ParseOSName(ua)] += count
	}

	topUserAgents = make([]DistributionItem, 0, accessLogOverviewTopLimit)
	for _, row := range uaRows {
		if len(topUserAgents) >= accessLogOverviewTopLimit {
			break
		}
		topUserAgents = append(topUserAgents, row)
	}

	return toDistributionItems(deviceAcc, 0),
		toDistributionItems(browserAcc, accessLogOverviewTopLimit),
		toDistributionItems(osAcc, accessLogOverviewTopLimit),
		topUserAgents
}

func buildAccessLogOverviewTrends(
	ctx context.Context,
	now time.Time,
	hours int,
	query model.OpenFlareAccessLogQuery,
) (
	requests []AccessLogOverviewMetricPoint,
	visits []AccessLogOverviewMetricPoint,
	bandwidth []AccessLogOverviewMetricPoint,
) {
	if hours <= 0 {
		hours = defaultAccessLogOverviewHours
	}
	start := now.Truncate(time.Hour).Add(-time.Duration(hours-1) * time.Hour)
	requests = make([]AccessLogOverviewMetricPoint, hours)
	visits = make([]AccessLogOverviewMetricPoint, hours)
	bandwidth = make([]AccessLogOverviewMetricPoint, hours)
	for index := range requests {
		bucketAt := start.Add(time.Duration(index) * time.Hour)
		requests[index].BucketStartedAt = bucketAt
		visits[index].BucketStartedAt = bucketAt
		bandwidth[index].BucketStartedAt = bucketAt
	}

	buckets, err := model.ListOpenFlareAccessLogBuckets(ctx, model.OpenFlareAccessLogBucketQuery{
		NodeID:      query.NodeID,
		Host:        query.Host,
		Since:       query.Since,
		Until:       query.Until,
		FoldMinutes: 60,
		SortBy:      defaultAccessLogSortBy,
		SortOrder:   sortOrderAsc,
	})
	if err != nil || len(buckets) == 0 {
		return requests, visits, bandwidth
	}
	byEpoch := make(map[int64]*model.OpenFlareAccessLogBucketRow, len(buckets))
	for _, row := range buckets {
		if row == nil {
			continue
		}
		byEpoch[row.BucketEpoch] = row
	}
	for index := range requests {
		row, ok := byEpoch[requests[index].BucketStartedAt.Unix()]
		if !ok {
			continue
		}
		requests[index].Value = row.RequestCount
		visits[index].Value = row.UniqueIPCount
		bandwidth[index].Value = row.BytesSent
	}
	return requests, visits, bandwidth
}

// ListAccessLogs returns paginated access logs.
func ListAccessLogs(ctx context.Context, input AccessLogQuery) (*AccessLogList, error) {
	normalized := normalizeAccessLogQuery(input)
	modelQuery := buildModelAccessLogQuery(normalized)
	logs, err := model.ListOpenFlareAccessLogs(ctx, modelQuery)
	if err != nil {
		return nil, err
	}
	totalRecords, totalIPs, _, err := model.CountOpenFlareAccessLogs(ctx, modelQuery)
	if err != nil {
		return nil, err
	}
	nodeNames, err := listNodeNameMap(ctx, logs)
	if err != nil {
		return nil, err
	}
	views := make([]AccessLogView, 0, len(logs))
	for _, item := range logs {
		if item == nil {
			continue
		}
		views = append(views, AccessLogView{
			ID:         formatAccessLogID(item.ID),
			NodeID:     item.NodeID,
			NodeName:   nodeNames[item.NodeID],
			LoggedAt:   item.LoggedAt,
			RemoteAddr: item.RemoteAddr,
			Region:     item.Region,
			Host:       item.Host,
			Path:       item.Path,
			UserAgent:  item.UserAgent,
			StatusCode: item.StatusCode,
		})
	}
	return &AccessLogList{
		Items:       views,
		Page:        normalized.Page,
		PageSize:    normalized.PageSize,
		HasMore:     int64((normalized.Page+1)*normalized.PageSize) < totalRecords,
		TotalRecord: totalRecords,
		TotalIP:     totalIPs,
	}, nil
}

// ListFoldedAccessLogs returns paginated folded access logs.
func ListFoldedAccessLogs(ctx context.Context, input AccessLogQuery) (*FoldedAccessLogList, error) {
	normalized := normalizeAccessLogQuery(input)
	foldMinutes, err := normalizeFoldMinutes(normalized.FoldMinutes)
	if err != nil {
		return nil, err
	}
	modelQuery := buildModelAccessLogQuery(normalized)
	bucketQuery := model.OpenFlareAccessLogBucketQuery{
		NodeID:      modelQuery.NodeID,
		RemoteAddr:  modelQuery.RemoteAddr,
		Host:        modelQuery.Host,
		Path:        modelQuery.Path,
		Since:       modelQuery.Since,
		Page:        normalized.Page,
		PageSize:    normalized.PageSize,
		SortBy:      normalizeFoldSortBy(input.SortBy),
		SortOrder:   normalized.SortOrder,
		FoldMinutes: foldMinutes,
	}
	items, err := model.ListOpenFlareAccessLogBuckets(ctx, bucketQuery)
	if err != nil {
		return nil, err
	}
	totalBuckets, err := model.CountOpenFlareAccessLogBuckets(ctx, bucketQuery)
	if err != nil {
		return nil, err
	}
	totalRecords, totalIPs, _, err := model.CountOpenFlareAccessLogs(ctx, modelQuery)
	if err != nil {
		return nil, err
	}
	views := make([]FoldedAccessLogView, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		views = append(views, FoldedAccessLogView{
			BucketStartedAt:  time.Unix(item.BucketEpoch, 0).UTC(),
			RequestCount:     item.RequestCount,
			UniqueIPCount:    item.UniqueIPCount,
			UniqueHostCount:  item.UniqueHostCount,
			SuccessCount:     item.SuccessCount,
			ClientErrorCount: item.ClientErrorCount,
			ServerErrorCount: item.ServerErrorCount,
		})
	}
	return &FoldedAccessLogList{
		Items:       views,
		Page:        normalized.Page,
		PageSize:    normalized.PageSize,
		HasMore:     int64((normalized.Page+1)*normalized.PageSize) < totalBuckets,
		TotalBucket: totalBuckets,
		TotalRecord: totalRecords,
		TotalIP:     totalIPs,
		FoldMinutes: foldMinutes,
	}, nil
}

// ListFoldedAccessLogIPs returns paginated folded IP summaries.
func ListFoldedAccessLogIPs(ctx context.Context, input FoldedAccessLogIPQuery) (*FoldedAccessLogIPList, error) {
	normalized, bucketStartedAt, err := normalizeFoldedAccessLogIPQuery(input)
	if err != nil {
		return nil, err
	}
	modelQuery := model.OpenFlareAccessLogBucketIPQuery{
		NodeID:          normalized.NodeID,
		RemoteAddr:      normalized.RemoteAddr,
		Host:            normalized.Host,
		Path:            normalized.Path,
		BucketStartedAt: bucketStartedAt,
		FoldMinutes:     normalized.FoldMinutes,
		Page:            normalized.Page,
		PageSize:        normalized.PageSize,
		SortBy:          normalized.SortBy,
		SortOrder:       normalized.SortOrder,
	}
	items, err := model.ListOpenFlareAccessLogBucketIPs(ctx, modelQuery)
	if err != nil {
		return nil, err
	}
	totalIP, err := model.CountOpenFlareAccessLogBucketIPs(ctx, modelQuery)
	if err != nil {
		return nil, err
	}
	views := make([]FoldedAccessLogIPView, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		views = append(views, FoldedAccessLogIPView{
			RemoteAddr:       item.RemoteAddr,
			RequestCount:     item.RequestCount,
			SuccessCount:     item.SuccessCount,
			ClientErrorCount: item.ClientErrorCount,
			ServerErrorCount: item.ServerErrorCount,
			LastSeenAt:       time.Unix(item.LastSeenEpoch, 0).UTC(),
		})
	}
	return &FoldedAccessLogIPList{
		Items:           views,
		Page:            normalized.Page,
		PageSize:        normalized.PageSize,
		HasMore:         int64((normalized.Page+1)*normalized.PageSize) < totalIP,
		TotalIP:         totalIP,
		BucketStartedAt: bucketStartedAt,
		FoldMinutes:     normalized.FoldMinutes,
		SortBy:          normalized.SortBy,
		SortOrder:       normalized.SortOrder,
	}, nil
}

// ListAccessLogIPSummaries returns paginated IP summaries.
func ListAccessLogIPSummaries(ctx context.Context, input AccessLogIPSummaryQuery) (*AccessLogIPSummaryList, error) {
	normalized := normalizeAccessLogIPSummaryQuery(input)
	since := defaultAccessLogSince()
	recentSince := time.Now().UTC().Add(-3 * time.Hour)
	query := model.OpenFlareAccessLogIPSummaryQuery{
		NodeID:     strings.TrimSpace(normalized.NodeID),
		RemoteAddr: strings.TrimSpace(normalized.RemoteAddr),
		Host:       strings.TrimSpace(normalized.Host),
		Since:      since,
		Page:       normalized.Page,
		PageSize:   normalized.PageSize,
		SortBy:     normalized.SortBy,
		SortOrder:  normalized.SortOrder,
	}
	items, err := model.ListOpenFlareAccessLogIPSummaries(ctx, query, recentSince)
	if err != nil {
		return nil, err
	}
	totalIP, err := model.CountOpenFlareAccessLogIPSummaries(ctx, query)
	if err != nil {
		return nil, err
	}
	views := make([]AccessLogIPSummaryView, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		views = append(views, AccessLogIPSummaryView{
			RemoteAddr:     item.RemoteAddr,
			TotalRequests:  item.TotalRequests,
			RecentRequests: item.RecentRequests,
			LastSeenAt:     time.Unix(item.LastSeenEpoch, 0).UTC(),
		})
	}
	return &AccessLogIPSummaryList{
		Items:     views,
		Page:      normalized.Page,
		PageSize:  normalized.PageSize,
		HasMore:   int64((normalized.Page+1)*normalized.PageSize) < totalIP,
		TotalIP:   totalIP,
		SortBy:    normalized.SortBy,
		SortOrder: normalized.SortOrder,
	}, nil
}

// GetAccessLogIPTrend returns IP request trend points.
func GetAccessLogIPTrend(ctx context.Context, input AccessLogIPTrendQuery) (*AccessLogIPTrendView, error) {
	normalized, err := normalizeAccessLogIPTrendQuery(input)
	if err != nil {
		return nil, err
	}
	points, err := model.ListOpenFlareAccessLogIPTrend(ctx, model.OpenFlareAccessLogIPTrendQuery{
		NodeID:        strings.TrimSpace(normalized.NodeID),
		RemoteAddr:    strings.TrimSpace(normalized.RemoteAddr),
		Host:          strings.TrimSpace(normalized.Host),
		Since:         time.Now().UTC().Add(-time.Duration(normalized.Hours) * time.Hour),
		BucketMinutes: normalized.BucketMinutes,
	})
	if err != nil {
		return nil, err
	}
	pointMap := make(map[int64]int64, len(points))
	for _, item := range points {
		if item == nil {
			continue
		}
		pointMap[item.BucketEpoch] = item.RequestCount
	}
	bucketDuration := time.Duration(normalized.BucketMinutes) * time.Minute
	start := time.Now().UTC().Add(-time.Duration(normalized.Hours) * time.Hour).Truncate(bucketDuration)
	end := time.Now().UTC().Truncate(bucketDuration)
	views := make([]AccessLogIPTrendPoint, 0, int(end.Sub(start)/bucketDuration)+1)
	for cursor := start; !cursor.After(end); cursor = cursor.Add(bucketDuration) {
		views = append(views, AccessLogIPTrendPoint{
			BucketStartedAt: cursor,
			RequestCount:    pointMap[cursor.Unix()],
		})
	}
	return &AccessLogIPTrendView{
		RemoteAddr:    normalized.RemoteAddr,
		Hours:         normalized.Hours,
		BucketMinutes: normalized.BucketMinutes,
		Points:        views,
	}, nil
}

// CleanupAccessLogs removes access logs older than retention days.
func CleanupAccessLogs(ctx context.Context, input AccessLogCleanupInput) (*AccessLogCleanupResult, error) {
	if input.RetentionDays <= 0 || input.RetentionDays > nodeAccessLogRetentionDays {
		return nil, errors.New("retention_days 必须在 1 到 90 之间")
	}
	cutoff := time.Now().UTC().Add(-time.Duration(input.RetentionDays) * 24 * time.Hour)
	deleted, err := model.DeleteOpenFlareAccessLogsBefore(ctx, cutoff)
	if err != nil {
		return nil, err
	}
	return &AccessLogCleanupResult{
		RetentionDays: input.RetentionDays,
		DeletedCount:  deleted,
		Cutoff:        cutoff,
	}, nil
}

func buildModelAccessLogQuery(input AccessLogQuery) model.OpenFlareAccessLogQuery {
	return model.OpenFlareAccessLogQuery{
		NodeID:     strings.TrimSpace(input.NodeID),
		RemoteAddr: strings.TrimSpace(input.RemoteAddr),
		Host:       strings.TrimSpace(input.Host),
		Path:       strings.TrimSpace(input.Path),
		Since:      defaultAccessLogSince(),
		Page:       input.Page,
		PageSize:   input.PageSize,
		SortBy:     input.SortBy,
		SortOrder:  input.SortOrder,
	}
}

func defaultAccessLogSince() time.Time {
	return time.Now().UTC().Add(-defaultAccessLogQueryWindow)
}

func listNodeNameMap(ctx context.Context, logs []*model.OpenFlareAccessLog) (map[string]string, error) {
	nodeIDs := make([]string, 0, len(logs))
	seen := make(map[string]struct{}, len(logs))
	for _, item := range logs {
		if item == nil || item.NodeID == "" {
			continue
		}
		if _, exists := seen[item.NodeID]; exists {
			continue
		}
		seen[item.NodeID] = struct{}{}
		nodeIDs = append(nodeIDs, item.NodeID)
	}
	if len(nodeIDs) == 0 {
		return map[string]string{}, nil
	}
	nodes, err := model.ListOpenFlareNodesByNodeIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(nodes))
	for _, node := range nodes {
		result[node.NodeID] = node.Name
	}
	return result, nil
}

func normalizeAccessLogQuery(input AccessLogQuery) AccessLogQuery {
	return AccessLogQuery{
		NodeID:      strings.TrimSpace(input.NodeID),
		RemoteAddr:  strings.TrimSpace(input.RemoteAddr),
		Host:        strings.TrimSpace(input.Host),
		Path:        strings.TrimSpace(input.Path),
		Page:        normalizeAccessLogPage(input.Page),
		PageSize:    normalizeAccessLogPageSize(input.PageSize),
		SortBy:      normalizeAccessLogSortBy(input.SortBy),
		SortOrder:   normalizeAccessLogSortOrder(input.SortOrder),
		FoldMinutes: input.FoldMinutes,
	}
}

func normalizeAccessLogIPSummaryQuery(input AccessLogIPSummaryQuery) AccessLogIPSummaryQuery {
	return AccessLogIPSummaryQuery{
		NodeID:     strings.TrimSpace(input.NodeID),
		RemoteAddr: strings.TrimSpace(input.RemoteAddr),
		Host:       strings.TrimSpace(input.Host),
		Page:       normalizeAccessLogPage(input.Page),
		PageSize:   normalizeAccessLogPageSize(input.PageSize),
		SortBy:     normalizeIPSummarySortBy(input.SortBy),
		SortOrder:  normalizeAccessLogSortOrder(input.SortOrder),
	}
}

func normalizeFoldedAccessLogIPQuery(input FoldedAccessLogIPQuery) (FoldedAccessLogIPQuery, time.Time, error) {
	foldMinutes, err := normalizeFoldMinutes(input.FoldMinutes)
	if err != nil {
		return FoldedAccessLogIPQuery{}, time.Time{}, err
	}
	bucketStartedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.BucketStartedAt))
	if err != nil {
		return FoldedAccessLogIPQuery{}, time.Time{}, errors.New("bucket_started_at 必须为 RFC3339 时间")
	}
	normalizedSortBy := strings.TrimSpace(input.SortBy)
	switch normalizedSortBy {
	case "last_seen_at", accessLogFieldRemoteAddr:
	default:
		normalizedSortBy = accessLogFieldRequestCount
	}
	return FoldedAccessLogIPQuery{
		NodeID:          strings.TrimSpace(input.NodeID),
		RemoteAddr:      strings.TrimSpace(input.RemoteAddr),
		Host:            strings.TrimSpace(input.Host),
		Path:            strings.TrimSpace(input.Path),
		BucketStartedAt: strings.TrimSpace(input.BucketStartedAt),
		FoldMinutes:     foldMinutes,
		Page:            normalizeAccessLogPage(input.Page),
		PageSize:        normalizeAccessLogPageSize(input.PageSize),
		SortBy:          normalizedSortBy,
		SortOrder:       normalizeAccessLogSortOrder(input.SortOrder),
	}, bucketStartedAt.UTC(), nil
}

func normalizeAccessLogIPTrendQuery(input AccessLogIPTrendQuery) (AccessLogIPTrendQuery, error) {
	remoteAddr := strings.TrimSpace(input.RemoteAddr)
	if remoteAddr == "" {
		return AccessLogIPTrendQuery{}, errors.New("remote_addr 不能为空")
	}
	hours := input.Hours
	if hours <= 0 {
		hours = defaultIPTrendHours
	}
	if hours > maxIPTrendHours {
		hours = maxIPTrendHours
	}
	bucketMinutes := input.BucketMinutes
	if bucketMinutes <= 0 {
		bucketMinutes = defaultIPTrendBucketMinute
	}
	switch bucketMinutes {
	case 5, 10, 15, 30, 60:
	default:
		return AccessLogIPTrendQuery{}, errors.New("bucket_minutes 仅支持 5、10、15、30、60")
	}
	return AccessLogIPTrendQuery{
		NodeID:        strings.TrimSpace(input.NodeID),
		RemoteAddr:    remoteAddr,
		Host:          strings.TrimSpace(input.Host),
		Hours:         hours,
		BucketMinutes: bucketMinutes,
	}, nil
}

func normalizeAccessLogPage(page int) int {
	if page < 0 {
		return 0
	}
	return page
}

func normalizeAccessLogPageSize(pageSize int) int {
	if pageSize <= 0 {
		return defaultAccessLogPageSize
	}
	if pageSize > maxAccessLogPageSize {
		return maxAccessLogPageSize
	}
	return pageSize
}

func normalizeAccessLogSortBy(sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case "status_code", accessLogFieldRemoteAddr, "host", "path":
		return strings.TrimSpace(sortBy)
	default:
		return defaultAccessLogSortBy
	}
}

func normalizeAccessLogSortOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), accessLogSortOrderAsc) {
		return accessLogSortOrderAsc
	}
	return defaultAccessLogSortOrder
}

func normalizeFoldSortBy(sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case accessLogFieldRequestCount:
		return accessLogFieldRequestCount
	default:
		return "bucket_started_at"
	}
}

func normalizeIPSummarySortBy(sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case "recent_requests", "last_seen_at", accessLogFieldRemoteAddr:
		return strings.TrimSpace(sortBy)
	default:
		return "total_requests"
	}
}

func normalizeFoldMinutes(value int) (int, error) {
	if value <= 0 {
		return defaultAccessLogFoldMinute, nil
	}
	switch value {
	case 3, 5:
		return value, nil
	default:
		return 0, errors.New("fold_minutes 仅支持 3 或 5")
	}
}

func formatAccessLogID(id uint64) string {
	return strconv.FormatUint(id, 10)
}
