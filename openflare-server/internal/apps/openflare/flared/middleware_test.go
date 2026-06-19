// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package flared

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

func setupFlaredMiddlewareTestDB(t *testing.T) func() {
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

func seedFlaredNode(t *testing.T, nodeType, accessToken string) *model.OpenFlareNode {
	t.Helper()
	ctx := context.Background()
	node := &model.OpenFlareNode{
		NodeID:      "flared-test-node",
		Name:        "flared-test",
		Status:      "pending",
		NodeType:    nodeType,
		AccessToken: accessToken,
	}
	require.NoError(t, model.CreateOpenFlareNode(ctx, node))
	return node
}

func TestTunnelAuthMissingToken(t *testing.T) {
	cleanup := setupFlaredMiddlewareTestDB(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/flared/test", TunnelAuth(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/flared/test", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestTunnelAuthRejectsWrongNodeType(t *testing.T) {
	cleanup := setupFlaredMiddlewareTestDB(t)
	defer cleanup()
	seedFlaredNode(t, "edge_node", "edge-token-flared")

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/flared/test", TunnelAuth(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/flared/test", nil)
	req.Header.Set("X-Tunnel-Token", "edge-token-flared")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTunnelAuthAcceptsTunnelClient(t *testing.T) {
	cleanup := setupFlaredMiddlewareTestDB(t)
	defer cleanup()
	node := seedFlaredNode(t, "tunnel_client", "tunnel-token-valid")

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/flared/test", TunnelAuth(), func(c *gin.Context) {
		authNode, ok := c.Get(ctxFlaredNodeKey)
		require.True(t, ok)
		assert.Equal(t, node.NodeID, authNode.(*model.OpenFlareNode).NodeID)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/flared/test", nil)
	req.Header.Set("X-Tunnel-Token", "tunnel-token-valid")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
