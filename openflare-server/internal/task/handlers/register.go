// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package handlers 注册异步任务处理器
package handlers

import (
	"github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare"
	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/apps/user"
	"github.com/Rain-kl/Wavelet/internal/task"
)

// Register registers all built-in task handlers and their metadata.
func Register() {
	task.RegisterHandler(upload.StorageMigrationTask, &upload.MigrationHandler{})
	task.RegisterTaskMeta(upload.StorageMigrationMeta)

	// system cleanup
	task.RegisterHandler(upload.SystemCleanupTask, &upload.SystemCleanupHandler{})
	task.RegisterTaskMeta(upload.SystemCleanupMeta)

	// upload
	task.RegisterHandler(upload.WarmImageCacheTask, &upload.WarmImageCacheHandler{})
	task.RegisterTaskMeta(upload.WarmImageCacheMeta)

	task.RegisterHandler(upload.RebuildUploadStatsTask, &upload.RebuildUploadStatsHandler{})
	task.RegisterTaskMeta(upload.RebuildUploadStatsMeta)

	// user
	task.RegisterHandler(user.SendEmailTask, &user.SendEmailHandler{})
	task.RegisterTaskMeta(user.SendEmailMeta)

	// push
	task.RegisterHandler(push.SendNotificationTask, &push.PushHandler{})
	task.RegisterTaskMeta(push.SendNotificationMeta)

	// openflare
	task.RegisterHandler(openflare.SSLRenewTask, &openflare.SSLRenewHandler{})
	task.RegisterTaskMeta(openflare.SSLRenewMeta)

	task.RegisterHandler(openflare.DatabaseAutoCleanupTask, &openflare.DatabaseAutoCleanupHandler{})
	task.RegisterTaskMeta(openflare.DatabaseAutoCleanupMeta)

	task.RegisterHandler(openflare.WAFIPGroupSyncTask, &openflare.WAFIPGroupSyncHandler{})
	task.RegisterTaskMeta(openflare.WAFIPGroupSyncMeta)

	task.RegisterHandler(openflare.UptimeKumaSyncTask, &openflare.UptimeKumaSyncHandler{})
	task.RegisterTaskMeta(openflare.UptimeKumaSyncMeta)
}
