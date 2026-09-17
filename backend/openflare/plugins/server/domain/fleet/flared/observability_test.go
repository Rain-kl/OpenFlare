// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package flared

import (
	"context"
	"testing"

	"Wavelet/openflare/plugins/server/kernel/repository"

	"Wavelet/openflare/plugins/server/domain/fleet/agent"
	"Wavelet/openflare/plugins/server/kernel/model"

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
		&model.SystemConfig{},
		&model.ConfigVersion{},
	))

	repository.SetDBForTest(sqliteDB)
	agent.ResetAuthCacheForTest()

	return func() {
		repository.SetDBForTest(nil)
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
	require.NoError(t, repository.DB(ctx).Create(node).Error)

	_, err := Heartbeat(ctx, node, HeartbeatPayload{
		ClientVersion:   "v0.2.0",
		FrpVersion:      "0.61.0",
		TunnelStatus:    "unhealthy",
		CurrentVersion:  "v1",
		CurrentChecksum: "checksum-1",
	})
	require.NoError(t, err)

	events, err := repository.ListOpenFlareHealthEvents(ctx, node.NodeID, false, 20)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, flaredRuntimeUnhealthyEventType, events[0].EventType)
	assert.Equal(t, "active", events[0].Status)
}
