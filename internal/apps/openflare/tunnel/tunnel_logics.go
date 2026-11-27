// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

// TunnelInput creates or updates a logical private-network tunnel.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelInput struct {
	Name               string         `json:"name"`
	Description        string         `json:"description"`
	PoolID             uint           `json:"pool_id"`
	PoolKey            string         `json:"pool_key"`
	HARelayCount       int            `json:"ha_relay_count"`
	PreferredTransport string         `json:"preferred_transport"`
	FallbackTransports []string       `json:"fallback_transports"`
	ClientProxyURL     string         `json:"client_proxy_url"`
	AllowedTargetCIDRs []string       `json:"allowed_target_cidrs"`
	BlockedTargetCIDRs []string       `json:"blocked_target_cidrs"`
	MaxConnectors      int            `json:"max_connectors"`
	Enabled            *bool          `json:"enabled"`
	AutoUpdateEnabled  bool           `json:"auto_update_enabled"`
	UpdateChannel      string         `json:"update_channel"`
	UpdateTag          string         `json:"update_tag"`
	Metadata           map[string]any `json:"metadata"`
}

// TunnelServiceInput creates or updates an L4 TCP/UDP exposed service on a tunnel.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelServiceInput struct {
	Name        string         `json:"name"`
	ServiceType string         `json:"service_type"`
	RelayID     uint           `json:"relay_id"`
	ListenPort  int            `json:"listen_port"`
	TargetAddr  string         `json:"target_addr"`
	Enabled     *bool          `json:"enabled"`
	Options     map[string]any `json:"options"`
}

// TunnelQuarantineInput carries an optional isolation reason.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelQuarantineInput struct {
	Reason string `json:"reason"`
}

// TunnelConnectorView describes a single openflared connector replica.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelConnectorView struct {
	ID              uint                                  `json:"id"`
	ConnectorID     string                                `json:"connector_id"`
	TunnelID        uint                                  `json:"tunnel_id"`
	Hostname        string                                `json:"hostname"`
	OSArch          string                                `json:"os_arch"`
	PublicIP        string                                `json:"public_ip"`
	Version         string                                `json:"version"`
	EngineVersion   string                                `json:"engine_version"`
	ContractVersion int                                   `json:"contract_version"`
	Capabilities    []string                              `json:"capabilities"`
	AppliedRevision int64                                 `json:"applied_revision"`
	Status          string                                `json:"status"`
	EffectiveStatus string                                `json:"effective_status"`
	WSConnected     bool                                  `json:"ws_connected"`
	LastError       string                                `json:"last_error"`
	ConnectedRelays []protocol.TunnelConnectedRelayReport `json:"connected_relays"`
	Draining        bool                                  `json:"draining"`
	RevokedAt       *time.Time                            `json:"revoked_at"`
	LastSeenAt      *time.Time                            `json:"last_seen_at"`
	CreatedAt       time.Time                             `json:"created_at"`
	UpdatedAt       time.Time                             `json:"updated_at"`
}

// TunnelAttachmentView describes one dynamic connector <-> relay session lease.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelAttachmentView struct {
	ID              uint       `json:"id"`
	TunnelID        uint       `json:"tunnel_id"`
	ConnectorID     string     `json:"connector_id"`
	RelayID         uint       `json:"relay_id"`
	RelayName       string     `json:"relay_name"`
	RelayPublicID   string     `json:"relay_public_id"`
	ServiceName     string     `json:"service_name"`
	Transport       string     `json:"transport"`
	State           string     `json:"state"`
	RTTMs           int        `json:"rtt_ms"`
	ActiveStreams   int        `json:"active_streams"`
	RelayVerifiedAt *time.Time `json:"relay_verified_at"`
	LeaseExpiresAt  *time.Time `json:"lease_expires_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TunnelServiceView describes an L4 TCP/UDP forwarding service attached to a tunnel.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelServiceView struct {
	ID          uint           `json:"id"`
	TunnelID    uint           `json:"tunnel_id"`
	Name        string         `json:"name"`
	ServiceType string         `json:"service_type"`
	RelayID     uint           `json:"relay_id"`
	ListenPort  int            `json:"listen_port"`
	TargetAddr  string         `json:"target_addr"`
	Enabled     bool           `json:"enabled"`
	Options     map[string]any `json:"options"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// TunnelBoundRouteView summarizes an HTTP ProxyRoute routed through a tunnel.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelBoundRouteView struct {
	RouteID              uint     `json:"route_id"`
	SiteName             string   `json:"site_name"`
	Domains              []string `json:"domains"`
	Enabled              bool     `json:"enabled"`
	TunnelTargetAddr     string   `json:"tunnel_target_addr"`
	TunnelTargetProtocol string   `json:"tunnel_target_protocol"`
}

