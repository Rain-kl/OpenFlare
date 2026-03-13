package service

import (
	"atsflare/common"
	"atsflare/model"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	NodeStatusOnline         = "online"
	NodeStatusOffline        = "offline"
	NodeStatusPending        = "pending"
	ApplyResultOK            = "success"
	ApplyResultFailed        = "failed"
	OpenrestyStatusHealthy   = "healthy"
	OpenrestyStatusUnhealthy = "unhealthy"
	OpenrestyStatusUnknown   = "unknown"
)

type AgentNodePayload struct {
	NodeID           string `json:"node_id"`
	Name             string `json:"name"`
	IP               string `json:"ip"`
	AgentVersion     string `json:"agent_version"`
	NginxVersion     string `json:"nginx_version"`
	CurrentVersion   string `json:"current_version"`
	LastError        string `json:"last_error"`
	OpenrestyStatus  string `json:"openresty_status"`
	OpenrestyMessage string `json:"openresty_message"`
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

type AgentConfigResponse struct {
	Version        string        `json:"version"`
	Checksum       string        `json:"checksum"`
	MainConfig     string        `json:"main_config"`
	RouteConfig    string        `json:"route_config"`
	RenderedConfig string        `json:"rendered_config"`
	SupportFiles   []SupportFile `json:"support_files"`
	CreatedAt      time.Time     `json:"created_at"`
}

type AgentSettings struct {
	HeartbeatInterval   int    `json:"heartbeat_interval"`
	AutoUpdate          bool   `json:"auto_update"`
	UpdateRepo          string `json:"update_repo"`
	UpdateNow           bool   `json:"update_now"`
	UpdateChannel       string `json:"update_channel"`
	UpdateTag           string `json:"update_tag"`
	RestartOpenrestyNow bool   `json:"restart_openresty_now"`
}

type ActiveConfigMeta struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

type HeartbeatResponse struct {
	Node          *model.Node       `json:"node"`
	AgentSettings *AgentSettings    `json:"agent_settings"`
	ActiveConfig  *ActiveConfigMeta `json:"active_config"`
}

type NodeView struct {
	ID                        uint       `json:"id"`
	NodeID                    string     `json:"node_id"`
	Name                      string     `json:"name"`
	IP                        string     `json:"ip"`
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
	LastSeenAt                time.Time  `json:"last_seen_at"`
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
}

func RegisterNode(node *model.Node, payload AgentNodePayload) (*AgentRegistrationResponse, error) {
	return RegisterNodeWithAgentToken(node, payload)
}

func HeartbeatNode(node *model.Node, payload AgentNodePayload) (*HeartbeatResponse, error) {
	common.SysLog("agent heartbeat received: node_id=" + node.NodeID + " current_version=" + strings.TrimSpace(payload.CurrentVersion))
	payload.NodeID = node.NodeID
	payload = normalizeAgentNodePayload(payload)
	if err := validateAgentNodePayload(payload); err != nil {
		return nil, err
	}
	updateNow := node.UpdateRequested
	restartOpenrestyNow := node.RestartOpenrestyRequested
	updateChannel := normalizeReleaseChannel(node.UpdateChannel)
	updateTag := strings.TrimSpace(node.UpdateTag)
	applyNodeRuntime(node, payload, true)
	node.UpdateRequested = false
	node.UpdateChannel = ReleaseChannelStable.String()
	node.UpdateTag = ""
	node.RestartOpenrestyRequested = false
	if err := model.DB.Model(node).Select("ip", "agent_version", "nginx_version", "openresty_status", "openresty_message", "status", "current_version", "last_seen_at", "last_error", "update_requested", "update_channel", "update_tag", "restart_openresty_requested").Updates(node).Error; err != nil {
		return nil, err
	}
	activeConfig, err := GetActiveConfigMetaForAgent()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &HeartbeatResponse{
		Node: node,
		AgentSettings: &AgentSettings{
			HeartbeatInterval:   common.AgentHeartbeatInterval,
			AutoUpdate:          node.AutoUpdateEnabled,
			UpdateRepo:          common.AgentUpdateRepo,
			UpdateNow:           updateNow,
			UpdateChannel:       updateChannel.String(),
			UpdateTag:           updateTag,
			RestartOpenrestyNow: restartOpenrestyNow,
		},
		ActiveConfig: activeConfig,
	}, nil
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
		common.SysError("agent requested active config but no active version is available")
		return nil, err
	}
	var supportFiles []SupportFile
	if version.SupportFilesJSON != "" {
		if err = json.Unmarshal([]byte(version.SupportFilesJSON), &supportFiles); err != nil {
			return nil, err
		}
	}
	common.SysLog("agent fetched active config: version=" + version.Version + " checksum=" + version.Checksum)
	return &AgentConfigResponse{
		Version:        version.Version,
		Checksum:       version.Checksum,
		MainConfig:     version.MainConfig,
		RouteConfig:    version.RenderedConfig,
		RenderedConfig: version.RenderedConfig,
		SupportFiles:   supportFiles,
		CreatedAt:      version.CreatedAt,
	}, nil
}

