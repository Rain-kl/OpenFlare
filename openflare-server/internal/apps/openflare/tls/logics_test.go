// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var tlsTestDBMu sync.Mutex

func setupTLSTestDB(t *testing.T) func() {
	t.Helper()
	tlsTestDBMu.Lock()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.TLSCertificate{},
		&model.ManagedDomain{},
		&model.DNSAccount{},
		&model.AcmeAccount{},
	))

	db.SetDB(sqliteDB)
	oldSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "test_session_secret_for_tls_encryption"
	return func() {
		db.SetDB(nil)
		config.Config.App.SessionSecret = oldSecret
		tlsTestDBMu.Unlock()
	}
}

func generateTestCertificatePair(t *testing.T, dnsNames []string) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
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
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return string(certPEM), string(keyPEM)
}

func TestCreateManagedDomain(t *testing.T) {
	cleanup := setupTLSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	certPEM, keyPEM := generateTestCertificatePair(t, []string{"api.example.com"})
	certificate, err := CreateCertificate(ctx, CertificateInput{
		Name:    "api-cert",
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	})
	require.NoError(t, err)

	certID := certificate.ID
	domain, err := CreateManagedDomain(ctx, ManagedDomainInput{
		Domain:  "api.example.com",
		CertID:  &certID,
		Enabled: true,
		Remark:  "primary api",
	})
	require.NoError(t, err)
	assert.NotZero(t, domain.ID)
	assert.Equal(t, "api.example.com", domain.Domain)
	assert.Equal(t, certID, *domain.CertID)
	assert.True(t, domain.Enabled)
	assert.Equal(t, "primary api", domain.Remark)

	_, err = CreateManagedDomain(ctx, ManagedDomainInput{
		Domain:  "api.example.com",
		Enabled: true,
	})
	require.Error(t, err)
	assert.Equal(t, errManagedDomainExists, err.Error())
}

func TestCreateManagedDomainRejectsInvalidWildcard(t *testing.T) {
	cleanup := setupTLSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := CreateManagedDomain(ctx, ManagedDomainInput{
		Domain:  "*.*.example.com",
		Enabled: true,
	})
	require.Error(t, err)
	assert.Equal(t, errManagedDomainWildcardInvalid, err.Error())
}

func TestCreateCertificateEncryptsPrivateKey(t *testing.T) {
	cleanup := setupTLSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	certPEM, keyPEM := generateTestCertificatePair(t, []string{"secure.example.com"})
	certificate, err := CreateCertificate(ctx, CertificateInput{
		Name:    "secure-cert",
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	})
	require.NoError(t, err)

	stored, err := model.GetTLSCertificateByID(ctx, certificate.ID)
	require.NoError(t, err)
	assert.NotEqual(t, keyPEM, stored.KeyPEM)
	assert.Contains(t, stored.KeyPEM, sensitiveValuePrefix)

	content, err := GetCertificateContent(ctx, certificate.ID)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(keyPEM), strings.TrimSpace(content.KeyPEM))
}
