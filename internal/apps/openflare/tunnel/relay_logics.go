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

	ofgeoip "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	pkggeoip "github.com/Rain-kl/Wavelet/pkg/geoip"
	"github.com/Rain-kl/Wavelet/pkg/geoip/iputil"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	defaultRelayTransportPort = 7443
	defaultRelayGatewayPort   = 8443
	defaultRelayWeight        = 100
)

var validTunnelTransports = []string{
	model.TunnelTransportNoise,
	model.TunnelTransportWebSocket,
	model.TunnelTransportTLS,
	model.TunnelTransportTCP,
}

// RelayInput creates or updates a tunnel relay PoP.
type RelayInput struct {
	PoolID              uint           `json:"pool_id"`
	PoolKey             string         `json:"pool_key"`
	Name                string         `json:"name"`
	PublicAddr          string         `json:"public_addr"`
	IPManualOverride    *bool          `json:"ip_manual_override"`
	TransportPort       int            `json:"transport_port"`
	WSPort              int            `json:"ws_port"`
	GatewayAddr         string         `json:"gateway_addr"`
	GatewayPort         int            `json:"gateway_port"`
	GatewayTLS          bool           `json:"gateway_tls"`
	SupportedTransports []string       `json:"supported_transports"`
	GeoName             string         `json:"geo_name"`
	GeoLatitude         *float64       `json:"geo_latitude"`
	GeoLongitude        *float64       `json:"geo_longitude"`
	GeoManualOverride   bool           `json:"geo_manual_override"`
	ZoneTag             string         `json:"zone_tag"`
	ColocatedEdgeNodeID string         `json:"colocated_edge_node_id"`
	Weight              int            `json:"weight"`
	MaxTunnels          int            `json:"max_tunnels"`
	AutoUpdateEnabled   bool           `json:"auto_update_enabled"`
	UpdateChannel       string         `json:"update_channel"`
	UpdateTag           string         `json:"update_tag"`
	Metadata            map[string]any `json:"metadata"`
}

// RelayQuarantineInput isolates a relay PoP with an optional reason.
type RelayQuarantineInput struct {
	Reason string `json:"reason"`
}

// RelayDrainInput toggles graceful draining on a relay PoP.
type RelayDrainInput struct {
	Draining bool `json:"draining"`
}

// RelayBootstrapView exposes the global relay discovery bootstrap token and server deploy hint.
type RelayBootstrapView struct {
	DiscoveryToken string `json:"discovery_token"`
	BootstrapToken string `json:"bootstrap_token"`
	ServerURL      string `json:"server_url"`
	DeployCommand  string `json:"deploy_command"`
}

// RelayDeployView carries single-use or rotated relay credentials and CLI launch command.
type RelayDeployView struct {
	Relay          RelayView `json:"relay"`
	AccessToken    string    `json:"access_token"`
	BootstrapToken string    `json:"bootstrap_token"`
	ServerURL      string    `json:"server_url"`
	DeployCommand  string    `json:"deploy_command"`
}

