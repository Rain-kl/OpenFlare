// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
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

func setupSSLRenewTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.TLSCertificate{}))

	db.SetDB(sqliteDB)
	oldSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "test_session_secret_for_ssl_renew"
	return func() {
		db.SetDB(nil)
		config.Config.App.SessionSecret = oldSecret
	}
}

func TestRunSSLRenewJobTriggersDueCertificates(t *testing.T) {
	cleanup := setupSSLRenewTestDB(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
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
	require.NoError(t, model.CreateTLSCertificateRecord(ctx, due))
	require.NoError(t, model.CreateTLSCertificateRecord(ctx, fresh))

	require.NoError(t, RunSSLRenewJob(ctx))

	renewed, err := model.GetTLSCertificateByID(ctx, due.ID)
	require.NoError(t, err)
	assert.Equal(t, "applying", renewed.ApplyStatus)

	unchanged, err := model.GetTLSCertificateByID(ctx, fresh.ID)
	require.NoError(t, err)
	assert.Equal(t, "ready", unchanged.ApplyStatus)
}
