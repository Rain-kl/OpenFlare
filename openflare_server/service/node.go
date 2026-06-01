package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"openflare/common"
	"openflare/model"
	"openflare/utils/geoip"
	"openflare/utils/geoip/iputil"
	"strings"
	"time"
)

type NodeInput struct {
	Name              string   `json:"name"`
	IP                string   `json:"ip"`
	IPManualOverride  *bool    `json:"ip_manual_override"`
	AutoUpdateEnabled bool     `json:"auto_update_enabled"`
	GeoName           string   `json:"geo_name"`
	GeoLatitude       *float64 `json:"geo_latitude"`
	GeoLongitude      *float64 `json:"geo_longitude"`
	GeoManualOverride bool     `json:"geo_manual_override"`
	// TunnelRelay fields
	NodeType              string `json:"node_type"`
	RelayBindPort         int    `json:"relay_bind_port"`
	RelayVhostHTTPPort    int    `json:"relay_vhost_http_port"`
	RelayAgentAccessAddr  string `json:"relay_agent_access_addr"`
	RelayClientAccessAddr string `json:"relay_client_access_addr"`
	RelayClientProxyURL   string `json:"relay_client_proxy_url"`
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
	NodeID      string `json:"node_id"`
	AccessToken string `json:"access_token"`
	Name        string `json:"name"`
}

func CreateNode(input NodeInput) (*NodeView, error) {
	name, ip, geoName, geoLatitude, geoLongitude, geoManualOverride, err := normalizeNodeInput(input)
	if name == "" {
		return nil, errors.New("节点名不能为空")
	}
	ipManualOverride := resolveNodeIPManualOverride(input, nil, ip)
	node := &model.Node{
		Name:              name,
		IP:                ip,
		IPManualOverride:  ipManualOverride,
		GeoName:           geoName,
		GeoLatitude:       geoLatitude,
		GeoLongitude:      geoLongitude,
		GeoManualOverride: geoManualOverride,
		Version:           "",
		ExtVersion:        "",
		Status:            NodeStatusPending,
		AutoUpdateEnabled: input.AutoUpdateEnabled,
		NodeType:          normalizeNodeType(input.NodeType),
	}
	node.NodeID, err = newServerNodeID()
	if err != nil {
		return nil, err
	}
	node.AccessToken, err = newRandomToken()
	if err != nil {
		return nil, err
	}
	if node.NodeType == "tunnel_relay" {
		node.RelayBindPort = normalizeRelayPort(input.RelayBindPort, 7000)
		node.RelayVhostHTTPPort = normalizeRelayPort(input.RelayVhostHTTPPort, 8080)
		node.RelayAuthToken, err = newRandomToken()
		if err != nil {
			return nil, err
		}
		node.RelayAgentAccessAddr = strings.TrimSpace(input.RelayAgentAccessAddr)
		node.RelayClientAccessAddr = strings.TrimSpace(input.RelayClientAccessAddr)
		node.RelayClientProxyURL = strings.TrimSpace(input.RelayClientProxyURL)
	}
	if !node.GeoManualOverride {
		applyGeoInfoFromIP(node, node.IP)
	}
	if err := node.Insert(); err != nil {
		if model.IsUniqueConstraintError(err) {
			return nil, errors.New("节点标识生成冲突，请重试")
		}
		return nil, err
	}
	refreshAccessTokenCache(node)
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
	ipManualOverride := resolveNodeIPManualOverride(input, node, ip)
	node.Name = name
	node.IP = ip
	node.IPManualOverride = ipManualOverride
	node.GeoName = geoName
	node.GeoLatitude = geoLatitude
	node.GeoLongitude = geoLongitude
	node.GeoManualOverride = geoManualOverride
	node.AutoUpdateEnabled = input.AutoUpdateEnabled
	if node.NodeType == "tunnel_relay" {
		node.RelayAgentAccessAddr = strings.TrimSpace(input.RelayAgentAccessAddr)
		node.RelayClientAccessAddr = strings.TrimSpace(input.RelayClientAccessAddr)
		node.RelayClientProxyURL = strings.TrimSpace(input.RelayClientProxyURL)
		if input.RelayBindPort > 0 {
			node.RelayBindPort = input.RelayBindPort
		}
		if input.RelayVhostHTTPPort > 0 {
			node.RelayVhostHTTPPort = input.RelayVhostHTTPPort
		}
	}
	if !node.GeoManualOverride {
		applyGeoInfoFromIP(node, strings.TrimSpace(node.IP))
	}
	if err = node.Update(); err != nil {
		return nil, err
	}
	refreshAccessTokenCache(node)
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
	invalidateAccessTokenCache(node.AccessToken)
	DisconnectAgentWSClient(node.NodeID)
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
	refreshAccessTokenCache(node)
	if SendAgentWSSettings(node.NodeID, buildAgentSettings(node, true, channel.String(), tagName, node.RestartOpenrestyRequested)) {
		slog.Debug("agent manual update pushed via ws", "node_id", node.NodeID, "channel", channel.String(), "tag", tagName)
	} else {
		slog.Debug("agent manual update waiting for next heartbeat", "node_id", node.NodeID, "channel", channel.String(), "tag", tagName)
	}
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
	refreshAccessTokenCache(node)
	slog.Info("openresty restart requested", "node_id", node.NodeID, "name", node.Name)
	return buildNodeView(node), nil
}

