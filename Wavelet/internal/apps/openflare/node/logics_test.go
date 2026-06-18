// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/option"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupNodeTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareNode{},
		&model.OpenFlareOption{},
		&model.OpenFlareApplyLog{},
	))

	db.SetDB(sqliteDB)
	option.ResetInitializationForTest()

	return func() {
		db.SetDB(nil)
		option.ResetInitializationForTest()
	}
}

func TestCreateEdgeNode(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	view, err := CreateNode(ctx, Input{
		Name:              "edge-1",
		IP:                "10.0.0.1",
		AutoUpdateEnabled: true,
	})
	require.NoError(t, err)
	assert.NotZero(t, view.ID)
	assert.True(t, strings.HasPrefix(view.NodeID, "node-"))
	assert.Len(t, view.AccessToken, 32)
	assert.Equal(t, "edge_node", view.NodeType)
	assert.Equal(t, nodeStatusPending, view.Status)
	assert.True(t, view.AutoUpdateEnabled)
}

func TestCreateTunnelRelayNode(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	view, err := CreateNode(ctx, Input{
		Name:     "relay-1",
		NodeType: "tunnel_relay",
	})
	require.NoError(t, err)
	assert.Equal(t, "tunnel_relay", view.NodeType)
	assert.Equal(t, 7000, view.RelayBindPort)
	assert.Equal(t, 8080, view.RelayVhostHTTPPort)

	stored, err := model.GetOpenFlareNodeByID(ctx, view.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, stored.RelayAuthToken)
}

func TestCreateTunnelClientNode(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	view, err := CreateNode(ctx, Input{
		Name:     "client-1",
		NodeType: "tunnel_client",
	})
	require.NoError(t, err)
	assert.Equal(t, "tunnel_client", view.NodeType)
}

func TestCreateNodeRequiresName(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := CreateNode(ctx, Input{IP: "10.0.0.2"})
	require.Error(t, err)
	assert.Equal(t, errNodeNameRequired, err.Error())
}

func TestUpdateNode(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	created, err := CreateNode(ctx, Input{Name: "edge-update"})
	require.NoError(t, err)

	updated, err := UpdateNode(ctx, created.ID, Input{
		Name:              "edge-updated",
		IP:                "192.168.1.10",
		AutoUpdateEnabled: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "edge-updated", updated.Name)
	assert.Equal(t, "192.168.1.10", updated.IP)
	assert.True(t, updated.AutoUpdateEnabled)
}

func TestDeleteNode(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	created, err := CreateNode(ctx, Input{Name: "edge-delete"})
	require.NoError(t, err)

	require.NoError(t, DeleteNode(ctx, created.ID))
	_, err = model.GetOpenFlareNodeByID(ctx, created.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestListNodesWithApplyLogMetadata(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	created, err := CreateNode(ctx, Input{Name: "edge-list"})
	require.NoError(t, err)

	applyAt := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareApplyLog{
		NodeID:              created.NodeID,
		Version:             "20260618-001",
		Result:              "success",
		Message:             "ok",
		Checksum:            "checksum-1",
		MainConfigChecksum:  "main-1",
		RouteConfigChecksum: "route-1",
		SupportFileCount:    3,
		CreatedAt:           applyAt,
	}).Error)

	views, err := ListNodes(ctx)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, "success", views[0].LatestApplyResult)
	assert.Equal(t, "checksum-1", views[0].LatestApplyChecksum)
	assert.Equal(t, 3, views[0].LatestSupportFileCount)
	require.NotNil(t, views[0].LatestApplyAt)
	assert.Equal(t, applyAt, views[0].LatestApplyAt.UTC())
}

func TestBootstrapTokenLifecycle(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	first, err := GetBootstrapToken(ctx)
	require.NoError(t, err)
	assert.Len(t, first.DiscoveryToken, 32)

	second, err := GetBootstrapToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, first.DiscoveryToken, second.DiscoveryToken)

	rotated, err := RotateBootstrapToken(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, first.DiscoveryToken, rotated.DiscoveryToken)
	assert.Equal(t, rotated.DiscoveryToken, model.OptionValue("AgentDiscoveryToken"))
}

