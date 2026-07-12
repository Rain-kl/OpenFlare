// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	oftls "github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCertificateSupportFilesDecryptsSealedPrivateKey(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	require.NoError(t, db.DB(context.Background()).AutoMigrate(&model.TLSCertificate{}))

	oldSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "test-session-secret-for-tls-seal"
	t.Cleanup(func() { config.Config.App.SessionSecret = oldSecret })

	ctx := context.Background()
	certPEM, keyPEM := generateTestCertKeyPairForSnapshot(t)
	certificate, err := oftls.CreateCertificate(ctx, oftls.CertificateInput{
		Name:    "publish-cert",
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	})
	require.NoError(t, err)

	files, err := buildCertificateSupportFiles(ctx, []snapshotRoute{
		{DomainCertIDs: []uint{certificate.ID}},
	})
	require.NoError(t, err)
	require.Len(t, files, 2)

	var keyContent string
	for _, file := range files {
		if file.Path == certificateKeyFileName(certificate.ID) {
			keyContent = file.Content
		}
		assert.NotContains(t, file.Content, "enc:v1:")
	}
	assert.Contains(t, keyContent, "BEGIN")
	assert.Equal(t, normalizePEM(strings.TrimSpace(keyPEM)), keyContent)
}

func TestBuildSnapshotReadsZoneDomainCertificates(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, db.DB(ctx).AutoMigrate(&model.TLSCertificate{}))

	oldSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "test-session-secret-for-zone-domain-snapshots"
	t.Cleanup(func() { config.Config.App.SessionSecret = oldSecret })

	firstCertPEM, firstKeyPEM := generateTestCertKeyPairForSnapshotForDomain(t, "one.example.com")
	first, err := oftls.CreateCertificate(ctx, oftls.CertificateInput{Name: "first", CertPEM: firstCertPEM, KeyPEM: firstKeyPEM})
	require.NoError(t, err)
	secondCertPEM, secondKeyPEM := generateTestCertKeyPairForSnapshotForDomain(t, "two.example.com")
	second, err := oftls.CreateCertificate(ctx, oftls.CertificateInput{Name: "second", CertPEM: secondCertPEM, KeyPEM: secondKeyPEM})
	require.NoError(t, err)

	route := &model.ProxyRoute{SiteName: "tls-site", Domain: "legacy.invalid", OriginURL: "http://origin:8080", Upstreams: `["http://origin:8080"]`, Enabled: true, EnableHTTPS: true}
	require.NoError(t, model.CreateProxyRouteRecord(ctx, route))
	zone := &model.Zone{Domain: "example.com"}
	require.NoError(t, db.DB(ctx).Create(zone).Error)
	require.NoError(t, db.DB(ctx).Create(&model.ZoneDomain{ZoneID: zone.ID, ProxyRouteID: &route.ID, Domain: "one.example.com", CertID: &first.ID}).Error)
	require.NoError(t, db.DB(ctx).Create(&model.ZoneDomain{ZoneID: zone.ID, ProxyRouteID: &route.ID, Domain: "two.example.com", CertID: &second.ID}).Error)

	bundle, err := buildCurrentConfigBundle(ctx, true)
	require.NoError(t, err)
	require.Len(t, bundle.SnapshotRoutes, 1)
	assert.Equal(t, []string{"one.example.com", "two.example.com"}, bundle.SnapshotRoutes[0].Domains)
	assert.Equal(t, []uint{first.ID, second.ID}, bundle.SnapshotRoutes[0].DomainCertIDs)
	assert.Contains(t, bundle.RouteConfig, "server_name one.example.com;")
	assert.Contains(t, bundle.RouteConfig, "server_name two.example.com;")
}

func generateTestCertKeyPairForSnapshot(t *testing.T) (certPEM string, keyPEM string) {
	return generateTestCertKeyPairForSnapshotForDomain(t, "test.example.com")
}

func generateTestCertKeyPairForSnapshotForDomain(t *testing.T, domain string) (certPEM string, keyPEM string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: domain},
		DNSNames:     []string{domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))
	return certPEM, keyPEM
}
