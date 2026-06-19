// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/task"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	pkgpush "github.com/Rain-kl/Wavelet/pkg/push"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

var adminLoginEvent = EventMetadata{
	Key:  "admin_login",
	Name: "管理员登录",
	DefaultTemplate: NotificationMessage{
		Title:   "管理员登录提醒",
		Content: "管理员 {{user.username}} 于 {{time}} 从 IP {{ip}} 登录系统。",
		Level:   "INFO",
	},
	Description: "当管理员成功登录系统时触发此通知",
}

func init() {
	RegisterBuiltInEvent(adminLoginEvent)
}

// mockPusher mock implementation of pkgpush.Pusher
type mockPusher struct {
	mu       sync.Mutex
	sentBody map[string]any
	sentTgt  string
}

func (m *mockPusher) Send(ctx context.Context, cfg pkgpush.Config, target string, body map[string]any, template string, ext map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentBody = body
	m.sentTgt = target
	return nil
}

func (m *mockPusher) ValidateConfig(cfg pkgpush.Config) error {
	return nil
}

func setupPushTest(t *testing.T) (*gorm.DB, *miniredis.Miniredis, func()) {
	dbConn, mr, cleanup := testhelper.SetupTestEnvironment(t)

	// AutoMigrate push tables in SQLite test environment
	err := dbConn.AutoMigrate(&model.PushEvent{}, &model.PushHistory{}, &model.User{}, &model.PushChannel{}, &model.SystemConfig{})
	require.NoError(t, err)

	// 写入数据库系统默认用户 Seed 记录
	sysUser := &model.User{
		ID:       999,
		Username: "system",
		Nickname: "系统",
		Password: "*",
		IsActive: true,
	}
	err = dbConn.Create(sysUser).Error
	require.NoError(t, err)

	// Initialize AsynqClient pointing to miniredis
	task.AsynqClient = asynq.NewClient(asynq.RedisClientOpt{
		Addr: mr.Addr(),
	})

	// Register the task handler and metadata
	task.RegisterHandler(SendNotificationTask, &PushHandler{})
	task.RegisterTaskMeta(SendNotificationMeta)

	return dbConn, mr, func() {
		cleanup()
		if task.AsynqClient != nil {
			task.AsynqClient.Close()
			task.AsynqClient = nil
		}
	}
}

func setupTestRouter(authUser *model.User) *gin.Engine {
	r := testhelper.NewTestGinEngine()
	adminGroup := r.Group("/api/v1/admin/push")

	adminGroup.Use(func(c *gin.Context) {
		if authUser != nil {
			oauth.SetToContext(c, "user_obj", authUser)
		}
		c.Next()
	})

	adminGroup.GET("/events", ListEvents)
	adminGroup.GET("/events/builtin", ListBuiltInEvents)
	adminGroup.POST("/events", CreateEvent)
	adminGroup.PUT("/events/:id", UpdateEvent)
	adminGroup.DELETE("/events/:id", DeleteEvent)
	adminGroup.POST("/events/:id/toggle", ToggleEvent)
	adminGroup.GET("/histories", ListHistories)
	adminGroup.POST("/test", TestPush)

	return r
}

func TestSyncEvents(t *testing.T) {
	dbConn, _, cleanup := setupPushTest(t)
	defer cleanup()

	// 1. SyncEvents first time
	err := SyncEvents(context.Background())
	require.NoError(t, err)

	// Verify event exists in DB
	var event model.PushEvent
	err = dbConn.Where("event_key = ?", "admin_login").First(&event).Error
	require.NoError(t, err)
	assert.Equal(t, "管理员登录", event.Name)
	assert.False(t, event.Enabled)

	// Verify DefaultTemplate matches GORM template field
	var defaultMsg NotificationMessage
	err = json.Unmarshal([]byte(event.Template), &defaultMsg)
	require.NoError(t, err)
	assert.Equal(t, adminLoginEvent.DefaultTemplate.Title, defaultMsg.Title)
	assert.Equal(t, adminLoginEvent.DefaultTemplate.Content, defaultMsg.Content)
}

