// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	nodeStatusOnline  = "online"
	applyResultOK     = "success"
	applyResultWarn   = "warning"
	applyResultFailed = "failed"
)

// NodePayload is the agent register/heartbeat payload.
type NodePayload struct {
	NodeID                string                        `json:"node_id"`
	Name                  string                        `json:"name"`
	IP                    string                        `json:"ip"`
	Version               string                        `json:"version"`
	ExtVersion            string                        `json:"ext_version"`
	CurrentVersion        string                        `json:"current_version"`
	LastError             string                        `json:"last_error"`
	OpenrestyStatus       string                        `json:"openresty_status"`
	OpenrestyMessage      string                        `json:"openresty_message"`
	Profile               *NodeSystemProfile            `json:"profile,omitempty"`
	Snapshot              *NodeMetricSnapshot           `json:"snapshot,omitempty"`
	OpenrestyObservation  *NodeOpenrestyObservation     `json:"openresty_observation,omitempty"`
	TrafficReport         *NodeTrafficReport            `json:"traffic_report,omitempty"`
	AccessLogs            []NodeAccessLog               `json:"access_logs,omitempty"`
	BufferedObservability []BufferedObservabilityRecord `json:"buffered_observability,omitempty"`
	HealthEvents          []NodeHealthEvent             `json:"health_events"`
	WAFIPGroupChecksums   map[string]string             `json:"waf_ip_group_checksums,omitempty"`
}

// ApplyLogPayload is the agent apply log report payload.
type ApplyLogPayload struct {
	NodeID              string `json:"node_id"`
	Version             string `json:"version"`
	Result              string `json:"result"`
	Message             string `json:"message"`
	Checksum            string `json:"checksum"`
	MainConfigChecksum  string `json:"main_config_checksum"`
	RouteConfigChecksum string `json:"route_config_checksum"`
	SupportFileCount    int    `json:"support_file_count"`
}

// RegistrationResponse is returned after agent registration.
type RegistrationResponse struct {
	NodeID      string `json:"node_id"`
	AccessToken string `json:"access_token"`
	Name        string `json:"name"`
}

// Settings carries remote agent control flags.
type Settings struct {
	HeartbeatInterval       int    `json:"heartbeat_interval"`
	WebsocketUpgradeEnabled bool   `json:"websocket_upgrade_enabled"`
	AutoUpdate              bool   `json:"auto_update"`
	UpdateRepo              string `json:"update_repo"`
	UpdateNow               bool   `json:"update_now"`
	UpdateChannel           string `json:"update_channel"`
	UpdateTag               string `json:"update_tag"`
	RestartOpenrestyNow     bool   `json:"restart_openresty_now"`
}

// ActiveConfigMeta summarizes the active configuration version.
type ActiveConfigMeta struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

// SupportFile is a configuration support artifact shipped to agents.
type SupportFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ConfigResponse is the full active config payload for agents.
type ConfigResponse struct {
	Version          string        `json:"version"`
	Checksum         string        `json:"checksum"`
	SourceConfigJSON string        `json:"source_config_json"`
	SupportFiles     []SupportFile `json:"support_files"`
	CreatedAt        time.Time     `json:"created_at"`
}

// WAFIPGroup is a WAF IP group snapshot for agents.
type WAFIPGroup struct {
	ID       uint     `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Enabled  bool     `json:"enabled"`
	IPList   []string `json:"ip_list"`
	Checksum string   `json:"checksum"`
}

// WAFIPGroupSyncInput requests changed WAF IP groups.
type WAFIPGroupSyncInput struct {
	IDs       []uint            `json:"ids"`
	Checksums map[string]string `json:"checksums"`
}

// WAFIPGroupSyncResult returns synced WAF IP groups.
type WAFIPGroupSyncResult struct {
	Groups []WAFIPGroup `json:"groups"`
}

// HeartbeatResponse is the heartbeat handler result.
type HeartbeatResponse struct {
	Node          *model.OpenFlareNode `json:"node"`
	AgentSettings *Settings            `json:"agent_settings"`
	ActiveConfig  *ActiveConfigMeta    `json:"active_config"`
	WAFIPGroups   []WAFIPGroup         `json:"waf_ip_groups,omitempty"`
}
