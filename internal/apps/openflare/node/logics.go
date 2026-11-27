// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	cf "github.com/Rain-kl/Wavelet/internal/apps/openflare/cloudflare"
	ofgeoip "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/observability"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/ping"
	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"gorm.io/gorm"
)

// AgentUpdateRepo returns the configured node release repository.
func AgentUpdateRepo(ctx context.Context) string {
	config, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyAgentUpdateRepo)
	if err != nil || strings.TrimSpace(config.Value) == "" {
		return "Rain-kl/OpenFlare" // 默认值
	}
	return strings.TrimSpace(config.Value)
}

// Input is the create/update node payload.
type Input struct {
	Name                  string            `json:"name"`
	IP                    string            `json:"ip"`
	IPManualOverride      *bool             `json:"ip_manual_override"`
	AutoUpdateEnabled     bool              `json:"auto_update_enabled"`
	GeoName               string            `json:"geo_name"`
	GeoLatitude           *float64          `json:"geo_latitude"`
	GeoLongitude          *float64          `json:"geo_longitude"`
	GeoManualOverride     bool              `json:"geo_manual_override"`
	NetworkGroup          string            `json:"network_group"`
	NodeType              string            `json:"node_type"`
	CapacityBandwidthMbps int               `json:"capacity_bandwidth_mbps"`
	CapacityConnections   int               `json:"capacity_connections"`
	CapacityWeight        int               `json:"capacity_weight"`
	Tags                  []TagBindingInput `json:"tags"`
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
	ID                        uint                   `json:"id"`
	NodeID                    string                 `json:"node_id"`
	Name                      string                 `json:"name"`
	IP                        string                 `json:"ip"`
	IPManualOverride          bool                   `json:"ip_manual_override"`
	ASN                       uint32                 `json:"asn"`
	ASNOrg                    string                 `json:"asn_org"`
	GeoName                   string                 `json:"geo_name"`
	GeoLatitude               *float64               `json:"geo_latitude"`
	GeoLongitude              *float64               `json:"geo_longitude"`
	GeoManualOverride         bool                   `json:"geo_manual_override"`
	GeoSource                 string                 `json:"geo_source"`
	NetworkGroup              string                 `json:"network_group"`
	CapacityBandwidthMbps     int                    `json:"capacity_bandwidth_mbps"`
	CapacityConnections       int                    `json:"capacity_connections"`
	CapacityWeight            int                    `json:"capacity_weight"`
	Tags                      []model.NodeTagSummary `json:"tags"`
	AccessToken               string                 `json:"access_token"`
	AutoUpdateEnabled         bool                   `json:"auto_update_enabled"`
	EdgeUpdateSupported       bool                   `json:"edge_update_supported"`
	UpdateRequested           bool                   `json:"update_requested"`
	UpdateChannel             string                 `json:"update_channel"`
	UpdateTag                 string                 `json:"update_tag"`
	RestartOpenrestyRequested bool                   `json:"restart_openresty_requested"`
	Version                   string                 `json:"version"`
	ExtVersion                string                 `json:"ext_version"`
	OpenrestyStatus           string                 `json:"openresty_status"`
	OpenrestyMessage          string                 `json:"openresty_message"`
	Status                    string                 `json:"status"`
	CurrentVersion            string                 `json:"current_version"`
	LastSeenAt                any                    `json:"last_seen_at"`
	LastHeartbeatAt           *time.Time             `json:"-"`
	LastError                 string                 `json:"last_error"`
	LatestApplyResult         string                 `json:"latest_apply_result"`
	LatestApplyMessage        string                 `json:"latest_apply_message"`
	LatestApplyChecksum       string                 `json:"latest_apply_checksum"`
	LatestMainConfigChecksum  string                 `json:"latest_main_config_checksum"`
	LatestRouteConfigChecksum string                 `json:"latest_route_config_checksum"`
	LatestSupportFileCount    int                    `json:"latest_support_file_count"`
	LatestApplyAt             *time.Time             `json:"latest_apply_at"`
	CreatedAt                 time.Time              `json:"created_at"`
	UpdatedAt                 time.Time              `json:"updated_at"`
	NodeType                  string                 `json:"node_type"`
	QuarantinedAt             *time.Time             `json:"quarantined_at"`
	QuarantineReason          string                 `json:"quarantine_reason"`
	IsQuarantined             bool                   `json:"is_quarantined"`
	Monitoring                *MonitoringView        `json:"monitoring,omitempty"`
}

