package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"openflare/common"
	"openflare/model"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateTLSCertificateAndRenderHTTPSConfig(t *testing.T) {
	setupServiceTestDB(t)

	certPEM, keyPEM := generateCertificatePair(t, []string{"app.example.com"})
	certificate, err := CreateTLSCertificate(TLSCertificateInput{
		Name:    "app-example",
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		Remark:  "test cert",
	})
	if err != nil {
		t.Fatalf("CreateTLSCertificate failed: %v", err)
	}
	if certificate.NotAfter.Before(certificate.NotBefore) {
		t.Fatal("expected certificate validity period to be parsed")
	}

	route, err := CreateProxyRoute(ProxyRouteInput{
		Domain:       "app.example.com",
		OriginURL:    "https://origin.internal",
		Enabled:      true,
		EnableHTTPS:  true,
		CertID:       &certificate.ID,
		RedirectHTTP: true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	if !route.EnableHTTPS || route.CertID == nil {
		t.Fatal("expected https fields to be persisted")
	}

	result, err := PublishConfigVersion("root")
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.MainConfig, "include __OPENFLARE_ROUTE_CONFIG__;") {
		t.Fatal("expected main config to include managed route config placeholder")
	}
	if !strings.Contains(result.Version.MainConfig, "access_log __OPENFLARE_ACCESS_LOG__ openflare_json;") {
		t.Fatal("expected main config to include managed access log placeholder")
	}
	if !strings.Contains(result.Version.MainConfig, "log_by_lua_file __OPENFLARE_LUA_DIR__/log.lua;") {
		t.Fatal("expected main config to include managed openresty lua log hook")
	}
	if !strings.Contains(result.Version.MainConfig, "listen __OPENFLARE_OBSERVABILITY_LISTEN__;") {
		t.Fatal("expected main config to include managed openresty observability listen placeholder")
	}
	if !strings.Contains(result.Version.MainConfig, "__OPENFLARE_RESOLVER_DIRECTIVE__") {
		t.Fatal("expected main config to include managed resolver directive placeholder")
	}
	if strings.Contains(result.Version.MainConfig, "allow 127.0.0.1;") {
		t.Fatal("expected main config to avoid hard-coded allow rules on observability server")
	}
	if !strings.Contains(result.Version.RenderedConfig, "listen 443 ssl;") {
		t.Fatal("expected rendered config to include https server block")
	}
	if !strings.Contains(result.Version.RenderedConfig, `if ($host != "app.example.com") {`) {
		t.Fatal("expected rendered config to reject unmatched host headers with 404")
	}
	if !strings.Contains(result.Version.RenderedConfig, "return 301 https://$host$request_uri;") {
		t.Fatal("expected rendered config to include http redirect")
	}
	if !strings.Contains(result.Version.RenderedConfig, "__OPENFLARE_CERT_DIR__/") {
		t.Fatal("expected rendered config to keep cert dir placeholder for certificates")
	}
	if !strings.Contains(result.Version.SupportFilesJSON, ".crt") || !strings.Contains(result.Version.SupportFilesJSON, ".key") {
		t.Fatal("expected support files to contain certificate and key")
	}
}

func TestCreateProxyRouteRejectsHTTPSWithoutCertificate(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:      "secure.example.com",
		OriginURL:   "https://origin.internal",
		Enabled:     true,
		EnableHTTPS: true,
	})
	if err == nil || !strings.Contains(err.Error(), "必须选择证书") {
		t.Fatalf("expected certificate validation error, got %v", err)
	}
}

func TestPublishConfigVersionRendersCustomHeaders(t *testing.T) {
	setupServiceTestDB(t)
	if err := model.UpdateOption("OpenRestyWebsocketEnabled", "true"); err != nil {
		t.Fatalf("UpdateOption OpenRestyWebsocketEnabled failed: %v", err)
	}

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "custom.example.com",
		OriginURL: "https://origin.internal",
		Enabled:   true,
		CustomHeaders: []ProxyRouteCustomHeaderInput{
			{Key: "X-Trace-Id", Value: "$request_id"},
			{Key: "X-Env", Value: "staging edge"},
		},
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	result, err := PublishConfigVersion("root")
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.RenderedConfig, `proxy_set_header X-Trace-Id "$request_id";`) {
		t.Fatal("expected rendered config to include custom header")
	}
	if !strings.Contains(result.Version.RenderedConfig, `proxy_set_header X-Env "staging edge";`) {
		t.Fatal("expected rendered config to include quoted custom header value")
	}
	if !strings.Contains(result.Version.SnapshotJSON, "custom_headers") {
		t.Fatal("expected snapshot to include custom headers")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_http_version 1.1;") {
		t.Fatal("expected rendered config to enable HTTP/1.1 proxying for websocket upgrades")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_set_header Upgrade $http_upgrade;") {
		t.Fatal("expected rendered config to forward websocket upgrade header")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_set_header Connection $http_connection;") {
		t.Fatal("expected rendered config to forward websocket connection header")
	}
	if !strings.Contains(result.Version.RenderedConfig, `set $openflare_upstream "https://origin.internal";`) {
		t.Fatal("expected rendered config to defer upstream resolution via variable proxy_pass")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_pass $openflare_upstream$request_uri;") {
		t.Fatal("expected rendered config to proxy via runtime-resolved upstream variable")
	}
}

