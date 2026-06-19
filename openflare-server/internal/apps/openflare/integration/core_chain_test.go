// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
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
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.User{},
		&model.AccessToken{},
		&model.Origin{},
		&model.ProxyRoute{},
		&model.ConfigVersion{},
		&model.OpenFlareWAFRuleGroup{},
		&model.OpenFlareWAFRuleGroupBinding{},
		&model.OpenFlareWAFIPGroup{},
		&model.OpenFlareNode{},
		&model.OpenFlareOption{},
		&model.OpenFlareApplyLog{},
	))

	db.SetDB(sqliteDB)
	option.ResetInitializationForTest()
	agent.ResetAuthCacheForTest()

	seed, err := seedAdminWithAccessToken(sqliteDB)
	require.NoError(t, err)

	engine := testhelper.NewTestGinEngine()
	mountOpenFlareTestRoutes(engine)

	cleanup := func() {
		db.SetDB(nil)
		option.ResetInitializationForTest()
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

	var (
		originID       uint
		proxyRouteID   uint
		configVersion  string
		configChecksum string
		nodeID         uint
		nodePublicID   string
		agentToken     string
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
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/proxy-routes/"), map[string]any{
			"site_name":     "core-chain-site",
			"domain":        "core-chain.example.com",
			"origin_id":     originID,
			"origin_scheme": "http",
			"origin_port":   "8080",
			"enabled":       true,
		}, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		proxyRouteID = uint(data["id"].(float64))
		assert.NotZero(t, proxyRouteID)
		assert.Equal(t, "core-chain-site", data["site_name"])
		assert.Equal(t, "core-chain.example.com", data["domain"])
		assert.Equal(t, float64(originID), data["origin_id"])
		assert.Equal(t, "http://origin.core-chain.internal:8080", data["origin_url"])
	})

	t.Run("publish config version", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, apiPath("/config-versions/publish"), nil, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, rec.Code)

		resp := requireAPIOK(t, rec)
		data := unmarshalAPIMap(t, resp.Data)
		configVersion, _ = data["version"].(string)
		configChecksum, _ = data["checksum"].(string)
		assert.NotEmpty(t, configVersion)
		assert.NotEmpty(t, configChecksum)
		assert.Equal(t, true, data["is_active"])

		activeRec := performJSONRequest(t, engine, http.MethodGet, apiPath("/config-versions/active"), nil, map[string]string{
			"X-Access-Token": seed.Token,
		})
		require.Equal(t, http.StatusOK, activeRec.Code)
		activeResp := requireAPIOK(t, activeRec)

		activeData := unmarshalAPIMap(t, activeResp.Data)
		assert.Equal(t, configVersion, activeData["version"])
		assert.Equal(t, configChecksum, activeData["checksum"])
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

	t.Run("create apply log for node", func(t *testing.T) {
		rec := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/apply-logs", map[string]any{
			"version":               configVersion,
			"result":                "success",
			"message":               "config applied",
			"checksum":              configChecksum,
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
		assert.Equal(t, float64(1), listData["total"])

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
		assert.Equal(t, float64(nodeID), nodeView["id"])
		assert.Equal(t, nodePublicID, nodeView["node_id"])
		assert.Equal(t, "success", nodeView["latest_apply_result"])
		assert.Equal(t, configChecksum, nodeView["latest_apply_checksum"])
		assert.Equal(t, float64(2), nodeView["latest_support_file_count"])
	})
}