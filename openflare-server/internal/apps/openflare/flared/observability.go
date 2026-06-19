// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package flared

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/db"
	"go.uber.org/zap"
)

const flaredRuntimeUnhealthyEventType = "flared_runtime_unhealthy"

func persistFlaredObservability(ctx context.Context, nodeID string, payload HeartbeatPayload, reportedAt time.Time) {
	connected := make([]string, 0, len(payload.ConnectedRelays))
	for _, relay := range payload.ConnectedRelays {
		connected = append(connected, fmt.Sprintf("%s:%s", relay.RelayNodeID, relay.Status))
	}
	managedTypes := map[string]struct{}{
		flaredRuntimeUnhealthyEventType: {},
	}
	var events []agent.NodeHealthEvent
	if payload.TunnelStatus == "unhealthy" {
		events = append(events, agent.NodeHealthEvent{
			EventType:       flaredRuntimeUnhealthyEventType,
			Severity:        "critical",
			Message:         "openflared runtime is not healthy",
			TriggeredAtUnix: reportedAt.Unix(),
			Metadata: map[string]string{
				"tunnel_status":    payload.TunnelStatus,
				"client_version":   payload.ClientVersion,
				"current_version":  payload.CurrentVersion,
				"current_checksum": payload.CurrentChecksum,
				"connected_relays": strings.Join(connected, ","),
			},
		})
	}
	conn := db.DB(ctx)
	if conn == nil {
		return
	}
	if err := agent.ReconcileScopedNodeHealthEvents(conn, nodeID, events, reportedAt, managedTypes); err != nil {
		zap.L().Error("persist flared health events failed", zap.String("node_id", nodeID), zap.Error(err))
	}
}
