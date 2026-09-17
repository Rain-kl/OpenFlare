// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/idgen"
	"Wavelet/plugins/domain/admin"
	"Wavelet/plugins/domain/auth"
	"Wavelet/plugins/domain/msg_gateway"
	"Wavelet/plugins/domain/risk_control"
	"Wavelet/plugins/domain/system"
	"Wavelet/plugins/domain/upload"
	"Wavelet/plugins/domain/user"
	"Wavelet/plugins/infra/cache"
	"Wavelet/plugins/infra/logger"
	"Wavelet/plugins/infra/storage"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	db "Wavelet/plugins/infra/database"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_ = idgen.Init(1)
	dbPath := filepath.Join(t.TempDir(), "domain_test.db")
	testDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, testDB.AutoMigrate(
		&user.User{},
		&user.AccessToken{},
		&auth.AuthSource{},
		&auth.ExternalAccount{},
		&msg_gateway.MessageChannel{},
		&msg_gateway.MessageBinding{},
		&msg_gateway.MessagePairingCode{},
		&admin.SystemConfig{},
		&admin.TaskExecution{},
		&msg_gateway.PushChannel{},
		&msg_gateway.PushEvent{},
		&msg_gateway.PushHistory{},
		&upload.Upload{},
		&upload.UploadStat{},
	))

	db.SetDB(testDB)
	return testDB
}

type mockOAuthProvider struct {
	name string
}

func (m *mockOAuthProvider) Name() string {
	return m.name
}

func (m *mockOAuthProvider) GetAuthURL(state string) string {
	return "https://auth.example.com/auth?state=" + state
}

func (m *mockOAuthProvider) ExchangeCode(ctx context.Context, code string) (*contracts.OAuthUserInfoDTO, error) {
	return &contracts.OAuthUserInfoDTO{
		ID:       1001,
		Username: "mock_user",
		Email:    "mock@example.com",
		Active:   true,
	}, nil
}

func TestAuthPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())
	testDB := setupTestDB(t)

	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New().Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))

	p := auth.New()
	assert.Equal(t, "auth", p.Name())
	assert.Equal(t, "auth", p.Manifest().Name)
	require.NoError(t, p.Apply(ctx))

	// 1. Verify migrations registered
	entry, ok := ctx.Migrations().Get("auth")
	require.True(t, ok)
	assert.Equal(t, "auth", entry.PluginID)
	entries, err := fs.ReadDir(entry.FS, entry.Dir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// 2. Verify AuthService
	authSvc, err := core.Inject[contracts.AuthService](ctx)
	require.NoError(t, err)
	require.NotNil(t, authSvc)
	assert.NotNil(t, authSvc.RequireAuthMiddleware())
	assert.NotNil(t, authSvc.RequireAdminMiddleware())

	// 3. Verify AuthRegistry
	authReg, err := core.Inject[contracts.AuthRegistry](ctx)
	require.NoError(t, err)
	require.NotNil(t, authReg)

	mockProv := &mockOAuthProvider{name: "github"}
	authReg.RegisterOAuthProvider("github", mockProv)
	retrieved, ok := authReg.GetOAuthProvider("github")
	require.True(t, ok)
	assert.Equal(t, "github", retrieved.Name())
	assert.Contains(t, authReg.ListOAuthProviders(), "github")

	// 4. Verify Routes
	routes := ctx.Router().Routes()
	var hasSources, hasLogin, hasUserInfo bool
	for _, r := range routes {
		if r.Path == "/api/v1/oauth/sources" {
			hasSources = true
		}
		if r.Path == "/api/v1/oauth/login" {
			hasLogin = true
		}
		if r.Path == "/api/v1/user-info" {
			hasUserInfo = true
		}
	}
	assert.True(t, hasSources)
	assert.True(t, hasLogin)
	assert.True(t, hasUserInfo)

	// 5. Verify Settings
	schema, ok := ctx.Settings().Get("auth.session_age")
	require.True(t, ok)
	assert.Equal(t, 86400*7, schema.Default)
}

func TestUserPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())
	testDB := setupTestDB(t)

	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New().Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))
	require.NoError(t, auth.New().Apply(ctx))

	p := user.New()
	assert.Equal(t, "user", p.Name())
	assert.Equal(t, "user", p.Manifest().Name)
	require.NoError(t, p.Apply(ctx))

	// 1. Verify migrations
	entry, ok := ctx.Migrations().Get("user")
	require.True(t, ok)
	assert.Equal(t, "user", entry.PluginID)

	// 2. Verify UserService
	userSvc, err := core.Inject[contracts.UserService](ctx)
	require.NoError(t, err)
	require.NotNil(t, userSvc)

	testCtx := context.Background()

	// 3. Create user
	created, err := userSvc.CreateUser(testCtx, contracts.CreateUserRequest{
		Username: "bob",
		Password: "SecurePassword123!",
		Nickname: "Bob Builder",
		Email:    "bob@example.com",
		IsAdmin:  false,
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "bob", created.Username)
	assert.Equal(t, "Bob Builder", created.Nickname)
	assert.Equal(t, "bob@example.com", created.Email)
	assert.False(t, created.IsAdmin)

	// 4. Query user
	byID, err := userSvc.GetUserByID(testCtx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "bob", byID.Username)

	byUsername, err := userSvc.GetUserByUsername(testCtx, "bob")
	require.NoError(t, err)
	assert.Equal(t, created.ID, byUsername.ID)

	byEmail, err := userSvc.GetUserByEmail(testCtx, "bob@example.com")
	require.NoError(t, err)
	assert.Equal(t, created.ID, byEmail.ID)

	// 5. Password verification and update
	assert.True(t, userSvc.VerifyPassword(testCtx, created.ID, "SecurePassword123!"))
	assert.False(t, userSvc.VerifyPassword(testCtx, created.ID, "WrongPass"))

	require.NoError(t, userSvc.UpdatePassword(testCtx, created.ID, "SecurePassword123!", "NewSecurePassword456!"))
	assert.True(t, userSvc.VerifyPassword(testCtx, created.ID, "NewSecurePassword456!"))

	// 6. Update Profile
	newBio := "I build things"
	newPhone := "13800138000"
	updated, err := userSvc.UpdateProfile(testCtx, created.ID, contracts.UpdateUserProfileRequest{
		Bio:   &newBio,
		Phone: &newPhone,
	})
	require.NoError(t, err)
	assert.Equal(t, newBio, updated.Bio)
	assert.Equal(t, newPhone, updated.Phone)

	// 7. Update Last Login
	require.NoError(t, userSvc.UpdateLastLogin(testCtx, created.ID, "127.0.0.1"))

	// 8. Admin operations: SetUserActive, SetUserAdmin, ListUsers
	require.NoError(t, userSvc.SetUserAdmin(testCtx, created.ID, true))
	reloaded, err := userSvc.GetUserByID(testCtx, created.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.IsAdmin)

	require.NoError(t, userSvc.SetUserActive(testCtx, created.ID, false))
	reloadedBanned, err := userSvc.GetUserByID(testCtx, created.ID)
	require.NoError(t, err)
	assert.False(t, reloadedBanned.IsActive)

	list, total, err := userSvc.ListUsers(testCtx, 1, 10, "bob")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, list, 1)
	assert.Equal(t, "bob", list[0].Username)

	// 9. Tasks
	taskDef, ok := ctx.Tasks().Get("user:send_email_code")
	require.True(t, ok)
	assert.Equal(t, 3, taskDef.Retry)

	// 10. Settings
	sReg, ok := ctx.Settings().Get("user.registration_enabled")
	require.True(t, ok)
	assert.Equal(t, true, sReg.Default)
}

func TestMessageGatewayPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())
	testDB := setupTestDB(t)

	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New().Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))

	p := msg_gateway.New()
	assert.Equal(t, "msg_gateway", p.Name())
	assert.Equal(t, "msg_gateway", p.Manifest().Name)
	require.NoError(t, p.Apply(ctx))

	// 1. Migrations
	entry, ok := ctx.Migrations().Get("msg_gateway")
	require.True(t, ok)
	assert.Equal(t, "msg_gateway", entry.PluginID)

	// 2. Routes
	routes := ctx.Router().Routes()
	var hasChannels, hasBindings bool
	for _, r := range routes {
		if r.Path == "/api/v1/message-gateway/channels" {
			hasChannels = true
		}
		if r.Path == "/api/v1/message-gateway/bindings" {
			hasBindings = true
		}
	}
	assert.True(t, hasChannels)
	assert.True(t, hasBindings)

	// 3. Tasks & Schedules
	taskDef, ok := ctx.Tasks().Get("msg_gateway:push_notification")
	require.True(t, ok)
	assert.Equal(t, 3, taskDef.Retry)

	schedDef, ok := ctx.Schedules().Get("msg_gateway:cleanup_pairing_codes")
	require.True(t, ok)
	assert.Equal(t, "*/10 * * * *", schedDef.Spec)

	// 4. EventBus Trigger
	var receivedEvent msg_gateway.PushNotificationEvent
	var eventFired bool
	ctx.Events().On("notification:push", func(c context.Context, e msg_gateway.PushNotificationEvent) error {
		eventFired = true
		receivedEvent = e
		return nil
	})

	err := ctx.Events().Emit(context.Background(), "notification:push", msg_gateway.PushNotificationEvent{
		UserID:  99,
		Channel: "telegram",
		Title:   "System Alert",
		Content: "Disk 85% full",
	})
	require.NoError(t, err)
	assert.True(t, eventFired)
	assert.Equal(t, uint64(99), receivedEvent.UserID)
	assert.Equal(t, "telegram", receivedEvent.Channel)
	assert.Equal(t, "System Alert", receivedEvent.Title)

	// 5. Settings
	schema, ok := ctx.Settings().Get("msg_gateway.pairing_code_expiry_minutes")
	require.True(t, ok)
	assert.Equal(t, 15, schema.Default)
}

func TestRiskControlPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())
	p := risk_control.New()
	assert.Equal(t, "risk_control", p.Name())
	assert.Equal(t, "risk_control", p.Manifest().Name)
	require.NoError(t, p.Apply(ctx))

	// 1. Middleware registered on Router
	mws := ctx.Router().Middlewares()
	assert.NotEmpty(t, mws)

	// 2. Settings
	schema, ok := ctx.Settings().Get("risk_control.ip_rate_limit_per_minute")
	require.True(t, ok)
	assert.Equal(t, 60, schema.Default)

	// 3. Disposal cleanup
	require.NoError(t, ctx.Dispose())
}

func TestAdminPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())
	testDB := setupTestDB(t)

	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New().Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))

	p := admin.New()
	assert.Equal(t, "admin", p.Name())
	assert.Equal(t, "admin", p.Manifest().Name)
	require.NoError(t, p.Apply(ctx))

	// 1. Admin Routes
	routes := ctx.Router().Routes()
	var hasStatus, hasDBOverview, hasUsers, hasTasks, hasConfigs bool
	for _, r := range routes {
		if r.Path == "/api/v1/admin/status" {
			hasStatus = true
		}
		if r.Path == "/api/v1/admin/db-manage/overview" {
			hasDBOverview = true
		}
		if r.Path == "/api/v1/admin/users" {
			hasUsers = true
		}
		if r.Path == "/api/v1/admin/tasks/types" {
			hasTasks = true
		}
		if r.Path == "/api/v1/admin/system-configs" {
			hasConfigs = true
		}
	}
	assert.True(t, hasStatus)
	assert.True(t, hasDBOverview)
	assert.True(t, hasUsers)
	assert.True(t, hasTasks)
	assert.True(t, hasConfigs)

	// 2. Task
	_, ok := ctx.Tasks().Get("logs:db_switch")
	require.True(t, ok)

	// 3. Settings
	schema, ok := ctx.Settings().Get("admin.system_cleanup_cron")
	require.True(t, ok)
	assert.Equal(t, "0 3 * * *", schema.Default)

	provider, err := core.Inject[contracts.PublicConfigProvider](ctx)
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestPublicConfigExposesVisibleAdminRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())
	testDB := setupTestDB(t)

	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New().Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))

	require.NoError(t, testDB.Create(&admin.SystemConfig{
		Key:        "cap_login_enabled",
		Value:      "true",
		Type:       "system",
		Visibility: 1,
	}).Error)

	require.NoError(t, admin.New().Apply(ctx))
	require.NoError(t, system.New().Apply(ctx))

	var handler gin.HandlerFunc
	for _, rd := range ctx.Router().Routes() {
		if rd.Method != "GET" || rd.Path != "/api/v1/config/public" {
			continue
		}
		require.NotEmpty(t, rd.Handlers)
		switch h := rd.Handlers[0].(type) {
		case gin.HandlerFunc:
			handler = h
		case func(*gin.Context):
			handler = h
		default:
			t.Fatalf("unexpected handler type %T", rd.Handlers[0])
		}
		break
	}
	require.NotNil(t, handler)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/config/public", nil)
	c.Request = c.Request.WithContext(core.WithAppContext(c.Request.Context(), ctx))
	handler(c)

	var body struct {
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body = %s", w.Body.String())
	assert.Equal(t, "true", body.Data["cap_login_enabled"])
	_, wrapped := body.Data["configs"]
	assert.False(t, wrapped, "payload must be a flat map, got %v", body.Data)
}

