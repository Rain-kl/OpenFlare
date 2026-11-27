// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

// Tunnel wire protocol constants and headers shared across control plane, relay, connector, and edge.
const (
	TunnelContractVersion = 1

	HeaderRelayToken          = "X-Relay-Token"           //nolint:gosec // HTTP header name, not a credential
	HeaderRelayBootstrapToken = "X-Relay-Bootstrap-Token" //nolint:gosec // HTTP header name, not a credential
	HeaderTunnelToken         = "X-Tunnel-Token"          //nolint:gosec // HTTP header name, not a credential
	HeaderConnectorID         = "X-Connector-ID"

	HeaderEdgeTunnelID = "X-OpenFlare-Tunnel-ID"
	HeaderEdgeRouteID  = "X-OpenFlare-Route-ID"
	HeaderRelayAuth    = "X-OpenFlare-Relay-Auth"

	RelayHealthPath = "/_openflare/health"

	TunnelWSMessageTypeConfigChanged = "config_changed"
	TunnelWSMessageTypeForceSync     = "force_sync"
	TunnelWSMessageTypeRevoked       = "revoked"
	TunnelWSMessageTypeDrain         = "drain"
)

// TunnelUpdateSettings controls binary self-update policy for openflare-relay and openflared.
type TunnelUpdateSettings struct {
	Repo       string `json:"repo"`
	AutoUpdate bool   `json:"auto_update"`
	Channel    string `json:"channel,omitempty"`
	Tag        string `json:"tag,omitempty"`
}

// RelayRegisterRequest self-registers an openflare-relay PoP using a pool or discovery bootstrap token.
type RelayRegisterRequest struct {
	RelayID             string         `json:"relay_id,omitempty"`
	Name                string         `json:"name"`
	PoolKey             string         `json:"pool_key,omitempty"`
	PublicAddr          string         `json:"public_addr"`
	TransportPort       int            `json:"transport_port"`
	WSPort              int            `json:"ws_port,omitempty"`
	GatewayAddr         string         `json:"gateway_addr,omitempty"`
	GatewayPort         int            `json:"gateway_port"`
	GatewayTLS          bool           `json:"gateway_tls"`
	SupportedTransports []string       `json:"supported_transports,omitempty"`
	NoisePublicKey      string         `json:"noise_public_key,omitempty"`
	GeoName             string         `json:"geo_name,omitempty"`
	GeoLatitude         *float64       `json:"geo_latitude,omitempty"`
	GeoLongitude        *float64       `json:"geo_longitude,omitempty"`
	ZoneTag             string         `json:"zone_tag,omitempty"`
	ColocatedEdgeNodeID string         `json:"colocated_edge_node_id,omitempty"`
	Weight              int            `json:"weight,omitempty"`
	Version             string         `json:"version,omitempty"`
	EngineVersion       string         `json:"engine_version,omitempty"`
	ContractVersion     int            `json:"contract_version,omitempty"`
	Capabilities        []string       `json:"capabilities,omitempty"`
	Metadata            map[string]any `json:"metadata,omitempty"`
}

// RelayRegisterResponse returns credentials and server-assigned identity after relay bootstrap registration.
type RelayRegisterResponse struct {
	ID              uint   `json:"id"`
	RelayID         string `json:"relay_id"`
	PoolID          uint   `json:"pool_id"`
	AccessToken     string `json:"access_token"`
	PublicAddr      string `json:"public_addr"`
	GatewaySecret   string `json:"gateway_secret"`
	NoisePrivateKey string `json:"noise_private_key,omitempty"`
	NoisePublicKey  string `json:"noise_public_key"`
}

// RelayVerifiedAttachment reports a relay-verified live connector session inside Rathole.
type RelayVerifiedAttachment struct {
	ConnectorID   string `json:"connector_id"`
	ServiceName   string `json:"service_name"`
	RTTMs         int    `json:"rtt_ms"`
	ActiveStreams int    `json:"active_streams"`
}

