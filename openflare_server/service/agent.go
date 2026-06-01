package service

import (
	"encoding/json"
	"errors"
	"log/slog"
	"openflare/common"
	"openflare/model"
	"openflare/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	NodeStatusOnline         = "online"
	NodeStatusOffline        = "offline"
	NodeStatusPending        = "pending"
	ApplyResultOK            = "success"
	ApplyResultWarning       = "warning"
	ApplyResultFailed        = "failed"
	OpenrestyStatusHealthy   = "healthy"
	OpenrestyStatusUnhealthy = "unhealthy"
	OpenrestyStatusUnknown   = "unknown"
)

type AgentNodePayload struct {
	NodeID                string                             `json:"node_id"`
	Name                  string                             `json:"name"`
	IP                    string                             `json:"ip"`
	AgentVersion          string                             `json:"agent_version"`
	NginxVersion          string                             `json:"nginx_version"`
	CurrentVersion        string                             `json:"current_version"`
	LastError             string                             `json:"last_error"`
	OpenrestyStatus       string                             `json:"openresty_status"`
	OpenrestyMessage      string                             `json:"openresty_message"`
	Profile               *AgentNodeSystemProfile            `json:"profile,omitempty"`
	Snapshot              *AgentNodeMetricSnapshot           `json:"snapshot,omitempty"`
	TrafficReport         *AgentNodeTrafficReport            `json:"traffic_report,omitempty"`
	AccessLogs            []AgentNodeAccessLog               `json:"access_logs,omitempty"`
	BufferedObservability []AgentBufferedObservabilityRecord `json:"buffered_observability,omitempty"`
	HealthEvents          []AgentNodeHealthEvent             `json:"health_events"`
	WAFIPGroupChecksums   map[string]string                  `json:"waf_ip_group_checksums,omitempty"`
}

type ApplyLogPayload struct {
	NodeID              string `json:"node_id"`
	Version             string `json:"version"`
	Result              string `json:"result"`
	Message             string `json:"message"`
	Checksum            string `json:"checksum"`
	MainConfigChecksum  string `json:"main_config_checksum"`
	RouteConfigChecksum string `json:"route_config_checksum"`
	SupportFileCount    int    `json:"support_file_count"`
}

type ApplyLogListQuery struct {
	NodeID   string `json:"node_id"`
	PageNo   int    `json:"pageNo"`
	PageSize int    `json:"pageSize"`
}

type ApplyLogListResult struct {
	Rows      []*model.ApplyLog `json:"rows"`
	Current   int               `json:"current"`
	Total     int               `json:"total"`
	TotalPage int               `json:"totalPage"`
}

type ApplyLogCleanupInput struct {
	DeleteAll     bool `json:"delete_all"`
	RetentionDays int  `json:"retention_days"`
}

type ApplyLogCleanupResult struct {
	DeleteAll     bool       `json:"delete_all"`
	RetentionDays int        `json:"retention_days"`
	DeletedCount  int64      `json:"deleted_count"`
	Cutoff        *time.Time `json:"cutoff,omitempty"`
}

type AgentConfigResponse struct {
	Version          string        `json:"version"`
	Checksum         string        `json:"checksum"`
	SourceConfigJSON string        `json:"source_config_json"`
	SupportFiles     []SupportFile `json:"support_files"`
	CreatedAt        time.Time     `json:"created_at"`
}

type AgentSettings struct {
	HeartbeatInterval       int    `json:"heartbeat_interval"`
	WebsocketUpgradeEnabled bool   `json:"websocket_upgrade_enabled"`
	AutoUpdate              bool   `json:"auto_update"`
	UpdateRepo              string `json:"update_repo"`
	UpdateNow               bool   `json:"update_now"`
	UpdateChannel           string `json:"update_channel"`
	UpdateTag               string `json:"update_tag"`
	RestartOpenrestyNow     bool   `json:"restart_openresty_now"`
}