func TestAllDomainPluginsCombined(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(map[string]any{
		"redis": map[string]any{
			"enabled": true,
			"addrs":   []string{mr.Addr()},
		},
	}))
	require.NoError(t, ctx.Config().Resolve())
	testDB := setupTestDB(t)

	// Apply Infra plugins
	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New(cache.WithRedis(rdb)).Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))
	require.NoError(t, storage.New().Apply(ctx))

	// Apply Domain plugins
	require.NoError(t, auth.New().Apply(ctx))
	require.NoError(t, user.New().Apply(ctx))
	require.NoError(t, msg_gateway.New().Apply(ctx))
	require.NoError(t, risk_control.New().Apply(ctx))
	require.NoError(t, admin.New().Apply(ctx))

	// Verify cross-plugin service injection via Using3
	var resolved bool
	err = core.Using3(ctx, func(authSvc contracts.AuthService, userSvc contracts.UserService, authReg contracts.AuthRegistry) {
		resolved = true
		assert.NotNil(t, authSvc)
		assert.NotNil(t, userSvc)
		assert.NotNil(t, authReg)
	})
	require.NoError(t, err)
	assert.True(t, resolved)

	// Verify all migration entries
	allMigrations := ctx.Migrations().Entries()
	assert.GreaterOrEqual(t, len(allMigrations), 3)

	// Verify total routes registered
	allRoutes := ctx.Router().Routes()
	assert.GreaterOrEqual(t, len(allRoutes), 20)

	// Verify total tasks registered
	allTasks := ctx.Tasks().Tasks()
	assert.GreaterOrEqual(t, len(allTasks), 4)
	for _, task := range allTasks {
		dto := task.ToDTO()
		assert.NotEmpty(t, dto.Type, "task %s should have type", task.Pattern)
		assert.NotEmpty(t, dto.AsynqTask, "task %s should have asynq_task", task.Pattern)
		assert.NotEmpty(t, dto.Name, "task %s should have name", task.Pattern)
	}

	// Verify total schedules registered
	allSchedules := ctx.Schedules().Schedules()
	assert.GreaterOrEqual(t, len(allSchedules), 1)

	// 每个调度指向的任务类型都必须已注册 Handler，否则触发时会投递到无人处理的
	// 任务类型，预期的清理逻辑静默失效。
	for _, sched := range allSchedules {
		_, ok := ctx.Tasks().Get(sched.TaskType)
		assert.Truef(t, ok, "schedule %q dispatches to task %q, which is never registered",
			sched.Spec, sched.TaskType)
	}

	// Verify total settings schemas registered
	allSettings := ctx.Settings().Schemas()
	assert.GreaterOrEqual(t, len(allSettings), 7)

	// Clean shutdown
	require.NoError(t, ctx.Dispose())
}