func RequestNodeForceSync(id uint) (*NodeView, error) {
	node, err := model.GetNodeByID(id)
	if err != nil {
		return nil, err
	}
	activeConfig, err := GetActiveConfigMetaForAgent()
	if err != nil {
		return nil, errors.New("无法获取当前激活的配置版本：" + err.Error())
	}
	if !SendAgentWSForceSyncConfig(node.NodeID, activeConfig) {
		return nil, errors.New("节点不在线或通过 WebSocket 发送同步指令失败")
	}
	slog.Info("force sync requested via ws", "node_id", node.NodeID, "name", node.Name)
	return buildNodeView(node), nil
}

func AuthenticateAccessToken(token string) (*model.Node, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("缺少 Agent Token")
	}
	return authenticateAccessTokenWithCache(token)
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
	token := strings.TrimSpace(common.AgentDiscoveryToken)
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
		IPManualOverride:          node.IPManualOverride,
		GeoName:                   strings.TrimSpace(node.GeoName),
		GeoLatitude:               node.GeoLatitude,
		GeoLongitude:              node.GeoLongitude,
		GeoManualOverride:         node.GeoManualOverride,
		AccessToken:               node.AccessToken,
		UpdateChannel:             strings.TrimSpace(node.UpdateChannel),
		UpdateTag:                 strings.TrimSpace(node.UpdateTag),
		RestartOpenrestyRequested: node.RestartOpenrestyRequested,
		Version:                   node.Version,
		ExtVersion:                node.ExtVersion,
		OpenrestyStatus:           normalizeOpenrestyStatus(node.OpenrestyStatus),
		OpenrestyMessage:          strings.TrimSpace(node.OpenrestyMessage),
		Status:                    status,
		CurrentVersion:            node.CurrentVersion,
		LastSeenAt:                nodeViewLastSeenAt(node),
		LastError:                 node.LastError,
		CreatedAt:                 node.CreatedAt,
		UpdatedAt:                 node.UpdatedAt,
		AutoUpdateEnabled:         node.AutoUpdateEnabled,
		UpdateRequested:           node.UpdateRequested,
	}
	if view.UpdateChannel == "" {
		view.UpdateChannel = ReleaseChannelStable.String()
	}
	view.NodeType = node.NodeType
	if view.NodeType == "" {
		view.NodeType = "edge_node"
	}
	view.RelayBindPort = node.RelayBindPort
	view.RelayVhostHTTPPort = node.RelayVhostHTTPPort
	view.RelayAgentAccessAddr = node.RelayAgentAccessAddr
	view.RelayClientAccessAddr = node.RelayClientAccessAddr
	view.RelayClientProxyURL = node.RelayClientProxyURL
	view.RelayStatus = node.RelayStatus
	view.Version = node.Version
	view.ExtVersion = node.ExtVersion
	return view
}