type ActiveConfigMeta struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

type HeartbeatResponse struct {
	Node          *model.Node       `json:"node"`
	AgentSettings *AgentSettings    `json:"agent_settings"`
	ActiveConfig  *ActiveConfigMeta `json:"active_config"`
	WAFIPGroups   []AgentWAFIPGroup `json:"waf_ip_groups,omitempty"`
}

type AgentWAFIPGroup struct {
	ID       uint     `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Enabled  bool     `json:"enabled"`
	IPList   []string `json:"ip_list"`
	Checksum string   `json:"checksum"`
}

type AgentWAFIPGroupSyncInput struct {
	IDs       []uint            `json:"ids"`
	Checksums map[string]string `json:"checksums"`
}

type AgentWAFIPGroupSyncResult struct {
	Groups []AgentWAFIPGroup `json:"groups"`
}

type NodeView struct {
	ID                        uint       `json:"id"`
	NodeID                    string     `json:"node_id"`
	Name                      string     `json:"name"`
	IP                        string     `json:"ip"`
	IPManualOverride          bool       `json:"ip_manual_override"`
	GeoName                   string     `json:"geo_name"`
	GeoLatitude               *float64   `json:"geo_latitude"`
	GeoLongitude              *float64   `json:"geo_longitude"`
	GeoManualOverride         bool       `json:"geo_manual_override"`
	AgentToken                string     `json:"agent_token"`
	AutoUpdateEnabled         bool       `json:"auto_update_enabled"`
	UpdateRequested           bool       `json:"update_requested"`
	UpdateChannel             string     `json:"update_channel"`
	UpdateTag                 string     `json:"update_tag"`
	RestartOpenrestyRequested bool       `json:"restart_openresty_requested"`
	AgentVersion              string     `json:"agent_version"`
	NginxVersion              string     `json:"nginx_version"`
	OpenrestyStatus           string     `json:"openresty_status"`
	OpenrestyMessage          string     `json:"openresty_message"`
	Status                    string     `json:"status"`
	CurrentVersion            string     `json:"current_version"`
	LastSeenAt                any        `json:"last_seen_at"`
	LastError                 string     `json:"last_error"`
	LatestApplyResult         string     `json:"latest_apply_result"`
	LatestApplyMessage        string     `json:"latest_apply_message"`
	LatestApplyChecksum       string     `json:"latest_apply_checksum"`
	LatestMainConfigChecksum  string     `json:"latest_main_config_checksum"`
	LatestRouteConfigChecksum string     `json:"latest_route_config_checksum"`
	LatestSupportFileCount    int        `json:"latest_support_file_count"`
	LatestApplyAt             *time.Time `json:"latest_apply_at"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
	// TunnelRelay fields
	NodeType              string `json:"node_type"`
	RelayBindPort         int    `json:"relay_bind_port"`
	RelayVhostHTTPPort    int    `json:"relay_vhost_http_port"`
	RelayAgentAccessAddr  string `json:"relay_agent_access_addr"`
	RelayClientAccessAddr string `json:"relay_client_access_addr"`
	RelayClientProxyURL   string `json:"relay_client_proxy_url"`
	RelayStatus           string `json:"relay_status"`
	RelayFrpVersion       string `json:"relay_frp_version"`
	RelayVersion          string `json:"relay_version"`
}

