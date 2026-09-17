// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"testing"
	"time"

	"Wavelet/openflare/plugins/server/kernel/model"
	"Wavelet/openflare/plugins/server/kernel/repository"
	"Wavelet/openflare/plugins/server/kernel/runtimeconfig"
	oftask "Wavelet/openflare/plugins/server/kernel/task"
	"Wavelet/openflare/plugins/server/kernel/testhelper"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSSLRenewTestDB(t *testing.T) func() {
	t.Helper()

	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	require.NoError(t, repository.DB(context.Background()).AutoMigrate(&model.TLSCertificate{}, &model.TaskExecution{}))
	previous := runtimeconfig.Get()
	runtimeconfig.SetSessionSecret("test_session_secret_for_ssl_renew")
	oftask.SetService(&testhelper.NoopTaskService{})
	return func() {
		runtimeconfig.Set(previous)
		cleanup()
	}
}

func TestRunSSLRenewJobTriggersDueCertificates(t *testing.T) {
	cleanup := setupSSLRenewTestDB(t)
	defer cleanup()
	ctx := context.Background()

	restore := SetObtainCertificateFuncForTest(func(ctx context.Context, cert *model.TLSCertificate) error {
		return nil
	})
	defer restore()

	now := time.Now().UTC()
	due := &model.TLSCertificate{
		Name:          "due-cert",
		Provider:      "acme",
		AutoRenew:     true,
		ApplyStatus:   "ready",
		PrimaryDomain: "due.example.com",
		CertPEM:       " ",
		KeyPEM:        " ",
		NotAfter:      now.Add(2 * 24 * time.Hour),
	}
	fresh := &model.TLSCertificate{
		Name:          "fresh-cert",
		Provider:      "acme",
		AutoRenew:     true,
		ApplyStatus:   "ready",
		PrimaryDomain: "fresh.example.com",
		CertPEM:       " ",
		KeyPEM:        " ",
		NotAfter:      now.Add(30 * 24 * time.Hour),
	}
	require.NoError(t, repository.CreateTLSCertificateRecord(ctx, due))
	require.NoError(t, repository.CreateTLSCertificateRecord(ctx, fresh))

	require.NoError(t, RunSSLRenewJob(ctx))

	renewed, err := repository.GetTLSCertificateByID(ctx, due.ID)
	require.NoError(t, err)
	assert.Equal(t, "applying", renewed.ApplyStatus)

	unchanged, err := repository.GetTLSCertificateByID(ctx, fresh.ID)
	require.NoError(t, err)
	assert.Equal(t, "ready", unchanged.ApplyStatus)
}
