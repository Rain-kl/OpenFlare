package router_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"openflare/common"
	"openflare/model"
	"openflare/router"
	"openflare/service"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestPhase2RateLimitOptionsHotReload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)
	model.InitOptionMap()

	oldGlobalApiRateLimitNum := common.GlobalApiRateLimitNum
	oldGlobalApiRateLimitDuration := common.GlobalApiRateLimitDuration
	oldCriticalRateLimitNum := common.CriticalRateLimitNum
	oldCriticalRateLimitDuration := common.CriticalRateLimitDuration
	t.Cleanup(func() {
		common.GlobalApiRateLimitNum = oldGlobalApiRateLimitNum
		common.GlobalApiRateLimitDuration = oldGlobalApiRateLimitDuration
		common.CriticalRateLimitNum = oldCriticalRateLimitNum
		common.CriticalRateLimitDuration = oldCriticalRateLimitDuration
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	loginCookie := loginAsRoot(t, engine)

	performSessionJSONRequest(t, engine, loginCookie, http.MethodPost, "/api/option/update-batch", map[string]any{
		"options": []map[string]any{
			{
				"key":   "GlobalApiRateLimitNum",
				"value": "450",
			},
			{
				"key":   "GlobalApiRateLimitDuration",
				"value": "240",
			},
			{
				"key":   "CriticalRateLimitNum",
				"value": "150",
			},
			{
				"key":   "CriticalRateLimitDuration",
				"value": "900",
			},
		},
	})

	if common.GlobalApiRateLimitNum != 450 {
		t.Fatalf("expected GlobalApiRateLimitNum to be hot reloaded, got %d", common.GlobalApiRateLimitNum)
	}
	if common.GlobalApiRateLimitDuration != 240 {
		t.Fatalf("expected GlobalApiRateLimitDuration to be hot reloaded, got %d", common.GlobalApiRateLimitDuration)
	}
	if common.CriticalRateLimitNum != 150 {
		t.Fatalf("expected CriticalRateLimitNum to be hot reloaded, got %d", common.CriticalRateLimitNum)
	}
	if common.CriticalRateLimitDuration != 900 {
		t.Fatalf("expected CriticalRateLimitDuration to be hot reloaded, got %d", common.CriticalRateLimitDuration)
	}

	resp := performSessionJSONRequest(t, engine, loginCookie, http.MethodGet, "/api/option/", nil)
	var options []model.Option
	decodeResponseData(t, resp, &options)

	optionMap := make(map[string]string, len(options))
	for _, option := range options {
		optionMap[option.Key] = option.Value
	}

	if optionMap["GlobalApiRateLimitNum"] != "450" {
		t.Fatalf("expected option payload to include GlobalApiRateLimitNum=450, got %q", optionMap["GlobalApiRateLimitNum"])
	}
	if optionMap["CriticalRateLimitDuration"] != "900" {
		t.Fatalf("expected option payload to include CriticalRateLimitDuration=900, got %q", optionMap["CriticalRateLimitDuration"])
	}
}

func TestPhase2BatchOptionUpdateIsAtomic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)
	model.InitOptionMap()

	oldGlobalAPI := common.GlobalApiRateLimitNum
	t.Cleanup(func() {
		common.GlobalApiRateLimitNum = oldGlobalAPI
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	loginCookie := loginAsRoot(t, engine)

	payload, err := json.Marshal(map[string]any{
		"options": []map[string]any{
			{
				"key":   "GlobalApiRateLimitNum",
				"value": "451",
			},
			{
				"key":   "CriticalRateLimitDuration",
				"value": "1800",
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal batch payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/option/update-batch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginCookie)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}

	var resp apiResponse
	if err = json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected invalid batch update to fail")
	}

	if common.GlobalApiRateLimitNum != oldGlobalAPI {
		t.Fatalf("expected GlobalApiRateLimitNum to remain %d after failed batch, got %d", oldGlobalAPI, common.GlobalApiRateLimitNum)
	}

	resp = performSessionJSONRequest(t, engine, loginCookie, http.MethodGet, "/api/option/", nil)
	var options []model.Option
	decodeResponseData(t, resp, &options)

	optionMap := make(map[string]string, len(options))
	for _, option := range options {
		optionMap[option.Key] = option.Value
	}

	if optionMap["GlobalApiRateLimitNum"] == "451" {
		t.Fatal("expected failed batch update to avoid persisting partial values")
	}
}

func TestPhase2BatchOptionUpdateValidatesMergedState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)
	model.InitOptionMap()

	oldGitHubClientID := common.GitHubClientId
	oldGitHubOAuthEnabled := common.GitHubOAuthEnabled
	t.Cleanup(func() {
		common.GitHubClientId = oldGitHubClientID
		common.GitHubOAuthEnabled = oldGitHubOAuthEnabled
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	loginCookie := loginAsRoot(t, engine)

	performSessionJSONRequest(t, engine, loginCookie, http.MethodPost, "/api/option/update-batch", map[string]any{
		"options": []map[string]any{
			{
				"key":   "GitHubClientId",
				"value": "client-id-from-batch",
			},
			{
				"key":   "GitHubOAuthEnabled",
				"value": "true",
			},
		},
	})

	if common.GitHubClientId != "client-id-from-batch" {
		t.Fatalf("expected GitHubClientId to be updated from batch, got %q", common.GitHubClientId)
	}
	if !common.GitHubOAuthEnabled {
		t.Fatal("expected GitHubOAuthEnabled to be enabled by merged batch state")
	}
}

func loginAsRoot(t *testing.T, engine http.Handler) *http.Cookie {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"username": "root",
		"password": "123456",
	})
	if err != nil {
		t.Fatalf("failed to marshal login payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected login status %d: %s", recorder.Code, recorder.Body.String())
	}

	var resp apiResponse
	if err = json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("root login failed: %s", resp.Message)
	}

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "session" {
			return cookie
		}
	}
	t.Fatal("expected session cookie after root login")
	return nil
}

