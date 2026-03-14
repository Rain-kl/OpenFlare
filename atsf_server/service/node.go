package service

import (
	"atsflare/common"
	"atsflare/model"
	"atsflare/utils/geoip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"
)

type NodeInput struct {
	Name              string   `json:"name"`
	IP                string   `json:"ip"`
	AutoUpdateEnabled bool     `json:"auto_update_enabled"`
	GeoName           string   `json:"geo_name"`
	GeoLatitude       *float64 `json:"geo_latitude"`
	GeoLongitude      *float64 `json:"geo_longitude"`
	GeoManualOverride bool     `json:"geo_manual_override"`
}

type NodeAgentUpdateInput struct {
	Channel string `json:"channel"`
	TagName string `json:"tag_name"`
}

type NodeAgentReleaseInfo struct {
	TagName          string `json:"tag_name"`
	Body             string `json:"body"`
	HTMLURL          string `json:"html_url"`
	PublishedAt      string `json:"published_at"`
	CurrentVersion   string `json:"current_version"`
	HasUpdate        bool   `json:"has_update"`
	Channel          string `json:"channel"`
	Prerelease       bool   `json:"prerelease"`
	UpdateRequested  bool   `json:"update_requested"`
	RequestedChannel string `json:"requested_channel"`
	RequestedTag     string `json:"requested_tag"`
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
	name, ip, geoName, geoLatitude, geoLongitude, geoManualOverride, err := normalizeNodeInput(input)
	if name == "" {
		return nil, errors.New("节点名不能为空")
	}
	node := &model.Node{
		Name:              name,
		IP:                ip,
		GeoName:           geoName,
		GeoLatitude:       geoLatitude,
		GeoLongitude:      geoLongitude,
		GeoManualOverride: geoManualOverride,
		AgentVersion:      "",
		NginxVersion:      "",
		Status:            NodeStatusPending,
		AutoUpdateEnabled: input.AutoUpdateEnabled,
	}
	node.NodeID, err = newServerNodeID()
	if err != nil {
		return nil, err
	}
	node.AgentToken, err = newRandomToken()
	if err != nil {
		return nil, err
	}
	if !node.GeoManualOverride {
		applyGeoInfoFromIP(node, node.IP)
	}
	if err := node.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("节点标识生成冲突，请重试")
		}
		return nil, err
	}
	refreshAgentTokenCache(node)
	slog.Info("node created", "name", node.Name, "node_id", node.NodeID)
	return buildNodeView(node), nil
}

func UpdateNode(id uint, input NodeInput) (*NodeView, error) {
	name, ip, geoName, geoLatitude, geoLongitude, geoManualOverride, err := normalizeNodeInput(input)
	if name == "" {
		return nil, errors.New("节点名不能为空")
	}
	node, err := model.GetNodeByID(id)
	if err != nil {
		return nil, err
	}
	node.Name = name
	node.IP = ip
	node.GeoName = geoName
	node.GeoLatitude = geoLatitude
	node.GeoLongitude = geoLongitude
	node.GeoManualOverride = geoManualOverride
	node.AutoUpdateEnabled = input.AutoUpdateEnabled
	if !node.GeoManualOverride {
		applyGeoInfoFromIP(node, strings.TrimSpace(node.IP))
	}
	if err = node.Update(); err != nil {
		return nil, err
	}
	refreshAgentTokenCache(node)
	slog.Info("node updated", "name", node.Name, "node_id", node.NodeID)
	return buildNodeView(node), nil
}

func DeleteNode(id uint) error {
	node, err := model.GetNodeByID(id)
	if err != nil {
		return err
	}
	slog.Info("node deleted", "name", node.Name, "node_id", node.NodeID)
	if err := node.Delete(); err != nil {
		return err
	}
	invalidateAgentTokenCache(node.AgentToken)
	return nil
}

func GetNodeAgentRelease(ctx context.Context, id uint, channel string) (*NodeAgentReleaseInfo, error) {
	node, err := model.GetNodeByID(id)
	if err != nil {
		return nil, err
	}
	release, err := fetchLatestGitHubRelease(ctx, common.AgentUpdateRepo, normalizeReleaseChannel(channel))
	if err != nil {
		return nil, err
	}
	return buildNodeAgentReleaseView(node, release, normalizeReleaseChannel(channel)), nil
}

