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
	if strings.Contains(result.Version.MainConfig, "resolver ") {
		t.Fatal("expected main config to omit resolver directive when no resolvers are configured")
	}
	if !strings.Contains(result.Version.MainConfig, "use epoll;") {
		t.Fatal("expected main config to default to epoll event model")
	}
	if !strings.Contains(result.Version.MainConfig, "multi_accept on;") {
		t.Fatal("expected main config to default multi_accept to on")
	}
	if !strings.Contains(result.Version.MainConfig, "keepalive_timeout 20;") {
		t.Fatal("expected main config to default keepalive_timeout to 20")
	}
	if !strings.Contains(result.Version.MainConfig, "proxy_connect_timeout 3;") {
		t.Fatal("expected main config to default proxy_connect_timeout to 3")
	}
	if strings.Contains(result.Version.MainConfig, "allow 127.0.0.1;") {
		t.Fatal("expected main config to avoid hard-coded allow rules on observability server")
	}
	if !strings.Contains(result.Version.RenderedConfig, "listen 443 ssl;") {
		t.Fatal("expected rendered config to include https ssl listener")
	}
	if !strings.Contains(result.Version.RenderedConfig, "http2 on;") {
		t.Fatal("expected rendered config to enable http2 with dedicated directive")
	}
	if strings.Contains(result.Version.RenderedConfig, `if ($host != "app.example.com") {`) {
		t.Fatal("expected rendered config to avoid per-route host guard")
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

func TestCreateProxyRouteSupportsWebsiteDomains(t *testing.T) {
	setupServiceTestDB(t)

	route, err := CreateProxyRoute(ProxyRouteInput{
		SiteName:  "main-site",
		Domains:   []string{"app.example.com", "www.example.com"},
		OriginURL: "https://origin.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	if route.SiteName != "main-site" {
		t.Fatalf("unexpected site name: %s", route.SiteName)
	}
	if route.Domain != "app.example.com" {
		t.Fatalf("expected primary domain mirror, got %s", route.Domain)
	}
	if !strings.Contains(route.Domains, "www.example.com") {
		t.Fatalf("expected domains payload to contain alias, got %s", route.Domains)
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
	if !strings.Contains(result.Version.RenderedConfig, "proxy_set_header Connection $connection_upgrade;") {
		t.Fatal("expected rendered config to use normalized websocket connection header")
	}
	if !strings.Contains(result.Version.RenderedConfig, "upstream backend_custom_example_com_1 {") {
		t.Fatal("expected hostname origin to render named upstream")
	}
	if !strings.Contains(result.Version.RenderedConfig, "server origin.internal max_fails=3 fail_timeout=10s;") {
		t.Fatal("expected hostname origin to render upstream server entry")
	}
	if !strings.Contains(result.Version.RenderedConfig, "keepalive 128;") {
		t.Fatal("expected named upstream to enable keepalive")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_pass https://backend_custom_example_com_1;") {
		t.Fatal("expected hostname origin to proxy through named upstream")
	}
}

func TestCreateProxyRouteRejectsCachePolicyWithoutRules(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:       "cache.example.com",
		OriginURL:    "https://origin.internal",
		Enabled:      true,
		CacheEnabled: true,
		CachePolicy:  proxyRouteCachePolicySuffix,
	})
	if err == nil || !strings.Contains(err.Error(), "至少填写一个后缀") {
		t.Fatalf("expected cache rule validation error, got %v", err)
	}
}

func TestPublishConfigVersionRendersRouteLevelCachePolicy(t *testing.T) {
	setupServiceTestDB(t)
	if err := model.UpdateOption("OpenRestyCacheEnabled", "true"); err != nil {
		t.Fatalf("UpdateOption OpenRestyCacheEnabled failed: %v", err)
	}
	if err := model.UpdateOption("OpenRestyCachePath", "/var/cache/openresty/openflare"); err != nil {
		t.Fatalf("UpdateOption OpenRestyCachePath failed: %v", err)
	}

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:       "static.example.com",
		OriginURL:    "https://origin.internal",
		Enabled:      true,
		CacheEnabled: true,
		CachePolicy:  proxyRouteCachePolicySuffix,
		CacheRules:   []string{"jpg", ".css", "js"},
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute cached failed: %v", err)
	}
	_, err = CreateProxyRoute(ProxyRouteInput{
		Domain:    "nocache.example.com",
		OriginURL: "https://origin.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute uncached failed: %v", err)
	}

	result, err := PublishConfigVersion("root")
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.MainConfig, "proxy_cache_path /var/cache/openresty/openflare") {
		t.Fatal("expected main config to include cache zone when cache infra is enabled")
	}
	if !strings.Contains(result.Version.MainConfig, `proxy_cache_key "$scheme$host$request_uri";`) {
		t.Fatal("expected main config to default cache key to host dimension")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_cache_methods GET;") {
		t.Fatal("expected rendered config to only cache GET requests")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_cache_bypass $openflare_skip_cache;") {
		t.Fatal("expected rendered config to bypass cache when request is unsafe")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_no_cache $openflare_skip_cache;") {
		t.Fatal("expected rendered config to avoid storing unsafe requests in cache")
	}
	if !strings.Contains(result.Version.RenderedConfig, "if ($http_authorization != \"\")") {
		t.Fatal("expected rendered config to bypass authenticated requests")
	}
	if !strings.Contains(result.Version.RenderedConfig, "if ($request_method != GET)") {
		t.Fatal("expected rendered config to bypass non-GET requests")
	}
	if !strings.Contains(result.Version.RenderedConfig, "if ($uri !~* \"\\\\.(?:jpg|css|js)$\")") {
		t.Fatal("expected rendered config to render suffix cache matching rule")
	}
	if strings.Count(result.Version.RenderedConfig, "proxy_cache openflare_cache;") != 1 {
		t.Fatal("expected only cache-enabled route to include proxy_cache directive")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_pass https://backend_static_example_com_1;") {
		t.Fatal("expected cache-enabled hostname route to proxy through named upstream")
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"cache_enabled":true`) {
		t.Fatal("expected snapshot to include route cache toggle")
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"cache_policy":"suffix"`) {
		t.Fatal("expected snapshot to include route cache policy")
	}
}

