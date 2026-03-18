package service

import (
	"errors"
	"openflare/model"
	"strings"
	"time"
)

const (
	defaultAccessLogPageSize   = 20
	maxAccessLogPageSize       = 200
	defaultAccessLogSortBy     = "logged_at"
	defaultAccessLogSortOrder  = "desc"
	defaultAccessLogFoldMinute = 3
	defaultIPTrendHours        = 24
	defaultIPTrendBucketMinute = 30
	maxIPTrendHours            = 168
	nodeAccessLogRetentionDays = 90
)

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

type AccessLogView struct {
	ID         uint      `json:"id"`
	NodeID     string    `json:"node_id"`
	NodeName   string    `json:"node_name"`
	LoggedAt   time.Time `json:"logged_at"`
	RemoteAddr string    `json:"remote_addr"`
	Region     string    `json:"region"`
	Host       string    `json:"host"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
}

type AccessLogList struct {
	Items       []AccessLogView `json:"items"`
	Page        int             `json:"page"`
	PageSize    int             `json:"page_size"`
	HasMore     bool            `json:"has_more"`
	TotalRecord int64           `json:"total_record"`
	TotalIP     int64           `json:"total_ip"`
}

type FoldedAccessLogView struct {
	BucketStartedAt  time.Time `json:"bucket_started_at"`
	RequestCount     int64     `json:"request_count"`
	UniqueIPCount    int64     `json:"unique_ip_count"`
	UniqueHostCount  int64     `json:"unique_host_count"`
	SuccessCount     int64     `json:"success_count"`
	ClientErrorCount int64     `json:"client_error_count"`
	ServerErrorCount int64     `json:"server_error_count"`
}

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

type AccessLogIPSummaryQuery struct {
	NodeID     string `json:"node_id"`
	RemoteAddr string `json:"remote_addr"`
	Host       string `json:"host"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	SortBy     string `json:"sort_by"`
	SortOrder  string `json:"sort_order"`
}

type AccessLogIPSummaryView struct {
	RemoteAddr     string    `json:"remote_addr"`
	TotalRequests  int64     `json:"total_requests"`
	RecentRequests int64     `json:"recent_requests"`
	LastSeenAt     time.Time `json:"last_seen_at"`
}

type AccessLogIPSummaryList struct {
	Items     []AccessLogIPSummaryView `json:"items"`
	Page      int                      `json:"page"`
	PageSize  int                      `json:"page_size"`
	HasMore   bool                     `json:"has_more"`
	TotalIP   int64                    `json:"total_ip"`
	SortBy    string                   `json:"sort_by"`
	SortOrder string                   `json:"sort_order"`
}

type AccessLogIPTrendQuery struct {
	NodeID        string `json:"node_id"`
	RemoteAddr    string `json:"remote_addr"`
	Host          string `json:"host"`
	Hours         int    `json:"hours"`
	BucketMinutes int    `json:"bucket_minutes"`
}

type AccessLogIPTrendPoint struct {
	BucketStartedAt time.Time `json:"bucket_started_at"`
	RequestCount    int64     `json:"request_count"`
}

type AccessLogIPTrendView struct {
	RemoteAddr    string                  `json:"remote_addr"`
	Hours         int                     `json:"hours"`
	BucketMinutes int                     `json:"bucket_minutes"`
	Points        []AccessLogIPTrendPoint `json:"points"`
}

type AccessLogCleanupInput struct {
	RetentionDays int `json:"retention_days"`
}

type AccessLogCleanupResult struct {
	RetentionDays int       `json:"retention_days"`
	DeletedCount  int64     `json:"deleted_count"`
	Cutoff        time.Time `json:"cutoff"`
}

