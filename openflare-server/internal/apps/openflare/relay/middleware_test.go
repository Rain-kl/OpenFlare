// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRelayMiddlewareTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.OpenFlareNode{}))
	db.SetDB(sqliteDB)

	return func() {
		db.SetDB(nil)
	}
}

func seedRelayNode(t *testing.T, nodeType, accessToken string) *model.OpenFlareNode {
	t.Helper()
	ctx := context.Background()
	node := &model.OpenFlareNode{
		NodeID:      "relay-test-node",
		Name:        "relay-test",
		Status:      "pending",
		NodeType:    nodeType,
		AccessToken: accessToken,
	}
	require.NoError(t, model.CreateOpenFlareNode(ctx, node))
	return node
}

func TestRelayAuthMissingToken(t *testing.T) {
	cleanup := setupRelayMiddlewareTestDB(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/relay/test", RelayAuth(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/relay/test", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRelayAuthRejectsWrongNodeType(t *testing.T) {
	cleanup := setupRelayMiddlewareTestDB(t)
	defer cleanup()
	seedRelayNode(t, "edge_node", "edge-token-relay")

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/relay/test", RelayAuth(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/relay/test", nil)
	req.Header.Set("X-Agent-Token", "edge-token-relay")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRelayAuthAcceptsTunnelRelay(t *testing.T) {
	cleanup := setupRelayMiddlewareTestDB(t)
	defer cleanup()
	node := seedRelayNode(t, "tunnel_relay", "relay-token-valid")

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/relay/test", RelayAuth(), func(c *gin.Context) {
		authNode, ok := c.Get(ctxRelayNodeKey)
		require.True(t, ok)
		assert.Equal(t, node.NodeID, authNode.(*model.OpenFlareNode).NodeID)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/relay/test", nil)
	req.Header.Set("X-Agent-Token", "relay-token-valid")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