// RelayView is the console API representation of an openflare-relay PoP.
type RelayView struct {
	ID                  uint           `json:"id"`
	RelayID             string         `json:"relay_id"`
	PoolID              uint           `json:"pool_id"`
	PoolKey             string         `json:"pool_key"`
	Name                string         `json:"name"`
	PublicAddr          string         `json:"public_addr"`
	IPManualOverride    bool           `json:"ip_manual_override"`
	TransportPort       int            `json:"transport_port"`
	WSPort              int            `json:"ws_port"`
	GatewayAddr         string         `json:"gateway_addr"`
	GatewayPort         int            `json:"gateway_port"`
	GatewayTLS          bool           `json:"gateway_tls"`
	SupportedTransports []string       `json:"supported_transports"`
	NoisePublicKey      string         `json:"noise_public_key"`
	GeoName             string         `json:"geo_name"`
	GeoLatitude         *float64       `json:"geo_latitude"`
	GeoLongitude        *float64       `json:"geo_longitude"`
	GeoManualOverride   bool           `json:"geo_manual_override"`
	ZoneTag             string         `json:"zone_tag"`
	ColocatedEdgeNodeID string         `json:"colocated_edge_node_id"`
	Weight              int            `json:"weight"`
	MaxTunnels          int            `json:"max_tunnels"`
	Draining            bool           `json:"draining"`
	Status              string         `json:"status"`
	EffectiveStatus     string         `json:"effective_status"`
	WSConnected         bool           `json:"ws_connected"`
	RuntimeStatus       string         `json:"runtime_status"`
	ActiveConnectors    int            `json:"active_connectors"`
	ActiveStreams       int            `json:"active_streams"`
	CPUUsagePercent     float64        `json:"cpu_usage_percent"`
	MemoryUsagePercent  float64        `json:"memory_usage_percent"`
	Version             string         `json:"version"`
	EngineVersion       string         `json:"engine_version"`
	ContractVersion     int            `json:"contract_version"`
	Capabilities        []string       `json:"capabilities"`
	AutoUpdateEnabled   bool           `json:"auto_update_enabled"`
	UpdateRequested     bool           `json:"update_requested"`
	UpdateChannel       string         `json:"update_channel"`
	UpdateTag           string         `json:"update_tag"`
	LastSeenAt          *time.Time     `json:"last_seen_at"`
	LastError           string         `json:"last_error"`
	QuarantinedAt       *time.Time     `json:"quarantined_at"`
	QuarantineReason    string         `json:"quarantine_reason"`
	Metadata            map[string]any `json:"metadata"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

// ListRelays returns all registered relay PoPs ordered newest first.
func ListRelays(ctx context.Context) ([]RelayView, error) {
	relays, err := repository.ListTunnelRelays(ctx)
	if err != nil {
		return nil, err
	}
	poolKeys := loadPoolKeyMap(ctx)
	offlineWindow := resolveRelayOfflineWindow(ctx)
	now := time.Now().UTC()

	views := make([]RelayView, 0, len(relays))
	for i := range relays {
		views = append(views, buildRelayView(ctx, &relays[i], poolKeys[relays[i].PoolID], now, offlineWindow))
	}
	return views, nil
}

// GetRelay returns one relay PoP view by primary ID.
func GetRelay(ctx context.Context, id uint) (*RelayView, error) {
	relay, err := repository.GetTunnelRelayByID(ctx, id)
	if err != nil {
		return nil, err
	}
	poolKeys := loadPoolKeyMap(ctx)
	view := buildRelayView(ctx, relay, poolKeys[relay.PoolID], time.Now().UTC(), resolveRelayOfflineWindow(ctx))
	return &view, nil
}

// GetRelayDeploy returns deployment command and credentials for a relay PoP.
func GetRelayDeploy(ctx context.Context, id uint) (*RelayDeployView, error) {
	relay, err := repository.GetTunnelRelayByID(ctx, id)
	if err != nil {
		return nil, err
	}
	poolKeys := loadPoolKeyMap(ctx)
	view := buildRelayView(ctx, relay, poolKeys[relay.PoolID], time.Now().UTC(), resolveRelayOfflineWindow(ctx))
	return buildRelayDeployView(ctx, relay, view, poolKeys[relay.PoolID]), nil
}

// CreateRelay provisions a relay PoP with generated credentials and Noise X25519 keys.
func CreateRelay(ctx context.Context, input RelayInput) (*RelayDeployView, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New(errRelayNameRequired)
	}
	publicAddr := strings.TrimSpace(input.PublicAddr)
	if publicAddr == "" {
		return nil, errors.New(errRelayPublicAddrRequired)
	}
	poolID, err := resolvePoolIDForMutation(ctx, input.PoolID, input.PoolKey)
	if err != nil {
		return nil, err
	}
	transportPort, wsPort, gatewayPort, err := validateRelayPorts(input.TransportPort, input.WSPort, input.GatewayPort)
	if err != nil {
		return nil, err
	}
	transports, err := normalizeRelayTransports(input.SupportedTransports)
	if err != nil {
		return nil, err
	}
	channel, err := normalizeUpdateChannel(input.UpdateChannel)
	if err != nil {
		return nil, err
	}
	metaJSON, err := encodeJSONObject(input.Metadata)
	if err != nil {
		return nil, errors.New(errInvalidJSONPayload)
	}

	relayID, err := GenerateRandomToken("relay-", 12)
	if err != nil {
		return nil, err
	}
	accessToken, err := GenerateRandomToken("rly_", 24)
	if err != nil {
		return nil, err
	}
	gatewaySecret, err := GenerateRandomToken("gws_", 24)
	if err != nil {
		return nil, err
	}
	privKey, pubKey, err := GenerateNoiseKeypair()
	if err != nil {
		return nil, err
	}

	weight := input.Weight
	if weight <= 0 {
		weight = defaultRelayWeight
	}
	ipManual := true
	if input.IPManualOverride != nil {
		ipManual = *input.IPManualOverride
	}

	relay := &model.TunnelRelay{
		RelayID:                 relayID,
		PoolID:                  poolID,
		Name:                    name,
		AccessToken:             accessToken,
		PublicAddr:              publicAddr,
		IPManualOverride:        ipManual,
		TransportPort:           transportPort,
		WSPort:                  wsPort,
		GatewayAddr:             strings.TrimSpace(input.GatewayAddr),
		GatewayPort:             gatewayPort,
		GatewayTLS:              input.GatewayTLS,
		GatewaySecret:           gatewaySecret,
		SupportedTransportsJSON: encodeJSONStringSlice(transports),
		NoisePrivateKey:         privKey,
		NoisePublicKey:          pubKey,
		GeoName:                 strings.TrimSpace(input.GeoName),
		GeoLatitude:             cloneCoordinate(input.GeoLatitude),
		GeoLongitude:            cloneCoordinate(input.GeoLongitude),
		GeoManualOverride:       input.GeoManualOverride,
		ZoneTag:                 strings.TrimSpace(input.ZoneTag),
		ColocatedEdgeNodeID:     strings.TrimSpace(input.ColocatedEdgeNodeID),
		Weight:                  weight,
		MaxTunnels:              max(0, input.MaxTunnels),
		Status:                  model.TunnelNodeStatusPending,
		RuntimeStatus:           "unknown",
		CapabilitiesJSON:        "[]",
		AutoUpdateEnabled:       input.AutoUpdateEnabled,
		UpdateChannel:           channel,
		UpdateTag:               strings.TrimSpace(input.UpdateTag),
		MetadataJSON:            metaJSON,
	}
	applyRelayGeo(ctx, relay, publicAddr)

	if err := repository.CreateTunnelRelay(ctx, relay); err != nil {
		return nil, err
	}
	websocket.NotifyTunnelPoolsChanged(ctx)

	poolKeys := loadPoolKeyMap(ctx)
	view := buildRelayView(ctx, relay, poolKeys[relay.PoolID], time.Now().UTC(), resolveRelayOfflineWindow(ctx))
	return buildRelayDeployView(ctx, relay, view, poolKeys[relay.PoolID]), nil
}

// UpdateRelay updates relay PoP configuration and wakes the relay + tunnels if needed.
func UpdateRelay(ctx context.Context, id uint, input RelayInput) (*RelayView, error) {
	relay, err := repository.GetTunnelRelayByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New(errRelayNameRequired)
	}
	publicAddr := strings.TrimSpace(input.PublicAddr)
	if publicAddr == "" {
		return nil, errors.New(errRelayPublicAddrRequired)
	}
	poolID, err := resolvePoolIDForMutation(ctx, input.PoolID, input.PoolKey)
	if err != nil {
		return nil, err
	}
	transportPort, wsPort, gatewayPort, err := validateRelayPorts(input.TransportPort, input.WSPort, input.GatewayPort)
	if err != nil {
		return nil, err
	}
	transports, err := normalizeRelayTransports(input.SupportedTransports)
	if err != nil {
		return nil, err
	}
	channel, err := normalizeUpdateChannel(input.UpdateChannel)
	if err != nil {
		return nil, err
	}

	relay.PoolID = poolID
	relay.Name = name
	relay.PublicAddr = publicAddr
	if input.IPManualOverride != nil {
		relay.IPManualOverride = *input.IPManualOverride
	}
	relay.TransportPort = transportPort
	relay.WSPort = wsPort
	relay.GatewayAddr = strings.TrimSpace(input.GatewayAddr)
	relay.GatewayPort = gatewayPort
	relay.GatewayTLS = input.GatewayTLS
	relay.SupportedTransportsJSON = encodeJSONStringSlice(transports)
	relay.GeoManualOverride = input.GeoManualOverride
	if input.GeoManualOverride {
		relay.GeoName = strings.TrimSpace(input.GeoName)
		relay.GeoLatitude = cloneCoordinate(input.GeoLatitude)
		relay.GeoLongitude = cloneCoordinate(input.GeoLongitude)
	} else {
		applyRelayGeo(ctx, relay, publicAddr)
	}
	relay.ZoneTag = strings.TrimSpace(input.ZoneTag)
	relay.ColocatedEdgeNodeID = strings.TrimSpace(input.ColocatedEdgeNodeID)
	if input.Weight > 0 {
		relay.Weight = input.Weight
	}
	relay.MaxTunnels = max(0, input.MaxTunnels)
	relay.AutoUpdateEnabled = input.AutoUpdateEnabled
	relay.UpdateChannel = channel
	relay.UpdateTag = strings.TrimSpace(input.UpdateTag)
	if input.Metadata != nil {
		metaJSON, err := encodeJSONObject(input.Metadata)
		if err != nil {
			return nil, errors.New(errInvalidJSONPayload)
		}
		relay.MetadataJSON = metaJSON
	}

	if err := repository.UpdateTunnelRelayFields(ctx, relay,
		"pool_id", "name", "public_addr", "ip_manual_override",
		"transport_port", "ws_port", "gateway_addr", "gateway_port", "gateway_tls",
		"supported_transports_json", "geo_name", "geo_latitude", "geo_longitude",
		"geo_manual_override", "zone_tag", "colocated_edge_node_id", "weight",
		"max_tunnels", "auto_update_enabled", "update_channel", "update_tag", "metadata_json",
	); err != nil {
		return nil, err
	}

	websocket.NotifyTunnelRelayChanged(ctx, relay.RelayID)
	websocket.NotifyTunnelChanged(ctx, 0)
	websocket.NotifyAgentConfigChanged(ctx)

	poolKeys := loadPoolKeyMap(ctx)
	view := buildRelayView(ctx, relay, poolKeys[relay.PoolID], time.Now().UTC(), resolveRelayOfflineWindow(ctx))
	return &view, nil
}

// DeleteRelay removes a relay PoP, invalidating its attachments and waking Edge + connectors.
func DeleteRelay(ctx context.Context, id uint) error {
	relay, err := repository.GetTunnelRelayByID(ctx, id)
	if err != nil {
		return err
	}
	websocket.DisconnectTunnelRelay(ctx, relay.RelayID, "relay deleted")
	if err := repository.DeleteTunnelRelay(ctx, id); err != nil {
		return err
	}
	websocket.NotifyTunnelChanged(ctx, 0)
	websocket.NotifyAgentConfigChanged(ctx)
	return nil
}

// RotateRelayToken rotates a relay PoP access token and drops active WS connections using the old token.
func RotateRelayToken(ctx context.Context, id uint) (*RelayDeployView, error) {
	relay, err := repository.GetTunnelRelayByID(ctx, id)
	if err != nil {
		return nil, err
	}
	newToken, err := GenerateRandomToken("rly_", 24)
	if err != nil {
		return nil, err
	}
	if err := repository.UpdateTunnelRelayColumns(ctx, relay, map[string]any{"access_token": newToken}); err != nil {
		return nil, err
	}
	relay.AccessToken = newToken
	websocket.DisconnectTunnelRelay(ctx, relay.RelayID, "access token rotated")
	poolKeys := loadPoolKeyMap(ctx)
	view := buildRelayView(ctx, relay, poolKeys[relay.PoolID], time.Now().UTC(), resolveRelayOfflineWindow(ctx))
	return buildRelayDeployView(ctx, relay, view, poolKeys[relay.PoolID]), nil
}

// SetRelayDraining marks a relay as draining so connectors perform Make-Before-Break migration away from it.
func SetRelayDraining(ctx context.Context, id uint, draining bool) (*RelayView, error) {
	relay, err := repository.GetTunnelRelayByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := repository.UpdateTunnelRelayColumns(ctx, relay, map[string]any{"draining": draining}); err != nil {
		return nil, err
	}
	relay.Draining = draining
	websocket.NotifyTunnelRelayChanged(ctx, relay.RelayID)
	websocket.NotifyTunnelChanged(ctx, 0)
	websocket.NotifyAgentConfigChanged(ctx)

	poolKeys := loadPoolKeyMap(ctx)
	view := buildRelayView(ctx, relay, poolKeys[relay.PoolID], time.Now().UTC(), resolveRelayOfflineWindow(ctx))
	return &view, nil
}

// QuarantineRelay isolates a relay PoP, revokes its token, disconnects WS, and removes it from Edge routing.
func QuarantineRelay(ctx context.Context, id uint, reason string) (*RelayView, error) {
	relay, err := repository.GetTunnelRelayByID(ctx, id)
	if err != nil {
		return nil, err
	}
	newToken, err := GenerateRandomToken("rly_revoked_", 24)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = "isolated by administrator"
	}
	changes := map[string]any{
		"access_token":      newToken,
		"status":            model.TunnelNodeStatusQuarantined,
		"quarantined_at":    &now,
		"quarantine_reason": trimmedReason,
	}
	if err := repository.UpdateTunnelRelayColumns(ctx, relay, changes); err != nil {
		return nil, err
	}
	relay.AccessToken = newToken
	relay.Status = model.TunnelNodeStatusQuarantined
	relay.QuarantinedAt = &now
	relay.QuarantineReason = trimmedReason

	websocket.DisconnectTunnelRelay(ctx, relay.RelayID, trimmedReason)
	websocket.NotifyTunnelChanged(ctx, 0)
	websocket.NotifyAgentConfigChanged(ctx)

	poolKeys := loadPoolKeyMap(ctx)
	view := buildRelayView(ctx, relay, poolKeys[relay.PoolID], now, resolveRelayOfflineWindow(ctx))
	return &view, nil
}

// UnquarantineRelay clears quarantine state on a relay and issues a fresh access token.
func UnquarantineRelay(ctx context.Context, id uint) (*RelayDeployView, error) {
	relay, err := repository.GetTunnelRelayByID(ctx, id)
	if err != nil {
		return nil, err
	}
	newToken, err := GenerateRandomToken("rly_", 24)
	if err != nil {
		return nil, err
	}
	changes := map[string]any{
		"access_token":      newToken,
		"status":            model.TunnelNodeStatusOffline,
		"quarantined_at":    nil,
		"quarantine_reason": "",
	}
	if err := repository.UpdateTunnelRelayColumns(ctx, relay, changes); err != nil {
		return nil, err
	}
	relay.AccessToken = newToken
	relay.Status = model.TunnelNodeStatusOffline
	relay.QuarantinedAt = nil
	relay.QuarantineReason = ""

	poolKeys := loadPoolKeyMap(ctx)
	view := buildRelayView(ctx, relay, poolKeys[relay.PoolID], time.Now().UTC(), resolveRelayOfflineWindow(ctx))
	return buildRelayDeployView(ctx, relay, view, poolKeys[relay.PoolID]), nil
}

// RequestRelayBinaryUpdate schedules a managed self-update on a relay PoP.
func RequestRelayBinaryUpdate(ctx context.Context, id uint, input BinaryUpdateInput) (*RelayView, error) {
	relay, err := repository.GetTunnelRelayByID(ctx, id)
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
	changes := map[string]any{
		"update_requested": true,
		"update_channel":   channel,
		"update_tag":       tag,
	}
	if err := repository.UpdateTunnelRelayColumns(ctx, relay, changes); err != nil {
		return nil, err
	}
	relay.UpdateRequested = true
	relay.UpdateChannel = channel
	relay.UpdateTag = tag
	websocket.NotifyTunnelRelayChanged(ctx, relay.RelayID)

	poolKeys := loadPoolKeyMap(ctx)
	view := buildRelayView(ctx, relay, poolKeys[relay.PoolID], time.Now().UTC(), resolveRelayOfflineWindow(ctx))
	return &view, nil
}

// GetRelayBootstrapToken returns (initializing if empty) the global relay discovery bootstrap token.
func GetRelayBootstrapToken(ctx context.Context) (*RelayBootstrapView, error) {
	token, err := ensureRelayDiscoveryToken(ctx)
	if err != nil {
		return nil, err
	}
	return buildRelayBootstrapView(ctx, token), nil
}

// RotateRelayBootstrapToken rotates the global relay discovery bootstrap token.
func RotateRelayBootstrapToken(ctx context.Context) (*RelayBootstrapView, error) {
	token, err := GenerateRandomToken("rdisc_", 24)
	if err != nil {
		return nil, err
	}
	if err := repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyRelayDiscoveryToken, token); err != nil {
		return nil, err
	}
	return buildRelayBootstrapView(ctx, token), nil
}

func ensureRelayDiscoveryToken(ctx context.Context) (string, error) {
	sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyRelayDiscoveryToken)
	if err == nil && strings.TrimSpace(sc.Value) != "" {
		return strings.TrimSpace(sc.Value), nil
	}
	token, err := GenerateRandomToken("rdisc_", 24)
	if err != nil {
		return "", err
	}
	if err := repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyRelayDiscoveryToken, token); err != nil {
		return "", err
	}
	return token, nil
}

func buildRelayBootstrapView(ctx context.Context, discoveryToken string) *RelayBootstrapView {
	serverURL := resolveControlPlaneServerURL(ctx)
	wrapped, err := EncodeBootstrapToken(serverURL, bootstrapKindRelay, discoveryToken, "default")
	if err != nil {
		wrapped = discoveryToken
	}
	return &RelayBootstrapView{
		DiscoveryToken: discoveryToken,
		BootstrapToken: wrapped,
		ServerURL:      serverURL,
		DeployCommand:  fmt.Sprintf("openflare-relay --token %s", wrapped),
	}
}

func buildRelayDeployView(ctx context.Context, relay *model.TunnelRelay, view RelayView, poolKey string) *RelayDeployView {
	serverURL := resolveControlPlaneServerURL(ctx)
	wrapped, err := EncodeBootstrapToken(serverURL, bootstrapKindRelay, relay.AccessToken, poolKey)
	if err != nil {
		wrapped = relay.AccessToken
	}
	cmd := fmt.Sprintf("openflare-relay --token %s", wrapped)
	if serverURL == "" {
		cmd = fmt.Sprintf("openflare-relay --server-url <SERVER_URL> --token %s", relay.AccessToken)
	}
	return &RelayDeployView{
		Relay:          view,
		AccessToken:    relay.AccessToken,
		BootstrapToken: wrapped,
		ServerURL:      serverURL,
		DeployCommand:  cmd,
	}
}

func resolveControlPlaneServerURL(ctx context.Context) string {
	sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyServerAddress)
	if err != nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(sc.Value), "/")
}

func validateRelayPorts(transportPort, wsPort, gatewayPort int) (int, int, int, error) {
	if transportPort == 0 {
		transportPort = defaultRelayTransportPort
	}
	if transportPort < 1 || transportPort > 65535 {
		return 0, 0, 0, errors.New(errRelayPortInvalid)
	}
	if wsPort < 0 || wsPort > 65535 {
		return 0, 0, 0, errors.New(errRelayOptionalPortInvalid)
	}
	if gatewayPort == 0 {
		gatewayPort = defaultRelayGatewayPort
	}
	if gatewayPort < 1 || gatewayPort > 65535 {
		return 0, 0, 0, errors.New(errRelayPortInvalid)
	}
	return transportPort, wsPort, gatewayPort, nil
}

func normalizeRelayTransports(input []string) ([]string, error) {
	if len(input) == 0 {
		return []string{model.TunnelTransportNoise, model.TunnelTransportTCP}, nil
	}
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, raw := range input {
		t := strings.ToLower(strings.TrimSpace(raw))
		if t == "" {
			continue
		}
		if !slices.Contains(validTunnelTransports, t) {
			return nil, errors.New(errRelayTransportsInvalid)
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, errors.New(errRelayTransportsInvalid)
	}
	return out, nil
}

func applyRelayGeo(ctx context.Context, relay *model.TunnelRelay, rawAddr string) {
	if relay == nil || relay.GeoManualOverride {
		return
	}
	ip := net.ParseIP(strings.TrimSpace(rawAddr))
	if ip == nil {
		if addrs, err := net.LookupIP(strings.TrimSpace(rawAddr)); err == nil && len(addrs) > 0 {
			ip = addrs[0]
		}
	}
	if ip == nil || !iputil.IsPublic(ip) {
		return
	}
	info, err := ofgeoip.GeoInfoFromIP(ip)
	if err != nil {
		logger.WarnF(ctx, "resolve relay geoip failed: ip=%s err=%v", ip.String(), err)
		return
	}
	if info == nil {
		return
	}
	if name := strings.TrimSpace(info.Name); name != "" {
		relay.GeoName = name
	}
	if info.Latitude != nil && info.Longitude != nil {
		relay.GeoLatitude = cloneCoordinate(info.Latitude)
		relay.GeoLongitude = cloneCoordinate(info.Longitude)
		return
	}
	if lat, lon, ok := pkggeoip.CountryCentroidByISO(info.ISOCode); ok {
		relay.GeoLatitude = cloneFloat(lat)
		relay.GeoLongitude = cloneFloat(lon)
		return
	}
	if lat, lon, ok := pkggeoip.CountryCentroidByName(info.Name); ok {
		relay.GeoLatitude = cloneFloat(lat)
		relay.GeoLongitude = cloneFloat(lon)
	}
}

func cloneFloat(v float64) *float64 {
	c := v
	return &c
}

func cloneCoordinate(v *float64) *float64 {
	if v == nil {
		return nil
	}
	return cloneFloat(*v)
}

func loadPoolKeyMap(ctx context.Context) map[uint]string {
	pools, err := repository.ListTunnelRelayPools(ctx)
	if err != nil {
		return map[uint]string{}
	}
	out := make(map[uint]string, len(pools))
	for _, pool := range pools {
		out[pool.ID] = pool.PoolKey
	}
	return out
}

func resolveRelayOfflineWindow(ctx context.Context) time.Duration {
	ttlSeconds := resolveTunnelLeaseTTLSeconds(ctx)
	hbMs := resolveTunnelHeartbeatMS(ctx)
	window := max(time.Duration(ttlSeconds)*time.Second, time.Duration(hbMs*3)*time.Millisecond)
	if window < 15*time.Second {
		return 15 * time.Second
	}
	return window
}

func buildRelayView(ctx context.Context, relay *model.TunnelRelay, poolKey string, now time.Time, offlineWindow time.Duration) RelayView {
	wsConnected := websocket.IsRelayConnected(relay.RelayID)
	effective := computeRelayEffectiveStatus(relay, wsConnected, now, offlineWindow)
	return RelayView{
		ID:                  relay.ID,
		RelayID:             relay.RelayID,
		PoolID:              relay.PoolID,
		PoolKey:             poolKey,
		Name:                relay.Name,
		PublicAddr:          relay.PublicAddr,
		IPManualOverride:    relay.IPManualOverride,
		TransportPort:       relay.TransportPort,
		WSPort:              relay.WSPort,
		GatewayAddr:         relay.GatewayAddr,
		GatewayPort:         relay.GatewayPort,
		GatewayTLS:          relay.GatewayTLS,
		SupportedTransports: decodeJSONStringSlice(ctx, relay.SupportedTransportsJSON),
		NoisePublicKey:      relay.NoisePublicKey,
		GeoName:             relay.GeoName,
		GeoLatitude:         cloneCoordinate(relay.GeoLatitude),
		GeoLongitude:        cloneCoordinate(relay.GeoLongitude),
		GeoManualOverride:   relay.GeoManualOverride,
		ZoneTag:             relay.ZoneTag,
		ColocatedEdgeNodeID: relay.ColocatedEdgeNodeID,
		Weight:              relay.Weight,
		MaxTunnels:          relay.MaxTunnels,
		Draining:            relay.Draining,
		Status:              relay.Status,
		EffectiveStatus:     effective,
		WSConnected:         wsConnected,
		RuntimeStatus:       relay.RuntimeStatus,
		ActiveConnectors:    relay.ActiveConnectors,
		ActiveStreams:       relay.ActiveStreams,
		CPUUsagePercent:     relay.CPUUsagePercent,
		MemoryUsagePercent:  relay.MemoryUsagePercent,
		Version:             relay.Version,
		EngineVersion:       relay.EngineVersion,
		ContractVersion:     relay.ContractVersion,
		Capabilities:        decodeJSONStringSlice(ctx, relay.CapabilitiesJSON),
		AutoUpdateEnabled:   relay.AutoUpdateEnabled,
		UpdateRequested:     relay.UpdateRequested,
		UpdateChannel:       relay.UpdateChannel,
		UpdateTag:           relay.UpdateTag,
		LastSeenAt:          relay.LastSeenAt,
		LastError:           relay.LastError,
		QuarantinedAt:       relay.QuarantinedAt,
		QuarantineReason:    relay.QuarantineReason,
		Metadata:            decodeJSONObject(ctx, relay.MetadataJSON),
		CreatedAt:           relay.CreatedAt,
		UpdatedAt:           relay.UpdatedAt,
	}
}

func computeRelayEffectiveStatus(relay *model.TunnelRelay, wsConnected bool, now time.Time, offlineWindow time.Duration) string {
	if relay.IsQuarantined() {
		return model.TunnelNodeStatusQuarantined
	}
	if relay.Draining {
		return model.TunnelNodeStatusDraining
	}
	if relay.LastSeenAt == nil || relay.LastSeenAt.IsZero() {
		if wsConnected {
			return model.TunnelNodeStatusOnline
		}
		return model.TunnelNodeStatusPending
	}
	if wsConnected || now.Sub(*relay.LastSeenAt) <= offlineWindow {
		return model.TunnelNodeStatusOnline
	}
	return model.TunnelNodeStatusOffline
}

func appendCapability(ctx context.Context, capabilitiesJSON string, capName string, supported bool) string {
	caps := decodeJSONStringSlice(ctx, capabilitiesJSON)
	has := slices.Contains(caps, capName)
	if has == supported {
		raw, err := json.Marshal(caps)
		if err != nil {
			return "[]"
		}
		return string(raw)
	}
	if supported {
		caps = append(caps, capName)
	} else {
		caps = slices.DeleteFunc(caps, func(item string) bool { return item == capName })
	}
	return encodeJSONStringSlice(caps)
}
