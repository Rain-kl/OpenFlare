// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/config_version"
	"github.com/Rain-kl/Wavelet/internal/automationcontract"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/Rain-kl/Wavelet/internal/testhelper/testdb"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

func TestOAuthAndReadOnlyTransports(t *testing.T) {
	server, database := automationTestServer(t, false)
	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	client.Jar, _ = cookiejar.New(nil)

	mcpURL := server.URL + "/mcp"
	reply, err := client.Post(mcpURL, "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, reply.StatusCode)
	require.Contains(t, reply.Header.Get("WWW-Authenticate"), server.URL+metadataPath+"/mcp")
	require.NoError(t, reply.Body.Close())

	for _, forwarded := range []struct {
		host   string
		status int
	}{
		{strings.TrimPrefix(server.URL, "http://"), http.StatusUnauthorized},
		{"wrong.example", http.StatusForbidden},
	} {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, mcpURL, strings.NewReader("{}"))
		require.NoError(t, err)
		request.Host = "localhost:3000"
		request.Header.Set("X-Forwarded-Host", forwarded.host)
		reply, err := client.Do(request)
		require.NoError(t, err)
		require.Equal(t, forwarded.status, reply.StatusCode)
		require.NoError(t, reply.Body.Close())
	}

	ctx := t.Context()
	const redirect = "http://127.0.0.1:4321/callback"
	clientID := registerTestClient(t, client, server.URL, redirect)
	mcpTokens := authorizeTestClient(t, client, server.URL, clientID, redirect, mcpURL, "nodes:read", "read")
	apiURL := server.URL + "/api/v1/automation"
	apiTokens := authorizeTestClient(t, client, server.URL, cliClientID, redirect, apiURL, "", "read")

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "automation-test", Version: "1"}, nil)
	authClient := &http.Client{Transport: authorizationTransport{base: client.Transport, token: mcpTokens.AccessToken}}
	session, err := mcpClient.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: mcpURL, HTTPClient: authClient, DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, session.Close()) })
	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 2)
	for _, tool := range tools.Tools {
		require.True(t, tool.Annotations.ReadOnlyHint)
		require.NotEqual(t, connectionOperation, tool.Name)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: nodeListOperation, Arguments: map[string]any{"search": "demo-edge"}})
	require.NoError(t, err)
	require.False(t, result.IsError, "%v", result.Content)
	results, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	require.Contains(t, string(results), "demo-edge")
	require.NotContains(t, string(results), "must-not-appear")
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: nodeListOperation, Arguments: map[string]any{"page_size": 101}})
	require.NoError(t, err)
	require.True(t, result.IsError)

	checkRead := func(token, path string, status int) string {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+path, nil)
		require.NoError(t, reqErr)
		req.Header.Set("Authorization", "Bearer "+token)
		reply, callErr := client.Do(req)
		require.NoError(t, callErr)
		defer reply.Body.Close()
		body, readErr := io.ReadAll(reply.Body)
		require.NoError(t, readErr)
		require.Equal(t, status, reply.StatusCode, "%s", body)
		return string(body)
	}
	checkRead(mcpTokens.AccessToken, "/nodes", http.StatusUnauthorized)
	checkRead("at_legacy_admin", "/nodes", http.StatusUnauthorized)
	require.Contains(t, checkRead(apiTokens.AccessToken, "/connection", http.StatusOK), "instance_id")
	require.Contains(t, checkRead(apiTokens.AccessToken, "/nodes/42", http.StatusOK), `"node_record_id":"42"`)
	require.NotContains(t, checkRead(apiTokens.AccessToken, "/nodes", http.StatusOK), "must-not-appear")
	siteTokens := authorizeTestClient(t, client, server.URL, cliClientID, redirect, apiURL, "sites:read", "read")
	checkRead(siteTokens.AccessToken, "/sites", http.StatusOK)
	require.NotContains(t, checkRead(siteTokens.AccessToken, "/operations", http.StatusOK), "openflare_site_overview")
	checkRead(siteTokens.AccessToken, "/sites/overview", http.StatusForbidden)
	checkRead(apiTokens.AccessToken, "/nodes?page=1&page=2", http.StatusBadRequest)
	checkRead(apiTokens.AccessToken, "/nodes/42?node_record_id=1", http.StatusBadRequest)

	values := url.Values{"grant_type": {grantRefresh}, "client_id": {cliClientID}, "resource": {apiURL}, "refresh_token": {apiTokens.RefreshToken}}
	var refreshed testTokenResponse
	postOAuthForm(t, client, server.URL+tokenPath, values, "", http.StatusOK, &refreshed)
	require.NotEqual(t, apiTokens.RefreshToken, refreshed.RefreshToken)
	checkRead(apiTokens.AccessToken, "/nodes", http.StatusUnauthorized)
	checkRead(refreshed.AccessToken, "/nodes", http.StatusOK)
	postOAuthForm(t, client, server.URL+tokenPath, values, "", http.StatusBadRequest, nil)
	postOAuthForm(t, client, server.URL+revokePath, url.Values{"client_id": {cliClientID}, "token": {refreshed.RefreshToken}}, "", http.StatusOK, nil)
	checkRead(refreshed.AccessToken, "/nodes", http.StatusUnauthorized)

	personalToken := "at_existing_personal_access_token"
	personalTokenRecord := model.AccessToken{
		UserID: 991, Name: "MCP integration", TokenHash: model.HashToken(personalToken),
		MaskedToken: model.MaskTokenString(personalToken), IsAdmin: true,
	}
	require.NoError(t, database.Create(&personalTokenRecord).Error)
	personalClient := &http.Client{Transport: authorizationTransport{base: client.Transport, token: personalToken}}
	personalSession, err := mcpClient.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: mcpURL, HTTPClient: personalClient, DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, personalSession.Close()) })
	personalTools, err := personalSession.ListTools(ctx, nil)
	require.NoError(t, err)
	require.Len(t, personalTools.Tools, len(bindings()))
	checkRead(personalToken, "/nodes", http.StatusUnauthorized)
	toolNames := make([]string, 0, len(personalTools.Tools))
	for _, tool := range personalTools.Tools {
		toolNames = append(toolNames, tool.Name)
	}
	require.Contains(t, toolNames, "openflare_site_create")
	require.Contains(t, toolNames, "openflare_site_overview")

	readOnlyToken := "at_existing_read_only_token"
	require.NoError(t, database.Create(&model.AccessToken{
		UserID: 991, Name: "MCP read only", TokenHash: model.HashToken(readOnlyToken),
		MaskedToken: model.MaskTokenString(readOnlyToken), IsAdmin: false,
	}).Error)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, strings.NewReader("{}"))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+readOnlyToken)
	denied, err := client.Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, denied.StatusCode)
	require.NoError(t, denied.Body.Close())

	require.NoError(t, database.Model(&model.User{}).Where("id = ?", 991).Update("is_admin", false).Error)
	_, err = session.ListTools(ctx, nil)
	require.Error(t, err)
	require.NoError(t, database.Model(&model.User{}).Where("id = ?", 991).Update("is_admin", true).Error)
	revoke, err := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/v1/admin/automation/grants", nil)
	require.NoError(t, err)
	revoked, err := client.Do(revoke)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, revoked.StatusCode)
	require.NoError(t, revoked.Body.Close())
	_, err = personalSession.ListTools(ctx, nil)
	require.NoError(t, err)
	_, err = session.ListTools(ctx, nil)
	require.Error(t, err)
	config.Config.App.SessionSecret = "short"
	settings, err := client.Get(server.URL + "/api/v1/admin/automation/mcp")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, settings.StatusCode)
	var disabled struct {
		Data mcpSettingsResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(settings.Body).Decode(&disabled))
	require.NoError(t, settings.Body.Close())
	require.False(t, disabled.Data.Enabled)
	require.Equal(t, "session_secret_too_short", disabled.Data.ConfigurationError)

}

