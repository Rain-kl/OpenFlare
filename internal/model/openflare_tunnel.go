// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"slices"
	"time"
)

const (
	// TunnelUpdateCapability identifies tunnel relays and connectors that support managed binary updates.
	TunnelUpdateCapability = "tunnel_self_update_v1"

	// TunnelTransportAuto selects the best reachable transport on the connector.
	TunnelTransportAuto = "auto"
	// TunnelTransportNoise uses Rathole Noise_NK_25519_ChaChaPoly_BLAKE2s transport.
	TunnelTransportNoise = "noise"
	// TunnelTransportWebSocket uses Rathole WebSocket over TLS transport.
	TunnelTransportWebSocket = "websocket"
	// TunnelTransportTLS uses Rathole TLS transport.
	TunnelTransportTLS = "tls"
	// TunnelTransportTCP uses Rathole raw TCP transport.
	TunnelTransportTCP = "tcp"

	// TunnelNodeStatusPending indicates a relay has been provisioned but has never checked in.
	TunnelNodeStatusPending = "pending"
	// TunnelNodeStatusOnline indicates an active relay or connector within heartbeat threshold.
	TunnelNodeStatusOnline = "online"
	// TunnelNodeStatusOffline indicates an expired heartbeat threshold.
	TunnelNodeStatusOffline = "offline"
	// TunnelNodeStatusDraining indicates graceful drain before maintenance or shutdown.
	TunnelNodeStatusDraining = "draining"
	// TunnelNodeStatusQuarantined indicates revoked credentials after emergency isolation.
	TunnelNodeStatusQuarantined = "quarantined"

	// TunnelAttachmentStateConnecting indicates control-plane grant issued before relay verification.
	TunnelAttachmentStateConnecting = "connecting"
	// TunnelAttachmentStateActive indicates relay-verified active Rathole session.
	TunnelAttachmentStateActive = "active"
	// TunnelAttachmentStateDraining indicates graceful handoff or drain in progress.
	TunnelAttachmentStateDraining = "draining"
	// TunnelAttachmentStateStale indicates missed relay lease renewal.
	TunnelAttachmentStateStale = "stale"
)

