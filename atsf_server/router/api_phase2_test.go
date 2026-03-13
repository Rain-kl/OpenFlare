package router_test

import (
	"atsflare/common"
	"atsflare/model"
	"atsflare/router"
	"atsflare/service"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	performSessionJSONRequest(t, engine, loginCookie, http.MethodPut, "/api/option/", map[string]any{
		"key":   "GlobalApiRateLimitNum",
		"value": "450",
	})
	performSessionJSONRequest(t, engine, loginCookie, http.MethodPut, "/api/option/", map[string]any{
		"key":   "GlobalApiRateLimitDuration",
		"value": "240",
	})
	performSessionJSONRequest(t, engine, loginCookie, http.MethodPut, "/api/option/", map[string]any{
		"key":   "CriticalRateLimitNum",
		"value": "150",
	})
	performSessionJSONRequest(t, engine, loginCookie, http.MethodPut, "/api/option/", map[string]any{
		"key":   "CriticalRateLimitDuration",
		"value": "900",
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

	unauthorizedRequest := httptest.NewRequest(http.MethodPost, "/api/agent/nodes/register", bytes.NewReader([]byte(`{}`)))
	unauthorizedRecorder := httptest.NewRecorder()
	engine.ServeHTTP(unauthorizedRecorder, unauthorizedRequest)
	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status for missing discovery token, got %d", unauthorizedRecorder.Code)
	}

	createdNodeResp := performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/nodes/", map[string]any{
		"name": "shanghai-edge-1",
	})
	var createdNode service.NodeView
	decodeResponseData(t, createdNodeResp, &createdNode)
	if createdNode.AgentToken == "" || createdNode.Status != service.NodeStatusPending {
		t.Fatal("expected created node to expose agent token with pending status")
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
	resp := performAgentJSONRequestWithToken(t, engine, createdNode.AgentToken, http.MethodPost, "/api/agent/nodes/heartbeat", heartbeatPayload)
	var registeredNode model.Node
	decodeResponseData(t, resp, &registeredNode)
	if registeredNode.IP != "10.0.0.9" || registeredNode.AgentVersion != "0.1.1" || registeredNode.NodeID != createdNode.NodeID {
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

	logsResp := performJSONRequest(t, engine, adminToken, http.MethodGet, "/api/apply-logs/?node_id="+createdNode.NodeID, nil)
	var logs []model.ApplyLog
	decodeResponseData(t, logsResp, &logs)
	if len(logs) != 2 {
		t.Fatalf("expected 2 apply logs, got %d", len(logs))
	}

	updatedNodeResp := performJSONRequest(t, engine, adminToken, http.MethodPut, "/api/nodes/"+toString(createdNode.ID), map[string]any{
		"name": "shanghai-edge-1-renamed",
	})
	decodeResponseData(t, updatedNodeResp, &createdNode)
	if createdNode.Name != "shanghai-edge-1-renamed" {
		t.Fatal("expected node name to be editable")
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

	deleteResp := performJSONRequest(t, engine, adminToken, http.MethodDelete, "/api/nodes/"+toString(createdNode.ID), nil)
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
		"domain":     "preview.example.com",
		"origin_url": "https://origin-a.internal",
		"enabled":    true,
		"custom_headers": []map[string]any{
			{"key": "X-Trace-Id", "value": "$request_id"},
		},
	})
	var createdRoute model.ProxyRoute
	decodeResponseData(t, createResp, &createdRoute)
	if !strings.Contains(createdRoute.CustomHeaders, "X-Trace-Id") {
		t.Fatalf("expected custom headers to be stored as json, got %s", createdRoute.CustomHeaders)
	}

	performJSONRequest(t, engine, token, http.MethodPost, "/api/config-versions/publish", nil)

	performJSONRequest(t, engine, token, http.MethodPut, "/api/proxy-routes/"+toString(createdRoute.ID), map[string]any{
		"domain":     "preview.example.com",
		"origin_url": "https://origin-b.internal",
		"enabled":    true,
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
	if !strings.Contains(renderedConfig, `proxy_set_header X-Release "candidate";`) {
		t.Fatalf("expected preview endpoint to return custom header, got %s", renderedConfig)
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

	resp := performAgentJSONRequestWithToken(t, engine, bootstrap.DiscoveryToken, http.MethodPost, "/api/agent/nodes/register", map[string]any{
		"node_id":         "local-node-id",
		"name":            "bulk-edge-1",
		"ip":              "10.0.0.18",
		"agent_version":   "0.2.0",
		"nginx_version":   "1.25.5",
		"current_version": "",
		"last_error":      "",
	})
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
}

func performAgentJSONRequestWithToken(t *testing.T, engine http.Handler, token string, method string, path string, body any) apiResponse {
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
