// Package protocol defines type aliases and constants for the agent protocol.
package protocol

import pkgprotocol "github.com/Rain-kl/Wavelet/pkg/protocol"

// APIResponse is an alias for pkgprotocol.APIResponse.
type APIResponse[T any] = pkgprotocol.APIResponse[T]

// HeartbeatData is an alias for pkgprotocol.HeartbeatData.
type HeartbeatData = pkgprotocol.HeartbeatData

// HeartbeatResult is an alias for pkgprotocol.HeartbeatResult.
type HeartbeatResult = pkgprotocol.HeartbeatResult

// AgentSettings is an alias for pkgprotocol.AgentSettings.
type AgentSettings = pkgprotocol.AgentSettings

// WSMessage is an alias for pkgprotocol.WSMessage.
type WSMessage = pkgprotocol.WSMessage

// WSOutboundMessage is an alias for pkgprotocol.WSOutboundMessage.
type WSOutboundMessage = pkgprotocol.WSOutboundMessage

// WebSocketConnection is an alias for pkgprotocol.WebSocketConnection.
type WebSocketConnection = pkgprotocol.WebSocketConnection

// NodePayload is an alias for pkgprotocol.NodePayload.
type NodePayload = pkgprotocol.NodePayload

// NodeSystemProfile is an alias for pkgprotocol.NodeSystemProfile.
type NodeSystemProfile = pkgprotocol.NodeSystemProfile

// NodeMetricSnapshot is an alias for pkgprotocol.NodeMetricSnapshot.
type NodeMetricSnapshot = pkgprotocol.NodeMetricSnapshot

// NodeEdgeHealth is an alias for pkgprotocol.NodeEdgeHealth.
type NodeEdgeHealth = pkgprotocol.NodeEdgeHealth

// NodeAccessLog is an alias for pkgprotocol.NodeAccessLog.
type NodeAccessLog = pkgprotocol.NodeAccessLog

// BufferedObservabilityRecord is an alias for pkgprotocol.BufferedObservabilityRecord.
type BufferedObservabilityRecord = pkgprotocol.BufferedObservabilityRecord

// NodeHealthEvent is an alias for pkgprotocol.NodeHealthEvent.
type NodeHealthEvent = pkgprotocol.NodeHealthEvent

// RegisterNodeResponse is an alias for pkgprotocol.RegisterNodeResponse.
type RegisterNodeResponse = pkgprotocol.RegisterNodeResponse

// ApplyLogPayload is an alias for pkgprotocol.ApplyLogPayload.
type ApplyLogPayload = pkgprotocol.ApplyLogPayload

// ActiveConfigResponse is an alias for pkgprotocol.ActiveConfigResponse.
type ActiveConfigResponse = pkgprotocol.ActiveConfigResponse

// ActiveConfigMeta is an alias for pkgprotocol.ActiveConfigMeta.
type ActiveConfigMeta = pkgprotocol.ActiveConfigMeta

// WAFIPGroup is an alias for pkgprotocol.WAFIPGroup.
type WAFIPGroup = pkgprotocol.WAFIPGroup

// WAFIPGroupSyncRequest is an alias for pkgprotocol.WAFIPGroupSyncRequest.
type WAFIPGroupSyncRequest = pkgprotocol.WAFIPGroupSyncRequest

// WAFIPGroupSyncResponse is an alias for pkgprotocol.WAFIPGroupSyncResponse.
type WAFIPGroupSyncResponse = pkgprotocol.WAFIPGroupSyncResponse

// SupportFile is an alias for pkgprotocol.SupportFile.
type SupportFile = pkgprotocol.SupportFile

// PagesDeploymentHashResponse is an alias for pkgprotocol.PagesDeploymentHashResponse.
type PagesDeploymentHashResponse = pkgprotocol.PagesDeploymentHashResponse

// PagesProjectLatestHashResponse is an alias for pkgprotocol.PagesProjectLatestHashResponse.
type PagesProjectLatestHashResponse = pkgprotocol.PagesProjectLatestHashResponse

const (
	// WSMessageTypeStatus is an alias for pkgprotocol.WSMessageTypeStatus.
	WSMessageTypeStatus = pkgprotocol.WSMessageTypeStatus
	// WSMessageTypeSettings is an alias for pkgprotocol.WSMessageTypeSettings.
	WSMessageTypeSettings = pkgprotocol.WSMessageTypeSettings
	// WSMessageTypeActiveConfig is an alias for pkgprotocol.WSMessageTypeActiveConfig.
	WSMessageTypeActiveConfig = pkgprotocol.WSMessageTypeActiveConfig
	// WSMessageTypeForceSyncConfig is an alias for pkgprotocol.WSMessageTypeForceSyncConfig.
	WSMessageTypeForceSyncConfig = pkgprotocol.WSMessageTypeForceSyncConfig
	// WSMessageTypeWAFIPGroups is an alias for pkgprotocol.WSMessageTypeWAFIPGroups.
	WSMessageTypeWAFIPGroups = pkgprotocol.WSMessageTypeWAFIPGroups
	// WSMessageTypePing is an alias for pkgprotocol.WSMessageTypePing.
	WSMessageTypePing = pkgprotocol.WSMessageTypePing
	// WSMessageTypePong is an alias for pkgprotocol.WSMessageTypePong.
	WSMessageTypePong = pkgprotocol.WSMessageTypePong
)

const (
	// OpenrestyStatusHealthy is an alias for pkgprotocol.OpenrestyStatusHealthy.
	OpenrestyStatusHealthy = pkgprotocol.OpenrestyStatusHealthy
	// OpenrestyStatusUnhealthy is an alias for pkgprotocol.OpenrestyStatusUnhealthy.
	OpenrestyStatusUnhealthy = pkgprotocol.OpenrestyStatusUnhealthy
	// OpenrestyStatusUnknown is an alias for pkgprotocol.OpenrestyStatusUnknown.
	OpenrestyStatusUnknown = pkgprotocol.OpenrestyStatusUnknown
)