func RequestNodeAgentUpdate(id uint, input NodeAgentUpdateInput) (*NodeView, error) {
	node, err := model.GetNodeByID(id)
	if err != nil {
		return nil, err
	}
	channel := normalizeReleaseChannel(input.Channel)
	tagName := strings.TrimSpace(input.TagName)
	if tagName != "" {
		release, releaseErr := fetchGitHubReleaseByTag(context.Background(), common.AgentUpdateRepo, tagName)
		if releaseErr != nil {
			return nil, releaseErr
		}
		if channel == ReleaseChannelPreview && !release.Prerelease {
			return nil, errors.New("指定版本不是 preview 发布")
		}
		if channel == ReleaseChannelStable && release.Prerelease {
			return nil, errors.New("正式版更新不能选择 preview 发布")
		}
	}
	node.UpdateRequested = true
	node.UpdateChannel = channel.String()
	node.UpdateTag = tagName
	if err = model.DB.Model(node).Select("update_requested", "update_channel", "update_tag").Updates(node).Error; err != nil {
		return nil, err
	}
	refreshAgentTokenCache(node)
	slog.Info("agent manual update requested", "node_id", node.NodeID, "name", node.Name, "channel", channel.String(), "tag", tagName)
	return buildNodeView(node), nil
}

func RequestNodeOpenrestyRestart(id uint) (*NodeView, error) {
	node, err := model.GetNodeByID(id)
	if err != nil {
		return nil, err
	}
	node.RestartOpenrestyRequested = true
	if err = model.DB.Model(node).Select("restart_openresty_requested").Updates(node).Error; err != nil {
		return nil, err
	}
	refreshAgentTokenCache(node)
	slog.Info("openresty restart requested", "node_id", node.NodeID, "name", node.Name)
	return buildNodeView(node), nil
}

func AuthenticateAgentToken(token string) (*model.Node, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("缺少 Agent Token")
	}
	return authenticateAgentTokenWithCache(token)
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
		ID:                        node.ID,
		NodeID:                    node.NodeID,
		Name:                      node.Name,
		IP:                        node.IP,
		GeoName:                   strings.TrimSpace(node.GeoName),
		GeoLatitude:               node.GeoLatitude,
		GeoLongitude:              node.GeoLongitude,
		GeoManualOverride:         node.GeoManualOverride,
		AgentToken:                node.AgentToken,
		UpdateChannel:             strings.TrimSpace(node.UpdateChannel),
		UpdateTag:                 strings.TrimSpace(node.UpdateTag),
		RestartOpenrestyRequested: node.RestartOpenrestyRequested,
		AgentVersion:              node.AgentVersion,
		NginxVersion:              node.NginxVersion,
		OpenrestyStatus:           normalizeOpenrestyStatus(node.OpenrestyStatus),
		OpenrestyMessage:          strings.TrimSpace(node.OpenrestyMessage),
		Status:                    status,
		CurrentVersion:            node.CurrentVersion,
		LastSeenAt:                node.LastSeenAt,
		LastError:                 node.LastError,
		CreatedAt:                 node.CreatedAt,
		UpdatedAt:                 node.UpdatedAt,
		AutoUpdateEnabled:         node.AutoUpdateEnabled,
		UpdateRequested:           node.UpdateRequested,
	}
	if view.UpdateChannel == "" {
		view.UpdateChannel = ReleaseChannelStable.String()
	}
	return view
}

