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

// RegistrationResponse is returned after agent registration.
// Server uses access_token; the agent client expects agent_token via RegisterNodeResponse.
type RegistrationResponse struct {
	NodeID      string `json:"node_id"`
	AccessToken string `json:"access_token"`
	Name        string `json:"name"`
}

// ConfigResponse is the full active config payload for agents.
// Server uses time.Time for CreatedAt; the agent client uses string via ActiveConfigResponse.
type ConfigResponse struct {
	Version          string        `json:"version"`
	Checksum         string        `json:"checksum"`
	SourceConfigJSON string        `json:"source_config_json"`
	SupportFiles     []SupportFile `json:"support_files"`
	CreatedAt        time.Time     `json:"created_at"`
}

// HeartbeatResponse is the heartbeat handler result.
type HeartbeatResponse struct {
	Node          *model.OpenFlareNode `json:"node"`
	AgentSettings *Settings            `json:"agent_settings"`
	ActiveConfig  *ActiveConfigMeta    `json:"active_config"`
	WAFIPGroups   []WAFIPGroup         `json:"waf_ip_groups,omitempty"`
}
