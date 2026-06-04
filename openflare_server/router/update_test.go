package router_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"openflare/common"
	"openflare/router"
	"openflare/service"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestLatestReleaseProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	originalClient := service.UpdateHTTPClientForTest()
	service.SetUpdateHTTPClientForTest(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.github.com/repos/Rain-kl/OpenFlare/releases/latest" {
				t.Fatalf("unexpected request url: %s", req.URL.String())
			}
			if req.Header.Get("Accept") != "application/vnd.github+json" {
				t.Fatalf("unexpected accept header: %s", req.Header.Get("Accept"))
			}
			if req.Header.Get("User-Agent") != "OpenFlare-Server" {
				t.Fatalf("unexpected user-agent header: %s", req.Header.Get("User-Agent"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"tag_name":"v1.2.3",
					"body":"release notes",
					"html_url":"https://github.com/Rain-kl/OpenFlare/releases/tag/v1.2.3",
					"published_at":"2026-03-11T00:00:00Z"
				}`)),
			}, nil
		}),
	})
	t.Cleanup(func() {
		service.SetUpdateHTTPClientForTest(originalClient)
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	loginBody, err := json.Marshal(map[string]string{
		"username": "root",
		"password": "123456",
	})
	if err != nil {
		t.Fatalf("failed to marshal login body: %v", err)
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRecorder := httptest.NewRecorder()
	engine.ServeHTTP(loginRecorder, loginReq)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected login status code: %d", loginRecorder.Code)
	}
	var loginResp apiResponse
	if err = json.Unmarshal(loginRecorder.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	var loginUser struct {
		Token string `json:"token"`
	}
	if err = json.Unmarshal(loginResp.Data, &loginUser); err != nil {
		t.Fatalf("failed to decode login user: %v", err)
	}
	if loginUser.Token == "" {
		t.Fatal("expected OpenFlare-Token after login")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/update/latest-release", nil)
	req.Header.Set("OpenFlare-Token", loginUser.Token)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got message: %s", resp.Message)
	}

	var data map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("failed to decode response data: %v", err)
	}
	if data["tag_name"] != "v1.2.3" {
		t.Fatalf("unexpected tag_name: %#v", data["tag_name"])
	}
	if data["current_version"] != common.Version {
		t.Fatalf("unexpected current_version: %#v", data["current_version"])
	}
}

func loginRootAndBuildEngine(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	loginBody, err := json.Marshal(map[string]string{
		"username": "root",
		"password": "123456",
	})
	if err != nil {
		t.Fatalf("failed to marshal login body: %v", err)
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRecorder := httptest.NewRecorder()
	engine.ServeHTTP(loginRecorder, loginReq)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected login status code: %d", loginRecorder.Code)
	}
	var loginResp apiResponse
	if err = json.Unmarshal(loginRecorder.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	var loginUser struct {
		Token string `json:"token"`
	}
	if err = json.Unmarshal(loginResp.Data, &loginUser); err != nil {
		t.Fatalf("failed to decode login user: %v", err)
	}
	if loginUser.Token == "" {
		t.Fatal("expected OpenFlare-Token after login")
	}

	return engine, loginUser.Token
}

func fakeManualServerBinary(version string) (string, []byte) {
	if runtime.GOOS == "windows" {
		return "openflare-server-test.cmd", []byte("@echo off\r\necho " + version + "\r\n")
	}
	return "openflare-server-test.sh", []byte("#!/bin/sh\necho " + version + "\n")
}

func TestManualUploadRoute(t *testing.T) {
	originalVersion := common.Version
	common.Version = "v0.4.0"
	t.Cleanup(func() {
		common.Version = originalVersion
		service.SetServerBinaryUpgradeExecutorForTest(nil)
		service.SetServerUpgradeDispatchDelayForTest(500 * time.Millisecond)
	})

	engine, token := loginRootAndBuildEngine(t)
	fileName, content := fakeManualServerBinary("v0.5.0")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("binary", fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err = part.Write(content); err != nil {
		t.Fatalf("failed to write upload content: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/update/manual-upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("OpenFlare-Token", token)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var resp apiResponse
	if err = json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected failure response for disabled manual upload feature")
	}
	if resp.Message != "手动升级功能已禁用" {
		t.Fatalf("unexpected failure message: %s", resp.Message)
	}
}

func TestManualUpgradeConfirmRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine, token := loginRootAndBuildEngine(t)

	confirmBody, err := json.Marshal(map[string]string{"upload_token": "fake-token"})
	if err != nil {
		t.Fatalf("failed to marshal confirm body: %v", err)
	}
	confirmReq := httptest.NewRequest(http.MethodPost, "/api/update/manual-upgrade", bytes.NewReader(confirmBody))
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmReq.Header.Set("OpenFlare-Token", token)

	confirmRecorder := httptest.NewRecorder()
	engine.ServeHTTP(confirmRecorder, confirmReq)
	if confirmRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected confirm status code: %d", confirmRecorder.Code)
	}

	var confirmResp apiResponse
	if err = json.Unmarshal(confirmRecorder.Body.Bytes(), &confirmResp); err != nil {
		t.Fatalf("failed to decode confirm response: %v", err)
	}
	if confirmResp.Success {
		t.Fatal("expected failure response for disabled manual upgrade feature")
	}
	if confirmResp.Message != "手动升级功能已禁用" {
		t.Fatalf("unexpected failure message: %s", confirmResp.Message)
	}
}