// TunnelView is the console list representation of a logical tunnel.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelView struct {
	ID                 uint           `json:"id"`
	TunnelID           string         `json:"tunnel_id"`
	Name               string         `json:"name"`
	Description        string         `json:"description"`
	PoolID             uint           `json:"pool_id"`
	PoolKey            string         `json:"pool_key"`
	HARelayCount       int            `json:"ha_relay_count"`
	PreferredTransport string         `json:"preferred_transport"`
	FallbackTransports []string       `json:"fallback_transports"`
	ClientProxyURL     string         `json:"client_proxy_url"`
	AllowedTargetCIDRs []string       `json:"allowed_target_cidrs"`
	BlockedTargetCIDRs []string       `json:"blocked_target_cidrs"`
	MaxConnectors      int            `json:"max_connectors"`
	Enabled            bool           `json:"enabled"`
	ConfigRevision     int64          `json:"config_revision"`
	Status             string         `json:"status"`
	OnlineConnectors   int            `json:"online_connectors"`
	ActiveAttachments  int            `json:"active_attachments"`
	BoundRoutesCount   int64          `json:"bound_routes_count"`
	AutoUpdateEnabled  bool           `json:"auto_update_enabled"`
	UpdateRequested    bool           `json:"update_requested"`
	UpdateChannel      string         `json:"update_channel"`
	UpdateTag          string         `json:"update_tag"`
	QuarantinedAt      *time.Time     `json:"quarantined_at"`
	QuarantineReason   string         `json:"quarantine_reason"`
	Metadata           map[string]any `json:"metadata"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// TunnelDeployView returns single-use/rotated connector deployment credentials.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelDeployView struct {
	Tunnel         TunnelView `json:"tunnel"`
	AccessToken    string     `json:"access_token"`
	BootstrapToken string     `json:"bootstrap_token"`
	ServerURL      string     `json:"server_url"`
	DeployCommand  string     `json:"deploy_command"`
}

// TunnelDetailView expands full topology, replicas, leases, L4 services, and bound HTTP routes for a tunnel.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelDetailView struct {
	Tunnel      TunnelView             `json:"tunnel"`
	Connectors  []TunnelConnectorView  `json:"connectors"`
	Attachments []TunnelAttachmentView `json:"attachments"`
	Services    []TunnelServiceView    `json:"services"`
	BoundRoutes []TunnelBoundRouteView `json:"bound_routes"`
}

// TunnelTopologyView aggregates the global mesh of pools, relays, tunnels, connectors, and active leases.
//
//nolint:revive // keeps explicit Tunnel prefix to distinguish from Relay and RelayPool types/handlers and preserve Swagger schema
type TunnelTopologyView struct {
	Pools       []PoolView             `json:"pools"`
	Relays      []RelayView            `json:"relays"`
	Tunnels     []TunnelView           `json:"tunnels"`
	Connectors  []TunnelConnectorView  `json:"connectors"`
	Attachments []TunnelAttachmentView `json:"attachments"`
}

// ListTunnels returns all logical tunnels with aggregated connector/lease/route counters.
func ListTunnels(ctx context.Context) ([]TunnelView, error) {
	tunnels, err := repository.ListTunnels(ctx)
	if err != nil {
		return nil, err
	}
	poolKeys := loadPoolKeyMap(ctx)
	allConnectors, err := repository.ListTunnelConnectors(ctx, 0)
	if err != nil {
		return nil, err
	}
	allAttachments, err := repository.ListTunnelAttachments(ctx, 0, 0, "")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	offlineWindow := resolveRelayOfflineWindow(ctx)

	onlineConnectorsByTunnel := make(map[uint]int)
	for _, c := range allConnectors {
		if computeConnectorEffectiveStatus(&c, websocket.IsConnectorConnected(c.ConnectorID), now, offlineWindow) == model.TunnelNodeStatusOnline {
			onlineConnectorsByTunnel[c.TunnelID]++
		}
	}
	activeAttByTunnel := make(map[uint]int)
	for _, att := range allAttachments {
		if att.State == model.TunnelAttachmentStateActive && (att.LeaseExpiresAt == nil || att.LeaseExpiresAt.After(now)) {
			activeAttByTunnel[att.TunnelID]++
		}
	}

	views := make([]TunnelView, 0, len(tunnels))
	for i := range tunnels {
		boundCount, err := repository.CountProxyRoutesByTunnelID(ctx, tunnels[i].ID)
		if err != nil {
			return nil, err
		}
		views = append(views, buildTunnelView(
			ctx, &tunnels[i], poolKeys[tunnels[i].PoolID],
			onlineConnectorsByTunnel[tunnels[i].ID], activeAttByTunnel[tunnels[i].ID], boundCount,
		))
	}
	return views, nil
}

// GetTunnelDetail returns complete metadata, connectors, attachments, L4 services, and bound HTTP routes.
func GetTunnelDetail(ctx context.Context, id uint) (*TunnelDetailView, error) {
	tunnel, err := repository.GetTunnelByID(ctx, id)
	if err != nil {
		return nil, err
	}
	poolKeys := loadPoolKeyMap(ctx)
	connectors, err := repository.ListTunnelConnectors(ctx, id)
	if err != nil {
		return nil, err
	}
	attachments, err := repository.ListTunnelAttachments(ctx, id, 0, "")
	if err != nil {
		return nil, err
	}
	services, err := repository.ListTunnelServices(ctx, id, 0)
	if err != nil {
		return nil, err
	}
	routes, err := repository.ListProxyRoutesByTunnelID(ctx, id)
	if err != nil {
		return nil, err
	}

	relayMap := loadRelayMap(ctx)
	now := time.Now().UTC()
	offlineWindow := resolveRelayOfflineWindow(ctx)

	connectorViews := make([]TunnelConnectorView, 0, len(connectors))
	onlineCount := 0
	for i := range connectors {
		cv := buildConnectorView(ctx, &connectors[i], now, offlineWindow)
		if cv.EffectiveStatus == model.TunnelNodeStatusOnline {
			onlineCount++
		}
		connectorViews = append(connectorViews, cv)
	}

	attViews := make([]TunnelAttachmentView, 0, len(attachments))
	activeAttCount := 0
	for i := range attachments {
		if attachments[i].State == model.TunnelAttachmentStateActive && (attachments[i].LeaseExpiresAt == nil || attachments[i].LeaseExpiresAt.After(now)) {
			activeAttCount++
		}
		attViews = append(attViews, buildAttachmentView(&attachments[i], relayMap))
	}

	svcViews := make([]TunnelServiceView, 0, len(services))
	for i := range services {
		svcViews = append(svcViews, buildServiceView(ctx, &services[i]))
	}

	routeViews := make([]TunnelBoundRouteView, 0, len(routes))
	for _, r := range routes {
		domains := make([]string, 0, len(r.ZoneDomains))
		for _, zd := range r.ZoneDomains {
			domains = append(domains, zd.Domain)
		}
		routeViews = append(routeViews, TunnelBoundRouteView{
			RouteID:              r.ID,
			SiteName:             r.SiteName,
			Domains:              domains,
			Enabled:              r.Enabled,
			TunnelTargetAddr:     r.TunnelTargetAddr,
			TunnelTargetProtocol: r.TunnelTargetProtocol,
		})
	}

	view := buildTunnelView(ctx, tunnel, poolKeys[tunnel.PoolID], onlineCount, activeAttCount, int64(len(routes)))
	return &TunnelDetailView{
		Tunnel:      view,
		Connectors:  connectorViews,
		Attachments: attViews,
		Services:    svcViews,
		BoundRoutes: routeViews,
	}, nil
}

// GetTunnelDeploy returns the deployment token and CLI launch command for an existing tunnel.
func GetTunnelDeploy(ctx context.Context, id uint) (*TunnelDeployView, error) {
	tunnel, err := repository.GetTunnelByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildTunnelDeployView(ctx, tunnel), nil
}

// CreateTunnel provisions a logical tunnel with an auto-generated access token.
func CreateTunnel(ctx context.Context, input TunnelInput) (*TunnelDeployView, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New(errTunnelNameRequired)
	}
	poolID, err := resolvePoolIDForMutation(ctx, input.PoolID, input.PoolKey)
	if err != nil {
		return nil, err
	}
	haCount := input.HARelayCount
	if haCount == 0 {
		haCount = resolveDefaultHARelays(ctx)
	}
	if haCount < 1 || haCount > 10 {
		return nil, errors.New(errTunnelHACountInvalid)
	}
	prefTransport, err := normalizePreferredTransport(input.PreferredTransport)
	if err != nil {
		return nil, err
	}
	fallbacks, err := normalizeFallbackTransports(input.FallbackTransports)
	if err != nil {
		return nil, err
	}
	allowedCIDRs, err := normalizeCIDRList(input.AllowedTargetCIDRs)
	if err != nil {
		return nil, err
	}
	blockedCIDRs, err := normalizeBlockedCIDRList(input.BlockedTargetCIDRs)
	if err != nil {
		return nil, err
	}
	if input.MaxConnectors < 0 {
		return nil, errors.New(errTunnelMaxConnectorsInvalid)
	}
	channel, err := normalizeUpdateChannel(input.UpdateChannel)
	if err != nil {
		return nil, err
	}
	metaJSON, err := encodeJSONObject(input.Metadata)
	if err != nil {
		return nil, errors.New(errInvalidJSONPayload)
	}

	tunnelID, err := GenerateRandomToken("tun-", tunnelIDTokenBytes)
	if err != nil {
		return nil, err
	}
	accessToken, err := GenerateRandomToken("tun_", tunnelSecretTokenBytes)
	if err != nil {
		return nil, err
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	tunnel := &model.Tunnel{
		TunnelID:               tunnelID,
		Name:                   name,
		Description:            strings.TrimSpace(input.Description),
		PoolID:                 poolID,
		AccessToken:            accessToken,
		HARelayCount:           haCount,
		PreferredTransport:     prefTransport,
		FallbackTransportsJSON: encodeJSONStringSlice(fallbacks),
		ClientProxyURL:         strings.TrimSpace(input.ClientProxyURL),
		AllowedTargetCIDRsJSON: encodeJSONStringSlice(allowedCIDRs),
		BlockedTargetCIDRsJSON: encodeJSONStringSlice(blockedCIDRs),
		MaxConnectors:          input.MaxConnectors,
		Enabled:                enabled,
		ConfigRevision:         1,
		AutoUpdateEnabled:      input.AutoUpdateEnabled,
		UpdateChannel:          channel,
		UpdateTag:              strings.TrimSpace(input.UpdateTag),
		MetadataJSON:           metaJSON,
	}

	if err := repository.CreateTunnel(ctx, tunnel); err != nil {
		return nil, err
	}
	return buildTunnelDeployView(ctx, tunnel), nil
}

// UpdateTunnel updates tunnel configuration, increments config_revision, and wakes live connectors.
func UpdateTunnel(ctx context.Context, id uint, input TunnelInput) (*TunnelView, error) {
	tunnel, err := repository.GetTunnelByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New(errTunnelNameRequired)
	}
	poolID, err := resolvePoolIDForMutation(ctx, input.PoolID, input.PoolKey)
	if err != nil {
		return nil, err
	}
	haCount := input.HARelayCount
	if haCount == 0 {
		haCount = tunnel.HARelayCount
	}
	if haCount < 1 || haCount > 10 {
		return nil, errors.New(errTunnelHACountInvalid)
	}
	prefTransport, err := normalizePreferredTransport(input.PreferredTransport)
	if err != nil {
		return nil, err
	}
	fallbacks, err := normalizeFallbackTransports(input.FallbackTransports)
	if err != nil {
		return nil, err
	}
	allowedCIDRs, err := normalizeCIDRList(input.AllowedTargetCIDRs)
	if err != nil {
		return nil, err
	}
	var blockedCIDRs []string
	if input.BlockedTargetCIDRs != nil {
		blockedCIDRs, err = normalizeCIDRList(input.BlockedTargetCIDRs)
		if err != nil {
			return nil, err
		}
	} else {
		blockedCIDRs = decodeJSONStringSlice(ctx, tunnel.BlockedTargetCIDRsJSON)
	}
	if input.MaxConnectors < 0 {
		return nil, errors.New(errTunnelMaxConnectorsInvalid)
	}
	channel, err := normalizeUpdateChannel(input.UpdateChannel)
	if err != nil {
		return nil, err
	}

	tunnel.Name = name
	tunnel.Description = strings.TrimSpace(input.Description)
	tunnel.PoolID = poolID
	tunnel.HARelayCount = haCount
	tunnel.PreferredTransport = prefTransport
	tunnel.FallbackTransportsJSON = encodeJSONStringSlice(fallbacks)
	tunnel.ClientProxyURL = strings.TrimSpace(input.ClientProxyURL)
	tunnel.AllowedTargetCIDRsJSON = encodeJSONStringSlice(allowedCIDRs)
	tunnel.BlockedTargetCIDRsJSON = encodeJSONStringSlice(blockedCIDRs)
	tunnel.MaxConnectors = input.MaxConnectors
	if input.Enabled != nil {
		tunnel.Enabled = *input.Enabled
	}
	tunnel.ConfigRevision++
	tunnel.AutoUpdateEnabled = input.AutoUpdateEnabled
	tunnel.UpdateChannel = channel
	tunnel.UpdateTag = strings.TrimSpace(input.UpdateTag)
	if input.Metadata != nil {
		metaJSON, err := encodeJSONObject(input.Metadata)
		if err != nil {
			return nil, errors.New(errInvalidJSONPayload)
		}
		tunnel.MetadataJSON = metaJSON
	}

	if err := repository.UpdateTunnelFields(ctx, tunnel,
		"name", "description", "pool_id", "ha_relay_count", "preferred_transport",
		"fallback_transports_json", "client_proxy_url", "allowed_target_cidrs_json",
		"blocked_target_cidrs_json", "max_connectors", "enabled", "config_revision",
		"auto_update_enabled", "update_channel", "update_tag", "metadata_json",
	); err != nil {
		return nil, err
	}

	websocket.NotifyTunnelChanged(ctx, tunnel.ID)
	websocket.NotifyTunnelRelayChanged(ctx, "")
	websocket.NotifyAgentConfigChanged(ctx)

	detail, err := GetTunnelDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	return &detail.Tunnel, nil
}

// DeleteTunnel removes an unbound logical tunnel and disconnects all active connectors.
func DeleteTunnel(ctx context.Context, id uint) error {
	if _, err := repository.GetTunnelByID(ctx, id); err != nil {
		return err
	}
	boundCount, err := repository.CountProxyRoutesByTunnelID(ctx, id)
	if err != nil {
		return err
	}
	if boundCount > 0 {
		return errors.New(errTunnelInUseByProxyRoutes)
	}
	websocket.DisconnectTunnel(ctx, id, "tunnel deleted")
	if err := repository.DeleteTunnel(ctx, id); err != nil {
		return err
	}
	websocket.NotifyTunnelRelayChanged(ctx, "")
	websocket.NotifyAgentConfigChanged(ctx)
	return nil
}

// RotateTunnelToken issues a new access token for connectors and drops active connections.
func RotateTunnelToken(ctx context.Context, id uint) (*TunnelDeployView, error) {
	tunnel, err := repository.GetTunnelByID(ctx, id)
	if err != nil {
		return nil, err
	}
	newToken, err := GenerateRandomToken("tun_", tunnelSecretTokenBytes)
	if err != nil {
		return nil, err
	}
	if err := repository.UpdateTunnelColumns(ctx, tunnel, map[string]any{colTunnelAccessToken: newToken}); err != nil {
		return nil, err
	}
	tunnel.AccessToken = newToken
	websocket.DisconnectTunnel(ctx, tunnel.ID, "tunnel access token rotated")
	return buildTunnelDeployView(ctx, tunnel), nil
}

// QuarantineTunnel isolates a tunnel, rotates its token, drops connectors, and updates Edge routing.
func QuarantineTunnel(ctx context.Context, id uint, reason string) (*TunnelView, error) {
	tunnel, err := repository.GetTunnelByID(ctx, id)
	if err != nil {
		return nil, err
	}
	newToken, err := GenerateRandomToken("tun_revoked_", tunnelSecretTokenBytes)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = "isolated by administrator"
	}
	if err := repository.UpdateTunnelColumns(ctx, tunnel, map[string]any{
		colTunnelAccessToken:      newToken,
		colTunnelQuarantinedAt:    &now,
		colTunnelQuarantineReason: trimmedReason,
	}); err != nil {
		return nil, err
	}
	websocket.DisconnectTunnel(ctx, tunnel.ID, trimmedReason)
	websocket.NotifyTunnelRelayChanged(ctx, "")
	websocket.NotifyAgentConfigChanged(ctx)

	detail, err := GetTunnelDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	return &detail.Tunnel, nil
}

// UnquarantineTunnel clears tunnel quarantine and issues a fresh access token.
func UnquarantineTunnel(ctx context.Context, id uint) (*TunnelDeployView, error) {
	tunnel, err := repository.GetTunnelByID(ctx, id)
	if err != nil {
		return nil, err
	}
	newToken, err := GenerateRandomToken("tun_", tunnelSecretTokenBytes)
	if err != nil {
		return nil, err
	}
	if err := repository.UpdateTunnelColumns(ctx, tunnel, map[string]any{
		colTunnelAccessToken:      newToken,
		colTunnelQuarantinedAt:    nil,
		colTunnelQuarantineReason: "",
	}); err != nil {
		return nil, err
	}
	tunnel.AccessToken = newToken
	tunnel.QuarantinedAt = nil
	tunnel.QuarantineReason = ""
	websocket.NotifyAgentConfigChanged(ctx)
	return buildTunnelDeployView(ctx, tunnel), nil
}

// RevokeConnector permanently revokes a specific connector replica and tears down its attachments.
func RevokeConnector(ctx context.Context, tunnelID uint, connectorID string) error {
	if _, err := repository.GetTunnelByID(ctx, tunnelID); err != nil {
		return err
	}
	connector, err := repository.GetTunnelConnectorByConnectorID(ctx, strings.TrimSpace(connectorID))
	if err != nil {
		return err
	}
	if connector.TunnelID != tunnelID {
		return errors.New(errTunnelConnectorNotFound)
	}
	now := time.Now().UTC()
	if err := repository.UpdateTunnelConnectorColumns(ctx, connector.ID, map[string]any{
		"revoked_at":    &now,
		colTunnelStatus: model.TunnelNodeStatusQuarantined,
	}); err != nil {
		return err
	}
	if err := repository.DeleteTunnelAttachmentsByConnector(ctx, connector.ConnectorID); err != nil {
		return err
	}
	websocket.DisconnectTunnelConnector(ctx, connector.ConnectorID, "connector revoked", true)
	websocket.NotifyTunnelRelayChanged(ctx, "")
	websocket.NotifyAgentConfigChanged(ctx)
	return nil
}

// RequestTunnelBinaryUpdate schedules a managed self-update across a tunnel's connectors.
func RequestTunnelBinaryUpdate(ctx context.Context, id uint, input BinaryUpdateInput) (*TunnelView, error) {
	tunnel, err := repository.GetTunnelByID(ctx, id)
	if err != nil {
		return nil, err
	}
	channel, err := normalizeUpdateChannel(input.Channel)
	if err != nil {
		return nil, err
	}
	tag := strings.TrimSpace(input.Tag)
	if tag == "" {
		release, err := GetLatestTunnelRelease(ctx, channel)
		if err != nil {
			return nil, err
		}
		tag = release.TagName
	}
	if _, _, ok := parseTunnelReleaseVersion(tag); !ok {
		return nil, errors.New(errRelayUpdateTagRequired)
	}
	if err := repository.UpdateTunnelColumns(ctx, tunnel, map[string]any{
		colTunnelUpdateRequested: true,
		"update_channel":         channel,
		"update_tag":             tag,
	}); err != nil {
		return nil, err
	}
	websocket.NotifyTunnelChanged(ctx, tunnel.ID)
	detail, err := GetTunnelDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	return &detail.Tunnel, nil
}

// CreateTunnelService creates an L4 TCP/UDP exposed service under a tunnel.
func CreateTunnelService(ctx context.Context, tunnelID uint, input TunnelServiceInput) (*TunnelServiceView, error) {
	if _, err := repository.GetTunnelByID(ctx, tunnelID); err != nil {
		return nil, err
	}
	svc, err := buildTunnelServiceModel(ctx, nil, tunnelID, input)
	if err != nil {
		return nil, err
	}
	if err := repository.CreateTunnelService(ctx, svc); err != nil {
		return nil, err
	}
	bumpTunnelAndNotify(ctx, tunnelID)
	view := buildServiceView(ctx, svc)
	return &view, nil
}

// UpdateTunnelService modifies an L4 TCP/UDP exposed service under a tunnel.
func UpdateTunnelService(ctx context.Context, tunnelID uint, serviceID uint, input TunnelServiceInput) (*TunnelServiceView, error) {
	existing, err := repository.GetTunnelServiceByID(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	if existing.TunnelID != tunnelID {
		return nil, errors.New(errTunnelServiceNotFound)
	}
	svc, err := buildTunnelServiceModel(ctx, existing, tunnelID, input)
	if err != nil {
		return nil, err
	}
	if err := repository.UpdateTunnelServiceFields(ctx, svc,
		"name", "service_type", "relay_id", "listen_port", "target_addr", "enabled", "options_json",
	); err != nil {
		return nil, err
	}
	bumpTunnelAndNotify(ctx, tunnelID)
	view := buildServiceView(ctx, svc)
	return &view, nil
}

// DeleteTunnelService removes an L4 TCP/UDP exposed service.
func DeleteTunnelService(ctx context.Context, tunnelID uint, serviceID uint) error {
	existing, err := repository.GetTunnelServiceByID(ctx, serviceID)
	if err != nil {
		return err
	}
	if existing.TunnelID != tunnelID {
		return errors.New(errTunnelServiceNotFound)
	}
	if err := repository.DeleteTunnelService(ctx, serviceID); err != nil {
		return err
	}
	bumpTunnelAndNotify(ctx, tunnelID)
	return nil
}

// GetTopology returns the global control-plane view of pools, relays, tunnels, connectors, and attachments.
func GetTopology(ctx context.Context) (*TunnelTopologyView, error) {
	pools, err := ListPools(ctx)
	if err != nil {
		return nil, err
	}
	relays, err := ListRelays(ctx)
	if err != nil {
		return nil, err
	}
	tunnels, err := ListTunnels(ctx)
	if err != nil {
		return nil, err
	}
	connectors, err := repository.ListTunnelConnectors(ctx, 0)
	if err != nil {
		return nil, err
	}
	attachments, err := repository.ListTunnelAttachments(ctx, 0, 0, "")
	if err != nil {
		return nil, err
	}

	relayMap := loadRelayMap(ctx)
	now := time.Now().UTC()
	offlineWindow := resolveRelayOfflineWindow(ctx)

	connectorViews := make([]TunnelConnectorView, 0, len(connectors))
	for i := range connectors {
		connectorViews = append(connectorViews, buildConnectorView(ctx, &connectors[i], now, offlineWindow))
	}
	attViews := make([]TunnelAttachmentView, 0, len(attachments))
	for i := range attachments {
		attViews = append(attViews, buildAttachmentView(&attachments[i], relayMap))
	}
	return &TunnelTopologyView{
		Pools:       pools,
		Relays:      relays,
		Tunnels:     tunnels,
		Connectors:  connectorViews,
		Attachments: attViews,
	}, nil
}

func bumpTunnelAndNotify(ctx context.Context, tunnelID uint) {
	if err := repository.IncrementTunnelConfigRevision(ctx, tunnelID); err != nil {
		return
	}
	websocket.NotifyTunnelChanged(ctx, tunnelID)
	websocket.NotifyTunnelRelayChanged(ctx, "")
}

func buildTunnelServiceModel(ctx context.Context, target *model.TunnelService, tunnelID uint, input TunnelServiceInput) (*model.TunnelService, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New(errTunnelServiceNameRequired)
	}
	svcType := strings.ToLower(strings.TrimSpace(input.ServiceType))
	if svcType != "tcp" && svcType != "udp" {
		return nil, errors.New(errTunnelServiceTypeInvalid)
	}
	if input.ListenPort < 1 || input.ListenPort > 65535 {
		return nil, errors.New(errTunnelServicePortConflict)
	}
	targetAddr := strings.TrimSpace(input.TargetAddr)
	if _, _, err := net.SplitHostPort(targetAddr); err != nil {
		return nil, errors.New(errTunnelServiceAddrInvalid)
	}
	if input.RelayID > 0 {
		if _, err := repository.GetTunnelRelayByID(ctx, input.RelayID); err != nil {
			return nil, errors.New(errRelayNotFound)
		}
	}
	optionsJSON, err := encodeJSONObject(input.Options)
	if err != nil {
		return nil, errors.New(errInvalidJSONPayload)
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	if target == nil {
		target = &model.TunnelService{TunnelID: tunnelID}
	}
	target.Name = name
	target.ServiceType = svcType
	target.RelayID = input.RelayID
	target.ListenPort = input.ListenPort
	target.TargetAddr = targetAddr
	target.Enabled = enabled
	target.OptionsJSON = optionsJSON
	return target, nil
}

func buildTunnelDeployView(ctx context.Context, tunnel *model.Tunnel) *TunnelDeployView {
	poolKeys := loadPoolKeyMap(ctx)
	serverURL := resolveControlPlaneServerURL(ctx)
	poolKey := poolKeys[tunnel.PoolID]
	wrapped, err := EncodeBootstrapToken(serverURL, bootstrapKindTunnel, tunnel.AccessToken, poolKey)
	if err != nil {
		wrapped = tunnel.AccessToken
	}
	cmd := fmt.Sprintf("openflared --token %s", wrapped)
	if serverURL == "" {
		cmd = fmt.Sprintf("openflared --server-url <SERVER_URL> --token %s", tunnel.AccessToken)
	}
	boundCount, _ := repository.CountProxyRoutesByTunnelID(ctx, tunnel.ID)
	view := buildTunnelView(ctx, tunnel, poolKey, 0, 0, boundCount)
	return &TunnelDeployView{
		Tunnel:         view,
		AccessToken:    tunnel.AccessToken,
		BootstrapToken: wrapped,
		ServerURL:      serverURL,
		DeployCommand:  cmd,
	}
}

func buildTunnelView(
	ctx context.Context,
	tunnel *model.Tunnel,
	poolKey string,
	onlineConnectors int,
	activeAttachments int,
	boundRoutesCount int64,
) TunnelView {
	status := model.TunnelNodeStatusOffline
	switch {
	case tunnel.IsQuarantined():
		status = model.TunnelNodeStatusQuarantined
	case !tunnel.Enabled:
		status = "disabled"
	case onlineConnectors > 0 && activeAttachments > 0:
		status = model.TunnelNodeStatusOnline
	case onlineConnectors > 0:
		status = "degraded"
	}
	return TunnelView{
		ID:                 tunnel.ID,
		TunnelID:           tunnel.TunnelID,
		Name:               tunnel.Name,
		Description:        tunnel.Description,
		PoolID:             tunnel.PoolID,
		PoolKey:            poolKey,
		HARelayCount:       tunnel.HARelayCount,
		PreferredTransport: tunnel.PreferredTransport,
		FallbackTransports: decodeJSONStringSlice(ctx, tunnel.FallbackTransportsJSON),
		ClientProxyURL:     tunnel.ClientProxyURL,
		AllowedTargetCIDRs: decodeJSONStringSlice(ctx, tunnel.AllowedTargetCIDRsJSON),
		BlockedTargetCIDRs: decodeJSONStringSlice(ctx, tunnel.BlockedTargetCIDRsJSON),
		MaxConnectors:      tunnel.MaxConnectors,
		Enabled:            tunnel.Enabled,
		ConfigRevision:     tunnel.ConfigRevision,
		Status:             status,
		OnlineConnectors:   onlineConnectors,
		ActiveAttachments:  activeAttachments,
		BoundRoutesCount:   boundRoutesCount,
		AutoUpdateEnabled:  tunnel.AutoUpdateEnabled,
		UpdateRequested:    tunnel.UpdateRequested,
		UpdateChannel:      tunnel.UpdateChannel,
		UpdateTag:          tunnel.UpdateTag,
		QuarantinedAt:      tunnel.QuarantinedAt,
		QuarantineReason:   tunnel.QuarantineReason,
		Metadata:           decodeJSONObject(ctx, tunnel.MetadataJSON),
		CreatedAt:          tunnel.CreatedAt,
		UpdatedAt:          tunnel.UpdatedAt,
	}
}

func buildConnectorView(ctx context.Context, c *model.TunnelConnector, now time.Time, offlineWindow time.Duration) TunnelConnectorView {
	wsConnected := websocket.IsConnectorConnected(c.ConnectorID)
	var connectedRelays []protocol.TunnelConnectedRelayReport
	if trimmed := strings.TrimSpace(c.ConnectedRelaysJSON); trimmed != "" && trimmed != "[]" {
		if err := json.Unmarshal([]byte(trimmed), &connectedRelays); err != nil {
			connectedRelays = nil
		}
	}
	if connectedRelays == nil {
		connectedRelays = []protocol.TunnelConnectedRelayReport{}
	}
	return TunnelConnectorView{
		ID:              c.ID,
		ConnectorID:     c.ConnectorID,
		TunnelID:        c.TunnelID,
		Hostname:        c.Hostname,
		OSArch:          c.OSArch,
		PublicIP:        c.PublicIP,
		Version:         c.Version,
		EngineVersion:   c.EngineVersion,
		ContractVersion: c.ContractVersion,
		Capabilities:    decodeJSONStringSlice(ctx, c.CapabilitiesJSON),
		AppliedRevision: c.AppliedRevision,
		Status:          c.Status,
		EffectiveStatus: computeConnectorEffectiveStatus(c, wsConnected, now, offlineWindow),
		WSConnected:     wsConnected,
		LastError:       c.LastError,
		ConnectedRelays: connectedRelays,
		Draining:        c.Draining,
		RevokedAt:       c.RevokedAt,
		LastSeenAt:      c.LastSeenAt,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

func computeConnectorEffectiveStatus(c *model.TunnelConnector, wsConnected bool, now time.Time, offlineWindow time.Duration) string {
	if c.IsRevoked() {
		return model.TunnelNodeStatusQuarantined
	}
	if c.Draining {
		return model.TunnelNodeStatusDraining
	}
	if wsConnected || (c.LastSeenAt != nil && now.Sub(*c.LastSeenAt) <= offlineWindow) {
		return model.TunnelNodeStatusOnline
	}
	return model.TunnelNodeStatusOffline
}

func buildAttachmentView(att *model.TunnelAttachment, relays map[uint]model.TunnelRelay) TunnelAttachmentView {
	relay := relays[att.RelayID]
	return TunnelAttachmentView{
		ID:              att.ID,
		TunnelID:        att.TunnelID,
		ConnectorID:     att.ConnectorID,
		RelayID:         att.RelayID,
		RelayName:       relay.Name,
		RelayPublicID:   relay.RelayID,
		ServiceName:     att.ServiceName,
		Transport:       att.Transport,
		State:           att.State,
		RTTMs:           att.RTTMs,
		ActiveStreams:   att.ActiveStreams,
		RelayVerifiedAt: att.RelayVerifiedAt,
		LeaseExpiresAt:  att.LeaseExpiresAt,
		UpdatedAt:       att.UpdatedAt,
	}
}

func buildServiceView(ctx context.Context, svc *model.TunnelService) TunnelServiceView {
	return TunnelServiceView{
		ID:          svc.ID,
		TunnelID:    svc.TunnelID,
		Name:        svc.Name,
		ServiceType: svc.ServiceType,
		RelayID:     svc.RelayID,
		ListenPort:  svc.ListenPort,
		TargetAddr:  svc.TargetAddr,
		Enabled:     svc.Enabled,
		Options:     decodeJSONObject(ctx, svc.OptionsJSON),
		CreatedAt:   svc.CreatedAt,
		UpdatedAt:   svc.UpdatedAt,
	}
}

func loadRelayMap(ctx context.Context) map[uint]model.TunnelRelay {
	relays, err := repository.ListTunnelRelays(ctx)
	if err != nil {
		return map[uint]model.TunnelRelay{}
	}
	out := make(map[uint]model.TunnelRelay, len(relays))
	for _, r := range relays {
		out[r.ID] = r
	}
	return out
}

func normalizePreferredTransport(raw string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(raw))
	if t == "" {
		return model.TunnelTransportAuto, nil
	}
	if t != model.TunnelTransportAuto && !slices.Contains(validTunnelTransports, t) {
		return "", errors.New(errTunnelTransportInvalid)
	}
	return t, nil
}

func normalizeFallbackTransports(input []string) ([]string, error) {
	if len(input) == 0 {
		return []string{model.TunnelTransportNoise, model.TunnelTransportWebSocket, model.TunnelTransportTCP}, nil
	}
	return normalizeRelayTransports(input)
}

func normalizeCIDRList(input []string) ([]string, error) {
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, raw := range input {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		_, network, err := net.ParseCIDR(trimmed)
		if err != nil {
			return nil, errors.New(errTunnelCIDRInvalid)
		}
		canonical := network.String()
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	return out, nil
}

func normalizeBlockedCIDRList(input []string) ([]string, error) {
	if input == nil {
		return []string{"169.254.169.254/32"}, nil
	}
	return normalizeCIDRList(input)
}