func normalizeNodeInput(input NodeInput) (string, string, string, *float64, *float64, bool, error) {
	name := strings.TrimSpace(input.Name)
	ip := strings.TrimSpace(input.IP)
	geoName := strings.TrimSpace(input.GeoName)
	manualOverride := input.GeoManualOverride || geoName != "" || input.GeoLatitude != nil || input.GeoLongitude != nil
	if len(ip) > 64 {
		return "", "", "", nil, nil, false, errors.New("节点 IP 不能超过 64 个字符")
	}
	if ip != "" && net.ParseIP(ip) == nil {
		return "", "", "", nil, nil, false, errors.New("节点 IP 格式无效")
	}
	if len(geoName) > 128 {
		return "", "", "", nil, nil, false, errors.New("节点位置名不能超过 128 个字符")
	}

	geoLatitude := cloneCoordinate(input.GeoLatitude)
	geoLongitude := cloneCoordinate(input.GeoLongitude)
	if (geoLatitude == nil) != (geoLongitude == nil) {
		return "", "", "", nil, nil, false, errors.New("地图坐标必须同时填写纬度和经度")
	}
	if geoLatitude != nil && (*geoLatitude < -90 || *geoLatitude > 90) {
		return "", "", "", nil, nil, false, errors.New("纬度必须在 -90 到 90 之间")
	}
	if geoLongitude != nil && (*geoLongitude < -180 || *geoLongitude > 180) {
		return "", "", "", nil, nil, false, errors.New("经度必须在 -180 到 180 之间")
	}

	if !manualOverride {
		return name, ip, "", nil, nil, false, nil
	}
	if geoLatitude == nil && geoLongitude == nil && geoName == "" {
		return name, ip, "", nil, nil, false, nil
	}

	return name, ip, geoName, geoLatitude, geoLongitude, true, nil
}

func cloneCoordinate(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func buildNodeAgentReleaseView(node *model.Node, release *githubReleaseResponse, channel ReleaseChannel) *NodeAgentReleaseInfo {
	currentVersion := strings.TrimSpace(node.AgentVersion)
	view := &NodeAgentReleaseInfo{
		CurrentVersion:   currentVersion,
		Channel:          channel.String(),
		UpdateRequested:  node.UpdateRequested,
		RequestedChannel: normalizeReleaseChannel(node.UpdateChannel).String(),
		RequestedTag:     strings.TrimSpace(node.UpdateTag),
	}
	if release == nil {
		return view
	}
	view.TagName = release.TagName
	view.Body = release.Body
	view.HTMLURL = release.HTMLURL
	view.PublishedAt = release.PublishedAt
	view.Prerelease = release.Prerelease
	view.HasUpdate = isVersionNewer(currentVersion, release.TagName)
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
	refreshAgentTokenCache(node)
	slog.Info("agent register succeeded on reserved node", "node_id", node.NodeID, "name", node.Name)
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
	refreshAgentTokenCache(node)
	slog.Info("agent discovery register succeeded", "node_id", node.NodeID, "name", node.Name)
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
	payload.OpenrestyStatus = normalizeOpenrestyStatus(payload.OpenrestyStatus)
	payload.OpenrestyMessage = strings.TrimSpace(payload.OpenrestyMessage)
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
	node.OpenrestyStatus = normalizeOpenrestyStatus(payload.OpenrestyStatus)
	node.OpenrestyMessage = strings.TrimSpace(payload.OpenrestyMessage)
	node.Status = NodeStatusOnline
	node.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	node.LastSeenAt = time.Now()
	node.LastError = strings.TrimSpace(payload.LastError)
	if !node.GeoManualOverride {
		applyGeoInfoFromIP(node, node.IP)
	}
}

func applyGeoInfoFromIP(node *model.Node, rawIP string) {
	if node == nil {
		return
	}
	node.GeoName = ""
	node.GeoLatitude = nil
	node.GeoLongitude = nil
	ip := net.ParseIP(strings.TrimSpace(rawIP))
	if ip == nil {
		return
	}
	info, err := geoip.GetGeoInfo(ip)
	if err != nil || info == nil {
		return
	}
	if strings.TrimSpace(info.Name) != "" {
		node.GeoName = strings.TrimSpace(info.Name)
	}
	if info.Latitude != nil && info.Longitude != nil {
		node.GeoLatitude = cloneCoordinate(info.Latitude)
		node.GeoLongitude = cloneCoordinate(info.Longitude)
	}
}

func normalizeOpenrestyStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case OpenrestyStatusHealthy:
		return OpenrestyStatusHealthy
	case OpenrestyStatusUnhealthy:
		return OpenrestyStatusUnhealthy
	default:
		return OpenrestyStatusUnknown
	}
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
