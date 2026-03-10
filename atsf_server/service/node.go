package service

import (
	"atsflare/common"
	"atsflare/model"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type NodeInput struct {
	Name string `json:"name"`
}

type NodeBootstrapView struct {
	DiscoveryToken string `json:"discovery_token"`
}

type AgentRegistrationResponse struct {
	NodeID     string `json:"node_id"`
	AgentToken string `json:"agent_token"`
	Name       string `json:"name"`
}

func CreateNode(input NodeInput) (*NodeView, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("节点名不能为空")
	}
	node := &model.Node{
		Name:         name,
		IP:           "",
		AgentVersion: "",
		NginxVersion: "",
		Status:       NodeStatusPending,
	}
	var err error
	node.NodeID, err = newServerNodeID()
	if err != nil {
		return nil, err
	}
	node.AgentToken, err = newRandomToken()
	if err != nil {
		return nil, err
	}
	if err := node.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("节点标识生成冲突，请重试")
		}
		return nil, err
	}
	common.SysLog("node created: name=" + node.Name + " node_id=" + node.NodeID)
	return buildNodeView(node), nil
}

func UpdateNode(id uint, input NodeInput) (*NodeView, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("节点名不能为空")
	}
	node, err := model.GetNodeByID(id)
	if err != nil {
		return nil, err
	}
	node.Name = name
	if err = node.Update(); err != nil {
		return nil, err
	}
	common.SysLog("node updated: name=" + node.Name + " node_id=" + node.NodeID)
	return buildNodeView(node), nil
}

func DeleteNode(id uint) error {
	node, err := model.GetNodeByID(id)
	if err != nil {
		return err
	}
	common.SysLog("node deleted: name=" + node.Name + " node_id=" + node.NodeID)
	return node.Delete()
}

func AuthenticateAgentToken(token string) (*model.Node, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("缺少 Agent Token")
	}
	return model.GetNodeByAgentToken(token)
}

func ValidateDiscoveryToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("缺少 Discovery Token")
	}
	discoveryToken, err := EnsureGlobalDiscoveryToken()
	if err != nil {
		return err
	}
	if token != discoveryToken {
		return errors.New("Discovery Token 无效")
	}
	return nil
}

func EnsureGlobalDiscoveryToken() (string, error) {
	common.OptionMapRWMutex.RLock()
	needsInit := common.OptionMap == nil
	common.OptionMapRWMutex.RUnlock()
	if needsInit {
		model.InitOptionMap()
	}
	common.OptionMapRWMutex.RLock()
	token := strings.TrimSpace(common.OptionMap["AgentDiscoveryToken"])
	common.OptionMapRWMutex.RUnlock()
	if token != "" {
		return token, nil
	}
	token, err := newRandomToken()
	if err != nil {
		return "", err
	}
	if err = model.UpdateOption("AgentDiscoveryToken", token); err != nil {
		return "", err
	}
	return token, nil
}

func GetNodeBootstrapView() (*NodeBootstrapView, error) {
	token, err := EnsureGlobalDiscoveryToken()
	if err != nil {
		return nil, err
	}
	return &NodeBootstrapView{DiscoveryToken: token}, nil
}

func RotateGlobalDiscoveryToken() (*NodeBootstrapView, error) {
	token, err := newRandomToken()
	if err != nil {
		return nil, err
	}
	if err = model.UpdateOption("AgentDiscoveryToken", token); err != nil {
		return nil, err
	}
	return &NodeBootstrapView{DiscoveryToken: token}, nil
}

func buildNodeView(node *model.Node) *NodeView {
	status := computeNodeStatus(node)
	view := &NodeView{
		ID:             node.ID,
		NodeID:         node.NodeID,
		Name:           node.Name,
		IP:             node.IP,
		AgentToken:     node.AgentToken,
		AgentVersion:   node.AgentVersion,
		NginxVersion:   node.NginxVersion,
		Status:         status,
		CurrentVersion: node.CurrentVersion,
		LastSeenAt:     node.LastSeenAt,
		LastError:      node.LastError,
		CreatedAt:      node.CreatedAt,
		UpdatedAt:      node.UpdatedAt,
		Pending:        status == NodeStatusPending,
	}
	return view
}

func RegisterNodeWithAgentToken(node *model.Node, payload AgentNodePayload) (*AgentRegistrationResponse, error) {
	payload = normalizeAgentNodePayload(payload)
	if node == nil {
		return nil, errors.New("节点不存在")
	}
	if err := validateAgentNodePayload(payload); err != nil {
		return nil, err
	}
	applyNodeRuntime(node, payload, true)
	if err := node.Update(); err != nil {
		return nil, err
	}
	common.SysLog("agent register succeeded on reserved node: node_id=" + node.NodeID + " name=" + node.Name)
	return &AgentRegistrationResponse{
		NodeID:     node.NodeID,
		AgentToken: node.AgentToken,
		Name:       node.Name,
	}, nil
}

func RegisterNodeWithDiscovery(payload AgentNodePayload) (*AgentRegistrationResponse, error) {
	payload = normalizeAgentNodePayload(payload)
	if err := validateAgentNodePayload(payload); err != nil {
		return nil, err
	}
	nodeID, err := newServerNodeID()
	if err != nil {
		return nil, err
	}
	agentToken, err := newRandomToken()
	if err != nil {
		return nil, err
	}
	nodeName := payload.Name
	if nodeName == "" {
		nodeName = nodeID
	}
	node := &model.Node{
		NodeID:     nodeID,
		Name:       nodeName,
		AgentToken: agentToken,
	}
	applyNodeRuntime(node, payload, false)
	if err = node.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("节点标识生成冲突，请重试")
		}
		return nil, err
	}
	common.SysLog("agent discovery register succeeded: node_id=" + node.NodeID + " name=" + node.Name)
	return &AgentRegistrationResponse{
		NodeID:     node.NodeID,
		AgentToken: node.AgentToken,
		Name:       node.Name,
	}, nil
}

func normalizeAgentNodePayload(payload AgentNodePayload) AgentNodePayload {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.IP = strings.TrimSpace(payload.IP)
	payload.AgentVersion = strings.TrimSpace(payload.AgentVersion)
	payload.NginxVersion = strings.TrimSpace(payload.NginxVersion)
	payload.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	payload.LastError = strings.TrimSpace(payload.LastError)
	return payload
}

func validateAgentNodePayload(payload AgentNodePayload) error {
	if payload.IP == "" {
		return errors.New("ip 不能为空")
	}
	if payload.AgentVersion == "" {
		return errors.New("agent_version 不能为空")
	}
	return nil
}

func applyNodeRuntime(node *model.Node, payload AgentNodePayload, preserveName bool) {
	if !preserveName || strings.TrimSpace(node.Name) == "" {
		if strings.TrimSpace(payload.Name) != "" {
			node.Name = strings.TrimSpace(payload.Name)
		}
	}
	node.IP = strings.TrimSpace(payload.IP)
	node.AgentVersion = strings.TrimSpace(payload.AgentVersion)
	node.NginxVersion = strings.TrimSpace(payload.NginxVersion)
	node.Status = NodeStatusOnline
	node.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	node.LastSeenAt = time.Now()
	node.LastError = strings.TrimSpace(payload.LastError)
}

func newRandomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func newServerNodeID() (string, error) {
	token, err := newRandomToken()
	if err != nil {
		return "", err
	}
	return "node-" + token, nil
}