func TestPublishConfigVersionRendersMultipleUpstreams(t *testing.T) {
	setupServiceTestDB(t)

	route, err := CreateProxyRoute(ProxyRouteInput{
		Domain:     "lb.example.com",
		OriginURL:  "http://10.0.0.11:39010",
		Upstreams:  []string{"http://10.0.0.12:39010", "http://10.0.0.13:39010"},
		Enabled:    true,
		OriginHost: "lb.example.com",
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	if !strings.Contains(route.Upstreams, "10.0.0.12:39010") {
		t.Fatalf("expected route upstreams to persist, got %s", route.Upstreams)
	}

	result, err := PublishConfigVersion("root")
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.RenderedConfig, "upstream backend_lb_example_com_1 {") {
		t.Fatal("expected rendered config to define upstream block for load balancing route")
	}
	if strings.Count(result.Version.RenderedConfig, "max_fails=3 fail_timeout=10s;") < 3 {
		t.Fatal("expected rendered config to include every upstream server")
	}
	if !strings.Contains(result.Version.RenderedConfig, "server 10.0.0.11:39010 max_fails=3 fail_timeout=10s;") {
		t.Fatal("expected rendered config to include primary upstream server")
	}
	if !strings.Contains(result.Version.RenderedConfig, "server 10.0.0.12:39010 max_fails=3 fail_timeout=10s;") {
		t.Fatal("expected rendered config to include secondary upstream server")
	}
	if !strings.Contains(result.Version.RenderedConfig, "server 10.0.0.13:39010 max_fails=3 fail_timeout=10s;") {
		t.Fatal("expected rendered config to include tertiary upstream server")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_pass http://backend_lb_example_com_1;") {
		t.Fatal("expected rendered config to proxy through load balancing upstream")
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"upstreams":["http://10.0.0.11:39010","http://10.0.0.12:39010","http://10.0.0.13:39010"]`) {
		t.Fatal("expected snapshot to include upstream list")
	}
}

func TestPublishConfigVersionRendersMultiDomainWebsite(t *testing.T) {
	setupServiceTestDB(t)

	certPEM, keyPEM := generateCertificatePair(t, []string{"app.example.com", "www.example.com"})
	certificate, err := CreateTLSCertificate(TLSCertificateInput{
		Name:    "multi-domain",
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	})
	if err != nil {
		t.Fatalf("CreateTLSCertificate failed: %v", err)
	}

	_, err = CreateProxyRoute(ProxyRouteInput{
		SiteName:      "marketing-site",
		Domains:       []string{"app.example.com", "www.example.com"},
		OriginURL:     "https://origin.internal",
		Enabled:       true,
		EnableHTTPS:   true,
		CertID:        &certificate.ID,
		RedirectHTTP:  true,
		CacheEnabled:  true,
		CachePolicy:   proxyRouteCachePolicyPathPrefix,
		CacheRules:    []string{"/assets"},
		CustomHeaders: []ProxyRouteCustomHeaderInput{{Key: "X-Site", Value: "marketing"}},
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	result, err := PublishConfigVersion("root")
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.RenderedConfig, "server_name app.example.com www.example.com;") {
		t.Fatal("expected rendered config to include all domains in one server_name")
	}
	if strings.Contains(result.Version.RenderedConfig, "server_name app.example.com;") {
		t.Fatal("expected rendered config to avoid standalone primary-domain server block")
	}
	if strings.Contains(result.Version.RenderedConfig, "server_name www.example.com;") {
		t.Fatal("expected rendered config to avoid standalone alias server block")
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"site_name":"marketing-site"`) {
		t.Fatal("expected snapshot to include site_name")
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"domains":["app.example.com","www.example.com"]`) {
		t.Fatal("expected snapshot to include domain list")
	}
}

func TestDiffConfigVersionTracksAddedDomainWithinWebsite(t *testing.T) {
	setupServiceTestDB(t)

	route, err := CreateProxyRoute(ProxyRouteInput{
		SiteName:  "main-site",
		Domains:   []string{"app.example.com"},
		OriginURL: "https://origin.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	if _, err := PublishConfigVersion("root"); err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}

	if _, err := UpdateProxyRoute(route.ID, ProxyRouteInput{
		SiteName:  "main-site",
		Domains:   []string{"app.example.com", "www.example.com"},
		OriginURL: "https://origin.internal",
		Enabled:   true,
	}); err != nil {
		t.Fatalf("UpdateProxyRoute failed: %v", err)
	}

	diff, err := DiffConfigVersion()
	if err != nil {
		t.Fatalf("DiffConfigVersion failed: %v", err)
	}
	if len(diff.AddedDomains) != 1 || diff.AddedDomains[0] != "www.example.com" {
		t.Fatalf("unexpected added domains: %#v", diff.AddedDomains)
	}
	if len(diff.ModifiedDomains) != 1 || diff.ModifiedDomains[0] != "app.example.com" {
		t.Fatalf("unexpected modified domains: %#v", diff.ModifiedDomains)
	}
}

func TestPublishConfigVersionRendersHostnameLoadBalancingUpstream(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "hostname-lb.example.com",
		OriginURL: "http://c1:39010",
		Upstreams: []string{"http://c2:39010"},
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	result, err := PublishConfigVersion("root")
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.RenderedConfig, "upstream backend_hostname_lb_example_com_1 {") {
		t.Fatal("expected hostname load balancing route to define named upstream")
	}
	if !strings.Contains(result.Version.RenderedConfig, "server c1:39010 max_fails=3 fail_timeout=10s;") {
		t.Fatal("expected rendered config to include primary hostname upstream")
	}
	if !strings.Contains(result.Version.RenderedConfig, "server c2:39010 max_fails=3 fail_timeout=10s;") {
		t.Fatal("expected rendered config to include secondary hostname upstream")
	}
	if strings.Contains(result.Version.RenderedConfig, " resolve ") {
		t.Fatal("expected hostname upstreams to avoid resolver-based server parameters")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_pass http://backend_hostname_lb_example_com_1;") {
		t.Fatal("expected hostname load balancing route to proxy through named upstream")
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
	if !strings.Contains(result.Version.RenderedConfig, "upstream backend_git_arctel_de_1 {") {
		t.Fatal("expected hostname origin to render named upstream")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_pass https://backend_git_arctel_de_1;") {
		t.Fatal("expected rendered config to proxy through named upstream for hostname origin")
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"origin_host":"git.arctel.net"`) {
		t.Fatal("expected snapshot to include origin_host override")
	}
}

func TestPublishConfigVersionUsesNamedUpstreamForOriginBasePath(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "resolver.example.com",
		OriginURL: "https://origin.internal/api/",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	result, err := PublishConfigVersion("root")
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.RenderedConfig, "upstream backend_resolver_example_com_1 {") {
		t.Fatal("expected hostname origin with base path to still render named upstream")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_pass https://backend_resolver_example_com_1/api/;") {
		t.Fatal("expected rendered config to preserve base path while proxying through named upstream")
	}
}

