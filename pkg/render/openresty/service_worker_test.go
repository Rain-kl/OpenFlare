// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openresty

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestEffectiveSWOfflineHTML(t *testing.T) {
	if got := EffectiveSWOfflineHTML(ConfigSnapshot{}); got != DefaultSWOfflineHTML {
		t.Fatalf("default mismatch")
	}
	custom := "<html>custom</html>"
	if got := EffectiveSWOfflineHTML(ConfigSnapshot{SWOfflineHTML: custom}); got != custom {
		t.Fatalf("custom mismatch")
	}
}

func TestServiceWorkerSupportFiles(t *testing.T) {
	disabled := ServiceWorkerSupportFiles(ConfigSnapshot{})
	if disabled != nil {
		t.Fatalf("expected nil when disabled, got %v", disabled)
	}
	enabled := ServiceWorkerSupportFiles(ConfigSnapshot{SWOfflineEnabled: true})
	if len(enabled) != 2 {
		t.Fatalf("expected 2 support files, got %d", len(enabled))
	}
	paths := map[string]string{}
	for _, f := range enabled {
		paths[f.Path] = f.Content
	}
	if _, ok := paths["sw/sw.js"]; !ok {
		t.Fatalf("missing sw/sw.js")
	}
	if _, ok := paths["sw/offline.html"]; !ok {
		t.Fatalf("missing sw/offline.html")
	}
	if paths["sw/offline.html"] != DefaultSWOfflineHTML {
		t.Fatalf("expected built-in offline html, got %q", paths["sw/offline.html"])
	}
	if !strings.Contains(paths["sw/sw.js"], `var OFFLINE = "/offline.html";`) {
		t.Fatalf("offline path must stay stable (exact location match), got:\n%s", paths["sw/sw.js"])
	}
}

func TestDefaultSWJSCacheNameTracksOfflineHTML(t *testing.T) {
	htmlA := "<html>page-a</html>"
	htmlB := "<html>page-b</html>"
	jsA := defaultSWJS(htmlA)
	jsB := defaultSWJS(htmlB)
	if jsA == jsB {
		t.Fatal("sw.js content must change when the offline HTML changes")
	}
	extractCache := func(js string) string {
		const prefix = `var CACHE = "`
		start := strings.Index(js, prefix)
		if start < 0 {
			t.Fatalf("missing cache name in:\n%s", js)
		}
		rest := js[start+len(prefix):]
		end := strings.Index(rest, `"`)
		if end < 0 {
			t.Fatalf("unterminated cache name in:\n%s", js)
		}
		return rest[:end]
	}
	cacheA := extractCache(jsA)
	cacheB := extractCache(jsB)
	if cacheA == cacheB {
		t.Fatalf("cache names must differ per HTML, got %q", cacheA)
	}
	if !strings.HasPrefix(cacheA, "openflare-offline-") {
		t.Fatalf("unexpected cache name %q", cacheA)
	}
	for _, js := range []string{jsA, jsB} {
		if strings.Contains(js, "openflare-offline-v1") {
			t.Fatalf("static cache name must not remain, got:\n%s", js)
		}
		if strings.Contains(js, "__CACHE_NAME__") {
			t.Fatalf("template placeholder leaked into sw.js:\n%s", js)
		}
	}
	// Same HTML must produce identical sw.js (deterministic checksum).
	if defaultSWJS(htmlA) != jsA {
		t.Fatal("sw.js must be deterministic for identical HTML")
	}
}

func TestRenderAccessBlockWithSWMergesSingleBlock(t *testing.T) {
	for _, powEnabled := range []bool{false, true} {
		name := "pow-disabled"
		if powEnabled {
			name = "pow-enabled"
		}
		t.Run(name, func(t *testing.T) {
			got := renderAccessBlockWithSW("example.com", powEnabled, ConfigSnapshot{})
			if n := strings.Count(got, "access_by_lua_block"); n != 1 {
				t.Fatalf("expected exactly 1 access_by_lua_block, got %d:\n%s", n, got)
			}
			if !strings.Contains(got, `require("sw.runtime").check()`) {
				t.Fatalf("expected sw.runtime check, got:\n%s", got)
			}
			wafIdx := strings.Index(got, `require("waf.runtime").check()`)
			swIdx := strings.Index(got, `require("sw.runtime").check()`)
			if wafIdx < 0 || swIdx < 0 || wafIdx > swIdx {
				t.Fatalf("expected waf.runtime before sw.runtime, got:\n%s", got)
			}
			if powEnabled {
				powIdx := strings.Index(got, `require("pow.runtime").check()`)
				if powIdx < 0 || wafIdx > powIdx || powIdx > swIdx {
					t.Fatalf("expected waf.runtime before pow.runtime before sw.runtime, got:\n%s", got)
				}
			}
		})
	}
}

