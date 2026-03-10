package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"gin-template/common"
	"gin-template/model"
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
