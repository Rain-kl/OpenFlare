// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/repository"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBuildSnapshotRoutesPages(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()
	conn := requireDB(t, ctx)

	project := &model.PagesProject{
		Name:               "Speed Test",
		Slug:               "speedtest",
		Enabled:            true,
		SPAFallbackEnabled: true,
		SPAFallbackPath:    "/index.html",
		RootDir:            "public/site",
		EntryFile:          "index.html",
	}
	require.NoError(t, conn.Create(project).Error)

	deployment := &model.PagesDeployment{
		ProjectID:        project.ID,
		DeploymentNumber: 1,
		Checksum:         "abc123checksum",
		Status:           model.PagesDeploymentStatusActive,
		FileCount:        1,
		TotalSize:        12,
	}
	require.NoError(t, conn.Create(deployment).Error)
	require.NoError(t, conn.Model(project).Update("active_deployment_id", deployment.ID).Error)

	route := &model.ProxyRoute{
		SiteName:       "speedtest",
		OriginURL:      "openflare-pages://project/1",
		Upstreams:      `["openflare-pages://project/1"]`,
		Enabled:        true,
		UpstreamType:   "pages",
		PagesProjectID: &project.ID,
	}
	require.NoError(t, repository.CreateProxyRouteRecord(ctx, route))
	createSnapshotZoneDomains(t, ctx, route, "speedtest.arctel.net")

	bundle, err := buildCurrentConfigBundle(ctx, true)
	require.NoError(t, err)
	require.Len(t, bundle.SnapshotRoutes, 1)

	snapshotRoute := bundle.SnapshotRoutes[0]
	assert.Equal(t, "pages", snapshotRoute.UpstreamType)
	assert.Equal(t, "openflare-pages://project/1", snapshotRoute.OriginURL)
	require.NotNil(t, snapshotRoute.PagesDeployment)
	assert.Equal(t, deployment.ID, snapshotRoute.PagesDeployment.DeploymentID)
	assert.Equal(t, deployment.Checksum, snapshotRoute.PagesDeployment.Checksum)
	assert.Equal(t, "__OPENFLARE_PAGES_DIR__/projects/1/current/public/site", snapshotRoute.PagesDeployment.LocalRoot)

	_, err = renderSnapshotConfig(bundle.SnapshotJSON, nil)
	require.NoError(t, err)

	var decoded struct {
		Routes []struct {
			PagesDeployment *openrestyrender.PagesDeployment `json:"pages_deployment"`
		} `json:"routes"`
	}
	require.NoError(t, json.Unmarshal([]byte(bundle.SnapshotJSON), &decoded))
	require.NotNil(t, decoded.Routes[0].PagesDeployment)
}

func TestBuildSnapshotPagesDeploymentRejectsUnsafeStoredPaths(t *testing.T) {
	deployment := &model.PagesDeployment{ID: 1, ProjectID: 1, Checksum: "checksum"}
	for _, project := range []*model.PagesProject{
		{ID: 1, RootDir: "../escape", EntryFile: "index.html"},
		{ID: 1, RootDir: "public", EntryFile: "/index.html"},
	} {
		_, err := buildSnapshotPagesDeployment(project, deployment)
		require.Error(t, err)
	}
}

func requireDB(t *testing.T, ctx context.Context) *gorm.DB {
	t.Helper()
	conn := db.DB(ctx)
	require.NotNil(t, conn)
	require.NoError(t, conn.AutoMigrate(
		&model.PagesProject{},
		&model.PagesDeployment{},
	))
	return conn
}
