package model

import (
	"context"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupApplyLogModelTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&OpenFlareApplyLog{}))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
}

func TestIsRepeatSuccessApplyLog(t *testing.T) {
	latest := &OpenFlareApplyLog{
		Version:  "20260615-001",
		Checksum: "checksum-a",
		Result:   "success",
	}

	assert.True(t, IsRepeatSuccessApplyLog(latest, "20260615-001", "checksum-a", "success"))
	assert.False(t, IsRepeatSuccessApplyLog(latest, "20260615-002", "checksum-a", "success"))
	assert.False(t, IsRepeatSuccessApplyLog(latest, "20260615-001", "checksum-b", "success"))
	assert.False(t, IsRepeatSuccessApplyLog(latest, "20260615-001", "checksum-a", "failed"))
	assert.False(t, IsRepeatSuccessApplyLog(nil, "20260615-001", "checksum-a", "success"))
}

func TestGetLatestOpenFlareApplyLogByNodeID(t *testing.T) {
	cleanup := setupApplyLogModelTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, db.DB(ctx).Create(&OpenFlareApplyLog{
		NodeID:    "node-1",
		Version:   "v1",
		Result:    "success",
		Checksum:  "checksum-1",
		CreatedAt: now.Add(-time.Hour),
	}).Error)
	require.NoError(t, db.DB(ctx).Create(&OpenFlareApplyLog{
		NodeID:    "node-1",
		Version:   "v2",
		Result:    "success",
		Checksum:  "checksum-2",
		CreatedAt: now,
	}).Error)

	latest, err := GetLatestOpenFlareApplyLogByNodeID(ctx, "node-1")
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "v2", latest.Version)

	missing, err := GetLatestOpenFlareApplyLogByNodeID(ctx, "node-missing")
	require.NoError(t, err)
	assert.Nil(t, missing)
}