func performSessionJSONRequest(t *testing.T, engine http.Handler, sessionCookie *http.Cookie, method string, path string, body any) apiResponse {
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
	req.AddCookie(sessionCookie)

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

func TestPhase2AgentLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	adminToken := prepareRootToken(t)

	createRouteAndPublishVersion(t, engine, adminToken)

	dashboardResp := performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/dashboard/overview", nil)
	var dashboard struct {
		Summary service.DashboardSummary `json:"summary"`
	}
	decodeResponseData(t, dashboardResp, &dashboard)
	if dashboard.Summary.TotalNodes != 0 {
		t.Fatalf("expected empty dashboard node summary before node registration, got %+v", dashboard.Summary)
	}

	unauthorizedRequest := httptest.NewRequest(http.MethodPost, "/api/agent/nodes/register", bytes.NewReader([]byte(`{}`)))
	unauthorizedRecorder := httptest.NewRecorder()
	engine.ServeHTTP(unauthorizedRecorder, unauthorizedRequest)
	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status for missing discovery token, got %d", unauthorizedRecorder.Code)
	}

	createdNodeResp := performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/nodes/", map[string]any{
		"name":                "shanghai-edge-1",
		"geo_manual_override": true,
		"geo_name":            "Shanghai",
		"geo_latitude":        31.2304,
		"geo_longitude":       121.4737,
	})
	var createdNode service.NodeView
	decodeResponseData(t, createdNodeResp, &createdNode)
	if createdNode.AgentToken == "" || createdNode.Status != service.NodeStatusPending {
		t.Fatal("expected created node to expose agent token with pending status")
	}
	if createdNode.GeoName != "Shanghai" || createdNode.GeoLatitude == nil || createdNode.GeoLongitude == nil {
		t.Fatalf("expected created node to expose geo metadata, got %+v", createdNode)
	}

	heartbeatPayload := map[string]any{
		"node_id":           "spoofed-node-id",
		"name":              "shanghai-edge-1",
		"ip":                "10.0.0.9",
		"agent_version":     "0.1.1",
		"nginx_version":     "1.27.1.2",
		"openresty_status":  service.OpenrestyStatusUnhealthy,
		"openresty_message": "docker run openresty failed: bind 80 already allocated",
		"current_version":   "",
		"last_error":        "",
	}
	resp := performAgentJSONRequestWithTokenAndRemote(t, engine, createdNode.AgentToken, http.MethodPost, "/api/agent/nodes/heartbeat", heartbeatPayload, "198.51.100.10:1234")
	var registeredNode model.Node
	decodeResponseData(t, resp, &registeredNode)
	if registeredNode.IP != "198.51.100.10" || registeredNode.AgentVersion != "0.1.1" || registeredNode.NodeID != createdNode.NodeID {
		t.Fatal("expected heartbeat to update node metadata")
	}
	if registeredNode.OpenrestyStatus != service.OpenrestyStatusUnhealthy {
		t.Fatal("expected heartbeat to update openresty status")
	}

	activeConfigResp := performAgentJSONRequestWithToken(t, engine, createdNode.AgentToken, http.MethodGet, "/api/agent/config-versions/active", nil)
	var activeConfig service.AgentConfigResponse
	decodeResponseData(t, activeConfigResp, &activeConfig)
	if activeConfig.Version == "" || activeConfig.RenderedConfig == "" || activeConfig.Checksum == "" {
		t.Fatal("expected active config response to contain version payload")
	}

	successApplyResp := performAgentJSONRequestWithToken(t, engine, createdNode.AgentToken, http.MethodPost, "/api/agent/apply-logs", map[string]any{
		"node_id": "spoofed-node-id",
		"version": activeConfig.Version,
		"result":  service.ApplyResultOK,
		"message": "apply ok",
	})
	var successApplyLog model.ApplyLog
	decodeResponseData(t, successApplyResp, &successApplyLog)
	if successApplyLog.Result != service.ApplyResultOK {
		t.Fatal("expected apply log success to be recorded")
	}

	failedApplyResp := performAgentJSONRequestWithToken(t, engine, createdNode.AgentToken, http.MethodPost, "/api/agent/apply-logs", map[string]any{
		"node_id": "spoofed-node-id",
		"version": activeConfig.Version,
		"result":  service.ApplyResultFailed,
		"message": "openresty reload failed",
	})
	var failedApplyLog model.ApplyLog
	decodeResponseData(t, failedApplyResp, &failedApplyLog)
	if failedApplyLog.Result != service.ApplyResultFailed {
		t.Fatal("expected failed apply log to be recorded")
	}

	nodesResp := performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/nodes/", nil)
	var nodes []service.NodeView
	decodeResponseData(t, nodesResp, &nodes)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Status != service.NodeStatusOnline {
		t.Fatal("expected registered node to become online")
	}
	if nodes[0].AgentToken != createdNode.AgentToken {
		t.Fatal("expected node auth token to remain stable after occupancy")
	}
	if nodes[0].LatestApplyResult != service.ApplyResultFailed || nodes[0].LatestApplyMessage != "openresty reload failed" {
		t.Fatal("expected node list to expose latest apply status")
	}
	if nodes[0].CurrentVersion != activeConfig.Version {
		t.Fatal("expected node current_version to remain at last successful version")
	}
	if nodes[0].LastError != "openresty reload failed" {
		t.Fatal("expected node last_error to reflect failed apply")
	}
	if nodes[0].OpenrestyStatus != service.OpenrestyStatusUnhealthy {
		t.Fatal("expected node list to expose openresty status")
	}
	if nodes[0].OpenrestyMessage != "docker run openresty failed: bind 80 already allocated" {
		t.Fatal("expected node list to expose openresty message")
	}

	if err := model.DB.Create(&model.NodeHealthEvent{
		NodeID:           createdNode.NodeID,
		EventType:        "openresty_down",
		Severity:         service.NodeHealthSeverityCritical,
		Status:           service.NodeHealthEventStatusActive,
		Message:          "docker run openresty failed: bind 80 already allocated",
		FirstTriggeredAt: time.Now().Add(-2 * time.Minute),
		LastTriggeredAt:  time.Now().Add(-time.Minute),
		ReportedAt:       time.Now().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("failed to insert node health event: %v", err)
	}

	observabilityResp := performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/nodes/"+toString(createdNode.ID)+"/observability?hours=24&limit=20", nil)
	var observability service.NodeObservabilityView
	decodeResponseData(t, observabilityResp, &observability)
	if observability.NodeID != createdNode.NodeID {
		t.Fatalf("expected observability response for node %s, got %s", createdNode.NodeID, observability.NodeID)
	}
	if len(observability.HealthEvents) != 1 {
		t.Fatalf("expected observability response to include health events, got %+v", observability.HealthEvents)
	}

	cleanupHealthResp := performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/nodes/"+toString(createdNode.ID)+"/observability/cleanup", nil)
	var cleanupHealthResult service.NodeHealthEventCleanupResult
	decodeResponseData(t, cleanupHealthResp, &cleanupHealthResult)
	if cleanupHealthResult.NodeID != createdNode.NodeID || cleanupHealthResult.DeletedCount != 1 {
		t.Fatalf("unexpected node health cleanup result: %+v", cleanupHealthResult)
	}

	observabilityAfterCleanupResp := performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/nodes/"+toString(createdNode.ID)+"/observability?hours=24&limit=20", nil)
	decodeResponseData(t, observabilityAfterCleanupResp, &observability)
	if len(observability.HealthEvents) != 0 {
		t.Fatalf("expected health events to be cleaned up, got %+v", observability.HealthEvents)
	}

	restartResp := performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/nodes/"+toString(createdNode.ID)+"/openresty-restart", nil)
	decodeResponseData(t, restartResp, &createdNode)
	if !createdNode.RestartOpenrestyRequested {
		t.Fatal("expected openresty restart request flag to be set")
	}

	rawHeartbeatPayload, err := json.Marshal(heartbeatPayload)
	if err != nil {
		t.Fatalf("failed to marshal heartbeat payload: %v", err)
	}
	restartHeartbeatReq := httptest.NewRequest(http.MethodPost, "/api/agent/nodes/heartbeat", bytes.NewReader(rawHeartbeatPayload))
	restartHeartbeatReq.Header.Set("Content-Type", "application/json")
	restartHeartbeatReq.Header.Set("X-Agent-Token", createdNode.AgentToken)
	restartHeartbeatReq.RemoteAddr = "198.51.100.10:1234"
	restartHeartbeatRecorder := httptest.NewRecorder()
	engine.ServeHTTP(restartHeartbeatRecorder, restartHeartbeatReq)
	if restartHeartbeatRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected heartbeat status %d: %s", restartHeartbeatRecorder.Code, restartHeartbeatRecorder.Body.String())
	}
	var restartHeartbeatBody struct {
		Success       bool                      `json:"success"`
		Message       string                    `json:"message"`
		AgentSettings service.AgentSettings     `json:"agent_settings"`
		ActiveConfig  *service.ActiveConfigMeta `json:"active_config"`
	}
	if err = json.Unmarshal(restartHeartbeatRecorder.Body.Bytes(), &restartHeartbeatBody); err != nil {
		t.Fatalf("failed to decode heartbeat response: %v", err)
	}
	if !restartHeartbeatBody.Success {
		t.Fatalf("expected heartbeat request success, got %s", restartHeartbeatBody.Message)
	}
	if !restartHeartbeatBody.AgentSettings.RestartOpenrestyNow {
		t.Fatal("expected heartbeat response to instruct openresty restart")
	}
	if restartHeartbeatBody.ActiveConfig == nil || restartHeartbeatBody.ActiveConfig.Version == "" || restartHeartbeatBody.ActiveConfig.Checksum == "" {
		t.Fatal("expected heartbeat response to include active config summary")
	}

	logsResp := performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/apply-logs/?node_id="+createdNode.NodeID+"&pageNo=1&pageSize=1", nil)
	var logs service.ApplyLogListResult
	decodeResponseData(t, logsResp, &logs)
	if logs.Current != 1 || logs.Total != 2 || logs.TotalPage != 2 {
		t.Fatalf("unexpected paged apply logs result: %+v", logs)
	}
	if len(logs.Rows) != 1 {
		t.Fatalf("expected 1 apply log row on page 1, got %d", len(logs.Rows))
	}
	if logs.Rows[0].Result != service.ApplyResultFailed {
		t.Fatalf("expected newest apply log first, got %s", logs.Rows[0].Result)
	}
	oldApplyLogTime := time.Now().Add(-48 * time.Hour)
	if err := model.DB.Model(&model.ApplyLog{}).Where("id = ?", successApplyLog.ID).Update("created_at", oldApplyLogTime).Error; err != nil {
		t.Fatalf("failed to backdate apply log: %v", err)
	}
	cleanupResp := performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/apply-logs/cleanup", map[string]any{
		"retention_days": 1,
	})
	var cleanupResult service.ApplyLogCleanupResult
	decodeResponseData(t, cleanupResp, &cleanupResult)
	if cleanupResult.DeleteAll {
		t.Fatal("expected retention cleanup instead of delete-all cleanup")
	}
	if cleanupResult.RetentionDays != 1 || cleanupResult.DeletedCount != 1 {
		t.Fatalf("unexpected cleanup result: %+v", cleanupResult)
	}
	postCleanupResp := performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/apply-logs/?node_id="+createdNode.NodeID, nil)
	decodeResponseData(t, postCleanupResp, &logs)
	if logs.Total != 1 || len(logs.Rows) != 1 {
		t.Fatalf("expected one apply log after retention cleanup, got %+v", logs)
	}
	deleteAllResp := performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/apply-logs/cleanup", map[string]any{
		"delete_all": true,
	})
	decodeResponseData(t, deleteAllResp, &cleanupResult)
	if !cleanupResult.DeleteAll || cleanupResult.DeletedCount != 1 {
		t.Fatalf("unexpected delete-all cleanup result: %+v", cleanupResult)
	}
	emptyLogsResp := performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/apply-logs/?node_id="+createdNode.NodeID, nil)
	decodeResponseData(t, emptyLogsResp, &logs)
	if logs.Total != 0 || len(logs.Rows) != 0 || logs.Current != 1 || logs.TotalPage != 0 {
		t.Fatalf("expected empty apply log page after delete-all cleanup, got %+v", logs)
	}

	updatedNodeResp := performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/nodes/"+toString(createdNode.ID)+"/update", map[string]any{
		"name":                "shanghai-edge-1-renamed",
		"geo_manual_override": true,
		"geo_name":            "Tokyo",
		"geo_latitude":        35.6762,
		"geo_longitude":       139.6503,
	})
	decodeResponseData(t, updatedNodeResp, &createdNode)
	if createdNode.Name != "shanghai-edge-1-renamed" {
		t.Fatal("expected node name to be editable")
	}
	if createdNode.GeoName != "Tokyo" || createdNode.GeoLatitude == nil || createdNode.GeoLongitude == nil {
		t.Fatalf("expected node geo metadata to be editable, got %+v", createdNode)
	}

	oldTime := time.Now().Add(-common.NodeOfflineThreshold - time.Minute)
	if err := model.DB.Model(&model.Node{}).Where("node_id = ?", createdNode.NodeID).Update("last_seen_at", oldTime).Error; err != nil {
		t.Fatalf("failed to update node last_seen_at: %v", err)
	}
	nodesResp = performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/nodes/", nil)
	decodeResponseData(t, nodesResp, &nodes)
	if nodes[0].Status != service.NodeStatusOffline {
		t.Fatal("expected node to be shown as offline after timeout")
	}

	deleteResp := performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/nodes/"+toString(createdNode.ID)+"/delete", nil)
	if !deleteResp.Success {
		t.Fatalf("expected delete node success, got %s", deleteResp.Message)
	}

	deniedReq := httptest.NewRequest(http.MethodPost, "/api/agent/nodes/heartbeat", bytes.NewReader([]byte(`{"ip":"10.0.0.9","agent_version":"0.1.1"}`)))
	deniedReq.Header.Set("Content-Type", "application/json")
	deniedReq.Header.Set("X-Agent-Token", createdNode.AgentToken)
	deniedRecorder := httptest.NewRecorder()
	engine.ServeHTTP(deniedRecorder, deniedReq)
	if deniedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected deleted node token to be rejected, got %d", deniedRecorder.Code)
	}
}