func TestValidateDiscoveryToken(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	bootstrap, err := GetBootstrapToken(ctx)
	require.NoError(t, err)

	require.NoError(t, ValidateDiscoveryToken(ctx, bootstrap.DiscoveryToken))
	require.Error(t, ValidateDiscoveryToken(ctx, "invalid-token"))
}

func TestRequestAgentUpdateWithPreviewTag(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	created, err := CreateNode(ctx, Input{Name: "edge-update-agent"})
	require.NoError(t, err)

	originalClient := setReleaseHTTPClientForTest(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			expected := "https://api.github.com/repos/" + model.AgentUpdateRepo + "/releases/tags/v0.5.0-rc.1"
			require.Equal(t, expected, req.URL.String())
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v0.5.0-rc.1","prerelease":true}`)),
			}, nil
		}),
	})
	t.Cleanup(func() {
		setReleaseHTTPClientForTest(originalClient)
	})

	updated, err := RequestAgentUpdate(ctx, created.ID, AgentUpdateInput{
		Channel: "preview",
		TagName: "v0.5.0-rc.1",
	})
	require.NoError(t, err)
	assert.True(t, updated.UpdateRequested)
	assert.Equal(t, "preview", updated.UpdateChannel)
	assert.Equal(t, "v0.5.0-rc.1", updated.UpdateTag)
}

func TestRequestOpenrestyRestart(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	created, err := CreateNode(ctx, Input{Name: "edge-restart"})
	require.NoError(t, err)

	updated, err := RequestOpenrestyRestart(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, updated.RestartOpenrestyRequested)
}

func seedActiveConfigVersion(t *testing.T, ctx context.Context) {
	t.Helper()
	conn := db.DB(ctx)
	require.NotNil(t, conn)
	require.NoError(t, conn.AutoMigrate(&model.ConfigVersion{}))
	require.NoError(t, conn.Create(&model.ConfigVersion{
		Version:        "20260618-001",
		SnapshotJSON:   `{}`,
		RenderedConfig: `server {}`,
		Checksum:       "abc123",
		IsActive:       true,
		CreatedBy:      "test",
	}).Error)
}

func TestRequestForceSyncRequiresWebSocket(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()
	seedActiveConfigVersion(t, ctx)

	created, err := CreateNode(ctx, Input{Name: "edge-sync"})
	require.NoError(t, err)

	_, err = RequestForceSync(ctx, created.ID)
	require.Error(t, err)
	assert.Equal(t, errNodeForceSyncFailed, err.Error())
}

func TestRequestForceSyncRequiresActiveConfig(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()
	conn := db.DB(ctx)
	require.NotNil(t, conn)
	require.NoError(t, conn.AutoMigrate(&model.ConfigVersion{}))

	created, err := CreateNode(ctx, Input{Name: "edge-sync-active"})
	require.NoError(t, err)

	_, err = RequestForceSync(ctx, created.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errNoActiveConfigVersion)
}

func TestGetObservabilityStub(t *testing.T) {
	cleanup := setupNodeTestDB(t)
	defer cleanup()
	ctx := context.Background()

	created, err := CreateNode(ctx, Input{Name: "edge-obs"})
	require.NoError(t, err)

	view, err := GetObservability(ctx, created.ID, ObservabilityQuery{Hours: 24, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, created.NodeID, view.NodeID)
	assert.Empty(t, view.MetricSnapshots)
}

func TestComputeNodeStatus(t *testing.T) {
	now := time.Now()
	pending := &model.OpenFlareNode{}
	assert.Equal(t, nodeStatusPending, computeNodeStatus(pending))

	online := &model.OpenFlareNode{LastSeenAt: &now}
	assert.Equal(t, nodeStatusOnline, computeNodeStatus(online))

	offlineAt := now.Add(-model.NodeOfflineThreshold - time.Minute)
	offline := &model.OpenFlareNode{LastSeenAt: &offlineAt}
	assert.Equal(t, nodeStatusOffline, computeNodeStatus(offline))
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
