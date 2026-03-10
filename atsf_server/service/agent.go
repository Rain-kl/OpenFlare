package service

import (
	"encoding/json"
	"errors"
	"gin-template/common"
	"gin-template/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	NodeStatusOnline  = "online"
	NodeStatusOffline = "offline"
	ApplyResultOK     = "success"
	ApplyResultFailed = "failed"
)

type AgentNodePayload struct {
	NodeID         string `json:"node_id"`
	Name           string `json:"name"`
	IP             string `json:"ip"`
	AgentVersion   string `json:"agent_version"`
	NginxVersion   string `json:"nginx_version"`
	CurrentVersion string `json:"current_version"`
	LastError      string `json:"last_error"`
}

type ApplyLogPayload struct {
	NodeID  string `json:"node_id"`
	Version string `json:"version"`
	Result  string `json:"result"`
	Message string `json:"message"`
}

type AgentConfigResponse struct {
	Version        string        `json:"version"`
	Checksum       string        `json:"checksum"`
	RenderedConfig string        `json:"rendered_config"`
	SupportFiles   []SupportFile `json:"support_files"`
	CreatedAt      time.Time     `json:"created_at"`
}

type NodeView struct {
	ID                 uint       `json:"id"`
	NodeID             string     `json:"node_id"`
	Name               string     `json:"name"`
	IP                 string     `json:"ip"`
	AgentVersion       string     `json:"agent_version"`
	NginxVersion       string     `json:"nginx_version"`
	Status             string     `json:"status"`
	CurrentVersion     string     `json:"current_version"`
	LastSeenAt         time.Time  `json:"last_seen_at"`
	LastError          string     `json:"last_error"`
	LatestApplyResult  string     `json:"latest_apply_result"`
	LatestApplyMessage string     `json:"latest_apply_message"`
	LatestApplyAt      *time.Time `json:"latest_apply_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func RegisterNode(payload AgentNodePayload) (*model.Node, error) {
	return upsertNode(payload)
}

func HeartbeatNode(payload AgentNodePayload) (*model.Node, error) {
	return upsertNode(payload)
}

func GetActiveConfigForAgent() (*AgentConfigResponse, error) {
	version, err := model.GetActiveConfigVersion()
	if err != nil {
		return nil, err
	}
	var supportFiles []SupportFile
	if version.SupportFilesJSON != "" {
		if err = json.Unmarshal([]byte(version.SupportFilesJSON), &supportFiles); err != nil {
			return nil, err
		}
	}
	return &AgentConfigResponse{
		Version:        version.Version,
		Checksum:       version.Checksum,
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
	if payload.NodeID == "" {
		return nil, errors.New("node_id 不能为空")
	}
	if payload.Version == "" {
		return nil, errors.New("version 不能为空")
	}
	if payload.Result != ApplyResultOK && payload.Result != ApplyResultFailed {
		return nil, errors.New("result 仅支持 success 或 failed")
	}

	log := &model.ApplyLog{
		NodeID:    payload.NodeID,
		Version:   payload.Version,
		Result:    payload.Result,
		Message:   payload.Message,
		CreatedAt: now,
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
	return log, nil
}

func ListNodeViews() ([]*NodeView, error) {
	nodes, err := model.ListNodes()
	if err != nil {
		return nil, err
	}
	views := make([]*NodeView, 0, len(nodes))
	for _, node := range nodes {
		view := &NodeView{
			ID:             node.ID,
			NodeID:         node.NodeID,
			Name:           node.Name,
			IP:             node.IP,
			AgentVersion:   node.AgentVersion,
			NginxVersion:   node.NginxVersion,
			Status:         computeNodeStatus(node.LastSeenAt),
			CurrentVersion: node.CurrentVersion,
			LastSeenAt:     node.LastSeenAt,
			LastError:      node.LastError,
			CreatedAt:      node.CreatedAt,
			UpdatedAt:      node.UpdatedAt,
		}
		if log, err := model.GetLatestApplyLog(node.NodeID); err == nil {
			view.LatestApplyResult = log.Result
			view.LatestApplyMessage = log.Message
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
	now := time.Now()
	payload.NodeID = strings.TrimSpace(payload.NodeID)
	payload.Name = strings.TrimSpace(payload.Name)
	payload.IP = strings.TrimSpace(payload.IP)
	payload.AgentVersion = strings.TrimSpace(payload.AgentVersion)
	payload.NginxVersion = strings.TrimSpace(payload.NginxVersion)
	payload.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	payload.LastError = strings.TrimSpace(payload.LastError)
	if payload.NodeID == "" {
		return nil, errors.New("node_id 不能为空")
	}
	if payload.Name == "" {
		return nil, errors.New("name 不能为空")
	}
	if payload.IP == "" {
		return nil, errors.New("ip 不能为空")
	}
	if payload.AgentVersion == "" {
		return nil, errors.New("agent_version 不能为空")
	}

	node := &model.Node{}
	err := model.DB.Where("node_id = ?", payload.NodeID).First(node).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		node = &model.Node{
			NodeID: payload.NodeID,
		}
	}
	node.Name = payload.Name
	node.IP = payload.IP
	node.AgentVersion = payload.AgentVersion
	node.NginxVersion = payload.NginxVersion
	node.Status = NodeStatusOnline
	node.CurrentVersion = payload.CurrentVersion
	node.LastSeenAt = now
	node.LastError = payload.LastError
	if node.ID == 0 {
		if err = model.DB.Create(node).Error; err != nil {
			return nil, err
		}
		return node, nil
	}
	if err = model.DB.Model(node).Select("name", "ip", "agent_version", "nginx_version", "status", "current_version", "last_seen_at", "last_error").Updates(node).Error; err != nil {
		return nil, err
	}
	return node, nil
}

func computeNodeStatus(lastSeenAt time.Time) string {
	if lastSeenAt.IsZero() {
		return NodeStatusOffline
	}
	if time.Since(lastSeenAt) > common.NodeOfflineThreshold {
		return NodeStatusOffline
	}
	return NodeStatusOnline
}
