// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	cf "github.com/Rain-kl/Wavelet/internal/apps/openflare/cloudflare"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/dnsaccount"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netquality"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pages"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tasks"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/apps/user"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
)

// registerBuiltInTasks registers all built-in task handlers and their metadata.
func registerBuiltInTasks() {
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
	task.RegisterHandler(tasks.SSLRenewTask, &tasks.SSLRenewHandler{})
	task.RegisterTaskMeta(tasks.SSLRenewMeta)

	task.RegisterHandler(tasks.WAFIPGroupSyncTask, &tasks.WAFIPGroupSyncHandler{})
	task.RegisterTaskMeta(tasks.WAFIPGroupSyncMeta)

	task.RegisterHandler(tasks.UptimeKumaSyncTask, &tasks.UptimeKumaSyncHandler{})
	task.RegisterTaskMeta(tasks.UptimeKumaSyncMeta)

	task.RegisterHandler(tasks.LogDBSwitchTask, &tasks.LogDBSwitchHandler{})
	task.RegisterTaskMeta(tasks.LogDBSwitchMeta)

	task.RegisterHandler(cf.SyncMemberTask, &cf.SyncMemberTaskHandler{})
	task.RegisterTaskMeta(cf.SyncMemberMeta)
	task.RegisterHandler(cf.SyncGroupTask, &cf.SyncGroupTaskHandler{})
	task.RegisterTaskMeta(cf.SyncGroupMeta)
	task.RegisterHandler(cf.SyncByNodeTask, &cf.SyncByNodeTaskHandler{})
	task.RegisterTaskMeta(cf.SyncByNodeMeta)
	task.RegisterHandler(cf.FailoverCheckTask, &cf.FailoverCheckTaskHandler{})
	task.RegisterTaskMeta(cf.FailoverCheckMeta)

	task.RegisterHandler(dnsaccount.RefreshTask, &dnsaccount.RefreshHandler{})
	task.RegisterTaskMeta(dnsaccount.RefreshMeta)

	// pages source actions are only dispatched by the Pages domain API/scanner.
	task.RegisterHandler(pages.PagesSourceScanTask, &pages.SourceScanHandler{})
	task.RegisterTaskMeta(pages.PagesSourceScanMeta)

	task.RegisterHandler(pages.PagesSourceActionTask, &pages.SourceActionHandler{})
	task.RegisterTaskMeta(pages.PagesSourceActionMeta)

	// tls single renew
	task.RegisterHandler(tls.SSLSingleRenewTask, &tls.SSLSingleRenewHandler{})
	task.RegisterTaskMeta(tls.SSLSingleRenewMeta)

	// network intelligence & quality matrix
	task.RegisterHandler(netintel.NetIntelSyncTask, &netintel.SyncHandler{})
	task.RegisterTaskMeta(netintel.NetIntelSyncMeta)

	task.RegisterHandler(netquality.NetQualityRollupTask, &netquality.RollupHandler{})
	task.RegisterTaskMeta(netquality.NetQualityRollupMeta)

	task.RegisterHandler(netquality.NetQualityProbeTask, &netquality.ProbeHandler{})
	task.RegisterTaskMeta(netquality.NetQualityProbeMeta)
}