func ReportApplyLog(payload ApplyLogPayload) (*model.ApplyLog, error) {
	now := time.Now()
	payload.NodeID = strings.TrimSpace(payload.NodeID)
	payload.Version = strings.TrimSpace(payload.Version)
	payload.Result = strings.TrimSpace(strings.ToLower(payload.Result))
	payload.Message = strings.TrimSpace(payload.Message)
	payload.Checksum = strings.TrimSpace(payload.Checksum)
	payload.MainConfigChecksum = strings.TrimSpace(payload.MainConfigChecksum)
	payload.RouteConfigChecksum = strings.TrimSpace(payload.RouteConfigChecksum)
	if payload.NodeID == "" {
		return nil, errors.New("node_id 不能为空")
	}
	if payload.Version == "" {
		return nil, errors.New("version 不能为空")
	}
	if payload.Result != ApplyResultOK && payload.Result != ApplyResultFailed {
		return nil, errors.New("result 仅支持 success 或 failed")
	}
	common.SysLog("agent apply log received: node_id=" + payload.NodeID + " version=" + payload.Version + " result=" + payload.Result)

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
		common.SysLog("agent apply reported success: node_id=" + payload.NodeID + " version=" + payload.Version)
	} else {
		common.SysError("agent apply reported failure: node_id=" + payload.NodeID + " version=" + payload.Version + " message=" + payload.Message)
	}
	return log, nil
}

func ListNodeViews() ([]*NodeView, error) {
	nodes, err := model.ListNodes()
	if err != nil {
		return nil, err
	}
	views := make([]*NodeView, 0, len(nodes))
	for _, node := range nodes {
		computedStatus := computeNodeStatus(node)
		if node.Status != computedStatus {
			if computedStatus == NodeStatusOffline {
				common.SysError("node offline: node_id=" + node.NodeID + " name=" + node.Name + " ip=" + node.IP + " last_seen_at=" + node.LastSeenAt.Format(time.RFC3339))
			} else if computedStatus == NodeStatusOnline {
				common.SysLog("node online: node_id=" + node.NodeID + " name=" + node.Name + " ip=" + node.IP)
			}
			_ = model.DB.Model(node).Update("status", computedStatus).Error
			node.Status = computedStatus
		}
		view := buildNodeView(node)
		view.Status = computedStatus
		if log, err := model.GetLatestApplyLog(node.NodeID); err == nil {
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

func ListApplyLogs(nodeID string) ([]*model.ApplyLog, error) {
	return model.ListApplyLogs(strings.TrimSpace(nodeID))
}

func upsertNode(payload AgentNodePayload) (*model.Node, error) {
	return nil, errors.New("不再支持匿名自动注册")
}

func computeNodeStatus(node *model.Node) string {
	if node == nil {
		return NodeStatusOffline
	}
	if node.LastSeenAt.IsZero() {
		return NodeStatusPending
	}
	if time.Since(node.LastSeenAt) > common.NodeOfflineThreshold {
		return NodeStatusOffline
	}
	return NodeStatusOnline
}
