// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	ofnode "github.com/Rain-kl/Wavelet/internal/apps/openflare/node"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/option"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type configVersionRecord struct {
	ID               uint   `gorm:"primaryKey"`
	Version          string `gorm:"column:version"`
	SnapshotJSON     string `gorm:"column:snapshot_json"`
	SupportFilesJSON string `gorm:"column:support_files_json"`
	Checksum         string `gorm:"column:checksum"`
	IsActive         bool   `gorm:"column:is_active"`
}

func (configVersionRecord) TableName() string {
	return "of_config_versions"
}

func setupProtocolTestEnv(t *testing.T) (*gin.Engine, func()) {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareNode{},
		&model.OpenFlareOption{},
		&model.OpenFlareApplyLog{},
		&model.OpenFlareNodeSystemProfile{},
		&model.OpenFlareMetricSnapshot{},
		&model.OpenFlareHealthEvent{},
		&model.OpenFlareNodeObservationFrps{},
		&model.OpenFlareNodeObservationFrpc{},
		&configVersionRecord{},
	))

	db.SetDB(sqliteDB)
	option.ResetInitializationForTest()
	agent.ResetAuthCacheForTest()

	engine := testhelper.NewTestGinEngine()
	mountOpenFlareTestRoutes(engine)

	cleanup := func() {
		db.SetDB(nil)
		option.ResetInitializationForTest()
		agent.ResetAuthCacheForTest()
	}
	return engine, cleanup
}

func TestAgentRelayFlaredProtocol(t *testing.T) {
	engine, cleanup := setupProtocolTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create edge node and heartbeat with X-Agent-Token", func(t *testing.T) {
		edge, err := ofnode.CreateNode(ctx, ofnode.Input{
			Name: "edge-1",
			IP:   "10.0.0.1",
		})
		require.NoError(t, err)
		require.NotEmpty(t, edge.AccessToken)
		assert.Equal(t, "edge_node", edge.NodeType)

		rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/nodes/heartbeat", map[string]any{
			"name":    "edge-1",
			"ip":      "203.0.113.10",
			"version": "0.1.0",
		}, map[string]string{
			"X-Agent-Token": edge.AccessToken,
		})
		assert.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		assert.NotNil(t, data["agent_settings"])
	})

	t.Run("create tunnel_relay node and relay heartbeat", func(t *testing.T) {
		relayNode, err := ofnode.CreateNode(ctx, ofnode.Input{
			Name:     "relay-1",
			NodeType: "tunnel_relay",
		})
		require.NoError(t, err)
		require.NotEmpty(t, relayNode.AccessToken)

		rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/relay/heartbeat", map[string]any{
			"version":      "v0.1.0",
			"frp_version":  "0.61.0",
			"relay_status": "healthy",
			"name":         "relay-1",
			"ip":           "203.0.113.20",
		}, map[string]string{
			"X-Agent-Token": relayNode.AccessToken,
		})
		assert.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		var heartbeatData struct {
			RelayConfig   map[string]any `json:"relay_config"`
			RelaySettings map[string]any `json:"relay_settings"`
		}
		unmarshalAPIData(t, resp.Data, &heartbeatData)
		assert.NotNil(t, heartbeatData.RelayConfig)
		assert.NotNil(t, heartbeatData.RelaySettings)

		stored, err := model.GetOpenFlareNodeByNodeID(ctx, relayNode.NodeID)
		require.NoError(t, err)
		assert.Equal(t, "online", stored.Status)
		assert.Equal(t, "healthy", stored.RelayStatus)
	})

	t.Run("create tunnel_client node and flared heartbeat with X-Tunnel-Token", func(t *testing.T) {
		clientNode, err := ofnode.CreateNode(ctx, ofnode.Input{
			Name:     "client-1",
			NodeType: "tunnel_client",
		})
		require.NoError(t, err)
		require.NotEmpty(t, clientNode.AccessToken)

		rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/tunnel/heartbeat", map[string]any{
			"client_version": "v0.2.0",
			"frp_version":    "0.61.0",
			"tunnel_status":  "running",
		}, map[string]string{
			"X-Tunnel-Token": clientNode.AccessToken,
		})
		assert.Equal(t, http.StatusOK, rec.Code)
		requireAPIOK(t, rec)

		stored, err := model.GetOpenFlareNodeByNodeID(ctx, clientNode.NodeID)
		require.NoError(t, err)
		assert.Equal(t, "online", stored.Status)
		assert.Equal(t, "v0.2.0", stored.Version)
	})

	t.Run("agent register with discovery token from options", func(t *testing.T) {
		bootstrap, err := ofnode.GetBootstrapToken(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, bootstrap.DiscoveryToken)

		rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/nodes/register", map[string]any{
			"name":    "discovered-edge",
			"ip":      "203.0.113.30",
			"version": "0.2.0",
		}, map[string]string{
			"X-Agent-Token": bootstrap.DiscoveryToken,
		})
		assert.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		var registration agent.RegistrationResponse
		unmarshalAPIData(t, resp.Data, &registration)
		assert.NotEmpty(t, registration.NodeID)
		assert.NotEmpty(t, registration.AccessToken)
		assert.Equal(t, "discovered-edge", registration.Name)

		stored, err := model.GetOpenFlareNodeByNodeID(ctx, registration.NodeID)
		require.NoError(t, err)
		assert.Equal(t, "online", stored.Status)
		assert.Equal(t, registration.AccessToken, stored.AccessToken)
	})

	t.Run("POST agent apply-logs", func(t *testing.T) {
		edge, err := ofnode.CreateNode(ctx, ofnode.Input{
			Name: "edge-apply",
			IP:   "10.0.0.2",
		})
		require.NoError(t, err)

		rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/apply-logs", map[string]any{
			"version": "20260618-001",
			"result":  "success",
			"message": "apply ok",
		}, map[string]string{
			"X-Agent-Token": edge.AccessToken,
		})
		assert.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		var applyLog model.OpenFlareApplyLog
		unmarshalAPIData(t, resp.Data, &applyLog)
		assert.Equal(t, edge.NodeID, applyLog.NodeID)
		assert.Equal(t, "success", applyLog.Result)
		assert.Equal(t, "20260618-001", applyLog.Version)

		stored, err := model.GetOpenFlareNodeByNodeID(ctx, edge.NodeID)
		require.NoError(t, err)
		assert.Equal(t, "online", stored.Status)
		assert.Equal(t, "20260618-001", stored.CurrentVersion)
	})
}