// RelayHeartbeatPayload carries runtime status and active verified connector sessions from openflare-relay.
type RelayHeartbeatPayload struct {
	Version             string                    `json:"version"`
	EngineVersion       string                    `json:"engine_version,omitempty"`
	ContractVersion     int                       `json:"contract_version,omitempty"`
	Capabilities        []string                  `json:"capabilities,omitempty"`
	PublicAddr          string                    `json:"public_addr,omitempty"`
	GatewayAddr         string                    `json:"gateway_addr,omitempty"`
	TransportPort       int                       `json:"transport_port,omitempty"`
	WSPort              int                       `json:"ws_port,omitempty"`
	GatewayPort         int                       `json:"gateway_port,omitempty"`
	GatewayTLS          *bool                     `json:"gateway_tls,omitempty"`
	SupportedTransports []string                  `json:"supported_transports,omitempty"`
	RuntimeStatus       string                    `json:"runtime_status,omitempty"`
	Draining            bool                      `json:"draining,omitempty"`
	ActiveConnectors    int                       `json:"active_connectors"`
	ActiveStreams       int                       `json:"active_streams"`
	CPUUsagePercent     float64                   `json:"cpu_usage_percent"`
	MemoryUsagePercent  float64                   `json:"memory_usage_percent"`
	LastError           string                    `json:"last_error,omitempty"`
	AppliedUpdateTag    string                    `json:"applied_update_tag,omitempty"`
	VerifiedAttachments []RelayVerifiedAttachment `json:"verified_attachments,omitempty"`
}

// RelayHeartbeatResponse acknowledges relay heartbeats and returns scheduling/quarantine/update directives.
type RelayHeartbeatResponse struct {
	RelayID           string                `json:"relay_id"`
	PoolID            uint                  `json:"pool_id"`
	HeartbeatInterval int                   `json:"heartbeat_interval"`
	LeaseTTLSeconds   int                   `json:"lease_ttl_seconds"`
	Draining          bool                  `json:"draining"`
	Quarantined       bool                  `json:"quarantined"`
	QuarantineReason  string                `json:"quarantine_reason,omitempty"`
	ForceSync         bool                  `json:"force_sync"`
	UpdateSettings    *TunnelUpdateSettings `json:"update_settings,omitempty"`
}

// RelayServiceSpec defines one authorized Rathole server service and its L7 gateway or L4 forwarding role.
type RelayServiceSpec struct {
	ServiceName    string `json:"service_name"`
	ServiceType    string `json:"service_type"`
	TunnelID       uint   `json:"tunnel_id"`
	TunnelPublicID string `json:"tunnel_public_id"`
	ConnectorID    string `json:"connector_id,omitempty"`
	Token          string `json:"token"`
	Transport      string `json:"transport"`
	ListenPort     int    `json:"listen_port,omitempty"`
	State          string `json:"state"`
}

// RelayRuntimeConfigResponse returns full Rathole server & HTTP/WS gateway configuration for openflare-relay.
type RelayRuntimeConfigResponse struct {
	ID                  uint               `json:"id"`
	RelayID             string             `json:"relay_id"`
	PoolID              uint               `json:"pool_id"`
	Name                string             `json:"name"`
	PublicAddr          string             `json:"public_addr"`
	TransportPort       int                `json:"transport_port"`
	WSPort              int                `json:"ws_port"`
	GatewayAddr         string             `json:"gateway_addr"`
	GatewayPort         int                `json:"gateway_port"`
	GatewayTLS          bool               `json:"gateway_tls"`
	GatewaySecret       string             `json:"gateway_secret"`
	SupportedTransports []string           `json:"supported_transports"`
	NoisePrivateKey     string             `json:"noise_private_key"`
	NoisePublicKey      string             `json:"noise_public_key"`
	Draining            bool               `json:"draining"`
	LeaseTTLSeconds     int                `json:"lease_ttl_seconds"`
	UpdatedAtUnix       int64              `json:"updated_at_unix"`
	Services            []RelayServiceSpec `json:"services"`
}

// TunnelConnectedRelayReport reports one relay attachment from an openflared connector replica.
type TunnelConnectedRelayReport struct {
	RelayID       uint   `json:"relay_id"`
	RelayPublicID string `json:"relay_public_id,omitempty"`
	ServiceName   string `json:"service_name"`
	Transport     string `json:"transport"`
	State         string `json:"state"`
	RTTMs         int    `json:"rtt_ms"`
	ActiveStreams int    `json:"active_streams"`
}

