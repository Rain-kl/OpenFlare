// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/config_version"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/migrator"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	adminUserID   = uint64(1001)
	adminUsername = "openflare-admin"
)

type adminSeed struct {
	User      model.User
	Token     string
	TokenHash string
}

func setupCoreChainTest(t *testing.T) (*gin.Engine, adminSeed, func()) {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := sqliteDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	db.SetDB(sqliteDB)
	require.True(t, migrator.Migrate().Applied)
	agent.ResetAuthCacheForTest()

	seed, err := seedAdminWithAccessToken(sqliteDB)
	require.NoError(t, err)

	engine := testhelper.NewTestGinEngine()
	mountOpenFlareTestRoutes(engine)

	cleanup := func() {
		db.SetDB(nil)
		agent.ResetAuthCacheForTest()
	}

	return engine, seed, cleanup
}

func seedAdminWithAccessToken(conn *gorm.DB) (adminSeed, error) {
	now := time.Now().UTC()
	admin := model.User{
		ID:          adminUserID,
		Username:    adminUsername,
		Nickname:    "OpenFlare Admin",
		IsActive:    true,
		IsAdmin:     true,
		LastLoginAt: now,
	}
	if err := conn.Create(&admin).Error; err != nil {
		return adminSeed{}, err
	}

	token, err := model.GenerateTokenString()
	if err != nil {
		return adminSeed{}, err
	}
	tokenHash := model.HashToken(token)
	tokenRecord := model.AccessToken{
		UserID:      adminUserID,
		Name:        "integration-admin-token",
		TokenHash:   tokenHash,
		MaskedToken: model.MaskTokenString(token),
		IsAdmin:     true,
	}
	if err := conn.Create(&tokenRecord).Error; err != nil {
		return adminSeed{}, err
	}

	return adminSeed{
		User:      admin,
		Token:     token,
		TokenHash: tokenHash,
	}, nil
}

