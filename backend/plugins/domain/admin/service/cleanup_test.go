// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/service"
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSystemCleanupHandler_Execute(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, sqliteDB.AutoMigrate(&model.TaskExecution{}))

	service.SetDBService(&testDBService{db: sqliteDB})
	defer service.ResetServices()

	now := time.Now()
	oldTime := now.Add(-10 * 24 * time.Hour)
	recentTime := now.Add(-1 * time.Hour)

	// Seed old execution (should be cleaned)
	oldExec := model.TaskExecution{
		ID:        1,
		TaskID:    "task-old",
		TaskType:  "sample_task",
		Status:    "success",
		CreatedAt: oldTime,
	}
	require.NoError(t, sqliteDB.Create(&oldExec).Error)

	// Seed recent execution (should remain)
	recentExec := model.TaskExecution{
		ID:        2,
		TaskID:    "task-recent",
		TaskType:  "sample_task",
		Status:    "success",
		CreatedAt: recentTime,
	}
	require.NoError(t, sqliteDB.Create(&recentExec).Error)

	var eventFired atomic.Bool
	service.SetEventEmitter(func(ctx context.Context, topic string, payload any) error {
		if topic == contracts.EventTopicSystemCleanup {
			eventFired.Store(true)
		}
		return nil
	})

	handler := &service.SystemCleanupHandler{}
	res, err := handler.Execute(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, res.Message, "系统垃圾清理完成")

	// Verify old execution is deleted and recent remains
	var count int64
	sqliteDB.Model(&model.TaskExecution{}).Count(&count)
	assert.Equal(t, int64(1), count)

	var remaining model.TaskExecution
	sqliteDB.First(&remaining)
	assert.Equal(t, uint64(2), remaining.ID)

	assert.True(t, eventFired.Load(), "EventTopicSystemCleanup must be emitted")
}
