// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/option"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRelayTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareNode{},
		&model.OpenFlareOption{},
		&model.OpenFlareNodeSystemProfile{},
		&model.OpenFlareMetricSnapshot{},
		&model.OpenFlareHealthEvent{},
		&model.OpenFlareNodeObservationFrps{},
	))

	db.SetDB(sqliteDB)
	option.ResetInitializationForTest()
	agent.ResetAuthCacheForTest()

	return func() {
		db.SetDB(nil)
		option.ResetInitializationForTest()
		agent.ResetAuthCacheForTest()
	}
}

func TestHeartbeatPayloadBindingAndFrpsObservationInsert(t *testing.T) {
	cleanup := setupRelayTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	node := &model.OpenFlareNode{
		NodeID:      "node-relay-observe",
		Name:        "relay-1",
		AccessToken: "relay-token",
		Status:      "pending",
		NodeType:    "tunnel_relay",
		RelayStatus: "unknown",
	}
	require.NoError(t, db.DB(ctx).Create(node).Error)

	proxies := []ProxyStat{
		{
			Name:          "proxy-a",
			Type:          "http",
			Status:        "online",
			ClientVersion: "0.61.0",
			ClientAddr:    "10.0.0.2:12345",
		},
	}
	_, err := Heartbeat(ctx, node, HeartbeatPayload{
		Version:         "v0.1.0",
		ExtVersion:      "0.61.0",
		RelayStatus:     "healthy",
		FrpsConnCount:   7,
		FrpsProxyCount:  3,
		FrpsClientCount: 2,
		FrpsProxies:     proxies,
		Name:            "relay-runtime",
		IP:              "203.0.113.9",
		Profile: &agent.NodeSystemProfile{
			Hostname:       "relay-runtime",
			OSName:         "Ubuntu",
			OSVersion:      "24.04",
			Architecture:   "amd64",
			CPUCores:       4,
			ReportedAtUnix: now.Unix(),
		},
		Snapshot: &agent.NodeMetricSnapshot{
			CapturedAtUnix:  now.Unix(),
			CPUUsagePercent: 12.5,
			NetworkRxBytes:  1024,
			NetworkTxBytes:  2048,
		},
		HealthEvents: []agent.NodeHealthEvent{},
	})
	require.NoError(t, err)

	var stored model.OpenFlareNode
	require.NoError(t, db.DB(ctx).Where("node_id = ?", node.NodeID).First(&stored).Error)
	assert.Equal(t, "online", stored.Status)
	assert.Equal(t, "healthy", stored.RelayStatus)
	assert.Equal(t, "203.0.113.9", stored.IP)
	assert.Equal(t, "v0.1.0", stored.Version)
	assert.Equal(t, "0.61.0", stored.ExtVersion)

	profile, err := model.GetOpenFlareNodeSystemProfile(ctx, node.NodeID)
	require.NoError(t, err)
	assert.Equal(t, "relay-runtime", profile.Hostname)
	assert.Equal(t, "Ubuntu", profile.OSName)

	snapshots, err := model.ListOpenFlareMetricSnapshotsSince(ctx, node.NodeID, now.Add(-time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	assert.Equal(t, 12.5, snapshots[0].CPUUsagePercent)

	frpsObs, err := model.ListOpenFlareNodeObservationFrps(ctx, node.NodeID, time.Time{}, 1)
	require.NoError(t, err)
	require.Len(t, frpsObs, 1)
	assert.Equal(t, 7, frpsObs[0].FrpsConnections)
	assert.Equal(t, 3, frpsObs[0].FrpsProxyCount)
	assert.Equal(t, 2, frpsObs[0].FrpsClientCount)

	var decoded []ProxyStat
	require.NoError(t, json.Unmarshal([]byte(frpsObs[0].FrpsProxies), &decoded))
	require.Len(t, decoded, 1)
	assert.Equal(t, "proxy-a", decoded[0].Name)
	assert.Equal(t, "online", decoded[0].Status)
}

func TestHeartbeatRelayReconcilesFrpsUnhealthyEvent(t *testing.T) {
	cleanup := setupRelayTestDB(t)
	defer cleanup()

	ctx := context.Background()
	node := &model.OpenFlareNode{
		NodeID:      "node-relay-unhealthy",
		Name:        "relay-unhealthy",
		AccessToken: "relay-token-unhealthy",
		Status:      "pending",
		NodeType:    "tunnel_relay",
		RelayStatus: "healthy",
	}
	require.NoError(t, db.DB(ctx).Create(node).Error)

	_, err := Heartbeat(ctx, node, HeartbeatPayload{
		Version:     "v0.1.0",
		ExtVersion:  "0.61.0",
		RelayStatus: "unhealthy",
	})
	require.NoError(t, err)

	events, err := model.ListOpenFlareHealthEvents(ctx, node.NodeID, true, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, relayFrpsUnhealthyEventType, events[0].EventType)
	assert.Equal(t, "active", events[0].Status)

	_, err = Heartbeat(ctx, node, HeartbeatPayload{
		Version:     "v0.1.0",
		ExtVersion:  "0.61.0",
		RelayStatus: "healthy",
	})
	require.NoError(t, err)

	events, err = model.ListOpenFlareHealthEvents(ctx, node.NodeID, false, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "resolved", events[0].Status)
}
