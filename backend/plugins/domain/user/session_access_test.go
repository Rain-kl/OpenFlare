// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/auth"
	"Wavelet/plugins/domain/upload"
	uploadmodels "Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/user"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	database "Wavelet/plugins/infra/database"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

type stubStorageService struct{ contracts.StorageService }

type loginEnvelope struct {
	ErrorMsg string          `json:"error_msg"`
	Data     json.RawMessage `json:"data"`
}

func mountUserAuthEngine(t *testing.T) (*gin.Engine, contracts.UserService) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	if err := ctx.Config().Resolve(); err != nil {
		t.Fatalf("Config.Resolve() error = %v", err)
	}
	testDB := setupTestDB(t)
	if err := testDB.AutoMigrate(&uploadmodels.Upload{}); err != nil {
		t.Fatalf("AutoMigrate(Upload) error = %v", err)
	}
	if err := database.New(database.WithDB(testDB)).Apply(ctx); err != nil {
		t.Fatalf("database.Apply() error = %v", err)
	}
	if err := auth.New().Apply(ctx); err != nil {
		t.Fatalf("auth.Apply() error = %v", err)
	}
	if err := user.New().Apply(ctx); err != nil {
		t.Fatalf("user.Apply() error = %v", err)
	}
	core.Provide[contracts.StorageService](ctx, stubStorageService{})
	if err := upload.New().Apply(ctx); err != nil {
		t.Fatalf("upload.Apply() error = %v", err)
	}

	userSvc, err := core.Inject[contracts.UserService](ctx)
	if err != nil || userSvc == nil {
		t.Fatalf("Inject UserService: svc=%v err=%v", userSvc, err)
	}

	engine := gin.New()
	engine.Use(response.ErrorHandlerMiddleware())
	engine.Use(sessions.Sessions("wavelet_session_id", cookie.NewStore([]byte("test-session-secret"))))
	engine.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(core.WithAppContext(c.Request.Context(), ctx.Root()))
		c.Next()
	})

	for _, rd := range ctx.Router().Routes() {
		handlers := make([]gin.HandlerFunc, 0, len(rd.Middlewares)+len(rd.Handlers))
		for _, m := range rd.Middlewares {
			h, ok := m.(gin.HandlerFunc)
			if !ok {
				fn, ok := m.(func(*gin.Context))
				if !ok {
					t.Fatalf("unsupported middleware type %T for %s %s", m, rd.Method, rd.Path)
				}
				h = fn
			}
			handlers = append(handlers, h)
		}
		for _, raw := range rd.Handlers {
			h, ok := raw.(gin.HandlerFunc)
			if !ok {
				fn, ok := raw.(func(*gin.Context))
				if !ok {
					t.Fatalf("unsupported handler type %T for %s %s", raw, rd.Method, rd.Path)
				}
				h = fn
			}
			handlers = append(handlers, h)
		}
		engine.Handle(rd.Method, rd.Path, handlers...)
	}

	return engine, userSvc
}

func loginAndCookie(t *testing.T, engine *gin.Engine, username, password string) []*http.Cookie {
	t.Helper()
	body := `{"username":"` + username + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/user/login username=%s status = %d, want 200 body=%s", username, rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("POST /api/v1/user/login username=%s Set-Cookie missing, headers=%v", username, rec.Header())
	}
	return cookies
}

