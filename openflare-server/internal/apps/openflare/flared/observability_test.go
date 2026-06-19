// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package flared

import (
	"context"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/option"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFlaredObservabilityTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareNode{},
		&model.OpenFlareHealthEvent{},
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

func TestHeartbeatFlaredEmitsHealthEventOnUnhealthy(t *testing.T) {
	cleanup := setupFlaredObservabilityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	node := &model.OpenFlareNode{
		NodeID:      "node-flared-unhealthy",
		Name:        "flared-unhealthy",
		AccessToken: "tunnel-token-unhealthy",
		Status:      "pending",
		NodeType:    "tunnel_client",
	}
	require.NoError(t, db.DB(ctx).Create(node).Error)

	_, err := Heartbeat(ctx, node, HeartbeatPayload{
		ClientVersion:   "v0.2.0",
		FrpVersion:      "0.61.0",
		TunnelStatus:    "unhealthy",
		CurrentVersion:  "v1",
		CurrentChecksum: "checksum-1",
	})
	require.NoError(t, err)

	events, err := model.ListOpenFlareHealthEvents(ctx, node.NodeID, false, 20)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, flaredRuntimeUnhealthyEventType, events[0].EventType)
	assert.Equal(t, "active", events[0].Status)
}