func TestSystemCleanupEventDrivenCoordination(t *testing.T) {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())
	testDB := setupTestDB(t)

	// Apply Infra & Domain plugins
	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New().Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))
	require.NoError(t, storage.New().Apply(ctx))

	require.NoError(t, user.New().Apply(ctx))
	require.NoError(t, msg_gateway.New().Apply(ctx))
	require.NoError(t, upload.New().Apply(ctx))
	require.NoError(t, admin.New().Apply(ctx))

	// 1. Seed old and recent task executions (admin domain)
	now := time.Now()
	oldTime := now.Add(-10 * 24 * time.Hour)
	recentTime := now.Add(-1 * time.Hour)

	oldExec := admin.TaskExecution{
		ID:        101,
		TaskID:    "task-old-exec",
		TaskType:  "test_task",
		Status:    "success",
		CreatedAt: oldTime,
	}
	recentExec := admin.TaskExecution{
		ID:        102,
		TaskID:    "task-recent-exec",
		TaskType:  "test_task",
		Status:    "success",
		CreatedAt: recentTime,
	}
	require.NoError(t, testDB.Create(&oldExec).Error)
	require.NoError(t, testDB.Model(&oldExec).UpdateColumn("created_at", oldTime).Error)
	require.NoError(t, testDB.Create(&recentExec).Error)

	// 2. Seed old and recent push histories (msg_gateway domain, 30 days retention)
	oldHistoryTime := now.Add(-40 * 24 * time.Hour)
	oldHistory := msg_gateway.PushHistory{
		EventKey:  "login",
		Channel:   "telegram",
		Target:    "123",
		Title:     "Old login",
		Content:   "Old content",
		Level:     "info",
		Status:    "success",
		CreatedAt: oldHistoryTime,
	}
	recentHistory := msg_gateway.PushHistory{
		EventKey:  "login",
		Channel:   "telegram",
		Target:    "123",
		Title:     "Recent login",
		Content:   "Recent content",
		Level:     "info",
		Status:    "success",
		CreatedAt: recentTime,
	}
	require.NoError(t, testDB.Create(&oldHistory).Error)
	require.NoError(t, testDB.Model(&oldHistory).UpdateColumn("created_at", oldHistoryTime).Error)
	require.NoError(t, testDB.Create(&recentHistory).Error)

	// 3. Seed old pending upload and recent pending upload (upload domain)
	oldUpload := upload.Upload{
		ID:        901,
		UserID:    1,
		FileName:  "old.png",
		FilePath:  "uploads/old.png",
		FileSize:  100,
		Status:    upload.UploadStatusPending,
		CreatedAt: now.Add(-2 * time.Hour),
	}
	recentUpload := upload.Upload{
		ID:        902,
		UserID:    1,
		FileName:  "recent.png",
		FilePath:  "uploads/recent.png",
		FileSize:  100,
		Status:    upload.UploadStatusPending,
		CreatedAt: now.Add(-10 * time.Minute),
	}
	require.NoError(t, testDB.Create(&oldUpload).Error)
	require.NoError(t, testDB.Model(&oldUpload).UpdateColumn("created_at", now.Add(-2*time.Hour)).Error)
	require.NoError(t, testDB.Create(&recentUpload).Error)

	// Dispatch admin system cleanup task handler
	taskDef, ok := ctx.Tasks().Get("system:cleanup")
	require.True(t, ok, "system:cleanup task must be registered in admin")
	require.NotNil(t, taskDef.Handler)

	type resultExecutor interface {
		Execute(ctx context.Context, payload []byte) (*contracts.TaskResultDTO, error)
	}
	handler, ok := taskDef.Handler.(resultExecutor)
	require.True(t, ok)

	res, err := handler.Execute(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify Admin cleanup: old task execution deleted, recent remains
	var execCount int64
	testDB.Model(&admin.TaskExecution{}).Count(&execCount)
	assert.Equal(t, int64(1), execCount)

	// Verify MsgGateway cleanup: old push history deleted, recent remains
	var historyCount int64
	testDB.Model(&msg_gateway.PushHistory{}).Count(&historyCount)
	assert.Equal(t, int64(1), historyCount)

	// Verify Upload cleanup: old pending upload deleted, recent remains
	var uploadCount int64
	testDB.Model(&upload.Upload{}).Count(&uploadCount)
	assert.Equal(t, int64(1), uploadCount)

	require.NoError(t, ctx.Dispose())
}