func HeartbeatNode(node *model.Node, payload AgentNodePayload) (*HeartbeatResponse, error) {
	slog.Debug("agent heartbeat received", "node_id", node.NodeID, "current_version", strings.TrimSpace(payload.CurrentVersion))
	payload.NodeID = node.NodeID
	payload = normalizeAgentNodePayload(payload)
	if err := validateAgentNodePayload(payload); err != nil {
		return nil, err
	}
	previous := *node
	updateNow := node.UpdateRequested
	restartOpenrestyNow := node.RestartOpenrestyRequested
	updateChannel := normalizeReleaseChannel(node.UpdateChannel)
	updateTag := strings.TrimSpace(node.UpdateTag)
	applyNodeRuntime(node, payload, true)
	node.UpdateRequested = false
	node.UpdateChannel = ReleaseChannelStable.String()
	node.UpdateTag = ""
	node.RestartOpenrestyRequested = false
	changes := collectNodeHeartbeatChanges(&previous, node)
	if len(changes) > 0 {
		if err := model.DB.Model(node).Updates(changes).Error; err != nil {
			return nil, err
		}
	}
	refreshAgentTokenCache(node)
	persistHeartbeatObservability(node.NodeID, payload, node.LastSeenAt)
	activeConfig, err := GetActiveConfigMetaForAgent()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	wafIPGroups, err := ChangedWAFIPGroupsForAgent(nil, payload.WAFIPGroupChecksums)
	if err != nil {
		return nil, err
	}
	return &HeartbeatResponse{
		Node:          node,
		AgentSettings: buildAgentSettings(node, updateNow, updateChannel.String(), updateTag, restartOpenrestyNow),
		ActiveConfig:  activeConfig,
		WAFIPGroups:   wafIPGroups,
	}, nil
}

func buildAgentSettings(node *model.Node, updateNow bool, updateChannel string, updateTag string, restartOpenrestyNow bool) *AgentSettings {
	autoUpdate := false
	if node != nil {
		autoUpdate = node.AutoUpdateEnabled
	}
	if strings.TrimSpace(updateChannel) == "" {
		updateChannel = ReleaseChannelStable.String()
	}
	return &AgentSettings{
		HeartbeatInterval:       common.AgentHeartbeatInterval,
		WebsocketUpgradeEnabled: common.AgentWebsocketUpgradeEnabled,
		AutoUpdate:              autoUpdate,
		UpdateRepo:              common.AgentUpdateRepo,
		UpdateNow:               updateNow,
		UpdateChannel:           updateChannel,
		UpdateTag:               strings.TrimSpace(updateTag),
		RestartOpenrestyNow:     restartOpenrestyNow,
	}
}

func GetActiveConfigMetaForAgent() (*ActiveConfigMeta, error) {
	version, err := model.GetActiveConfigVersion()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &ActiveConfigMeta{
		Version:  version.Version,
		Checksum: version.Checksum,
	}, nil
}

func GetActiveConfigForAgent() (*AgentConfigResponse, error) {
	version, err := model.GetActiveConfigVersion()
	if err != nil {
		slog.Error("agent requested active config but no active version is available")
		return nil, err
	}
	var supportFiles []SupportFile
	if version.SupportFilesJSON != "" {
		if err = json.Unmarshal([]byte(version.SupportFilesJSON), &supportFiles); err != nil {
			return nil, err
		}
	}
	slog.Debug("agent fetched active config", "version", version.Version, "checksum", version.Checksum)
	return &AgentConfigResponse{
		Version:          version.Version,
		Checksum:         version.Checksum,
		SourceConfigJSON: version.SnapshotJSON,
		SupportFiles:     sourceSupportFiles(supportFiles),
		CreatedAt:        version.CreatedAt,
	}, nil
}

func normalizeApplyLogPayload(payload ApplyLogPayload) ApplyLogPayload {
	payload.Result = strings.ToLower(payload.Result)
	utils.TrimStringFields(
		&payload.NodeID,
		&payload.Version,
		&payload.Result,
		&payload.Message,
		&payload.Checksum,
		&payload.MainConfigChecksum,
		&payload.RouteConfigChecksum,
	)
	payload.Message = truncateForDatabase(payload.Message, 16000)
	return payload
}