// ObservabilityQuery filters node observability data.
type ObservabilityQuery struct {
	Hours    int  `form:"hours" json:"hours"`
	Limit    int  `form:"limit" json:"limit"`
	Realtime bool `form:"realtime" json:"realtime"`
}

// ObservabilityView is the node observability API response.
type ObservabilityView = observability.NodeView

// HealthEventCleanupResult reports health event cleanup outcome.
type HealthEventCleanupResult = observability.HealthEventCleanupResult

// ListNodes returns all node views with latest apply log metadata.
func ListNodes(ctx context.Context) ([]*View, error) {
	nodes, err := repository.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, err
	}
	return buildNodeViews(ctx, nodes, time.Now())
}

func buildNodeViews(ctx context.Context, nodes []model.OpenFlareNode, now time.Time) ([]*View, error) {
	nodeIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.NodeID)
	}
	latestLogs, err := repository.GetLatestOpenFlareApplyLogsByNodeIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}
	views := make([]*View, 0, len(nodes))
	for _, node := range nodes {
		view := buildNodeView(&node)
		view.Status = computeNodeStatusAt(&node, now)
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
	attachNodeTagsToViews(ctx, views)
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
	tagSpecs, err := validateNodeCapacityAndTags(ctx, input)
	if err != nil {
		return nil, err
	}
	ipManualOverride := resolveNodeIPManualOverride(input, nil, ip)
	geoSource := ""
	if geoManualOverride {
		geoSource = pkgnetintel.GeoSourceManual
	}
	capacityWeight := input.CapacityWeight
	if capacityWeight == 0 {
		capacityWeight = defaultNodeCapacityWt
	}
	node := &model.OpenFlareNode{
		Name:                  name,
		IP:                    ip,
		IPManualOverride:      ipManualOverride,
		GeoName:               geoName,
		GeoLatitude:           geoLatitude,
		GeoLongitude:          geoLongitude,
		GeoManualOverride:     geoManualOverride,
		GeoSource:             geoSource,
		NetworkGroup:          normalizeNodeNetworkGroup(input.NetworkGroup),
		CapacityBandwidthMbps: input.CapacityBandwidthMbps,
		CapacityConnections:   input.CapacityConnections,
		CapacityWeight:        capacityWeight,
		Version:               "",
		ExtVersion:            "",
		Status:                nodeStatusPending,
		AutoUpdateEnabled:     input.AutoUpdateEnabled,
		NodeType:              normalizeNodeType(input.NodeType),
		CapabilitiesJSON:      "[]",
	}
	node.NodeID, err = newServerNodeID()
	if err != nil {
		return nil, err
	}
	node.AccessToken, err = newRandomToken()
	if err != nil {
		return nil, err
	}
	if node.NodeType == "edge_node" {
		node.Engine = enginePingora
	}
	if !geoManualOverride && ip != "" {
		ofgeoip.ApplyNodeGeoFromIP(ctx, node, ip)
	}
	netintel.EnrichNodeASN(node)
	if err = repository.CreateOpenFlareNode(ctx, node); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errNodeIDConflict)
		}
		return nil, err
	}
	if input.Tags != nil {
		if err = repository.ReplaceOpenFlareNodeTagBindings(ctx, node.NodeID, tagSpecs); err != nil {
			return nil, err
		}
	}
	ping.AttachNewNode(ctx, node.NodeID)
	view := buildNodeView(node)
	attachNodeTagsToViews(ctx, []*View{view})
	return view, nil
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
	tagSpecs, err := validateNodeCapacityAndTags(ctx, input)
	if err != nil {
		return nil, err
	}
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ipManualOverride := resolveNodeIPManualOverride(input, node, ip)
	previousIP := node.IP
	node.Name = name
	node.IP = ip
	node.IPManualOverride = ipManualOverride
	if strings.TrimSpace(previousIP) != strings.TrimSpace(ip) {
		node.ASN = 0
		node.ASNOrg = ""
	}
	netintel.EnrichNodeASN(node)
	node.GeoName = geoName
	node.GeoLatitude = geoLatitude
	node.GeoLongitude = geoLongitude
	node.GeoManualOverride = geoManualOverride
	if geoManualOverride {
		node.GeoSource = pkgnetintel.GeoSourceManual
	} else {
		ofgeoip.ApplyNodeGeoFromIP(ctx, node, node.IP)
	}
	node.NetworkGroup = normalizeNodeNetworkGroup(input.NetworkGroup)
	node.CapacityBandwidthMbps = input.CapacityBandwidthMbps
	node.CapacityConnections = input.CapacityConnections
	if input.CapacityWeight > 0 {
		node.CapacityWeight = input.CapacityWeight
	} else {
		node.CapacityWeight = normalizeNodeCapacityWeight(node.CapacityWeight)
	}
	node.AutoUpdateEnabled = input.AutoUpdateEnabled
	fields := []string{
		"name", "ip", "ip_manual_override", "asn", "asn_org",
		"geo_name", "geo_latitude", "geo_longitude", "geo_manual_override",
		"geo_source", "network_group", "capacity_bandwidth_mbps", "capacity_connections", "capacity_weight",
		"auto_update_enabled",
	}
	if err = repository.UpdateOpenFlareNodeFields(ctx, node, fields...); err != nil {
		return nil, err
	}
	if input.Tags != nil {
		if err = repository.ReplaceOpenFlareNodeTagBindings(ctx, node.NodeID, tagSpecs); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(previousIP) != strings.TrimSpace(node.IP) {
		if _, dispatchErr := cf.DispatchNodeSync(ctx, node.ID, "cloudflare_node_ip_update"); dispatchErr != nil {
			logger.ErrorF(ctx, "[Cloudflare] enqueue node sync failed: node_id=%d error=%v", node.ID, dispatchErr)
		}
	}
	view := buildNodeView(node)
	attachNodeTagsToViews(ctx, []*View{view})
	return view, nil
}

