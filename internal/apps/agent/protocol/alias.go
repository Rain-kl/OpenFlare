package protocol

import pkgprotocol "github.com/Rain-kl/Wavelet/pkg/protocol"

type APIResponse[T any] = pkgprotocol.APIResponse[T]
type HeartbeatData = pkgprotocol.HeartbeatData
type HeartbeatResult = pkgprotocol.HeartbeatResult
type AgentSettings = pkgprotocol.AgentSettings
type WSMessage = pkgprotocol.WSMessage
type WSOutboundMessage = pkgprotocol.WSOutboundMessage
type WebSocketConnection = pkgprotocol.WebSocketConnection
type NodePayload = pkgprotocol.NodePayload
type NodeSystemProfile = pkgprotocol.NodeSystemProfile
type NodeMetricSnapshot = pkgprotocol.NodeMetricSnapshot
type NodeOpenrestyObservation = pkgprotocol.NodeOpenrestyObservation
type NodeTrafficReport = pkgprotocol.NodeTrafficReport
type NodeAccessLog = pkgprotocol.NodeAccessLog
type BufferedObservabilityRecord = pkgprotocol.BufferedObservabilityRecord
type NodeHealthEvent = pkgprotocol.NodeHealthEvent
type RegisterNodeResponse = pkgprotocol.RegisterNodeResponse
type ApplyLogPayload = pkgprotocol.ApplyLogPayload
type ActiveConfigResponse = pkgprotocol.ActiveConfigResponse
type ActiveConfigMeta = pkgprotocol.ActiveConfigMeta
type WAFIPGroup = pkgprotocol.WAFIPGroup
type WAFIPGroupSyncRequest = pkgprotocol.WAFIPGroupSyncRequest
type WAFIPGroupSyncResponse = pkgprotocol.WAFIPGroupSyncResponse
type SupportFile = pkgprotocol.SupportFile

const (
	WSMessageTypeStatus          = pkgprotocol.WSMessageTypeStatus
	WSMessageTypeSettings        = pkgprotocol.WSMessageTypeSettings
	WSMessageTypeActiveConfig    = pkgprotocol.WSMessageTypeActiveConfig
	WSMessageTypeForceSyncConfig = pkgprotocol.WSMessageTypeForceSyncConfig
	WSMessageTypeWAFIPGroups     = pkgprotocol.WSMessageTypeWAFIPGroups
	WSMessageTypePing            = pkgprotocol.WSMessageTypePing
	WSMessageTypePong            = pkgprotocol.WSMessageTypePong
)

const (
	OpenrestyStatusHealthy   = pkgprotocol.OpenrestyStatusHealthy
	OpenrestyStatusUnhealthy = pkgprotocol.OpenrestyStatusUnhealthy
	OpenrestyStatusUnknown   = pkgprotocol.OpenrestyStatusUnknown
)