func ReportApplyLog(payload ApplyLogPayload) (*model.ApplyLog, error) {
	now := time.Now()
	payload = normalizeApplyLogPayload(payload)
	if payload.NodeID == "" {
		return nil, errors.New("node_id 不能为空")
	}
	if payload.Version == "" {
		return nil, errors.New("version 不能为空")
	}
	if payload.Result != ApplyResultOK && payload.Result != ApplyResultWarning && payload.Result != ApplyResultFailed {
		return nil, errors.New("result 仅支持 success、warning 或 failed")
	}
	slog.Debug("agent apply log received", "node_id", payload.NodeID, "version", payload.Version, "result", payload.Result)

	log := &model.ApplyLog{
		NodeID:              payload.NodeID,
		Version:             payload.Version,
		Result:              payload.Result,
		Message:             payload.Message,
		Checksum:            payload.Checksum,
		MainConfigChecksum:  payload.MainConfigChecksum,
		RouteConfigChecksum: payload.RouteConfigChecksum,
		SupportFileCount:    payload.SupportFileCount,
		CreatedAt:           now,
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		node := &model.Node{}
		if err := tx.Where("node_id = ?", payload.NodeID).First(node).Error; err != nil {
			return err
		}
		node.Status = NodeStatusOnline
		node.LastSeenAt = now
		if payload.Result == ApplyResultOK {
			node.CurrentVersion = payload.Version
			node.LastError = ""
		} else {
			node.LastError = payload.Message
		}
		if err := tx.Create(log).Error; err != nil {
			return err
		}
		return tx.Model(node).Select("status", "last_seen_at", "current_version", "last_error").Updates(node).Error
	})
	if err != nil {
		return nil, err
	}
	if payload.Result == ApplyResultOK {
		slog.Debug("agent apply reported success", "node_id", payload.NodeID, "version", payload.Version)
	} else if payload.Result == ApplyResultWarning {
		slog.Warn("agent apply reported warning", "node_id", payload.NodeID, "version", payload.Version, "message", payload.Message)
	} else {
		slog.Error("agent apply reported failure", "node_id", payload.NodeID, "version", payload.Version, "message", payload.Message)
	}
	return log, nil
}

func ListNodeViews() ([]*NodeView, error) {
	nodes, err := model.ListNodes()
	if err != nil {
		return nil, err
	}
	nodeIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.NodeID)
	}
	latestLogs, err := model.GetLatestApplyLogsByNodeIDs(nodeIDs)
	if err != nil {
		return nil, err
	}
	views := make([]*NodeView, 0, len(nodes))
	for _, node := range nodes {
		computedStatus := computeNodeStatus(node)
		view := buildNodeView(node)
		view.Status = computedStatus
		if log, ok := latestLogs[node.NodeID]; ok {
			view.LatestApplyResult = log.Result
			view.LatestApplyMessage = log.Message
			view.LatestApplyChecksum = log.Checksum
			view.LatestMainConfigChecksum = log.MainConfigChecksum
			view.LatestRouteConfigChecksum = log.RouteConfigChecksum
			view.LatestSupportFileCount = log.SupportFileCount
			view.LatestApplyAt = &log.CreatedAt
		}
		views = append(views, view)
	}
	return views, nil
}

