package router_test

import (
	"atsflare/common"
	"atsflare/model"
	"atsflare/router"
	"atsflare/service"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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
	if version1.MainConfig == "" {
		t.Fatal("expected published version to contain main config")
	}

	repeatPublishReq := httptest.NewRequest(http.MethodPost, "/api/config-versions/publish", nil)
	repeatPublishReq.Header.Set("Authorization", "Bearer "+token)
	repeatPublishRecorder := httptest.NewRecorder()
	engine.ServeHTTP(repeatPublishRecorder, repeatPublishReq)
	if repeatPublishRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d for repeated publish: %s", repeatPublishRecorder.Code, repeatPublishRecorder.Body.String())
	}
	var repeatPublishResp apiResponse
	if err := json.Unmarshal(repeatPublishRecorder.Body.Bytes(), &repeatPublishResp); err != nil {
		t.Fatalf("failed to unmarshal repeated publish response: %v", err)
	}
	if repeatPublishResp.Success {
		t.Fatal("expected repeated publish without route changes to be rejected")
	}
	if !strings.Contains(repeatPublishResp.Message, "当前规则没有变更") {
		t.Fatalf("unexpected repeated publish message: %s", repeatPublishResp.Message)
	}

	initialSnapshot := version1.SnapshotJSON
	initialMainConfig := version1.MainConfig
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
	if storedVersion1.MainConfig != initialMainConfig {
		t.Fatal("expected version1 main config to remain immutable")
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

func TestPhase1HTTPSAndCertificateImportLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	setupTestDB(t)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	router.SetApiRouter(engine)

	token := prepareRootToken(t)
	certPEM, keyPEM := generateCertificatePairForRouterTest(t, []string{"secure.example.com"})

	manualResp := performJSONRequest(t, engine, token, http.MethodPost, "/api/tls-certificates/", map[string]any{
		"name":     "secure-example",
		"cert_pem": certPEM,
		"key_pem":  keyPEM,
		"remark":   "manual import",
	})
	var manualCertificate model.TLSCertificate
	decodeResponseData(t, manualResp, &manualCertificate)
	if manualCertificate.ID == 0 {
		t.Fatal("expected manual certificate import to persist certificate")
	}

	fileCertPEM, fileKeyPEM := generateCertificatePairForRouterTest(t, []string{"upload.example.com"})
	multipartResp := performMultipartRequest(t, engine, token, "/api/tls-certificates/import-file", map[string]string{
		"name":   "upload-example",
		"remark": "upload import",
	}, map[string]string{
		"cert_file": fileCertPEM,
		"key_file":  fileKeyPEM,
	})
	var uploadedCertificate model.TLSCertificate
	decodeResponseData(t, multipartResp, &uploadedCertificate)
	if uploadedCertificate.ID == 0 {
		t.Fatal("expected file certificate import to persist certificate")
	}

	resp := performJSONRequest(t, engine, token, http.MethodPost, "/api/proxy-routes/", map[string]any{
		"domain":        "secure.example.com",
		"origin_url":    "https://origin-secure.internal",
		"enabled":       true,
		"enable_https":  true,
		"cert_id":       manualCertificate.ID,
		"redirect_http": true,
		"remark":        "https route",
	})
	var route model.ProxyRoute
	decodeResponseData(t, resp, &route)
	if !route.EnableHTTPS || route.CertID == nil || *route.CertID != manualCertificate.ID {
		t.Fatal("expected route to persist https certificate binding")
	}

	updateResp := performJSONRequest(t, engine, token, http.MethodPut, "/api/proxy-routes/"+toString(route.ID), map[string]any{
		"domain":        "secure.example.com",
		"origin_url":    "http://origin-secure.internal",
		"enabled":       true,
		"enable_https":  false,
		"cert_id":       nil,
		"redirect_http": false,
		"remark":        "downgraded route",
	})
	decodeResponseData(t, updateResp, &route)
	if route.EnableHTTPS || route.CertID != nil || route.RedirectHTTP {
		t.Fatalf("expected route to disable https flags, got %+v", route)
	}

	updateResp = performJSONRequest(t, engine, token, http.MethodPut, "/api/proxy-routes/"+toString(route.ID), map[string]any{
		"domain":        "secure.example.com",
		"origin_url":    "https://origin-secure.internal",
		"enabled":       true,
		"enable_https":  true,
		"cert_id":       manualCertificate.ID,
		"redirect_http": true,
		"remark":        "re-enabled https route",
	})
	decodeResponseData(t, updateResp, &route)
	if !route.EnableHTTPS || route.CertID == nil || *route.CertID != manualCertificate.ID || !route.RedirectHTTP {
		t.Fatalf("expected route update to persist https fields, got %+v", route)
	}

	listResp := performJSONRequest(t, engine, token, http.MethodGet, "/api/proxy-routes/", nil)
	var routes []model.ProxyRoute
	decodeResponseData(t, listResp, &routes)
	if len(routes) != 1 || !routes[0].EnableHTTPS || routes[0].CertID == nil || *routes[0].CertID != manualCertificate.ID || !routes[0].RedirectHTTP {
		t.Fatalf("expected route list to reflect https update, got %+v", routes)
	}

	certificateListResp := performJSONRequest(t, engine, token, http.MethodGet, "/api/tls-certificates/", nil)
	var certificateList []map[string]any
	decodeResponseData(t, certificateListResp, &certificateList)
	if len(certificateList) == 0 {
		t.Fatal("expected certificate list to return records")
	}
	if _, exists := certificateList[0]["cert_pem"]; exists {
		t.Fatal("expected certificate list to omit cert_pem")
	}
	if _, exists := certificateList[0]["key_pem"]; exists {
		t.Fatal("expected certificate list to omit key_pem")
	}

	resp = performJSONRequest(t, engine, token, http.MethodPost, "/api/config-versions/publish", nil)
	var version model.ConfigVersion
	decodeResponseData(t, resp, &version)
	if !strings.Contains(version.MainConfig, "include __ATSF_ROUTE_CONFIG__;") {
		t.Fatal("expected active config to render managed main config")
	}
	if !strings.Contains(version.RenderedConfig, "listen 443 ssl;") {
		t.Fatal("expected active config to render https listener")
	}
	if !strings.Contains(version.RenderedConfig, "return 301 https://$host$request_uri;") {
		t.Fatal("expected active config to render redirect server")
	}
	if !strings.Contains(version.SupportFilesJSON, ".crt") || !strings.Contains(version.SupportFilesJSON, ".key") {
		t.Fatal("expected support files json to contain certificate artifacts")
	}
	if err := (&model.Node{
		NodeID:       "phase1-node",
		Name:         "phase1-node",
		IP:           "10.0.0.8",
		AgentToken:   common.AgentToken,
		AgentVersion: "0.1.0",
		NginxVersion: "1.25.5",
		Status:       service.NodeStatusOnline,
		LastSeenAt:   time.Now(),
	}).Insert(); err != nil {
		t.Fatalf("failed to seed phase1 node: %v", err)
	}

	agentResp := performAgentJSONRequestWithToken(t, engine, common.AgentToken, http.MethodGet, "/api/agent/config-versions/active", nil)
	var activeConfig map[string]any
	decodeResponseData(t, agentResp, &activeConfig)
	mainConfig, ok := activeConfig["main_config"].(string)
	if !ok || !strings.Contains(mainConfig, "include __ATSF_ROUTE_CONFIG__;") {
		t.Fatalf("expected active config to expose main_config, got %#v", activeConfig["main_config"])
	}
	supportFiles, ok := activeConfig["support_files"].([]any)
	if !ok || len(supportFiles) != 2 {
		t.Fatalf("expected active config to expose 2 support files, got %#v", activeConfig["support_files"])
	}
}

func setupTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "phase1.db")
	common.SQLitePath = dbPath
	common.AgentToken = "phase1-agent-token"
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

func performMultipartRequest(t *testing.T, engine http.Handler, token string, path string, fields map[string]string, files map[string]string) apiResponse {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("failed to write multipart field: %v", err)
		}
	}
	for fieldName, content := range files {
		part, err := writer.CreateFormFile(fieldName, fieldName+".pem")
		if err != nil {
			t.Fatalf("failed to create multipart file: %v", err)
		}
		if _, err = part.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write multipart file content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d for multipart %s: %s", recorder.Code, path, recorder.Body.String())
	}
	var resp apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal multipart response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("multipart request %s failed: %s", path, resp.Message)
	}
	return resp
}

func generateCertificatePairForRouterTest(t *testing.T, dnsNames []string) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: dnsNames[0],
		},
		DNSNames:    dnsNames,
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate failed: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return string(certPEM), string(keyPEM)
}
