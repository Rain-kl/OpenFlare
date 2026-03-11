package router_test

import (
	"atsflare/common"
	"atsflare/router"
	"atsflare/service"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
			if req.URL.String() != "https://api.github.com/repos/Rain-kl/ATSFlare/releases/latest" {
				t.Fatalf("unexpected request url: %s", req.URL.String())
			}
			if req.Header.Get("Accept") != "application/vnd.github+json" {
				t.Fatalf("unexpected accept header: %s", req.Header.Get("Accept"))
			}
			if req.Header.Get("User-Agent") != "ATSFlare-Server" {
				t.Fatalf("unexpected user-agent header: %s", req.Header.Get("User-Agent"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"tag_name":"v1.2.3",
					"body":"release notes",
					"html_url":"https://github.com/Rain-kl/ATSFlare/releases/tag/v1.2.3",
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

	req := httptest.NewRequest(http.MethodGet, "/api/update/latest-release", nil)
	for _, cookieValue := range loginRecorder.Result().Cookies() {
		req.AddCookie(cookieValue)
	}

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