func ListAccessLogs(input AccessLogQuery) (*AccessLogList, error) {
	normalized := normalizeAccessLogQuery(input)
	modelQuery := buildModelAccessLogQuery(normalized)
	logs, err := model.ListNodeAccessLogs(modelQuery)
	if err != nil {
		return nil, err
	}
	totalRecords, totalIPs, err := model.CountNodeAccessLogs(modelQuery)
	if err != nil {
		return nil, err
	}
	nodeNames, err := listNodeNameMap(logs)
	if err != nil {
		return nil, err
	}
	views := make([]AccessLogView, 0, len(logs))
	for _, item := range logs {
		if item == nil {
			continue
		}
		views = append(views, AccessLogView{
			ID:         item.ID,
			NodeID:     item.NodeID,
			NodeName:   nodeNames[item.NodeID],
			LoggedAt:   item.LoggedAt,
			RemoteAddr: item.RemoteAddr,
			Region:     item.Region,
			Host:       item.Host,
			Path:       item.Path,
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

func ListFoldedAccessLogs(input AccessLogQuery) (*FoldedAccessLogList, error) {
	normalized := normalizeAccessLogQuery(input)
	foldMinutes, err := normalizeFoldMinutes(normalized.FoldMinutes)
	if err != nil {
		return nil, err
	}
	modelQuery := buildModelAccessLogQuery(normalized)
	bucketQuery := model.NodeAccessLogBucketQuery{
		NodeID:      modelQuery.NodeID,
		RemoteAddr:  modelQuery.RemoteAddr,
		Host:        modelQuery.Host,
		Path:        modelQuery.Path,
		Since:       modelQuery.Since,
		Page:        normalized.Page,
		PageSize:    normalized.PageSize,
		SortBy:      normalizeFoldSortBy(normalized.SortBy),
		SortOrder:   normalized.SortOrder,
		FoldMinutes: foldMinutes,
	}
	items, err := model.ListNodeAccessLogBuckets(bucketQuery)
	if err != nil {
		return nil, err
	}
	totalBuckets, err := model.CountNodeAccessLogBuckets(bucketQuery)
	if err != nil {
		return nil, err
	}
	totalRecords, totalIPs, err := model.CountNodeAccessLogs(modelQuery)
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

func ListAccessLogIPSummaries(input AccessLogIPSummaryQuery) (*AccessLogIPSummaryList, error) {
	normalized := normalizeAccessLogIPSummaryQuery(input)
	since := time.Now().UTC().Add(-nodeAccessLogRetentionWindow)
	recentSince := time.Now().UTC().Add(-3 * time.Hour)
	query := model.NodeAccessLogIPSummaryQuery{
		NodeID:     strings.TrimSpace(normalized.NodeID),
		RemoteAddr: strings.TrimSpace(normalized.RemoteAddr),
		Host:       strings.TrimSpace(normalized.Host),
		Since:      since,
		Page:       normalized.Page,
		PageSize:   normalized.PageSize,
		SortBy:     normalized.SortBy,
		SortOrder:  normalized.SortOrder,
	}
	items, err := model.ListNodeAccessLogIPSummaries(query, recentSince)
	if err != nil {
		return nil, err
	}
	totalIP, err := model.CountNodeAccessLogIPSummaries(query)
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

func GetAccessLogIPTrend(input AccessLogIPTrendQuery) (*AccessLogIPTrendView, error) {
	normalized, err := normalizeAccessLogIPTrendQuery(input)
	if err != nil {
		return nil, err
	}
	points, err := model.ListNodeAccessLogIPTrend(model.NodeAccessLogIPTrendQuery{
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

func CleanupAccessLogs(input AccessLogCleanupInput) (*AccessLogCleanupResult, error) {
	if input.RetentionDays <= 0 || input.RetentionDays > nodeAccessLogRetentionDays {
		return nil, errors.New("retention_days 必须在 1 到 90 之间")
	}
	cutoff := time.Now().UTC().Add(-time.Duration(input.RetentionDays) * 24 * time.Hour)
	deleted, err := model.DeleteNodeAccessLogsBefore(cutoff)
	if err != nil {
		return nil, err
	}
	return &AccessLogCleanupResult{
		RetentionDays: input.RetentionDays,
		DeletedCount:  deleted,
		Cutoff:        cutoff,
	}, nil
}

func buildModelAccessLogQuery(input AccessLogQuery) model.NodeAccessLogQuery {
	return model.NodeAccessLogQuery{
		NodeID:     strings.TrimSpace(input.NodeID),
		RemoteAddr: strings.TrimSpace(input.RemoteAddr),
		Host:       strings.TrimSpace(input.Host),
		Path:       strings.TrimSpace(input.Path),
		Since:      time.Now().UTC().Add(-nodeAccessLogRetentionWindow),
		Page:       input.Page,
		PageSize:   input.PageSize,
		SortBy:     input.SortBy,
		SortOrder:  input.SortOrder,
	}
}

func listNodeNameMap(logs []*model.NodeAccessLog) (map[string]string, error) {
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
	nodes, err := model.ListNodesByNodeIDs(nodeIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
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
	case "status_code", "remote_addr", "host", "path":
		return strings.TrimSpace(sortBy)
	default:
		return defaultAccessLogSortBy
	}
}

func normalizeAccessLogSortOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		return "asc"
	}
	return defaultAccessLogSortOrder
}

func normalizeFoldSortBy(sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case "request_count":
		return "request_count"
	default:
		return "bucket_started_at"
	}
}

func normalizeIPSummarySortBy(sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case "recent_requests", "last_seen_at", "remote_addr":
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