func TestCoreChainMigrationFlow(t *testing.T) {
	engine, seed, cleanup := setupCoreChainTest(t)
	defer cleanup()

	ctx := context.Background()
	stop := config_version.StartRefreshLoop(ctx)
	defer func() {
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		require.NoError(t, stop(stopCtx))
	}()

	var initial *model.ConfigVersion
	require.Eventually(t, func() bool {
		var err error
		initial, err = repository.GetActivePingoraConfigVersion(ctx)
		return err == nil
	}, time.Second, 10*time.Millisecond)
	require.Contains(t, initial.SnapshotJSON, `"routes":[]`)

	var (
		releaseSequence int64
		originID        uint
		proxyRouteID    uint
		configVersion   string
		configChecksum  string
		nodeID          uint
		nodePublicID    string
		agentToken      string
	)

	t.Run("create origin", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/origins/"), map[string]any{
			"name":    "Primary Origin",
			"address": "origin.core-chain.internal",
			"remark":  "integration upstream",
		}, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		originID = uint(data["id"].(float64))
		assert.NotZero(t, originID)
		assert.Equal(t, "Primary Origin", data["name"])
		assert.Equal(t, "origin.core-chain.internal", data["address"])
	})

	t.Run("create proxy route linked to origin", func(t *testing.T) {
		// Create Zone and ZoneDomain directly in the DB
		zone := model.Zone{Domain: "example.com"}
		require.NoError(t, db.DB(context.Background()).Create(&zone).Error)
		zoneDomain := model.ZoneDomain{
			ZoneID: zone.ID,
			Domain: "core-chain.example.com",
		}
		require.NoError(t, db.DB(context.Background()).Create(&zoneDomain).Error)

		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/proxy-routes/"), map[string]any{
			"site_name":       "core-chain-site",
			"zone_domain_ids": []uint{zoneDomain.ID},
			"origin_id":       originID,
			"origin_scheme":   "http",
			"origin_port":     "8080",
			"enabled":         true,
		}, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		proxyRouteID = uint(data["id"].(float64))
		assert.NotZero(t, proxyRouteID)
		assert.Equal(t, "core-chain-site", data["site_name"])
		assert.NotEmpty(t, data["zone_domains"])
		zoneDomains := data["zone_domains"].([]any)
		assert.Len(t, zoneDomains, 1)
		assert.Equal(t, "core-chain.example.com", zoneDomains[0].(map[string]any)["domain"])
		assert.InDelta(t, float64(originID), data["origin_id"], 1e-9)
		assert.Equal(t, "http://origin.core-chain.internal:8080", data["origin_url"])
	})

	t.Run("assemble saved configuration", func(t *testing.T) {
		var active *model.ConfigVersion
		require.Eventually(t, func() bool {
			var err error
			active, err = repository.GetActivePingoraConfigVersion(ctx)
			return err == nil && active.Version != initial.Version
		}, time.Second, 10*time.Millisecond)
		configVersion, configChecksum, releaseSequence = active.Version, active.Checksum, active.ReleaseSequence
		assert.NotEmpty(t, configVersion)
		assert.NotEmpty(t, configChecksum)
		assert.Contains(t, active.SnapshotJSON, "core-chain.example.com")
		assert.Greater(t, active.ReleaseSequence, initial.ReleaseSequence)

		require.NoError(t, config_version.RefreshSiteSnapshot(ctx))
		unchanged, err := repository.GetActivePingoraConfigMetadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, configVersion, unchanged.Version)
	})

	t.Run("create node", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/nodes/"), map[string]any{
			"name":                "edge-core-chain",
			"ip":                  "10.10.0.1",
			"auto_update_enabled": true,
		}, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		nodeID = uint(data["id"].(float64))
		nodePublicID, _ = data["node_id"].(string)
		agentToken, _ = data["access_token"].(string)
		assert.NotZero(t, nodeID)
		assert.NotEmpty(t, nodePublicID)
		assert.Len(t, agentToken, 32)
	})

	t.Run("fetch the site snapshot selected by the heartbeat", func(t *testing.T) {
		headers := map[string]string{"X-Agent-Token": agentToken}
		heartbeat := func(cursor string) protocol.ConfigResource {
			t.Helper()
			rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/nodes/heartbeat", protocol.NodePayload{
				Engine: "pingora", ContractVersion: edgeconfig.ContractVersion, ConfigSyncVersion: protocol.EdgeConfigSyncVersion,
				LatestConfigVersion: cursor, Version: "test",
			}, headers)
			var data struct{ Config protocol.ConfigManifest }
			unmarshalAPIData(t, requireAPIOK(t, rec).Data, &data)
			return data.Config.Resources["sites.v1"]
		}
		resource := heartbeat(initial.Version)
		require.NotNil(t, resource.Metadata)
		assert.Equal(t, configVersion, resource.Revision)
		assert.Equal(t, configChecksum, resource.SHA256)
		assert.Empty(t, resource.InlineJSON)
		known := heartbeat(configVersion)
		assert.Nil(t, known.Metadata)
		assert.Equal(t, resource.SHA256, known.SHA256)

		pull := "/api/v1/agent/config/pull?" + url.Values{
			"key": {"sites.v1"}, "sha256": {known.SHA256},
		}.Encode()
		reply := performJSONRequest(t, engine, http.MethodGet, pull, nil, headers)
		require.Equal(t, http.StatusOK, reply.Code, reply.Body.String())
		assert.Contains(t, reply.Body.String(), "core-chain.example.com")
	})

	t.Run("create apply log for node", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/apply-logs", map[string]any{
			"version":               configVersion,
			"result":                "success",
			"message":               "config applied",
			"checksum":              configChecksum,
			"engine":                "pingora",
			"release_sequence":      releaseSequence,
			"payload_checksum":      configChecksum,
			"main_config_checksum":  "main-checksum",
			"route_config_checksum": "route-checksum",
			"support_file_count":    2,
		}, map[string]string{
			"X-Agent-Token": agentToken,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		assert.Equal(t, nodePublicID, data["node_id"])
		assert.Equal(t, configVersion, data["version"])
		assert.Equal(t, "success", data["result"])
		assert.Equal(t, configChecksum, data["checksum"])
	})

	t.Run("verify apply log listing and node metadata", func(t *testing.T) {
		listRec := performJSONRequest(
			t,
			engine,
			http.MethodGet,
			apiPath("/apply-logs/?node_id="+nodePublicID+"&pageNo=1&pageSize=10"),
			nil,
			map[string]string{
				"X-Access-Token": seed.Token,
			},
		)
		require.Equal(t, http.StatusOK, listRec.Code)

		listResp := requireAPIOK(t, listRec)
		listData := unmarshalAPIMap(t, listResp.Data)
		assert.InDelta(t, float64(1), listData["total"], 1e-9)

		rows, ok := listData["rows"].([]any)
		require.True(t, ok)
		require.Len(t, rows, 1)
		row, ok := rows[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, nodePublicID, row["node_id"])
		assert.Equal(t, configVersion, row["version"])
		assert.Equal(t, "success", row["result"])

		nodeRec := performJSONRequest(t, engine, http.MethodGet, apiPath("/nodes/"), nil, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, nodeRec.Code)
		nodeResp := requireAPIOK(t, nodeRec)

		nodes := unmarshalAPISlice(t, nodeResp.Data)
		require.Len(t, nodes, 1)
		nodeView, ok := nodes[0].(map[string]any)
		require.True(t, ok)
		assert.InDelta(t, float64(nodeID), nodeView["id"], 1e-9)
		assert.Equal(t, nodePublicID, nodeView["node_id"])
		assert.Equal(t, "success", nodeView["latest_apply_result"])
		assert.Equal(t, configChecksum, nodeView["latest_apply_checksum"])
		assert.InDelta(t, float64(2), nodeView["latest_support_file_count"], 1e-9)
	})
	t.Run("retain the last snapshot if compilation fails", func(t *testing.T) {
		var route model.ProxyRoute
		require.NoError(t, db.DB(ctx).Take(&route, proxyRouteID).Error)
		require.NoError(t, db.DB(ctx).Model(&model.ProxyRoute{}).Where("id = ?", proxyRouteID).UpdateColumn("upstreams", "invalid JSON").Error)
		require.Error(t, config_version.RefreshSiteSnapshot(ctx))
		retained, err := repository.GetActivePingoraConfigMetadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, configVersion, retained.Version)

		require.NoError(t, db.DB(ctx).Model(&model.ProxyRoute{}).Where("id = ?", proxyRouteID).UpdateColumn("upstreams", route.Upstreams).Error)
		require.NoError(t, config_version.RefreshSiteSnapshot(ctx))
		retained, err = repository.GetActivePingoraConfigMetadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, configVersion, retained.Version)
	})

	t.Run("automatically clear the last route", func(t *testing.T) {
		require.NoError(t, db.DB(ctx).Model(&model.ProxyRoute{}).Where("id = ?", proxyRouteID).Update("site_name", "updated-site").Error)
		state, err := repository.GetEdgeSnapshotState(ctx)
		require.NoError(t, err)
		require.NoError(t, db.DB(ctx).Model(&model.ProxyRoute{}).Where("id = ?", proxyRouteID).Update("enabled", false).Error)
		stale, err := repository.StoreEdgeSnapshot(ctx, state.SourceRevision, &model.ConfigVersion{PayloadChecksum: "stale"})
		require.NoError(t, err)
		require.False(t, stale)

		var empty *model.ConfigVersion
		require.Eventually(t, func() bool {
			var err error
			empty, err = repository.GetActivePingoraConfigVersion(ctx)
			return err == nil && empty.Version != configVersion
		}, 3*time.Second, 20*time.Millisecond)
		assert.Contains(t, empty.SnapshotJSON, `"routes":[]`)
	})

}