func TestPublishConfigVersionOverridesOriginHostHeader(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:     "git.arctel.de",
		OriginURL:  "https://git.arctel.net",
		OriginHost: "git.arctel.net",
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	result, err := PublishConfigVersion("root")
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.RenderedConfig, `proxy_set_header Host "git.arctel.net";`) {
		t.Fatal("expected rendered config to override host header for origin routing")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_ssl_server_name on;") {
		t.Fatal("expected rendered config to enable proxy ssl server name for https origin")
	}
	if !strings.Contains(result.Version.RenderedConfig, `proxy_ssl_name "git.arctel.net";`) {
		t.Fatal("expected rendered config to set proxy ssl name from origin host override")
	}
	if !strings.Contains(result.Version.RenderedConfig, `set $openflare_upstream "https://git.arctel.net";`) {
		t.Fatal("expected rendered config to avoid resolving https upstream during config load")
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"origin_host":"git.arctel.net"`) {
		t.Fatal("expected snapshot to include origin_host override")
	}
}

func TestPreviewConfigVersionCanDisableWebsocketHeaders(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "ws-off.example.com",
		OriginURL: "https://origin.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	if err := model.UpdateOption("OpenRestyWebsocketEnabled", "false"); err != nil {
		t.Fatalf("UpdateOption OpenRestyWebsocketEnabled failed: %v", err)
	}

	preview, err := PreviewConfigVersion()
	if err != nil {
		t.Fatalf("PreviewConfigVersion failed: %v", err)
	}
	if strings.Contains(preview.RenderedConfig, "proxy_http_version 1.1;") {
		t.Fatal("expected preview config to omit websocket proxy_http_version when disabled")
	}
	if strings.Contains(preview.RenderedConfig, "proxy_set_header Upgrade $http_upgrade;") {
		t.Fatal("expected preview config to omit websocket upgrade header when disabled")
	}
	if strings.Contains(preview.RenderedConfig, "proxy_set_header Connection $http_connection;") {
		t.Fatal("expected preview config to omit websocket connection header when disabled")
	}
}

func TestPreviewAndDiffConfigVersion(t *testing.T) {
	setupServiceTestDB(t)
	if err := model.UpdateOption("OpenRestyWebsocketEnabled", "true"); err != nil {
		t.Fatalf("UpdateOption OpenRestyWebsocketEnabled failed: %v", err)
	}

	stableRoute, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "stable.example.com",
		OriginURL: "https://origin-a.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute stable failed: %v", err)
	}
	modifiedRoute, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "api.example.com",
		OriginURL: "https://origin-api-a.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute modified failed: %v", err)
	}
	removedRoute, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "old.example.com",
		OriginURL: "https://origin-old.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute removed failed: %v", err)
	}
	if _, err = PublishConfigVersion("root"); err != nil {
		t.Fatalf("initial PublishConfigVersion failed: %v", err)
	}

	if _, err = UpdateProxyRoute(modifiedRoute.ID, ProxyRouteInput{
		Domain:    "api.example.com",
		OriginURL: "https://origin-api-b.internal",
		Enabled:   true,
		CustomHeaders: []ProxyRouteCustomHeaderInput{
			{Key: "X-Release", Value: "candidate"},
		},
	}); err != nil {
		t.Fatalf("UpdateProxyRoute failed: %v", err)
	}
	if _, err = UpdateProxyRoute(removedRoute.ID, ProxyRouteInput{
		Domain:    "old.example.com",
		OriginURL: "https://origin-old.internal",
		Enabled:   false,
	}); err != nil {
		t.Fatalf("disable removed route failed: %v", err)
	}
	if _, err = CreateProxyRoute(ProxyRouteInput{
		Domain:    "new.example.com",
		OriginURL: "https://origin-new.internal",
		Enabled:   true,
	}); err != nil {
		t.Fatalf("CreateProxyRoute new failed: %v", err)
	}
	if _, err = UpdateProxyRoute(stableRoute.ID, ProxyRouteInput{
		Domain:    stableRoute.Domain,
		OriginURL: stableRoute.OriginURL,
		Enabled:   true,
		Remark:    "remark only change",
	}); err != nil {
		t.Fatalf("UpdateProxyRoute stable failed: %v", err)
	}

	preview, err := PreviewConfigVersion()
	if err != nil {
		t.Fatalf("PreviewConfigVersion failed: %v", err)
	}
	if !strings.Contains(preview.MainConfig, "include __OPENFLARE_ROUTE_CONFIG__;") {
		t.Fatal("expected preview main config to include managed route config placeholder")
	}
	if !strings.Contains(preview.MainConfig, "log_by_lua_file __OPENFLARE_LUA_DIR__/log.lua;") {
		t.Fatal("expected preview main config to include managed openresty lua log hook")
	}
	if !strings.Contains(preview.RenderedConfig, `proxy_set_header X-Release "candidate";`) {
		t.Fatal("expected preview config to include modified custom header")
	}
	if preview.RouteCount != 3 {
		t.Fatalf("expected 3 enabled routes in preview, got %d", preview.RouteCount)
	}

	diff, err := DiffConfigVersion()
	if err != nil {
		t.Fatalf("DiffConfigVersion failed: %v", err)
	}
	if len(diff.AddedDomains) != 1 || diff.AddedDomains[0] != "new.example.com" {
		t.Fatalf("unexpected added domains: %#v", diff.AddedDomains)
	}
	if len(diff.RemovedDomains) != 1 || diff.RemovedDomains[0] != "old.example.com" {
		t.Fatalf("unexpected removed domains: %#v", diff.RemovedDomains)
	}
	if len(diff.ModifiedDomains) != 1 || diff.ModifiedDomains[0] != "api.example.com" {
		t.Fatalf("unexpected modified domains: %#v", diff.ModifiedDomains)
	}
	if diff.MainConfigChanged {
		t.Fatal("expected main config to remain unchanged when only routes change")
	}

	if err = model.UpdateOption("OpenRestyProxyReadTimeout", "120"); err != nil {
		t.Fatalf("UpdateOption failed: %v", err)
	}
	if err = model.UpdateOption("OpenRestyWebsocketEnabled", "false"); err != nil {
		t.Fatalf("UpdateOption OpenRestyWebsocketEnabled failed: %v", err)
	}
	diff, err = DiffConfigVersion()
	if err != nil {
		t.Fatalf("DiffConfigVersion after option change failed: %v", err)
	}
	if !diff.MainConfigChanged {
		t.Fatal("expected main config change after OpenResty option update")
	}
	if len(diff.ChangedOptionKeys) == 0 || diff.ChangedOptionKeys[0] == "" {
		t.Fatal("expected changed OpenResty option keys to be reported")
	}
	if len(diff.ChangedOptionDetails) == 0 {
		t.Fatal("expected changed OpenResty option details to be reported")
	}
	found := false
	foundWebsocket := false
	for _, item := range diff.ChangedOptionDetails {
		if item.Key == "OpenRestyProxyReadTimeout" {
			found = true
			if item.PreviousValue != "60" || item.CurrentValue != "120" {
				t.Fatalf("unexpected option diff values: %+v", item)
			}
		}
		if item.Key == "OpenRestyWebsocketEnabled" {
			foundWebsocket = true
			if item.PreviousValue != "true" || item.CurrentValue != "false" {
				t.Fatalf("unexpected websocket option diff values: %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("expected OpenRestyProxyReadTimeout diff detail")
	}
	if !foundWebsocket {
		t.Fatal("expected OpenRestyWebsocketEnabled diff detail")
	}
}

func TestRenderConfigRejectsUnknownSubdomainHosts(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "git.arctel.net",
		OriginURL: "http://127.0.0.1:8080",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	preview, err := PreviewConfigVersion()
	if err != nil {
		t.Fatalf("PreviewConfigVersion failed: %v", err)
	}

	if !strings.Contains(preview.RenderedConfig, `server_name git.arctel.net;`) {
		t.Fatal("expected rendered config to include exact server_name")
	}
	if !strings.Contains(preview.RenderedConfig, `if ($host != "git.arctel.net") {`) {
		t.Fatal("expected rendered config to guard against unknown subdomain host matches")
	}
	if !strings.Contains(preview.RenderedConfig, "return 404;") {
		t.Fatal("expected rendered config to return 404 when host does not exactly match route domain")
	}
}

func TestCreateTLSCertificateRejectsInvalidPEM(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateTLSCertificate(TLSCertificateInput{
		Name:    "broken-cert",
		CertPEM: "invalid",
		KeyPEM:  "invalid",
	})
	if err == nil {
		t.Fatal("expected invalid pem to fail")
	}
}

func TestOpenRestyMainConfigTemplateRenderAndValidate(t *testing.T) {
	setupServiceTestDB(t)

	customTemplate := strings.ReplaceAll(
		common.OpenRestyMainConfigTemplate,
		"pid logs/nginx.pid;",
		"pid logs/nginx.pid;\nworker_shutdown_timeout 10s;",
	)
	if err := ValidateOpenRestyMainConfigTemplate(customTemplate); err != nil {
		t.Fatalf("ValidateOpenRestyMainConfigTemplate failed: %v", err)
	}
	if err := model.UpdateOption("OpenRestyMainConfigTemplate", customTemplate); err != nil {
		t.Fatalf("UpdateOption OpenRestyMainConfigTemplate failed: %v", err)
	}

	preview, err := PreviewConfigVersion()
	if err != nil {
		t.Fatalf("PreviewConfigVersion failed: %v", err)
	}
	if !strings.Contains(preview.MainConfig, "worker_shutdown_timeout 10s;") {
		t.Fatal("expected preview main config to include custom template content")
	}
	if strings.Contains(preview.MainConfig, "{{OpenRestyWorkerProcesses}}") {
		t.Fatal("expected preview main config placeholders to be rendered")
	}
	if !strings.Contains(preview.MainConfig, "include __OPENFLARE_ROUTE_CONFIG__;") {
		t.Fatal("expected preview main config to preserve managed route include")
	}
	if !strings.Contains(preview.MainConfig, "access_log __OPENFLARE_ACCESS_LOG__ openflare_json;") {
		t.Fatal("expected preview main config to preserve managed access log placeholder")
	}

	invalidTemplate := strings.ReplaceAll(
		common.OpenRestyMainConfigTemplate,
		"{{OpenRestyRouteConfigInclude}}",
		"",
	)
	if err := ValidateOpenRestyMainConfigTemplate(invalidTemplate); err == nil {
		t.Fatal("expected template without managed route placeholder to fail validation")
	}

	invalidTemplate = strings.ReplaceAll(
		common.OpenRestyMainConfigTemplate,
		"{{OpenRestyAccessLogPath}}",
		"",
	)
	if err := ValidateOpenRestyMainConfigTemplate(invalidTemplate); err == nil {
		t.Fatal("expected template without managed access log placeholder to fail validation")
	}
}

func TestOpenRestyCommonRequestOptionsRender(t *testing.T) {
	setupServiceTestDB(t)

	if err := model.UpdateOption("OpenRestyClientMaxBodySize", "128m"); err != nil {
		t.Fatalf("UpdateOption OpenRestyClientMaxBodySize failed: %v", err)
	}
	if err := model.UpdateOption("OpenRestyLargeClientHeaderBuffers", "8 32k"); err != nil {
		t.Fatalf("UpdateOption OpenRestyLargeClientHeaderBuffers failed: %v", err)
	}
	if err := model.UpdateOption("OpenRestyProxyRequestBufferingEnabled", "false"); err != nil {
		t.Fatalf("UpdateOption OpenRestyProxyRequestBufferingEnabled failed: %v", err)
	}

	preview, err := PreviewConfigVersion()
	if err != nil {
		t.Fatalf("PreviewConfigVersion failed: %v", err)
	}
	if !strings.Contains(preview.MainConfig, "client_max_body_size 128m;") {
		t.Fatal("expected preview main config to include client_max_body_size")
	}
	if !strings.Contains(preview.MainConfig, "large_client_header_buffers 8 32k;") {
		t.Fatal("expected preview main config to include large_client_header_buffers")
	}
	if !strings.Contains(preview.MainConfig, "proxy_request_buffering off;") {
		t.Fatal("expected preview main config to include proxy_request_buffering off")
	}
}

func TestOpenRestyProxyRequestBufferingDefaultsToOff(t *testing.T) {
	setupServiceTestDB(t)

	preview, err := PreviewConfigVersion()
	if err != nil {
		t.Fatalf("PreviewConfigVersion failed: %v", err)
	}
	if !strings.Contains(preview.MainConfig, "proxy_request_buffering off;") {
		t.Fatal("expected preview main config to default proxy_request_buffering to off")
	}
}

func setupServiceTestDB(t *testing.T) {
	t.Helper()
	nodeAgentTokenCache.reset()
	common.SQLitePath = filepath.Join(t.TempDir(), "service.db")
	if err := model.InitDB(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	t.Cleanup(func() {
		nodeAgentTokenCache.reset()
		if err := model.CloseDB(); err != nil {
			t.Fatalf("failed to close db: %v", err)
		}
	})
}

func generateCertificatePair(t *testing.T, dnsNames []string) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	template := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: dnsNames[0],
		},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         false,
		SerialNumber: big.NewInt(time.Now().UnixNano()),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate failed: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return string(certPEM), string(keyPEM)
}