func TestPhase2CustomHeadersPreviewAndDiffLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	token := prepareRootToken(t)

	createResp := performJSONRequest(t, engine, token, http.MethodPost, "/api/proxy-routes/", map[string]any{
		"domain":      "preview.example.com",
		"origin_url":  "https://origin-a.internal",
		"origin_host": "preview-origin.internal",
		"enabled":     true,
		"custom_headers": []map[string]any{
			{"key": "X-Trace-Id", "value": "$request_id"},
		},
	})
	var createdRoute service.ProxyRouteView
	decodeResponseData(t, createResp, &createdRoute)
	if !strings.Contains(createdRoute.CustomHeaders, "X-Trace-Id") {
		t.Fatalf("expected custom headers to be stored as json, got %s", createdRoute.CustomHeaders)
	}
	if createdRoute.OriginHost != "preview-origin.internal" {
		t.Fatalf("expected origin_host to be stored, got %s", createdRoute.OriginHost)
	}
	if createdRoute.SiteName != "preview.example.com" || createdRoute.PrimaryDomain != "preview.example.com" || createdRoute.DomainCount != 1 {
		t.Fatalf("expected website identity fields in create response, got %+v", createdRoute)
	}

	performJSONRequest(t, engine, token, http.MethodPost, "/api/config-versions/publish", nil)

	performJSONRequest(t, engine, token, http.MethodPost, "/api/proxy-routes/"+toString(createdRoute.ID)+"/update", map[string]any{
		"domain":      "preview.example.com",
		"origin_url":  "https://origin-b.internal",
		"origin_host": "preview-upstream.internal",
		"enabled":     true,
		"custom_headers": []map[string]any{
			{"key": "X-Trace-Id", "value": "$request_id"},
			{"key": "X-Release", "value": "candidate"},
		},
	})
	performJSONRequest(t, engine, token, http.MethodPost, "/api/proxy-routes/", map[string]any{
		"domain":     "new-preview.example.com",
		"origin_url": "https://origin-new.internal",
		"enabled":    true,
	})

	previewResp := performJSONRequest(t, engine, token, http.MethodGet, "/api/config-versions/preview", nil)
	var preview map[string]any
	decodeResponseData(t, previewResp, &preview)
	renderedConfig, _ := preview["rendered_config"].(string)
	if websiteCount, ok := preview["website_count"].(float64); !ok || int(websiteCount) != 2 {
		t.Fatalf("expected preview website_count=2, got %#v", preview["website_count"])
	}
	if !strings.Contains(renderedConfig, `proxy_set_header X-Release "candidate";`) {
		t.Fatalf("expected preview endpoint to return custom header, got %s", renderedConfig)
	}
	if !strings.Contains(renderedConfig, `proxy_set_header Host "preview-upstream.internal";`) {
		t.Fatalf("expected preview endpoint to return overridden host header, got %s", renderedConfig)
	}
	if !strings.Contains(renderedConfig, "proxy_ssl_server_name on;") {
		t.Fatalf("expected preview endpoint to enable proxy ssl server name, got %s", renderedConfig)
	}
	if !strings.Contains(renderedConfig, `proxy_ssl_name "preview-upstream.internal";`) {
		t.Fatalf("expected preview endpoint to return proxy ssl name, got %s", renderedConfig)
	}

	diffResp := performJSONRequest(t, engine, token, http.MethodGet, "/api/config-versions/diff", nil)
	var diff map[string]any
	decodeResponseData(t, diffResp, &diff)
	modifiedDomains, ok := diff["modified_domains"].([]any)
	if !ok || len(modifiedDomains) != 1 || modifiedDomains[0].(string) != "preview.example.com" {
		t.Fatalf("unexpected modified domains: %#v", diff["modified_domains"])
	}
	addedDomains, ok := diff["added_domains"].([]any)
	if !ok || len(addedDomains) != 1 || addedDomains[0].(string) != "new-preview.example.com" {
		t.Fatalf("unexpected added domains: %#v", diff["added_domains"])
	}
	modifiedSites, ok := diff["modified_sites"].([]any)
	if !ok || len(modifiedSites) != 1 || modifiedSites[0].(string) != "preview.example.com" {
		t.Fatalf("unexpected modified sites: %#v", diff["modified_sites"])
	}
	addedSites, ok := diff["added_sites"].([]any)
	if !ok || len(addedSites) != 1 || addedSites[0].(string) != "new-preview.example.com" {
		t.Fatalf("unexpected added sites: %#v", diff["added_sites"])
	}
}

