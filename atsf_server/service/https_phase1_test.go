package service

import (
	"atsflare/common"
	"atsflare/model"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
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
	if !strings.Contains(result.Version.MainConfig, "include __ATSF_ROUTE_CONFIG__;") {
		t.Fatal("expected main config to include managed route config placeholder")
	}
	if !strings.Contains(result.Version.RenderedConfig, "listen 443 ssl;") {
		t.Fatal("expected rendered config to include https server block")
	}
	if !strings.Contains(result.Version.RenderedConfig, "return 301 https://$host$request_uri;") {
		t.Fatal("expected rendered config to include http redirect")
	}
	if !strings.Contains(result.Version.RenderedConfig, "__ATSF_CERT_DIR__/") {
		t.Fatal("expected rendered config to keep certificate dir placeholder")
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
}

func TestPreviewAndDiffConfigVersion(t *testing.T) {
	setupServiceTestDB(t)

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
	if !strings.Contains(preview.MainConfig, "include __ATSF_ROUTE_CONFIG__;") {
		t.Fatal("expected preview main config to include managed route config placeholder")
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

func setupServiceTestDB(t *testing.T) {
	t.Helper()
	common.SQLitePath = filepath.Join(t.TempDir(), "service.db")
	if err := model.InitDB(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	t.Cleanup(func() {
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