func TestEventTrigger(t *testing.T) {
	dbConn, _, cleanup := setupPushTest(t)
	defer cleanup()

	// SyncEvents
	err := SyncEvents(context.Background())
	require.NoError(t, err)

	t.Run("trigger disabled event silently ignored", func(t *testing.T) {
		body := map[string]any{
			"user": map[string]any{"username": "test_admin"},
			"ip":   "127.0.0.1",
		}
		DefaultTrigger.Trigger(context.Background(), adminLoginEvent, body)

		// Sleep briefly since Trigger runs in goroutine
		time.Sleep(50 * time.Millisecond)

		// Verify no tasks enqueued in TaskExecution GORM table
		var count int64
		dbConn.Model(&model.TaskExecution{}).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("trigger enabled event enqueues task", func(t *testing.T) {
		// Create an enabled custom channel in GORM
		customChan := &model.PushChannel{
			Name:    "mock_channel",
			Type:    "custom",
			URL:     "https://webhook.site/trigger",
			Other:   `{"text": "$content"}`,
			Enabled: true,
		}
		err = dbConn.Create(customChan).Error
		require.NoError(t, err)
		defer dbConn.Delete(customChan)

		// Enable the push event in DB using struct to trigger JSON serializer
		var event model.PushEvent
		err = dbConn.Where("event_key = ?", "admin_login").First(&event).Error
		require.NoError(t, err)

		event.Enabled = true
		event.Channels = []string{"mock_channel"}
		event.Targets = []string{"admin_user"}
		err = dbConn.Save(&event).Error
		require.NoError(t, err)

		// Trigger
		body := map[string]any{
			"user": map[string]any{
				"username": "super_admin",
			},
			"ip":   "1.1.1.1",
			"time": "2026-06-14 18:00:00",
		}
		DefaultTrigger.Trigger(context.Background(), adminLoginEvent, body)

		// Wait for goroutine execution
		time.Sleep(50 * time.Millisecond)

		// Verify TaskExecution enqueued record
		var execution model.TaskExecution
		err = dbConn.Where("task_type = ?", SendNotificationTask).First(&execution).Error
		require.NoError(t, err)

		// Verify enqueued payload structure
		var payload SendPayload
		err = json.Unmarshal([]byte(execution.Payload), &payload)
		require.NoError(t, err)
		assert.Equal(t, "admin_login", payload.EventKey)
		assert.Equal(t, "custom", payload.Config.Channel)
		assert.Equal(t, "https://webhook.site/trigger", payload.Config.URL)
		assert.Equal(t, "admin_user", payload.Target)
		assert.Equal(t, "管理员登录提醒", payload.Body.Title)
		assert.Contains(t, payload.Body.Content, "super_admin")
		assert.Contains(t, payload.Body.Content, "1.1.1.1")
	})

	t.Run("trigger without user injects virtual system user", func(t *testing.T) {
		// Create an enabled custom channel in GORM
		customChan := &model.PushChannel{
			Name:    "mock_channel",
			Type:    "custom",
			URL:     "https://webhook.site/trigger",
			Other:   `{"text": "$content"}`,
			Enabled: true,
		}
		err = dbConn.Create(customChan).Error
		require.NoError(t, err)
		defer dbConn.Delete(customChan)

		// Enable the push event in DB
		var event model.PushEvent
		err = dbConn.Where("event_key = ?", "admin_login").First(&event).Error
		require.NoError(t, err)

		// 清理旧任务执行记录
		dbConn.Where("task_type = ?", SendNotificationTask).Delete(&model.TaskExecution{})

		event.Enabled = true
		event.Channels = []string{"mock_channel"}
		event.Targets = []string{"user.username"} // 动态目标
		err = dbConn.Save(&event).Error
		require.NoError(t, err)

		// Trigger with empty body (simulates cron scheduler triggering)
		DefaultTrigger.Trigger(context.Background(), adminLoginEvent, nil)

		// Wait for goroutine execution
		time.Sleep(50 * time.Millisecond)

		// Verify TaskExecution enqueued record
		var execution model.TaskExecution
		err = dbConn.Where("task_type = ?", SendNotificationTask).First(&execution).Error
		require.NoError(t, err)

		var payload SendPayload
		err = json.Unmarshal([]byte(execution.Payload), &payload)
		require.NoError(t, err)

		// 检查 payload 是否将 target (user.username) 成功替换为 "system"
		assert.Equal(t, "system", payload.Target)
		// 检查 payload 中的 Content，应当被替换为 "system" 变量
		assert.Contains(t, payload.Body.Content, "system")
	})
}

func TestPushHandler(t *testing.T) {
	dbConn, _, cleanup := setupPushTest(t)
	defer cleanup()

	mPusher := &mockPusher{}
	pkgpush.Register("mock_channel", mPusher)

	handler := &PushHandler{}

	payload := SendPayload{
		EventKey: "admin_login",
		Config: pkgpush.Config{
			Channel: "mock_channel",
			URL:     "http://mock-url",
		},
		Target: "admin_user",
		Body: NotificationMessage{
			Title:   "Structured Alert",
			Content: "Hello World",
			Level:   "WARNING",
			Ext:     map[string]any{"extra_val": 42},
		},
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	t.Run("validate payload", func(t *testing.T) {
		validated, valErr := handler.ValidatePayload(payloadBytes)
		require.NoError(t, valErr)
		assert.NotEmpty(t, validated)
	})

	t.Run("execute task successfully", func(t *testing.T) {
		res, execErr := handler.Execute(context.Background(), payloadBytes)
		require.NoError(t, execErr)
		assert.Contains(t, res.Message, "推送成功")

		// Verify mock pusher received flattened variables
		mPusher.mu.Lock()
		assert.Equal(t, "admin_user", mPusher.sentTgt)
		assert.Equal(t, "Structured Alert", mPusher.sentBody["title"])
		assert.Equal(t, "Hello World", mPusher.sentBody["content"])
		assert.Equal(t, "WARNING", mPusher.sentBody["level"])
		assert.Equal(t, float64(42), mPusher.sentBody["extra_val"]) // unmarshaled json numbers are float64 by default
		mPusher.mu.Unlock()

		// Verify PushHistory recorded
		var history model.PushHistory
		err = dbConn.First(&history).Error
		require.NoError(t, err)
		assert.Equal(t, "admin_login", history.EventKey)
		assert.Equal(t, "mock_channel", history.Channel)
		assert.Equal(t, "success", history.Status)
		assert.Equal(t, "Structured Alert", history.Title)
	})
}

func TestPushRouters(t *testing.T) {
	dbConn, _, cleanup := setupPushTest(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	r := setupTestRouter(adminUser)

	// Sync events to populate db
	err := SyncEvents(context.Background())
	require.NoError(t, err)

	t.Run("list events", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/push/events", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Any
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		dataBytes, _ := json.Marshal(resp.Data)
		var events []model.PushEvent
		err = json.Unmarshal(dataBytes, &events)
		require.NoError(t, err)

		assert.Len(t, events, 1)
		assert.Equal(t, "admin_login", events[0].EventKey)
	})

	t.Run("toggle event status", func(t *testing.T) {
		var event model.PushEvent
		dbConn.First(&event)

		// 1. 未配置任何渠道时开启，应该被拒绝
		req, _ := http.NewRequest("POST", "/api/v1/admin/push/events/"+strconv.FormatUint(event.ID, 10)+"/toggle", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 2. 为该事件关联渠道后，再切换开启，应当成功
		event.Channels = []string{"email"}
		dbConn.Save(&event)

		req2, _ := http.NewRequest("POST", "/api/v1/admin/push/events/"+strconv.FormatUint(event.ID, 10)+"/toggle", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		var updated model.PushEvent
		dbConn.First(&updated)
		assert.True(t, updated.Enabled)
	})

	t.Run("update event", func(t *testing.T) {
		var event model.PushEvent
		dbConn.First(&event)

		updateReq := UpdateEventRequest{
			Channels: []string{"email"},
			Targets:  []string{"user@test.com"},
			Template: `{"title": "Custom Login Alert", "content": "Alert", "level": "WARNING"}`,
			Enabled:  true,
		}
		bodyBytes, _ := json.Marshal(updateReq)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/push/events/"+strconv.FormatUint(event.ID, 10), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var updated model.PushEvent
		dbConn.First(&updated)
		assert.Equal(t, []string{"email"}, updated.Channels)
		assert.Equal(t, []string{"user@test.com"}, updated.Targets)
		assert.Contains(t, updated.Template, "Custom Login Alert")
	})

	t.Run("list push histories", func(t *testing.T) {
		// Populate history record
		hist := model.PushHistory{
			EventKey: "admin_login",
			Channel:  "email",
			Target:   "user@test.com",
			Title:    "Custom Login Alert",
			Content:  "Alert",
			Level:    "WARNING",
			Status:   "success",
		}
		dbConn.Create(&hist)

		req, _ := http.NewRequest("GET", "/api/v1/admin/push/histories?page=1&page_size=10", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)

		dataMap, ok := resp.Data.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, float64(1), dataMap["total"])
	})

	t.Run("test push endpoint", func(t *testing.T) {
		mPusher := &mockPusher{}
		pkgpush.Register("test_channel", mPusher)

		testReq := TestPushRequest{
			Config: pkgpush.Config{
				Channel: "test_channel",
				URL:     "http://test-url",
			},
			Target: "test_target",
		}
		bodyBytes, _ := json.Marshal(testReq)
		req, _ := http.NewRequest("POST", "/api/v1/admin/push/test", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("list built-in events", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/push/events/builtin", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		builtins, ok := resp.Data.([]any)
		assert.True(t, ok)
		assert.NotEmpty(t, builtins)
	})

	t.Run("create and delete push event", func(t *testing.T) {
		// Clean up any existing admin_login event first
		dbConn.Where("event_key = ?", "admin_login").Delete(&model.PushEvent{})

		// 1. Create event
		createReq := CreateEventRequest{
			EventKey: "admin_login",
			Channels: []string{"email"},
			Targets:  []string{"admin@test.com"},
			Enabled:  true,
		}
		bodyBytes, _ := json.Marshal(createReq)
		req, _ := http.NewRequest("POST", "/api/v1/admin/push/events", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify created in DB
		var event model.PushEvent
		err := dbConn.Where("event_key = ?", "admin_login").First(&event).Error
		require.NoError(t, err)
		assert.Equal(t, "admin_login", event.EventKey)
		assert.Equal(t, "管理员登录", event.Name)
		assert.True(t, event.Enabled)

		// 2. Try creating again (should fail)
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("POST", "/api/v1/admin/push/events", bytes.NewBuffer(bodyBytes))
		req2.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusBadRequest, w2.Code)

		// 3. Delete event
		w3 := httptest.NewRecorder()
		req3, _ := http.NewRequest("DELETE", "/api/v1/admin/push/events/"+strconv.FormatUint(event.ID, 10), nil)
		r.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusOK, w3.Code)

		// Verify deleted from DB
		var count int64
		dbConn.Model(&model.PushEvent{}).Where("event_key = ?", "admin_login").Count(&count)
		assert.Equal(t, int64(0), count)
	})
}

func TestResolveTarget(t *testing.T) {
	dbConn, _, cleanup := setupPushTest(t)
	defer cleanup()

	// 1. 创建测试用户与管理员用户
	testUser := &model.User{
		ID:       9999,
		Username: "target_user",
		Email:    "target@test.com",
		IsAdmin:  false,
	}
	err := dbConn.Create(testUser).Error
	require.NoError(t, err)

	adminUser := &model.User{
		ID:       8888,
		Username: "admin_user",
		Email:    "admin@test.com",
		IsAdmin:  true,
	}
	err = dbConn.Create(adminUser).Error
	require.NoError(t, err)

	flatBody := map[string]any{
		"user.id":       float64(9999), // JSON 反序列化后一般是 float64
		"user.username": "target_user",
		"user.email":    "target@test.com",
	}

	ctx := context.Background()

	t.Run("dynamic user.id resolved and converted for email channel", func(t *testing.T) {
		res := resolveTarget(ctx, "user.id", flatBody, "email")
		assert.Equal(t, "target@test.com", res)
	})

	t.Run("dynamic user.username resolved and converted for email channel", func(t *testing.T) {
		res := resolveTarget(ctx, "user.username", flatBody, "email")
		assert.Equal(t, "target@test.com", res)
	})

	t.Run("dynamic user.email resolved directly for email channel", func(t *testing.T) {
		res := resolveTarget(ctx, "user.email", flatBody, "email")
		assert.Equal(t, "target@test.com", res)
	})

	t.Run("fixed user id resolved and converted for email channel", func(t *testing.T) {
		res := resolveTarget(ctx, "9999", flatBody, "email")
		assert.Equal(t, "target@test.com", res)
	})

	t.Run("fixed username resolved and converted for email channel", func(t *testing.T) {
		res := resolveTarget(ctx, "target_user", flatBody, "email")
		assert.Equal(t, "target@test.com", res)
	})

	t.Run("fixed email address resolved directly for email channel", func(t *testing.T) {
		res := resolveTarget(ctx, "fixed@example.com", flatBody, "email")
		assert.Equal(t, "fixed@example.com", res)
	})

	t.Run("fixed username resolved for non-email channel", func(t *testing.T) {
		res := resolveTarget(ctx, "target_user", flatBody, "lark")
		assert.Equal(t, "target_user", res)
	})

	t.Run("non-exist user resolved as fallback", func(t *testing.T) {
		res := resolveTarget(ctx, "non_exist_user", flatBody, "email")
		assert.Equal(t, "non_exist_user", res)
	})

	t.Run("system target resolves to admin email for email channel", func(t *testing.T) {
		res := resolveTarget(ctx, "系统", flatBody, "email")
		assert.Equal(t, "admin@test.com", res)

		res2 := resolveTarget(ctx, "system", flatBody, "email")
		assert.Equal(t, "admin@test.com", res2)

		res3 := resolveTarget(ctx, "0", flatBody, "email")
		assert.Equal(t, "admin@test.com", res3)
	})

	t.Run("system target resolves to admin username for lark channel", func(t *testing.T) {
		res := resolveTarget(ctx, "系统", flatBody, "lark")
		assert.Equal(t, "admin_user", res)
	})
}

func TestPushChannelAPI(t *testing.T) {
	// 1. 模型校验测试
	t.Run("validate push channel model constraints", func(t *testing.T) {
		// 校验名称合法性
		c1 := &model.PushChannel{Name: "invalid-name!", URL: "https://hook.com", Other: "{}"}
		assert.Error(t, c1.Validate())

		// 校验 URL 安全前缀 HTTPS
		c2 := &model.PushChannel{Name: "custom_channel", URL: "http://insecure-hook.com", Other: "{}"}
		assert.Error(t, c2.Validate())

		// 校验 JSON 格式
		c3 := &model.PushChannel{Name: "custom_channel", URL: "https://hook.com", Other: "{invalid-json}"}
		assert.Error(t, c3.Validate())

		// 正确配置
		c4 := &model.PushChannel{Name: "custom_channel", URL: "https://hook.com", Other: "{\"content\":\"$content\"}"}
		assert.NoError(t, c4.Validate())

		// 飞书渠道校验：非 HTTPS 地址报错
		c5 := &model.PushChannel{Name: "lark_channel", Type: "lark", URL: "http://open.feishu.cn", Other: ""}
		assert.Error(t, c5.Validate())

		// 飞书正确配置
		c6 := &model.PushChannel{Name: "lark_channel", Type: "lark", URL: "https://open.feishu.cn", Other: ""}
		assert.NoError(t, c6.Validate())

		// Telegram 渠道校验
		cTelegramErr := &model.PushChannel{Name: "tg_channel", Type: "telegram", URL: "https://api.telegram.org", Token: "", Other: ""}
		assert.Error(t, cTelegramErr.Validate())

		cTelegramErr2 := &model.PushChannel{Name: "tg_channel", Type: "telegram", URL: "http://api.telegram.org", Token: "123:abc", Other: ""}
		assert.Error(t, cTelegramErr2.Validate())

		cTelegramOk := &model.PushChannel{Name: "tg_channel", Type: "telegram", URL: "", Token: "123:abc", Other: "-100123"}
		assert.NoError(t, cTelegramOk.Validate())
		assert.Equal(t, "https://api.telegram.org", cTelegramOk.URL)

		// 邮件配置校验：允许空配置以复用系统全局设置
		c7 := &model.PushChannel{Name: "email_channel", Type: "email", URL: "", Token: "", Other: ""}
		assert.NoError(t, c7.Validate())

		// 邮件正确配置 (非 HTTPS 协议 URL 允许)
		c8 := &model.PushChannel{Name: "email_channel", Type: "email", URL: "smtp.exmail.qq.com:465", Token: "user@example.com", Other: "authcode"}
		assert.NoError(t, c8.Validate())
	})

	// 2. HTTP CRUD & 触发鉴权测试
	dbConn, _, cleanup := setupPushTest(t)
	defer cleanup()

	// 构建路由以进行 HTTP 模拟请求
	r := testhelper.NewTestGinEngine()
	adminGroup := r.Group("/api/v1/admin")
	{
		adminGroup.GET("/push/channels", ListChannels)
		adminGroup.POST("/push/channels", CreateChannel)
		adminGroup.PUT("/push/channels/:id", UpdateChannel)
		adminGroup.DELETE("/push/channels/:id", DeleteChannel)
		adminGroup.POST("/push/channels/test", TestChannel)
	}

	var createdID uint64

	t.Run("admin create channel", func(t *testing.T) {
		reqBody := CreateChannelRequest{
			Name:        "my_custom_channel",
			Description: "My custom channel webhook",
			Type:        "custom",
			Token:       "my_chan_token",
			URL:         "https://webhook.site/test",
			Other:       `{"title": "$title", "body": "$content"}`,
			Enabled:     true,
		}
		bodyBytes, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/api/v1/admin/push/channels", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)

		dataMap, ok := resp.Data.(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "my_custom_channel", dataMap["name"])
		createdID = uint64(dataMap["id"].(float64))
	})

	t.Run("admin list channels", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/push/channels", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)
		list, ok := resp.Data.([]any)
		assert.True(t, ok)
		assert.Len(t, list, 1)
	})

	t.Run("admin update channel", func(t *testing.T) {
		updateReq := UpdateChannelRequest{
			Description: "Updated remark",
			Type:        "custom",
			Token:       "new_chan_token",
			URL:         "https://webhook.site/updated",
			Other:       `{"text": "$content"}`,
			Enabled:     true,
		}
		bodyBytes, _ := json.Marshal(updateReq)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/push/channels/"+strconv.FormatUint(createdID, 10), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var updated model.PushChannel
		dbConn.First(&updated, createdID)
		assert.Equal(t, "Updated remark", updated.Description)
		assert.Equal(t, "new_chan_token", updated.Token)
		assert.Equal(t, `{"text": "$content"}`, updated.Other)
	})

	t.Run("admin test channel endpoint", func(t *testing.T) {
		testReq := TestChannelRequest{
			Name:   "my_custom_channel",
			Target: "test_target",
		}
		bodyBytes, _ := json.Marshal(testReq)
		req, _ := http.NewRequest("POST", "/api/v1/admin/push/channels/test", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("admin delete channel", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/admin/push/channels/"+strconv.FormatUint(createdID, 10), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var count int64
		dbConn.Model(&model.PushChannel{}).Where("id = ?", createdID).Count(&count)
		assert.Equal(t, int64(0), count)
	})
}
