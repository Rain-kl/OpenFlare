package router_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/model"
	"github.com/rain-kl/openflare/openflare-server/router"
	"github.com/rain-kl/openflare/openflare-server/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestPhaseFlaredRoutesUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	heartbeatReq := httptest.NewRequest(http.MethodPost, "/api/flared/heartbeat", bytes.NewReader([]byte(`{}`)))
	heartbeatReq.Header.Set("Content-Type", "application/json")
	heartbeatRec := httptest.NewRecorder()
	engine.ServeHTTP(heartbeatRec, heartbeatReq)
	if heartbeatRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status for missing token, got %d body=%s", heartbeatRec.Code, heartbeatRec.Body.String())
	}

	activeReq := httptest.NewRequest(http.MethodGet, "/api/flared/config/active", nil)
	activeRec := httptest.NewRecorder()
	engine.ServeHTTP(activeRec, activeReq)
	if activeRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status for missing token on active config, got %d", activeRec.Code)
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/flared/apply-log", bytes.NewReader([]byte(`{}`)))
	applyReq.Header.Set("Content-Type", "application/json")
	applyRec := httptest.NewRecorder()
	engine.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status for missing token on apply log, got %d", applyRec.Code)
	}
}

func TestPhaseFlaredRoutesRejectWrongNodeType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	adminToken := prepareRootToken(t)
	createNodeResp := performJSONRequest(t, engine, adminToken, http.MethodPost, "/api/nodes/", map[string]any{
		"name": "edge-for-flared-test",
		"ip":   "10.0.0.20",
	})
	var createdNode service.NodeView
	decodeResponseData(t, createNodeResp, &createdNode)

	heartbeatReq := httptest.NewRequest(http.MethodPost, "/api/flared/heartbeat", bytes.NewReader([]byte(`{}`)))
	heartbeatReq.Header.Set("Content-Type", "application/json")
	heartbeatReq.Header.Set("X-Tunnel-Token", createdNode.AccessToken)
	heartbeatRec := httptest.NewRecorder()
	engine.ServeHTTP(heartbeatRec, heartbeatReq)
	if heartbeatRec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden status for edge_node token, got %d body=%s", heartbeatRec.Code, heartbeatRec.Body.String())
	}
}

func TestPhaseFlaredLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	adminToken := prepareRootToken(t)

	// Create an enabled proxy route that will be served to the flared client
	// through the tunnel upstream flow.
	createRouteAndPublishVersion(t, engine, adminToken)

	// Seed a tunnel_client node directly so we can use its access token as the
	// tunnel_token when calling the flared endpoints.
	tunnelNode := &model.Node{
		NodeID:      "tun-flared-1",
		Name:        "office-flared-1",
		IP:          "192.168.10.20",
		AccessToken: "tunnel-token-phase",
		Status:      service.NodeStatusPending,
		NodeType:    "tunnel_client",
		Version:     "",
	}
	if err := tunnelNode.Insert(); err != nil {
		t.Fatalf("failed to seed tunnel client node: %v", err)
	}

	heartbeatResp := performFlaredJSONRequest(t, engine, tunnelNode.AccessToken, http.MethodPost, "/api/flared/heartbeat", map[string]any{
		"client_version":  "v0.2.0",
		"frp_version":     "0.61.0",
		"tunnel_status":   "running",
		"current_version": "",
	})
	if !heartbeatResp.Success {
		t.Fatalf("flared heartbeat failed: %s", heartbeatResp.Message)
	}
	var heartbeatData service.FlaredHeartbeatResponse
	if err := json.Unmarshal(heartbeatResp.Data, &heartbeatData); err != nil {
		t.Fatalf("failed to decode flared heartbeat response: %v", err)
	}
	if heartbeatData.ActiveConfig == nil {
		t.Fatal("expected heartbeat to return active config summary")
	}
	if heartbeatData.TunnelSettings == nil {
		t.Fatal("expected heartbeat to return tunnel_settings")
	}

	// Re-fetch node and assert status flipped to online.
	updated, err := model.GetNodeByNodeID(tunnelNode.NodeID)
	if err != nil {
		t.Fatalf("failed to reload flared node: %v", err)
	}
	if updated.Status != service.NodeStatusOnline {
		t.Fatalf("expected flared node status to be online, got %q", updated.Status)
	}
	if updated.Version != "v0.2.0" {
		t.Fatalf("expected flared client_version to be stored, got %q", updated.Version)
	}

	activeResp := performFlaredJSONRequest(t, engine, tunnelNode.AccessToken, http.MethodGet, "/api/flared/config/active", nil)
	if !activeResp.Success {
		t.Fatalf("flared get active config failed: %s", activeResp.Message)
	}
	var activeConfig service.FlaredTunnelConfigResponse
	if err := json.Unmarshal(activeResp.Data, &activeConfig); err != nil {
		t.Fatalf("failed to decode flared active config: %v", err)
	}
	if activeConfig.Version == "" || activeConfig.Checksum == "" {
		t.Fatalf("expected flared active config to return version summary, got %+v", activeConfig)
	}

	applyResp := performFlaredJSONRequest(t, engine, tunnelNode.AccessToken, http.MethodPost, "/api/flared/apply-log", map[string]any{
		"version":  activeConfig.Version,
		"result":   service.ApplyResultOK,
		"message":  "apply ok",
		"checksum": activeConfig.Checksum,
	})
	if !applyResp.Success {
		t.Fatalf("flared apply log failed: %s", applyResp.Message)
	}
}

func performFlaredJSONRequest(t *testing.T, engine http.Handler, token string, method string, path string, body any) apiResponse {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Tunnel-Token", token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d for %s %s: %s", recorder.Code, method, path, recorder.Body.String())
	}
	var resp apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("request %s %s failed: %s", method, path, resp.Message)
	}
	return resp
}
