// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package origin

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

func setupOriginTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.Origin{}))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
}

func TestCreateOrigin(t *testing.T) {
	cleanup := setupOriginTestDB(t)
	defer cleanup()
	ctx := context.Background()

	origin, err := CreateOrigin(ctx, Input{
		Name:    "Primary Origin",
		Address: "origin-a.internal",
		Remark:  "main upstream",
	})
	require.NoError(t, err)
	assert.NotZero(t, origin.ID)
	assert.Equal(t, "Primary Origin", origin.Name)
	assert.Equal(t, "origin-a.internal", origin.Address)
	assert.Equal(t, "main upstream", origin.Remark)

	_, err = CreateOrigin(ctx, Input{
		Address: "origin-a.internal",
	})
	require.Error(t, err)
	assert.Equal(t, errOriginAddressExists, err.Error())
}

func TestListOrigins(t *testing.T) {
	cleanup := setupOriginTestDB(t)
	defer cleanup()
	ctx := context.Background()

	first, err := CreateOrigin(ctx, Input{
		Name:    "first-origin",
		Address: "origin-a.internal",
	})
	require.NoError(t, err)

	second, err := CreateOrigin(ctx, Input{
		Name:    "second-origin",
		Address: "origin-b.internal",
	})
	require.NoError(t, err)

	origins, err := ListOrigins(ctx)
	require.NoError(t, err)
	require.Len(t, origins, 2)
	assert.Equal(t, second.ID, origins[0].ID)
	assert.Equal(t, first.ID, origins[1].ID)
	assert.Equal(t, int64(0), origins[0].RouteCount)
	assert.Equal(t, int64(0), origins[1].RouteCount)
}
