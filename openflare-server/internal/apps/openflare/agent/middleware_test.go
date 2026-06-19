// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/option"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAgentAuthTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareNode{},
		&model.OpenFlareOption{},
	))

	db.SetDB(sqliteDB)
	option.ResetInitializationForTest()
	tokenCache.reset()

	return func() {
		db.SetDB(nil)
		option.ResetInitializationForTest()
		tokenCache.reset()
	}
}

func TestAuthenticateAccessToken(t *testing.T) {
	cleanup := setupAgentAuthTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareNode{
		NodeID:      "node-auth-1",
		Name:        "edge",
		AccessToken: "valid-agent-token",
		Status:      nodeStatusOnline,
		LastSeenAt:  &now,
		NodeType:    "edge_node",
	}).Error)

	t.Run("valid token", func(t *testing.T) {
		node, err := AuthenticateAccessToken(ctx, "valid-agent-token")
		require.NoError(t, err)
		assert.Equal(t, "node-auth-1", node.NodeID)
	})

	t.Run("cached token", func(t *testing.T) {
		originalLoader := tokenCache.loadNodeByToken
		t.Cleanup(func() {
			tokenCache.loadNodeByToken = originalLoader
		})
		tokenCache.loadNodeByToken = func(context.Context, string) (*model.OpenFlareNode, error) {
			t.Fatal("db should not be queried for cached token")
			return nil, nil
		}
		node, err := AuthenticateAccessToken(ctx, "valid-agent-token")
		require.NoError(t, err)
		assert.Equal(t, "node-auth-1", node.NodeID)
	})

	t.Run("missing token", func(t *testing.T) {
		_, err := AuthenticateAccessToken(ctx, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), errMissingAgentToken)
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := AuthenticateAccessToken(ctx, "invalid-token")
		require.Error(t, err)
	})
}

func TestAgentAuthMiddleware(t *testing.T) {
	cleanup := setupAgentAuthTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareNode{
		NodeID:      "node-mw-1",
		Name:        "edge",
		AccessToken: "middleware-token",
		Status:      nodeStatusOnline,
		LastSeenAt:  &now,
		NodeType:    "edge_node",
	}).Error)

	router := testhelper.NewTestGinEngine()
	router.GET("/protected", AgentAuth(), func(c *gin.Context) {
		node, ok := AgentNodeFromContext(c)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, response.OK(gin.H{"node_id": node.NodeID}))
	})

	t.Run("authorized request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set(agentTokenHeader, "middleware-token")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code)
		var apiResp response.Any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &apiResp))
		assert.Empty(t, apiResp.ErrorMsg)
	})

	t.Run("unauthorized request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set(agentTokenHeader, "bad-token")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})
}

func TestAgentRegisterAuthMiddleware(t *testing.T) {
	cleanup := setupAgentAuthTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareNode{
		NodeID:      "node-register-1",
		Name:        "edge",
		AccessToken: "existing-node-token",
		Status:      nodeStatusOnline,
		LastSeenAt:  &now,
		NodeType:    "edge_node",
	}).Error)
	require.NoError(t, model.UpdateOpenFlareOption(ctx, "AgentDiscoveryToken", "discovery-token"))

	router := testhelper.NewTestGinEngine()
	router.POST("/register", AgentRegisterAuth(), func(c *gin.Context) {
		if node, ok := AgentNodeFromContext(c); ok {
			c.JSON(http.StatusOK, response.OK(gin.H{"mode": "node", "node_id": node.NodeID}))
			return
		}
		if _, ok := c.Get("discovery_enabled"); ok {
			c.JSON(http.StatusOK, response.OK(gin.H{"mode": "discovery"}))
			return
		}
		c.Status(http.StatusInternalServerError)
	})

	t.Run("existing node token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/register", nil)
		req.Header.Set(agentTokenHeader, "existing-node-token")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code)
		var apiResp response.Any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &apiResp))
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "node", data["mode"])
	})

	t.Run("discovery token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/register", nil)
		req.Header.Set(agentTokenHeader, "discovery-token")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code)
		var apiResp response.Any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &apiResp))
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "discovery", data["mode"])
	})
}