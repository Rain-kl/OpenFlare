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

	"Wavelet/openflare/plugins/server/kernel/credential"
	"Wavelet/openflare/plugins/server/kernel/repository"
	oftask "Wavelet/openflare/plugins/server/kernel/task"
	"Wavelet/openflare/plugins/server/kernel/testhelper"

	"Wavelet/openflare/plugins/server/kernel/model"
	"Wavelet/openflare/plugins/server/kernel/runtimeconfig"
	"Wavelet/pkg/idgen"

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
		&model.Zone{},
		&model.ZoneDomain{},
		&model.DNSAccount{},
		&model.AcmeAccount{},
		&model.TaskExecution{}, // 异步任务执行记录也需要 migrate
	))

	repository.SetDBForTest(sqliteDB)
	require.NoError(t, idgen.Init(1))
	previous := runtimeconfig.Get()
	runtimeconfig.SetSessionSecret("test_session_secret_for_tls_encryption")
	credential.SetSessionSecret("test_session_secret_for_tls_encryption")
	oftask.SetService(&testhelper.NoopTaskService{})

	return func() {
		repository.SetDBForTest(nil)
		runtimeconfig.Set(previous)
		credential.SetSessionSecret(previous.SessionSecret)
		tlsTestDBMu.Unlock()
	}
}

func TestDeleteCertificateRejectsZoneDomainReference(t *testing.T) {
	cleanup := setupTLSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	certPEM, keyPEM := generateTestCertificatePair(t, []string{"api.example.com"})
	certificate, err := CreateCertificate(ctx, CertificateInput{Name: "api-cert", CertPEM: certPEM, KeyPEM: keyPEM})
	require.NoError(t, err)
	zone := &model.Zone{Domain: "example.com"}
	require.NoError(t, repository.DB(ctx).Create(zone).Error)
	require.NoError(t, repository.DB(ctx).Create(&model.ZoneDomain{ZoneID: zone.ID, Domain: "api.example.com", CertID: &certificate.ID}).Error)

	err = DeleteCertificate(ctx, certificate.ID)
	require.EqualError(t, err, errCertificateDeleteReferenced)
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

	stored, err := repository.GetTLSCertificateByID(ctx, certificate.ID)
	require.NoError(t, err)
	assert.NotEqual(t, keyPEM, stored.KeyPEM)
	assert.Contains(t, stored.KeyPEM, credential.Prefix)

	content, err := GetCertificateContent(ctx, certificate.ID)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(keyPEM), strings.TrimSpace(content.KeyPEM))
}