func TestOperationsWorkflow(t *testing.T) {
	server, _ := automationTestServer(t, true)
	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	client.Jar, _ = cookiejar.New(nil)
	const redirect = "http://127.0.0.1:4321/callback"
	apiURL, mcpURL := server.URL+"/api/v1/automation", server.URL+"/mcp"
	readTokens := authorizeTestClient(t, client, server.URL, cliClientID, redirect, apiURL, "", "read")
	writeTokens := authorizeTestClient(t, client, server.URL, cliClientID, redirect, apiURL, "", "operate")
	const hostedRedirect = "https://client.example/oauth/callback"
	clientID := registerTestClient(t, client, server.URL, hostedRedirect)
	mcpTokens := authorizeTestClient(t, client, server.URL, clientID, hostedRedirect, mcpURL, "", "operate")
	instanceID := testOperation[ConnectionInfo](t, client, apiURL, writeTokens.AccessToken, "", http.MethodGet, "/connection", "", "", http.StatusOK).InstanceID
	setup, err := client.Get(server.URL + "/api/v1/admin/automation/mcp")
	require.NoError(t, err)
	var settings struct {
		Data mcpSettingsResponse `json:"data"`
	}
	require.Equal(t, http.StatusOK, setup.StatusCode)
	require.NoError(t, json.NewDecoder(setup.Body).Decode(&settings))
	require.NoError(t, setup.Body.Close())
	require.Equal(t, mcpURL, settings.Data.MCPURL)
	require.Equal(t, instanceID, settings.Data.InstanceID)
	operations := make(map[string]automationcontract.Operation)
	for _, operation := range automationcontract.Operations() {
		operations[operation.ID] = operation
	}
	zoneContract := operations["openflare_zone_create"].Revision()
	zoneBody := `{"root_domain":"example.com"}`
	testOperation[ZoneSummary](t, client, apiURL, readTokens.AccessToken, instanceID, http.MethodPost, "/zones", zoneContract, zoneBody, http.StatusForbidden)
	testOperation[ZoneSummary](t, client, apiURL, writeTokens.AccessToken, "wrong-instance", http.MethodPost, "/zones", zoneContract, zoneBody, http.StatusConflict)
	testOperation[ZoneSummary](t, client, apiURL, writeTokens.AccessToken, instanceID, http.MethodPost, "/zones", "old-contract", zoneBody, http.StatusConflict)
	testOperation[ZoneSummary](t, client, apiURL, writeTokens.AccessToken, instanceID, http.MethodPost, "/zones", zoneContract, `{"root_domain":"example.com","root_domain":"example.net"}`, http.StatusBadRequest)
	zone := testOperation[ZoneSummary](t, client, apiURL, writeTokens.AccessToken, instanceID, http.MethodPost, "/zones", zoneContract, zoneBody, http.StatusOK)
	require.Equal(t, "example.com", zone.RootDomain)

	ctx := t.Context()
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "operations-test", Version: "1"}, nil)
	session, err := mcpClient.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: mcpURL, HTTPClient: &http.Client{Transport: authorizationTransport{client.Transport, mcpTokens.AccessToken}}, DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, session.Close()) })
	invoke := func(name string, arguments map[string]any, success bool) []byte {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
		require.NoError(t, err)
		content, err := json.Marshal(result.Content)
		require.NoError(t, err)
		require.Equal(t, !success, result.IsError, "%s: %s", name, content)
		encoded, err := json.Marshal(result.StructuredContent)
		require.NoError(t, err)
		return encoded
	}
	var domain DomainSummary
	require.NoError(t, json.Unmarshal(invoke("openflare_domain_create", map[string]any{"zone_id": zone.ZoneID, "hostname": "www.example.com"}, true), &domain))
	args := map[string]any{"site_name": "Example", "domain_record_ids": []string{domain.DomainRecordID}, "upstreams": []string{"http://192.0.2.10:8080"}}
	invoke("openflare_site_create", args, false)
	args["enable_https"] = "false"
	invoke("openflare_site_create", args, false)
	args["enable_https"] = true
	invoke("openflare_site_create", args, false)
	key, err := x509.MarshalPKCS8PrivateKey(server.TLS.Certificates[0].PrivateKey)
	require.NoError(t, err)
	certificate := &model.TLSCertificate{
		Name: "Test certificate", PrimaryDomain: "*.example.com",
		CertPEM:   string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})),
		KeyPEM:    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})),
		NotBefore: server.Certificate().NotBefore, NotAfter: server.Certificate().NotAfter,
	}
	require.NoError(t, repository.CreateTLSCertificateRecord(ctx, certificate))
	certificateID := decimalID(certificate.ID)
	require.NotContains(t, string(invoke("openflare_certificate_list", nil, true)), "PRIVATE KEY")
	require.NoError(t, json.Unmarshal(invoke("openflare_domain_create", map[string]any{"zone_id": zone.ZoneID, "hostname": "app.example.com", "certificate_id": certificateID}, true), &domain))
	args["domain_record_ids"] = []string{domain.DomainRecordID}
	var created SiteWriteResult
	require.NoError(t, json.Unmarshal(invoke("openflare_site_create", args, true), &created))
	require.Equal(t, savedState, created.State)
	require.True(t, created.Site.HTTPSEnabled)
	var alias DomainSummary
	require.NoError(t, json.Unmarshal(invoke("openflare_domain_create", map[string]any{"zone_id": zone.ZoneID, "hostname": "alias.example.com", "certificate_id": certificateID}, true), &alias))
	invoke("openflare_site_bind_domain", map[string]any{"site_id": created.Site.SiteID, "domain_record_id": alias.DomainRecordID}, true)
	contract := operations["openflare_site_set_upstreams"].Revision()
	path := "/sites/" + created.Site.SiteID + "/upstreams"
	testOperation[SiteWriteResult](t, client, apiURL, writeTokens.AccessToken, instanceID, http.MethodPatch, path, contract, `{"upstreams":["http://192.0.2.20"],"enabled":false}`, http.StatusBadRequest)
	testOperation[SiteWriteResult](t, client, apiURL, writeTokens.AccessToken, instanceID, http.MethodPatch, path, contract, `{"upstreams":["http://192.0.2.20:65536"]}`, http.StatusBadRequest)
	changed := testOperation[SiteWriteResult](t, client, apiURL, writeTokens.AccessToken, instanceID, http.MethodPatch, path, contract, `{"upstreams":["https://origin.example.com:8443"]}`, http.StatusOK)
	require.True(t, changed.Site.Enabled)
	require.True(t, changed.Site.HTTPSEnabled)
	require.Len(t, changed.Site.Domains, 2)
	require.Equal(t, []string{"https://origin.example.com:8443"}, changed.Site.Upstreams)
	require.NoError(t, config_version.RefreshSiteSnapshot(ctx))
	snapshot, err := repository.GetActivePingoraConfigVersion(ctx)
	require.NoError(t, err)
	require.Contains(t, snapshot.SnapshotJSON, "alias.example.com")
	require.Contains(t, snapshot.SnapshotJSON, "origin.example.com:8443")
	status := testOperation[NodeSummary](t, client, apiURL, writeTokens.AccessToken, "", http.MethodGet, "/nodes/42", "", "", http.StatusOK)
	require.Equal(t, snapshot.Version, status.DesiredVersion)
	require.NotEqual(t, status.DesiredVersion, status.CurrentVersion)

	policies := testOperation[PerformanceList](t, client, apiURL, writeTokens.AccessToken, "", http.MethodGet, "/performance", "", "", http.StatusOK)
	require.NotEmpty(t, policies.Policies)
	policyID := policies.Policies[0].PolicyID
	invoke("openflare_performance_set", map[string]any{"policy_id": policyID, "key": "threads", "value": "0"}, false)
	invoke("openflare_performance_set", map[string]any{"policy_id": policyID, "key": "threads", "value": "4"}, true)
	updated := testOperation[PerformanceList](t, client, apiURL, writeTokens.AccessToken, "", http.MethodGet, "/performance", "", "", http.StatusOK)
	policies.Policies[0].Settings["threads"] = float64(4)
	require.Equal(t, policies.Policies, updated.Policies)

	var wildcard DomainSummary
	require.NoError(t, json.Unmarshal(invoke("openflare_domain_create", map[string]any{
		"zone_id": zone.ZoneID, "hostname": "*.example.com",
	}, true), &wildcard))
	var wildcardSite SiteWriteResult
	require.NoError(t, json.Unmarshal(invoke("openflare_site_create", map[string]any{
		"site_name": "Wildcard", "domain_record_ids": []string{wildcard.DomainRecordID},
		"upstreams": []string{"http://192.0.2.20"}, "enable_https": false,
	}, true), &wildcardSite))

	now := time.Now().UTC().Add(-time.Minute)
	store, err := logstore.Active(ctx)
	require.NoError(t, err)
	require.NoError(t, store.AccessLogs.BatchInsertNodeAccessLogs(ctx, []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "demo-edge", LoggedAt: now, Host: "www.example.com", Path: "/health?secret=hidden", UserAgent: "untrusted-log-text", StatusCode: 503},
		{ID: 2, NodeID: "demo-edge", LoggedAt: now, Host: "www.example.com.example.net", Path: "/other-site", StatusCode: 503},
		{ID: 3, NodeID: "demo-edge", LoggedAt: now, Host: "APP.EXAMPLE.COM.", RemoteAddr: "192.0.2.5", StatusCode: 200, BytesSent: 16},
		{ID: 4, NodeID: "demo-edge", LoggedAt: now, Host: "alias.example.com", RemoteAddr: "192.0.2.5", StatusCode: 502, BytesSent: 8},
		{ID: 5, NodeID: "other-edge", LoggedAt: now, Host: "app.example.com", RemoteAddr: "192.0.2.6", StatusCode: 404},
	}))
	logs := invoke("openflare_logs_list", map[string]any{"host": "www.example.com", "status_code": 503}, true)
	require.Contains(t, string(logs), "/health")
	require.NotContains(t, string(logs), "secret=hidden")
	require.NotContains(t, string(logs), "untrusted-log-text")
	require.NotContains(t, string(logs), "/other-site")
	traffic := testOperation[TrafficMetrics](t, client, apiURL, writeTokens.AccessToken, "", http.MethodGet, "/metrics/traffic?host=www.example.com", "", "", http.StatusOK)
	require.EqualValues(t, 1, traffic.Summary.TotalRequests)
	require.NotNil(t, traffic.Trends)
	compact := invoke("openflare_metrics_traffic", map[string]any{"host": "www.example.com", "with_trends": false}, true)
	require.NotContains(t, string(compact), `"trends"`)
	require.Contains(t, string(compact), `"total_requests":1`)
	overviewArgs := map[string]any{"node_id": "demo-edge", "limit": 1, "hours": 1}
	var overview SiteOverview
	require.NoError(t, json.Unmarshal(invoke("openflare_site_overview", overviewArgs, true), &overview))
	require.Equal(t, time.Hour, overview.Until.Sub(overview.Since))
	require.Equal(t, SiteTrafficTotals{Requests: 4, UniqueIPs: 1, BytesSent: 24, ServerErrors: 3}, overview.TotalTraffic)
	require.EqualValues(t, 1, overview.UnmatchedRequests)
	require.Equal(t, 2, overview.SiteCount)
	require.Equal(t, 2, overview.EnabledSiteCount)
	require.Equal(t, 2, overview.SitesWithRequests)
	require.Len(t, overview.Sites, 1)
	require.Equal(t, created.Site.SiteID, overview.Sites[0].SiteID)
	require.Equal(t, []string{"app.example.com", "alias.example.com"}, overview.Sites[0].Hostnames)
	require.Equal(t, SiteTrafficTotals{Requests: 2, UniqueIPs: 1, BytesSent: 24, ServerErrors: 1}, overview.Sites[0].Traffic)
	require.NotNil(t, overview.NextOffset)
	page := testOperation[SiteOverview](t, client, apiURL, readTokens.AccessToken, "", http.MethodGet,
		"/sites/overview?node_id=demo-edge&hours=1&limit=1&offset=1&until="+url.QueryEscape(overview.Until.Format(time.RFC3339Nano)), "", "", http.StatusOK)
	require.True(t, overview.Until.Equal(page.Until))
	require.Equal(t, overview.TotalTraffic, page.TotalTraffic)
	require.Nil(t, page.NextOffset)
	require.Len(t, page.Sites, 1)
	require.Equal(t, wildcardSite.Site.SiteID, page.Sites[0].SiteID)
	require.Equal(t, SiteTrafficTotals{Requests: 1, ServerErrors: 1}, page.Sites[0].Traffic)
	require.NoError(t, store.Observability.BatchInsertNodeMetricSnapshots(ctx, []analyticsmodel.NodeMetricSnapshot{{NodeID: "demo-edge", CapturedAt: now, CPUUsagePercent: 42}}))
	metrics := invoke("openflare_node_metrics", map[string]any{"node_record_id": "42", "limit": 1}, true)
	require.Contains(t, string(metrics), `"cpu_usage_percent":42`)
	require.NotContains(t, string(metrics), "must-not-appear")
}