func nodeViewLastSeenAt(node *model.Node) any {
	if node != nil && IsAgentWSConnected(node.NodeID) {
		return AgentWSConnectedLastSeenValue
	}
	if node == nil {
		return time.Time{}
	}
	return node.LastSeenAt
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
	if input.IPManualOverride != nil && *input.IPManualOverride && ip == "" {
		return "", "", "", nil, nil, false, errors.New("锁定节点 IP 时必须填写节点 IP")
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

func resolveNodeIPManualOverride(input NodeInput, existing *model.Node, normalizedIP string) bool {
	if input.IPManualOverride != nil {
		return *input.IPManualOverride
	}
	if existing == nil {
		return strings.TrimSpace(normalizedIP) != ""
	}
	if existing.IPManualOverride {
		return true
	}
	return strings.TrimSpace(normalizedIP) != "" && strings.TrimSpace(normalizedIP) != strings.TrimSpace(existing.IP)
}

func cloneCoordinate(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func ResolveReportedNodeIP(reportedIP string, remoteAddr string) string {
	reported := iputil.NormalizeIP(reportedIP)
	remote := iputil.NormalizeRemoteAddr(remoteAddr)
	if reported == "" {
		return remote
	}
	if !shouldPreferRemoteNodeIP(reported) {
		return reported
	}
	if isPublicNodeIP(remote) {
		return remote
	}
	return reported
}

func shouldPreferRemoteNodeIP(ip string) bool {
	return !isPublicNodeIP(ip)
}

func isPublicNodeIP(raw string) bool {
	return iputil.IsPublicString(raw)
}

func buildNodeAgentReleaseView(node *model.Node, release *githubReleaseResponse, channel ReleaseChannel) *NodeAgentReleaseInfo {
	currentVersion := strings.TrimSpace(node.Version)
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

func RegisterNodeWithAccessToken(node *model.Node, payload AgentNodePayload) (*AgentRegistrationResponse, error) {
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
	refreshAccessTokenCache(node)
	slog.Info("agent register succeeded on reserved node", "node_id", node.NodeID, "name", node.Name)
	return &AgentRegistrationResponse{
		NodeID:      node.NodeID,
		AccessToken: node.AccessToken,
		Name:        node.Name,
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
		NodeID:      nodeID,
		Name:        nodeName,
		AccessToken: agentToken,
	}
	applyNodeRuntime(node, payload, false)
	if err = node.Insert(); err != nil {
		if model.IsUniqueConstraintError(err) {
			return nil, errors.New("节点标识生成冲突，请重试")
		}
		return nil, err
	}
	refreshAccessTokenCache(node)
	slog.Info("agent discovery register succeeded", "node_id", node.NodeID, "name", node.Name)
	return &AgentRegistrationResponse{
		NodeID:      node.NodeID,
		AccessToken: node.AccessToken,
		Name:        node.Name,
	}, nil
}

func normalizeAgentNodePayload(payload AgentNodePayload) AgentNodePayload {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.IP = strings.TrimSpace(payload.IP)
	payload.Version = strings.TrimSpace(payload.Version)
	payload.ExtVersion = strings.TrimSpace(payload.ExtVersion)
	payload.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	payload.LastError = truncateForDatabase(payload.LastError, 16000)
	payload.OpenrestyStatus = normalizeOpenrestyStatus(payload.OpenrestyStatus)
	payload.OpenrestyMessage = truncateForDatabase(payload.OpenrestyMessage, 16000)
	return payload
}

func validateAgentNodePayload(payload AgentNodePayload) error {
	if payload.IP == "" {
		return errors.New("ip 不能为空")
	}
	if net.ParseIP(payload.IP) == nil {
		return errors.New("ip 格式无效")
	}
	if payload.Version == "" {
		return errors.New("version 不能为空")
	}
	return nil
}

func applyNodeRuntime(node *model.Node, payload AgentNodePayload, preserveName bool) {
	if !preserveName || strings.TrimSpace(node.Name) == "" {
		if strings.TrimSpace(payload.Name) != "" {
			node.Name = strings.TrimSpace(payload.Name)
		}
	}
	if !node.IPManualOverride {
		node.IP = strings.TrimSpace(payload.IP)
	}
	node.Version = strings.TrimSpace(payload.Version)
	node.ExtVersion = strings.TrimSpace(payload.ExtVersion)
	node.OpenrestyStatus = normalizeOpenrestyStatus(payload.OpenrestyStatus)
	node.OpenrestyMessage = truncateForDatabase(payload.OpenrestyMessage, 16000)
	node.Status = NodeStatusOnline
	node.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	node.LastSeenAt = time.Now()
	node.LastError = truncateForDatabase(payload.LastError, 16000)
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

func normalizeNodeType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "tunnel_relay":
		return "tunnel_relay"
	case "tunnel_client":
		return "tunnel_client"
	default:
		return "edge_node"
	}
}

func normalizeRelayPort(port int, defaultPort int) int {
	if port <= 0 || port > 65535 {
		return defaultPort
	}
	return port
}
