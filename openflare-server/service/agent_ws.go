package service

import (
	"encoding/json"
	"log/slog"
)

const (
	AgentWSMessageTypeStatus          = "status"
	AgentWSMessageTypeSettings        = "settings"
	AgentWSMessageTypeActiveConfig    = "active_config"
	AgentWSMessageTypeForceSyncConfig = "force_sync_config"
	AgentWSMessageTypeWAFIPGroups     = "waf_ip_groups"
	AgentWSMessageTypePing            = "ping"
	AgentWSMessageTypePong            = "pong"

	AgentWSConnectedLastSeenValue = "__OPENFLARE_WS_CONNECTED__"
)

type AgentWSInboundMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type AgentWSBroadcastResult struct {
	Version      string   `json:"version"`
	Checksum     string   `json:"checksum"`
	ClientCount  int      `json:"client_count"`
	SuccessCount int      `json:"success_count"`
	FailedNodes  []string `json:"failed_nodes"`
}

var DefaultAgentWSHub = NewWSHub("agent")

func RegisterAgentWSClient(nodeID string) *WSClient {
	return DefaultAgentWSHub.Register(nodeID)
}

func UnregisterAgentWSClient(client *WSClient) {
	DefaultAgentWSHub.Unregister(client)
}

func DisconnectAgentWSClient(nodeID string) {
	DefaultAgentWSHub.Disconnect(nodeID)
}

func IsAgentWSConnected(nodeID string) bool {
	return DefaultAgentWSHub.IsConnected(nodeID)
}

func SendAgentWSSettings(nodeID string, settings *AgentSettings) bool {
	if settings == nil {
		return false
	}
	return DefaultAgentWSHub.SendMessage(nodeID, WSMessage{
		Type:    AgentWSMessageTypeSettings,
		Payload: settings,
	})
}

func SendAgentWSActiveConfig(nodeID string, activeConfig *ActiveConfigMeta) bool {
	if activeConfig == nil {
		return false
	}
	return DefaultAgentWSHub.SendMessage(nodeID, WSMessage{
		Type:    AgentWSMessageTypeActiveConfig,
		Payload: activeConfig,
	})
}

func SendAgentWSForceSyncConfig(nodeID string, activeConfig *ActiveConfigMeta) bool {
	if activeConfig == nil {
		return false
	}
	return DefaultAgentWSHub.SendMessage(nodeID, WSMessage{
		Type:    AgentWSMessageTypeForceSyncConfig,
		Payload: activeConfig,
	})
}

func SendAgentWSWAFIPGroups(nodeID string, groups []AgentWAFIPGroup) bool {
	if len(groups) == 0 {
		return false
	}
	return DefaultAgentWSHub.SendMessage(nodeID, WSMessage{
		Type:    AgentWSMessageTypeWAFIPGroups,
		Payload: groups,
	})
}

func SendAgentWSPong(nodeID string) bool {
	return DefaultAgentWSHub.SendMessage(nodeID, WSMessage{
		Type: AgentWSMessageTypePong,
	})
}

func BroadcastAgentWSActiveConfig(activeConfig *ActiveConfigMeta) AgentWSBroadcastResult {
	if activeConfig == nil {
		slog.Debug("agent ws broadcast skipped because active config is nil")
		return AgentWSBroadcastResult{}
	}

	res := DefaultAgentWSHub.Broadcast(WSMessage{
		Type:    AgentWSMessageTypeActiveConfig,
		Payload: activeConfig,
	})

	result := AgentWSBroadcastResult{
		Version:      activeConfig.Version,
		Checksum:     activeConfig.Checksum,
		ClientCount:  res.ClientCount,
		SuccessCount: res.SuccessCount,
		FailedNodes:  res.FailedIDs,
	}

	slog.Debug("agent ws broadcast active config",
		"version", result.Version,
		"checksum", result.Checksum,
		"client_count", result.ClientCount,
		"success_count", result.SuccessCount,
		"failed_nodes", result.FailedNodes,
	)
	return result
}

func BroadcastAgentWSWAFIPGroups(groups []AgentWAFIPGroup) WSBroadcastResult {
	if len(groups) == 0 {
		return WSBroadcastResult{}
	}
	result := DefaultAgentWSHub.Broadcast(WSMessage{
		Type:    AgentWSMessageTypeWAFIPGroups,
		Payload: groups,
	})
	slog.Debug("agent ws broadcast waf ip groups",
		"group_count", len(groups),
		"client_count", result.ClientCount,
		"success_count", result.SuccessCount,
		"failed_nodes", result.FailedIDs,
	)
	return result
}