func TestPhase2ProxyRouteWebsiteDetailAndLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	token := prepareRootToken(t)

	createResp := performJSONRequest(t, engine, token, http.MethodPost, "/api/proxy-routes/", map[string]any{
		"site_name":             "marketing-site",
		"domains":               []string{"app.example.com", "www.example.com"},
		"origin_url":            "https://origin.internal",
		"enabled":               true,
		"limit_conn_per_server": 120,
		"limit_conn_per_ip":     12,
		"limit_rate":            "512K",
	})
	var createdRoute service.ProxyRouteView
	decodeResponseData(t, createResp, &createdRoute)
	if createdRoute.SiteName != "marketing-site" || createdRoute.PrimaryDomain != "app.example.com" {
		t.Fatalf("unexpected create payload: %+v", createdRoute)
	}
	if createdRoute.DomainCount != 2 || len(createdRoute.Domains) != 2 || createdRoute.Domains[1] != "www.example.com" {
		t.Fatalf("expected multi-domain website view, got %+v", createdRoute)
	}
	if createdRoute.LimitConnPerServer != 120 || createdRoute.LimitConnPerIP != 12 || createdRoute.LimitRate != "512k" {
		t.Fatalf("expected normalized rate limit fields, got %+v", createdRoute)
	}
	if len(createdRoute.UpstreamList) != 1 || createdRoute.UpstreamList[0] != "https://origin.internal" {
		t.Fatalf("expected structured upstream list, got %+v", createdRoute.UpstreamList)
	}

	detailResp := performJSONRequest(t, engine, token, http.MethodGet, "/api/proxy-routes/"+toString(createdRoute.ID), nil)
	var detail service.ProxyRouteView
	decodeResponseData(t, detailResp, &detail)
	if detail.ID != createdRoute.ID || detail.SiteName != "marketing-site" || detail.LimitRate != "512k" {
		t.Fatalf("unexpected detail response: %+v", detail)
	}
	if len(detail.Domains) != 2 || detail.Domains[0] != "app.example.com" || detail.Domains[1] != "www.example.com" {
		t.Fatalf("expected detail response to expose full domain list, got %+v", detail.Domains)
	}

	listResp := performJSONRequest(t, engine, token, http.MethodGet, "/api/proxy-routes/", nil)
	var routes []service.ProxyRouteView
	decodeResponseData(t, listResp, &routes)
	if len(routes) != 1 || routes[0].SiteName != "marketing-site" || routes[0].LimitConnPerServer != 120 {
		t.Fatalf("unexpected proxy route list response: %+v", routes)
	}
}

