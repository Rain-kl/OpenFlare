package router_test

import (
	"bytes"
	"encoding/json"
	"gin-template/common"
	"gin-template/model"
	"gin-template/router"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
)

type apiResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestPhase1PublishLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	token := prepareRootToken(t)

	createBody := map[string]any{
		"domain":     "app.example.com",
		"origin_url": "https://origin-a.internal",
		"enabled":    true,
		"remark":     "primary route",
	}
	resp := performJSONRequest(t, engine, token, http.MethodPost, "/api/proxy-routes/", createBody)
	var createdRoute model.ProxyRoute
	decodeResponseData(t, resp, &createdRoute)
	if createdRoute.Domain != "app.example.com" {
		t.Fatalf("unexpected created route domain: %s", createdRoute.Domain)
	}

	resp = performJSONRequest(t, engine, token, http.MethodGet, "/api/proxy-routes/", nil)
	var routes []model.ProxyRoute
	decodeResponseData(t, resp, &routes)
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	resp = performJSONRequest(t, engine, token, http.MethodPost, "/api/config-versions/publish", nil)
	var version1 model.ConfigVersion
	decodeResponseData(t, resp, &version1)
	if !version1.IsActive {
		t.Fatal("expected published version to be active")
	}
	if version1.SnapshotJSON == "" || version1.RenderedConfig == "" || version1.Checksum == "" {
		t.Fatal("expected published version to contain snapshot, rendered config and checksum")
	}

	initialSnapshot := version1.SnapshotJSON
	initialRendered := version1.RenderedConfig

	updateBody := map[string]any{
		"domain":     "app.example.com",
		"origin_url": "https://origin-b.internal",
		"enabled":    true,
		"remark":     "updated route",
	}
	routePath := "/api/proxy-routes/" + toString(createdRoute.ID)
	resp = performJSONRequest(t, engine, token, http.MethodPut, routePath, updateBody)
	decodeResponseData(t, resp, &createdRoute)
	if createdRoute.OriginURL != "https://origin-b.internal" {
		t.Fatalf("unexpected updated route origin: %s", createdRoute.OriginURL)
	}

	resp = performJSONRequest(t, engine, token, http.MethodPost, "/api/config-versions/publish", nil)
	var version2 model.ConfigVersion
	decodeResponseData(t, resp, &version2)
	if version2.ID == version1.ID {
		t.Fatal("expected a new version record")
	}

	resp = performJSONRequest(t, engine, token, http.MethodGet, "/api/config-versions/", nil)
	var versions []model.ConfigVersion
	decodeResponseData(t, resp, &versions)
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}

	activeResp := performJSONRequest(t, engine, token, http.MethodGet, "/api/config-versions/active", nil)
	var activeVersion model.ConfigVersion
	decodeResponseData(t, activeResp, &activeVersion)
	if activeVersion.ID != version2.ID {
		t.Fatalf("expected version %d active, got %d", version2.ID, activeVersion.ID)
	}

	activatePath := "/api/config-versions/" + toString(version1.ID) + "/activate"
	resp = performJSONRequest(t, engine, token, http.MethodPut, activatePath, nil)
	decodeResponseData(t, resp, &activeVersion)
	if activeVersion.ID != version1.ID || !activeVersion.IsActive {
		t.Fatal("expected version1 to become active after rollback activation")
	}

	var storedVersion1 model.ConfigVersion
	if err := model.DB.First(&storedVersion1, version1.ID).Error; err != nil {
		t.Fatalf("failed to query version1: %v", err)
	}
	if storedVersion1.SnapshotJSON != initialSnapshot {
		t.Fatal("expected version1 snapshot to remain immutable")
	}
	if storedVersion1.RenderedConfig != initialRendered {
		t.Fatal("expected version1 rendered config to remain immutable")
	}

	deletePath := "/api/proxy-routes/" + toString(createdRoute.ID)
	resp = performJSONRequest(t, engine, token, http.MethodDelete, deletePath, nil)
	if !resp.Success {
		t.Fatalf("expected delete route success, got: %s", resp.Message)
	}
}

func setupTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "phase1.db")
	common.SQLitePath = dbPath
	if err := model.InitDB(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	t.Cleanup(func() {
		if err := model.CloseDB(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	})
}

func prepareRootToken(t *testing.T) string {
	t.Helper()
	user := &model.User{Username: "root"}
	if err := user.FillUserByUsername(); err != nil {
		t.Fatalf("failed to load root user: %v", err)
	}
	user.Token = "phase1-test-token"
	if err := model.DB.Model(user).Update("token", user.Token).Error; err != nil {
		t.Fatalf("failed to set root token: %v", err)
	}
	return user.Token
}

func performJSONRequest(t *testing.T, engine http.Handler, token string, method string, path string, body any) apiResponse {
	t.Helper()
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d for %s %s: %s", recorder.Code, method, path, recorder.Body.String())
	}
	var resp apiResponse
	if err = json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("request %s %s failed: %s", method, path, resp.Message)
	}
	return resp
}

func decodeResponseData(t *testing.T, resp apiResponse, target any) {
	t.Helper()
	if err := json.Unmarshal(resp.Data, target); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
}

func toString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
