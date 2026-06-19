// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

func setupTestRouter(authUser *model.User) *gin.Engine {
	r := testhelper.NewTestGinEngine()
	adminGroup := r.Group("/api/v1/admin")

	// Mock authentication middleware
	adminGroup.Use(func(c *gin.Context) {
		if authUser != nil {
			oauth.SetToContext(c, oauth.UserObjKey, authUser)
		}
		c.Next()
	})

	adminGroup.GET("/users", ListUsers)
	adminGroup.POST("/users", CreateUser)
	adminGroup.GET("/users/:id", GetUser)
	adminGroup.PUT("/users/:id/status", UpdateUserStatus)
	adminGroup.DELETE("/users/:id", DeleteUser)
	return r
}

func TestListUsers(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	// Seed users
	users := []model.User{
		{
			ID:          1001,
			Username:    "alice",
			Nickname:    "Alice Nickname",
			IsActive:    true,
			IsAdmin:     false,
			LastLoginAt: time.Now(),
		},
		{
			ID:          1002,
			Username:    "bob",
			Nickname:    "Bob Nickname",
			IsActive:    true,
			IsAdmin:     false,
			LastLoginAt: time.Now(),
		},
		{
			ID:          1003,
			Username:    "charlie",
			Nickname:    "Charlie Nickname",
			IsActive:    false,
			IsAdmin:     true,
			LastLoginAt: time.Now(),
		},
	}

	for _, u := range users {
		if err := dbConn.Create(&u).Error; err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}
	}

	adminUser := &model.User{ID: 1003, Username: "charlie", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("basic pagination list", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/users?page=1&page_size=2", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp response.Any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		// Parse data map to our structure
		dataBytes, _ := json.Marshal(resp.Data)
		var listResp listUsersResponse
		if err := json.Unmarshal(dataBytes, &listResp); err != nil {
			t.Fatalf("failed to parse list response: %v", err)
		}

		if len(listResp.Users) != 2 {
			t.Errorf("expected 2 users, got %d", len(listResp.Users))
		}
		if listResp.Total != 3 {
			t.Errorf("expected total 3, got %d", listResp.Total)
		}
		// Ordered by ID ASC
		if listResp.Users[0].ID != 1001 || listResp.Users[1].ID != 1002 {
			t.Errorf("expected ordered ASC, got first ID %d, second ID %d", listResp.Users[0].ID, listResp.Users[1].ID)
		}
	})

	t.Run("filter by user_id", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/users?page=1&page_size=10&user_id=1001", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var resp response.Any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var listResp listUsersResponse
		_ = json.Unmarshal(dataBytes, &listResp)

		if len(listResp.Users) != 1 || listResp.Users[0].ID != 1001 {
			t.Errorf("expected 1 user with ID 1001, got total %d", len(listResp.Users))
		}
	})

	t.Run("filter by username prefix", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/users?page=1&page_size=10&username=bo", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var resp response.Any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var listResp listUsersResponse
		_ = json.Unmarshal(dataBytes, &listResp)

		if len(listResp.Users) != 1 || listResp.Users[0].Username != "bob" {
			t.Errorf("expected bob, got %v", listResp.Users)
		}
	})

	t.Run("invalid pagination parameter", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/users?page=0&page_size=10", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestGetUser(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	targetUser := model.User{
		ID:        1001,
		Username:  "alice",
		Password:  "secret-hash",
		Nickname:  "Alice Nickname",
		Email:     "alice@example.com",
		AvatarURL: "https://example.com/avatar.png",
		IsActive:  true,
		IsAdmin:   false,
		Bio:       "hello",
		Phone:     "123456",
		Gender:    "female",
		Website:   "https://example.com",
		Location:  "Shanghai",
	}
	if err := dbConn.Create(&targetUser).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	adminUser := &model.User{ID: 1003, Username: "charlie", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("get full user profile", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/users/1001", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp response.Any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		dataBytes, _ := json.Marshal(resp.Data)
		var resUser user
		if err := json.Unmarshal(dataBytes, &resUser); err != nil {
			t.Fatalf("failed to parse response data: %v", err)
		}

		if resUser.Email != targetUser.Email || resUser.Bio != targetUser.Bio || resUser.Phone != targetUser.Phone ||
			resUser.Gender != targetUser.Gender || resUser.Website != targetUser.Website || resUser.Location != targetUser.Location {
			t.Errorf("profile fields were not returned correctly: %+v", resUser)
		}
		if bytes.Contains(dataBytes, []byte("secret-hash")) {
			t.Error("response should not include password")
		}
	})

	t.Run("get non-existent user", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/users/9999", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestUpdateUserStatus(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	// Seed users
	regularUser := model.User{
		ID:       1001,
		Username: "alice",
		IsActive: true,
		IsAdmin:  false,
	}
	adminUser := model.User{
		ID:       1002,
		Username: "bob",
		IsActive: true,
		IsAdmin:  true,
	}

	dbConn.Create(&regularUser)
	dbConn.Create(&adminUser)

	router := setupTestRouter(&adminUser)

	t.Run("disable regular user successfully", func(t *testing.T) {
		payload := updateUserStatusRequest{IsActive: false}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/users/1001/status", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		// Verify DB status
		var u model.User
		dbConn.First(&u, 1001)
		if u.IsActive {
			t.Error("user should be deactivated in the database")
		}
	})

	t.Run("cannot disable admin user", func(t *testing.T) {
		payload := updateUserStatusRequest{IsActive: false}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/users/1002/status", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp response.Any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.ErrorMsg != cannotDisable {
			t.Errorf("expected error message '%s', got '%s'", cannotDisable, resp.ErrorMsg)
		}
	})

	t.Run("cannot enable/disable non-existent user", func(t *testing.T) {
		payload := updateUserStatusRequest{IsActive: false}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/users/9999/status", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestCreateUser(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1003, Username: "charlie", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("create user successfully", func(t *testing.T) {
		payload := createUserRequest{
			Username: "newuser",
			Password: "newpassword123",
			Nickname: "New Nickname",
			Email:    "newuser@example.com",
			IsActive: true,
			IsAdmin:  false,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp response.Any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.ErrorMsg != "" {
			t.Errorf("expected empty error message, got '%s'", resp.ErrorMsg)
		}

		dataBytes, _ := json.Marshal(resp.Data)
		var resUser user
		if err := json.Unmarshal(dataBytes, &resUser); err != nil {
			t.Fatalf("failed to parse response data: %v", err)
		}

		if resUser.Username != "newuser" || resUser.Nickname != "New Nickname" || !resUser.IsActive || resUser.IsAdmin {
			t.Errorf("unexpected user values: %+v", resUser)
		}

		// Verify in DB
		var dbUser model.User
		if err := dbConn.Where("username = ?", "newuser").First(&dbUser).Error; err != nil {
			t.Fatalf("failed to find user in db: %v", err)
		}
		if dbUser.Email != "newuser@example.com" {
			t.Errorf("expected email 'newuser@example.com', got '%s'", dbUser.Email)
		}
		if !dbUser.CheckPassword("newpassword123") {
			t.Error("password was not hashed correctly")
		}
	})

	t.Run("create user with duplicate username", func(t *testing.T) {
		// Create the first user
		existing := model.User{
			ID:       2001,
			Username: "dupuser",
			Nickname: "Dup User",
			Email:    "dupuser@example.com",
		}
		dbConn.Create(&existing)

		payload := createUserRequest{
			Username: "dupuser",
			Password: "password123",
			Nickname: "Another Nick",
			Email:    "another@example.com",
			IsActive: true,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp response.Any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.ErrorMsg != usernameExists {
			t.Errorf("expected error '%s', got '%s'", usernameExists, resp.ErrorMsg)
		}
	})

	t.Run("create user with duplicate email", func(t *testing.T) {
		existing := model.User{
			ID:       2002,
			Username: "existingemail",
			Nickname: "Existing Email",
			Email:    "dupemail@example.com",
		}
		dbConn.Create(&existing)

		payload := createUserRequest{
			Username: "newuser2",
			Password: "password123",
			Nickname: "New User 2",
			Email:    "dupemail@example.com",
			IsActive: true,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp response.Any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.ErrorMsg != emailExists {
			t.Errorf("expected error '%s', got '%s'", emailExists, resp.ErrorMsg)
		}
	})

	t.Run("validation error - password too short", func(t *testing.T) {
		payload := createUserRequest{
			Username: "shortpass",
			Password: "123",
			Email:    "shortpass@example.com",
			IsActive: true,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("validation error - invalid email format", func(t *testing.T) {
		payload := map[string]interface{}{
			"username":  "bademail",
			"password":  "password123",
			"email":     "not-an-email",
			"is_active": true,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestDeleteUser(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	if err := dbConn.AutoMigrate(&model.AccessToken{}, &model.ExternalAccount{}); err != nil {
		t.Fatalf("failed to migrate delete-related tables: %v", err)
	}

	regularUser := model.User{
		ID:       1001,
		Username: "alice",
		IsActive: true,
		IsAdmin:  false,
	}
	adminUser := model.User{
		ID:       1002,
		Username: "bob",
		IsActive: true,
		IsAdmin:  true,
	}
	selfUser := model.User{
		ID:       1003,
		Username: "charlie",
		IsActive: true,
		IsAdmin:  false,
	}

	if err := dbConn.Create(&regularUser).Error; err != nil {
		t.Fatalf("failed to seed regular user: %v", err)
	}
	if err := dbConn.Create(&adminUser).Error; err != nil {
		t.Fatalf("failed to seed admin user: %v", err)
	}
	if err := dbConn.Create(&selfUser).Error; err != nil {
		t.Fatalf("failed to seed self user: %v", err)
	}
	if err := dbConn.Create(&model.AccessToken{
		UserID:      regularUser.ID,
		Name:        "api",
		TokenHash:   "hash-for-delete-user-test",
		MaskedToken: "at_****test",
	}).Error; err != nil {
		t.Fatalf("failed to seed access token: %v", err)
	}
	if err := dbConn.Create(&model.ExternalAccount{
		ID:           5001,
		AuthSourceID: 1,
		UserID:       regularUser.ID,
		ExternalID:   "external-alice",
	}).Error; err != nil {
		t.Fatalf("failed to seed external account: %v", err)
	}

	router := setupTestRouter(&selfUser)

	t.Run("delete regular user successfully", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/admin/users/1001", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var count int64
		if err := dbConn.Model(&model.User{}).Where("id = ?", 1001).Count(&count).Error; err != nil {
			t.Fatalf("failed to count deleted user: %v", err)
		}
		if count != 0 {
			t.Errorf("expected deleted user count 0, got %d", count)
		}

		if err := dbConn.Model(&model.AccessToken{}).Where("user_id = ?", 1001).Count(&count).Error; err != nil {
			t.Fatalf("failed to count deleted access tokens: %v", err)
		}
		if count != 0 {
			t.Errorf("expected deleted access token count 0, got %d", count)
		}

		if err := dbConn.Model(&model.ExternalAccount{}).Where("user_id = ?", 1001).Count(&count).Error; err != nil {
			t.Fatalf("failed to count deleted external accounts: %v", err)
		}
		if count != 0 {
			t.Errorf("expected deleted external account count 0, got %d", count)
		}
	})

	t.Run("cannot delete admin user", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/admin/users/1002", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("cannot delete current user", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/admin/users/1003", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("delete non-existent user", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/admin/users/9999", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}