func normalizeNodeNetworkGroup(raw string) string {
	trimmed := strings.TrimSpace(raw)
	const maxNetworkGroupLen = 64
	if len(trimmed) > maxNetworkGroupLen {
		return trimmed[:maxNetworkGroupLen]
	}
	return trimmed
}

// DeleteNode removes a node by id.
func DeleteNode(ctx context.Context, id uint) error {
	target, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return err
	}
	if err := repository.DeleteOpenFlareNode(ctx, id); err != nil {
		return err
	}
	if err := repository.ReplaceOpenFlareNodeTagBindings(ctx, target.NodeID, nil); err != nil {
		logger.WarnF(ctx, "failed to cleanup tag bindings for deleted node %s: %v", target.NodeID, err)
	}
	ping.DetachDeletedNode(ctx, target.NodeID)
	return nil
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
	if err = repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyAgentDiscoveryToken, token); err != nil {
		return nil, err
	}
	return &BootstrapView{DiscoveryToken: token}, nil
}

// GetAgentRelease checks the latest agent release for a node.
func GetAgentRelease(ctx context.Context, id uint, channel string) (*AgentReleaseInfo, error) {
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	normalized := normalizeReleaseChannel(channel)
	var release *githubReleaseResponse
	if isPingoraNode(node) {
		release, err = fetchEdgeRelease(ctx, AgentUpdateRepo(ctx), normalized, "")
	} else {
		release, err = fetchLatestGitHubRelease(ctx, AgentUpdateRepo(ctx), normalized)
	}
	if err != nil {
		return nil, err
	}
	return buildNodeAgentReleaseView(node, release, normalized), nil
}

// RequestAgentUpdate marks a node for manual agent update.
func RequestAgentUpdate(ctx context.Context, id uint, input AgentUpdateInput) (*View, error) {
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if node.IsQuarantined() {
		return nil, errors.New(errNodeQuarantined)
	}
	channel := normalizeReleaseChannel(input.Channel)
	tagName := strings.TrimSpace(input.TagName)
	if isPingoraNode(node) {
		tagName, err = resolveEdgeUpdateTag(ctx, node, channel, tagName)
	} else if tagName != "" {
		err = validateAgentReleaseTag(ctx, channel, tagName)
	}
	if err != nil {
		return nil, err
	}
	node.UpdateRequested = true
	node.UpdateChannel = channel.String()
	node.UpdateTag = tagName
	if err = repository.UpdateOpenFlareNodeFields(ctx, node, "update_requested", "update_channel", "update_tag"); err != nil {
		return nil, err
	}
	return buildNodeView(node), nil
}

