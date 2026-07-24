// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/util"
)

const (
	nodeStatusOnline         = "online"
	nodeStatusOffline        = "offline"
	nodeStatusPending        = "pending"
	openrestyStatusHealthy   = "healthy"
	openrestyStatusUnhealthy = "unhealthy"
	openrestyStatusUnknown   = "unknown"
	githubReleasesAPIBase    = "https://api.github.com/repos/%s/releases"
	nodeTypeTunnelRelay      = "tunnel_relay"
	nodeTypeTunnelClient     = "tunnel_client"
	nodeTypeEdgeNode         = "edge_node"

	nodeTokenByteLength  = 16
	maxNodeIPLength      = 64
	maxNodeGeoNameLength = 128
)

type releaseChannel string

const (
	releaseChannelStable  releaseChannel = "stable"
	releaseChannelPreview releaseChannel = "preview"
)

var releaseHTTPClient = &http.Client{Timeout: 30 * time.Second}

type githubReleaseResponse struct {
	TagName     string `json:"tag_name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
}

func newRandomToken() (string, error) {
	buf := make([]byte, nodeTokenByteLength)
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
	case nodeTypeTunnelRelay:
		return nodeTypeTunnelRelay
	case nodeTypeTunnelClient:
		return nodeTypeTunnelClient
	default:
		return nodeTypeEdgeNode
	}
}

func normalizeRelayPort(port int, defaultPort int) int {
	if port <= 0 || port > 65535 {
		return defaultPort
	}
	return port
}

func normalizeReleaseChannel(channel string) releaseChannel {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case string(releaseChannelPreview):
		return releaseChannelPreview
	default:
		return releaseChannelStable
	}
}

func (channel releaseChannel) String() string {
	if channel == releaseChannelPreview {
		return string(releaseChannelPreview)
	}
	return string(releaseChannelStable)
}

func normalizeOpenrestyStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case openrestyStatusHealthy:
		return openrestyStatusHealthy
	case openrestyStatusUnhealthy:
		return openrestyStatusUnhealthy
	default:
		return openrestyStatusUnknown
	}
}

func cloneCoordinate(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func resolveNodeIPManualOverride(input Input, existing *model.OpenFlareNode, normalizedIP string) bool {
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

func normalizeNodeInput(input Input) (string, string, string, *float64, *float64, bool, error) {
	name := strings.TrimSpace(input.Name)
	ip := strings.TrimSpace(input.IP)
	geoName := strings.TrimSpace(input.GeoName)
	if err := validateNodeIPInput(input, ip); err != nil {
		return "", "", "", nil, nil, false, err
	}
	if len(geoName) > maxNodeGeoNameLength {
		return "", "", "", nil, nil, false, fmt.Errorf("%s", errNodeGeoNameTooLong)
	}

	geoLatitude := cloneCoordinate(input.GeoLatitude)
	geoLongitude := cloneCoordinate(input.GeoLongitude)
	if err := validateNodeGeoCoordinates(geoLatitude, geoLongitude); err != nil {
		return "", "", "", nil, nil, false, err
	}

	manualOverride := input.GeoManualOverride || geoName != "" || geoLatitude != nil || geoLongitude != nil
	if !manualOverride || (geoLatitude == nil && geoLongitude == nil && geoName == "") {
		return name, ip, "", nil, nil, false, nil
	}
	return name, ip, geoName, geoLatitude, geoLongitude, true, nil
}

func validateNodeIPInput(input Input, ip string) error {
	if len(ip) > maxNodeIPLength {
		return fmt.Errorf("%s", errNodeIPTooLong)
	}
	if ip != "" && net.ParseIP(ip) == nil {
		return fmt.Errorf("%s", errNodeIPInvalid)
	}
	if input.IPManualOverride != nil && *input.IPManualOverride && ip == "" {
		return fmt.Errorf("%s", errNodeIPManualRequired)
	}
	return nil
}

func validateNodeGeoCoordinates(geoLatitude, geoLongitude *float64) error {
	if (geoLatitude == nil) != (geoLongitude == nil) {
		return fmt.Errorf("%s", errNodeGeoCoordinateMismatch)
	}
	if geoLatitude != nil && (*geoLatitude < -90 || *geoLatitude > 90) {
		return fmt.Errorf("%s", errNodeGeoLatitudeInvalid)
	}
	if geoLongitude != nil && (*geoLongitude < -180 || *geoLongitude > 180) {
		return fmt.Errorf("%s", errNodeGeoLongitudeInvalid)
	}
	return nil
}

func computeNodeStatus(node *model.OpenFlareNode) string {
	if node == nil {
		return nodeStatusOffline
	}
	if node.LastSeenAt == nil || node.LastSeenAt.IsZero() {
		return nodeStatusPending
	}
	// 默认离线阈值 60 秒（与 node_offline_threshold 默认一致），避免在这里读取配置
	// 实际阈值会在需要精确判断的地方通过 getNodeOfflineThreshold 读取
	threshold := 60 * time.Second
	if time.Since(*node.LastSeenAt) > threshold {
		return nodeStatusOffline
	}
	return nodeStatusOnline
}

func nodeViewLastSeenAt(node *model.OpenFlareNode) any {
	if node == nil {
		return time.Time{}
	}
	nodeType := strings.TrimSpace(node.NodeType)
	if nodeType == "" {
		nodeType = nodeTypeEdgeNode
	}
	if nodeType == nodeTypeTunnelRelay && ofws.IsRelayConnected(node.NodeID) {
		return ofws.RelayWSConnectedLastSeenValue
	}
	if nodeType == nodeTypeTunnelClient && ofws.IsFlaredConnected(node.NodeID) {
		return ofws.FlaredWSConnectedLastSeenValue
	}
	if ofws.IsAgentConnected(node.NodeID) {
		return ofws.AgentWSConnectedLastSeenValue
	}
	if node.LastSeenAt == nil {
		return time.Time{}
	}
	return *node.LastSeenAt
}

func buildNodeView(node *model.OpenFlareNode) *View {
	if node == nil {
		return nil
	}
	status := computeNodeStatus(node)
	view := &View{
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
		NodeType:                  node.NodeType,
		RelayBindPort:             node.RelayBindPort,
		RelayVhostHTTPPort:        node.RelayVhostHTTPPort,
		RelayAgentAccessAddr:      node.RelayAgentAccessAddr,
		RelayClientAccessAddr:     node.RelayClientAccessAddr,
		RelayClientProxyURL:       node.RelayClientProxyURL,
		RelayStatus:               node.RelayStatus,
		RelayWebServerEnabled:     node.RelayWebServerEnabled,
	}
	if view.UpdateChannel == "" {
		view.UpdateChannel = releaseChannelStable.String()
	}
	if view.NodeType == "" {
		view.NodeType = nodeTypeEdgeNode
	}
	return view
}

func buildNodeAgentReleaseView(node *model.OpenFlareNode, release *githubReleaseResponse, channel releaseChannel) *AgentReleaseInfo {
	currentVersion := strings.TrimSpace(node.Version)
	view := &AgentReleaseInfo{
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

func isVersionNewer(current string, latest string) bool {
	return compareVersions(current, latest) < 0
}

func compareVersions(local, remote string) int {
	return util.CompareVersions(local, remote)
}

func fetchLatestGitHubRelease(ctx context.Context, repo string, channel releaseChannel) (*githubReleaseResponse, error) {
	switch normalizeReleaseChannel(string(channel)) {
	case releaseChannelPreview:
		return fetchLatestPreviewGitHubRelease(ctx, repo)
	default:
		return fetchLatestStableGitHubRelease(ctx, repo)
	}
}

func fetchLatestStableGitHubRelease(ctx context.Context, repo string) (*githubReleaseResponse, error) {
	url := fmt.Sprintf(githubReleasesAPIBase+"/latest", strings.TrimSpace(repo))
	req, err := newGitHubReleaseRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("创建更新请求失败")
	}
	resp, err := releaseHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取最新版本失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回异常状态: %s", resp.Status)
	}
	return decodeGitHubRelease(resp.Body)
}

func fetchLatestPreviewGitHubRelease(ctx context.Context, repo string) (*githubReleaseResponse, error) {
	url := fmt.Sprintf(githubReleasesAPIBase+"?per_page=20", strings.TrimSpace(repo))
	req, err := newGitHubReleaseRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("创建更新请求失败")
	}
	resp, err := releaseHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取 preview 版本失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回异常状态: %s", resp.Status)
	}
	var releases []githubReleaseResponse
	if err = json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("解析 preview 版本信息失败")
	}
	for _, release := range releases {
		if release.Draft || !release.Prerelease {
			continue
		}
		releaseCopy := release
		return &releaseCopy, nil
	}
	return nil, fmt.Errorf("当前没有可用的 preview 发布")
}

func fetchGitHubReleaseByTag(ctx context.Context, repo string, tag string) (*githubReleaseResponse, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, fmt.Errorf("缺少发布版本号")
	}
	url := fmt.Sprintf(githubReleasesAPIBase+"/tags/%s", strings.TrimSpace(repo), tag)
	req, err := newGitHubReleaseRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("创建更新请求失败")
	}
	resp, err := releaseHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取指定版本失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("未找到指定版本: %s", tag)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回异常状态: %s", resp.Status)
	}
	return decodeGitHubRelease(resp.Body)
}

func newGitHubReleaseRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OpenFlare-Server")
	return req, nil
}

func decodeGitHubRelease(reader io.Reader) (*githubReleaseResponse, error) {
	var release githubReleaseResponse
	if err := json.NewDecoder(reader).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析版本信息失败")
	}
	return &release, nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}
