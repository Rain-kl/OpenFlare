package router_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/model"
	"github.com/rain-kl/openflare/openflare-server/router"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// mockKumaServer simulates Uptime Kuma's Engine.IO/Socket.IO polling endpoints
type mockKumaServer struct {
	mu             sync.Mutex
	postsReceived  []string
	pendingPackets chan string
	monitorList    string // JSON representing map[string]UptimeKumaMonitor
}

func newMockKumaServer(monitorList string) *mockKumaServer {
	return &mockKumaServer{
		pendingPackets: make(chan string, 100),
		monitorList:    monitorList,
	}
}

func (s *mockKumaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	transport := r.URL.Query().Get("transport")
	sid := r.URL.Query().Get("sid")

	if r.Method == "GET" {
		if transport == "polling" && sid == "" {
			// Handshake response
			w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
			_, _ = w.Write([]byte(`0{"sid":"mock-sid"}`))
			return
		}

		if transport == "polling" && sid == "mock-sid" {
			// Long-polling GET request
			w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
			select {
			case pkt := <-s.pendingPackets:
				_, _ = w.Write([]byte(pkt))
			case <-time.After(100 * time.Millisecond):
				_, _ = w.Write([]byte(""))
			}
			return
		}
	} else if r.Method == "POST" {
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyStr := string(bodyBytes)
		s.postsReceived = append(s.postsReceived, bodyStr)

		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
		w.WriteHeader(http.StatusOK)

		if bodyStr == "40" {
			// Namespace Connect event
			// Immediately queue the monitorList payload to be fetched by the next GET poll
			s.pendingPackets <- fmt.Sprintf(`42["monitorList",%s]`, s.monitorList)
			return
		}

		if strings.HasPrefix(bodyStr, "42") {
			// Socket.IO message: 42<ackID>[...]
			payload := bodyStr[2:]
			// Find ack ID (digits at the start of payload)
			digitsEnd := 0
			for digitsEnd < len(payload) && payload[digitsEnd] >= '0' && payload[digitsEnd] <= '9' {
				digitsEnd++
			}
			if digitsEnd == 0 {
				return
			}
			ackIDStr := payload[:digitsEnd]
			jsonArrayStr := payload[digitsEnd:]

			var arr []json.RawMessage
			if err := json.Unmarshal([]byte(jsonArrayStr), &arr); err != nil || len(arr) == 0 {
				return
			}

			var eventName string
			_ = json.Unmarshal(arr[0], &eventName)

			switch eventName {
			case "login", "loginByToken":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true}]", ackIDStr)
			case "getTags":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true,\"tags\":[{\"id\":10,\"name\":\"OpenFlare\",\"color\":\"#4f46e5\"}]}]", ackIDStr)
			case "addTag":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true,\"tag\":{\"id\":10}}]", ackIDStr)
			case "add":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true,\"monitorID\":100}]", ackIDStr)
			case "addMonitorTag":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true}]", ackIDStr)
			case "editMonitor":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true}]", ackIDStr)
			case "deleteMonitor":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true}]", ackIDStr)
			}
		}
	}
}

func TestUptimeKumaSyncDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	loginCookie := loginAsRoot(t, engine)

	// Keep integration disabled
	common.UptimeKumaEnabled = false

	// Request sync, should fail
	req := httptest.NewRequest(http.MethodPost, "/api/uptimekuma/sync", nil)
	req.Header.Set("OpenFlare-Token", loginCookie)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Fatal("expected sync request to fail when integration is disabled")
	}
	if !strings.Contains(resp.Message, "disabled") {
		t.Fatalf("expected error message to mention integration is disabled, got: %s", resp.Message)
	}
}

func TestUptimeKumaSyncSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	// Clean up route table just in case
	_ = model.DB.Where("1 = 1").Delete(&model.ProxyRoute{}).Error

	// Seed proxy routes
	// Route 1: site-a (exists in Uptime Kuma but has different check parameters - should trigger editMonitor)
	routeA := &model.ProxyRoute{
		SiteName:    "site-a",
		Domain:      "site-a.com",
		Domains:     `["site-a.com"]`,
		OriginURL:   "http://10.0.0.1",
		Enabled:     true,
		EnableHTTPS: false,
	}
	// Route 2: site-b (does not exist in Uptime Kuma - should trigger add & addMonitorTag)
	routeB := &model.ProxyRoute{
		SiteName:    "site-b",
		Domain:      "site-b.com",
		Domains:     `["site-b.com"]`,
		OriginURL:   "https://10.0.0.2",
		Enabled:     true,
		EnableHTTPS: true,
	}
	// Route 3: site-c (disabled locally - should NOT be processed/created)
	routeC := &model.ProxyRoute{
		SiteName:    "site-c",
		Domain:      "site-c.com",
		Domains:     `["site-c.com"]`,
		OriginURL:   "http://10.0.0.3",
		Enabled:     false,
		EnableHTTPS: false,
	}

	if err := model.DB.Create(routeA).Error; err != nil {
		t.Fatalf("failed to seed routeA: %v", err)
	}
	if err := model.DB.Create(routeB).Error; err != nil {
		t.Fatalf("failed to seed routeB: %v", err)
	}
	if err := model.DB.Create(routeC).Error; err != nil {
		t.Fatalf("failed to seed routeC: %v", err)
	}

	// Prepare mock monitorList
	// 1. "site-old": tagged with OpenFlare but doesn't exist locally anymore -> should trigger deleteMonitor
	// 2. "site-a": matches routeA but has interval = 30 (default UptimeKumaInterval is 60) -> should trigger editMonitor
	monitorListJSON := `{
		"99": {
			"id": 99,
			"name": "site-old",
			"url": "http://site-old.com",
			"interval": 60,
			"tags": [{"tag_id": 10, "name": "OpenFlare"}]
		},
		"98": {
			"id": 98,
			"name": "site-a",
			"url": "http://site-a.com",
			"interval": 30,
			"tags": [{"tag_id": 10, "name": "OpenFlare"}]
		}
	}`

	mockSrv := newMockKumaServer(monitorListJSON)
	server := httptest.NewServer(mockSrv)
	defer server.Close()

	// Backup and set configs
	oldEnabled := common.UptimeKumaEnabled
	oldUrl := common.UptimeKumaUrl
	oldUsername := common.UptimeKumaUsername
	oldPassword := common.UptimeKumaPassword
	oldScope := common.UptimeKumaMonitorScope
	oldInterval := common.UptimeKumaInterval
	oldRetry := common.UptimeKumaRetry
	oldRetryInterval := common.UptimeKumaRetryInterval
	oldTimeout := common.UptimeKumaTimeout

	common.UptimeKumaEnabled = true
	common.UptimeKumaUrl = server.URL
	common.UptimeKumaUsername = "admin"
	common.UptimeKumaPassword = "password"
	common.UptimeKumaMonitorScope = "all"
	common.UptimeKumaInterval = 60
	common.UptimeKumaRetry = 0
	common.UptimeKumaRetryInterval = 60
	common.UptimeKumaTimeout = 48

	defer func() {
		common.UptimeKumaEnabled = oldEnabled
		common.UptimeKumaUrl = oldUrl
		common.UptimeKumaUsername = oldUsername
		common.UptimeKumaPassword = oldPassword
		common.UptimeKumaMonitorScope = oldScope
		common.UptimeKumaInterval = oldInterval
		common.UptimeKumaRetry = oldRetry
		common.UptimeKumaRetryInterval = oldRetryInterval
		common.UptimeKumaTimeout = oldTimeout
	}()

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	loginCookie := loginAsRoot(t, engine)

	req := httptest.NewRequest(http.MethodPost, "/api/uptimekuma/sync", nil)
	req.Header.Set("OpenFlare-Token", loginCookie)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", recorder.Code, recorder.Body.String())
	}

	var resp apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("sync request failed: %s", resp.Message)
	}

	mockSrv.mu.Lock()
	posts := mockSrv.postsReceived
	mockSrv.mu.Unlock()

	// Verify events received
	hasLogin := false
	hasGetTags := false
	hasAddSiteB := false
	hasTagSiteB := false
	hasEditSiteA := false
	hasDeleteOld := false

	for _, body := range posts {
		if strings.Contains(body, `"login"`) && strings.Contains(body, `"admin"`) && strings.Contains(body, `"password"`) {
			hasLogin = true
		}
		if strings.Contains(body, `"getTags"`) {
			hasGetTags = true
		}
		if strings.Contains(body, `"add"`) && strings.Contains(body, `"site-b"`) && strings.Contains(body, `"https://site-b.com"`) {
			hasAddSiteB = true
		}
		if strings.Contains(body, `"addMonitorTag"`) && strings.Contains(body, `10`) && strings.Contains(body, `100`) {
			hasTagSiteB = true
		}
		if strings.Contains(body, `"editMonitor"`) && strings.Contains(body, `98`) && strings.Contains(body, `"site-a"`) && strings.Contains(body, `"interval":60`) {
			hasEditSiteA = true
		}
		if strings.Contains(body, `"deleteMonitor"`) && strings.Contains(body, `99`) {
			hasDeleteOld = true
		}
	}

	if !hasLogin {
		t.Error("expected login event to be called")
	}
	if !hasGetTags {
		t.Error("expected getTags event to be called")
	}
	if !hasAddSiteB {
		t.Error("expected site-b to be added")
	}
	if !hasTagSiteB {
		t.Error("expected site-b to be tagged")
	}
	if !hasEditSiteA {
		t.Error("expected site-a to be edited/updated")
	}
	if !hasDeleteOld {
		t.Error("expected site-old to be deleted")
	}
}

func TestUptimeKumaSyncSelectedScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	// Clean up route table
	_ = model.DB.Where("1 = 1").Delete(&model.ProxyRoute{}).Error

	// Seed proxy routes
	// Route 1: site-a (enabled, in selected list)
	routeA := &model.ProxyRoute{
		SiteName:    "site-a",
		Domain:      "site-a.com",
		Domains:     `["site-a.com"]`,
		OriginURL:   "http://10.0.0.1",
		Enabled:     true,
		EnableHTTPS: false,
	}
	// Route 2: site-b (enabled, NOT in selected list)
	routeB := &model.ProxyRoute{
		SiteName:    "site-b",
		Domain:      "site-b.com",
		Domains:     `["site-b.com"]`,
		OriginURL:   "http://10.0.0.2",
		Enabled:     true,
		EnableHTTPS: false,
	}

	if err := model.DB.Create(routeA).Error; err != nil {
		t.Fatalf("failed to seed routeA: %v", err)
	}
	if err := model.DB.Create(routeB).Error; err != nil {
		t.Fatalf("failed to seed routeB: %v", err)
	}

	mockSrv := newMockKumaServer(`{}`)
	server := httptest.NewServer(mockSrv)
	defer server.Close()

	// Backup and set configs
	oldEnabled := common.UptimeKumaEnabled
	oldUrl := common.UptimeKumaUrl
	oldUsername := common.UptimeKumaUsername
	oldPassword := common.UptimeKumaPassword
	oldScope := common.UptimeKumaMonitorScope
	oldSelected := common.UptimeKumaSelectedSites

	common.UptimeKumaEnabled = true
	common.UptimeKumaUrl = server.URL
	common.UptimeKumaUsername = "admin"
	common.UptimeKumaPassword = "password"
	common.UptimeKumaMonitorScope = "selected"
	common.UptimeKumaSelectedSites = "site-a" // site-b is excluded

	defer func() {
		common.UptimeKumaEnabled = oldEnabled
		common.UptimeKumaUrl = oldUrl
		common.UptimeKumaUsername = oldUsername
		common.UptimeKumaPassword = oldPassword
		common.UptimeKumaMonitorScope = oldScope
		common.UptimeKumaSelectedSites = oldSelected
	}()

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	loginCookie := loginAsRoot(t, engine)

	req := httptest.NewRequest(http.MethodPost, "/api/uptimekuma/sync", nil)
	req.Header.Set("OpenFlare-Token", loginCookie)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("sync request failed: %s", resp.Message)
	}

	mockSrv.mu.Lock()
	posts := mockSrv.postsReceived
	mockSrv.mu.Unlock()

	hasLogin := false
	hasAddSiteA := false
	hasAddSiteB := false

	for _, body := range posts {
		if strings.Contains(body, `"login"`) && strings.Contains(body, `"admin"`) && strings.Contains(body, `"password"`) {
			hasLogin = true
		}
		if strings.Contains(body, `"add"`) && strings.Contains(body, `"site-a"`) {
			hasAddSiteA = true
		}
		if strings.Contains(body, `"add"`) && strings.Contains(body, `"site-b"`) {
			hasAddSiteB = true
		}
	}

	if !hasLogin {
		t.Error("expected login event to be called")
	}
	if !hasAddSiteA {
		t.Error("expected site-a to be added")
	}
	if hasAddSiteB {
		t.Error("expected site-b NOT to be added (not in selected scope)")
	}
}
