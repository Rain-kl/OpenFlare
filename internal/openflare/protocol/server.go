// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

// AgentNodeSystemProfile is an alias for NodeSystemProfile used by server.
type AgentNodeSystemProfile = NodeSystemProfile

// AgentNodeMetricSnapshot is an alias for NodeMetricSnapshot used by server.
type AgentNodeMetricSnapshot = NodeMetricSnapshot

// AgentNodeHealthEvent is an alias for NodeHealthEvent used by server.
type AgentNodeHealthEvent = NodeHealthEvent

// ActiveConfigMeta holds active configuration metadata.
type ActiveConfigMeta struct {
	Version         string `json:"version"`
	Checksum        string `json:"checksum"`
	Engine          string `json:"engine,omitempty"`
	ReleaseSequence int64  `json:"release_sequence,omitempty"`
	PayloadChecksum string `json:"payload_checksum,omitempty"`
}

// ApplyLogPayload is the configuration apply log payload reported by edge nodes.
type ApplyLogPayload struct {
	NodeID              string `json:"node_id"`
	Version             string `json:"version"`
	Result              string `json:"result"`
	Message             string `json:"message"`
	Checksum            string `json:"checksum"`
	MainConfigChecksum  string `json:"main_config_checksum"`
	RouteConfigChecksum string `json:"route_config_checksum"`
	SupportFileCount    int    `json:"support_file_count"`
	Engine              string `json:"engine,omitempty"`
	ReleaseSequence     int64  `json:"release_sequence,omitempty"`
	PayloadChecksum     string `json:"payload_checksum,omitempty"`
}