// TunnelConnectorHeartbeatPayload reports openflared replica health and connected relay states.
type TunnelConnectorHeartbeatPayload struct {
	ConnectorID      string                       `json:"connector_id"`
	Hostname         string                       `json:"hostname,omitempty"`
	OSArch           string                       `json:"os_arch,omitempty"`
	PublicIP         string                       `json:"public_ip,omitempty"`
	Version          string                       `json:"version"`
	EngineVersion    string                       `json:"engine_version,omitempty"`
	ContractVersion  int                          `json:"contract_version,omitempty"`
	Capabilities     []string                     `json:"capabilities,omitempty"`
	AppliedRevision  int64                        `json:"applied_revision"`
	Draining         bool                         `json:"draining,omitempty"`
	LastError        string                       `json:"last_error,omitempty"`
	AppliedUpdateTag string                       `json:"applied_update_tag,omitempty"`
	ConnectedRelays  []TunnelConnectedRelayReport `json:"connected_relays,omitempty"`
}

// TunnelConnectorHeartbeatResponse directs openflared sync, quarantine, revocation, and self-updates.
type TunnelConnectorHeartbeatResponse struct {
	TunnelID          uint                  `json:"tunnel_id"`
	TunnelPublicID    string                `json:"tunnel_public_id"`
	ConnectorID       string                `json:"connector_id"`
	ConfigRevision    int64                 `json:"config_revision"`
	HeartbeatInterval int                   `json:"heartbeat_interval"`
	ForceSync         bool                  `json:"force_sync"`
	Disabled          bool                  `json:"disabled"`
	Quarantined       bool                  `json:"quarantined"`
	QuarantineReason  string                `json:"quarantine_reason,omitempty"`
	Revoked           bool                  `json:"revoked"`
	UpdateSettings    *TunnelUpdateSettings `json:"update_settings,omitempty"`
}

// TunnelRelayCandidate describes a relay PoP eligible for connector RTT probing and attachment.
type TunnelRelayCandidate struct {
	ID                  uint     `json:"id"`
	RelayID             string   `json:"relay_id"`
	PoolID              uint     `json:"pool_id"`
	Name                string   `json:"name"`
	PublicAddr          string   `json:"public_addr"`
	TransportPort       int      `json:"transport_port"`
	WSPort              int      `json:"ws_port"`
	GatewayAddr         string   `json:"gateway_addr"`
	GatewayPort         int      `json:"gateway_port"`
	GatewayTLS          bool     `json:"gateway_tls"`
	HealthPath          string   `json:"health_path"`
	SupportedTransports []string `json:"supported_transports"`
	NoisePublicKey      string   `json:"noise_public_key"`
	GeoName             string   `json:"geo_name,omitempty"`
	GeoLatitude         *float64 `json:"geo_latitude,omitempty"`
	GeoLongitude        *float64 `json:"geo_longitude,omitempty"`
	ZoneTag             string   `json:"zone_tag,omitempty"`
	Weight              int      `json:"weight"`
	Draining            bool     `json:"draining"`
}

// TunnelConnectorAttachmentGrant carries a control-plane lease token for connecting to a specific relay.
type TunnelConnectorAttachmentGrant struct {
	RelayID            uint   `json:"relay_id"`
	RelayPublicID      string `json:"relay_public_id,omitempty"`
	ServiceName        string `json:"service_name"`
	SessionToken       string `json:"session_token"`
	Transport          string `json:"transport"`
	State              string `json:"state"`
	LeaseExpiresAtUnix int64  `json:"lease_expires_at_unix"`
}

