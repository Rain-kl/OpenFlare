// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitAcmeDomains(t *testing.T) {
	assert.Equal(t, []string{"example.com"}, splitAcmeDomains("example.com", ""))
	assert.Equal(t, []string{"example.com", "*.example.com"}, splitAcmeDomains("example.com", "*.example.com"))
	assert.Equal(t, []string{"example.com", "www.example.com", "api.example.com"}, splitAcmeDomains("example.com", "www.example.com\napi.example.com"))
	assert.Equal(t, []string{"example.com", "www.example.com", "api.example.com"}, splitAcmeDomains("example.com", "www.example.com, api.example.com"))
}

func TestCertificatesDueForRenewal(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	certificates := []model.TLSCertificate{
		{ID: 1, Provider: "acme", AutoRenew: true, ApplyStatus: "ready", PrimaryDomain: "due.example.com", NotAfter: now.Add(3 * 24 * time.Hour)},
		{ID: 2, Provider: "acme", AutoRenew: true, ApplyStatus: "ready", PrimaryDomain: "fresh.example.com", NotAfter: now.Add(30 * 24 * time.Hour)},
		{ID: 3, Provider: "upload", AutoRenew: true, ApplyStatus: "ready", PrimaryDomain: "upload.example.com", NotAfter: now.Add(24 * time.Hour)},
		{ID: 4, Provider: "acme", AutoRenew: false, ApplyStatus: "ready", PrimaryDomain: "manual.example.com", NotAfter: now.Add(24 * time.Hour)},
		{ID: 5, Provider: "acme", AutoRenew: true, ApplyStatus: "applying", PrimaryDomain: "busy.example.com", NotAfter: now.Add(24 * time.Hour)},
	}

	due := CertificatesDueForRenewal(certificates, now)
	require.Len(t, due, 1)
	assert.Equal(t, uint(1), due[0].ID)
	assert.Equal(t, "due.example.com", due[0].PrimaryDomain)
}

func TestApplyCertificateReturnsApplying(t *testing.T) {
	cleanup := setupTLSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	dnsAccount, err := CreateDNSAccount(ctx, DNSAccountInput{
		Name:          "Test Cloudflare",
		Type:          "cloudflare",
		Authorization: `{"api_token": "dummy_token"}`,
	})
	require.NoError(t, err)

	restore := SetObtainCertificateFuncForTest(func(ctx context.Context, cert *model.TLSCertificate) error {
		return updateCertError(ctx, cert, "dns challenge failed")
	})
	defer restore()

	cert, err := ApplyCertificate(ctx, ApplyInput{
		Name:          "Test ACME Cert",
		PrimaryDomain: "example.com",
		OtherDomains:  "*.example.com",
		DnsAccountID:  dnsAccount.ID,
		KeyAlgorithm:  "RSA2048",
		AutoRenew:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, "applying", cert.ApplyStatus)
	assert.Equal(t, "acme", cert.Provider)
}

func TestRenewCertificateSetsApplying(t *testing.T) {
	cleanup := setupTLSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	cert := &model.TLSCertificate{
		Name:          "renew-cert",
		Provider:      "acme",
		AutoRenew:     true,
		ApplyStatus:   "ready",
		PrimaryDomain: "renew.example.com",
		CertPEM:       " ",
		KeyPEM:        " ",
	}
	require.NoError(t, model.CreateTLSCertificateRecord(ctx, cert))

	restore := SetObtainCertificateFuncForTest(func(ctx context.Context, c *model.TLSCertificate) error {
		return nil
	})
	defer restore()

	renewed, err := RenewCertificate(ctx, cert.ID)
	require.NoError(t, err)
	assert.Equal(t, "applying", renewed.ApplyStatus)
}

func TestConvertCertificateToACMEPreservesUploadOnFailure(t *testing.T) {
	cleanup := setupTLSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	originalCertPEM, originalKeyPEM := generateTestCertificatePair(t, []string{"manual.example.com"})
	cert, err := CreateCertificate(ctx, CertificateInput{
		Name:    "manual-cert",
		CertPEM: originalCertPEM,
		KeyPEM:  originalKeyPEM,
	})
	require.NoError(t, err)

	stored, err := model.GetTLSCertificateByID(ctx, cert.ID)
	require.NoError(t, err)
	originalStoredCertPEM := stored.CertPEM
	originalStoredKeyPEM := stored.KeyPEM

	stored.ApplyStatus = "applying"
	stored.PrimaryDomain = "manual.example.com"
	require.NoError(t, model.SaveTLSCertificate(ctx, stored))

	err = updateCertError(ctx, stored, "dns challenge failed")
	require.Error(t, err)

	finalCert, err := model.GetTLSCertificateByID(ctx, cert.ID)
	require.NoError(t, err)
	assert.Equal(t, "upload", finalCert.Provider)
	assert.Equal(t, "error", finalCert.ApplyStatus)
	assert.Equal(t, originalStoredCertPEM, finalCert.CertPEM)
	assert.Equal(t, originalStoredKeyPEM, finalCert.KeyPEM)
	assert.True(t, strings.Contains(finalCert.ApplyMessage, "dns challenge failed"))
}

func TestConvertCertificateToACMERejectsInvalidStates(t *testing.T) {
	cleanup := setupTLSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	certPEM, keyPEM := generateTestCertificatePair(t, []string{"manual.example.com"})
	cert, err := CreateCertificate(ctx, CertificateInput{
		Name:    "manual-cert",
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	})
	require.NoError(t, err)

	cert.Provider = "acme"
	require.NoError(t, model.SaveTLSCertificate(ctx, cert))
	_, err = ConvertCertificateToACME(ctx, cert.ID, ApplyInput{Name: "manual-cert"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only uploaded")

	cert.Provider = "upload"
	cert.ApplyStatus = "applying"
	require.NoError(t, model.SaveTLSCertificate(ctx, cert))
	_, err = ConvertCertificateToACME(ctx, cert.ID, ApplyInput{Name: "manual-cert"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already applying")
}
