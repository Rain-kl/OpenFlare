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
	"strconv"
	"strings"
	"time"

	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	nodeStatusOnline         = "online"
	nodeStatusOffline        = "offline"
	nodeStatusPending        = "pending"
	openrestyStatusHealthy   = "healthy"
	openrestyStatusUnhealthy = "unhealthy"
	openrestyStatusUnknown   = "unknown"
	githubReleasesAPIBase    = "https://api.github.com/repos/%s/releases"
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
	manualOverride := input.GeoManualOverride || geoName != "" || input.GeoLatitude != nil || input.GeoLongitude != nil
	if len(ip) > 64 {
		return "", "", "", nil, nil, false, fmt.Errorf("%s", errNodeIPTooLong)
	}
	if ip != "" && net.ParseIP(ip) == nil {
		return "", "", "", nil, nil, false, fmt.Errorf("%s", errNodeIPInvalid)
	}
	if input.IPManualOverride != nil && *input.IPManualOverride && ip == "" {
		return "", "", "", nil, nil, false, fmt.Errorf("%s", errNodeIPManualRequired)
	}
	if len(geoName) > 128 {
		return "", "", "", nil, nil, false, fmt.Errorf("%s", errNodeGeoNameTooLong)
	}

	geoLatitude := cloneCoordinate(input.GeoLatitude)
	geoLongitude := cloneCoordinate(input.GeoLongitude)
	if (geoLatitude == nil) != (geoLongitude == nil) {
		return "", "", "", nil, nil, false, fmt.Errorf("%s", errNodeGeoCoordinateMismatch)
	}
	if geoLatitude != nil && (*geoLatitude < -90 || *geoLatitude > 90) {
		return "", "", "", nil, nil, false, fmt.Errorf("%s", errNodeGeoLatitudeInvalid)
	}
	if geoLongitude != nil && (*geoLongitude < -180 || *geoLongitude > 180) {
		return "", "", "", nil, nil, false, fmt.Errorf("%s", errNodeGeoLongitudeInvalid)
	}

	if !manualOverride {
		return name, ip, "", nil, nil, false, nil
	}
	if geoLatitude == nil && geoLongitude == nil && geoName == "" {
		return name, ip, "", nil, nil, false, nil
	}

	return name, ip, geoName, geoLatitude, geoLongitude, true, nil
}

func computeNodeStatus(node *model.OpenFlareNode) string {
	if node == nil {
		return nodeStatusOffline
	}
	if node.LastSeenAt == nil || node.LastSeenAt.IsZero() {
		return nodeStatusPending
	}
	if time.Since(*node.LastSeenAt) > model.NodeOfflineThreshold {
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
		nodeType = "edge_node"
	}
	if nodeType == "tunnel_relay" && ofws.IsRelayConnected(node.NodeID) {
		return ofws.RelayWSConnectedLastSeenValue
	}
	if nodeType == "tunnel_client" && ofws.IsFlaredConnected(node.NodeID) {
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
		view.NodeType = "edge_node"
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
	left := parseVersionInfo(local)
	right := parseVersionInfo(remote)
	if left.isDev {
		if right.valid {
			return -1
		}
		return 0
	}
	if !left.valid || !right.valid {
		return 0
	}

	maxLen := len(left.numbers)
	if len(right.numbers) > maxLen {
		maxLen = len(right.numbers)
	}
	for index := 0; index < maxLen; index++ {
		leftValue := 0
		rightValue := 0
		if index < len(left.numbers) {
			leftValue = left.numbers[index]
		}
		if index < len(right.numbers) {
			rightValue = right.numbers[index]
		}
		if leftValue < rightValue {
			return -1
		}
		if leftValue > rightValue {
			return 1
		}
	}
	return 0
}

type versionInfo struct {
	valid   bool
	isDev   bool
	numbers []int
}

func parseVersionInfo(version string) versionInfo {
	normalized := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if normalized == "" || normalized == "dev" {
		return versionInfo{isDev: strings.EqualFold(normalized, "dev")}
	}
	base := normalized
	if separator := strings.IndexRune(normalized, '-'); separator >= 0 {
		base = normalized[:separator]
	}
	segments := strings.Split(base, ".")
	parts := make([]int, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			parts = append(parts, 0)
			continue
		}
		numeric := strings.Builder{}
		for _, r := range segment {
			if r < '0' || r > '9' {
				break
			}
			numeric.WriteRune(r)
		}
		if numeric.Len() == 0 {
			parts = append(parts, 0)
			continue
		}
		value, err := strconv.Atoi(numeric.String())
		if err != nil {
			return versionInfo{}
		}
		parts = append(parts, value)
	}
	return versionInfo{valid: len(parts) > 0, numbers: parts}
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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

func setReleaseHTTPClientForTest(client *http.Client) *http.Client {
	previous := releaseHTTPClient
	if client == nil {
		releaseHTTPClient = &http.Client{Timeout: 30 * time.Second}
	} else {
		releaseHTTPClient = client
	}
	return previous
}