func getWithCookies(engine *gin.Engine, path string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestNonAdminSessionCanAccessProtectedAPIs(t *testing.T) {
	engine, userSvc := mountUserAuthEngine(t)
	bg := context.Background()

	admin, err := userSvc.CreateUser(bg, contracts.CreateUserRequest{
		Username: "admin_user",
		Password: "Password123!",
		Email:    "admin_user@example.com",
		IsAdmin:  true,
	})
	if err != nil {
		t.Fatalf("CreateUser(admin) error = %v", err)
	}
	if err := userSvc.SetUserAdmin(bg, admin.ID, true); err != nil {
		t.Fatalf("SetUserAdmin() error = %v", err)
	}

	member, err := userSvc.CreateUser(bg, contracts.CreateUserRequest{
		Username: "plain_user",
		Password: "Password123!",
		Email:    "plain_user@example.com",
		IsAdmin:  false,
	})
	if err != nil {
		t.Fatalf("CreateUser(member) error = %v", err)
	}
	if member.IsAdmin {
		t.Fatalf("CreateUser(member).IsAdmin = true, want false")
	}

	cases := []struct {
		name     string
		username string
		wantID   uint64
	}{
		{name: "admin", username: "admin_user", wantID: admin.ID},
		{name: "non-admin", username: "plain_user", wantID: member.ID},
	}

	protected := []string{
		"/api/v1/user/self",
		"/api/v1/user-info",
		"/api/v1/upload/my?page=1&page_size=12",
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cookies := loginAndCookie(t, engine, tc.username, "Password123!")
			for _, path := range protected {
				rec := getWithCookies(engine, path, cookies)
				if rec.Code != http.StatusOK {
					t.Errorf("GET %s as %s status = %d, want 200 body=%s", path, tc.name, rec.Code, rec.Body.String())
					continue
				}
				var env loginEnvelope
				if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
					t.Errorf("GET %s as %s decode error = %v body=%s", path, tc.name, err, rec.Body.String())
					continue
				}
				if env.ErrorMsg != "" {
					t.Errorf("GET %s as %s error_msg = %q, want empty", path, tc.name, env.ErrorMsg)
				}
				if bytes.Contains(env.Data, []byte(`"username"`)) {
					var payload struct {
						ID       json.RawMessage `json:"id"`
						Username string          `json:"username"`
					}
					if err := json.Unmarshal(env.Data, &payload); err != nil {
						t.Errorf("GET %s as %s data decode error = %v data=%s", path, tc.name, err, string(env.Data))
						continue
					}
					if payload.Username != tc.username {
						t.Errorf("GET %s as %s username = %q, want %q", path, tc.name, payload.Username, tc.username)
					}
					if len(payload.ID) == 0 || payload.ID[0] != '"' {
						t.Errorf("GET %s as %s id JSON = %s, want a string (snowflake ids exceed JS MAX_SAFE_INTEGER)", path, tc.name, payload.ID)
					}
				}
			}
		})
	}
}

func TestLoginBackfillsNullUserIDSoProtectedAPIsSucceed(t *testing.T) {
	engine, _ := mountUserAuthEngine(t)
	db := database.DB(context.Background())
	if db == nil {
		t.Fatal("database.DB() = nil, want the test database")
	}

	legacy := user.User{
		Username: "legacy_zero",
		Email:    "legacy_zero@example.com",
		IsActive: true,
	}
	if err := legacy.SetEncryptedPassword("Password123!"); err != nil {
		t.Fatalf("SetEncryptedPassword() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	if _, err := sqlDB.Exec(
		"INSERT INTO w_users (id, username, password, email, is_active, is_admin) VALUES (0, ?, ?, ?, 1, 0)",
		legacy.Username, legacy.Password, legacy.Email,
	); err != nil {
		t.Fatalf("INSERT legacy user error = %v", err)
	}
	var stored sql.NullInt64
	if err := sqlDB.QueryRow("SELECT id FROM w_users WHERE username = ?", legacy.Username).Scan(&stored); err != nil {
		t.Fatalf("SELECT id error = %v", err)
	}
	if stored.Valid && stored.Int64 != 0 {
		t.Fatalf("legacy user id = %d, want 0 or NULL to reproduce the 401", stored.Int64)
	}

	cookies := loginAndCookie(t, engine, legacy.Username, "Password123!")
	protected := []string{
		"/api/v1/user/self",
		"/api/v1/user-info",
		"/api/v1/upload/my?page=1&page_size=12",
	}
	for _, path := range protected {
		rec := getWithCookies(engine, path, cookies)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s as legacy_zero status = %d, want 200 body=%s", path, rec.Code, rec.Body.String())
			continue
		}
		var env loginEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Errorf("GET %s as legacy_zero decode error = %v body=%s", path, err, rec.Body.String())
			continue
		}
		if env.ErrorMsg != "" {
			t.Errorf("GET %s as legacy_zero error_msg = %q, want empty", path, env.ErrorMsg)
		}
	}

	var backfilled uint64
	if err := db.Raw("SELECT id FROM w_users WHERE username = ?", legacy.Username).Scan(&backfilled).Error; err != nil {
		t.Fatalf("SELECT backfilled id error = %v", err)
	}
	if backfilled == 0 {
		t.Errorf("legacy user id after login = 0, want a snowflake id")
	}
}

func TestLoginRequiredRejectsMissingSession(t *testing.T) {
	engine, _ := mountUserAuthEngine(t)
	rec := getWithCookies(engine, "/api/v1/user/self", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/v1/user/self without cookie status = %d, want 401 body=%s", rec.Code, rec.Body.String())
	}
	body, _ := io.ReadAll(rec.Body)
	if !bytes.Contains(body, []byte("未登录")) && !bytes.Contains(body, []byte("用户不存在")) {
		t.Errorf("GET /api/v1/user/self without cookie body = %s, want 未登录", body)
	}
}
