// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const relayFrpsUnhealthyEventType = "frps_unhealthy"

func reconcileRelayHealthEvents(ctx context.Context, nodeID string, relayStatus string, reportedAt time.Time) error {
	if relayStatus == "unknown" {
		return nil
	}
	managedTypes := map[string]struct{}{
		relayFrpsUnhealthyEventType: {},
	}
	events := []agent.NodeHealthEvent{}
	if relayStatus == "unhealthy" {
		events = append(events, agent.NodeHealthEvent{
			EventType:       relayFrpsUnhealthyEventType,
			Severity:        "critical",
			Message:         "frps runtime is not healthy",
			TriggeredAtUnix: reportedAt.Unix(),
			Metadata: map[string]string{
				"relay_status": relayStatus,
			},
		})
	}
	conn := db.DB(ctx)
	if conn == nil {
		return nil
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		return agent.ReconcileScopedNodeHealthEvents(tx, nodeID, events, reportedAt, managedTypes)
	})
}

func persistRelayHeartbeatObservability(ctx context.Context, nodeID string, payload HeartbeatPayload, reportedAt time.Time) {
	agent.PersistHeartbeatObservability(ctx, nodeID, agent.NodePayload{
		Profile:      payload.Profile,
		Snapshot:     payload.Snapshot,
		HealthEvents: payload.HealthEvents,
	}, reportedAt)

	conn := db.DB(ctx)
	if conn == nil {
		return
	}
	frpsObs := &model.OpenFlareNodeObservationFrps{
		NodeID:          nodeID,
		CapturedAt:      reportedAt,
		FrpsConnections: payload.FrpsConnCount,
		FrpsProxyCount:  payload.FrpsProxyCount,
		FrpsClientCount: payload.FrpsClientCount,
		FrpsProxies:     agent.MarshalJSON(payload.FrpsProxies),
	}
	if err := conn.Create(frpsObs).Error; err != nil {
		zap.L().Error("persist relay frps observation failed", zap.String("node_id", nodeID), zap.Error(err))
	}
}
