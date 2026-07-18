// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import pkgprotocol "github.com/Rain-kl/Wavelet/pkg/protocol"

// NodePayload is the data sent by an agent on registration or heartbeat.
type NodePayload = pkgprotocol.NodePayload

// NodeSystemProfile carries static host information reported by an agent.
type NodeSystemProfile = pkgprotocol.NodeSystemProfile

// NodeMetricSnapshot holds a point-in-time resource-usage sample from an agent.
type NodeMetricSnapshot = pkgprotocol.NodeMetricSnapshot

// NodeEdgeHealth is L2 OpenResty health + connections.
type NodeEdgeHealth = pkgprotocol.NodeEdgeHealth

// NodeAccessLog is a single access-log record forwarded by an agent.
type NodeAccessLog = pkgprotocol.NodeAccessLog

// BufferedObservabilityRecord bundles multiple observability payloads into one upload.
type BufferedObservabilityRecord = pkgprotocol.BufferedObservabilityRecord

// NodeHealthEvent represents a discrete health-state change on an agent node.
type NodeHealthEvent = pkgprotocol.NodeHealthEvent

// ApplyLogPayload carries the result of a configuration-apply attempt reported by an agent.
type ApplyLogPayload = pkgprotocol.ApplyLogPayload

// Settings contains remote-control directives sent from the server to an agent.
type Settings = pkgprotocol.AgentSettings

// ActiveConfigMeta describes the currently active configuration version on the server.
type ActiveConfigMeta = pkgprotocol.ActiveConfigMeta

// SupportFile represents a supplementary file bundled with an agent configuration package.
type SupportFile = pkgprotocol.SupportFile

// WAFIPGroup is a named IP-address group used in WAF allow/block rules.
type WAFIPGroup = pkgprotocol.WAFIPGroup

// WAFIPGroupSyncRequest is sent by an agent to request an incremental WAF IP-group sync.
type WAFIPGroupSyncRequest = pkgprotocol.WAFIPGroupSyncRequest

// WAFIPGroupSyncResponse carries the server's reply to a WAF IP-group sync request.
type WAFIPGroupSyncResponse = pkgprotocol.WAFIPGroupSyncResponse

// Backward-compatible names used by server routers and handlers.

// WAFIPGroupSyncInput is an alias for WAFIPGroupSyncRequest kept for backward compatibility.
type WAFIPGroupSyncInput = WAFIPGroupSyncRequest

// WAFIPGroupSyncResult is an alias for WAFIPGroupSyncResponse kept for backward compatibility.
type WAFIPGroupSyncResult = WAFIPGroupSyncResponse
