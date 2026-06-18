// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/observability"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/option"
	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// Input is the create/update node payload.
type Input struct {
	Name                  string   `json:"name"`
	IP                    string   `json:"ip"`
	IPManualOverride      *bool    `json:"ip_manual_override"`
	AutoUpdateEnabled     bool     `json:"auto_update_enabled"`
	GeoName               string   `json:"geo_name"`
	GeoLatitude           *float64 `json:"geo_latitude"`
	GeoLongitude          *float64 `json:"geo_longitude"`
	GeoManualOverride     bool     `json:"geo_manual_override"`
	NodeType              string   `json:"node_type"`
	RelayBindPort         int      `json:"relay_bind_port"`
	RelayVhostHTTPPort    int      `json:"relay_vhost_http_port"`
	RelayAgentAccessAddr  string   `json:"relay_agent_access_addr"`
	RelayClientAccessAddr string   `json:"relay_client_access_addr"`
	RelayClientProxyURL   string   `json:"relay_client_proxy_url"`
	RelayWebServerEnabled bool     `json:"relay_web_server_enabled"`
}

// AgentUpdateInput requests an agent self-update on a node.
type AgentUpdateInput struct {
	Channel string `json:"channel"`
	TagName string `json:"tag_name"`
}

// AgentReleaseInfo describes the latest agent release for a node.
type AgentReleaseInfo struct {
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

// BootstrapView exposes the global discovery token.
type BootstrapView struct {
	DiscoveryToken string `json:"discovery_token"`
}

// View is the admin-facing node representation.
type View struct {
	ID                        uint       `json:"id"`
	NodeID                    string     `json:"node_id"`
	Name                      string     `json:"name"`
	IP                        string     `json:"ip"`
	IPManualOverride          bool       `json:"ip_manual_override"`
	GeoName                   string     `json:"geo_name"`
	GeoLatitude               *float64   `json:"geo_latitude"`
	GeoLongitude              *float64   `json:"geo_longitude"`
	GeoManualOverride         bool       `json:"geo_manual_override"`
	AccessToken               string     `json:"access_token"`
	AutoUpdateEnabled         bool       `json:"auto_update_enabled"`
	UpdateRequested           bool       `json:"update_requested"`
	UpdateChannel             string     `json:"update_channel"`
	UpdateTag                 string     `json:"update_tag"`
	RestartOpenrestyRequested bool       `json:"restart_openresty_requested"`
	Version                   string     `json:"version"`
	ExtVersion                string     `json:"ext_version"`
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
	NodeType                  string     `json:"node_type"`
	RelayBindPort             int        `json:"relay_bind_port"`
	RelayVhostHTTPPort        int        `json:"relay_vhost_http_port"`
	RelayAgentAccessAddr      string     `json:"relay_agent_access_addr"`
	RelayClientAccessAddr     string     `json:"relay_client_access_addr"`
	RelayClientProxyURL       string     `json:"relay_client_proxy_url"`
	RelayStatus               string     `json:"relay_status"`
	RelayWebServerEnabled     bool       `json:"relay_web_server_enabled"`
}

// ObservabilityQuery filters node observability data.
type ObservabilityQuery struct {
	Hours int `json:"hours"`
	Limit int `json:"limit"`
}

// ObservabilityView is the node observability API response.
type ObservabilityView = observability.NodeView

// HealthEventCleanupResult reports health event cleanup outcome.
type HealthEventCleanupResult = observability.HealthEventCleanupResult

// ListNodes returns all node views with latest apply log metadata.
func ListNodes(ctx context.Context) ([]*View, error) {
	nodes, err := model.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, err
	}
	nodeIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.NodeID)
	}
	latestLogs, err := model.GetLatestOpenFlareApplyLogsByNodeIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}
	views := make([]*View, 0, len(nodes))
	for _, node := range nodes {
		view := buildNodeView(&node)
		view.Status = computeNodeStatus(&node)
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

// CreateNode creates a reserved node with generated node_id and access_token.
func CreateNode(ctx context.Context, input Input) (*View, error) {
	name, ip, geoName, geoLatitude, geoLongitude, geoManualOverride, err := normalizeNodeInput(input)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New(errNodeNameRequired)
	}
	ipManualOverride := resolveNodeIPManualOverride(input, nil, ip)
	node := &model.OpenFlareNode{
		Name:              name,
		IP:                ip,
		IPManualOverride:  ipManualOverride,
		GeoName:           geoName,
		GeoLatitude:       geoLatitude,
		GeoLongitude:      geoLongitude,
		GeoManualOverride: geoManualOverride,
		Version:           "",
		ExtVersion:        "",
		Status:            nodeStatusPending,
		AutoUpdateEnabled: input.AutoUpdateEnabled,
		NodeType:          normalizeNodeType(input.NodeType),
		CapabilitiesJSON:  "[]",
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
		node.RelayWebServerEnabled = input.RelayWebServerEnabled
	}
	if err = model.CreateOpenFlareNode(ctx, node); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errNodeIDConflict)
		}
		return nil, err
	}
	return buildNodeView(node), nil
}

// UpdateNode updates an existing node.
func UpdateNode(ctx context.Context, id uint, input Input) (*View, error) {
	name, ip, geoName, geoLatitude, geoLongitude, geoManualOverride, err := normalizeNodeInput(input)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New(errNodeNameRequired)
	}
	node, err := model.GetOpenFlareNodeByID(ctx, id)
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
		node.RelayWebServerEnabled = input.RelayWebServerEnabled
		if input.RelayBindPort > 0 {
			node.RelayBindPort = input.RelayBindPort
		}
		if input.RelayVhostHTTPPort > 0 {
			node.RelayVhostHTTPPort = input.RelayVhostHTTPPort
		}
	}
	if err = model.SaveOpenFlareNode(ctx, node); err != nil {
		return nil, err
	}
	return buildNodeView(node), nil
}

