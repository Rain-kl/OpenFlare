// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package custom_events

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	"github.com/Rain-kl/Wavelet/internal/listener"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/task"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var registerOnce sync.Once

func ensureRegistered() {
	registerOnce.Do(Register)
}

func setupAdminLoginIntegrationTest(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	dbConn, mr, cleanup := testhelper.SetupTestEnvironment(t)

	err := dbConn.AutoMigrate(
		&model.PushEvent{},
		&model.PushHistory{},
		&model.PushChannel{},
	)
	require.NoError(t, err)

	sysUser := &model.User{
		ID:       999,
		Username: "system",
		Nickname: "系统",
		Password: "*",
		IsActive: true,
	}
	require.NoError(t, dbConn.Create(sysUser).Error)

	task.AsynqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: mr.Addr()})
	task.RegisterHandler(push.SendNotificationTask, &push.PushHandler{})
	task.RegisterTaskMeta(push.SendNotificationMeta)

	ensureRegistered()

	require.NoError(t, push.SyncEvents(context.Background()))

	return dbConn, func() {
		cleanup()
		if task.AsynqClient != nil {
			task.AsynqClient.Close()
			task.AsynqClient = nil
		}
	}
}

func seedMockPushChannel(t *testing.T, dbConn *gorm.DB) *model.PushChannel {
	t.Helper()

	channel := &model.PushChannel{
		Name:    "mock_channel",
		Type:    "custom",
		URL:     "https://webhook.site/admin-login",
		Other:   `{"text": "$content"}`,
		Enabled: true,
	}
	require.NoError(t, dbConn.Create(channel).Error)
	return channel
}

func enableAdminLoginEvent(t *testing.T, dbConn *gorm.DB, channelName string, targets []string) {
	t.Helper()

	var event model.PushEvent
	require.NoError(t, dbConn.Where("event_key = ?", AdminLogin.Key).First(&event).Error)

	event.Enabled = true
	event.Channels = []string{channelName}
	event.Targets = targets
	require.NoError(t, dbConn.Save(&event).Error)
}

func waitForAsyncTrigger(t *testing.T) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
}

func countPushTasks(t *testing.T, dbConn *gorm.DB) int64 {
	t.Helper()

	var count int64
	require.NoError(t, dbConn.Model(&model.TaskExecution{}).
		Where("task_type = ?", push.SendNotificationTask).
		Count(&count).Error)
	return count
}

func TestAdminLoginPushIntegration(t *testing.T) {
	dbConn, cleanup := setupAdminLoginIntegrationTest(t)
	defer cleanup()

	channel := seedMockPushChannel(t, dbConn)
	defer dbConn.Delete(channel)

	enableAdminLoginEvent(t, dbConn, channel.Name, []string{"ops_team"})

	adminUser := &model.User{
		ID:       1001,
		Username: "super_admin",
		IsAdmin:  true,
		IsActive: true,
	}
	require.NoError(t, dbConn.Create(adminUser).Error)

	t.Run("admin login emits push task with user and ip", func(t *testing.T) {
		dbConn.Where("task_type = ?", push.SendNotificationTask).Delete(&model.TaskExecution{})

		listener.EmitAdminLoggedIn(context.Background(), adminUser, "203.0.113.42")
		waitForAsyncTrigger(t)

		var execution model.TaskExecution
		require.NoError(t, dbConn.Where("task_type = ?", push.SendNotificationTask).First(&execution).Error)

		var payload push.SendPayload
		require.NoError(t, json.Unmarshal([]byte(execution.Payload), &payload))

		assert.Equal(t, AdminLogin.Key, payload.EventKey)
		assert.Equal(t, "ops_team", payload.Target)
		assert.Equal(t, "管理员登录提醒", payload.Body.Title)
		assert.Contains(t, payload.Body.Content, "super_admin")
		assert.Contains(t, payload.Body.Content, "203.0.113.42")
	})

	t.Run("non-admin login does not trigger push", func(t *testing.T) {
		dbConn.Where("task_type = ?", push.SendNotificationTask).Delete(&model.TaskExecution{})

		nonAdmin := &model.User{
			ID:       2002,
			Username: "regular_user",
			IsAdmin:  false,
			IsActive: true,
		}
		require.NoError(t, dbConn.Create(nonAdmin).Error)

		listener.EmitAdminLoggedIn(context.Background(), nonAdmin, "198.51.100.1")
		waitForAsyncTrigger(t)

		assert.Equal(t, int64(0), countPushTasks(t, dbConn))
	})

	t.Run("disabled admin login event does not enqueue push", func(t *testing.T) {
		dbConn.Where("task_type = ?", push.SendNotificationTask).Delete(&model.TaskExecution{})

		var event model.PushEvent
		require.NoError(t, dbConn.Where("event_key = ?", AdminLogin.Key).First(&event).Error)
		event.Enabled = false
		require.NoError(t, dbConn.Save(&event).Error)

		listener.EmitAdminLoggedIn(context.Background(), adminUser, "10.0.0.1")
		waitForAsyncTrigger(t)

		assert.Equal(t, int64(0), countPushTasks(t, dbConn))
	})
}
