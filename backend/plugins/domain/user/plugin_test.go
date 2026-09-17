// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/idgen"
	"Wavelet/plugins/domain/user"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	database "Wavelet/plugins/infra/database"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_ = idgen.Init(1)
	dbPath := filepath.Join(t.TempDir(), "user_test.db")
	testDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, testDB.AutoMigrate(
		&user.User{},
		&user.AccessToken{},
	))

	database.SetDB(testDB)
	return testDB
}

func TestUserPluginUnit(t *testing.T) {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())
	testDB := setupTestDB(t)

	dbPlugin := database.New(database.WithDB(testDB))
	require.NoError(t, dbPlugin.Apply(ctx))

	p := user.New()
	assert.Equal(t, "user", p.Name())
	assert.Equal(t, "1.0.0", p.Manifest().Version)
	require.NoError(t, p.Apply(ctx))

	userSvc, err := core.Inject[contracts.UserService](ctx)
	require.NoError(t, err)
	require.NotNil(t, userSvc)

	testCtx := context.Background()

	// 1. Create User
	u, err := userSvc.CreateUser(testCtx, contracts.CreateUserRequest{
		Username: "charlie",
		Password: "Password789!",
		Email:    "charlie@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "charlie", u.Username)

	// 2. Empty username error
	_, err = userSvc.CreateUser(testCtx, contracts.CreateUserRequest{})
	assert.Error(t, err)

	// 3. Verify Password
	assert.True(t, userSvc.VerifyPassword(testCtx, u.ID, "Password789!"))
	assert.False(t, userSvc.VerifyPassword(testCtx, u.ID, "Wrong"))

	// 4. Update Password with wrong old password
	err = userSvc.UpdatePassword(testCtx, u.ID, "WrongOld", "NewPass999!")
	assert.Error(t, err)

	// Update Password success
	err = userSvc.UpdatePassword(testCtx, u.ID, "Password789!", "NewPass999!")
	require.NoError(t, err)
	assert.True(t, userSvc.VerifyPassword(testCtx, u.ID, "NewPass999!"))

	// 5. Update Profile
	nickname := "Charlie Brown"
	email := "charlie.new@example.com"
	gender := "male"
	website := "https://charlie.me"
	loc := "SF"
	updated, err := userSvc.UpdateProfile(testCtx, u.ID, contracts.UpdateUserProfileRequest{
		Nickname: &nickname,
		Email:    &email,
		Gender:   &gender,
		Website:  &website,
		Location: &loc,
	})
	require.NoError(t, err)
	assert.Equal(t, "Charlie Brown", updated.Nickname)
	assert.Equal(t, "charlie.new@example.com", updated.Email)
	assert.Equal(t, "male", updated.Gender)
	assert.Equal(t, "https://charlie.me", updated.Website)
	assert.Equal(t, "SF", updated.Location)

	// 6. List and Status
	require.NoError(t, userSvc.SetUserAdmin(testCtx, u.ID, true))
	require.NoError(t, userSvc.SetUserActive(testCtx, u.ID, true))

	list, total, err := userSvc.ListUsers(testCtx, 1, 10, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, list)
}

func TestUserLoginHTTPHandler(t *testing.T) {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())
	testDB := setupTestDB(t)

	dbPlugin := database.New(database.WithDB(testDB))
	require.NoError(t, dbPlugin.Apply(ctx))

	p := user.New()
	require.NoError(t, p.Apply(ctx))

	userSvc, err := core.Inject[contracts.UserService](ctx)
	require.NoError(t, err)

	_, err = userSvc.CreateUser(context.Background(), contracts.CreateUserRequest{
		Username: "admin",
		Password: "Password123!",
		Email:    "admin@example.com",
	})
	require.NoError(t, err)

	r := gin.New()
	cookieStore := cookie.NewStore([]byte("test-session-secret"))
	r.Use(sessions.Sessions("wavelet_session", cookieStore))
	r.POST("/api/v1/user/login", user.Login)

	reqBody := `{"username":"admin","password":"Password123!"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/user/login", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"username":"admin"`)
	setCookie := w.Header().Get("Set-Cookie")
	assert.NotEmpty(t, setCookie)
	assert.Contains(t, setCookie, "wavelet_session=")

	// Plaintext default password seeded user
	plainUser := &user.User{
		Username: "plain_admin",
		Password: "12345678", // Plaintext seed
		Email:    "plain@example.com",
		IsActive: true,
	}
	require.NoError(t, user.CreateUser(context.Background(), plainUser))
	require.NotZero(t, plainUser.ID)
	require.Greater(t, plainUser.ID, uint64(10000), "user id must be snowflake, not sqlite autoincrement")

	reqBodyPlain := `{"username":"plain_admin","password":"12345678"}`
	reqPlain, _ := http.NewRequest(http.MethodPost, "/api/v1/user/login", bytes.NewBufferString(reqBodyPlain))
	reqPlain.Header.Set("Content-Type", "application/json")
	wPlain := httptest.NewRecorder()

	r.ServeHTTP(wPlain, reqPlain)

	assert.Equal(t, http.StatusOK, wPlain.Code)
	assert.Contains(t, wPlain.Body.String(), `"username":"plain_admin"`)
	assert.Contains(t, wPlain.Body.String(), `"need_change_password":true`)
	cookieHeader := wPlain.Header().Get("Set-Cookie")
	assert.NotEmpty(t, cookieHeader)

	// Change password
	r.POST("/api/v1/user/change-password", user.ChangePassword)
	changeBody := `{"old_password":"12345678","new_password":"NewStrongPassword123!"}`
	reqChange, _ := http.NewRequest(http.MethodPost, "/api/v1/user/change-password", bytes.NewBufferString(changeBody))
	reqChange.Header.Set("Content-Type", "application/json")
	reqChange.Header.Set("Cookie", cookieHeader)
	wChange := httptest.NewRecorder()

	r.ServeHTTP(wChange, reqChange)
	assert.Equal(t, http.StatusOK, wChange.Code)

	// Verify updated user model has encrypted password and need_change_password is false
	updatedUser, err := user.GetUserByUsername(context.Background(), "plain_admin")
	require.NoError(t, err)
	assert.False(t, updatedUser.IsPlaintextPassword())
	assert.True(t, updatedUser.CheckPassword("NewStrongPassword123!"))
	assert.False(t, updatedUser.NeedChangePassword)
}