func truncateForDatabase(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

const (
	defaultApplyLogPageSize  = 20
	maxApplyLogPageSize      = 200
	maxApplyLogRetentionDays = 3650
)

func ListApplyLogsPage(input ApplyLogListQuery) (*ApplyLogListResult, error) {
	pageNo := normalizeApplyLogPageNo(input.PageNo)
	pageSize := normalizeApplyLogPageSize(input.PageSize)
	nodeID := strings.TrimSpace(input.NodeID)
	rows, err := model.ListApplyLogs(model.ApplyLogQuery{
		NodeID:   nodeID,
		PageNo:   pageNo,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	total, err := model.CountApplyLogs(nodeID)
	if err != nil {
		return nil, err
	}
	totalPage := 0
	if total > 0 {
		totalPage = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &ApplyLogListResult{
		Rows:      rows,
		Current:   pageNo,
		Total:     int(total),
		TotalPage: totalPage,
	}, nil
}

func CleanupApplyLogs(input ApplyLogCleanupInput) (*ApplyLogCleanupResult, error) {
	if input.DeleteAll {
		deleted, err := model.DeleteAllApplyLogs()
		if err != nil {
			return nil, err
		}
		return &ApplyLogCleanupResult{
			DeleteAll:    true,
			DeletedCount: deleted,
		}, nil
	}
	if input.RetentionDays <= 0 || input.RetentionDays > maxApplyLogRetentionDays {
		return nil, errors.New("retention_days 必须在 1 到 3650 之间")
	}
	cutoff := time.Now().UTC().Add(-time.Duration(input.RetentionDays) * 24 * time.Hour)
	deleted, err := model.DeleteApplyLogsBefore(cutoff)
	if err != nil {
		return nil, err
	}
	return &ApplyLogCleanupResult{
		RetentionDays: input.RetentionDays,
		DeletedCount:  deleted,
		Cutoff:        &cutoff,
	}, nil
}

func normalizeApplyLogPageNo(pageNo int) int {
	if pageNo <= 0 {
		return 1
	}
	return pageNo
}

func normalizeApplyLogPageSize(pageSize int) int {
	if pageSize <= 0 {
		return defaultApplyLogPageSize
	}
	if pageSize > maxApplyLogPageSize {
		return maxApplyLogPageSize
	}
	return pageSize
}

func computeNodeStatus(node *model.Node) string {
	if node == nil {
		return NodeStatusOffline
	}
	if IsAgentWSConnected(node.NodeID) {
		return NodeStatusOnline
	}
	if node.LastSeenAt.IsZero() {
		return NodeStatusPending
	}
	if time.Since(node.LastSeenAt) > common.NodeOfflineThreshold {
		return NodeStatusOffline
	}
	return NodeStatusOnline
}

func collectNodeHeartbeatChanges(previous *model.Node, current *model.Node) map[string]any {
	if previous == nil || current == nil {
		return map[string]any{}
	}
	changes := make(map[string]any)
	appendIfChanged := func(key string, before any, after any) {
		if before != after {
			changes[key] = after
		}
	}
	appendIfChanged("name", previous.Name, current.Name)
	appendIfChanged("ip", previous.IP, current.IP)
	appendIfChanged("geo_name", previous.GeoName, current.GeoName)
	appendIfChanged("agent_version", previous.AgentVersion, current.AgentVersion)
	appendIfChanged("nginx_version", previous.NginxVersion, current.NginxVersion)
	appendIfChanged("openresty_status", previous.OpenrestyStatus, current.OpenrestyStatus)
	appendIfChanged("openresty_message", previous.OpenrestyMessage, current.OpenrestyMessage)
	appendIfChanged("status", previous.Status, current.Status)
	appendIfChanged("current_version", previous.CurrentVersion, current.CurrentVersion)
	appendIfChanged("last_error", previous.LastError, current.LastError)
	appendIfChanged("update_requested", previous.UpdateRequested, current.UpdateRequested)
	appendIfChanged("update_channel", previous.UpdateChannel, current.UpdateChannel)
	appendIfChanged("update_tag", previous.UpdateTag, current.UpdateTag)
	appendIfChanged("restart_openresty_requested", previous.RestartOpenrestyRequested, current.RestartOpenrestyRequested)
	if !coordinatesEqual(previous.GeoLatitude, current.GeoLatitude) {
		changes["geo_latitude"] = current.GeoLatitude
	}
	if !coordinatesEqual(previous.GeoLongitude, current.GeoLongitude) {
		changes["geo_longitude"] = current.GeoLongitude
	}
	if !previous.LastSeenAt.Equal(current.LastSeenAt) {
		changes["last_seen_at"] = current.LastSeenAt
	}
	return changes
}

func coordinatesEqual(before *float64, after *float64) bool {
	if before == nil || after == nil {
		return before == after
	}
	return *before == *after
}