func resolveEdgeUpdateTag(ctx context.Context, node *model.OpenFlareNode, channel releaseChannel, tag string) (string, error) {
	if !node.HasCapability(model.EdgeUpdateCapability) {
		return "", errors.New(errEdgeUpdateUnsupported)
	}
	release, err := fetchEdgeRelease(ctx, AgentUpdateRepo(ctx), channel, tag)
	if err != nil {
		return "", err
	}
	if release == nil || !isNewerEdgeRelease(node.Version, release.TagName) {
		return "", errors.New(errEdgeUpdateUnavailable)
	}
	return release.TagName, nil
}

func validateAgentReleaseTag(ctx context.Context, channel releaseChannel, tag string) error {
	if strings.HasPrefix(tag, "edge-v") {
		return errors.New(errEdgeUpdateUnsupported)
	}
	release, err := fetchGitHubReleaseByTag(ctx, AgentUpdateRepo(ctx), tag)
	if err != nil {
		return err
	}
	if channel == releaseChannelPreview && !release.Prerelease {
		return errors.New(errAgentPreviewTagInvalid)
	}
	if channel == releaseChannelStable && release.Prerelease {
		return errors.New(errAgentStableTagInvalid)
	}
	return nil
}

// RequestEdgeRestart marks a node for Pingora restart.
func RequestEdgeRestart(ctx context.Context, id uint) (*View, error) {
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if node.IsQuarantined() {
		return nil, errors.New(errNodeQuarantined)
	}
	node.RestartOpenrestyRequested = true
	if err = repository.UpdateOpenFlareNodeFields(ctx, node, "restart_openresty_requested"); err != nil {
		return nil, err
	}
	return buildNodeView(node), nil
}

// RequestForceSync sends a forced configuration refresh to a connected Edge.
func RequestForceSync(ctx context.Context, id uint) (*View, error) {
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if node.IsQuarantined() {
		return nil, errors.New(errNodeQuarantined)
	}
	engine := node.Engine
	if engine != enginePingora {
		return nil, errors.New(errNodeForceSyncFailed)
	}
	_, err = repository.GetActivePingoraConfigMetadata(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("unable to get the currently active configuration version: %s", errNoActiveConfigVersion)
		}
		return nil, fmt.Errorf("unable to get the currently active configuration version: %w", err)
	}
	if !ofws.SendForceSyncConfig(node.NodeID) {
		return nil, errors.New(errNodeForceSyncFailed)
	}
	return buildNodeView(node), nil
}

// GetObservability returns observability details for a node.
func GetObservability(ctx context.Context, id uint, query ObservabilityQuery) (*ObservabilityView, error) {
	return observability.GetNodeObservability(ctx, id, observability.NodeQuery{
		Hours:    query.Hours,
		Limit:    query.Limit,
		Realtime: query.Realtime,
	})
}

// CleanupHealthEvents removes all health events for a node.
func CleanupHealthEvents(ctx context.Context, id uint) (*HealthEventCleanupResult, error) {
	return observability.CleanupHealthEvents(ctx, id)
}

func ensureGlobalDiscoveryToken(ctx context.Context) (string, error) {
	// 从 SystemConfig 读取 Agent 发现令牌
	config, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyAgentDiscoveryToken)
	if err == nil && strings.TrimSpace(config.Value) != "" {
		return strings.TrimSpace(config.Value), nil
	}

	// 如果不存在，生成新令牌并保存
	token, err := newRandomToken()
	if err != nil {
		return "", err
	}

	// 更新到 SystemConfig
	if err = repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyAgentDiscoveryToken, token); err != nil {
		return "", err
	}
	return token, nil
}

// ValidateDiscoveryToken validates the global discovery token.
func ValidateDiscoveryToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("missing discovery token")
	}
	discoveryToken, err := ensureGlobalDiscoveryToken(ctx)
	if err != nil {
		return err
	}
	if !tokenEqual(token, discoveryToken) {
		return errors.New("invalid discovery token") // error 消息首字母小写
	}
	return nil
}