func TestPublishConfigVersionUsesNamedUpstreamForHostnameOrigins(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "resolver-upstream.example.com",
		OriginURL: "https://origin.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	result, err := PublishConfigVersion("root")
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.RenderedConfig, "upstream backend_resolver_upstream_example_com_1 {") {
		t.Fatal("expected rendered config to define named upstream for hostname origin")
	}
	if !strings.Contains(result.Version.RenderedConfig, "server origin.internal max_fails=3 fail_timeout=10s;") {
		t.Fatal("expected rendered config to include hostname upstream server entry")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_pass https://backend_resolver_upstream_example_com_1;") {
		t.Fatal("expected rendered config to proxy through named upstream for hostname origin")
	}
}

func TestPublishConfigVersionUsesNamedUpstreamForIPOrigins(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "ip-origin.example.com",
		OriginURL: "http://10.0.0.8:8080",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	result, err := PublishConfigVersion("root")
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.RenderedConfig, "upstream backend_ip_origin_example_com_1 {") {
		t.Fatal("expected rendered config to define named upstream for static IP origins")
	}
	if !strings.Contains(result.Version.RenderedConfig, "proxy_pass http://backend_ip_origin_example_com_1;") {
		t.Fatal("expected rendered config to proxy through named upstream for IP origin")
	}
	if strings.Contains(result.Version.RenderedConfig, `set $openflare_upstream "http://10.0.0.8:8080"`) {
		t.Fatal("expected rendered config to avoid runtime resolver variables for IP origin")
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
	if !strings.Contains(preview.RenderedConfig, "proxy_http_version 1.1;") {
		t.Fatal("expected preview config to keep HTTP/1.1 proxying for named upstream keepalive")
	}
	if !strings.Contains(preview.RenderedConfig, `proxy_set_header Connection "";`) {
		t.Fatal("expected preview config to clear connection header when websocket upgrades are disabled")
	}
	if strings.Contains(preview.RenderedConfig, "proxy_set_header Upgrade $http_upgrade;") {
		t.Fatal("expected preview config to omit websocket upgrade header when disabled")
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

func TestRenderConfigUsesDefaultServerFallback(t *testing.T) {
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
	if strings.Contains(preview.RenderedConfig, `if ($host != "git.arctel.net") {`) {
		t.Fatal("expected rendered config to avoid per-route host guard")
	}
	if !strings.Contains(preview.MainConfig, "listen 80 default_server;") {
		t.Fatal("expected preview main config to include default http server")
	}
	if !strings.Contains(preview.MainConfig, "server_name _;") {
		t.Fatal("expected preview main config to include default server_name")
	}
	if !strings.Contains(preview.MainConfig, "return 404;") {
		t.Fatal("expected preview main config to return 404 for unmatched hosts")
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
	if !strings.Contains(preview.MainConfig, "map $http_upgrade $connection_upgrade {") {
		t.Fatal("expected preview main config to preserve managed websocket upgrade map")
	}
	if !strings.Contains(preview.MainConfig, "listen 80 default_server;") {
		t.Fatal("expected preview main config to preserve managed default server block")
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

	invalidTemplate = strings.ReplaceAll(
		common.OpenRestyMainConfigTemplate,
		"{{OpenRestyConnectionUpgradeMap}}",
		"",
	)
	if err := ValidateOpenRestyMainConfigTemplate(invalidTemplate); err == nil {
		t.Fatal("expected template without managed websocket upgrade map placeholder to fail validation")
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
