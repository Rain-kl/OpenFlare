package service

import (
	"openflare/model"
	"strings"
	"time"
)

const (
	defaultAccessLogPageSize = 20
	maxAccessLogPageSize     = 200
)

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

func ListAccessLogs(nodeID string, page int, pageSize int) (*AccessLogList, error) {
	normalizedPage := normalizeAccessLogPage(page)
	normalizedPageSize := normalizeAccessLogPageSize(pageSize)
	offset := normalizedPage * normalizedPageSize
	trimmedNodeID := strings.TrimSpace(nodeID)
	since := time.Now().Add(-nodeAccessLogRetentionWindow)
	logs, err := model.ListNodeAccessLogs(
		trimmedNodeID,
		since,
		offset,
		normalizedPageSize+1,
	)
	if err != nil {
		return nil, err
	}
	totalRecords, totalIPs, err := model.CountNodeAccessLogs(trimmedNodeID, since)
	if err != nil {
		return nil, err
	}
	nodeIDs := make([]string, 0, len(logs))
	seenNodeIDs := make(map[string]struct{}, len(logs))
	for _, item := range logs {
		if item == nil {
			continue
		}
		if _, exists := seenNodeIDs[item.NodeID]; exists {
			continue
		}
		seenNodeIDs[item.NodeID] = struct{}{}
		nodeIDs = append(nodeIDs, item.NodeID)
	}
	nodes, err := model.ListNodesByNodeIDs(nodeIDs)
	if err != nil {
		return nil, err
	}
	nodeNames := make(map[string]string, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		nodeNames[node.NodeID] = node.Name
	}
	hasMore := len(logs) > normalizedPageSize
	if hasMore {
		logs = logs[:normalizedPageSize]
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
		Page:        normalizedPage,
		PageSize:    normalizedPageSize,
		HasMore:     hasMore,
		TotalRecord: totalRecords,
		TotalIP:     totalIPs,
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