// TunnelHTTPRouteRule maps a public L7 route onto a private origin target reached via openflared.
type TunnelHTTPRouteRule struct {
	RouteID        uint     `json:"route_id"`
	SiteName       string   `json:"site_name,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	TargetAddr     string   `json:"target_addr"`
	TargetProtocol string   `json:"target_protocol"`
	OriginHost     string   `json:"origin_host,omitempty"`
	OptionsJSON    string   `json:"options_json,omitempty"`
}

// TunnelL4ServiceRule maps a non-HTTP TCP/UDP port on a relay to a private target via openflared.
type TunnelL4ServiceRule struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	ServiceType string `json:"service_type"`
	RelayID     uint   `json:"relay_id"`
	ListenPort  int    `json:"listen_port"`
	TargetAddr  string `json:"target_addr"`
	OptionsJSON string `json:"options_json,omitempty"`
}

// TunnelConnectorConfigResponse returns candidate relays, active attachment grants, CIDR policies, and routes.
type TunnelConnectorConfigResponse struct {
	TunnelID           uint                             `json:"tunnel_id"`
	TunnelPublicID     string                           `json:"tunnel_public_id"`
	Name               string                           `json:"name"`
	ConfigRevision     int64                            `json:"config_revision"`
	Enabled            bool                             `json:"enabled"`
	PoolID             uint                             `json:"pool_id"`
	HARelayCount       int                              `json:"ha_relay_count"`
	PreferredTransport string                           `json:"preferred_transport"`
	FallbackTransports []string                         `json:"fallback_transports"`
	ClientProxyURL     string                           `json:"client_proxy_url,omitempty"`
	AllowedTargetCIDRs []string                         `json:"allowed_target_cidrs,omitempty"`
	BlockedTargetCIDRs []string                         `json:"blocked_target_cidrs,omitempty"`
	CandidateRelays    []TunnelRelayCandidate           `json:"candidate_relays"`
	Attachments        []TunnelConnectorAttachmentGrant `json:"attachments"`
	Routes             []TunnelHTTPRouteRule            `json:"routes"`
	Services           []TunnelL4ServiceRule            `json:"services,omitempty"`
}

// TunnelDesiredAttachment specifies one relay selected by openflared after local probing.
type TunnelDesiredAttachment struct {
	RelayID   uint   `json:"relay_id"`
	Transport string `json:"transport"`
	RTTMs     int    `json:"rtt_ms"`
}

// TunnelNegotiateAttachmentsRequest submits a connector's selected closest-K relays to receive session grants.
type TunnelNegotiateAttachmentsRequest struct {
	ConnectorID     string                    `json:"connector_id"`
	Hostname        string                    `json:"hostname,omitempty"`
	OSArch          string                    `json:"os_arch,omitempty"`
	PublicIP        string                    `json:"public_ip,omitempty"`
	Version         string                    `json:"version,omitempty"`
	EngineVersion   string                    `json:"engine_version,omitempty"`
	ContractVersion int                       `json:"contract_version,omitempty"`
	Capabilities    []string                  `json:"capabilities,omitempty"`
	Desired         []TunnelDesiredAttachment `json:"desired"`
}

// TunnelNegotiateAttachmentsResponse returns issued Rathole service credentials for requested relays.
type TunnelNegotiateAttachmentsResponse struct {
	TunnelID        uint                             `json:"tunnel_id"`
	ConfigRevision  int64                            `json:"config_revision"`
	LeaseTTLSeconds int                              `json:"lease_ttl_seconds"`
	Attachments     []TunnelConnectorAttachmentGrant `json:"attachments"`
}

// TunnelEdgeRelayEndpoint describes one healthy relay gateway PoP for an active tunnel in tunnel-routing.v1.
type TunnelEdgeRelayEndpoint struct {
	RelayID             uint   `json:"relay_id"`
	RelayPublicID       string `json:"relay_public_id"`
	GatewayURL          string `json:"gateway_url"`
	GatewaySecret       string `json:"gateway_secret"`
	ZoneTag             string `json:"zone_tag,omitempty"`
	ColocatedEdgeNodeID string `json:"colocated_edge_node_id,omitempty"`
	IsColocated         bool   `json:"is_colocated"`
	Weight              int    `json:"weight"`
	BestRTTMs           int    `json:"best_rtt_ms"`
	ActiveConnectors    int    `json:"active_connectors"`
	ActiveStreams       int    `json:"active_streams"`
}

// TunnelEdgeRoutingEntry maps one logical tunnel to its healthy relay endpoints in priority order.
type TunnelEdgeRoutingEntry struct {
	TunnelID           uint                      `json:"tunnel_id"`
	TunnelPublicID     string                    `json:"tunnel_public_id"`
	Enabled            bool                      `json:"enabled"`
	Quarantined        bool                      `json:"quarantined"`
	PreferredTransport string                    `json:"preferred_transport,omitempty"`
	Relays             []TunnelEdgeRelayEndpoint `json:"relays"`
}

// TunnelRoutingSnapshot is the lightweight control-plane -> edge tunnel-routing.v1 resource payload.
type TunnelRoutingSnapshot struct {
	SchemaVersion   int                      `json:"schema_version"`
	GeneratedAtUnix int64                    `json:"generated_at_unix"`
	Tunnels         []TunnelEdgeRoutingEntry `json:"tunnels"`
}