// DeleteNode removes a node by id.
func DeleteNode(ctx context.Context, id uint) error {
	if _, err := model.GetOpenFlareNodeByID(ctx, id); err != nil {
		return err
	}
	return model.DeleteOpenFlareNode(ctx, id)
}

// GetBootstrapToken returns the global discovery token, creating one if missing.
func GetBootstrapToken(ctx context.Context) (*BootstrapView, error) {
	token, err := ensureGlobalDiscoveryToken(ctx)
	if err != nil {
		return nil, err
	}
	return &BootstrapView{DiscoveryToken: token}, nil
}

// RotateBootstrapToken rotates the global discovery token.
func RotateBootstrapToken(ctx context.Context) (*BootstrapView, error) {
	token, err := newRandomToken()
	if err != nil {
		return nil, err
	}
	if err = model.UpdateOpenFlareOption(ctx, "AgentDiscoveryToken", token); err != nil {
		return nil, err
	}
	return &BootstrapView{DiscoveryToken: token}, nil
}

// GetAgentRelease checks the latest agent release for a node.
func GetAgentRelease(ctx context.Context, id uint, channel string) (*AgentReleaseInfo, error) {
	node, err := model.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	release, err := fetchLatestGitHubRelease(ctx, model.AgentUpdateRepo, normalizeReleaseChannel(channel))
	if err != nil {
		return nil, err
	}
	return buildNodeAgentReleaseView(node, release, normalizeReleaseChannel(channel)), nil
}

// RequestAgentUpdate marks a node for manual agent update.
func RequestAgentUpdate(ctx context.Context, id uint, input AgentUpdateInput) (*View, error) {
	node, err := model.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	channel := normalizeReleaseChannel(input.Channel)
	tagName := strings.TrimSpace(input.TagName)
	if tagName != "" {
		release, releaseErr := fetchGitHubReleaseByTag(ctx, model.AgentUpdateRepo, tagName)
		if releaseErr != nil {
			return nil, releaseErr
		}
		if channel == releaseChannelPreview && !release.Prerelease {
			return nil, errors.New(errAgentPreviewTagInvalid)
		}
		if channel == releaseChannelStable && release.Prerelease {
			return nil, errors.New(errAgentStableTagInvalid)
		}
	}
	node.UpdateRequested = true
	node.UpdateChannel = channel.String()
	node.UpdateTag = tagName
	if err = model.UpdateOpenFlareNodeFields(ctx, node, "update_requested", "update_channel", "update_tag"); err != nil {
		return nil, err
	}
	return buildNodeView(node), nil
}

// RequestOpenrestyRestart marks a node for openresty restart.
func RequestOpenrestyRestart(ctx context.Context, id uint) (*View, error) {
	node, err := model.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	node.RestartOpenrestyRequested = true
	if err = model.UpdateOpenFlareNodeFields(ctx, node, "restart_openresty_requested"); err != nil {
		return nil, err
	}
	return buildNodeView(node), nil
}

// RequestForceSync pushes force_sync_config to a connected agent websocket.
func RequestForceSync(ctx context.Context, id uint) (*View, error) {
	node, err := model.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	activeConfig, err := model.GetActiveConfigVersion(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("无法获取当前激活的配置版本：%s", errNoActiveConfigVersion)
		}
		return nil, fmt.Errorf("无法获取当前激活的配置版本：%v", err)
	}
	if !ofws.SendForceSyncConfig(node.NodeID, forceSyncConfigPayload{
		Version:  activeConfig.Version,
		Checksum: activeConfig.Checksum,
	}) {
		return nil, errors.New(errNodeForceSyncFailed)
	}
	return buildNodeView(node), nil
}

// GetObservability returns observability details for a node.
func GetObservability(ctx context.Context, id uint, query ObservabilityQuery) (*ObservabilityView, error) {
	return observability.GetNodeObservability(ctx, id, observability.NodeQuery{
		Hours: query.Hours,
		Limit: query.Limit,
	})
}

// CleanupHealthEvents removes all health events for a node.
func CleanupHealthEvents(ctx context.Context, id uint) (*HealthEventCleanupResult, error) {
	return observability.CleanupHealthEvents(ctx, id)
}

func ensureGlobalDiscoveryToken(ctx context.Context) (string, error) {
	if err := option.EnsureInitialized(ctx); err != nil {
		return "", err
	}
	model.OptionMapRWMutex.RLock()
	token := strings.TrimSpace(model.AgentDiscoveryToken)
	model.OptionMapRWMutex.RUnlock()
	if token != "" {
		return token, nil
	}
	token, err := newRandomToken()
	if err != nil {
		return "", err
	}
	if err = model.UpdateOpenFlareOption(ctx, "AgentDiscoveryToken", token); err != nil {
		return "", err
	}
	return token, nil
}

type forceSyncConfigPayload struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

// ValidateDiscoveryToken validates the global discovery token.
func ValidateDiscoveryToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("缺少 Discovery Token")
	}
	discoveryToken, err := ensureGlobalDiscoveryToken(ctx)
	if err != nil {
		return err
	}
	if token != discoveryToken {
		return fmt.Errorf("Discovery Token 无效")
	}
	return nil
}
