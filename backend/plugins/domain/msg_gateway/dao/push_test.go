// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dao_test

import (
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/testhelper"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type stubDBService struct{ db *gorm.DB }

func (s stubDBService) GORM() *gorm.DB                { return s.db }
func (s stubDBService) DB(_ context.Context) *gorm.DB { return s.db }
func (s stubDBService) Named(_ string) *gorm.DB       { return s.db }

func TestPushChannelDAO_CRUD(t *testing.T) {
	_ = idgen.Init(1)
	db, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	require.NoError(t, db.AutoMigrate(&entity.PushChannel{}, &entity.PushEvent{}, &entity.PushHistory{}))

	dao.SetDBServiceForTest(stubDBService{db: db})
	t.Cleanup(func() { dao.SetDBServiceForTest(nil) })

	ctx := context.Background()

	ch := entity.PushChannel{
		Name:    "test_webhook",
		Type:    "custom",
		URL:     "https://example.com/hook",
		Enabled: true,
	}
	require.NoError(t, dao.CreatePushChannelRecord(ctx, &ch))
	assert.NotZero(t, ch.ID)

	loaded, err := dao.GetPushChannelByIDRecord(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, "test_webhook", loaded.Name)

	active, err := dao.GetActivePushChannelByName(ctx, "test_webhook")
	require.NoError(t, err)
	assert.Equal(t, ch.ID, active.ID)

	channels, err := dao.ListPushChannelsRecord(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, channels)

	require.NoError(t, dao.DeletePushChannelRecord(ctx, &ch))
	_, err = dao.GetPushChannelByIDRecord(ctx, ch.ID)
	assert.Error(t, err)
}

func TestPushEventDAO_CRUD(t *testing.T) {
	_ = idgen.Init(1)
	db, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	require.NoError(t, db.AutoMigrate(&entity.PushChannel{}, &entity.PushEvent{}, &entity.PushHistory{}))

	dao.SetDBServiceForTest(stubDBService{db: db})
	t.Cleanup(func() { dao.SetDBServiceForTest(nil) })

	ctx := context.Background()

	ev := entity.PushEvent{
		EventKey: "test_event",
		Name:     "测试事件",
		Channels: []string{"test_webhook"},
		Targets:  []string{"admin"},
		Template: `{"title":"Hello"}`,
		Enabled:  true,
	}
	require.NoError(t, dao.CreatePushEventRecord(ctx, &ev))
	assert.NotZero(t, ev.ID)

	loaded, err := dao.GetPushEventByKeyRecord(ctx, "test_event")
	require.NoError(t, err)
	assert.Equal(t, "测试事件", loaded.Name)

	require.NoError(t, dao.UpdatePushEventEnabledRecord(ctx, &ev, false))
	loadedDisabled, err := dao.GetPushEventByIDRecord(ctx, ev.ID)
	require.NoError(t, err)
	assert.False(t, loadedDisabled.Enabled)

	require.NoError(t, dao.DeletePushEventRecord(ctx, &ev))
	_, err = dao.GetPushEventByIDRecord(ctx, ev.ID)
	assert.Error(t, err)
}

func TestPushHistoryDAO_Cleanup(t *testing.T) {
	_ = idgen.Init(1)
	db, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	require.NoError(t, db.AutoMigrate(&entity.PushHistory{}))

	dao.SetDBServiceForTest(stubDBService{db: db})
	t.Cleanup(func() { dao.SetDBServiceForTest(nil) })

	ctx := context.Background()

	now := time.Now()
	oldTime := now.Add(-40 * 24 * time.Hour)
	recentTime := now.Add(-5 * 24 * time.Hour)

	oldHistory := entity.PushHistory{
		EventKey:  "login",
		Channel:   "telegram",
		Target:    "123",
		Title:     "Old login",
		Content:   "Old content",
		Level:     "info",
		Status:    "success",
		CreatedAt: oldTime,
	}
	recentHistory := entity.PushHistory{
		EventKey:  "login",
		Channel:   "telegram",
		Target:    "123",
		Title:     "Recent login",
		Content:   "Recent content",
		Level:     "info",
		Status:    "success",
		CreatedAt: recentTime,
	}
	require.NoError(t, db.Create(&oldHistory).Error)
	require.NoError(t, db.Create(&recentHistory).Error)

	cutoff := now.Add(-30 * 24 * time.Hour)
	deleted, err := dao.DeletePushHistoriesBeforeRecord(ctx, cutoff)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	var count int64
	db.Model(&entity.PushHistory{}).Count(&count)
	assert.Equal(t, int64(1), count)
}
