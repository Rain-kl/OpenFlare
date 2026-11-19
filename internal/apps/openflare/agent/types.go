// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
)

const (
	nodeStatusOnline  = "online"
	applyResultOK     = "success"
	applyResultWarn   = "warning"
	applyResultFailed = "failed"
)

// RegistrationResponseDocs describes both token aliases emitted by the custom JSON marshaler.
// It is used only by Swagger; the runtime contract remains protocol.RegisterNodeResponse.
type RegistrationResponseDocs struct {
	NodeID      string `json:"node_id"`
	AgentToken  string `json:"agent_token"`
	AccessToken string `json:"access_token"`
	Name        string `json:"name"`
}

// ConfigResponse is the full active config payload for agents.
// Server uses time.Time for CreatedAt; the agent client uses string via ActiveConfigResponse.
type ConfigResponse struct {
	Version          string                 `json:"version"`
	Checksum         string                 `json:"checksum"`
	SourceConfigJSON string                 `json:"source_config_json"`
	SupportFiles     []protocol.SupportFile `json:"support_files"`
	CreatedAt        time.Time              `json:"created_at"`
}

// PingoraConfigResponse is the dedicated configuration contract for Pingora nodes.
type PingoraConfigResponse struct {
	Version         string    `json:"version"`
	Checksum        string    `json:"checksum"`
	Engine          string    `json:"engine"`
	ReleaseSequence int64     `json:"release_sequence"`
	PayloadJSON     string    `json:"payload_json"`
	PayloadChecksum string    `json:"payload_checksum"`
	CreatedAt       time.Time `json:"created_at"`
}

// HeartbeatResponse is the heartbeat handler result.
type HeartbeatResponse struct {
	Node          *model.OpenFlareNode       `json:"node"`
	AgentSettings *protocol.AgentSettings    `json:"agent_settings"`
	ActiveConfig  *protocol.ActiveConfigMeta `json:"active_config"`
	WAFIPGroups   []protocol.WAFIPGroup      `json:"waf_ip_groups,omitempty"`
}
