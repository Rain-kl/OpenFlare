// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

import (
	"context"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupProxyRouteTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.ProxyRoute{}, &model.Origin{}))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
}

func TestCreateProxyRoute(t *testing.T) {
	cleanup := setupProxyRouteTestDB(t)
	defer cleanup()
	ctx := context.Background()

	view, err := CreateProxyRoute(ctx, Input{
		SiteName:  "example-site",
		Domain:    "example.com",
		OriginURL: "http://origin.example.com:8080",
		Enabled:   true,
	})
	require.NoError(t, err)
	assert.NotZero(t, view.ID)
	assert.Equal(t, "example-site", view.SiteName)
	assert.Equal(t, "example.com", view.Domain)
	assert.Equal(t, []string{"example.com"}, view.Domains)
	assert.Equal(t, "http://origin.example.com:8080", view.OriginURL)
	assert.Equal(t, []string{"http://origin.example.com:8080"}, view.UpstreamList)
	assert.True(t, view.Enabled)

	_, err = CreateProxyRoute(ctx, Input{
		SiteName:  "duplicate-site",
		Domain:    "example.com",
		OriginURL: "http://origin.example.com:8080",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestListProxyRoutes(t *testing.T) {
	cleanup := setupProxyRouteTestDB(t)
	defer cleanup()
	ctx := context.Background()

	first, err := CreateProxyRoute(ctx, Input{
		SiteName:  "first-site",
		Domain:    "first.example.com",
		OriginURL: "http://origin-a.internal:80",
	})
	require.NoError(t, err)

	second, err := CreateProxyRoute(ctx, Input{
		SiteName:  "second-site",
		Domain:    "second.example.com",
		OriginURL: "http://origin-b.internal:80",
	})
	require.NoError(t, err)

	routes, err := ListProxyRoutes(ctx)
	require.NoError(t, err)
	require.Len(t, routes, 2)
	assert.Equal(t, second.ID, routes[0].ID)
	assert.Equal(t, first.ID, routes[1].ID)
	assert.Equal(t, "second.example.com", routes[0].Domain)
	assert.Equal(t, "first.example.com", routes[1].Domain)
}
