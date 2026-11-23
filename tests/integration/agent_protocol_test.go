// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	ofnode "github.com/Rain-kl/Wavelet/internal/apps/openflare/node"
	oftunnel "github.com/Rain-kl/Wavelet/internal/apps/openflare/tunnel"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupProtocolTestEnv(t *testing.T) (*gin.Engine, func()) {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareNode{},
		&model.SystemConfig{},
		&model.OpenFlareApplyLog{},
		&model.OpenFlareNodeSystemProfile{},
		&model.OpenFlareHealthEvent{},
		&model.ConfigVersion{},
		&model.TunnelRelayPool{},
		&model.TunnelRelay{},
		&model.Tunnel{},
		&model.TunnelConnector{},
		&model.TunnelAttachment{},
		&model.TunnelService{},
		&model.Zone{},
		&model.ZoneDomain{},
		&model.ProxyRoute{},
	))
	require.NoError(t, sqliteDB.Create(&model.TunnelRelayPool{
		PoolKey:           "default",
		Name:              "Default Pool",
		IsDefault:         true,
		RoutingPolicyJSON: "{}",
	}).Error)

	db.SetDB(sqliteDB)
	agent.ResetAuthCacheForTest()
	testhelper.SetupLogStoresForTest(t)

	engine := testhelper.NewTestGinEngine()
	mountOpenFlareTestRoutes(engine)

	cleanup := func() {
		db.SetDB(nil)
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
			"name":                "edge-1",
			"ip":                  "203.0.113.10",
			"version":             "0.1.0",
			"engine":              "pingora",
			"contract_version":    edgeconfig.ContractVersion,
			"config_sync_version": protocol.EdgeConfigSyncVersion,
		}, map[string]string{
			"X-Agent-Token": edge.AccessToken,
		})
		assert.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		assert.NotNil(t, data["agent_settings"])
		assert.NotNil(t, data["config"])
	})

	t.Run("relay discovery registration, connector negotiation, verified heartbeat and edge tunnel-routing.v1 manifest", func(t *testing.T) {
		bootstrap, err := oftunnel.GetRelayBootstrapToken(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, bootstrap.BootstrapToken)

		// 1. Relay self-registers using bootstrap discovery token.
		regRec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/relay/register", map[string]any{
			"relay_id":               "relay-edge-pop",
			"name":                   "HK Pop",
			"public_addr":            "198.51.100.50",
			"transport_port":         7443,
			"gateway_port":           8443,
			"zone_tag":               "hk-1",
			"colocated_edge_node_id": "",
			"supported_transports":   []string{"noise", "tcp"},
		}, map[string]string{
			"X-Relay-Token": bootstrap.BootstrapToken,
		})
		require.Equal(t, http.StatusOK, regRec.Code, regRec.Body.String())
		var regResp protocol.RelayRegisterResponse
		unmarshalAPIData(t, requireAPIOK(t, regRec).Data, &regResp)
		require.NotEmpty(t, regResp.AccessToken)
		require.NotEmpty(t, regResp.NoisePrivateKey)
		require.NotEmpty(t, regResp.NoisePublicKey)
		require.NotEmpty(t, regResp.GatewaySecret)

		// 2. Provision logical tunnel.
		tunDeploy, err := oftunnel.CreateTunnel(ctx, oftunnel.TunnelInput{
			Name:         "Intranet App",
			HARelayCount: 1,
		})
		require.NoError(t, err)

		// 3. Connector heartbeats and pulls candidate relays.
		hbRec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/tunnel/heartbeat", map[string]any{
			"connector_id": "conn-int-1",
			"version":      "0.1.0",
		}, map[string]string{
			"X-Tunnel-Token": tunDeploy.BootstrapToken,
			"X-Connector-ID": "conn-int-1",
		})
		require.Equal(t, http.StatusOK, hbRec.Code, hbRec.Body.String())

		cfgRec := performJSONRequest(t, engine, http.MethodGet, "/api/v1/tunnel/config?connector_id=conn-int-1", nil, map[string]string{
			"X-Tunnel-Token": tunDeploy.BootstrapToken,
		})
		require.Equal(t, http.StatusOK, cfgRec.Code, cfgRec.Body.String())
		var connCfg protocol.TunnelConnectorConfigResponse
		unmarshalAPIData(t, requireAPIOK(t, cfgRec).Data, &connCfg)
		require.Len(t, connCfg.CandidateRelays, 1)

		// 4. Connector negotiates attachment to relay-edge-pop.
		negRec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/tunnel/attachments", map[string]any{
			"connector_id": "conn-int-1",
			"desired": []map[string]any{
				{"relay_id": regResp.ID, "transport": "noise", "rtt_ms": 18},
			},
		}, map[string]string{
			"X-Tunnel-Token": tunDeploy.AccessToken,
		})
		require.Equal(t, http.StatusOK, negRec.Code, negRec.Body.String())
		var negResp protocol.TunnelNegotiateAttachmentsResponse
		unmarshalAPIData(t, requireAPIOK(t, negRec).Data, &negResp)
		require.Len(t, negResp.Attachments, 1)

		// 5. Relay pulls runtime config and verifies active attachment via heartbeat.
		relayCfgRec := performJSONRequest(t, engine, http.MethodGet, "/api/v1/relay/config", nil, map[string]string{
			"X-Relay-Token": regResp.AccessToken,
		})
		require.Equal(t, http.StatusOK, relayCfgRec.Code, relayCfgRec.Body.String())
		var relayCfg protocol.RelayRuntimeConfigResponse
		unmarshalAPIData(t, requireAPIOK(t, relayCfgRec).Data, &relayCfg)
		require.Len(t, relayCfg.Services, 1)
		assert.Equal(t, negResp.Attachments[0].SessionToken, relayCfg.Services[0].Token)

		relayHBRec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/relay/heartbeat", map[string]any{
			"version":           "0.1.0",
			"runtime_status":    "healthy",
			"active_connectors": 1,
			"verified_attachments": []map[string]any{
				{"connector_id": "conn-int-1", "service_name": relayCfg.Services[0].ServiceName, "rtt_ms": 18, "active_streams": 1},
			},
		}, map[string]string{
			"X-Relay-Token": regResp.AccessToken,
		})
		require.Equal(t, http.StatusOK, relayHBRec.Code, relayHBRec.Body.String())

		// 6. Edge node receives tunnel-routing.v1 inside config manifest on heartbeat.
		edgeNode, err := ofnode.CreateNode(ctx, ofnode.Input{Name: "edge-routing-check", IP: "10.0.0.9"})
		require.NoError(t, err)
		edgeHBRec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/nodes/heartbeat", map[string]any{
			"name":                "edge-routing-check",
			"ip":                  "203.0.113.9",
			"version":             "0.1.0",
			"engine":              "pingora",
			"contract_version":    edgeconfig.ContractVersion,
			"config_sync_version": protocol.EdgeConfigSyncVersion,
		}, map[string]string{
			"X-Agent-Token": edgeNode.AccessToken,
		})
		require.Equal(t, http.StatusOK, edgeHBRec.Code, edgeHBRec.Body.String())
		edgeResp := unmarshalAPIMap(t, requireAPIOK(t, edgeHBRec).Data)
		manifest := edgeResp["config"].(map[string]any)
		resources := manifest["resources"].(map[string]any)
		tunnelRoutingRes := resources["tunnel-routing.v1"].(map[string]any)
		inlineJSON, ok := tunnelRoutingRes["inline_json"].(string)
		require.True(t, ok)
		var routingSnap protocol.TunnelRoutingSnapshot
		require.NoError(t, json.Unmarshal([]byte(inlineJSON), &routingSnap))
		require.Len(t, routingSnap.Tunnels, 1)
		require.Len(t, routingSnap.Tunnels[0].Relays, 1)
		assert.Equal(t, "http://198.51.100.50:8443", routingSnap.Tunnels[0].Relays[0].GatewayURL)

		// 7. Revoking connector blocks subsequent connector requests with 403.
		require.NoError(t, oftunnel.RevokeConnector(ctx, tunDeploy.Tunnel.ID, "conn-int-1"))
		revokedRec := performJSONRequest(t, engine, http.MethodGet, "/api/v1/tunnel/config?connector_id=conn-int-1", nil, map[string]string{
			"X-Tunnel-Token": tunDeploy.AccessToken,
		})
		assert.Equal(t, http.StatusForbidden, revokedRec.Code)
	})

	t.Run("agent register with discovery token from options", func(t *testing.T) {
		bootstrap, err := ofnode.GetBootstrapToken(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, bootstrap.DiscoveryToken)

		rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/nodes/register", map[string]any{
			"name":                "discovered-edge",
			"ip":                  "203.0.113.30",
			"version":             "0.2.0",
			"engine":              "pingora",
			"contract_version":    edgeconfig.ContractVersion,
			"config_sync_version": protocol.EdgeConfigSyncVersion,
		}, map[string]string{
			"X-Agent-Token": bootstrap.DiscoveryToken,
		})
		assert.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		var registration protocol.RegisterNodeResponse
		unmarshalAPIData(t, resp.Data, &registration)
		assert.NotEmpty(t, registration.NodeID)
		assert.NotEmpty(t, registration.AccessToken)
		assert.Equal(t, "discovered-edge", registration.Name)

		stored, err := repository.GetOpenFlareNodeByNodeID(ctx, registration.NodeID)
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

		require.NoError(t, db.DB(ctx).Create(&model.ConfigVersion{
			Version: "20260618-001", Engine: "pingora", ReleaseSequence: 1, IsActive: true,
			PayloadChecksum: "digest-fixture",
		}).Error)

		rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/apply-logs", map[string]any{
			"version":          "20260618-001",
			"result":           "success",
			"message":          "apply ok",
			"engine":           "pingora",
			"release_sequence": 1,
			"payload_checksum": "digest-fixture",
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

		stored, err := repository.GetOpenFlareNodeByNodeID(ctx, edge.NodeID)
		require.NoError(t, err)
		assert.Equal(t, "online", stored.Status)
		assert.Equal(t, "20260618-001", stored.CurrentVersion)
	})

	t.Run("GET /api/geoip returns requester IP behind proxy and directly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/geoip", nil)
		req.RemoteAddr = "172.27.0.1:54321"
		req.Header.Set("X-Forwarded-For", "82.22.26.203")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		assert.Equal(t, "82.22.26.203", data["ip"])
	})

	t.Run("agent heartbeat never stores internal IP and falls back to 0.0.0.0", func(t *testing.T) {
		edge, err := ofnode.CreateNode(ctx, ofnode.Input{
			Name: "edge-internal-guard",
		})
		require.NoError(t, err)

		payloadBytes, err := json.Marshal(map[string]any{
			"name":                "edge-internal-guard",
			"ip":                  "172.27.0.1",
			"version":             "0.1.2",
			"engine":              "pingora",
			"contract_version":    edgeconfig.ContractVersion,
			"config_sync_version": protocol.EdgeConfigSyncVersion,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/nodes/heartbeat", strings.NewReader(string(payloadBytes)))
		req.RemoteAddr = "172.27.0.1:54321"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Agent-Token", edge.AccessToken)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		requireAPIOK(t, rec)

		stored, err := repository.GetOpenFlareNodeByNodeID(ctx, edge.NodeID)
		require.NoError(t, err)
		assert.Equal(t, "0.0.0.0", stored.IP)
	})
}
