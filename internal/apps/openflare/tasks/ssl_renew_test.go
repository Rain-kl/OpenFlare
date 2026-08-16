// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSSLRenewTestDB(t *testing.T) func() {
	t.Helper()

	task.RegisterTaskMeta(tls.SSLSingleRenewMeta)

	_, mr, cleanup := testhelper.SetupTestEnvironment(t)
	require.NoError(t, db.DB(nil).AutoMigrate(&model.TLSCertificate{}, &model.TaskExecution{}))

	// task 包 init() 会按配置创建指向真实 Redis 的客户端；测试显式改用
	// miniredis（与 executor_test 一致），避免依赖本地 redis 实例。
	oldClient := task.AsynqClient
	task.AsynqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = task.AsynqClient.Close()
		task.AsynqClient = oldClient
	})

	oldSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "test_session_secret_for_ssl_renew"
	return func() {
		config.Config.App.SessionSecret = oldSecret
		cleanup()
	}
}

func TestRunSSLRenewJobTriggersDueCertificates(t *testing.T) {
	cleanup := setupSSLRenewTestDB(t)
	defer cleanup()
	ctx := context.Background()

	restore := tls.SetObtainCertificateFuncForTest(func(ctx context.Context, cert *model.TLSCertificate) error {
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