func TestRenderServiceWorkerChallengerHTTPExclusion(t *testing.T) {
	cfg := ConfigSnapshot{SWOfflineEnabled: true}
	for name, rendered := range map[string]string{
		"proxy":  renderHTTPProxyServer("example.com", "example.com", "http://127.0.0.1:8080", "", nil, routeCacheConfig{}, routeLimitConfig{}, routeUpstreamConfig{}, false, false, "", "", false, cfg),
		"pages":  renderHTTPPagesServer("example.com", "example.com", nil, routeLimitConfig{}, false, false, "", "", false, cfg),
		"https":  renderHTTPSServer("example.com", "example.com", "http://127.0.0.1:8080", "", 1, nil, routeCacheConfig{}, routeLimitConfig{}, routeUpstreamConfig{}, false, false, "", "", true, cfg),
		"hpages": renderHTTPSPagesServer("example.com", "example.com", 1, nil, routeLimitConfig{}, false, false, "", "", true, cfg),
	} {
		if strings.Contains(rendered, "access_by_lua_block") && strings.Count(rendered, "access_by_lua_block") != 1 {
			t.Fatalf("%s: expected at most one access block, got:\n%s", name, rendered)
		}
	}
	httpProxy := renderHTTPProxyServer("example.com", "example.com", "http://127.0.0.1:8080", "", nil, routeCacheConfig{}, routeLimitConfig{}, routeUpstreamConfig{}, false, false, "", "", false, cfg)
	if strings.Contains(httpProxy, "sw.runtime") || strings.Contains(httpProxy, "openflare_sw_challenge") || strings.Contains(httpProxy, "location = /sw.js") {
		t.Fatalf("HTTP proxy server must not carry SW intercept, got:\n%s", httpProxy)
	}
	httpPages := renderHTTPPagesServer("example.com", "example.com", nil, routeLimitConfig{}, false, false, "", "", false, cfg)
	if strings.Contains(httpPages, "sw.runtime") || strings.Contains(httpPages, "openflare_sw_challenge") || strings.Contains(httpPages, "location = /sw.js") {
		t.Fatalf("HTTP pages server must not carry SW intercept, got:\n%s", httpPages)
	}
	httpsProxy := renderHTTPSServer("example.com", "example.com", "http://127.0.0.1:8080", "", 1, nil, routeCacheConfig{}, routeLimitConfig{}, routeUpstreamConfig{}, false, false, "", "", true, cfg)
	for _, want := range []string{"sw.runtime", "location = /sw.js", "location = /offline.html", "__openflare_sw_challenge"} {
		if !strings.Contains(httpsProxy, want) {
			t.Fatalf("HTTPS proxy server missing %q, got:\n%s", want, httpsProxy)
		}
	}
	httpsPages := renderHTTPSPagesServer("example.com", "example.com", 1, nil, routeLimitConfig{}, false, false, "", "", true, cfg)
	for _, want := range []string{"sw.runtime", "location = /sw.js", "location = /offline.html", "__openflare_sw_challenge"} {
		if !strings.Contains(httpsPages, want) {
			t.Fatalf("HTTPS pages server missing %q, got:\n%s", want, httpsPages)
		}
	}
}

func TestRouteSWEnabled(t *testing.T) {
	cfgOff := ConfigSnapshot{SWOfflineEnabled: false, SWOfflineDomains: []string{"example.com"}}
	if routeSWEnabled([]string{"example.com"}, cfgOff) {
		t.Fatal("expected false when master switch off")
	}
	cfgEmpty := ConfigSnapshot{SWOfflineEnabled: true, SWOfflineDomains: nil}
	if routeSWEnabled([]string{"example.com"}, cfgEmpty) {
		t.Fatal("expected false when scope empty")
	}
	cfgHit := ConfigSnapshot{SWOfflineEnabled: true, SWOfflineDomains: []string{"example.com", "other.com"}}
	if !routeSWEnabled([]string{"api.example.com", "example.com"}, cfgHit) {
		t.Fatal("expected true on single domain intersection")
	}
	if routeSWEnabled([]string{"api.example.com", "third.com"}, cfgHit) {
		t.Fatal("expected false on no intersection")
	}
}