func TestPhase2GlobalDiscoveryRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	adminToken := prepareRootToken(t)
	bootstrapResp := performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/nodes/bootstrap-token", nil)
	var bootstrap service.NodeBootstrapView
	decodeResponseData(t, bootstrapResp, &bootstrap)
	if bootstrap.DiscoveryToken == "" {
		t.Fatal("expected global discovery token to be available")
	}

	resp := performAgentJSONRequestWithTokenAndRemote(t, engine, bootstrap.DiscoveryToken, http.MethodPost, "/api/agent/nodes/register", map[string]any{
		"node_id":         "local-node-id",
		"name":            "bulk-edge-1",
		"ip":              "10.0.0.18",
		"agent_version":   "0.2.0",
		"nginx_version":   "1.25.5",
		"current_version": "",
		"last_error":      "",
	}, "203.0.113.18:4321")
	var registration service.AgentRegistrationResponse
	decodeResponseData(t, resp, &registration)
	if registration.AgentToken == "" || registration.NodeID == "" {
		t.Fatal("expected discovery registration to issue node-specific agent token")
	}

	nodesResp := performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/nodes/", nil)
	var nodes []service.NodeView
	decodeResponseData(t, nodesResp, &nodes)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 discovered node, got %d", len(nodes))
	}
	if nodes[0].Name != "bulk-edge-1" || nodes[0].AgentToken != registration.AgentToken || nodes[0].Status != service.NodeStatusOnline {
		t.Fatal("expected discovered node to be created online with issued agent token")
	}
	if nodes[0].IP != "203.0.113.18" {
		t.Fatalf("expected discovered node to keep public source ip, got %s", nodes[0].IP)
	}
}

func performAgentJSONRequestWithToken(t *testing.T, engine http.Handler, token string, method string, path string, body any) apiResponse {
	return performAgentJSONRequestWithTokenAndRemote(t, engine, token, method, path, body, "")
}

func performAgentJSONRequestWithTokenAndRemote(t *testing.T, engine http.Handler, token string, method string, path string, body any, remoteAddr string) apiResponse {
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
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	req.Header.Set("X-Agent-Token", token)
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

func createRouteAndPublishVersion(t *testing.T, engine http.Handler, adminToken string) {
	t.Helper()
	createBody := map[string]any{
		"domain":     "agent.example.com",
		"origin_url": "https://agent-origin.internal",
		"enabled":    true,
		"remark":     "agent route",
	}
	performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/proxy-routes/", createBody)
	performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/config-versions/publish", nil)
}