func testOperation[Result any](t *testing.T, client *http.Client, base, token, instanceID, method, path, revision, body string, status int) Result {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, base+path, strings.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set(automationcontract.InstanceHeader, instanceID)
	request.Header.Set(automationcontract.ContractHeader, revision)
	request.Header.Set("Content-Type", "application/json")
	reply, err := client.Do(request)
	require.NoError(t, err)
	defer reply.Body.Close()
	encoded, err := io.ReadAll(reply.Body)
	require.NoError(t, err)
	require.Equal(t, status, reply.StatusCode, "%s %s: %s", method, path, encoded)
	var envelope struct {
		Data Result `json:"data"`
	}
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	return envelope.Data
}

func automationTestServer(t *testing.T, useTLS bool) (*httptest.Server, *gorm.DB) {
	t.Helper()
	previous := config.Config
	t.Cleanup(func() { config.Config = previous })
	config.Config.App.APIPrefix = "/api"
	config.Config.App.SessionSecret = "test fixture secret with at least 32 bytes"
	config.Config.App.SessionCookieName = "automation_test"
	database := testdb.OpenMigratedSQLite(t)
	user := model.User{ID: 991, Username: "automation-admin", Email: "automation@openflare.test", IsAdmin: true, IsActive: true}
	require.NoError(t, database.Create(&user).Error)
	require.NoError(t, database.Create(&model.OpenFlareNode{
		ID: 42, NodeID: "demo-edge", Name: "Demo Edge", NodeType: "edge_node",
		AccessToken: "must-not-appear",
	}).Error)
	store := cookie.NewStore([]byte(config.Config.App.SessionSecret))
	router := testhelper.NewTestGinEngine(sessions.Sessions(config.Config.App.SessionCookieName, store))
	router.GET("/test-session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set(oauth.UserIDKey, user.ID)
		if err := session.Save(); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET(issuerPath, AuthorizationMetadata())
	router.GET(metadataPath+"/mcp", ProtectedResourceMetadata(true))
	router.POST("/mcp", MCPHandler())
	RegisterOAuth(router.Group(automationcontract.OAuthPath))
	RegisterBrowser(router.Group("/automation"))
	RegisterSettings(router.Group("/api/v1/admin/automation"))
	RegisterAPI(router.Group("/api/v1/automation"))
	server := httptest.NewUnstartedServer(router)
	if useTLS {
		server.StartTLS()
	} else {
		server.Start()
	}
	t.Cleanup(server.Close)
	require.NoError(t, database.Model(&model.SystemConfig{}).Where("key = ?", model.ConfigKeyAutomationPublicURL).Update("value", server.URL).Error)
	return server, database
}

func registerTestClient(t *testing.T, client *http.Client, origin, redirect string) string {
	t.Helper()
	request := registrationRequest{ClientName: "Test MCP", RedirectURIs: []string{redirect}, TokenEndpointAuth: "none"}
	payload, err := json.Marshal(request)
	require.NoError(t, err)
	reply, err := client.Post(origin+registerPath, "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer reply.Body.Close()
	require.Equal(t, http.StatusCreated, reply.StatusCode)
	var result struct {
		ClientID string `json:"client_id"`
	}
	require.NoError(t, json.NewDecoder(reply.Body).Decode(&result))
	require.NotEmpty(t, result.ClientID)
	return result.ClientID
}

type testTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func authorizeTestClient(t *testing.T, client *http.Client, origin, clientID, redirect, resource, scope, decision string) testTokenResponse {
	t.Helper()
	verifier := oauth2.GenerateVerifier()
	authorization := origin + authorizationPath + "?" + url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect}, "resource": {resource},
		"state": {"browser-state"}, "code_challenge": {oauth2.S256ChallengeFromVerifier(verifier)},
		"code_challenge_method": {"S256"}, "scope": {scope},
	}.Encode()
	reply, err := client.Get(origin + "/test-session")
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, reply.StatusCode)
	require.NoError(t, reply.Body.Close())
	reply, err = client.Get(authorization)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, reply.StatusCode, reply.Header.Get("Location"))
	page, err := reply.Location()
	require.NoError(t, err)
	require.NoError(t, reply.Body.Close())
	require.Equal(t, consentPagePath, page.Path)
	fragment, err := url.ParseQuery(page.Fragment)
	require.NoError(t, err)
	intent := fragment.Get("intent")
	require.NotEmpty(t, intent)
	var consent consentInfoResponse
	postOAuthForm(t, client, origin+automationcontract.OAuthPath+"/consent", url.Values{"intent": {intent}}, origin, http.StatusOK, &consent)
	require.Equal(t, redirect, consent.RedirectURI)
	scopes := consent.Scopes
	if decision == "read" {
		scopes = slices.DeleteFunc(scopes, func(scope string) bool { return !strings.HasSuffix(scope, ":read") })
	}
	values := url.Values{"intent": {intent}, "decision": {"authorize"}, "scope": {strings.Join(scopes, " ")}}
	postOAuthForm(t, client, origin+authorizationPath, values, "https://wrong.example", http.StatusForbidden, nil)
	location := postOAuthForm(t, client, origin+authorizationPath, values, origin, http.StatusSeeOther, nil)
	callback, err := url.Parse(location)
	require.NoError(t, err)
	require.Equal(t, "browser-state", callback.Query().Get("state"))
	code := callback.Query().Get("code")
	require.NotEmpty(t, code)
	form := url.Values{
		"grant_type": {grantCode}, "code": {code}, "redirect_uri": {redirect}, "client_id": {clientID},
		"resource": {resource}, "code_verifier": {oauth2.GenerateVerifier()},
	}
	postOAuthForm(t, client, origin+tokenPath, form, "", http.StatusBadRequest, nil)
	form.Set("code_verifier", verifier)
	var result testTokenResponse
	postOAuthForm(t, client, origin+tokenPath, form, "", http.StatusOK, &result)
	require.NotEmpty(t, result.AccessToken)
	return result
}

func postOAuthForm(t *testing.T, client *http.Client, endpoint string, form url.Values, origin string, status int, result any) string {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	reply, err := client.Do(request)
	require.NoError(t, err)
	defer reply.Body.Close()
	body, err := io.ReadAll(reply.Body)
	require.NoError(t, err)
	require.Equal(t, status, reply.StatusCode, "%s", body)
	if result != nil {
		require.NoError(t, json.Unmarshal(body, result))
	}
	return reply.Header.Get("Location")
}

type authorizationTransport struct {
	base  http.RoundTripper
	token string
}

func (transport authorizationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	cloned.Header.Set("Authorization", "Bearer "+transport.token)
	return transport.base.RoundTrip(cloned)
}