func TestRenderHTTPSServerSWScope(t *testing.T) {
	render := func(swEnabled bool) string {
		return renderHTTPSServer("example.com", "example.com", "http://127.0.0.1:8080", "", 1, nil, routeCacheConfig{}, routeLimitConfig{}, routeUpstreamConfig{}, false, false, "", "", swEnabled, ConfigSnapshot{SWOfflineEnabled: true})
	}
	hit := render(routeSWEnabled([]string{"example.com"}, ConfigSnapshot{SWOfflineEnabled: true, SWOfflineDomains: []string{"example.com"}}))
	for _, want := range []string{`require("sw.runtime").check()`, "location = /sw.js", "location = /offline.html", "__openflare_sw_challenge"} {
		if !strings.Contains(hit, want) {
			t.Fatalf("scoped HTTPS server missing %q, got:\n%s", want, hit)
		}
	}
	miss := render(routeSWEnabled([]string{"example.com"}, ConfigSnapshot{SWOfflineEnabled: true, SWOfflineDomains: []string{"other.com"}}))
	for _, notWant := range []string{`require("sw.runtime").check()`, "location = /sw.js", "location = /offline.html", "__openflare_sw_challenge"} {
		if strings.Contains(miss, notWant) {
			t.Fatalf("out-of-scope HTTPS server must not carry %q, got:\n%s", notWant, miss)
		}
	}
	if miss != renderHTTPSServer("example.com", "example.com", "http://127.0.0.1:8080", "", 1, nil, routeCacheConfig{}, routeLimitConfig{}, routeUpstreamConfig{}, false, false, "", "", false, ConfigSnapshot{}) {
		t.Fatalf("out-of-scope HTTPS server must match pre-feature bytes, got:\n%s", miss)
	}
}

func TestRenderRouteConfigSWSCOPEPerCertPartition(t *testing.T) {
	doc := Document{
		OpenRestyConfig: ConfigSnapshot{
			SWOfflineEnabled: true,
			SWOfflineDomains: []string{"a.com"},
		},
		Routes: []Route{{
			ID:            1,
			SiteName:      "multi.example.com",
			Domains:       []string{"a.com", "b.com"},
			OriginURL:     "http://127.0.0.1:8080",
			EnableHTTPS:   true,
			DomainCertIDs: []uint{11, 22},
		}},
	}
	certFiles := []SupportFile{
		{Path: "11.crt", Content: testCertificatePEMForDomain(t, "a.com")},
		{Path: "22.crt", Content: testCertificatePEMForDomain(t, "b.com")},
	}
	rendered, err := RenderRouteConfig(doc, certFiles)
	if err != nil {
		t.Fatalf("RenderRouteConfig() error = %v", err)
	}
	inScope := httpsServerBlockForCert(t, rendered, 11)
	if !strings.Contains(inScope, "server_name a.com;") {
		t.Fatalf("cert 11 block must serve a.com, got:\n%s", inScope)
	}
	for _, want := range []string{`require("sw.runtime").check()`, "location = /sw.js", "location = /offline.html", "__openflare_sw_challenge"} {
		if !strings.Contains(inScope, want) {
			t.Fatalf("in-scope cert partition (a.com) missing %q, got:\n%s", want, inScope)
		}
	}
	outOfScope := httpsServerBlockForCert(t, rendered, 22)
	if !strings.Contains(outOfScope, "server_name b.com;") {
		t.Fatalf("cert 22 block must serve b.com, got:\n%s", outOfScope)
	}
	for _, notWant := range []string{`require("sw.runtime").check()`, "location = /sw.js", "location = /offline.html", "__openflare_sw_challenge"} {
		if strings.Contains(outOfScope, notWant) {
			t.Fatalf("out-of-scope cert partition (b.com) must not carry %q, got:\n%s", notWant, outOfScope)
		}
	}
}

func httpsServerBlockForCert(t *testing.T, rendered string, certID uint) string {
	t.Helper()
	marker := fmt.Sprintf("ssl_certificate %s/%d.crt;", CertDirPlaceholder, certID)
	for _, block := range strings.Split(rendered, "server {") {
		if strings.Contains(block, marker) {
			return "server {" + block
		}
	}
	t.Fatalf("no server block found for cert %d in:\n%s", certID, rendered)
	return ""
}

func testCertificatePEMForDomain(t *testing.T, domain string) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: domain},
		DNSNames:     []string{domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() error = %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestRenderServiceWorkerChallenger(t *testing.T) {
	got := renderServiceWorkerChallenger(ConfigSnapshot{SWOfflineEnabled: true})
	for _, want := range []string{"location = /sw.js", "location = /offline.html", "challenge.lua", "content_by_lua"} {
		if !strings.Contains(got, want) {
			t.Fatalf("challenger missing %q", want)
		}
	}
}