// TunnelRelayPool groups public or private tunnel relays for geographic, compliance, or tenant affinity.
type TunnelRelayPool struct {
	ID                uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	PoolKey           string    `json:"pool_key" gorm:"uniqueIndex;size:64;not null"`
	Name              string    `json:"name" gorm:"size:128;not null"`
	Description       string    `json:"description" gorm:"size:512;not null;default:''"`
	Region            string    `json:"region" gorm:"size:64;not null;default:''"`
	IsDefault         bool      `json:"is_default" gorm:"not null;default:false"`
	RoutingPolicyJSON string    `json:"routing_policy_json" gorm:"type:text;not null;default:'{}'"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (TunnelRelayPool) TableName() string {
	return "of_tunnel_relay_pools"
}

// TunnelRelay stores a public or private Rathole relay PoP.
type TunnelRelay struct {
	ID                      uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	RelayID                 string     `json:"relay_id" gorm:"uniqueIndex;size:64;not null"`
	PoolID                  uint       `json:"pool_id" gorm:"index;not null;default:0"`
	Name                    string     `json:"name" gorm:"size:128;not null"`
	AccessToken             string     `json:"-" gorm:"index;size:128;not null;default:''"`
	PublicAddr              string     `json:"public_addr" gorm:"size:255;not null;default:''"`
	IPManualOverride        bool       `json:"ip_manual_override" gorm:"not null;default:false"`
	TransportPort           int        `json:"transport_port" gorm:"not null;default:7443"`
	WSPort                  int        `json:"ws_port" gorm:"not null;default:0"`
	GatewayAddr             string     `json:"gateway_addr" gorm:"size:255;not null;default:''"`
	GatewayPort             int        `json:"gateway_port" gorm:"not null;default:8443"`
	GatewayTLS              bool       `json:"gateway_tls" gorm:"not null;default:false"`
	GatewaySecret           string     `json:"-" gorm:"size:128;not null;default:''"`
	SupportedTransportsJSON string     `json:"supported_transports_json" gorm:"type:text;not null;default:'[\"noise\",\"tcp\"]'"`
	NoisePrivateKey         string     `json:"-" gorm:"size:128;not null;default:''"`
	NoisePublicKey          string     `json:"noise_public_key" gorm:"size:128;not null;default:''"`
	GeoName                 string     `json:"geo_name" gorm:"size:128;not null;default:''"`
	GeoLatitude             *float64   `json:"geo_latitude"`
	GeoLongitude            *float64   `json:"geo_longitude"`
	GeoManualOverride       bool       `json:"geo_manual_override" gorm:"not null;default:false"`
	ZoneTag                 string     `json:"zone_tag" gorm:"size:64;not null;default:''"`
	ColocatedEdgeNodeID     string     `json:"colocated_edge_node_id" gorm:"index;size:64;not null;default:''"`
	Weight                  int        `json:"weight" gorm:"not null;default:100"`
	MaxTunnels              int        `json:"max_tunnels" gorm:"not null;default:0"`
	Draining                bool       `json:"draining" gorm:"not null;default:false"`
	Status                  string     `json:"status" gorm:"index;size:16;not null;default:'pending'"`
	RuntimeStatus           string     `json:"runtime_status" gorm:"size:16;not null;default:'unknown'"`
	ActiveConnectors        int        `json:"active_connectors" gorm:"not null;default:0"`
	ActiveStreams           int        `json:"active_streams" gorm:"not null;default:0"`
	CPUUsagePercent         float64    `json:"cpu_usage_percent" gorm:"not null;default:0"`
	MemoryUsagePercent      float64    `json:"memory_usage_percent" gorm:"not null;default:0"`
	Version                 string     `json:"version" gorm:"size:64;not null;default:''"`
	EngineVersion           string     `json:"engine_version" gorm:"size:64;not null;default:''"`
	ContractVersion         int        `json:"contract_version" gorm:"not null;default:0"`
	CapabilitiesJSON        string     `json:"capabilities_json" gorm:"type:text;not null;default:'[]'"`
	AutoUpdateEnabled       bool       `json:"auto_update_enabled" gorm:"not null;default:false"`
	UpdateRequested         bool       `json:"update_requested" gorm:"not null;default:false"`
	UpdateChannel           string     `json:"update_channel" gorm:"size:16;not null;default:'stable'"`
	UpdateTag               string     `json:"update_tag" gorm:"size:64;not null;default:''"`
	LastSeenAt              *time.Time `json:"last_seen_at" gorm:"index"`
	LastError               string     `json:"last_error" gorm:"type:text"`
	QuarantinedAt           *time.Time `json:"quarantined_at" gorm:"index"`
	QuarantineReason        string     `json:"quarantine_reason" gorm:"size:255;not null;default:''"`
	MetadataJSON            string     `json:"metadata_json" gorm:"type:text;not null;default:'{}'"`
	CreatedAt               time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt               time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (TunnelRelay) TableName() string {
	return "of_tunnel_relays"
}

// IsQuarantined reports whether the relay is isolated.
func (r *TunnelRelay) IsQuarantined() bool {
	return r != nil && r.QuarantinedAt != nil && !r.QuarantinedAt.IsZero()
}

// HasCapability reports whether the relay advertised a runtime capability.
func (r *TunnelRelay) HasCapability(name string) bool {
	if r == nil {
		return false
	}
	var capabilities []string
	return json.Unmarshal([]byte(r.CapabilitiesJSON), &capabilities) == nil && slices.Contains(capabilities, name)
}

// Tunnel represents a logical private-network tunnel independent of physical connector replicas.
type Tunnel struct {
	ID                     uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	TunnelID               string     `json:"tunnel_id" gorm:"uniqueIndex;size:64;not null"`
	Name                   string     `json:"name" gorm:"size:128;not null"`
	Description            string     `json:"description" gorm:"size:512;not null;default:''"`
	PoolID                 uint       `json:"pool_id" gorm:"index;not null;default:0"`
	AccessToken            string     `json:"-" gorm:"index;size:128;not null;default:''"`
	HARelayCount           int        `json:"ha_relay_count" gorm:"column:ha_relay_count;not null;default:2"`
	PreferredTransport     string     `json:"preferred_transport" gorm:"size:16;not null;default:'auto'"`
	FallbackTransportsJSON string     `json:"fallback_transports_json" gorm:"type:text;not null;default:'[\"noise\",\"websocket\",\"tcp\"]'"`
	ClientProxyURL         string     `json:"client_proxy_url" gorm:"size:512;not null;default:''"`
	AllowedTargetCIDRsJSON string     `json:"allowed_target_cidrs_json" gorm:"type:text;not null;default:'[]'"`
	BlockedTargetCIDRsJSON string     `json:"blocked_target_cidrs_json" gorm:"type:text;not null;default:'[\"169.254.169.254/32\"]'"`
	MaxConnectors          int        `json:"max_connectors" gorm:"not null;default:0"`
	Enabled                bool       `json:"enabled" gorm:"not null;default:true"`
	ConfigRevision         int64      `json:"config_revision" gorm:"not null;default:1"`
	AutoUpdateEnabled      bool       `json:"auto_update_enabled" gorm:"not null;default:false"`
	UpdateRequested        bool       `json:"update_requested" gorm:"not null;default:false"`
	UpdateChannel          string     `json:"update_channel" gorm:"size:16;not null;default:'stable'"`
	UpdateTag              string     `json:"update_tag" gorm:"size:64;not null;default:''"`
	QuarantinedAt          *time.Time `json:"quarantined_at" gorm:"index"`
	QuarantineReason       string     `json:"quarantine_reason" gorm:"size:255;not null;default:''"`
	MetadataJSON           string     `json:"metadata_json" gorm:"type:text;not null;default:'{}'"`
	CreatedAt              time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (Tunnel) TableName() string {
	return "of_tunnels"
}

// IsQuarantined reports whether the tunnel is isolated.
func (t *Tunnel) IsQuarantined() bool {
	return t != nil && t.QuarantinedAt != nil && !t.QuarantinedAt.IsZero()
}

// TunnelConnector stores one live or previously seen openflared replica connected to a logical tunnel.
type TunnelConnector struct {
	ID                  uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	ConnectorID         string     `json:"connector_id" gorm:"uniqueIndex;size:64;not null"`
	TunnelID            uint       `json:"tunnel_id" gorm:"index;not null"`
	Hostname            string     `json:"hostname" gorm:"size:255;not null;default:''"`
	OSArch              string     `json:"os_arch" gorm:"size:64;not null;default:''"`
	PublicIP            string     `json:"public_ip" gorm:"size:64;not null;default:''"`
	Version             string     `json:"version" gorm:"size:64;not null;default:''"`
	EngineVersion       string     `json:"engine_version" gorm:"size:64;not null;default:''"`
	ContractVersion     int        `json:"contract_version" gorm:"not null;default:0"`
	CapabilitiesJSON    string     `json:"capabilities_json" gorm:"type:text;not null;default:'[]'"`
	AppliedRevision     int64      `json:"applied_revision" gorm:"not null;default:0"`
	Status              string     `json:"status" gorm:"index;size:16;not null;default:'online'"`
	LastError           string     `json:"last_error" gorm:"type:text"`
	ConnectedRelaysJSON string     `json:"connected_relays_json" gorm:"type:text;not null;default:'[]'"`
	Draining            bool       `json:"draining" gorm:"not null;default:false"`
	RevokedAt           *time.Time `json:"revoked_at"`
	LastSeenAt          *time.Time `json:"last_seen_at" gorm:"index"`
	CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (TunnelConnector) TableName() string {
	return "of_tunnel_connectors"
}

// IsRevoked reports whether this connector instance has been revoked.
func (c *TunnelConnector) IsRevoked() bool {
	return c != nil && c.RevokedAt != nil && !c.RevokedAt.IsZero()
}

// HasCapability reports whether the connector advertised a runtime capability.
func (c *TunnelConnector) HasCapability(name string) bool {
	if c == nil {
		return false
	}
	var capabilities []string
	return json.Unmarshal([]byte(c.CapabilitiesJSON), &capabilities) == nil && slices.Contains(capabilities, name)
}

// TunnelAttachment stores the lease-verified dynamic link between a connector replica and a relay PoP.
type TunnelAttachment struct {
	ID              uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	TunnelID        uint       `json:"tunnel_id" gorm:"index;not null"`
	ConnectorID     string     `json:"connector_id" gorm:"index;uniqueIndex:idx_of_tunnel_attachments_connector_relay;size:64;not null"`
	RelayID         uint       `json:"relay_id" gorm:"index;uniqueIndex:idx_of_tunnel_attachments_connector_relay;not null"`
	ServiceName     string     `json:"service_name" gorm:"size:128;not null;default:''"`
	SessionToken    string     `json:"-" gorm:"size:128;not null;default:''"`
	Transport       string     `json:"transport" gorm:"size:16;not null;default:'noise'"`
	State           string     `json:"state" gorm:"index;size:16;not null;default:'connecting'"`
	RTTMs           int        `json:"rtt_ms" gorm:"not null;default:0"`
	ActiveStreams   int        `json:"active_streams" gorm:"not null;default:0"`
	RelayVerifiedAt *time.Time `json:"relay_verified_at"`
	LeaseExpiresAt  *time.Time `json:"lease_expires_at" gorm:"index"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (TunnelAttachment) TableName() string {
	return "of_tunnel_attachments"
}

// TunnelService stores a non-HTTP L4 TCP/UDP (or future L3/CIDR) service exposed through a tunnel.
type TunnelService struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TunnelID    uint      `json:"tunnel_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"size:128;not null"`
	ServiceType string    `json:"service_type" gorm:"size:16;not null;default:'tcp'"`
	RelayID     uint      `json:"relay_id" gorm:"index;not null;default:0"`
	ListenPort  int       `json:"listen_port" gorm:"not null;default:0"`
	TargetAddr  string    `json:"target_addr" gorm:"size:512;not null;default:''"`
	Enabled     bool      `json:"enabled" gorm:"not null;default:true"`
	OptionsJSON string    `json:"options_json" gorm:"type:text;not null;default:'{}'"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (TunnelService) TableName() string {
	return "of_tunnel_services"
}
